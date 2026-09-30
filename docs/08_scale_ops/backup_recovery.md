# Backup and Recovery
status: LOCKED

## Scope
Defines launch backup scope, PostgreSQL recovery targets, restore validation, and disaster-recovery behavior.

## Durable Scope
Back up all canonical PostgreSQL durable gameplay/account state required to reconstruct:
- characters/progression,
- inventory/equipment/currency,
- quest/build/Soul state,
- guild/social durable state,
- Auction escrow/proceeds and trade settlement records,
- Reward Claims,
- cosmetics/entitlements,
- **WorldConsequence durable aggregate** (Di Tích world-consequence state; a partition must load this before accepting players — a missing or unreadable table is an unsafe restore; zero rows is valid),
- idempotency/audit records required for safe replay/recovery,
- active content revision metadata.

In-memory simulation state is not a backup target.

## PostgreSQL Strategy
Tooling (ADR-0066): pgBackRest `2.59.1` (`pgbackrest=2.59.1-1.pgdg24.04+1`) on the PostgreSQL host (Ubuntu Server 24.04 LTS, `postgresql-18=18.6-1.pgdg24.04+2`; pins in `../00_context/technology_versions.md` § Production Operations), repository = the S3-compatible bucket configured in Owner Setup and injected at deploy time as `BACKUP_STORAGE_URL`. The erasure ledger lives in the same bucket (§ Erasure Ledger), independently of the primary database/WAL. The server uses Go standard library `net/http` + `crypto/hmac` AWS Signature V4 conditional PUT, GET and LIST requests (no S3 SDK dependency), with separate least-privilege ledger credentials.

### Backup Storage Configuration (ADR-0070)
```text
BACKUP_STORAGE_URL               s3://<bucket>/<prefix>?region=<aws-region>&endpoint=<https-url>
                                 bucket/prefix = pgBackRest repo1-s3-bucket / repo1-path; region = SigV4 signing region;
                                 endpoint = S3-compatible HTTPS endpoint (omit for the AWS default); no other query keys
BACKUP_STORAGE_CREDENTIALS_FILE  world host: path to a 0600 file with exactly two lines, for a key limited to conditional
                                 PUT + GET + LIST under <prefix>/erasure-ledger/, no overwrite or DELETE:
                                   access_key_id=<value>
                                   secret_access_key=<value>
PGBACKREST_REPO1_S3_KEY,         PostgreSQL host only: pgBackRest repository credentials
PGBACKREST_REPO1_S3_KEY_SECRET
PGBACKREST_REPO1_CIPHER_PASS     PostgreSQL host only: the only source of the repository cipher key (aes-256-cbc)
```
The deploy step renders `/etc/pgbackrest/pgbackrest.conf` (`repo1-s3-bucket`, `repo1-path`, `repo1-s3-region`, `repo1-s3-endpoint`) from `BACKUP_STORAGE_URL`; pgBackRest reads the `PGBACKREST_REPO1_*` variables from its environment (`../07_security/external_integrations.md` env table). The world server reads only `BACKUP_STORAGE_URL` and `BACKUP_STORAGE_CREDENTIALS_FILE`.



Launch requires:
- continuous WAL archiving/PITR capability (`archive_command` = pgBackRest `archive-push`),
- at least one automated full backup weekly and a differential backup daily,
- backups stored independently from the primary database failure domain,
- encryption in transit (TLS to the repository) and at rest (pgBackRest repository cipher `aes-256-cbc`, key `PGBACKREST_REPO1_CIPHER_PASS`).

## Targets
Launch disaster-recovery targets:
```text
RPO <= 5 minutes
RTO <= 60 minutes
```

RPO/RTO are measured restore objectives, not promises that every process resumes its exact in-memory combat state.

## Retention
Minimum:
```text
daily restore points: 14 days
weekly restore points: 8 weeks
monthly restore points: 6 months
```

Six calendar months is the hard maximum age of any retained restore point, not a minimum that operators may widen. Retention configuration must enforce this maximum and the erasure-coverage conditions below together. Widening either requires an explicit privacy/backup ADR, never a unilateral bucket setting.

### Erasure Ledger (ADR-0079; supersedes ADR-0065/0070 publication order)
One immutable external object per **prepared** erasure is published and verified **before** destructive database mutation:
```text
key      <prefix>/erasure-ledger/<operation_id>.json (canonical server-job UUID v5)
body     {"format_version":2,"state":"PREPARED","operation_id":"<uuid>","account_id_hash":"<64 lowercase hex>","prepared_at":"YYYY-MM-DDTHH:mm:ss.ffffffZ"}
hash     account_id_hash = hex(SHA-256(raw 32-byte ERASURE_LEDGER_SALT || raw 16-byte account UUID))
writer   world-owned erasure worker from erasure_intents (../06_data/data_model.md § Account Erasure)
```
Object bytes are UTF-8, no BOM, whitespace or trailing newline, with members in exactly the displayed order. `prepared_at` is the first staged database timestamp, fixed UTC microsecond precision, serialized with exactly six fractional digits and `Z`; retries never regenerate it or the operation ID. Stage the intent and `accounts.erasure_started_at` under the account lock, commit, then release **all** database locks before storage calls. The fence rejects cancellation even during a storage outage. Conditional `PUT If-None-Match: *`, followed by GET and byte-for-byte comparison, establishes the external durability boundary. An already-existing matching object succeeds; mismatching/malformed objects fail closed; ambiguous PUT outcomes retry GET/the same key and bytes. No destructive transaction or completion acknowledgement is allowed before exact verification. The worker resumes pending intents at startup and every 60 s.

Storage activation requires observed independent durability, atomic create-if-absent and complete, strongly consistent paginated LIST/GET under the ledger prefix; merely possessing an S3-shaped URL is insufficient. The writer cannot overwrite/delete objects; a separate operations janitor credential can delete only after checking the **single canonical lifecycle gate** in `../07_security/personal_data_register.md` § 1.2, never object-creation age or a blanket bucket TTL. Uncompleted intents/objects never expire; verified completion, the calendar deadline and complete restore-point coverage are all required by that gate. The janitor and backup creation/deletion share an operations maintenance fence: freeze restore-point catalog changes while verifying the complete inventory and deleting eligible objects, and block cleanup during recovery. Confirm external object deletion before removing the intent holding its completion proof; ambiguity retries the same key and does not erase that proof. Preserve `ERASURE_LEDGER_SALT` securely for the complete restore/ledger lifetime.

The salted account hash is pseudonymous recovery metadata, not proof of anonymization. Every external key/body field and staged intent field, its purpose/basis, access scope and retention/at-erasure action is registered in `../07_security/personal_data_register.md` §§ 1.1–1.2; format-v2 objects contain no extra unregistered account/operator metadata. Restore-point deletion within the six-month maximum and the canonical ledger gate are both required.

## Restore Procedure
Recovery runbook:
1. stop/cordon writes,
2. choose verified restore point/PITR timestamp,
3. restore into isolated environment,
4. run database integrity/schema checks,
5. verify critical counts/foreign keys/idempotency tables, including exact restored receipt states/typed outcomes and source/content provenance; an absent or ambiguous original committed receipt/outcome is not permission to execute a journal command anew,
6. while public writes, erasure producers and ledger janitors remain cordoned, completely paginate LIST and GET-validate every retained PREPARED object regardless of preparation time relative to PITR. Match preserved salted hashes to restored accounts; following `data_model.md` § Account Erasure's continuation protocol, durably stage/reuse each matching object's exact pending intent and account admission fence for accounts not conclusively erased, and establish fences for every restored pending intent, before any local journal/domain callback. An exact committed completion is reconciled without reopening its completed intent; absent accounts stay absent. Complete storage verification outside database locks; stage/recheck intent/fence metadata in short canonical database transactions. This external pre-fence phase performs no destruction or completion acknowledgement. Missing/partial ledger enumeration or conflicting intent/object metadata fails restore; record keys/digests and exact metadata in the restore report,
6a. retain and validate the protected local outbox inventory and exact immutable referenced content revisions. Replay/dispose all local queued-client/server/chat-log references through `deployment.md` § Durable Outbox Journal, respecting external erasure fences: cancel uncommitted erased-subject commands, terminally reconcile other records, no reroll or active-content substitute. ERASURE_RESUME uses the canonical verified intent/PREPARED/fence continuation handoff, not a destructive callback or erasure-success acknowledgement. All referring files require terminal records followed by unlink/directory-fsync without database locks; dispose other live queue/in-flight references as well. Missing restored receipt/generic outcome/source proof, unverified continuation or incomplete file disposition fails restore; no public trusted-replay flag,
6b. only after complete local outbox/subject-reference disposal, finish destructive `LEDGER_REPLAY` for every matching object with its exact operation/preparation metadata and resume pending intents through the canonical erasure worker (`data_model.md` § Account Erasure; `deployment.md` startup step 6). It may erase restored ACTIVE accounts without a pending-deletion precondition, but the exact durable continuation/fence already exists before callbacks. Already-erased/absent accounts stay erased/absent. Record actual completion outcomes; a crash after handoff/disposal resumes from existing pending intents/objects, never treats handoff as completion. No timestamp filter or age-only cleanup,
6c. reconcile Auction escrow/proceeds and Reward Claims after journal/erasure disposition,
6d. reconcile WorldConsequence aggregate: load from backup, verify schema, foreign keys, and internal consistency against the checkpoint manifest; an absent table or corrupted payload must fail restore. Zero rows is valid; unreadable tables or unknown content IDs fail. Partition-start load repairs stale markers under `data_model.md` § Boss Aftermath Relic,
6e. re-validate time-sensitive expiries: stored wall-time relic/cosmetic expiries are evaluated against the restored-as-of timestamp, not restore-execution time; mark expired entries and explicitly resolve the recovery-window policy before promotion rather than carrying stale entries forward as active,
7. verify active content/schema compatibility,
8. run smoke login/character/inventory/economy tests,
9. promote recovered database,
10. restart routing/simulation only after the canonical erasure/competitive/boss/chest/world recovery order completes. Local outbox evidence is host-local, not restored by a PostgreSQL backup; preserve any available protected files and never claim WAL alone recovers commands absent from both restored durable proof and validated local inventory.

Do not restore client snapshots as server truth.

## Backup Verification
A backup is not considered valid merely because upload succeeded.

Required:
- automated backup job success monitoring,
- checksum/integrity validation,
- scheduled restore drill at least monthly,
- documented measured restore duration,
- sampled application-level verification after restore.

## Point-in-Time Recovery Safety
PITR may replay committed DB transactions only; external side effects must be reconciled using operation/audit IDs.

After recovery, idempotency records must prevent clients/workers from duplicating already-restored value mutations.

## Incident Behavior
If PostgreSQL durability cannot be guaranteed:
- reject/fail closed for value-changing operations,
- do not continue issuing items/currency in an unpersisted memory-only mode,
- simulation may enter controlled drain/maintenance behavior.

## Invariants
- canonical durable state is recoverable from PostgreSQL backup/PITR.
- RPO <=5m, RTO <=60m target.
- backups are outside primary failure domain.
- monthly restore drill is mandatory.
- value mutation never falls back to non-durable success.
- WorldConsequence aggregate is in backup scope; an absent/unreadable table or a row with unknown content IDs fails the restore; zero rows is valid.
- after PITR, timestamptz expiries are re-validated against the restored-as-of time, not restore-execution time.
- erasure requires immutable externally verified PREPARED intent before destruction; no network call holds database locks.
- every retained PREPARED object is replayed independently of PITR timestamps; incomplete coverage blocks promotion.
- restore points are at most six calendar months old; pending erasure intents/objects never expire; completed cleanup uses completion plus six calendar months plus 30 days and absence of older restore points.
