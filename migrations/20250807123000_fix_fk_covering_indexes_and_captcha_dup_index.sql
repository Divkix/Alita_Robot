-- =====================================================
-- Supabase Migration: Fix FK Covering Indexes & Drop Duplicate Captcha Index
-- =====================================================
-- Migration Name: fix_fk_covering_indexes_and_captcha_dup_index
-- Description:
--   - Drops duplicate unique index on captcha_settings.chat_id
--     (keeps the constraint-backed index captcha_settings_chat_id_key)
--   - Ensures covering indexes exist for the following foreign keys:
--       * fk_channels_channel   -> channels(channel_id)
--       * fk_chat_users_user    -> chat_users(user_id)
--       * fk_connection_chat    -> connection(chat_id)
-- Date: 2025-08-07
-- =====================================================

BEGIN;

-- =====================================================
-- 1) Remove duplicate unique index on captcha_settings.chat_id
--    The UNIQUE column definition created constraint/index
--    captcha_settings_chat_id_key. The explicit index
--    uk_captcha_settings_chat_id is redundant.
-- =====================================================

DROP INDEX IF EXISTS public.uk_captcha_settings_chat_id;

-- =====================================================
-- 2) Ensure covering indexes for critical foreign keys
-- =====================================================

-- channels.channel_id covers fk_channels_channel
CREATE INDEX IF NOT EXISTS idx_channels_channel_id
ON public.channels(channel_id);
COMMENT ON INDEX idx_channels_channel_id IS 'Covering index for foreign key fk_channels_channel';

-- chat_users.user_id covers fk_chat_users_user
CREATE INDEX IF NOT EXISTS idx_chat_users_user_id
ON public.chat_users(user_id);
COMMENT ON INDEX idx_chat_users_user_id IS 'Covering index for foreign key fk_chat_users_user';

-- connection.chat_id covers fk_connection_chat
CREATE INDEX IF NOT EXISTS idx_connection_chat_id
ON public.connection(chat_id);
COMMENT ON INDEX idx_connection_chat_id IS 'Covering index for foreign key fk_connection_chat';

-- Update statistics for planner
ANALYZE channels;
ANALYZE chat_users;
ANALYZE connection;

COMMIT;

-- =====================================================
-- VERIFICATION QUERIES (run manually after migration)
-- =====================================================
-- 1) Confirm duplicate captcha index is gone and constraint-backed remains
-- SELECT indexname FROM pg_indexes WHERE tablename = 'captcha_settings';
--
-- 2) Confirm covering indexes exist
-- SELECT tablename, indexname FROM pg_indexes
--  WHERE schemaname = 'public'
--    AND indexname IN ('idx_channels_channel_id','idx_chat_users_user_id','idx_connection_chat_id')
--  ORDER BY tablename, indexname;
