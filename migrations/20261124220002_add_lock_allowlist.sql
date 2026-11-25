-- Lock allowlist: per-chat exceptions the locks delete path honours for the
-- "url" and "invitelink" lock types. kind is 'host' for an exact host (a
-- subdomain of it counts) or 'invite' for a stable invite identity ('+hash',
-- 'joinchat/hash', 'id:<chat id>' or 'u:<username>').
--
-- A chat without rows has no exceptions, so nothing changes until an admin
-- runs /allowlist. Statements are idempotent; the Go runner and
-- scripts/migrate_psql.sh apply each file once inside a transaction and
-- record its sha256 in schema_migrations.

CREATE TABLE IF NOT EXISTS lock_allowlist (
    chat_id BIGINT NOT NULL,
    kind TEXT NOT NULL,
    value TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    PRIMARY KEY (chat_id, kind, value),
    CONSTRAINT chk_lock_allowlist_kind CHECK (kind IN ('host', 'invite')),
    CONSTRAINT chk_lock_allowlist_value CHECK (char_length(value) BETWEEN 1 AND 255)
);

CREATE INDEX IF NOT EXISTS idx_lock_allowlist_chat ON public.lock_allowlist(chat_id);

-- Add foreign key to chats table when available (self-managed)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE constraint_name = 'fk_lock_allowlist_chat') THEN
        ALTER TABLE lock_allowlist DROP CONSTRAINT fk_lock_allowlist_chat;
    END IF;

    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'chats') THEN
        ALTER TABLE lock_allowlist
        ADD CONSTRAINT fk_lock_allowlist_chat
        FOREIGN KEY (chat_id) REFERENCES chats(chat_id) ON DELETE CASCADE ON UPDATE CASCADE;
    END IF;
END $$;
