-- =====================================================
-- Supabase Migration: Add Foreign Key Relations
-- =====================================================
-- Migration Name: add_foreign_key_relations
-- Description: Adds all foreign key constraints to establish proper relationships
-- =====================================================

BEGIN;

-- =====================================================
-- STEP 1: Clean Up Orphaned Records (IMPORTANT!)
-- =====================================================
-- This section removes any orphaned records that would prevent foreign keys from being created
-- Comment out these DELETE statements if you want to manually review orphaned data first

-- Remove admin records with non-existent chat_ids
DELETE FROM admin WHERE chat_id NOT IN (SELECT chat_id FROM chats);

-- Remove antiflood_settings records with non-existent chat_ids
DELETE FROM antiflood_settings WHERE chat_id NOT IN (SELECT chat_id FROM chats);

-- Remove blacklists records with non-existent chat_ids
DELETE FROM blacklists WHERE chat_id NOT IN (SELECT chat_id FROM chats);

-- Remove channels records with non-existent chat_ids
DELETE FROM channels WHERE chat_id NOT IN (SELECT chat_id FROM chats);

-- Remove channels records with non-existent channel_ids
UPDATE channels SET channel_id = NULL WHERE channel_id IS NOT NULL AND channel_id NOT IN (SELECT chat_id FROM chats);

-- Remove connection_settings records with non-existent chat_ids
DELETE FROM connection_settings WHERE chat_id NOT IN (SELECT chat_id FROM chats);

-- Remove disable records with non-existent chat_ids
DELETE FROM disable WHERE chat_id NOT IN (SELECT chat_id FROM chats);

-- Remove filters records with non-existent chat_ids
DELETE FROM filters WHERE chat_id NOT IN (SELECT chat_id FROM chats);

-- Remove greetings records with non-existent chat_ids
DELETE FROM greetings WHERE chat_id NOT IN (SELECT chat_id FROM chats);

-- Remove locks records with non-existent chat_ids
DELETE FROM locks WHERE chat_id NOT IN (SELECT chat_id FROM chats);

-- Remove notes records with non-existent chat_ids
DELETE FROM notes WHERE chat_id NOT IN (SELECT chat_id FROM chats);

-- Remove notes_settings records with non-existent chat_ids
DELETE FROM notes_settings WHERE chat_id NOT IN (SELECT chat_id FROM chats);

-- Remove pins records with non-existent chat_ids
DELETE FROM pins WHERE chat_id NOT IN (SELECT chat_id FROM chats);

-- Remove report_chat_settings records with non-existent chat_ids
DELETE FROM report_chat_settings WHERE chat_id NOT IN (SELECT chat_id FROM chats);

-- Remove rules records with non-existent chat_ids
DELETE FROM rules WHERE chat_id NOT IN (SELECT chat_id FROM chats);

-- Remove warns_settings records with non-existent chat_ids
DELETE FROM warns_settings WHERE chat_id NOT IN (SELECT chat_id FROM chats);

-- Remove devs records with non-existent user_ids
DELETE FROM devs WHERE user_id NOT IN (SELECT user_id FROM users);

-- Remove report_user_settings records with non-existent user_ids
DELETE FROM report_user_settings WHERE user_id NOT IN (SELECT user_id FROM users);

-- Remove chat_users records with non-existent chat_ids or user_ids
DELETE FROM chat_users WHERE chat_id NOT IN (SELECT chat_id FROM chats) OR user_id NOT IN (SELECT user_id FROM users);

-- Remove connection records with non-existent chat_ids or user_ids
DELETE FROM connection WHERE chat_id NOT IN (SELECT chat_id FROM chats) OR user_id NOT IN (SELECT user_id FROM users);

-- Remove warns_users records with non-existent chat_ids or user_ids
DELETE FROM warns_users WHERE chat_id NOT IN (SELECT chat_id FROM chats) OR user_id NOT IN (SELECT user_id FROM users);

-- =====================================================
-- STEP 2: Add Unique Constraints on Natural Keys
-- =====================================================

-- Add unique constraint on chats.chat_id
ALTER TABLE chats
ADD CONSTRAINT uk_chats_chat_id UNIQUE (chat_id);

-- Add unique constraint on users.user_id
ALTER TABLE users
ADD CONSTRAINT uk_users_user_id UNIQUE (user_id);

-- =====================================================
-- STEP 3: Add Foreign Key Constraints
-- =====================================================

-- -----------------------------------------------------
-- CHAT-RELATED FOREIGN KEYS
-- -----------------------------------------------------

-- Admin to Chats
ALTER TABLE admin
ADD CONSTRAINT fk_admin_chat
FOREIGN KEY (chat_id) REFERENCES chats(chat_id)
ON DELETE CASCADE ON UPDATE CASCADE;

-- Antiflood Settings to Chats
ALTER TABLE antiflood_settings
ADD CONSTRAINT fk_antiflood_chat
FOREIGN KEY (chat_id) REFERENCES chats(chat_id)
ON DELETE CASCADE ON UPDATE CASCADE;

-- Blacklists to Chats
ALTER TABLE blacklists
ADD CONSTRAINT fk_blacklists_chat
FOREIGN KEY (chat_id) REFERENCES chats(chat_id)
ON DELETE CASCADE ON UPDATE CASCADE;

-- Channels to Chats (main chat reference)
ALTER TABLE channels
ADD CONSTRAINT fk_channels_chat
FOREIGN KEY (chat_id) REFERENCES chats(chat_id)
ON DELETE CASCADE ON UPDATE CASCADE;

-- Channels to Chats (linked channel reference - optional)
ALTER TABLE channels
ADD CONSTRAINT fk_channels_channel
FOREIGN KEY (channel_id) REFERENCES chats(chat_id)
ON DELETE SET NULL ON UPDATE CASCADE;

-- Connection Settings to Chats
ALTER TABLE connection_settings
ADD CONSTRAINT fk_connection_settings_chat
FOREIGN KEY (chat_id) REFERENCES chats(chat_id)
ON DELETE CASCADE ON UPDATE CASCADE;

-- Disable to Chats
ALTER TABLE disable
ADD CONSTRAINT fk_disable_chat
FOREIGN KEY (chat_id) REFERENCES chats(chat_id)
ON DELETE CASCADE ON UPDATE CASCADE;

-- Filters to Chats
ALTER TABLE filters
ADD CONSTRAINT fk_filters_chat
FOREIGN KEY (chat_id) REFERENCES chats(chat_id)
ON DELETE CASCADE ON UPDATE CASCADE;

-- Greetings to Chats
ALTER TABLE greetings
ADD CONSTRAINT fk_greetings_chat
FOREIGN KEY (chat_id) REFERENCES chats(chat_id)
ON DELETE CASCADE ON UPDATE CASCADE;

-- Locks to Chats
ALTER TABLE locks
ADD CONSTRAINT fk_locks_chat
FOREIGN KEY (chat_id) REFERENCES chats(chat_id)
ON DELETE CASCADE ON UPDATE CASCADE;

-- Notes to Chats
ALTER TABLE notes
ADD CONSTRAINT fk_notes_chat
FOREIGN KEY (chat_id) REFERENCES chats(chat_id)
ON DELETE CASCADE ON UPDATE CASCADE;

-- Notes Settings to Chats
ALTER TABLE notes_settings
ADD CONSTRAINT fk_notes_settings_chat
FOREIGN KEY (chat_id) REFERENCES chats(chat_id)
ON DELETE CASCADE ON UPDATE CASCADE;

-- Pins to Chats
ALTER TABLE pins
ADD CONSTRAINT fk_pins_chat
FOREIGN KEY (chat_id) REFERENCES chats(chat_id)
ON DELETE CASCADE ON UPDATE CASCADE;

-- Report Chat Settings to Chats
ALTER TABLE report_chat_settings
ADD CONSTRAINT fk_report_chat_settings_chat
FOREIGN KEY (chat_id) REFERENCES chats(chat_id)
ON DELETE CASCADE ON UPDATE CASCADE;

-- Rules to Chats
ALTER TABLE rules
ADD CONSTRAINT fk_rules_chat
FOREIGN KEY (chat_id) REFERENCES chats(chat_id)
ON DELETE CASCADE ON UPDATE CASCADE;

-- Warns Settings to Chats
ALTER TABLE warns_settings
ADD CONSTRAINT fk_warns_settings_chat
FOREIGN KEY (chat_id) REFERENCES chats(chat_id)
ON DELETE CASCADE ON UPDATE CASCADE;

-- -----------------------------------------------------
-- USER-RELATED FOREIGN KEYS
-- -----------------------------------------------------

-- Devs to Users
ALTER TABLE devs
ADD CONSTRAINT fk_devs_user
FOREIGN KEY (user_id) REFERENCES users(user_id)
ON DELETE CASCADE ON UPDATE CASCADE;

-- Report User Settings to Users
ALTER TABLE report_user_settings
ADD CONSTRAINT fk_report_user_settings_user
FOREIGN KEY (user_id) REFERENCES users(user_id)
ON DELETE CASCADE ON UPDATE CASCADE;

-- -----------------------------------------------------
-- MANY-TO-MANY RELATIONSHIP TABLES
-- -----------------------------------------------------

-- Chat Users Junction Table
ALTER TABLE chat_users
ADD CONSTRAINT fk_chat_users_chat
FOREIGN KEY (chat_id) REFERENCES chats(chat_id)
ON DELETE CASCADE ON UPDATE CASCADE;

ALTER TABLE chat_users
ADD CONSTRAINT fk_chat_users_user
FOREIGN KEY (user_id) REFERENCES users(user_id)
ON DELETE CASCADE ON UPDATE CASCADE;

-- Connection Table
ALTER TABLE connection
ADD CONSTRAINT fk_connection_user
FOREIGN KEY (user_id) REFERENCES users(user_id)
ON DELETE CASCADE ON UPDATE CASCADE;

ALTER TABLE connection
ADD CONSTRAINT fk_connection_chat
FOREIGN KEY (chat_id) REFERENCES chats(chat_id)
ON DELETE CASCADE ON UPDATE CASCADE;

-- Warns Users
ALTER TABLE warns_users
ADD CONSTRAINT fk_warns_users_user
FOREIGN KEY (user_id) REFERENCES users(user_id)
ON DELETE CASCADE ON UPDATE CASCADE;

ALTER TABLE warns_users
ADD CONSTRAINT fk_warns_users_chat
FOREIGN KEY (chat_id) REFERENCES chats(chat_id)
ON DELETE CASCADE ON UPDATE CASCADE;

-- =====================================================
-- STEP 4: Add Additional Unique Constraints
-- =====================================================

-- Ensure unique combinations
ALTER TABLE connection
ADD CONSTRAINT uk_connection_user_chat UNIQUE (user_id, chat_id);

ALTER TABLE warns_users
ADD CONSTRAINT uk_warns_users_user_chat UNIQUE (user_id, chat_id);

ALTER TABLE blacklists
ADD CONSTRAINT uk_blacklists_chat_word UNIQUE (chat_id, word);

ALTER TABLE disable
ADD CONSTRAINT uk_disable_chat_command UNIQUE (chat_id, command);

ALTER TABLE filters
ADD CONSTRAINT uk_filters_chat_keyword UNIQUE (chat_id, keyword);

ALTER TABLE locks
ADD CONSTRAINT uk_locks_chat_type UNIQUE (chat_id, lock_type);

ALTER TABLE notes
ADD CONSTRAINT uk_notes_chat_name UNIQUE (chat_id, note_name);

-- =====================================================
-- STEP 5: Add Check Constraints
-- =====================================================

-- Ensure positive numbers
ALTER TABLE warns_settings
ADD CONSTRAINT chk_warns_settings_limit CHECK (warn_limit > 0);

ALTER TABLE antiflood_settings
ADD CONSTRAINT chk_antiflood_limit CHECK (flood_limit > 0);

ALTER TABLE warns_users
ADD CONSTRAINT chk_warns_users_num CHECK (num_warns >= 0);

-- Ensure valid enum values
ALTER TABLE antiflood_settings
ADD CONSTRAINT chk_antiflood_action
CHECK (action IN ('mute', 'ban', 'kick', 'warn', 'tban', 'tmute'));

ALTER TABLE antiflood_settings
ADD CONSTRAINT chk_antiflood_mode
CHECK (mode IN ('mute', 'ban', 'kick', 'warn', 'tban', 'tmute'));

ALTER TABLE blacklists
ADD CONSTRAINT chk_blacklists_action
CHECK (action IN ('warn', 'mute', 'ban', 'kick', 'tban', 'tmute', 'delete'));

ALTER TABLE warns_settings
ADD CONSTRAINT chk_warns_mode
CHECK (warn_mode IS NULL OR warn_mode IN ('ban', 'kick', 'mute', 'tban', 'tmute'));

-- =====================================================
-- STEP 6: Create Additional Indexes for Performance
-- =====================================================

CREATE INDEX IF NOT EXISTS idx_connection_user_id ON connection(user_id);
CREATE INDEX IF NOT EXISTS idx_connection_chat_id ON connection(chat_id);
CREATE INDEX IF NOT EXISTS idx_warns_users_user_id ON warns_users(user_id);
CREATE INDEX IF NOT EXISTS idx_warns_users_chat_id ON warns_users(chat_id);
CREATE INDEX IF NOT EXISTS idx_chat_users_user_id ON chat_users(user_id);
CREATE INDEX IF NOT EXISTS idx_chat_users_chat_id ON chat_users(chat_id);

-- =====================================================
-- STEP 7: Add Table Comments
-- =====================================================

COMMENT ON TABLE chats IS 'Main table storing chat/group information';
COMMENT ON TABLE users IS 'Main table storing user information';
COMMENT ON TABLE chat_users IS 'Junction table for many-to-many relationship between chats and users';
COMMENT ON TABLE admin IS 'Stores admin users for each chat';
COMMENT ON TABLE antiflood_settings IS 'Anti-flood/spam settings per chat';
COMMENT ON TABLE blacklists IS 'Blacklisted words/phrases per chat';
COMMENT ON TABLE channels IS 'Linked channels for each chat';
COMMENT ON TABLE connection IS 'User-to-chat connection settings';
COMMENT ON TABLE connection_settings IS 'Chat-level connection settings';
COMMENT ON TABLE devs IS 'Bot developers and sudo users';
COMMENT ON TABLE disable IS 'Disabled commands per chat';
COMMENT ON TABLE filters IS 'Custom keyword filters per chat';
COMMENT ON TABLE greetings IS 'Welcome and goodbye message settings per chat';
COMMENT ON TABLE locks IS 'Locked permissions per chat';
COMMENT ON TABLE notes IS 'Saved notes/tags per chat';
COMMENT ON TABLE notes_settings IS 'Note settings per chat';
COMMENT ON TABLE pins IS 'Pinned message settings per chat';
COMMENT ON TABLE report_chat_settings IS 'Report settings per chat';
COMMENT ON TABLE report_user_settings IS 'Report settings per user';
COMMENT ON TABLE rules IS 'Chat rules text';
COMMENT ON TABLE warns_settings IS 'Warning system settings per chat';
COMMENT ON TABLE warns_users IS 'User warnings per chat';

COMMIT;

-- =====================================================
-- ROLLBACK INSTRUCTIONS
-- =====================================================
-- If you need to rollback this migration, create a new migration with:
/*
BEGIN;

-- Drop all foreign key constraints
ALTER TABLE admin DROP CONSTRAINT IF EXISTS fk_admin_chat;
ALTER TABLE antiflood_settings DROP CONSTRAINT IF EXISTS fk_antiflood_chat;
ALTER TABLE blacklists DROP CONSTRAINT IF EXISTS fk_blacklists_chat;
ALTER TABLE channels DROP CONSTRAINT IF EXISTS fk_channels_chat;
ALTER TABLE channels DROP CONSTRAINT IF EXISTS fk_channels_channel;
ALTER TABLE connection_settings DROP CONSTRAINT IF EXISTS fk_connection_settings_chat;
ALTER TABLE disable DROP CONSTRAINT IF EXISTS fk_disable_chat;
ALTER TABLE filters DROP CONSTRAINT IF EXISTS fk_filters_chat;
ALTER TABLE greetings DROP CONSTRAINT IF EXISTS fk_greetings_chat;
ALTER TABLE locks DROP CONSTRAINT IF EXISTS fk_locks_chat;
ALTER TABLE notes DROP CONSTRAINT IF EXISTS fk_notes_chat;
ALTER TABLE notes_settings DROP CONSTRAINT IF EXISTS fk_notes_settings_chat;
ALTER TABLE pins DROP CONSTRAINT IF EXISTS fk_pins_chat;
ALTER TABLE report_chat_settings DROP CONSTRAINT IF EXISTS fk_report_chat_settings_chat;
ALTER TABLE rules DROP CONSTRAINT IF EXISTS fk_rules_chat;
ALTER TABLE warns_settings DROP CONSTRAINT IF EXISTS fk_warns_settings_chat;
ALTER TABLE devs DROP CONSTRAINT IF EXISTS fk_devs_user;
ALTER TABLE report_user_settings DROP CONSTRAINT IF EXISTS fk_report_user_settings_user;
ALTER TABLE chat_users DROP CONSTRAINT IF EXISTS fk_chat_users_chat;
ALTER TABLE chat_users DROP CONSTRAINT IF EXISTS fk_chat_users_user;
ALTER TABLE connection DROP CONSTRAINT IF EXISTS fk_connection_user;
ALTER TABLE connection DROP CONSTRAINT IF EXISTS fk_connection_chat;
ALTER TABLE warns_users DROP CONSTRAINT IF EXISTS fk_warns_users_user;
ALTER TABLE warns_users DROP CONSTRAINT IF EXISTS fk_warns_users_chat;

-- Drop unique constraints
ALTER TABLE chats DROP CONSTRAINT IF EXISTS uk_chats_chat_id;
ALTER TABLE users DROP CONSTRAINT IF EXISTS uk_users_user_id;
ALTER TABLE connection DROP CONSTRAINT IF EXISTS uk_connection_user_chat;
ALTER TABLE warns_users DROP CONSTRAINT IF EXISTS uk_warns_users_user_chat;
ALTER TABLE blacklists DROP CONSTRAINT IF EXISTS uk_blacklists_chat_word;
ALTER TABLE disable DROP CONSTRAINT IF EXISTS uk_disable_chat_command;
ALTER TABLE filters DROP CONSTRAINT IF EXISTS uk_filters_chat_keyword;
ALTER TABLE locks DROP CONSTRAINT IF EXISTS uk_locks_chat_type;
ALTER TABLE notes DROP CONSTRAINT IF EXISTS uk_notes_chat_name;

-- Drop check constraints
ALTER TABLE warns_settings DROP CONSTRAINT IF EXISTS chk_warns_settings_limit;
ALTER TABLE antiflood_settings DROP CONSTRAINT IF EXISTS chk_antiflood_limit;
ALTER TABLE warns_users DROP CONSTRAINT IF EXISTS chk_warns_users_num;
ALTER TABLE antiflood_settings DROP CONSTRAINT IF EXISTS chk_antiflood_action;
ALTER TABLE antiflood_settings DROP CONSTRAINT IF EXISTS chk_antiflood_mode;
ALTER TABLE blacklists DROP CONSTRAINT IF EXISTS chk_blacklists_action;
ALTER TABLE warns_settings DROP CONSTRAINT IF EXISTS chk_warns_mode;

COMMIT;
*/
