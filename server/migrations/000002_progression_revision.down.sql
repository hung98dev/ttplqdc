-- 000002_progression_revision.down.sql
-- Local/staging rollback only (14-sql-migrations).

ALTER TABLE characters
  DROP COLUMN progression_revision;
