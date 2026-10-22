-- The shared reputation network is a third database the spam database join
-- check can ask. It is the bot's own aggregate over human moderation actions,
-- not a third-party service, so nothing leaves the bot for it.
--
-- It is off by default: a chat opts in with /spamdb db shared on, and the
-- operator keeps ENABLE_REPUTATION as the kill switch.
--
-- The widened check constraint keeps at least one database selected across all
-- three sources, so a chat that turned the check on always asks something, even
-- when two admins switch different databases off at the same time.
--
-- Statements are idempotent; the Go runner applies each file once inside a
-- transaction and records its sha256 in schema_migrations.

ALTER TABLE spam_db_settings
ADD COLUMN IF NOT EXISTS check_shared BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE spam_db_settings DROP CONSTRAINT IF EXISTS chk_spam_db_databases;

ALTER TABLE spam_db_settings
ADD CONSTRAINT chk_spam_db_databases
CHECK (check_cas OR check_lols OR check_shared);
