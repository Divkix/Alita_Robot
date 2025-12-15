-- Create captcha_settings table for chat-specific captcha configuration
CREATE TABLE IF NOT EXISTS captcha_settings (
    id SERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL UNIQUE,
    enabled BOOLEAN DEFAULT FALSE,
    captcha_mode VARCHAR(10) DEFAULT 'math' CHECK (captcha_mode IN ('math', 'text')),
    timeout INTEGER DEFAULT 2 CHECK (timeout > 0 AND timeout <= 10),
    failure_action VARCHAR(10) DEFAULT 'kick' CHECK (failure_action IN ('kick', 'ban', 'mute')),
    max_attempts INTEGER DEFAULT 3 CHECK (max_attempts > 0 AND max_attempts <= 10),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_captcha_settings_chat FOREIGN KEY (chat_id) REFERENCES chats(chat_id) ON DELETE CASCADE
);

-- Create unique index on chat_id for fast lookups
CREATE UNIQUE INDEX IF NOT EXISTS uk_captcha_settings_chat_id ON captcha_settings(chat_id);

-- Create captcha_attempts table for tracking active captcha attempts
CREATE TABLE IF NOT EXISTS captcha_attempts (
    id SERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    chat_id BIGINT NOT NULL,
    answer VARCHAR(255) NOT NULL,
    attempts INTEGER DEFAULT 0,
    message_id BIGINT,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_captcha_attempts_user FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    CONSTRAINT fk_captcha_attempts_chat FOREIGN KEY (chat_id) REFERENCES chats(chat_id) ON DELETE CASCADE
);

-- Create composite index for fast lookups by user and chat
CREATE INDEX IF NOT EXISTS idx_captcha_user_chat ON captcha_attempts(user_id, chat_id);

-- Create index for expired attempts cleanup
CREATE INDEX IF NOT EXISTS idx_captcha_expires_at ON captcha_attempts(expires_at);

-- Create function to automatically clean up expired captcha attempts
CREATE OR REPLACE FUNCTION cleanup_expired_captcha_attempts()
RETURNS INTEGER AS $$
DECLARE
    deleted_count INTEGER;
BEGIN
    DELETE FROM captcha_attempts
    WHERE expires_at < CURRENT_TIMESTAMP;
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RETURN deleted_count;
END;
$$ LANGUAGE plpgsql;

-- Create trigger to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_captcha_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Add triggers to update updated_at on both tables
CREATE TRIGGER update_captcha_settings_updated_at
    BEFORE UPDATE ON captcha_settings
    FOR EACH ROW
    EXECUTE FUNCTION update_captcha_updated_at();

CREATE TRIGGER update_captcha_attempts_updated_at
    BEFORE UPDATE ON captcha_attempts
    FOR EACH ROW
    EXECUTE FUNCTION update_captcha_updated_at();
