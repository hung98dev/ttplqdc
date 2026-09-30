# Migrations
status: LOCKED

## Scope
Defines PostgreSQL schema/data migration naming, deployment ownership, compatibility, rollback, verification and failure behavior.

Canonical tool:
~~~
github.com/golang-migrate/migrate/v4 = 4.20.1
~~~

Deployment owner: ../08_scale_ops/deployment.md.

# Migration Files
Canonical implementation layout:
~~~
server/migrations/
  000001_baseline_schema.up.sql
  000001_baseline_schema.down.sql
  000002_<short_name>.up.sql
  000002_<short_name>.down.sql
~~~

Rules:
- six-digit monotonically increasing sequence,
- one sequence number used once,
- descriptive lowercase snake_case name,
- migration files are immutable after any environment applies them,
- correction after apply is a new migration.

## Ownership
Exactly one deployment/migration job owns production schema migration at a time. The world process never migrates on startup.

Backend startup checks compatible schema version and fails closed when incompatible.

# Expand / Contract
Default online change:
1. EXPAND compatibly,
2. deploy code that handles old + new shape,
3. backfill in bounded batches if needed,
4. switch reads/writes,
5. verify,
6. remove old shape in a later CONTRACT migration.

Add columns/tables/indexes before code requires them. Tighten NOT NULL/constraints only after valid backfill.

# Lock / Runtime Safety
Every production migration declares affected tables/volume, expected lock/runtime, index strategy, backfill strategy and rollback/forward-fix.

Avoid long rewrites/ACCESS EXCLUSIVE locks on hot large tables.

Operations with PostgreSQL transaction restrictions must be explicitly handled; do not hide them inside an invalid migration transaction.

# Data Backfill
Backfill is idempotent/restartable, key/batch bounded, observable, preserves ownership/value invariants and can resume after interruption.

Never run an hours-long rewrite synchronously in application startup.

# Constraints
For a new constraint on existing data:
1. detect/report invalid rows,
2. repair/migrate deterministically,
3. validate,
4. then require it for new code.

Never delete player value merely to make a constraint pass.

# Down / Rollback
Production rollback primarily uses compatible application rollback or forward-fix.

A down migration may be used locally/staging only when data-safe and tested. Do not automatically run destructive DOWN in production if it could lose or reinterpret committed player state.

If rollback requires PITR, use backup/recovery procedures rather than pretending it is a normal deploy rollback.

# Transactionality
Use migration transactions where supported and safe.

A non-transactional step must define partial-state detection/retry and a deployment gate before continuing.

# Schema Compatibility
Every `thinhthan-server` build (ADR-0044) declares readable schema compatibility. Deploys are maintenance restarts (ADR-0052); expand/contract still applies so a failed deploy can restart the previous build against the migrated schema.

# Content vs Schema
Database migration and static-content activation are separate.

Order when both change:
1. compatible DB expand,
2. backend deploy,
3. verify,
4. content activate,
5. later DB contract cleanup.

# Verification
Every migration requires:
- fresh DB apply,
- previous supported schema -> new schema,
- representative data,
- invariant/constraint tests,
- previous-build compatibility with the migrated schema (restart-to-previous-build rollback, ADR-0052),
- measured runtime/lock behavior for large tables.

Before production: backup/PITR healthy, staging rehearsal passed, one migration owner, rollback/forward-fix known.

# Security
Migration credentials are deployment-only and never available to Unity. Migration SQL is repository-controlled; no client-provided DDL.

# Named Data Migrations

## Character Update Timestamp

`characters.updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()` is part of the `000001_baseline_schema` created by IMP-005 (no migration has been merged yet; ADR-0048 amendment). Every committed update to the `characters` row sets it to the server transaction time.

## EXP Formula Rescale (exp_required = 10000 × L²)

### Production row status
This migration is **pre-production**. No character has persisted `current_exp` under any previous formula at the time `exp_required(L) = 10000 * L * L` activates (the game has not launched). This must be confirmed explicitly before activation. If an earlier formula was active during a closed beta or internal test that wrote rows to a production-equivalent environment, the Backfill Path below is required and must be sequenced before content activation.

### Column type and semantics
`current_exp` is `integer` (PostgreSQL signed 32-bit). Stored EXP is the **absolute cumulative total** across all levels; the character's current level is derived by finding the largest `L` in `[1..60]` where `cumulative_exp_to_reach(L) <= current_exp`, with `cumulative_exp_to_reach(1) = 0` and `cumulative_exp_to_reach(L) = sum(exp_required(i) for i=1..L-1)` for `L in [2..60]`. The lifetime cumulative cap at maximum level 60 under the active formula is exactly 702,100,000, within the 32-bit range (~2.147B). If a future level cap increase raises the lifetime cumulative above 2,147,483,647, a migration to `bigint` is required before that cap activates. These semantics are migration-versioned: any formula change that reinterprets stored `current_exp` requires an explicit migration per `config.md` § Versioning.
### Backfill Path (required only if pre-production invariant cannot be confirmed)
Sequence within the expand/migrate/activate pipeline — the backfill must run **after schema expand and before content activation**:

1. **Schema expand** (earlier migration): confirm `current_exp integer` column exists; if upgrading column type, add new column and backfill, then remove old.
2. **Backfill** (this migration): update in key-bounded batches using a subquery (PostgreSQL does not support ORDER BY / LIMIT directly in UPDATE):
   ```sql
   UPDATE characters
      SET current_exp = current_exp * 100
    WHERE character_id IN (
      SELECT character_id FROM characters
       WHERE character_id > $last_id
       ORDER BY character_id
       LIMIT 1000
    );
   ```
   The multiplier is 100 because the new formula requires ~100× more cumulative EXP per level boundary. The value itself cannot distinguish migrated from unmigrated rows, so double-application is prevented by running the backfill exactly once inside its versioned golang-migrate step; a partially applied batch run resumes from the last committed `$last_id`. The deployment gate enforces that content activation follows this step.
3. **Content activate** `exp_required(L) = 10000 * L * L` only after the backfill migration has applied and been verified.

### Staging rehearsal requirement
Before the backfill applies to any production-equivalent environment, a staging rehearsal must verify:
- no character loses a level after the multiplier (final derived level ≥ original level for every row),
- no character unintentionally gains a level (the ×100 multiplier should map existing EXP below old boundaries proportionally; if any character would gain a free level, the backfill logic must be redesigned before production apply),
- `current_exp * 100` does not overflow `integer` for any existing row,
- the backfill is restartable: interrupted at any batch boundary and re-run, rows already multiplied must not be multiplied again (use a schema-level migration applied flag or verify by level consistency check before each batch).

# Invariants
~~~
tool = golang-migrate 4.20.1
migration number = unique + monotonic
applied migration files immutable
production migration has one owner
expand -> code -> contract
no destructive automatic rollback of player data
large backfill bounded + restartable
schema migration != content activation
EXP rescale migration = pre-production; if applicable: backfill (×100) after schema expand, before content activation; staging rehearsal confirms no level loss and no unintended level gain
current_exp stores absolute cumulative total; column type = integer (32-bit); formula change -> migration required
~~~
