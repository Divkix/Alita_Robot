-- =====================================================
-- CHAT_USERS TABLE CLEANUP MIGRATION
-- =====================================================
-- This migration removes the unused chat_users join table.
--
-- Background: The Chat model has both:
-- 1. A JSONB 'users' field (actively used by the application)
-- 2. A many2many relationship via chat_users table (never used)
--
-- The JSONB approach is preferred and actively maintained,
-- while the join table remains empty and unused, creating
-- maintenance overhead with no benefit.

-- =====================================================
-- Step 1: Log current state for audit purposes
-- =====================================================
DO $$
DECLARE
    table_exists BOOLEAN;
    row_count INTEGER;
    index_count INTEGER;
BEGIN
    -- Check if table exists
    SELECT EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_schema = 'public'
        AND table_name = 'chat_users'
    ) INTO table_exists;

    IF table_exists THEN
        -- Count rows in chat_users table
        EXECUTE 'SELECT COUNT(*) FROM chat_users' INTO row_count;

        -- Count indexes on chat_users table
        SELECT COUNT(*) INTO index_count
        FROM pg_indexes
        WHERE tablename = 'chat_users';

        RAISE NOTICE 'chat_users table cleanup starting:';
        RAISE NOTICE '- Table exists: %', table_exists;
        RAISE NOTICE '- Row count: %', row_count;
        RAISE NOTICE '- Index count: %', index_count;
    ELSE
        RAISE NOTICE 'chat_users table does not exist, migration not needed';
    END IF;
END $$;

-- =====================================================
-- Step 2: Drop foreign key constraints first
-- =====================================================
-- Drop any foreign key constraints pointing to or from chat_users
DO $$
BEGIN
    -- Drop foreign key constraints if they exist
    IF EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE constraint_type = 'FOREIGN KEY'
        AND table_name = 'chat_users'
    ) THEN
        -- Drop constraints dynamically
        DECLARE
            constraint_record RECORD;
        BEGIN
            FOR constraint_record IN
                SELECT constraint_name
                FROM information_schema.table_constraints
                WHERE constraint_type = 'FOREIGN KEY'
                AND table_name = 'chat_users'
            LOOP
                EXECUTE 'ALTER TABLE chat_users DROP CONSTRAINT IF EXISTS ' || constraint_record.constraint_name;
                RAISE NOTICE 'Dropped constraint: %', constraint_record.constraint_name;
            END LOOP;
        END;
    END IF;
END $$;

-- =====================================================
-- Step 3: Drop all indexes on chat_users table
-- =====================================================
DO $$
DECLARE
    index_record RECORD;
BEGIN
    -- Drop all indexes on chat_users table
    FOR index_record IN
        SELECT indexname
        FROM pg_indexes
        WHERE tablename = 'chat_users'
        AND schemaname = 'public'
    LOOP
        EXECUTE 'DROP INDEX IF EXISTS ' || index_record.indexname;
        RAISE NOTICE 'Dropped index: %', index_record.indexname;
    END LOOP;
END $$;

-- =====================================================
-- Step 4: Drop the chat_users table
-- =====================================================
DROP TABLE IF EXISTS chat_users CASCADE;

-- =====================================================
-- Step 5: Clean up any remaining references in migrations table
-- =====================================================
-- Note: We don't need to clean up migrations.go index mappings here
-- as that will be done in a separate commit to the Go code

-- =====================================================
-- Validation and logging
-- =====================================================
DO $$
DECLARE
    table_exists BOOLEAN;
    remaining_indexes INTEGER;
BEGIN
    -- Verify table was dropped
    SELECT EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_schema = 'public'
        AND table_name = 'chat_users'
    ) INTO table_exists;

    -- Count any remaining indexes that reference chat_users
    SELECT COUNT(*) INTO remaining_indexes
    FROM pg_indexes
    WHERE tablename = 'chat_users';

    RAISE NOTICE 'chat_users cleanup completed:';
    RAISE NOTICE '- Table exists: %', table_exists;
    RAISE NOTICE '- Remaining indexes: %', remaining_indexes;

    IF NOT table_exists AND remaining_indexes = 0 THEN
        RAISE NOTICE 'Migration completed successfully!';
        RAISE NOTICE 'Chat membership is now managed exclusively via JSONB users field';
    ELSE
        RAISE WARNING 'Migration may not have completed as expected';
    END IF;
END $$;
