-- =====================================================
-- CRITICAL PERFORMANCE INDEXES MIGRATION
-- =====================================================
-- This migration addresses severe performance issues identified in query performance reports
-- Priority 1: Most critical indexes for high-frequency queries
--
-- NOTE: This migration uses regular CREATE INDEX (without CONCURRENTLY)
-- to be compatible with Supabase migration system which runs in transactions.
-- Indexes will be created with minimal locking since tables are relatively small.

-- =====================================================
-- 1. LOCKS TABLE - Most Critical (319,183 calls)
-- =====================================================
-- Composite index for chat_id and lock_type lookups
CREATE INDEX IF NOT EXISTS idx_locks_chat_lock_lookup
ON locks(chat_id, lock_type)
WHERE locked = true;

-- Covering index to avoid table lookups
CREATE INDEX IF NOT EXISTS idx_locks_covering
ON locks(chat_id, lock_type)
INCLUDE (locked, id);

-- =====================================================
-- 2. USERS TABLE - High Impact (61,067 calls)
-- =====================================================
-- Primary lookup index
CREATE INDEX IF NOT EXISTS idx_users_user_id_active
ON users(user_id)
WHERE user_id IS NOT NULL;

-- Covering index for common fields (limited to small columns to avoid size limits)
CREATE INDEX IF NOT EXISTS idx_users_covering
ON users(user_id)
INCLUDE (language);

-- Note: Removed partial index for recently active users
-- (Date-based predicates cannot be used in indexes as they are not IMMUTABLE)

-- =====================================================
-- 3. CHATS TABLE - High Impact (123,242 calls)
-- =====================================================
-- Primary lookup index for active chats
CREATE INDEX IF NOT EXISTS idx_chats_chat_id_active
ON chats(chat_id)
WHERE is_inactive = false;

-- Covering index for common fields (limited to small columns to avoid size limits)
CREATE INDEX IF NOT EXISTS idx_chats_covering
ON chats(chat_id)
INCLUDE (language, is_inactive);

-- GIN index for users JSONB field if it exists
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'chats'
        AND column_name = 'users'
        AND data_type = 'jsonb'
    ) THEN
        CREATE INDEX IF NOT EXISTS idx_chats_users_gin
        ON chats USING GIN (users)
        WHERE users IS NOT NULL;
    END IF;
END $$;

-- =====================================================
-- 4. ANTIFLOOD SETTINGS - Medium Impact (58,166 calls)
-- =====================================================
-- Index for active antiflood settings
CREATE INDEX IF NOT EXISTS idx_antiflood_chat_active
ON antiflood_settings(chat_id)
WHERE "limit" > 0;

-- =====================================================
-- 5. FILTERS - Medium Impact (33,898 calls)
-- =====================================================
-- Optimized index without large text columns to avoid size limits
CREATE INDEX IF NOT EXISTS idx_filters_chat_optimized
ON filters(chat_id);

-- =====================================================
-- 6. BLACKLISTS - Easy Win (33,474 calls)
-- =====================================================
-- Optimized index with only small columns to avoid size limits
CREATE INDEX IF NOT EXISTS idx_blacklists_chat_word_optimized
ON blacklists(chat_id)
INCLUDE (action);

-- =====================================================
-- 7. CHANNELS - For Frequent Updates (17,907 calls)
-- =====================================================
-- Index for update operations
CREATE INDEX IF NOT EXISTS idx_channels_chat_update
ON channels(chat_id)
INCLUDE (channel_id, updated_at);

-- =====================================================
-- 8. GREETINGS - For Message Handling (1,407 calls)
-- =====================================================
-- Partial index for enabled greetings
CREATE INDEX IF NOT EXISTS idx_greetings_chat_enabled
ON greetings(chat_id)
WHERE welcome_enabled = true OR goodbye_enabled = true;

-- =====================================================
-- 9. ADDITIONAL PERFORMANCE INDEXES
-- =====================================================

-- Warns users composite index (without large JSONB column to avoid size limits)
CREATE INDEX IF NOT EXISTS idx_warns_users_composite
ON warns_users(user_id, chat_id)
INCLUDE (num_warns)
WHERE num_warns > 0;

-- Notes lookup index
CREATE INDEX IF NOT EXISTS idx_notes_chat_name
ON notes(chat_id, note_name);

-- Admin settings lookup
CREATE INDEX IF NOT EXISTS idx_admin_settings_chat
ON admin(chat_id);

-- Pins lookup index
CREATE INDEX IF NOT EXISTS idx_pins_chat
ON pins(chat_id);

-- Note: Removed cleanup indexes that used date-based predicates
-- (Date-based predicates cannot be used in indexes as they are not IMMUTABLE)

-- Simple index for inactive chats (without date predicate)
CREATE INDEX IF NOT EXISTS idx_chats_inactive
ON chats(chat_id)
WHERE is_inactive = true;

-- =====================================================
-- 10. UPDATE TABLE STATISTICS
-- =====================================================
-- Update statistics for better query planning
ANALYZE users;
ANALYZE chats;
ANALYZE locks;
ANALYZE antiflood_settings;
ANALYZE filters;
ANALYZE blacklists;
ANALYZE channels;
ANALYZE greetings;
ANALYZE warns_users;
ANALYZE notes;
ANALYZE admin;
ANALYZE pins;

-- Set higher statistics targets for frequently queried columns
ALTER TABLE users ALTER COLUMN user_id SET STATISTICS 1000;
ALTER TABLE chats ALTER COLUMN chat_id SET STATISTICS 1000;
ALTER TABLE locks ALTER COLUMN chat_id SET STATISTICS 500;
ALTER TABLE antiflood_settings ALTER COLUMN chat_id SET STATISTICS 500;
ALTER TABLE filters ALTER COLUMN chat_id SET STATISTICS 500;

-- =====================================================
-- VALIDATION QUERIES
-- =====================================================
-- These queries help verify that indexes are created and being used

-- List all new indexes created
DO $$
BEGIN
    RAISE NOTICE 'Performance indexes migration completed successfully';
    RAISE NOTICE 'Run the following query to verify indexes:';
    RAISE NOTICE 'SELECT tablename, indexname FROM pg_indexes WHERE indexname LIKE ''idx_%%'' ORDER BY tablename;';
END $$;
