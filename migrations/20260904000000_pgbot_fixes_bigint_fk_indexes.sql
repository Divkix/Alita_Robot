-- pgbot findings (2026-09-04): widen sequence-backed int4 ids to bigint,
-- back 9 FKs with single-col indexes, drop 3 redundant prefix indexes,
-- and leave HOT headroom on high-update tables.
-- Every statement is idempotent. The Go runner applies each file once inside
-- a transaction (no CONCURRENTLY here) and records sha256 in schema_migrations.

-- 1. captcha_settings.id / captcha_attempts.id: integer -> bigint.
-- Tables are KBs; the rewrite holds ACCESS EXCLUSIVE briefly.
-- The sequences must change type too, or inserts still cap at 2.1B rows.
ALTER TABLE public.captcha_settings ALTER COLUMN id TYPE BIGINT;
ALTER SEQUENCE public.captcha_settings_id_seq AS BIGINT;
ALTER TABLE public.captcha_attempts ALTER COLUMN id TYPE BIGINT;
ALTER SEQUENCE public.captcha_attempts_id_seq AS BIGINT;

-- 2. FK-backing indexes. The surviving composite/unique constraints lead with
-- a different column, so they cannot back these FKs (parent DELETE/UPDATE
-- would seq-scan the child while holding the lock).
CREATE INDEX IF NOT EXISTS idx_warns_users_chat_id ON public.warns_users (chat_id);
CREATE INDEX IF NOT EXISTS idx_captcha_muted_users_chat_id ON public.captcha_muted_users (chat_id);
CREATE INDEX IF NOT EXISTS idx_connection_chat_id ON public.connection (chat_id);
CREATE INDEX IF NOT EXISTS idx_captcha_attempts_chat_id ON public.captcha_attempts (chat_id);
CREATE INDEX IF NOT EXISTS idx_stored_attempt ON public.stored_messages (attempt_id);
CREATE INDEX IF NOT EXISTS idx_federation_admins_user_id ON public.federation_admins (user_id);
CREATE INDEX IF NOT EXISTS idx_federation_bans_user_id ON public.federation_bans (user_id);
CREATE INDEX IF NOT EXISTS idx_federation_chats_fed_id ON public.federation_chats (fed_id);
CREATE INDEX IF NOT EXISTS idx_federation_subs_subscribed_fed_id ON public.federation_subs (subscribed_fed_id);

-- 3. Drop redundant single-col indexes: each is a strict left-prefix of a
-- wider index on the same table that already serves the same lookups.
DROP INDEX IF EXISTS public.idx_warns_users_user_id;
DROP INDEX IF EXISTS public.idx_filters_chat_optimized;
DROP INDEX IF EXISTS public.idx_reactions_chat_id;

-- 4. HOT headroom: users/chats sit at 0-1% HOT ratio over ~600k updates
-- (per-message upserts touch indexed columns). fillfactor reserves page
-- space for HOT updates going forward; metadata-only, no rewrite.
ALTER TABLE public.users SET (fillfactor = 85);
ALTER TABLE public.chats SET (fillfactor = 85);
