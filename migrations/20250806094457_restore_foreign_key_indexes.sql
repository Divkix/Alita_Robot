-- =====================================================
-- Supabase Migration: Restore Foreign Key Indexes
-- =====================================================
-- Migration Name: restore_foreign_key_indexes
-- Description: Restores critical indexes for foreign key constraints that were
--              mistakenly dropped in migration 20250806093839_drop_unused_indexes
-- Date: 2025-08-06
-- =====================================================

BEGIN;

-- =====================================================
-- BACKGROUND: Why These Indexes Are Critical
-- =====================================================
-- These indexes were incorrectly identified as "unused" because the application
-- doesn't directly query them. However, PostgreSQL REQUIRES these indexes for:
-- 1. Efficient CASCADE DELETE/UPDATE operations on parent tables
-- 2. Foreign key constraint validation during INSERT/UPDATE
-- 3. Preventing full table scans during referential integrity checks
--
-- Without these indexes, operations like deleting a user or chat will cause
-- PostgreSQL to perform full table scans on child tables, severely impacting performance.

-- =====================================================
-- STEP 1: Restore Critical Foreign Key Indexes
-- =====================================================

-- chat_users table: Index on user_id for fk_chat_users_user
-- Required when deleting/updating users to efficiently find related chat_users
CREATE INDEX IF NOT EXISTS idx_chat_users_user_id
ON public.chat_users(user_id);

-- connection table: Index on chat_id for fk_connection_chat
-- Required when deleting/updating chats to efficiently find related connections
CREATE INDEX IF NOT EXISTS idx_connection_chat_id
ON public.connection(chat_id);

-- =====================================================
-- STEP 2: Add Comments for Documentation
-- =====================================================

COMMENT ON INDEX idx_chat_users_user_id IS
'Critical index for fk_chat_users_user foreign key - enables efficient CASCADE operations when deleting/updating users';

COMMENT ON INDEX idx_connection_chat_id IS
'Critical index for fk_connection_chat foreign key - enables efficient CASCADE operations when deleting/updating chats';

-- =====================================================
-- STEP 3: Update Table Statistics
-- =====================================================
-- Ensure query planner has accurate statistics after index creation

ANALYZE chat_users;
ANALYZE connection;

COMMIT;

-- =====================================================
-- PERFORMANCE IMPACT
-- =====================================================
-- These indexes will:
-- 1. Dramatically improve DELETE/UPDATE performance on users and chats tables
-- 2. Eliminate full table scans during foreign key constraint checks
-- 3. Speed up CASCADE operations from O(n) to O(log n)
--
-- Expected improvements:
-- - CASCADE DELETE on users/chats: 10-1000x faster depending on table size
-- - Foreign key validation: Near instant vs full table scan
-- - Reduced database CPU usage during maintenance operations

-- =====================================================
-- VERIFICATION QUERIES
-- =====================================================
/*
-- Verify indexes have been created
SELECT
    schemaname,
    tablename,
    indexname,
    indexdef
FROM pg_indexes
WHERE schemaname = 'public'
  AND indexname IN ('idx_chat_users_user_id', 'idx_connection_chat_id')
ORDER BY tablename, indexname;

-- Check that foreign keys now have covering indexes
SELECT
    conname AS constraint_name,
    conrelid::regclass AS table_name,
    a.attname AS column_name,
    confrelid::regclass AS foreign_table_name,
    af.attname AS foreign_column_name,
    EXISTS (
        SELECT 1
        FROM pg_index i
        WHERE i.indrelid = c.conrelid
        AND conkey[1] = ANY(i.indkey)
    ) AS has_index
FROM pg_constraint c
JOIN pg_attribute a ON a.attnum = c.conkey[1] AND a.attrelid = c.conrelid
JOIN pg_attribute af ON af.attnum = c.confkey[1] AND af.attrelid = c.confrelid
WHERE c.contype = 'f'
  AND c.conrelid IN ('chat_users'::regclass, 'connection'::regclass)
ORDER BY table_name, constraint_name;
*/

-- =====================================================
-- ROLLBACK INSTRUCTIONS
-- =====================================================
-- If you need to rollback this migration (NOT RECOMMENDED):
/*
BEGIN;
DROP INDEX IF EXISTS idx_chat_users_user_id;
DROP INDEX IF EXISTS idx_connection_chat_id;
COMMIT;

WARNING: Removing these indexes will severely degrade performance!
*/
