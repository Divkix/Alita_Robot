-- =====================================================
-- FILTERS FUNCTIONAL INDEX & WARNS CHAT INDEX
-- =====================================================
-- This migration adds targeted indexes to eliminate full-table scans
-- on high-frequency filter watcher and warn lookup queries.
--
-- NOTE: Uses regular CREATE INDEX (without CONCURRENTLY) to remain
-- compatible with Supabase migration system which runs in transactions.

-- =====================================================
-- 1. FILTERS - Functional index for case-insensitive keyword lookup
-- =====================================================
-- DoesFilterExists queries: WHERE chat_id = ? AND LOWER(keyword) = LOWER(?)
-- The existing idx_filters_chat_optimized ON filters(chat_id) only covers
-- the chat_id predicate. Without a functional index on LOWER(keyword),
-- PostgreSQL must heap-scan every filter row for the chat to evaluate
-- the LOWER(keyword) expression on every filter-watcher message.
-- This composite functional index lets PostgreSQL resolve both predicates
-- entirely from the index, eliminating the heap scan.
CREATE INDEX IF NOT EXISTS idx_filters_chat_keyword_lower
ON filters(chat_id, LOWER(keyword));

-- =====================================================
-- 2. WARNS USERS - Chat-only lookup index
-- =====================================================
-- GetAllChatWarns and ResetAllChatWarns query: WHERE chat_id = ?
-- The existing idx_warns_users_composite ON warns_users(user_id, chat_id)
-- leads with user_id, so PostgreSQL cannot use it for chat_id-only lookups.
-- This index allows efficient scans when only chat_id is provided.
CREATE INDEX IF NOT EXISTS idx_warns_users_chat_id
ON warns_users(chat_id);

-- =====================================================
-- 3. UPDATE TABLE STATISTICS
-- =====================================================
ANALYZE filters;
ANALYZE warns_users;

DO $$
BEGIN
    RAISE NOTICE 'Filters functional index and warns chat index migration completed successfully';
END $$;
