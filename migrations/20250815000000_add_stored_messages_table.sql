-- Add StoredMessages table for pre-captcha message storage
CREATE TABLE IF NOT EXISTS stored_messages (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    chat_id BIGINT NOT NULL,
    message_type INTEGER NOT NULL DEFAULT 1,
    content TEXT,
    file_id TEXT,
    caption TEXT,
    attempt_id BIGINT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create index for user+chat lookups
CREATE INDEX IF NOT EXISTS idx_stored_user_chat ON stored_messages(user_id, chat_id);

-- Create index for attempt lookups
CREATE INDEX IF NOT EXISTS idx_stored_attempt ON stored_messages(attempt_id);

-- Add foreign key constraint to captcha_attempts
ALTER TABLE stored_messages
ADD CONSTRAINT fk_stored_messages_attempt
FOREIGN KEY (attempt_id) REFERENCES captcha_attempts(id) ON DELETE CASCADE;

-- Add comment
COMMENT ON TABLE stored_messages IS 'Stores messages sent by users before completing captcha verification';
