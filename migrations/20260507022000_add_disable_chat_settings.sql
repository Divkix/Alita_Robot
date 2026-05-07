-- Add chat-level disable settings table for disabled command deletion behavior.
CREATE TABLE IF NOT EXISTS disable_chat_settings (
    id BIGSERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL UNIQUE,
    delete_commands BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

ALTER TABLE disable_chat_settings
ADD CONSTRAINT fk_disable_chat_settings_chat
FOREIGN KEY (chat_id) REFERENCES chats(chat_id) ON DELETE CASCADE ON UPDATE CASCADE;
