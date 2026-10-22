-- One row per ban the bot issues in a chat (/ban and its variants, the
-- restrict, report and join-request buttons, warn limit, antiflood,
-- blacklists, antiraid, captcha, federation bans, the bots lock and the spam
-- databases). Bans made in Telegram's own interface are not recorded, and the
-- table starts empty: there is no way to read a chat's earlier bans.
--
-- banned_by is the account that issued the ban, 0 for a ban the bot made on
-- its own. source names the path; source_detail adds what the path needs (the
-- federation ID of a federation ban, whose reason stays in federation_bans).
-- until is the end of a temporary ban, NULL for a permanent one; an open row
-- past until counts as expired. lifted_at and lifted_by are set when the bot
-- unbans the user. Only IDs are kept: names are read from users when shown.
--
-- A chat holds at most one open row per user (lifted_at IS NULL): a repeat
-- ban updates it. Lifted rows are deleted 90 days after lifted_at, expired
-- temporary bans 90 days after until; open permanent bans are kept.
--
-- Statements are idempotent; the Go runner applies each file once inside a
-- transaction and records its sha256 in schema_migrations.

CREATE TABLE IF NOT EXISTS ban_records (
    id BIGSERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    banned_by BIGINT NOT NULL DEFAULT 0,
    reason TEXT NOT NULL DEFAULT '',
    source VARCHAR(16) NOT NULL,
    source_detail VARCHAR(64) NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    until TIMESTAMP WITH TIME ZONE,
    lifted_at TIMESTAMP WITH TIME ZONE,
    lifted_by BIGINT,
    CONSTRAINT chk_ban_records_source CHECK (source IN (
        'manual', 'warn', 'antiflood', 'blacklist', 'antiraid', 'captcha',
        'fed', 'lock', 'report', 'join_request', 'spamdb'
    ))
);

-- The open row of a user in a chat: unique, and the lookup /baninfo, the
-- unban paths and the open-ban list read.
CREATE UNIQUE INDEX IF NOT EXISTS uq_ban_records_open
    ON ban_records (chat_id, user_id) WHERE lifted_at IS NULL;
-- A user's rows in a chat, newest first, for /baninfo when none is open.
CREATE INDEX IF NOT EXISTS idx_ban_records_chat_user_id
    ON ban_records (chat_id, user_id, id);
-- The retention sweep over lifted rows and expired temporary bans.
CREATE INDEX IF NOT EXISTS idx_ban_records_lifted_at
    ON ban_records (lifted_at) WHERE lifted_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_ban_records_open_until
    ON ban_records (until) WHERE lifted_at IS NULL AND until IS NOT NULL;

-- Keep the chat foreign key like reports and moderation_events keep theirs,
-- so a deleted chat leaves no orphan rows. user_id, banned_by and lifted_by
-- keep none: a banned user often has no users row, and retention, not a
-- user-row cascade, removes the record.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE constraint_name = 'fk_ban_records_chat') THEN
        ALTER TABLE ban_records DROP CONSTRAINT fk_ban_records_chat;
    END IF;

    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'chats') THEN
        ALTER TABLE ban_records
        ADD CONSTRAINT fk_ban_records_chat
        FOREIGN KEY (chat_id) REFERENCES chats(chat_id) ON DELETE CASCADE ON UPDATE CASCADE;
    END IF;
END $$;
