-- One row per human-initiated moderation action with a target: ban, kick,
-- mute and unban. The command variants fold into these four (tban, dban and
-- sban are bans; dkick and skick are kicks; tmute, dmute and smute are mutes),
-- /unmute and the self-target commands record nothing, and automated actions
-- never write here.
--
-- actor_id is the account that ran the command; for an anonymous admin it is
-- the group anonymous bot, the same account the log channel names. reason is
-- the moderator's own text, bounded by the application, and never message
-- content. The table feeds the shared reputation signal and the dashboard
-- timeline; rows older than 180 days are deleted by the retention job.
--
-- Statements are idempotent; the Go runner applies each file once inside a
-- transaction and records its sha256 in schema_migrations.

CREATE TABLE IF NOT EXISTS moderation_events (
    id BIGSERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL,
    actor_id BIGINT NOT NULL,
    target_user_id BIGINT NOT NULL,
    action TEXT NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_moderation_events_action CHECK (action IN ('ban', 'kick', 'mute', 'unban'))
);

-- One index per lookup: a target's actions across chats (the reputation
-- window) and a chat's timeline, both read newest first.
CREATE INDEX IF NOT EXISTS idx_moderation_events_target_created_at
    ON moderation_events (target_user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_moderation_events_chat_created_at
    ON moderation_events (chat_id, created_at);

-- Keep the chat foreign key like reports keeps its own, so a deleted chat
-- leaves no orphan rows. actor_id and target_user_id keep no foreign key:
-- this is evidence, and retention, not a user-row cascade, removes it.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE constraint_name = 'fk_moderation_events_chat') THEN
        ALTER TABLE moderation_events DROP CONSTRAINT fk_moderation_events_chat;
    END IF;

    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'chats') THEN
        ALTER TABLE moderation_events
        ADD CONSTRAINT fk_moderation_events_chat
        FOREIGN KEY (chat_id) REFERENCES chats(chat_id) ON DELETE CASCADE ON UPDATE CASCADE;
    END IF;
END $$;
