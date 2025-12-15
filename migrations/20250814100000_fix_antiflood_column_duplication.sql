-- =====================================================
-- ANTIFLOOD COLUMN CLEANUP MIGRATION
-- =====================================================
-- This migration fixes the antiflood_settings table by:
-- 1. Removing the unused 'limit' column (GORM uses 'flood_limit' only)
-- 2. Dropping the incorrect index on 'limit' > 0
-- 3. Creating the correct index on 'flood_limit' > 0
--
-- Background: The table has both 'limit' and 'flood_limit' columns.
-- The Go code only uses 'flood_limit', but the performance index
-- was created on 'limit', causing it to never be used by queries.

-- =====================================================
-- Step 1: Drop the incorrect index on 'limit' column
-- =====================================================
DROP INDEX IF EXISTS idx_antiflood_chat_active;

-- =====================================================
-- Step 2: Drop the unused 'limit' column
-- =====================================================
-- First check if the column exists before dropping it
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public'
        AND table_name = 'antiflood_settings'
        AND column_name = 'limit'
    ) THEN
        ALTER TABLE antiflood_settings DROP COLUMN "limit";
        RAISE NOTICE 'Dropped unused limit column from antiflood_settings';
    ELSE
        RAISE NOTICE 'limit column does not exist in antiflood_settings, skipping';
    END IF;
END $$;

-- =====================================================
-- Step 3: Create the correct index on 'flood_limit' > 0
-- =====================================================
-- This index will actually be used by the application queries
-- which check for active antiflood settings (flood_limit > 0)
CREATE INDEX IF NOT EXISTS idx_antiflood_chat_flood_active
ON antiflood_settings(chat_id)
WHERE flood_limit > 0;

-- =====================================================
-- Step 4: Update table statistics
-- =====================================================
-- Ensure the query planner has up-to-date statistics
ANALYZE antiflood_settings;

-- =====================================================
-- Validation and logging
-- =====================================================
DO $$
DECLARE
    col_count INTEGER;
    idx_count INTEGER;
BEGIN
    -- Check if limit column was successfully removed
    SELECT COUNT(*) INTO col_count
    FROM information_schema.columns
    WHERE table_schema = 'public'
    AND table_name = 'antiflood_settings'
    AND column_name = 'limit';

    -- Check if new index was created
    SELECT COUNT(*) INTO idx_count
    FROM pg_indexes
    WHERE tablename = 'antiflood_settings'
    AND indexname = 'idx_antiflood_chat_flood_active';

    RAISE NOTICE 'Antiflood column cleanup completed:';
    RAISE NOTICE '- limit column exists: %', (col_count > 0);
    RAISE NOTICE '- new index created: %', (idx_count > 0);

    IF col_count = 0 AND idx_count > 0 THEN
        RAISE NOTICE 'Migration completed successfully!';
    ELSE
        RAISE WARNING 'Migration may not have completed as expected';
    END IF;
END $$;
