-- Migration: Consolidate duplicate boolean fields
-- Issue 2 fix: Remove duplicate columns that store the same data

-- 1. Consolidate devs table: sync dev -> is_dev, then drop dev column
UPDATE devs SET is_dev = dev WHERE dev IS DISTINCT FROM is_dev;
ALTER TABLE devs DROP COLUMN IF EXISTS dev;

-- 2. Drop antiflood mode column (action is the canonical field)
-- Note: Mode was an alias for Action, code only uses Action
ALTER TABLE antiflood_settings DROP COLUMN IF EXISTS mode;

-- 3. Drop related check constraints that referenced the dropped columns
ALTER TABLE antiflood_settings DROP CONSTRAINT IF EXISTS chk_antiflood_mode;
