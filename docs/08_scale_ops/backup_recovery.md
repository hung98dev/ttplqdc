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

Storage activation requires observed independent durability, atomic create-if-absent and complete, strongly consistent paginated LIST/GET under the ledger prefix; merely possessing an S3-shaped URL is insufficient. The writer cannot overwrite/delete objects; a separate operations janitor credential can delete only after checking lifecycle eligibility. Uncompleted `erasure_intents` and PREPARED objects **never expire**. Completed intents/objects may be deleted only after independently verified database completion, `completed_at + INTERVAL '6 months' + INTERVAL '30 days'`, and proof that no retained restore point precedes that completion. Lack of completion evidence (including after database loss) blocks deletion. The janitor checks the current completion/backup inventory, not object creation age; no blanket bucket TTL applies to PREPARED objects. Preserve `ERASURE_LEDGER_SALT` securely for the complete restore/ledger lifetime.

The salted account hash is pseudonymous recovery metadata, not proof of anonymization; its restricted erasure-compliance retention is enumerated in `personal_data_register.md`. Restore-point deletion within the six-month maximum and ledger coverage are both required.

## Restore Procedure
Recovery runbook:
1. stop/cordon writes,
2. choose verified restore point/PITR timestamp,
3. restore into isolated environment,
4. run database integrity/schema checks,
5. verify critical counts/foreign keys/idempotency tables,
6. reconcile Auction escrow/proceeds and Reward Claims,
6a. reconcile WorldConsequence aggregate: load from backup, verify schema, foreign keys, and internal consistency against the checkpoint manifest; an absent table or corrupted payload **must fail the restore**. Zero rows is always a valid state (fresh launch, or every relic expired); the check fails only on an unreadable table or a row referencing unknown content IDs (stale markers are repaired by the partition-start load, `../06_data/data_model.md` § Boss Aftermath Relic); do not promote a recovered database where partitions would start against stale or missing world-consequence state,
6b. re-validate time-sensitive expiries: relic expiry timestamps and timed cosmetic expiry timestamps were stored against server wall time; after PITR, re-evaluate all `timestamptz` expiry fields against the restored-as-of timestamp (not the restore-execution time) and mark expired entries as expired; entries that fall in the recovery window must be resolved by policy, not silently carried forward as active,
6c. re-apply erasures: while all public writes, erasure producers and ledger janitors are cordoned, completely paginate LIST and GET-validate **every** retained PREPARED object, regardless of its preparation time relative to PITR. Match its hash against restored accounts using the preserved salt; run `LEDGER_REPLAY` with the exact operation ID and preparation metadata for each match (no pending/fence precondition; already-erased or absent accounts remain erased/absent). Preparation can precede the restore point while destruction follows it, so timestamp filtering is forbidden. Also resume every restored pending `erasure_intents` row through the same publication/verification/destruction protocol. Record enumerated object keys/digests and replay outcomes in the restore evidence. Missing permissions/salt, incomplete pagination, storage outage, invalid/mismatching objects or any unresolved matching intent block promotion; neither a timeout nor RPO allowance waives completeness. Then confirm no matched restored account retains personal rows before releasing the cordon,
7. verify active content/schema compatibility,
8. run smoke login/character/inventory/economy tests,
9. promote recovered database,
10. restart routing/simulation from canonical recovery rules.

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
