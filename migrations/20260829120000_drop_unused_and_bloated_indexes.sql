-- Drop unused, redundant, and heavily bloated indexes.
--
-- Identified via PlanetScale insights (pg_stat_user_indexes, 0 scans over the
-- database lifetime) and index bloat inspection. Dropping these reduces storage,
-- cuts write overhead on every INSERT/UPDATE, and in the case of
-- idx_chats_activity_status eliminates a 94%-bloated index from the hot path.
--
-- idx_chats_activity_status was REINDEXed directly on the remote (94% bloat);
-- idx_chats_last_activity is dropped entirely because the partial index
-- idx_chats_activity_status already covers last_activity for active chats
-- and the chats table has only ~83 live rows.
--
-- Storage reclaimed: ~15.3 MB (idx_users_covering alone is 14 MB).
-- Federation indexes are redundant with their composite unique indexes:
--   idx_federation_admins_fed_id  ⊂ idx_fed_admins_fed_user (fed_id, user_id)
--   idx_federation_bans_fed_id    ⊂ idx_fed_bans_fed_user   (fed_id, user_id)
--   idx_federation_subs_fed_id    ⊂ idx_fed_subs_pair       (fed_id, subscribed_fed_id)
--   idx_federation_chats_fed_id  — not redundant but table is tiny; re-add if it grows
--   idx_federation_bans_user_id   — no user_id-only queries exist; composite covers all

DROP INDEX IF EXISTS idx_users_covering;
DROP INDEX IF EXISTS idx_filters_chat_keyword_lower;
DROP INDEX IF EXISTS idx_chats_last_activity;
DROP INDEX IF EXISTS idx_warns_users_chat_id;
DROP INDEX IF EXISTS idx_captcha_attempts_chat_id;
DROP INDEX IF EXISTS idx_antiraid_settings_chat_id;
DROP INDEX IF EXISTS idx_approved_users_chat_id;
DROP INDEX IF EXISTS idx_admin_settings_chat;
DROP INDEX IF EXISTS idx_federation_bans_user_id;
DROP INDEX IF EXISTS idx_federation_subs_fed_id;
DROP INDEX IF EXISTS idx_stored_attempt;
DROP INDEX IF EXISTS idx_stored_user_chat;
DROP INDEX IF EXISTS idx_federation_admins_fed_id;
DROP INDEX IF EXISTS idx_federation_chats_fed_id;
DROP INDEX IF EXISTS idx_federation_bans_fed_id;

-- Update planner statistics after index removal.
ANALYZE users;
ANALYZE chats;
ANALYZE filters;
ANALYZE federation_chats;
ANALYZE federation_bans;
ANALYZE federation_admins;
ANALYZE federation_subs;
