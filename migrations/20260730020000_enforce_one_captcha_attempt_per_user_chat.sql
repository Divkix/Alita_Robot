-- A user can have only one live captcha challenge in a chat.
WITH ranked_attempts AS (
    SELECT
        id,
        ROW_NUMBER() OVER (
            PARTITION BY user_id, chat_id
            ORDER BY updated_at DESC NULLS LAST, id DESC
        ) AS row_number
    FROM captcha_attempts
)
DELETE FROM captcha_attempts
USING ranked_attempts
WHERE captcha_attempts.id = ranked_attempts.id
  AND ranked_attempts.row_number > 1;

DROP INDEX IF EXISTS idx_captcha_user_chat;

CREATE UNIQUE INDEX uk_captcha_user_chat
ON captcha_attempts (user_id, chat_id);
