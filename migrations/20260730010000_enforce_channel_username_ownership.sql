-- Telegram usernames are case-insensitive and may move between channels.
UPDATE channels
SET username = COALESCE(NULLIF(LOWER(LTRIM(BTRIM(username), '@')), ''), '');

WITH ranked_channel_usernames AS (
    SELECT
        id,
        ROW_NUMBER() OVER (
            PARTITION BY username
            ORDER BY updated_at DESC NULLS LAST, id DESC
        ) AS row_number
    FROM channels
    WHERE username IS NOT NULL
      AND username <> ''
)
UPDATE channels
SET username = ''
FROM ranked_channel_usernames
WHERE channels.id = ranked_channel_usernames.id
  AND ranked_channel_usernames.row_number > 1;

DROP INDEX IF EXISTS idx_channels_username;

CREATE UNIQUE INDEX idx_channels_username
ON channels (LOWER(username))
WHERE username <> '';
