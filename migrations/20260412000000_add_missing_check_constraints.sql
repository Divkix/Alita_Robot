-- =====================================================
-- Missing CHECK Constraints Migration
-- =====================================================
-- Migration Name: add_missing_check_constraints
-- Description: Add CHECK constraints for data validation that were expected by tests but missing from schema
-- =====================================================

BEGIN;

-- Captcha settings constraints
ALTER TABLE captcha_settings
ADD CONSTRAINT chk_captcha_timeout_range CHECK (timeout BETWEEN 1 AND 10),
ADD CONSTRAINT chk_captcha_max_attempts_range CHECK (max_attempts BETWEEN 1 AND 10),
ADD CONSTRAINT chk_captcha_mode CHECK (mode IN ('text', 'button', 'math', 'captcha')),
ADD CONSTRAINT chk_captcha_failure_action CHECK (failure_action IN ('kick', 'ban', 'mute', 'tgbanchat', 'kickme'));

-- Antiflood settings constraints
ALTER TABLE antiflood_settings
ADD CONSTRAINT chk_antiflood_action CHECK (action IN ('mute', 'kick', 'ban', 'tgbanchat')),
ADD CONSTRAINT chk_antiflood_mode CHECK (mode IN ('user', 'group', 'all'));

-- Warn settings constraints
ALTER TABLE warn_settings
ADD CONSTRAINT chk_warn_mode CHECK (warn_mode IN ('none', 'warn1', 'warn2', 'warn3', 'warn4', 'warn5', 'warn6', 'warn7', 'warn8', 'warn9'));

-- Blacklist settings constraints
ALTER TABLE blacklist_settings
ADD CONSTRAINT chk_blacklist_action CHECK (action IN ('mute', 'kick', 'ban', 'tgbanchat'));

COMMIT;

-- =====================================================
-- ROLLBACK INSTRUCTIONS
-- =====================================================
-- If you need to rollback this migration:
/*
BEGIN;

ALTER TABLE blacklist_settings DROP CONSTRAINT IF EXISTS chk_blacklist_action;
ALTER TABLE warn_settings DROP CONSTRAINT IF EXISTS chk_warn_mode;
ALTER TABLE antiflood_settings DROP CONSTRAINT IF EXISTS chk_antiflood_mode;
ALTER TABLE antiflood_settings DROP CONSTRAINT IF EXISTS chk_antiflood_action;
ALTER TABLE captcha_settings DROP CONSTRAINT IF EXISTS chk_captcha_failure_action;
ALTER TABLE captcha_settings DROP CONSTRAINT IF EXISTS chk_captcha_mode;
ALTER TABLE captcha_settings DROP CONSTRAINT IF EXISTS chk_captcha_max_attempts_range;
ALTER TABLE captcha_settings DROP CONSTRAINT IF EXISTS chk_captcha_timeout_range;

COMMIT;
*/
