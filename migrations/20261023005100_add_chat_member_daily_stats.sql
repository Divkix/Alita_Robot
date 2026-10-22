-- Per-member message counters per chat and UTC day, behind the panel's top
-- members list.
--
-- day is the UTC date computed in Go (alita/utils/stats). Only real users are
-- counted: channel posts and anonymous admins have no user to rank. Like
-- chat_daily_stats, alita/db/chatstats only ever adds to messages, so two
-- replicas flushing the same day produce the sum. The primary key's
-- (chat_id, day) prefix serves the panel's per-chat range sum.
--
-- Retention: alita/modules/stats_retention.go deletes rows older than 90 days
-- on a daily tick. The file is applied once inside a transaction by the Go
-- runner and by scripts/migrate_psql.sh; the statements are idempotent.

CREATE TABLE IF NOT EXISTS chat_member_daily_stats (
    chat_id BIGINT NOT NULL,
    day TEXT NOT NULL,
    user_id BIGINT NOT NULL,
    messages BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (chat_id, day, user_id)
);

-- The retention sweep deletes on day alone, a prefix the primary key cannot
-- serve.
CREATE INDEX IF NOT EXISTS idx_chat_member_daily_stats_day ON chat_member_daily_stats (day);
