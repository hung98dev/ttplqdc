# ADR-0048: Character Row Update Timestamp
status: ACCEPTED

> **AMENDMENT NOTICE (2026-09-24)**: No migration had been merged when implementation was reset to docs-only. `characters.updated_at` is included in the `000001_baseline_schema` created by IMP-005; the `000003` migration and existing-row initialization below no longer apply. The update-scope rule (row updates set `updated_at`; no trigger) still applies.

## Context

`data_model.md` includes character timestamps, but the merged baseline migration has only `created_at`. A physical-schema contract correction must not rewrite an applied migration.

## Decision

- Add `characters.updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()` in the next migration, `000003`.
- Existing rows receive the migration transaction time; historical edits cannot be reconstructed.
- Every committed update to the `characters` row explicitly sets `updated_at` to the server transaction time. Updates only to child/projection rows leave it unchanged. No database trigger is added.
- `current_exp` remains `INTEGER` as decided in ADR-0031; the stale `BIGINT` declaration is corrected without a type migration.

## Consequences

- `../06_data/data_model.md` names the timestamp and its update scope.
- `../06_data/physical_schema_contract.md` specifies `INTEGER current_exp` and both character timestamps.
- `../06_data/migrations.md` records the additive migration and initialization of existing rows.
- The implementation owner must add `server/migrations/000003_character_updated_at.up.sql` and its down pair, update the schema snapshot, and pass migration checks before character-row writes depend on the column.
