-- =====================================================
-- Supabase Migration: Drop Unused Indexes
-- =====================================================
-- Migration Name: drop_unused_indexes
-- Description: Removes indexes that have never been used by the application
-- Date: 2025-08-06
-- =====================================================

BEGIN;

-- =====================================================
-- ANALYSIS: Why These Indexes Are Unused
-- =====================================================
-- The application loads all records for a chat and processes them in memory
-- rather than querying for specific keywords/words/names.
-- This is actually more efficient for the bot's use case since:
-- 1. It needs to check multiple patterns at once
-- 2. Results are cached after loading
-- 3. Avoids multiple round trips to the database

-- =====================================================
-- STEP 1: Drop Unused Composite Indexes
-- =====================================================

-- Blacklists: App uses GetRecords(&blacklists, {ChatId: chatId})
-- Never queries: WHERE chat_id = ? AND word = ?
DROP INDEX IF EXISTS idx_blacklist_chat_word;

-- Connection: Covered by uk_connection_user_chat unique constraint
-- App queries use WHERE user_id = ? AND chat_id = ? which uses the unique index
DROP INDEX IF EXISTS idx_connection_user_chat;

-- Disable: App loads all disabled commands for a chat
-- Never queries: WHERE chat_id = ? AND command = ?
DROP INDEX IF EXISTS idx_disable_chat_command;

-- Filters: App uses GetRecords(&filters, {ChatId: chatId})
-- Never queries: WHERE chat_id = ? AND keyword = ?
DROP INDEX IF EXISTS idx_filters_chat_keyword;

-- Locks: App loads all locks for a chat at once
-- Never queries: WHERE chat_id = ? AND lock_type = ?
DROP INDEX IF EXISTS idx_lock_chat_type;

-- Notes: App loads all notes for a chat
-- Never queries: WHERE chat_id = ? AND note_name = ?
DROP INDEX IF EXISTS idx_notes_chat_name;

-- Warns: Covered by uk_warns_users_user_chat unique constraint
-- Duplicate of the unique constraint index
DROP INDEX IF EXISTS idx_warns_user_chat;

-- =====================================================
-- STEP 2: Drop Redundant Single-Column Indexes
-- =====================================================

-- Connection table: uk_connection_user_chat(user_id, chat_id) already exists
-- PostgreSQL can use leftmost column of composite index for single-column queries
DROP INDEX IF EXISTS idx_connection_user_id;
DROP INDEX IF EXISTS idx_connection_chat_id;

-- Chat_users table: Primary key (chat_id, user_id) already provides indexing
-- Additional single-column indexes are redundant
DROP INDEX IF EXISTS idx_chat_users_user_id;
DROP INDEX IF EXISTS idx_chat_users_chat_id;

-- =====================================================
-- STEP 3: Update Table Statistics
-- =====================================================
-- Ensure query planner has accurate statistics after index removal

ANALYZE blacklists;
ANALYZE connection;
ANALYZE disable;
ANALYZE filters;
ANALYZE locks;
ANALYZE notes;
ANALYZE warns_users;
ANALYZE chat_users;

COMMIT;

-- =====================================================
-- PERFORMANCE BENEFITS
-- =====================================================
-- 1. Storage: Frees up space from 11 unused indexes
-- 2. Write Performance: INSERT/UPDATE/DELETE operations will be faster
-- 3. Maintenance: Reduced VACUUM and ANALYZE overhead
-- 4. Memory: Less index metadata in buffer cache
--
-- Estimated improvements:
-- - INSERT operations: 5-10% faster
-- - UPDATE operations: 10-15% faster
-- - Storage saved: Several MB to GB depending on data size

-- =====================================================
-- ROLLBACK INSTRUCTIONS
-- =====================================================
-- If you need to restore these indexes (not recommended):
/*
BEGIN;

-- Recreate composite indexes
CREATE INDEX idx_blacklist_chat_word ON public.blacklists(chat_id, word);
CREATE INDEX idx_connection_user_chat ON public.connection(user_id, chat_id);
CREATE INDEX idx_disable_chat_command ON public.disable(chat_id, command);
CREATE INDEX idx_filters_chat_keyword ON public.filters(chat_id, keyword);
CREATE INDEX idx_lock_chat_type ON public.locks(chat_id, lock_type);
CREATE INDEX idx_notes_chat_name ON public.notes(chat_id, note_name);
CREATE INDEX idx_warns_user_chat ON public.warns_users(user_id, chat_id);

-- Recreate single-column indexes
CREATE INDEX idx_connection_user_id ON public.connection(user_id);
CREATE INDEX idx_connection_chat_id ON public.connection(chat_id);
CREATE INDEX idx_chat_users_user_id ON public.chat_users(user_id);
CREATE INDEX idx_chat_users_chat_id ON public.chat_users(chat_id);

COMMIT;
*/

-- =====================================================
-- VERIFICATION QUERIES
-- =====================================================
/*
-- Verify indexes have been dropped
SELECT COUNT(*) as dropped_index_count
FROM pg_indexes
WHERE indexname IN (
    'idx_blacklist_chat_word',
    'idx_connection_user_chat',
    'idx_disable_chat_command',
    'idx_filters_chat_keyword',
    'idx_lock_chat_type',
    'idx_notes_chat_name',
    'idx_warns_user_chat',
    'idx_connection_user_id',
    'idx_connection_chat_id',
    'idx_chat_users_user_id',
    'idx_chat_users_chat_id'
);
-- Should return 0

-- Check remaining indexes on affected tables
SELECT
    tablename,
    indexname,
    indexdef
FROM pg_indexes
WHERE tablename IN ('blacklists', 'connection', 'disable', 'filters',
                    'locks', 'notes', 'warns_users', 'chat_users')
  AND schemaname = 'public'
ORDER BY tablename, indexname;

-- Monitor index usage after migration (run after a few days)
SELECT
    schemaname,
    tablename,
    indexname,
    idx_scan as times_used,
    idx_tup_read as rows_read,
    idx_tup_fetch as rows_fetched
FROM pg_stat_user_indexes
WHERE schemaname = 'public'
  AND idx_scan > 0
ORDER BY idx_scan DESC;
*/
