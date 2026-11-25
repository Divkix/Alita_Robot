-- chat_members is the normalized membership list behind chats.users.
--
-- chats.users is a jsonb array: every join rewrites the whole array (34ms for
-- a 200k-member supergroup), and "which chats is this user in?" can only be
-- answered by a sequential scan. One row per (chat, user) makes the join a
-- single insert under the primary key and gives the reverse lookup an index.
-- Membership is never capped: /unbanall and /zombies need every member.
--
-- chats.users stays for one release because older binaries still read it.
-- New code writes only chat_members, so the array is frozen at deploy time.
--
-- Rollback consequence: a binary rolled back to the jsonb reader sees the
-- members known when this migration shipped; anyone who joined since is
-- invisible to /unbanall and /zombies. Copy chat_members back into chats.users
-- before rolling back, and do not drop the table: it is the only complete
-- membership list after this deploy.
--
-- Applied once inside a transaction by the Go runner and scripts/migrate_psql.sh;
-- the statements are idempotent.

CREATE TABLE IF NOT EXISTS chat_members (
    chat_id BIGINT NOT NULL REFERENCES chats (chat_id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL,
    PRIMARY KEY (chat_id, user_id)
);

-- The reverse lookup uses (user_id, chat_id): the primary key starts with
-- chat_id, so it cannot serve "which of these chats contain user X"
-- (federations.ChatsContainingUser) or "which active chats is X in"
-- (chats.ListChatsWithUser).
CREATE INDEX IF NOT EXISTS idx_chat_members_user ON chat_members (user_id, chat_id);

-- Backfill the existing arrays. The CASE keeps a null, JSON null or malformed
-- chats.users from failing the migration; the numeric filter does the same for
-- a non-numeric element. ON CONFLICT makes a retry safe.
INSERT INTO chat_members (chat_id, user_id)
SELECT c.chat_id, member.value::bigint
FROM chats c
CROSS JOIN LATERAL jsonb_array_elements_text(
    CASE WHEN jsonb_typeof(c.users) = 'array' THEN c.users END
) AS member(value)
WHERE member.value ~ '^-?[0-9]+$'
ON CONFLICT DO NOTHING;
