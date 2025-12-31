-- Add channel_name and username columns to channels table
-- These columns were missing, causing UpdateChannel to silently discard data

ALTER TABLE channels ADD COLUMN IF NOT EXISTS channel_name TEXT;
ALTER TABLE channels ADD COLUMN IF NOT EXISTS username TEXT;

-- Add index for username lookups
CREATE INDEX IF NOT EXISTS idx_channels_username ON channels (username) WHERE username IS NOT NULL;
