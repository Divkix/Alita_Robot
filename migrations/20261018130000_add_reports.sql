-- One row per member per reported message (/report and @admin).
--
-- Reports used to exist only as a group reply; this keeps who reported whom,
-- which message, and which admin closed it. status starts 'open' and moves
-- to a decision exactly once (the application updates only open rows).
-- admin_chat_id/admin_message_id point at the report card in the log channel,
-- which holds the copy of the message; the row keeps only its id and a short
-- snippet of its text. Closed reports are deleted 90 days after resolved_at.
--
-- Statements are idempotent; the Go runner applies each file once inside a
-- transaction and records its sha256 in schema_migrations.

CREATE TABLE IF NOT EXISTS reports (
    id BIGSERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL,
    reported_user_id BIGINT NOT NULL,
    reported_message_id BIGINT NOT NULL,
    reporter_id BIGINT NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    snippet VARCHAR(100) NOT NULL DEFAULT '',
    status VARCHAR(16) NOT NULL DEFAULT 'open',
    resolved_by BIGINT,
    resolved_at TIMESTAMP WITH TIME ZONE,
    admin_chat_id BIGINT,
    admin_message_id BIGINT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT uq_reports_chat_message_reporter UNIQUE (chat_id, reported_message_id, reporter_id),
    CONSTRAINT chk_reports_status CHECK (status IN ('open', 'banned', 'kicked', 'muted', 'deleted', 'dismissed'))
);

-- The unique constraint above serves lookups by message; this one serves the
-- retention sweep over closed reports.
CREATE INDEX IF NOT EXISTS idx_reports_resolved_at ON reports (resolved_at) WHERE status <> 'open';

-- Add foreign key to chats table when available (self-managed)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE constraint_name = 'fk_reports_chat') THEN
        ALTER TABLE reports DROP CONSTRAINT fk_reports_chat;
    END IF;

    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'chats') THEN
        ALTER TABLE reports
        ADD CONSTRAINT fk_reports_chat
        FOREIGN KEY (chat_id) REFERENCES chats(chat_id) ON DELETE CASCADE ON UPDATE CASCADE;
    END IF;
END $$;
