-- One row per warn, so /warns can show who issued each warn and when.
--
-- warns_users keeps num_warns and its warns jsonb array of reasons; the bot
-- still writes both, so an older release reads valid data after a rollback.
-- There is deliberately no foreign key to warns_users: the bot deletes a
-- user's entries in the same transaction as their warns_users row. The keys
-- to chats and users match warns_users, so deleting a chat or user drops its
-- entries with its warns.
--
-- warned_by = 0 and warned_at IS NULL mean "unknown" (copied from warns below,
-- or restored from a backup that predates this table).
-- Statements are idempotent; the Go runner applies each file once inside a
-- transaction and records its sha256 in schema_migrations.

CREATE TABLE IF NOT EXISTS warn_entries (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL CONSTRAINT fk_warn_entries_user
        REFERENCES users(user_id) ON DELETE CASCADE ON UPDATE CASCADE,
    chat_id BIGINT NOT NULL CONSTRAINT fk_warn_entries_chat
        REFERENCES chats(chat_id) ON DELETE CASCADE ON UPDATE CASCADE,
    reason TEXT NOT NULL DEFAULT '',
    warned_by BIGINT NOT NULL DEFAULT 0,
    anonymous BOOLEAN NOT NULL DEFAULT FALSE,
    warned_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_warn_entries_chat_user
ON warn_entries (chat_id, user_id, id);

-- Backs fk_warn_entries_user: deleting or renumbering a users row looks up
-- its entries by user_id alone, which the (chat_id, ...) index cannot serve.
CREATE INDEX IF NOT EXISTS idx_warn_entries_user_id
ON warn_entries (user_id);

-- Copy existing reasons in order. Users that already have entries are
-- skipped, so a second run copies nothing. A JSON null in the array comes out
-- as SQL NULL; it is stored as '' (what the bot reads it as) rather than
-- dropped, so each user keeps one entry per reason.
INSERT INTO warn_entries (user_id, chat_id, reason)
SELECT w.user_id, w.chat_id, COALESCE(r.value, '')
FROM warns_users w
CROSS JOIN LATERAL jsonb_array_elements_text(w.warns) WITH ORDINALITY AS r(value, ord)
WHERE jsonb_typeof(w.warns) = 'array'
  AND NOT EXISTS (
      SELECT 1 FROM warn_entries e
      WHERE e.chat_id = w.chat_id AND e.user_id = w.user_id
  )
ORDER BY w.id, r.ord;
