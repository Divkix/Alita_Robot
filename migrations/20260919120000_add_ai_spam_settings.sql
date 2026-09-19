-- Per-chat opt-in switch for the AI spam filter (TypeSafe Jev).
--
-- The flag lives in the database rather than Redis because it is the chat's
-- moderation policy, not a cache entry: a cache flush must not silently stop
-- deleting spam in a chat an admin turned the filter on for.
--
-- A missing row means disabled, so enabling is the only path that writes.
-- Statement is idempotent; the Go runner applies each file once inside a
-- transaction and records its sha256 in schema_migrations.

CREATE TABLE IF NOT EXISTS ai_spam_settings (
    id BIGSERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    -- The unique constraint already provides the index every lookup uses;
    -- a second index on chat_id would only cost write time.
    UNIQUE(chat_id)
);

-- Add foreign key to chats table when available (self-managed)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE constraint_name = 'fk_ai_spam_settings_chat') THEN
        ALTER TABLE ai_spam_settings DROP CONSTRAINT fk_ai_spam_settings_chat;
    END IF;

    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'chats') THEN
        ALTER TABLE ai_spam_settings
        ADD CONSTRAINT fk_ai_spam_settings_chat
        FOREIGN KEY (chat_id) REFERENCES chats(chat_id) ON DELETE CASCADE ON UPDATE CASCADE;
    END IF;
END $$;
