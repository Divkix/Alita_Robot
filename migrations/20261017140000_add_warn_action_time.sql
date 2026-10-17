-- Duration in seconds for the tban/tmute warn modes. The warn_mode CHECK has
-- always allowed them, but nothing stored how long the punishment lasts.
--
-- 0 means "not set" (rows from before this column, or ban/kick/mute); the bot
-- falls back to one hour for tban/tmute rather than punishing forever.
-- Statements are idempotent; the Go runner applies each file once inside a
-- transaction and records its sha256 in schema_migrations.

ALTER TABLE warns_settings
ADD COLUMN IF NOT EXISTS warn_action_time INTEGER NOT NULL DEFAULT 0;

ALTER TABLE warns_settings DROP CONSTRAINT IF EXISTS chk_warn_action_time;

ALTER TABLE warns_settings
ADD CONSTRAINT chk_warn_action_time
CHECK (warn_action_time >= 0);
