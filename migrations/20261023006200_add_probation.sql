-- New-member probation (/probation).
--
-- probation_settings holds one chat's choice. A missing row, and the
-- defaults below, mean probation is off, so every existing chat is unchanged.
-- duration_seconds is how long a member stays on probation after joining;
-- message_count is how many distinct messages of at least min_length
-- characters end it sooner. block_* choose what a member on probation may not
-- post; notice sends the member one notice per chat when a message is deleted.
--
-- probation_members holds one row per member who joined while probation was
-- on and has not finished it yet. Members of the chat before probation was
-- turned on never get a row. A row is deleted when the member finishes
-- probation or leaves, and by the retention sweep once it is older than the
-- longest allowed duration. The message count lives in Redis, never here.
--
-- Statements are idempotent; the Go runner and scripts/migrate_psql.sh apply
-- each file once inside a transaction and record its sha256 in
-- schema_migrations.

CREATE TABLE IF NOT EXISTS probation_settings (
    chat_id BIGINT PRIMARY KEY,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    duration_seconds INTEGER NOT NULL DEFAULT 86400,
    message_count INTEGER NOT NULL DEFAULT 5,
    min_length INTEGER NOT NULL DEFAULT 20,
    block_links BOOLEAN NOT NULL DEFAULT TRUE,
    block_forwards BOOLEAN NOT NULL DEFAULT TRUE,
    block_media BOOLEAN NOT NULL DEFAULT FALSE,
    notice BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT chk_probation_settings_duration CHECK (duration_seconds BETWEEN 3600 AND 2592000),
    CONSTRAINT chk_probation_settings_message_count CHECK (message_count BETWEEN 1 AND 100),
    CONSTRAINT chk_probation_settings_min_length CHECK (min_length BETWEEN 1 AND 500)
);

CREATE TABLE IF NOT EXISTS probation_members (
    chat_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    joined_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    notified BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (chat_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_probation_members_joined_at ON probation_members (joined_at);

-- Add foreign keys to chats table when available (self-managed)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE constraint_name = 'fk_probation_settings_chat') THEN
        ALTER TABLE probation_settings DROP CONSTRAINT fk_probation_settings_chat;
    END IF;
    IF EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE constraint_name = 'fk_probation_members_chat') THEN
        ALTER TABLE probation_members DROP CONSTRAINT fk_probation_members_chat;
    END IF;

    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'chats') THEN
        ALTER TABLE probation_settings
        ADD CONSTRAINT fk_probation_settings_chat
        FOREIGN KEY (chat_id) REFERENCES chats(chat_id) ON DELETE CASCADE ON UPDATE CASCADE;
        ALTER TABLE probation_members
        ADD CONSTRAINT fk_probation_members_chat
        FOREIGN KEY (chat_id) REFERENCES chats(chat_id) ON DELETE CASCADE ON UPDATE CASCADE;
    END IF;
END $$;
