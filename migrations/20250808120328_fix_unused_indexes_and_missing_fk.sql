-- =====================================================
-- Fix Unused Indexes and Missing Foreign Key Index
-- =====================================================
-- This migration addresses Supabase linter suggestions:
-- 1. Adds missing covering index for captcha_attempts.chat_id FK
-- 2. Drops unused indexes that were unnecessarily recreated
-- Date: 2025-08-08
-- =====================================================

BEGIN;

-- =====================================================
-- 1. Add Missing Foreign Key Covering Index
-- =====================================================
-- The foreign key fk_captcha_attempts_chat on captcha_attempts(chat_id)
-- needs a covering index for optimal JOIN performance.
-- This resolves: "Unindexed foreign keys" linter warning

CREATE INDEX IF NOT EXISTS idx_captcha_attempts_chat_id
ON public.captcha_attempts(chat_id);
COMMENT ON INDEX idx_captcha_attempts_chat_id IS 'Covering index for foreign key fk_captcha_attempts_chat';

-- =====================================================
-- 2. Drop Unused Indexes (0 scans in production)
-- =====================================================
-- These indexes were previously dropped in migration 20250806105636
-- but were mistakenly recreated in migration 20250807123000.
-- Supabase linter confirms they have never been used.

-- captcha_attempts.expires_at index - likely cleanup uses direct timestamp comparison
DROP INDEX IF EXISTS public.idx_captcha_expires_at;

-- channels.channel_id index - redundant, was recreated but still shows 0 scans
DROP INDEX IF EXISTS public.idx_channels_channel_id;

-- chat_users.user_id index - redundant, was recreated but still shows 0 scans
DROP INDEX IF EXISTS public.idx_chat_users_user_id;

-- connection.chat_id index - redundant, was recreated but still shows 0 scans
DROP INDEX IF EXISTS public.idx_connection_chat_id;

-- =====================================================
-- 3. Update Statistics for Query Planner
-- =====================================================
ANALYZE captcha_attempts;
ANALYZE channels;
ANALYZE chat_users;
ANALYZE connection;

COMMIT;

-- =====================================================
-- VERIFICATION QUERIES (run manually after migration)
-- =====================================================
-- 1. Confirm new covering index exists:
-- SELECT indexname FROM pg_indexes
--  WHERE tablename = 'captcha_attempts'
--    AND indexname = 'idx_captcha_attempts_chat_id';
--
-- 2. Confirm unused indexes are dropped:
-- SELECT indexname FROM pg_indexes
--  WHERE schemaname = 'public'
--    AND indexname IN (
--      'idx_captcha_expires_at',
--      'idx_channels_channel_id',
--      'idx_chat_users_user_id',
--      'idx_connection_chat_id'
--    );
-- (Should return 0 rows)
--
-- 3. Check foreign key has covering index:
-- SELECT
--   tc.constraint_name,
--   tc.table_name,
--   kcu.column_name,
--   (SELECT COUNT(*) FROM pg_indexes i
--    WHERE i.tablename = tc.table_name
--      AND i.indexdef LIKE '%' || kcu.column_name || '%') as index_count
-- FROM information_schema.table_constraints tc
-- JOIN information_schema.key_column_usage kcu
--   ON tc.constraint_name = kcu.constraint_name
-- WHERE tc.constraint_type = 'FOREIGN KEY'
--   AND tc.table_name = 'captcha_attempts'
--   AND tc.constraint_name = 'fk_captcha_attempts_chat';
-- (Should show index_count > 0)
