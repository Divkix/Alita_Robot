-- Recurring messages: content an admin set to be posted in a chat every
-- interval (/recurring).
--
-- The content columns match notes (text with the notes directives, file_id,
-- msg_type, buttons) so the same media sender posts it. next_run_at is the
-- exactly-once claim: a replica posts an occurrence only when its conditional
-- update moving next_run_at forward wins, so two replicas never both post it.
-- ends_at, when set, stops the message after that instant. last_message_id is
-- the message the last occurrence posted, which delete_previous removes once
-- the next one is out. enabled turns false when the message ended or when a
-- send failed in a way retrying cannot fix (the bot was removed, the chat is
-- gone, or the topic was deleted).
--
-- Statements are idempotent; the Go runner and scripts/migrate_psql.sh apply
-- each file once inside a transaction and record its sha256 in
-- schema_migrations.

CREATE TABLE IF NOT EXISTS recurring_messages (
    id BIGSERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL,
    thread_id BIGINT NOT NULL DEFAULT 0,
    text TEXT NOT NULL DEFAULT '',
    file_id TEXT NOT NULL DEFAULT '',
    msg_type INTEGER NOT NULL DEFAULT 1,
    buttons JSONB,
    interval_seconds BIGINT NOT NULL,
    next_run_at TIMESTAMP WITH TIME ZONE NOT NULL,
    ends_at TIMESTAMP WITH TIME ZONE NULL,
    delete_previous BOOLEAN NOT NULL DEFAULT FALSE,
    last_message_id BIGINT NOT NULL DEFAULT 0,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_by BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT chk_recurring_messages_interval CHECK (interval_seconds >= 3600)
);

-- /recurring list and the per-chat limit read a chat's rows.
CREATE INDEX IF NOT EXISTS idx_recurring_messages_chat_id ON recurring_messages (chat_id);

-- The one-minute ticker reads the enabled rows that are due.
CREATE INDEX IF NOT EXISTS idx_recurring_messages_next_run_at ON recurring_messages (next_run_at) WHERE enabled;

-- Add foreign key to chats table when available (self-managed)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE constraint_name = 'fk_recurring_messages_chat') THEN
        ALTER TABLE recurring_messages DROP CONSTRAINT fk_recurring_messages_chat;
    END IF;

    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'chats') THEN
        ALTER TABLE recurring_messages
        ADD CONSTRAINT fk_recurring_messages_chat
        FOREIGN KEY (chat_id) REFERENCES chats(chat_id) ON DELETE CASCADE ON UPDATE CASCADE;
    END IF;
END $$;
