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
4. Flush: Durable Domain drains its queue until empty or `SHUTDOWN_FLUSH_MAX`; WSS connections close with `SERVER_DRAINING`; listeners stop; the pgx pool closes.
5. Exit 0 with `durable_queue_depth = 0` logged. If the flush deadline passes, every command still queued or in flight (unacknowledged commit) is written to the **durable outbox journal** (below), log critical `shutdown_flush_timeout` with the remaining depth, and exit 1. Nothing relies on clients retrying: most queued commands are server-originated (kill settlements, checkpoints, `WriteWorldConsequence`, chest settlement).

### Durable Outbox Journal (ADR-0070)
```text
DURABLE_OUTBOX_DIR   = /var/lib/thinhthan/outbox   (systemd StateDirectory, same host, not PostgreSQL)
file                 = <boot_id>.journal, written then fsync'd, then renamed to <boot_id>.ready
record               = uint32 length | DurableCommandRecord (protobuf) | uint32 CRC32C of the record bytes
DurableCommandRecord = operation_family, owner_id (16 B), operation_id (16 B), command_type, payload bytes, enqueued_at_ms
write budget         = the 120 s TimeoutStopSec margin
```
- The journal is written without PostgreSQL, so a database outage (the usual flush-timeout cause) cannot lose the queue.
- Startup order (before § Health Gates report ready and before any partition accepts players): (1) replay every `*.ready` file in `boot_id` order, records in file order, each through its normal durable command handler; every command is idempotent on `(operation_family, owner_id, operation_id)` (`../06_data/data_model.md` § operations), so a command that did commit before shutdown reconstructs its outcome instead of writing twice; (2) delete a file only after all its records committed; (3) then run PUBLIC boss schedule load, chest-eligibility settlement and WorldConsequence partition loads.
- A record with a bad CRC or unknown `command_type` stops startup (exit 1, alert `durable_outbox_corrupt`); the operator restores from backup/PITR (`backup_recovery.md`). A missing or partially written `.journal` (never renamed) means the write did not finish: the process was killed, so the crash rule below applies.

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
