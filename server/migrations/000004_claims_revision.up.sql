-- 000004_claims_revision.up.sql
-- characters.claims_revision: aggregate revision carried by
-- S2C_REWARD_CLAIM_DELTA (441); bump +1 under the character lock in
-- every committed claims-mutation transaction (data_model.md
-- § Character; physical_schema_contract.md §3). 000003 retired per
-- ADR-0048 amendment.

ALTER TABLE characters
  ADD COLUMN claims_revision BIGINT NOT NULL DEFAULT 0;
