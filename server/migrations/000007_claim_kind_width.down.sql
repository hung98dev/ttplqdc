-- Local/staging rollback only (14-sql-migrations). Fails if any row
-- already stores a >16-char claim_kind — data cannot shrink, so such a
-- database must forward-fix instead.

ALTER TABLE reward_claims
  ALTER COLUMN claim_kind TYPE VARCHAR(16);
