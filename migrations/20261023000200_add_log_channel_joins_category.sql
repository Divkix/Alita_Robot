-- The joins log category records each member who passes the captcha. It is
-- off by default, as it can be one entry per join, so existing log channels
-- get nothing new until an admin turns it on with /log joins.
--
-- Statements are idempotent; the Go runner applies each file once inside a
-- transaction and records its sha256 in schema_migrations.

ALTER TABLE log_channels
ADD COLUMN IF NOT EXISTS cat_joins BOOLEAN NOT NULL DEFAULT FALSE;
