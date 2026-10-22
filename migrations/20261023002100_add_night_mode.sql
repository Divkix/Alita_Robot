-- Night mode: lock types the locks delete path enforces every day between two
-- times in the chat's timezone (chat_timezones, UTC without a row).
--
-- A chat without a row, or with enabled = FALSE, is untouched. start_minute
-- and end_minute are minutes after local midnight; a window whose end is
-- before its start crosses midnight. They must differ: an empty or all-day
-- window is refused in Go before it is stored. lock_types is a sorted,
-- comma-separated list of lock names as /lock takes them. Statements are
-- idempotent; the Go runner and scripts/migrate_psql.sh apply each file once
-- inside a transaction and record its sha256 in schema_migrations.

CREATE TABLE IF NOT EXISTS night_mode_settings (
    chat_id BIGINT PRIMARY KEY,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    start_minute INTEGER NOT NULL DEFAULT 0,
    end_minute INTEGER NOT NULL DEFAULT 0,
    lock_types TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT chk_night_mode_settings_start CHECK (start_minute BETWEEN 0 AND 1439),
    CONSTRAINT chk_night_mode_settings_end CHECK (end_minute BETWEEN 0 AND 1439),
    CONSTRAINT chk_night_mode_settings_lock_types CHECK (char_length(lock_types) <= 512)
);

-- Add foreign key to chats table when available (self-managed)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE constraint_name = 'fk_night_mode_settings_chat') THEN
        ALTER TABLE night_mode_settings DROP CONSTRAINT fk_night_mode_settings_chat;
    END IF;

    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'chats') THEN
        ALTER TABLE night_mode_settings
        ADD CONSTRAINT fk_night_mode_settings_chat
        FOREIGN KEY (chat_id) REFERENCES chats(chat_id) ON DELETE CASCADE ON UPDATE CASCADE;
    END IF;
END $$;
