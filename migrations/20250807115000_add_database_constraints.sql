-- =====================================================
-- Database Constraints Migration
-- =====================================================
-- Migration Name: add_database_constraints
-- Description: Align antiflood constraint with app behavior and add captcha expiry validation
-- =====================================================

BEGIN;

-- Allow antiflood to be explicitly disabled with flood_limit = 0.
ALTER TABLE antiflood_settings
DROP CONSTRAINT IF EXISTS chk_antiflood_limit;

ALTER TABLE antiflood_settings
ADD CONSTRAINT chk_antiflood_limit
CHECK (flood_limit >= 0) NOT VALID;

ALTER TABLE antiflood_settings
VALIDATE CONSTRAINT chk_antiflood_limit;

-- Ensure captcha attempts always expire after they are created.
ALTER TABLE captcha_attempts
DROP CONSTRAINT IF EXISTS chk_captcha_expires_at;

ALTER TABLE captcha_attempts
ADD CONSTRAINT chk_captcha_expires_at
CHECK (expires_at > created_at) NOT VALID;

ALTER TABLE captcha_attempts
VALIDATE CONSTRAINT chk_captcha_expires_at;

COMMIT;

-- =====================================================
-- ROLLBACK INSTRUCTIONS
-- =====================================================
-- If you need to rollback this migration:
/*
BEGIN;

ALTER TABLE captcha_attempts DROP CONSTRAINT IF EXISTS chk_captcha_expires_at;
ALTER TABLE antiflood_settings DROP CONSTRAINT IF EXISTS chk_antiflood_limit;
ALTER TABLE antiflood_settings
ADD CONSTRAINT chk_antiflood_limit CHECK (flood_limit > 0);

COMMIT;
*/
