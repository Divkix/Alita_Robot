-- Per-chat daily activity counters behind the weekly digest and /digest now.
--
-- day is the UTC date computed in Go (YYYY-MM-DD). No database date function
-- is involved, so every replica and the SQLite tests produce the same key.
-- The primary key is the whole table: one row per chat per day, and
-- alita/db/chatstats only ever adds to the three counters, so two replicas
-- flushing the same day produce the sum.
--
-- Retention: alita/modules/digest_retention.go deletes rows older than 400
-- days on a weekly tick. The file is applied once inside a transaction by the
-- Go runner and by scripts/migrate_psql.sh; the statement is idempotent.

CREATE TABLE IF NOT EXISTS chat_daily_stats (
    chat_id BIGINT NOT NULL,
    day TEXT NOT NULL,
    messages BIGINT NOT NULL DEFAULT 0,
    joins BIGINT NOT NULL DEFAULT 0,
    leaves BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (chat_id, day)
);

-- The retention sweep deletes on day alone, a prefix the (chat_id, day)
-- primary key cannot serve. Keeping it in the same migration means an
-- AutoMigrated database has the index before the first sweep runs.
CREATE INDEX IF NOT EXISTS idx_chat_daily_stats_day ON chat_daily_stats (day);
