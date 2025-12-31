-- Migration: Drop unused 'enabled' column from connection_settings table
-- The 'enabled' column was a duplicate of 'allow_connect' that was never properly
-- synchronized. The codebase now uses 'allow_connect' consistently.

-- First, sync any existing 'enabled' values to 'allow_connect' to preserve settings
-- In case there were any chats where enabled was set but allow_connect wasn't
UPDATE connection_settings
SET allow_connect = enabled
WHERE allow_connect = true AND enabled = false;

-- Drop the unused 'enabled' column
ALTER TABLE connection_settings DROP COLUMN IF EXISTS enabled;
