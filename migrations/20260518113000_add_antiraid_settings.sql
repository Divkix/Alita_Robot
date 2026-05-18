-- Add antiraid_settings table for per-chat join-raid detection and protection.
CREATE TABLE IF NOT EXISTS antiraid_settings (
    id BIGSERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL,
    raid_time INT NOT NULL DEFAULT 21600,
    raid_action_time INT NOT NULL DEFAULT 3600,
    auto_antiraid_threshold INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(chat_id)
);

CREATE INDEX IF NOT EXISTS idx_antiraid_settings_chat_id ON antiraid_settings(chat_id);

-- Add foreign key to chats table when available (self-managed)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE constraint_name = 'fk_antiraid_settings_chat') THEN
        ALTER TABLE antiraid_settings DROP CONSTRAINT fk_antiraid_settings_chat;
    END IF;

    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'chats') THEN
        ALTER TABLE antiraid_settings
        ADD CONSTRAINT fk_antiraid_settings_chat
        FOREIGN KEY (chat_id) REFERENCES chats(chat_id) ON DELETE CASCADE ON UPDATE CASCADE;
    END IF;
END $$;
