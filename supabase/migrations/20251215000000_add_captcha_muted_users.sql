-- Add captcha_muted_users table for tracking users to auto-unmute
CREATE TABLE IF NOT EXISTS captcha_muted_users (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    chat_id BIGINT NOT NULL,
    unmute_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create indexes for efficient queries
CREATE INDEX IF NOT EXISTS idx_captcha_muted_user_chat ON captcha_muted_users(user_id, chat_id);
CREATE INDEX IF NOT EXISTS idx_captcha_unmute_at ON captcha_muted_users(unmute_at);

-- Add foreign key constraints (optional, for data integrity)
-- These reference the users and chats tables if they exist
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'users') THEN
        ALTER TABLE captcha_muted_users
        ADD CONSTRAINT fk_captcha_muted_user
        FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE;
    END IF;
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'chats') THEN
        ALTER TABLE captcha_muted_users
        ADD CONSTRAINT fk_captcha_muted_chat
        FOREIGN KEY (chat_id) REFERENCES chats(chat_id) ON DELETE CASCADE;
    END IF;
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;
