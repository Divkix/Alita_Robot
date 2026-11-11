-- Per-chat /ephemeral switch: answers Alita would otherwise redirect to PM
-- (private rules, private notes, the group /help redirect) are shown only
-- to the member who asked, in the group, through Bot API ephemeral messages.
--
-- A chat without a row is off, so nothing changes for a chat until an admin
-- runs /ephemeral on. Statements are idempotent; the Go runner and
-- scripts/migrate_psql.sh apply each file once inside a transaction and
-- record its sha256 in schema_migrations.

CREATE TABLE IF NOT EXISTS ephemeral_settings (
    chat_id BIGINT PRIMARY KEY,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Add foreign key to chats table when available (self-managed)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE constraint_name = 'fk_ephemeral_settings_chat') THEN
        ALTER TABLE ephemeral_settings DROP CONSTRAINT fk_ephemeral_settings_chat;
    END IF;

    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'chats') THEN
        ALTER TABLE ephemeral_settings
        ADD CONSTRAINT fk_ephemeral_settings_chat
        FOREIGN KEY (chat_id) REFERENCES chats(chat_id) ON DELETE CASCADE ON UPDATE CASCADE;
    END IF;
END $$;
