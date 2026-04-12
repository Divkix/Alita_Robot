-- =====================================================
-- Missing CHECK Constraints Migration
-- =====================================================
-- Migration Name: add_missing_check_constraints
-- Description: Add missing CHECK constraints to align runtime schema with app validation
-- =====================================================

BEGIN;

-- Captcha settings constraints
ALTER TABLE captcha_settings DROP CONSTRAINT IF EXISTS chk_captcha_timeout_range;
ALTER TABLE captcha_settings DROP CONSTRAINT IF EXISTS chk_captcha_max_attempts_range;
ALTER TABLE captcha_settings DROP CONSTRAINT IF EXISTS chk_captcha_mode;
ALTER TABLE captcha_settings DROP CONSTRAINT IF EXISTS chk_captcha_failure_action;

ALTER TABLE captcha_settings
ADD CONSTRAINT chk_captcha_timeout_range
CHECK (timeout BETWEEN 1 AND 10) NOT VALID;

ALTER TABLE captcha_settings
ADD CONSTRAINT chk_captcha_max_attempts_range
CHECK (max_attempts BETWEEN 1 AND 10) NOT VALID;

ALTER TABLE captcha_settings
ADD CONSTRAINT chk_captcha_mode
CHECK (captcha_mode IN ('math', 'text')) NOT VALID;

ALTER TABLE captcha_settings
ADD CONSTRAINT chk_captcha_failure_action
CHECK (failure_action IN ('kick', 'ban', 'mute')) NOT VALID;

-- Antiflood settings constraints
ALTER TABLE antiflood_settings DROP CONSTRAINT IF EXISTS chk_antiflood_action;
ALTER TABLE antiflood_settings DROP CONSTRAINT IF EXISTS chk_antiflood_mode;

ALTER TABLE antiflood_settings
ADD CONSTRAINT chk_antiflood_action
CHECK (action IN ('mute', 'ban', 'kick', 'warn', 'tban', 'tmute')) NOT VALID;

ALTER TABLE antiflood_settings
ADD CONSTRAINT chk_antiflood_mode
CHECK (mode = '' OR mode IN ('mute', 'ban', 'kick', 'warn', 'tban', 'tmute')) NOT VALID;

-- Warn settings constraints
ALTER TABLE warns_settings DROP CONSTRAINT IF EXISTS chk_warn_limit;
ALTER TABLE warns_settings DROP CONSTRAINT IF EXISTS chk_warn_mode;

ALTER TABLE warns_settings
ADD CONSTRAINT chk_warn_limit
CHECK (warn_limit > 0) NOT VALID;

ALTER TABLE warns_settings
ADD CONSTRAINT chk_warn_mode
CHECK (
  warn_mode IS NULL OR
  warn_mode = '' OR
  warn_mode IN ('ban', 'kick', 'mute', 'tban', 'tmute')
) NOT VALID;

-- Warn users constraints
ALTER TABLE warns_users DROP CONSTRAINT IF EXISTS chk_warns_num_warns;
ALTER TABLE warns_users
ADD CONSTRAINT chk_warns_num_warns
CHECK (num_warns >= 0) NOT VALID;

-- Blacklist settings constraints
ALTER TABLE blacklists DROP CONSTRAINT IF EXISTS chk_blacklist_action;
ALTER TABLE blacklists
ADD CONSTRAINT chk_blacklist_action
CHECK (action IN ('warn', 'mute', 'ban', 'kick', 'tban', 'tmute', 'delete', 'none')) NOT VALID;

COMMIT;

-- =====================================================
-- ROLLBACK INSTRUCTIONS
-- =====================================================
-- If you need to rollback this migration:
/*
BEGIN;

ALTER TABLE blacklists DROP CONSTRAINT IF EXISTS chk_blacklist_action;
ALTER TABLE warns_users DROP CONSTRAINT IF EXISTS chk_warns_num_warns;
ALTER TABLE warns_settings DROP CONSTRAINT IF EXISTS chk_warn_mode;
ALTER TABLE warns_settings DROP CONSTRAINT IF EXISTS chk_warn_limit;
ALTER TABLE antiflood_settings DROP CONSTRAINT IF EXISTS chk_antiflood_mode;
ALTER TABLE antiflood_settings DROP CONSTRAINT IF EXISTS chk_antiflood_action;
ALTER TABLE captcha_settings DROP CONSTRAINT IF EXISTS chk_captcha_failure_action;
ALTER TABLE captcha_settings DROP CONSTRAINT IF EXISTS chk_captcha_mode;
ALTER TABLE captcha_settings DROP CONSTRAINT IF EXISTS chk_captcha_max_attempts_range;
ALTER TABLE captcha_settings DROP CONSTRAINT IF EXISTS chk_captcha_timeout_range;

COMMIT;
*/
