-- =====================================================
-- DROP UNUSED INDEXES MIGRATION
-- =====================================================
-- This migration removes indexes that have been identified as unused through
-- production monitoring. These indexes were consuming storage and slowing down
-- writes without providing any query performance benefits.
--
-- Analysis showed these indexes had 0 scans while the unique constraint indexes
-- (uk_*) were handling all queries efficiently.
-- =====================================================

-- =====================================================
-- 1. UNUSED PERFORMANCE INDEXES FROM critical_performance_indexes.sql
-- =====================================================

-- Partial index with WHERE locked = true, but queries don't include this condition
-- uk_locks_chat_type is handling all 72,907 queries efficiently
DROP INDEX IF EXISTS idx_locks_chat_lock_lookup;

-- Covering index not being used, queries are using uk_locks_chat_type instead
DROP INDEX IF EXISTS idx_locks_covering;

-- Partial index with WHERE user_id IS NOT NULL, but uk_users_user_id handles all queries
-- This was consuming 5.4MB with 0 scans
DROP INDEX IF EXISTS idx_users_user_id_active;

-- Partial index with WHERE is_inactive = false, but queries don't include this condition
-- uk_chats_chat_id is handling all 132,292 queries efficiently
DROP INDEX IF EXISTS idx_chats_chat_id_active;

-- Partial index with WHERE limit > 0, but queries don't include this condition
-- idx_antiflood_settings_chat_id is handling all 16,685 queries efficiently
DROP INDEX IF EXISTS idx_antiflood_chat_active;

-- Optimized index not being used, uk_blacklists_chat_word handles all 12,930 queries
DROP INDEX IF EXISTS idx_blacklists_chat_word_optimized;

-- Update-specific index not being used for channel updates
DROP INDEX IF EXISTS idx_channels_chat_update;

-- Partial index with WHERE conditions for enabled greetings, but queries don't match
DROP INDEX IF EXISTS idx_greetings_chat_enabled;

-- Composite index with WHERE num_warns > 0, but queries don't include this condition
-- uk_warns_users_user_chat is handling all 489 queries efficiently
DROP INDEX IF EXISTS idx_warns_users_composite;

-- Duplicate of uk_notes_chat_name which is being used for all 17 queries
DROP INDEX IF EXISTS idx_notes_chat_name;

-- Duplicate of idx_pins_chat_id which is being used for all 104 queries
DROP INDEX IF EXISTS idx_pins_chat;

-- Partial index for inactive chats, but queries don't filter by is_inactive
DROP INDEX IF EXISTS idx_chats_inactive;

-- GIN index for JSONB users field, consuming 400KB with 0 scans
-- Not needed for current query patterns
DROP INDEX IF EXISTS idx_chats_users_gin;

-- =====================================================
-- 2. OTHER UNUSED INDEXES
-- =====================================================

-- Not being used, idx_channels_chat_id handles all 340 queries
DROP INDEX IF EXISTS idx_channels_channel_id;

-- Connection queries using uk_connection_user_chat instead (36 queries)
DROP INDEX IF EXISTS idx_connection_chat_id;

-- Chat users queries not using this index
DROP INDEX IF EXISTS idx_chat_users_user_id;

-- =====================================================
-- INDEXES BEING KEPT (for reference)
-- =====================================================
-- The following indexes ARE being used and should be kept:
--
-- HEAVILY USED:
-- - uk_users_user_id: 309,275 scans (primary user lookups)
-- - uk_chats_chat_id: 132,292 scans (primary chat lookups)
-- - uk_locks_chat_type: 72,907 scans (lock checks)
-- - uk_filters_chat_keyword: 35,746 scans (filter lookups)
-- - idx_antiflood_settings_chat_id: 16,685 scans
-- - uk_blacklists_chat_word: 12,930 scans
--
-- COVERING INDEXES THAT WORK:
-- - idx_users_covering: 3,696 scans (optimized user queries)
-- - idx_chats_covering: 1,263 scans (optimized chat queries)
-- - idx_filters_chat_optimized: 16 scans
--
-- These indexes are sufficient for current query patterns and are being
-- efficiently utilized by the PostgreSQL query planner.

-- =====================================================
-- SUMMARY
-- =====================================================
-- Dropped: 16 unused indexes
-- Storage freed: ~6MB
-- Expected impact: Faster writes, no change to read performance
-- =====================================================
