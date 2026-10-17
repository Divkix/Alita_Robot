-- How long a warn counts, in seconds; 0 (the default) means warns never
-- expire. A warn older than this stops counting towards the warn limit and
-- is no longer listed; the bot deletes it on the user's next /warn or
-- /rmwarn. Warns without a time (warn_entries.warned_at IS NULL) never expire.
-- Statements are idempotent; the Go runner applies each file once inside a
-- transaction and records its sha256 in schema_migrations.

ALTER TABLE warns_settings
ADD COLUMN IF NOT EXISTS warn_expiry INTEGER NOT NULL DEFAULT 0;

ALTER TABLE warns_settings DROP CONSTRAINT IF EXISTS chk_warn_expiry;

ALTER TABLE warns_settings
ADD CONSTRAINT chk_warn_expiry
CHECK (warn_expiry >= 0);
