---
name: database-change
description: Schema or migration work — expand/contract discipline, immutable files, rehearsal, rollback plan.
---

# Database Change

Canonical: `docs/06_data/migrations.md`, `database.md`, `physical_schema_contract.md`, `data_model.md`. Tool: golang-migrate `v4.20.1`; access: pgx/v5 in `durable/` only.

## Workflow

1. **Contract readiness.** Read the protected `docs/06_data/` owner + every consumer spec. If they do not already specify the model change, record the gap in `docs/10_implementation/known_blockers.md`, mark the task BLOCKED, and hand it to the `spec-owner` agent (spec-change PR + `policy-review`); never edit protected specs as an implementer. Runtime identities (`map_instance_id` etc.) are never durable foreign keys.
2. **Path.** `server/migrations/` only. Add the next monotonic `NNNNNN_name.up.sql` + `.down.sql`; never edit a committed migration.
3. **Expand-first.** Compatible expand → deploy code reading old+new → bounded idempotent backfill → switch → later contract migration. No `ACCESS EXCLUSIVE` rewrites on hot tables.
4. **Down file.** Data-safe only; production rollback is app-rollback/forward-fix or PITR, not destructive down.
5. **Code.** Update `durable/` queries; parameterized SQL; transactions + lock ordering.
6. **Tests.** Fresh-DB apply; previous→new schema upgrade on representative data; interrupted/retried migration safety; old persisted instances must not reroll/rebind.
7. **Rehearse.** Q5 (when verifier exists): apply → down → apply on PostgreSQL 18.6. Declare expected lock/runtime for large tables.

## Acceptance

- Forward migration verified on representative data; interrupted run is safe.
- Old data semantics preserved; no orphaned escrow/rewards/storage.
- The change packet under `docs/10_implementation/` records migration order vs content activation.
