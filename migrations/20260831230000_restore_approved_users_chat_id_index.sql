-- Restore idx_approved_users_chat_id on approved_users.
--
-- Dropped by 20260829120000_drop_unused_and_bloated_indexes.sql as "unused",
-- but PlanetScale Query Insights later showed ~5,400 sequential scans on
-- approved_users (chat_id lookups from the approval checks), each reading
-- the full table. This index serves those lookups.
--
-- Shipped as a migration rather than applied directly because the table is
-- owned by the app's own database role: admin-tier roles (CLI or console)
-- cannot CREATE INDEX on it (42501 must be owner), so the app itself applies
-- this on boot as the owner.
--
-- Non-concurrent on purpose: the migration runner wraps each file in one
-- transaction and CREATE INDEX CONCURRENTLY cannot run inside a transaction
-- block. The table is tiny (~92 rows), so the build-time lock is momentary.

CREATE INDEX IF NOT EXISTS idx_approved_users_chat_id ON approved_users(chat_id);
