-- 000002_progression_revision.up.sql
-- characters.progression_revision: aggregate revision carried by
-- S2C_PROGRESSION_STATE (data_model.md § Character;
-- physical_schema_contract.md §3). Bumped by exactly 1 under the
-- character lock inside every committed progression-mutation
-- transaction (IMP-011 owns the bump; this pair only adds the column).

ALTER TABLE characters
  ADD COLUMN progression_revision BIGINT NOT NULL DEFAULT 0;
