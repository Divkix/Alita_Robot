-- Add last_activity column to track when users were last active
ALTER TABLE users
ADD COLUMN IF NOT EXISTS last_activity timestamp with time zone DEFAULT CURRENT_TIMESTAMP;

-- Update existing records to set last_activity based on updated_at
UPDATE users
SET last_activity = COALESCE(updated_at, created_at, CURRENT_TIMESTAMP)
WHERE last_activity IS NULL;

-- Create index for efficient activity queries
CREATE INDEX IF NOT EXISTS idx_users_last_activity ON users(last_activity DESC);

-- Add comment explaining the column
COMMENT ON COLUMN users.last_activity IS 'Timestamp of user''s last interaction with the bot';
