---
description: PostgreSQL schema + golang-migrate rules
trigger: glob
globs:
  - "server/migrations/**"
  - "server/internal/durable/**"
---

# Database & Migration Rules

Canonical: `docs/06_data/migrations.md`, `database.md`, `physical_schema_contract.md`. PostgreSQL `18.6`, golang-migrate `v4.20.1`, pgx/v5 `5.11.0`.

## Migration files

- Layout: `NNNNNN_snake_case.up.sql` + `.down.sql` pairs; six-digit, monotonic, each number used once.
- Immutable once applied/committed — corrections are new migrations, never edits.
- Directory: `server/migrations/`; `000001_baseline_schema` (owned by IMP-005) contains every table in `docs/06_data/data_model.md`.

## Change discipline

- Default to expand → migrate code → contract. Add columns/tables/indexes before code needs them; tighten constraints only after backfill.
- No long rewrites / `ACCESS EXCLUSIVE` on hot tables. Backfills: key-bounded batches, idempotent, restartable.
- Every production migration declares: affected tables/volume, expected lock/runtime, index strategy, backfill strategy, rollback/forward-fix.
- Schema migration ≠ content activation. Order: DB expand → backend deploy → verify → content activate → later contract cleanup.
- Down migrations: local/staging only when data-safe and tested. Never auto-run destructive DOWN in production. No client-provided DDL ever.

## Durable layer (only SQL owner)

- All gameplay SQL lives in `server/internal/durable/`; `sim` and `edge` never touch the database.
- Parameterized queries only; transactions across co-mutated values; consistent lock ordering (sorted IDs).
- Forbidden schema elements: `item_instances.durability`, `global_leader_lease` (Q4 fails on them).
- Every persistent mutation: stable operation identity, atomicity, optimistic revision/locking, deterministic restart recovery.
