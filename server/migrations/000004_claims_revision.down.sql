-- 000004_claims_revision.down.sql
-- Local/staging rollback only (14-sql-migrations).

ALTER TABLE characters
  DROP COLUMN claims_revision;
