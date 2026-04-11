-- =====================================================
-- Database Constraints Migration
-- =====================================================
-- Migration Name: add_database_constraints
-- Description: Add CHECK constraints for business logic validation
-- Risk: ZERO - New data only, no existing data impact
-- =====================================================

BEGIN;

-- =====================================================
-- WARNINGS SYSTEM CONSTRAINTS
-- =====================================================

-- Ensure warn limit is positive
ALTER TABLE warns_settings
ADD CONSTRAINT IF NOT EXISTS chk_warns_settings_limit
CHECK (warn_limit > 0);

-- Ensure num_warns is non-negative
ALTER TABLE warns_users
ADD CONSTRAINT IF NOT EXISTS chk_warns_users_num
CHECK (num_warns >= 0);

-- =====================================================
-- ANTIFLOOD CONSTRAINTS
-- =====================================================

-- Ensure flood limit is positive when set
ALTER TABLE antiflood_settings
ADD CONSTRAINT IF NOT EXISTS chk_antiflood_limit
CHECK (flood_limit IS NULL OR flood_limit >= 0);

-- Ensure valid action values
ALTER TABLE antiflood_settings
ADD CONSTRAINT IF NOT EXISTS chk_antiflood_action
CHECK (action IS NULL OR action IN ('mute', 'ban', 'kick', 'warn', 'tban', 'tmute'));

-- Ensure valid mode values (alias for action)
ALTER TABLE antiflood_settings
ADD CONSTRAINT IF NOT EXISTS chk_antiflood_mode
CHECK (mode IS NULL OR mode IN ('mute', 'ban', 'kick', 'warn', 'tban', 'tmute'));

-- =====================================================
-- BLACKLIST CONSTRAINTS
-- =====================================================

-- Ensure valid blacklist action values
ALTER TABLE blacklists
ADD CONSTRAINT IF NOT EXISTS chk_blacklists_action
CHECK (action IN ('warn', 'mute', 'ban', 'kick', 'tban', 'tmute', 'delete'));

-- =====================================================
-- CAPTCHA CONSTRAINTS
-- =====================================================

-- Ensure captcha timeout is positive and reasonable
ALTER TABLE captcha_settings
ADD CONSTRAINT IF NOT EXISTS chk_captcha_timeout
CHECK (timeout > 0 AND timeout <= 10);

-- Ensure valid captcha failure actions
ALTER TABLE captcha_settings
ADD CONSTRAINT IF NOT EXISTS chk_captcha_failure_action
CHECK (failure_action IN ('kick', 'ban', 'mute'));

-- Ensure captcha max attempts is reasonable
ALTER TABLE captcha_settings
ADD CONSTRAINT IF NOT EXISTS chk_captcha_max_attempts
CHECK (max_attempts > 0 AND max_attempts <= 10);

-- Ensure valid captcha mode
ALTER TABLE captcha_settings
ADD CONSTRAINT IF NOT EXISTS chk_captcha_mode
CHECK (captcha_mode IN ('math', 'text'));

-- Ensure captcha expires_at is after created_at
ALTER TABLE captcha_attempts
ADD CONSTRAINT IF NOT EXISTS chk_captcha_expires_at
CHECK (expires_at > created_at);

-- =====================================================
-- WARNINGS MODE CONSTRAINTS
-- =====================================================

-- Ensure valid warn mode values
ALTER TABLE warns_settings
ADD CONSTRAINT IF NOT EXISTS chk_warns_mode
CHECK (warn_mode IS NULL OR warn_mode IN ('ban', 'kick', 'mute', 'tban', 'tmute'));

COMMIT;

-- =====================================================
-- ROLLBACK INSTRUCTIONS
-- =====================================================
-- If you need to rollback this migration:
/*
BEGIN;

ALTER TABLE warns_settings DROP CONSTRAINT IF EXISTS chk_warns_settings_limit;
ALTER TABLE warns_users DROP CONSTRAINT IF EXISTS chk_warns_users_num;
ALTER TABLE antiflood_settings DROP CONSTRAINT IF EXISTS chk_antiflood_limit;
ALTER TABLE antiflood_settings DROP CONSTRAINT IF EXISTS chk_antiflood_action;
ALTER TABLE antiflood_settings DROP CONSTRAINT IF EXISTS chk_antiflood_mode;
ALTER TABLE blacklists DROP CONSTRAINT IF EXISTS chk_blacklists_action;
ALTER TABLE captcha_settings DROP CONSTRAINT IF EXISTS chk_captcha_timeout;
ALTER TABLE captcha_settings DROP CONSTRAINT IF EXISTS chk_captcha_failure_action;
ALTER TABLE captcha_settings DROP CONSTRAINT IF EXISTS chk_captcha_max_attempts;
ALTER TABLE captcha_settings DROP CONSTRAINT IF EXISTS chk_captcha_mode;
ALTER TABLE captcha_attempts DROP CONSTRAINT IF EXISTS chk_captcha_expires_at;
ALTER TABLE warns_settings DROP CONSTRAINT IF EXISTS chk_warns_mode;

COMMIT;
*/
