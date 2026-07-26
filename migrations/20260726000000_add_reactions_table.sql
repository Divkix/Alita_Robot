-- Add reactions table for per-chat keyword-to-emoji reaction mappings.
-- Previously reactions were stored only in Redis and were wiped on every
-- startup FlushDB (ClearCacheOnStartup defaults to true), causing silent
-- loss of all configured reactions on restart.
CREATE TABLE IF NOT EXISTS reactions (
    id BIGSERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL,
    keyword TEXT NOT NULL,
    emoji TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(chat_id, keyword)
);

CREATE INDEX IF NOT EXISTS idx_reactions_chat_id ON reactions(chat_id);

-- Add foreign key to chats table for referential integrity when available.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE constraint_name = 'fk_reactions_chat') THEN
        ALTER TABLE reactions DROP CONSTRAINT fk_reactions_chat;
    END IF;

    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'chats') THEN
        ALTER TABLE reactions
        ADD CONSTRAINT fk_reactions_chat
        FOREIGN KEY (chat_id) REFERENCES chats(chat_id) ON DELETE CASCADE ON UPDATE CASCADE;
    END IF;
END $$;
