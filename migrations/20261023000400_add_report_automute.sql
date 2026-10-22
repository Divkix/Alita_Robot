-- Report auto-mute (/reports mute <N>).
--
-- mute_threshold is the number of different members whose open reports
-- against one user mute that user in the chat; 0 (the default, so every
-- existing chat is unchanged) turns the feature off.
--
-- report_auto_mutes holds one row per user the feature muted and no admin
-- has decided on yet. muted_until is the until_date sent to Telegram (at most
-- 24 hours after the mute), so the mute ends on its own even if the bot is
-- down; the row tells an admin decision that this mute, and not one an admin
-- set, may be lifted, and keeps the same reports from muting the user again.
-- Rows are deleted by an admin decision, or 90 days after muted_until.
--
-- Statements are idempotent; the Go runner and scripts/migrate_psql.sh apply
-- each file once inside a transaction and record its sha256 in
-- schema_migrations.

ALTER TABLE report_chat_settings
ADD COLUMN IF NOT EXISTS mute_threshold INTEGER NOT NULL DEFAULT 0;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE constraint_name = 'chk_report_chat_settings_mute_threshold') THEN
        ALTER TABLE report_chat_settings
        ADD CONSTRAINT chk_report_chat_settings_mute_threshold
        CHECK (mute_threshold = 0 OR mute_threshold BETWEEN 2 AND 20);
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS report_auto_mutes (
    chat_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    muted_until TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    PRIMARY KEY (chat_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_report_auto_mutes_muted_until ON report_auto_mutes (muted_until);

-- Counts the different members with open reports against one user.
CREATE INDEX IF NOT EXISTS idx_reports_open_target ON reports (chat_id, reported_user_id, reporter_id) WHERE status = 'open';

-- Add foreign key to chats table when available (self-managed)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE constraint_name = 'fk_report_auto_mutes_chat') THEN
        ALTER TABLE report_auto_mutes DROP CONSTRAINT fk_report_auto_mutes_chat;
    END IF;

    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'chats') THEN
        ALTER TABLE report_auto_mutes
        ADD CONSTRAINT fk_report_auto_mutes_chat
        FOREIGN KEY (chat_id) REFERENCES chats(chat_id) ON DELETE CASCADE ON UPDATE CASCADE;
    END IF;
END $$;
