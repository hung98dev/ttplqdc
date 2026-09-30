# Deployment
status: LOCKED

## Scope
Defines environments, backend artifact flow, PostgreSQL migration order, content activation, rollout, drain, and rollback.

## Environments
At minimum:
```text
local/dev
staging
production
```

Staging uses production-like protocol/content/database migration flow and can execute load/fault tests without production player value.

Production hosts (Ubuntu Server 24.04 LTS, `../00_context/technology_versions.md` § Production Operations; ADR-0066): the world host (`thinhthan-server`, OpenTelemetry Collector, node_exporter), the PostgreSQL host (PostgreSQL 18.6, pgBackRest, postgres_exporter, node_exporter) and the ops host (Prometheus, Alertmanager, Grafana, node_exporter). Their configuration is versioned in `deploy/prod/`; secrets and receiver/backup endpoints are injected at runtime.

## Artifacts
Backend deployment ships one immutable artifact per release: the statically linked `thinhthan-server` binary (`CGO_ENABLED=0`, `GOOS=linux`, `GOARCH=amd64`, built by the pinned Go toolchain on the Linux CI job (ADR-0058)) plus the pinned Mozilla CA bundle (`../00_context/technology_versions.md`). Production runs it as one systemd service on one Linux host (`deploy/prod/thinhthan-server.service`); PostgreSQL 18.6 runs on its own host. No container runtime or Kubernetes is used in production (one process, ADR-0052).

Artifact identity includes:
- source commit,
- build version,
- canonical technology-version-matrix revision,
- exact Go/Unity/protobuf toolchain versions where applicable,
- resolved direct dependency/lockfile checksum state,
- protocol support range,
- schema migration compatibility,
- content schema compatibility.

Unity client builds are versioned separately and gated through protocol/versioning rules. Production build/bootstrap tooling must fail rather than silently using an editor/tool/dependency version that differs from `../00_context/technology_versions.md`.

## Deployment Order
Safe default order for compatible changes:
1. deploy backward-compatible PostgreSQL migration (expand),
2. deploy backend capable of old+new representation,
3. verify health,
4. publish and verify any required immutable Addressables catalog/bundles before they are referenced,
5. activate compatible static content revision if part of release,
6. release/gate Unity build and compatible asset-catalog revision,
7. after compatibility window, remove retired paths in a later deploy (contract).

Do not deploy destructive DB change before all running code stops depending on the old shape.

## Migrations
Database migrations:
- are versioned,
- are reviewed/tested against production-like volume,
- have explicit lock/runtime expectations,
- use expand/contract for online changes,
- do not silently rewrite large tables inside startup path,
- are run once by the deploy step before the new process starts, never by the server at startup.

## Backend Deploy (Maintenance Restart)
The deployable unit is the single `thinhthan-server` world process (ADR-0044, ADR-0052); there are no replicas. A deploy is a maintenance restart: `systemctl stop thinhthan-server` (SIGTERM) runs § Drain and Shutdown, migrations run, then `systemctl start` starts the new build; owners recover/reconnect through canonical checkpoint/entry recovery.

Do not have two processes simultaneously own one partition to make rollout appear seamless.

### Drain and Shutdown (ADR-0066)
```text
DRAIN_LEAD            = 600 s   announce -> simulation stop
SHUTDOWN_FLUSH_MAX    = 60 s    durable queue flush after simulation stop
DRAIN_RECONNECT_AFTER = 120 s   reconnect_after_ms in S2C_SERVER_DRAINING
systemd unit          KillSignal=SIGTERM, TimeoutStopSec=780 (DRAIN_LEAD + SHUTDOWN_FLUSH_MAX + 120 s margin), Restart=on-failure
```
SIGTERM or SIGINT starts the sequence once (a second signal is ignored; only the systemd timeout kills):
1. `t0`: refuse new logins, gameplay tickets, character attaches, login-queue admission, instance creation and matchmaking/queue joins with `SERVER_DRAINING`; send `S2C_SERVER_DRAINING` (`reason = MAINTENANCE`, `drain_deadline_ms = t0 + DRAIN_LEAD`, `reconnect_after_ms = DRAIN_RECONNECT_AFTER`) to every session. Attached players keep playing; reconnects of already-attached characters inside their grace window are still accepted; normal-map transfers continue within their budgets.
2. `t0 .. t0 + DRAIN_LEAD`: running instances may finish; nothing new is created.
3. `t0 + DRAIN_LEAD`: every partition finishes its current tick, runs `EMIT_DURABLE_COMMANDS` and stops ticking; unfinished transfers cancel to the source checkpoint; open instances close without completion (only committed rewards stand, `../02_world/dungeons.md`); every attached character's checkpoint is enqueued.
4. Flush: stop all producers (partition ticks plus Global schedule transitions, lifecycle callbacks and background admissions), then Durable Domain drains until empty or `SHUTDOWN_FLUSH_MAX`; WSS connections close with `SERVER_DRAINING`; listeners stop. At the deadline cancel/wait for transaction executors to quiesce and freeze the unacknowledged-command inventory before closing the pgx pool; an ambiguous in-flight commit remains in that inventory. No executor may remove/modify a frozen record or issue a post-snapshot acknowledgement.
5. Exit 0 only with `durable_queue_depth = 0` and no unacknowledged in-flight commands. On timeout write **every** frozen queued/in-flight command using the codec below, log critical `shutdown_flush_timeout` with the remaining depth, and exit 1 only after the file is durably published. Coverage is the closed producer registry in `../06_data/save_rules.md`, not five representative server commands. It includes accepted queued client requests, standalone Quest/Discovery, PUBLIC schedule/eligibility/chest transitions, all finalized rewards, checkpoints, match/Guild EventSink outputs and queued jobs. No client retry is required to reconcile that published inventory.

### Durable Outbox Journal (ADR-0070)
The exact typed schema is `../05_network/protobuf_conventions.md` §7; identities, client receipt trust/expiry and terminal outcomes are `../06_data/ids.md` § Trusted Queued-Client Replay. No opaque payload bytes, JSON commands or network Envelope are a journal representation.

```text
DURABLE_OUTBOX_DIR = /var/lib/thinhthan/outbox       -- local Linux filesystem, systemd StateDirectory
filename          = <boot_uuid>.journal -> <boot_uuid>.ready
boot_uuid         = lowercase canonical crypto-v4 UUID, new per process start
header            = magic[8] | boot_uuid[16] | record_count:u64le | data_length:u64le | header_crc:u32le
magic             = ASCII "TTJRN001" (54 54 4a 52 4e 30 30 31)
header_crc        = CRC32C of the first 40 header bytes, Castagnoli, reflected poly 0x82f63b78,
                    initial 0xffffffff, final xor 0xffffffff
record            = payload_length:u32le | DurableCommandRecord protobuf bytes | record_crc:u32le
record_crc        = same CRC32C over the exact four little-endian length bytes then the payload bytes
data_length       = sum(payload_length + 8), excludes the 44-byte header
file_length       = exactly 44 + data_length, no trailer/padding/trailing bytes
payload_length    = 1..1048576 bytes, bounds and typed nested limits checked before allocation
record_count      = 1..1048576, also equals frozen inventory count
data_length       = <= record_count * (1048576 + 8), absolute ceiling 1099520016384;
                    every length/count/product/addition uses checked uint64 arithmetic
write budget      = the existing 120 s TimeoutStopSec margin; failure never logs a successful publication
```

`boot_uuid` in the filename and header must match. Records are in global admission_sequence order (strictly increasing within this boot; assigned before enqueue, never reused/overflowed). The queue inventory covers every producer/queue and in-flight executor; duplicates of the same owner/family/operation/fingerprint may be encoded, never two conflicting commands. No file-size limit permits truncating the inventory: bounded queues plus codec bounds must fit; a process unable to encode/publish the complete snapshot exits nonzero with `durable_outbox_write_failed`, retains all available bytes and makes no maintenance-losslessness claim.

Serialize/validate records as a bounded stream (one ≤1MiB record buffer), not by allocating/copying the entire journal; the frozen queue inventory holds immutable command references. Activation proves `sum(all configured durable queue capacities + per-partition holding capacities + maximum in-flight executors) <= 1048576` and reserves outbox disk space for that count × (1048576+8) +44 bytes, including all 720 normal-channel partitions plus allowed instances/Global/client/job queues. The maximum canonical inventory publication must fit the existing 120-second margin in the maintenance-load scenario; insufficient disk/write throughput/configured bound refuses readiness/activation, never trims the queue or silently changes TimeoutStopSec. The bound is derived from all producers, not an arbitrary 1GiB file cap.

The directory is mode 0700 and files mode 0600, owned by the service UID; exclude symlinks, path traversal and nonregular files, use exclusive create, no overwrite of an existing boot name. Edge/client uploads and public/private admin APIs cannot write replay files or select TRUSTED_JOURNAL_REPLAY. CRC detects accidental corruption, **not** malicious authenticity; trusted server-owned serializer plus protected host files is the source of frozen admission evidence. Host/root compromise is outside this replay capability's trust boundary. Client intent fingerprint cannot authenticate newly substituted server snapshots.

Publication: serialize the complete bounded inventory to `.journal`; write the header with final count/length; fsync the file; close it; atomically rename in the same filesystem to `.ready`; fsync `DURABLE_OUTBOX_DIR` before logging publication or exiting. Do not claim fsync(file)+rename alone survives a directory-metadata power loss. Filesystem/directory-fsync capability is required at deployment activation; no fallback to an unverified filesystem.

Deterministic primitive vectors: UUID `00112233-4455-4677-8899-aabbccddeeff` encodes `00 11 22 33 44 55 46 77 88 99 aa bb cc dd ee ff`; integer length 258 encodes `02 01 00 00`; CRC32C of ASCII `123456789` is `0xe3069283` and its stored little-endian bytes are `83 92 06 e3`. These are scalar/framing vectors, not abbreviated valid command records; valid golden records must supply every §7-required field and matching family/type/owner.

Startup, before Global/Sim owners or any player admission:
1. Open exclusively as the sole process owner; inventory all `.ready` and `.journal` files. Validate **all files and all records** (header/length/count/CRC/schema/discriminator/oneof/owner/family/UUID/fingerprint/source/revision/bounds) before executing any. Load the exact referenced immutable content revisions; absence/incompatibility blocks startup, never reinterpret with current content. On a restored database, `backup_recovery.md` § Restore Procedure step 6 must already have completed external PREPARED-object enumeration/verification and durable intent/admission-fence staging before any local journal/domain callback; this pre-fence phase is not destructive erasure replay. An ACTIVE restored account is not proof its pending command remains admissible. Normal startup also establishes known erasure-intent fences before trusted callbacks; both defer destruction until subject references are durably disposed.
2. A complete valid `.journal` is an interrupted publication: fsync it, rename to `.ready`, fsync the directory and include it. A torn/short/bad `.journal` is retained at its original recognized path and stops startup with `durable_outbox_corrupt`; never silently ignore it, rename it out of the startup inventory or claim its queued work survived. Unknown/quarantined files in the outbox also block startup until conclusive recovery disposition. A missing file after abrupt termination remains the bounded crash rule, not proof of a published journal.
3. Replay ready files sorted by canonical boot UUID raw-byte order, records in file order. No new producer starts while any file remains unresolved, so later boot files cannot overtake an unresolved earlier dependency. Call the normal domain mutation core through the private trusted replay entry, **not** the public UUIDv7 admission gate. Client receipts reconstruct COMMITTED, preserve REJECTED or conclusively terminalize expired ADMITTED as EXPIRED_UNCOMMITTED; server v5 commands use retained outcomes and their actual owning source dedup (never a partial reward-claim contribution as proof of the whole kill). ERASURE_RESUME instead uses `../06_data/data_model.md` § Account Erasure's source-backed continuation handoff; do not invoke its destructive worker while the referring file exists. All queued/in-flight/journal references pin generic outcomes until disposition; no second server ledger. All retries keep original owner/ID/source/RNG/revision.
4. Record a terminal disposition only after the exact commit/prior-outcome reconstruction, conclusive domain rejection/expiry, or canonical ERASURE_RESUME durable continuation proof has been verified. That handoff terminalizes only the reference: it is neither erasure completion nor a generic erasure-success outcome/client completion acknowledgement. The same handoff rule applies to normal queue processing. Dependency/unknown-history errors remain unresolved and stop readiness. A rejected/expired client is reconciliation with a typed terminal result, not a successful mutation and not a statement that every uncommitted request executed. A file is removable only when **all** records are terminal.
5. Unlink the complete terminal ready file; fsync the directory with no database locks held; only then release receipt/outcome/content-revision recovery holds after proving no remaining file/queue/in-flight reference. Crash before unlink repeats unchanged outcome reconciliation or the same source-backed continuation verification; crash after unlink/before release leaves a safe extra hold repaired at next startup. Never set disposition_ack_at ahead of durable unlink. An ERASURE_RESUME handoff never releases its pending intent/PREPARED continuation proof.
6. Only after the outbox is empty/resolved and all other subject queue/in-flight references are disposed, run the actual destructive erasure worker for verified ledger objects/fenced pending intents under `data_model.md` § Account Erasure's canonical lock order and reference recheck. A crash after durable disposal but before this stage is recovered from those existing intents/objects, not from a new ledger or a false erasure-success acknowledgement. Then reconcile competitive RESOLVING/admission snapshots, load/recover PUBLIC boss schedules, settle defeated-copy chest eligibility (delete only undefeated-copy rows), then start WorldConsequence partition loads and ordinary workers/Global/Sim/readiness. Recovery jobs created by these stages use the same closed typed commands and must finish before their dependent stage.

Any unknown enum/type/schema, bad CRC/count/length, conflicting fingerprint/owner, missing receipt/source, incompatible revision or ambiguous committed outcome exits 1, retains the files, alerts `durable_outbox_corrupt` (data) or `durable_outbox_reconciliation_failed` (history/dependency) and leaves not-ready. Restore/reconciliation must preserve receipt/natural-key history and the outbox; operator review is not permission to delete an unknown record. Restore/PITR can roll back database gameplay/receipt history, so replay requires the explicit backup recovery reconciliation procedure rather than assuming CRC proves database commit history.

### Journal Requirements

| Requirement ID | Measurable contract | Owning implementation packet |
|---|---|---|
| JRN-001 | Every queued/in-flight producer maps to one closed typed codec/family/owner; unknown producer refuses admission | IMP-061, IMP-082 |
| JRN-002 | UUID bytes, all field/enum tags, optionality, finalized item/RNG/source/revision outputs round-trip without an opaque command blob | IMP-061 |
| JRN-003 | Exact little-endian framing/CRC/count/length/bounds and complete-publication fsync/rename/directory-fsync; torn/unknown data stops startup | IMP-069 |
| JRN-004 | Replay retains original source incarnation/counter/tick, owner, operation, content revision and natural-key dedup; no reroll/double commit | IMP-001, IMP-005, IMP-069 |
| JRN-005 | Public UUIDv7 expiry stays 180 days; only private trusted local replay can consult prior admitted-client receipts | IMP-005, IMP-069 |
| JRN-006 | Client admission DB outage enqueues nothing; receipt + commit outcome updates are atomic and identical retries single-flight | IMP-005, IMP-082 |
| JRN-007 | Expired COMMITTED reconstructs exact retained outcome; expired uncommitted ADMITTED terminalizes without value writes; ambiguous/missing history blocks readiness | IMP-005, IMP-069 |
| JRN-008 | Outcome/receipt holds release only after every reference is terminal and unlink+directory-fsync/queue ack; ERASURE_RESUME verifies the canonical durable continuation without destruction/completion ack, and purge never removes pending proof | IMP-005, IMP-056, IMP-082, IMP-069 |
| JRN-009 | External/known-intent erasure fences precede callbacks; journal continuation handoff/disposal precedes destructive erasure, then competitive/boss/chest/world recovery finishes before ready | IMP-069 |
| JRN-010 | Every registered network ID has exactly one owner in the existing nine network proto files, including 16/307/308/442/443/516/517/518/656 unchanged | IMP-061 |

Unplanned termination (crash, kill) skips the sequence and the journal; restart recovery (`../04_architecture/realtime_loop.md` § Restart) applies; uncommitted commands are lost as bounded by `../06_data/save_rules.md` ("crash before commit").

## Content Activation
Static content revision activation is separate from binary deployment and follows atomic validation in `../06_data/config.md`.

A binary must declare which content schema versions it supports.

Invalid candidate content never partially activates.

## Health Gates
A newly started world process must pass, before it reports ready and accepts logins:
- startup/schema/content compatibility,
- PostgreSQL connectivity/pool health,
- protocol handshake,
- synthetic durable mutation/read,
- tick health of the world simulation,
- error/reconnect/latency thresholds.

A failed gate keeps the process not-ready and exits non-zero; the operator rolls back per § Rollback.

## Rollback
Backend rollback is allowed only while DB/content remain compatible with the older binary.

If a migration is not backward compatible, restore/forward-fix plan must be explicit before production deployment.

Content rollback activates the previous validated immutable revision when persistence compatibility permits; it does not mutate old IDs in place.

### Schema-Coupled Content Revisions
A content revision is tagged **`schema-coupled`** when it changes a formula whose output is persisted to character rows — specifically: EXP threshold tables, level cap, or any other value stored in durable character state rather than recomputed on load. Examples: the EXP x100 rescale that writes `current_exp` and character level to PostgreSQL.

**Binary rollback of a schema-coupled revision is not permitted.** A content rollback that reverts EXP thresholds or level caps cannot undo committed character rows; characters would be re-evaluated against old thresholds and could land above the level cap or in an invalid progression bucket. When a schema-coupled revision must be reverted:
- a compensating migration that re-normalizes affected character rows is required, or
- a forward-only fix plan (new revision that repairs the invariant without reverting) must be prepared and deployed instead of a binary rollback.

The schema-coupled tag is recorded in the content revision metadata and must be present before a release that carries formula-persisting changes may be activated.

## Secrets
Secrets/config are injected at runtime and are not baked into Unity builds, repository files, or the server binary artifact.

## Production Change Rule
Every production release records:
- backend build,
- Unity minimum/current build gate,
- DB schema version,
- active content revision,
- required Unity asset_catalog_revision / Addressables build identity when applicable,
- migration set,
- operator/deployment identity and timestamp.

## Invariants
- immutable deploy artifacts,
- expand -> code -> contract migration pattern,
- simulation ownership is drained, never duplicated,
- content activation is atomic,
- rollback compatibility is known before deploy,
- production secrets never ship in Unity client,
- schema-coupled content revisions cannot be binary-rolled-back without a migration or forward-only fix plan.
