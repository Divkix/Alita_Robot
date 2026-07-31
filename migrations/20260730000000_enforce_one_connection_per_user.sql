-- Each user has at most one saved connection. Existing repository reads use
-- GORM's First ordering, so retain the lowest-id row they already observed.
WITH ranked_connections AS (
    SELECT
        id,
        ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY id) AS row_number
    FROM connection
)
DELETE FROM connection
USING ranked_connections
WHERE connection.id = ranked_connections.id
  AND ranked_connections.row_number > 1;

ALTER TABLE connection
DROP CONSTRAINT IF EXISTS uk_connection_user_chat;

DROP INDEX IF EXISTS idx_connection_user_chat;

ALTER TABLE connection
ADD CONSTRAINT uk_connection_user_id UNIQUE (user_id);

-- Duplicate mute schedules can make separate workers unmute the same user.
-- Retain the latest deadline so cleanup cannot unmute a newer mute early.
WITH ranked_captcha_mutes AS (
    SELECT
        id,
        ROW_NUMBER() OVER (
            PARTITION BY user_id, chat_id
            ORDER BY unmute_at DESC, id DESC
        ) AS row_number
    FROM captcha_muted_users
)
DELETE FROM captcha_muted_users
USING ranked_captcha_mutes
WHERE captcha_muted_users.id = ranked_captcha_mutes.id
  AND ranked_captcha_mutes.row_number > 1;

DROP INDEX IF EXISTS idx_captcha_muted_user_chat;

ALTER TABLE captcha_muted_users
ADD CONSTRAINT uk_captcha_muted_user_chat UNIQUE (user_id, chat_id);
