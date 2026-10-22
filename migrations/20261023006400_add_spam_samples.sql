-- Texts admins labelled, and each chat's choice to match new messages
-- against them.
--
-- spam_samples holds one row per text per chat: a /spam stores the replied
-- message as 'spam', and an unban of a user /spam banned, or a "not spam"
-- press on a match the log channel showed, turns it into 'ham'. normalized is
-- the text matching compares (NFKC, look-alike letters folded, lower case, no
-- punctuation) and content_hash its sha256, so the same text flagged twice in
-- a chat updates one row. text is the message as sent, bounded by the
-- application, and is shown back only to the chat's admins.
--
-- sender_id is the user whose message it was, so an unban finds the samples
-- their ban stored. owner_id and admin_ids (sorted, comma-separated human
-- admins, owner_id and added_by included) are the chat's owner and admins when
-- the text was flagged: a text joins the corpus every chat matches against
-- only when two chats with different owners and no admin in common flagged
-- it. owner_id 0 means the owner was unknown, and such a row never counts.
--
-- Rows older than 90 days are deleted by the retention job, and the
-- application keeps at most 200 rows of each label per chat.
--
-- spam_filter_settings is the per-chat switch: mode 'off' (the default),
-- 'dryrun' (log matches to the log channel, act on none) or 'on' (delete a
-- match, after seven days of dry run counted from enabled_at), and the
-- similarity a match needs.
--
-- Statements are idempotent; the Go runner applies each file once inside a
-- transaction and records its sha256 in schema_migrations.

CREATE TABLE IF NOT EXISTS spam_samples (
    id BIGSERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL,
    label TEXT NOT NULL DEFAULT 'spam',
    text TEXT NOT NULL,
    normalized TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    sender_id BIGINT NOT NULL DEFAULT 0,
    added_by BIGINT NOT NULL DEFAULT 0,
    owner_id BIGINT NOT NULL DEFAULT 0,
    admin_ids TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_spam_samples_label CHECK (label IN ('spam', 'ham'))
);

-- One row per text per chat: a later label replaces an earlier one.
CREATE UNIQUE INDEX IF NOT EXISTS uq_spam_samples_chat_hash
    ON spam_samples (chat_id, content_hash);
-- A chat's samples newest first (matching, /spamsamples and the per-label cap).
CREATE INDEX IF NOT EXISTS idx_spam_samples_chat_label_created_at
    ON spam_samples (chat_id, label, created_at);
-- The shared corpus groups spam rows by text across chats.
CREATE INDEX IF NOT EXISTS idx_spam_samples_hash
    ON spam_samples (content_hash);
-- The retention sweep deletes by age alone.
CREATE INDEX IF NOT EXISTS idx_spam_samples_created_at
    ON spam_samples (created_at);

CREATE TABLE IF NOT EXISTS spam_filter_settings (
    chat_id BIGINT PRIMARY KEY,
    mode TEXT NOT NULL DEFAULT 'off',
    threshold DOUBLE PRECISION NOT NULL DEFAULT 0.85,
    enabled_at TIMESTAMP WITH TIME ZONE NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT chk_spam_filter_settings_mode CHECK (mode IN ('off', 'dryrun', 'on')),
    CONSTRAINT chk_spam_filter_settings_threshold CHECK (threshold >= 0.5 AND threshold <= 1)
);

-- Both tables belong to a chat, so a deleted chat leaves no orphan rows.
-- sender_id, added_by and owner_id keep no foreign key: a sample outlives the
-- users it names until retention removes it.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE constraint_name = 'fk_spam_samples_chat') THEN
        ALTER TABLE spam_samples DROP CONSTRAINT fk_spam_samples_chat;
    END IF;
    IF EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE constraint_name = 'fk_spam_filter_settings_chat') THEN
        ALTER TABLE spam_filter_settings DROP CONSTRAINT fk_spam_filter_settings_chat;
    END IF;

    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'chats') THEN
        ALTER TABLE spam_samples
        ADD CONSTRAINT fk_spam_samples_chat
        FOREIGN KEY (chat_id) REFERENCES chats(chat_id) ON DELETE CASCADE ON UPDATE CASCADE;
        ALTER TABLE spam_filter_settings
        ADD CONSTRAINT fk_spam_filter_settings_chat
        FOREIGN KEY (chat_id) REFERENCES chats(chat_id) ON DELETE CASCADE ON UPDATE CASCADE;
    END IF;
END $$;
