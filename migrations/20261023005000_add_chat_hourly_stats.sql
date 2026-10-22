-- Per-chat message counters per UTC hour, behind the panel's hourly heatmap.
--
-- day is the UTC date and hour the UTC hour (0-23), both computed in Go
-- (alita/utils/stats), never by the database, so every replica and the SQLite
-- tests produce the same key. Like chat_daily_stats, alita/db/chatstats only
-- ever adds to messages, so two replicas flushing the same hour produce the
-- sum. A reader that shows a chat's local time shifts (day, hour) itself.
--
-- Retention: alita/modules/stats_retention.go deletes rows older than 90 days
-- (the panel's longest window) on a daily tick. The file is applied once
-- inside a transaction by the Go runner and by scripts/migrate_psql.sh; the
-- statements are idempotent.

CREATE TABLE IF NOT EXISTS chat_hourly_stats (
    chat_id BIGINT NOT NULL,
    day TEXT NOT NULL,
    hour SMALLINT NOT NULL,
    messages BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (chat_id, day, hour),
    CONSTRAINT chk_chat_hourly_stats_hour CHECK (hour >= 0 AND hour <= 23)
);

-- The retention sweep deletes on day alone, a prefix the primary key cannot
-- serve.
CREATE INDEX IF NOT EXISTS idx_chat_hourly_stats_day ON chat_hourly_stats (day);
