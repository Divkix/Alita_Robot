-- One Telegram member count per chat and UTC day, behind the panel's member
-- growth chart. Telegram gives bots no member history, so the series starts
-- on the day this ships.
--
-- day is the UTC date computed in Go (alita/utils/stats). The row is also the
-- day's claim: alita/membersnapshot inserts it with count NULL, and only the
-- replica whose insert won calls getChatMemberCount and fills count in, so two
-- replicas never both call Telegram for the same chat and day. A transient
-- failure deletes the claim so the next tick retries; a permanent one (the bot
-- left the chat) keeps it with count NULL, and readers skip NULL rows.
--
-- Retention: alita/modules/stats_retention.go deletes rows older than 400
-- days on a daily tick. The file is applied once inside a transaction by the
-- Go runner and by scripts/migrate_psql.sh; the statements are idempotent.

CREATE TABLE IF NOT EXISTS chat_member_counts (
    chat_id BIGINT NOT NULL,
    day TEXT NOT NULL,
    count BIGINT NULL,
    PRIMARY KEY (chat_id, day)
);

-- The due-chat lookup finds today's claims by (chat_id, day), the primary
-- key; the retention sweep deletes on day alone.
CREATE INDEX IF NOT EXISTS idx_chat_member_counts_day ON chat_member_counts (day);
