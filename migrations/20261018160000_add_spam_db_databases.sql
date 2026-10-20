-- Which spam databases a chat's join check asks. CAS stays the default;
-- lols.bot lists accounts more aggressively, so a chat opts into it.
--
-- The check constraint keeps at least one database selected, so a chat that
-- turned the check on always asks something, even when two admins switch
-- databases off at the same time.
--
-- Statements are idempotent; the Go runner applies each file once inside a
-- transaction and records its sha256 in schema_migrations.

ALTER TABLE spam_db_settings
ADD COLUMN IF NOT EXISTS check_cas BOOLEAN NOT NULL DEFAULT TRUE;

ALTER TABLE spam_db_settings
ADD COLUMN IF NOT EXISTS check_lols BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE spam_db_settings DROP CONSTRAINT IF EXISTS chk_spam_db_databases;

ALTER TABLE spam_db_settings
ADD CONSTRAINT chk_spam_db_databases
CHECK (check_cas OR check_lols);
