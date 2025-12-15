-- =====================================================
-- Supabase Migration: Add Missing Channel Foreign Key Index
-- =====================================================
-- Migration Name: add_missing_channel_index
-- Description: Adds missing index for channels.channel_id foreign key
-- Date: 2025-08-06
-- =====================================================

BEGIN;

-- =====================================================
-- Add Missing Foreign Key Index
-- =====================================================
-- This index is critical for the fk_channels_channel foreign key
-- Without it, referential integrity checks cause full table scans
-- when deleting or updating chats that may be referenced as channels

CREATE INDEX IF NOT EXISTS idx_channels_channel_id ON public.channels(channel_id);

-- Add comment for documentation
COMMENT ON INDEX idx_channels_channel_id IS 'Index for foreign key fk_channels_channel to improve referential integrity performance on CASCADE operations';

-- Update statistics for query planner
ANALYZE channels;

COMMIT;

-- =====================================================
-- PERFORMANCE IMPACT
-- =====================================================
-- This index will:
-- 1. Significantly improve DELETE/UPDATE performance on chats table
-- 2. Speed up queries that JOIN channels on channel_id
-- 3. Eliminate full table scans during foreign key constraint checks
--
-- Expected improvements:
-- - CASCADE DELETE operations: 10-100x faster
-- - Foreign key validation: Near instant vs full table scan

-- =====================================================
-- ROLLBACK INSTRUCTIONS
-- =====================================================
-- If you need to rollback this migration:
/*
DROP INDEX IF EXISTS idx_channels_channel_id;
*/

-- =====================================================
-- VERIFICATION QUERY
-- =====================================================
-- After running this migration, verify with:
/*
SELECT
    indexname,
    indexdef,
    tablename
FROM pg_indexes
WHERE tablename = 'channels'
  AND indexname = 'idx_channels_channel_id';
*/
