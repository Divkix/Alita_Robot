-- Restore a GIN index on chats.users for the private /settings group picker.
--
-- 20250806105636_drop_unused_indexes.sql dropped idx_chats_users_gin with
-- "0 scans": nothing queried the column by containment then. The picker's
-- chats.ListChatsWithUser now does (users @> to_jsonb($1::bigint)); without
-- an index that is a sequential scan that de-TOASTs every chat's users array.
--
-- jsonb_path_ops: the query only uses @>, and this opclass builds a smaller
-- index than the default jsonb_ops the original used.
--
-- Non-concurrent on purpose: the migration runner wraps each file in one
-- transaction and CREATE INDEX CONCURRENTLY cannot run inside a transaction
-- block, so the build takes a short write lock on chats.

CREATE INDEX IF NOT EXISTS idx_chats_users_gin ON chats USING gin (users jsonb_path_ops);
