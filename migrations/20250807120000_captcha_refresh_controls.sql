-- Add refresh_count to captcha_attempts to cap refreshes per attempt
ALTER TABLE IF EXISTS captcha_attempts
    ADD COLUMN IF NOT EXISTS refresh_count INTEGER DEFAULT 0;

-- Note: updated_at trigger already exists on captcha_attempts
