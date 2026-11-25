-- Captcha now asks CaptchaRounds (2) correct questions per challenge, one at a
-- time. Each pass advances stage and replaces answer/message_id, so callbacks
-- from an earlier round fail their atomic stage fence.
--
-- max_attempts tightens from 1..10 to 1..3 and defaults to 2. Rows above the
-- new ceiling are clamped before the constraint is replaced; NOT VALID keeps
-- the migration fast on large tables (the Go runner applies it in one
-- transaction and records its sha256 in schema_migrations).

ALTER TABLE captcha_attempts ADD COLUMN stage integer NOT NULL DEFAULT 0;

ALTER TABLE join_request_captchas ADD COLUMN stage integer NOT NULL DEFAULT 0;

UPDATE captcha_settings SET max_attempts = 2 WHERE max_attempts > 3;

ALTER TABLE captcha_settings ALTER COLUMN max_attempts SET DEFAULT 2;

ALTER TABLE captcha_settings DROP CONSTRAINT IF EXISTS chk_captcha_max_attempts_range;

ALTER TABLE captcha_settings
    ADD CONSTRAINT chk_captcha_max_attempts_range
    CHECK (max_attempts BETWEEN 1 AND 3) NOT VALID;
