-- Per-chat settings for checking new members against the CAS spam database.
--
-- The settings live in the database rather than Redis because they are the
-- chat's moderation policy, not a cache entry: a cache flush must not silently
-- stop the check in a chat an admin turned it on for.
--
-- A missing row means disabled with the default action. Statements are
-- idempotent; the Go runner applies each file once inside a transaction and
-- records its sha256 in schema_migrations.

CREATE TABLE IF NOT EXISTS spam_db_settings (
    id BIGSERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    action VARCHAR(10) NOT NULL DEFAULT 'ban',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    -- The unique constraint already provides the index every lookup uses.
    UNIQUE(chat_id),
    CONSTRAINT chk_spam_db_action CHECK (action IN ('ban','mute','log'))
);

-- Add foreign key to chats table when available (self-managed)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE constraint_name = 'fk_spam_db_settings_chat') THEN
        ALTER TABLE spam_db_settings DROP CONSTRAINT fk_spam_db_settings_chat;
    END IF;

    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'chats') THEN
        ALTER TABLE spam_db_settings
        ADD CONSTRAINT fk_spam_db_settings_chat
        FOREIGN KEY (chat_id) REFERENCES chats(chat_id) ON DELETE CASCADE ON UPDATE CASCADE;
    END IF;
END $$;
