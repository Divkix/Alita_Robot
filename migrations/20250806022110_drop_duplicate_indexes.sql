-- =====================================================
-- Supabase Migration: Drop Duplicate Indexes
-- =====================================================
-- Migration Name: drop_duplicate_indexes
-- Description: Recreates foreign keys to use unique constraints then drops redundant indexes
-- Date: 2025-08-06
-- =====================================================

BEGIN;

-- =====================================================
-- STEP 1: Drop Foreign Keys that depend on idx_chats_chat_id
-- =====================================================
-- We need to drop and recreate these foreign keys to reference uk_chats_chat_id instead

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
ALTER TABLE chat_users DROP CONSTRAINT IF EXISTS fk_chat_users_chat;
ALTER TABLE connection DROP CONSTRAINT IF EXISTS fk_connection_chat;
ALTER TABLE warns_users DROP CONSTRAINT IF EXISTS fk_warns_users_chat;

-- =====================================================
-- STEP 2: Drop Foreign Keys that depend on idx_users_user_id
-- =====================================================

ALTER TABLE devs DROP CONSTRAINT IF EXISTS fk_devs_user;
ALTER TABLE report_user_settings DROP CONSTRAINT IF EXISTS fk_report_user_settings_user;
ALTER TABLE chat_users DROP CONSTRAINT IF EXISTS fk_chat_users_user;
ALTER TABLE connection DROP CONSTRAINT IF EXISTS fk_connection_user;
ALTER TABLE warns_users DROP CONSTRAINT IF EXISTS fk_warns_users_user;

-- =====================================================
-- STEP 3: Drop Redundant Indexes
-- =====================================================
-- Now we can safely drop the redundant indexes

DROP INDEX IF EXISTS idx_chats_chat_id;
DROP INDEX IF EXISTS idx_users_user_id;

-- =====================================================
-- STEP 4: Recreate Foreign Keys using uk_chats_chat_id
-- =====================================================

ALTER TABLE admin
ADD CONSTRAINT fk_admin_chat
FOREIGN KEY (chat_id) REFERENCES chats(chat_id)
ON DELETE CASCADE ON UPDATE CASCADE;

ALTER TABLE antiflood_settings
ADD CONSTRAINT fk_antiflood_chat
FOREIGN KEY (chat_id) REFERENCES chats(chat_id)
ON DELETE CASCADE ON UPDATE CASCADE;

ALTER TABLE blacklists
ADD CONSTRAINT fk_blacklists_chat
FOREIGN KEY (chat_id) REFERENCES chats(chat_id)
ON DELETE CASCADE ON UPDATE CASCADE;

ALTER TABLE channels
ADD CONSTRAINT fk_channels_chat
FOREIGN KEY (chat_id) REFERENCES chats(chat_id)
ON DELETE CASCADE ON UPDATE CASCADE;

ALTER TABLE channels
ADD CONSTRAINT fk_channels_channel
FOREIGN KEY (channel_id) REFERENCES chats(chat_id)
ON DELETE SET NULL ON UPDATE CASCADE;

ALTER TABLE connection_settings
ADD CONSTRAINT fk_connection_settings_chat
FOREIGN KEY (chat_id) REFERENCES chats(chat_id)
ON DELETE CASCADE ON UPDATE CASCADE;

ALTER TABLE disable
ADD CONSTRAINT fk_disable_chat
FOREIGN KEY (chat_id) REFERENCES chats(chat_id)
ON DELETE CASCADE ON UPDATE CASCADE;

ALTER TABLE filters
ADD CONSTRAINT fk_filters_chat
FOREIGN KEY (chat_id) REFERENCES chats(chat_id)
ON DELETE CASCADE ON UPDATE CASCADE;

ALTER TABLE greetings
ADD CONSTRAINT fk_greetings_chat
FOREIGN KEY (chat_id) REFERENCES chats(chat_id)
ON DELETE CASCADE ON UPDATE CASCADE;

ALTER TABLE locks
ADD CONSTRAINT fk_locks_chat
FOREIGN KEY (chat_id) REFERENCES chats(chat_id)
ON DELETE CASCADE ON UPDATE CASCADE;

ALTER TABLE notes
ADD CONSTRAINT fk_notes_chat
FOREIGN KEY (chat_id) REFERENCES chats(chat_id)
ON DELETE CASCADE ON UPDATE CASCADE;

ALTER TABLE notes_settings
ADD CONSTRAINT fk_notes_settings_chat
FOREIGN KEY (chat_id) REFERENCES chats(chat_id)
ON DELETE CASCADE ON UPDATE CASCADE;

ALTER TABLE pins
ADD CONSTRAINT fk_pins_chat
FOREIGN KEY (chat_id) REFERENCES chats(chat_id)
ON DELETE CASCADE ON UPDATE CASCADE;

ALTER TABLE report_chat_settings
ADD CONSTRAINT fk_report_chat_settings_chat
FOREIGN KEY (chat_id) REFERENCES chats(chat_id)
ON DELETE CASCADE ON UPDATE CASCADE;

ALTER TABLE rules
ADD CONSTRAINT fk_rules_chat
FOREIGN KEY (chat_id) REFERENCES chats(chat_id)
ON DELETE CASCADE ON UPDATE CASCADE;

ALTER TABLE warns_settings
ADD CONSTRAINT fk_warns_settings_chat
FOREIGN KEY (chat_id) REFERENCES chats(chat_id)
ON DELETE CASCADE ON UPDATE CASCADE;

ALTER TABLE chat_users
ADD CONSTRAINT fk_chat_users_chat
FOREIGN KEY (chat_id) REFERENCES chats(chat_id)
ON DELETE CASCADE ON UPDATE CASCADE;

ALTER TABLE connection
ADD CONSTRAINT fk_connection_chat
FOREIGN KEY (chat_id) REFERENCES chats(chat_id)
ON DELETE CASCADE ON UPDATE CASCADE;

ALTER TABLE warns_users
ADD CONSTRAINT fk_warns_users_chat
FOREIGN KEY (chat_id) REFERENCES chats(chat_id)
ON DELETE CASCADE ON UPDATE CASCADE;

-- =====================================================
-- STEP 5: Recreate Foreign Keys using uk_users_user_id
-- =====================================================

ALTER TABLE devs
ADD CONSTRAINT fk_devs_user
FOREIGN KEY (user_id) REFERENCES users(user_id)
ON DELETE CASCADE ON UPDATE CASCADE;

ALTER TABLE report_user_settings
ADD CONSTRAINT fk_report_user_settings_user
FOREIGN KEY (user_id) REFERENCES users(user_id)
ON DELETE CASCADE ON UPDATE CASCADE;

ALTER TABLE chat_users
ADD CONSTRAINT fk_chat_users_user
FOREIGN KEY (user_id) REFERENCES users(user_id)
ON DELETE CASCADE ON UPDATE CASCADE;

ALTER TABLE connection
ADD CONSTRAINT fk_connection_user
FOREIGN KEY (user_id) REFERENCES users(user_id)
ON DELETE CASCADE ON UPDATE CASCADE;

ALTER TABLE warns_users
ADD CONSTRAINT fk_warns_users_user
FOREIGN KEY (user_id) REFERENCES users(user_id)
ON DELETE CASCADE ON UPDATE CASCADE;

-- =====================================================
-- STEP 6: Add Comments for Documentation
-- =====================================================

COMMENT ON CONSTRAINT uk_chats_chat_id ON chats IS 'Unique constraint on chat_id - used for foreign key references';
COMMENT ON CONSTRAINT uk_users_user_id ON users IS 'Unique constraint on user_id - used for foreign key references';

COMMIT;

-- =====================================================
-- ROLLBACK INSTRUCTIONS
-- =====================================================
-- If you need to rollback this migration, create a new migration with:
/*
BEGIN;

-- Step 1: Drop all foreign keys again
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
ALTER TABLE chat_users DROP CONSTRAINT IF EXISTS fk_chat_users_chat;
ALTER TABLE connection DROP CONSTRAINT IF EXISTS fk_connection_chat;
ALTER TABLE warns_users DROP CONSTRAINT IF EXISTS fk_warns_users_chat;
ALTER TABLE devs DROP CONSTRAINT IF EXISTS fk_devs_user;
ALTER TABLE report_user_settings DROP CONSTRAINT IF EXISTS fk_report_user_settings_user;
ALTER TABLE chat_users DROP CONSTRAINT IF EXISTS fk_chat_users_user;
ALTER TABLE connection DROP CONSTRAINT IF EXISTS fk_connection_user;
ALTER TABLE warns_users DROP CONSTRAINT IF EXISTS fk_warns_users_user;

-- Step 2: Recreate the old indexes
CREATE UNIQUE INDEX idx_chats_chat_id ON public.chats USING btree (chat_id);
CREATE UNIQUE INDEX idx_users_user_id ON public.users USING btree (user_id);

-- Step 3: Recreate all foreign keys (they will use the old indexes)
-- [Include all the ALTER TABLE ADD CONSTRAINT statements from steps 4 and 5 above]

COMMIT;
*/

-- =====================================================
-- VERIFICATION QUERY
-- =====================================================
-- After running this migration, you can verify the indexes with:
/*
SELECT
    schemaname,
    tablename,
    indexname,
    indexdef
FROM pg_indexes
WHERE tablename IN ('chats', 'users')
    AND schemaname = 'public'
ORDER BY tablename, indexname;

-- Verify foreign keys are using the correct constraints:
SELECT
    conname AS constraint_name,
    conrelid::regclass AS table_name,
    confrelid::regclass AS referenced_table
FROM pg_constraint
WHERE contype = 'f'
    AND (confrelid = 'chats'::regclass OR confrelid = 'users'::regclass)
ORDER BY conrelid::regclass::text, conname;
*/
