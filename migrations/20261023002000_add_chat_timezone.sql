-- Per-chat timezone, an IANA name such as 'Asia/Kolkata'.
--
-- A chat without a row is on UTC, so nothing changes for a chat until an
-- admin runs /timezone. Features that act on the chat's local time (night
-- mode, schedules) read it through alita/db/chattz. The name is validated in
-- Go against the time zone database before it is stored; the CHECK only
-- keeps out empty and oversized values. Statements are idempotent; the Go
-- runner and scripts/migrate_psql.sh apply each file once inside a
-- transaction and record its sha256 in schema_migrations.

CREATE TABLE IF NOT EXISTS chat_timezones (
    chat_id BIGINT PRIMARY KEY,
    timezone TEXT NOT NULL DEFAULT 'UTC',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT chk_chat_timezones_timezone CHECK (char_length(timezone) BETWEEN 1 AND 64)
);

-- Add foreign key to chats table when available (self-managed)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE constraint_name = 'fk_chat_timezones_chat') THEN
        ALTER TABLE chat_timezones DROP CONSTRAINT fk_chat_timezones_chat;
    END IF;

    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'chats') THEN
        ALTER TABLE chat_timezones
        ADD CONSTRAINT fk_chat_timezones_chat
        FOREIGN KEY (chat_id) REFERENCES chats(chat_id) ON DELETE CASCADE ON UPDATE CASCADE;
    END IF;
END $$;
