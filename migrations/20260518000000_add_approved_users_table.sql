-- Add approved_users table for per-chat user whitelist exempting users from anti-spam.
CREATE TABLE IF NOT EXISTS approved_users (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    chat_id BIGINT NOT NULL,
    reason TEXT DEFAULT '',
    approved_by BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(user_id, chat_id)
);

CREATE INDEX IF NOT EXISTS idx_approved_users_chat_id ON approved_users(chat_id);

-- Add foreign key to chats table (self-managed) for referential integrity when available
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE constraint_name = 'fk_approved_users_chat') THEN
        ALTER TABLE approved_users DROP CONSTRAINT fk_approved_users_chat;
    END IF;

    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'chats') THEN
        ALTER TABLE approved_users
        ADD CONSTRAINT fk_approved_users_chat
        FOREIGN KEY (chat_id) REFERENCES chats(chat_id) ON DELETE CASCADE ON UPDATE CASCADE;
    END IF;
END $$;
