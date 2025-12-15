-- Add last_activity column to track when groups were last active
ALTER TABLE chats
ADD COLUMN IF NOT EXISTS last_activity timestamp with time zone DEFAULT CURRENT_TIMESTAMP;

-- Update existing records to set last_activity based on updated_at
UPDATE chats
SET last_activity = COALESCE(updated_at, created_at, CURRENT_TIMESTAMP)
WHERE last_activity IS NULL;

-- Create index for efficient activity queries
CREATE INDEX IF NOT EXISTS idx_chats_last_activity ON chats(last_activity DESC);

-- Create index for activity status queries
CREATE INDEX IF NOT EXISTS idx_chats_activity_status ON chats(is_inactive, last_activity DESC) WHERE is_inactive = false;
