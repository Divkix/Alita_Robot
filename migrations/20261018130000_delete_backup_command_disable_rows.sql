-- /export and /import are admin commands and are no longer disableable, but
-- chats that ran `/disable export` or `/disable import` on an older release
-- still have rows for them. /disabled would keep listing those rows and
-- /enable refuses commands that are not disableable, so nothing could clear
-- them.
--
-- Idempotent; the Go runner applies each file once inside a transaction and
-- records its sha256 in schema_migrations.

DELETE FROM disable WHERE lower(command) IN ('export', 'import');
