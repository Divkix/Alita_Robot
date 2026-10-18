-- 20250805204145_add_foreign_key_relations.sql created chk_blacklists_action
-- without 'none'. 20260412000000_add_missing_check_constraints.sql replaced
-- chk_blacklist_action (no "s"), which is a different constraint, so the
-- legacy one stayed and PostgreSQL rejected `/blaction none`.
--
-- chk_blacklist_action keeps validating blacklists.action with 'none' allowed.
-- Idempotent; the Go runner applies each file once inside a transaction and
-- records its sha256 in schema_migrations.

ALTER TABLE blacklists DROP CONSTRAINT IF EXISTS chk_blacklists_action;
