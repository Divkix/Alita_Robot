-- Duration in seconds for timed antiflood and blacklist actions. 0 means
-- "not set" (existing rows or untimed actions); tban/tmute then keep their
-- one-hour fallback. Statements are idempotent; the migration runner applies
-- this file once inside a transaction and records its sha256.

ALTER TABLE antiflood_settings
ADD COLUMN IF NOT EXISTS flood_action_time INTEGER NOT NULL DEFAULT 0;

ALTER TABLE antiflood_settings DROP CONSTRAINT IF EXISTS chk_flood_action_time;

ALTER TABLE antiflood_settings
ADD CONSTRAINT chk_flood_action_time
CHECK (flood_action_time >= 0);

ALTER TABLE blacklists
ADD COLUMN IF NOT EXISTS action_time INTEGER NOT NULL DEFAULT 0;

ALTER TABLE blacklists DROP CONSTRAINT IF EXISTS chk_blacklist_action_time;

ALTER TABLE blacklists
ADD CONSTRAINT chk_blacklist_action_time
CHECK (action_time >= 0);
