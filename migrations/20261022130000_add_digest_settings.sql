-- Per-chat settings for the weekly activity digest.
--
-- The settings live in the database rather than Redis because they are the
-- chat's preference, not a cache entry: a cache flush must not silently stop
-- a digest an admin turned on. last_sent_at is the exactly-once claim: a
-- replica sends a week's digest only when its conditional update wins, and a
-- failed send restores the previous value so the next tick retries.
--
-- destination is '' (follow the default: the log channel when the chat has
-- one, otherwise the group), 'log' or 'group'. Statements are idempotent; the
-- Go runner and scripts/migrate_psql.sh apply each file once inside a
-- transaction and record its sha256 in schema_migrations.

CREATE TABLE IF NOT EXISTS digest_settings (
    chat_id BIGINT PRIMARY KEY,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    destination TEXT NOT NULL DEFAULT '',
    last_sent_at TIMESTAMP WITH TIME ZONE NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT chk_digest_settings_destination CHECK (destination IN ('', 'log', 'group'))
);

-- Add foreign key to chats table when available (self-managed)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE constraint_name = 'fk_digest_settings_chat') THEN
        ALTER TABLE digest_settings DROP CONSTRAINT fk_digest_settings_chat;
    END IF;

    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'chats') THEN
        ALTER TABLE digest_settings
        ADD CONSTRAINT fk_digest_settings_chat
        FOREIGN KEY (chat_id) REFERENCES chats(chat_id) ON DELETE CASCADE ON UPDATE CASCADE;
    END IF;
END $$;
