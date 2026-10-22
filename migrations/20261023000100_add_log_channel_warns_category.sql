-- The warns log category records warnings admins give, remove and reset, and
-- the warn limit their warns reach. It is off by default, so existing log
-- channels get nothing new until an admin turns it on with /log warns.
--
-- Statements are idempotent; the Go runner applies each file once inside a
-- transaction and records its sha256 in schema_migrations.

ALTER TABLE log_channels
ADD COLUMN IF NOT EXISTS cat_warns BOOLEAN NOT NULL DEFAULT FALSE;
