# Design: move `chats.users` (JSONB array) to a `chat_users` table

- Issue: #875 (part of #862)
- Status: proposed. Nothing ships until a maintainer approves this design on #875.
- Scope: research and design only. This document adds no migration and no Go code.
- Base commit: `de76475f` (`perf/863-metrics`). Line numbers below refer to that commit.

This file sits outside `docs/src/content/docs/`, so the docs generator and the Blume site do not
read it, and `make check-docs` does not check it.

## Background

`chats.users` holds the IDs of every sender the bot has seen in a group, as a JSONB array
(`models.Int64Array`). It costs more as a group grows:

1. **Each new member rewrites the whole array.** `UpdateChat` runs
   `UPDATE chats SET users = users || to_jsonb(?::bigint) WHERE chat_id = ? AND NOT (users @> ...)`.
   Postgres writes a new version of the whole TOASTed value, plus a new `chats` row version,
   every time one member is added.
2. **Membership checks read the whole array.** `UpdateChat` first calls `GetChatUsersCached`,
   which fetches and decodes the full list from Redis (or Postgres on a miss), and then runs
   `slices.Contains`.
3. **Federations search the arrays in Go.** `ChatsContainingUser` loads `users` for every chat in a
   federation and loops over each list.

---

## Part 1: Research

### 1.1 History worth knowing

- A `chat_users(chat_id, user_id)` table has existed before. The initial migration
  (`20250805200527`) created it, `20250805204145` gave it FKs to both `chats(chat_id)` and
  `users(user_id)`, and `20250814100001_drop_unused_chat_users_table.sql` dropped it because no
  code used it. The new table has to be created from scratch by a new migration. The old files
  cannot be edited because their checksums are recorded.
- `20250806100000` created a GIN index `idx_chats_users_gin` on `chats.users`, and
  `20250806105636` dropped it. No index serves `@>` on the array today.
- `20260904000000` set `chats` to `fillfactor = 85` because the table had a 0 to 1 % HOT-update
  ratio. The array append is one of the updates that makes `chats` churn.

### 1.2 What actually lands in the array

`alita/modules/users.go` `logUsers` (handler group `-1`) calls `updateCurrentChat(chat.Id, chat.Title,
user.Id())` for every group message whose sender is not an anonymous channel. In gotgbot
`rc.36`, `Sender.IsAnonymousChannel()` is false for **anonymous admins** (sender chat is the group
itself) and for **linked-channel auto-forwards**. `Sender.Id()` then returns a **negative chat ID**,
and that ID is appended to the array. Every current reader already discards those entries
(`IsValidUserId`, or matching against a positive fban target). The design uses this: see
`CHECK (user_id > 0)` below.

The array only ever grows. Nothing removes a member when they leave, so "member" here means
"has spoken in the chat at least once". The new table keeps that meaning.

### 1.3 Readers and writers

"Hotness" is how often the code path runs in production.

| # | Location | R/W | What it needs | Hotness | Notes |
|---|----------|-----|---------------|---------|-------|
| 1 | `alita/db/chats/repository.go:54` `UpdateChat` | R + W | **Membership check** (reads the full cached list, then `slices.Contains`), then **append one ID** | **Per message** in groups (group `-1`). Limited to once per `(chat, user)` per 5 min per replica by `chatUpdateCache` (`constants.ChatUpdateInterval`) plus singleflight. Runs in an async goroutine. | The upsert also inserts new chats with `users = [userid]`. The append is a whole-row, whole-TOAST rewrite and takes the `chats` row lock, so it queues behind the `SELECT ... FOR UPDATE` on the `chats` row in `warns` (`repository.go:102,181`) and `reports` (`repository.go:70`). A chat whose `users` is SQL `NULL` never gets appended to, because `NULL || x` is `NULL` and `NOT (NULL @> x)` is not true. |
| 2 | `alita/db/chats/repository.go:31` `EnsureChatInDb` | W (implicit) | Nothing about membership. Inserts `users = '[]'` via `Int64Array.Value()` and deletes the `chat_users` cache key | Per settings write (`logchannels`, `warns`, `greetings`, `connections`, `notes`, `rules`, `reports`, `federations` join, `captcha`) | GORM `Create(&models.Chat{})` names the `users` column explicitly. That matters for step 4. |
| 3 | `alita/db/chats/optimized.go:71-102` `GetChatUsers`, `GetChatUsersContext`, `GetChatUsersCached`, `GetChatUsersCachedContext` | R | **Full list** | Called only by rows 1, 4 and 5 | Cache key `alita:cache:chat_users:<chatID>` (`cache.CacheKey("chat_users", id)`), TTL `CacheTTLChatSettings` = 30 min, with a not-found sentinel through `loadChatCache`. Only `EnsureChatInDb` and `UpdateChat` invalidate it. |
| 4 | `alita/modules/misc_extras.go:86` `zombies` | R | **Full list**, then one `GetChatMember` per positive ID | **Per command** (`/zombies`, admin only, rare) | Telegram API calls dominate the cost, not the DB read. Filters with `IsValidUserId`. |
| 5 | `alita/modules/bans.go:1020` `unbanAllCallback` | R | **Full list**, then one `UnbanChatMember` per positive ID | **Per command** (owner confirmation callback, rare) | Same pattern as row 4. |
| 6 | `alita/db/federations/repository.go:637` `ChatsContainingUser` | R | **Membership check** of one user across the N chats in a federation | **Per command** (`/fban`), run from the `applyActiveFban` goroutine (`alita/modules/federations.go:756`) | Not cached. `SELECT chat_id, users ... WHERE chat_id IN ?` loads every array in the federation. The target is always positive because `applyFban` rejects `!IsValidUserId`. |
| 7 | `alita/db/chats/repository.go:106` `GetAllChats` | R (implicit) | **Neither.** It only uses `ChatName` and `IsInactive` | **Per command** (dev-only `/chatlist`, `alita/modules/devs.go:97`) | `db.DB.Find(&chatArray)` is `SELECT *`, so it detoasts **every array in the database** only to throw them away. |
| 8 | `alita/db/lang/repository.go:131` `ChangeGroupLanguage` | W (implicit) | Nothing. `Create(&models.Chat{...})` writes `users = '[]'` | Per command (rare) | Same step-4 concern as row 2. |
| 9 | `alita/db/backup/backup.go:921` `ensureBackupChat` | W (implicit) | Nothing. `Create(&models.Chat{ChatId})` with `ON CONFLICT DO NOTHING` | Per `/import` | **Backup export and import do not include the member list.** No backup module reads or writes `users`. |
| 10 | `alita/db/test_helpers.go:23` `EnsureChatInDb` (`testtools` build) | W (implicit) | Same as row 2 | Tests only | |
| 11 | `alita/db/chats/optimized.go:24` `GetChatBasicInfoContext` | none | Its `Select(...)` list leaves out `users` | Per update | Listed only to rule it out. `lang.getCachedLanguage` (`SELECT language`) and the `warns`/`reports` `SELECT id` lock queries also leave `users` out. |
| 12 | `alita/db/models/user.go:25` `Chat.Users`, and `alita/db/db.go:21` alias `db.Chat` | model | n/a | n/a | `Int64Array` is also used by `ReportChatSettings.BlockedList`, so the type stays. |
| 13 | `scripts/validate_orphaned_data.go:147` | R (SQL) | Orphan check on `chat_users` (a `chat_id` not in `chats` **or** a `user_id` not in `users`) | Manual (`make validate-db`) | **This is broken today.** The table was dropped in `20250814100001`, so `findOrphanReports` returns `failed to query chat_users` and the script exits 1. Its test creates the table in SQLite, so the test does not catch it. |
| 14 | `scripts/backup_database.sh` | R (pg_dump) | Whole database | Manual or cron | `pg_dump` picks up the new table automatically. No change needed. |
| 15 | `scripts/migrate_psql.sh`, `alita/db/migrations/runner.go` | applier | n/a | Startup (`AUTO_MIGRATE=true`) | Neither mentions `chat_users`. `runner.go:792` `verifyIndexes` lists the `chats` indexes. It could list `chat_users_pkey` (optional). |
| 16 | `docs/src/content/docs/api-reference/database-schema.md` (lines 35, 263, 780, 784) | docs | n/a | n/a | Manually maintained. It says membership lives in the JSONB column and that `chat_users` was dropped. Update it in steps 1 and 4. |
| 17 | Tests: `alita/db/chats/repository_test.go:117-203`, `alita/db/federations/repository_test.go:246` (`Update("users", ...)`), `alita/modules/misc_extras_test.go:129` (`Update("users", ...)`), plus `bans_command_test.go` and `federations_command_test.go` | R + W | Seed or assert the array | Tests | Must switch to seeding `chat_users` in step 3. |

What the readers need:

- **Membership check (one user):** rows 1 and 6. Row 1 is the only per-message path.
- **Full list:** rows 4 and 5. Both are rare commands whose cost is dominated by Telegram API
  calls.
- **Count:** no reader needs one today. `activity_monitor` counts the `users` table, not chat
  members.
- **Reads the column without needing it:** row 7.
- **Writes the column only because GORM names it:** rows 2, 8, 9 and 10.

---

## Part 2: Design

### 2.1 Table DDL

```sql
-- Membership of seen senders per chat. Replaces the chats.users JSONB array.
-- PK leads with chat_id: it serves "is user U in chat C", "list chat C", the
-- federation lookup (chat_id IN (...) AND user_id = ?) and the FK cascade.
CREATE TABLE IF NOT EXISTS chat_users (
    chat_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chat_users_pkey PRIMARY KEY (chat_id, user_id),
    CONSTRAINT chk_chat_users_user_id_positive CHECK (user_id > 0)
);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.table_constraints
                   WHERE constraint_name = 'fk_chat_users_chat') THEN
        ALTER TABLE chat_users
        ADD CONSTRAINT fk_chat_users_chat
        FOREIGN KEY (chat_id) REFERENCES chats(chat_id) ON DELETE CASCADE ON UPDATE CASCADE;
    END IF;
END $$;
```

Decisions:

- **`BIGINT` for both columns.** Telegram IDs do not fit in `int4`. This matches every other
  `chat_id`/`user_id` column.
- **No surrogate `id`.** The pair is the identity. A `BIGSERIAL` would add 8 bytes and a second
  index to every row for nothing. A GORM model can use a composite primary key
  (`gorm:"primaryKey"` on both fields).
- **FK to `chats(chat_id)` with `ON DELETE CASCADE ON UPDATE CASCADE`: yes.**
  - `chats.chat_id` has the unique constraint `uk_chats_chat_id`, so the FK is valid.
  - The parent row always exists when a membership row is written. `UpdateChat` upserts the
    `chats` row earlier in the same function, and the backfill selects from `chats`.
  - The app never deletes chats, so the cascade costs nothing at runtime. It stops orphans when
    an operator deletes a chat by hand, and it matches the convention in newer migrations
    (for example `fk_ai_spam_settings_chat`).
  - The PK leads with `chat_id`, so it already backs the FK. A parent delete does not scan the
    child table, and no extra index is needed.
  - Each insert pays one FK lookup on the `chats` unique index. That is cheap next to the array
    rewrite it replaces.
- **No FK to `users(user_id)`: deliberate. The dropped table had one.**
  1. `updateCurrentUser` and `updateCurrentChat` are separate async goroutines, so a membership
     row can arrive before its `users` row. With an FK, those inserts would fail at random.
  2. Every FK insert would take a `FOR KEY SHARE` lock on a row of `users`, the table with the
     most updates in the schema. That is extra contention on the per-message path.
  3. No reader joins to `users`.
- **`CHECK (user_id > 0)`: yes.** It stores only what readers already use (see 1.2). The write
  path and the backfill both skip non-positive IDs, so a group's own ID and linked-channel IDs
  stop taking space. Nothing observable changes: rows 4 and 5 filter with `IsValidUserId`, and
  row 6 only runs for positive targets.
- **No secondary index on `user_id`.** No reader asks "every chat for user U" without a list of
  chat IDs. Add one only when such a reader appears, following the "no unused indexes" history
  in `migrations/`.
- **`created_at`: optional.** It is cheap, gives a join date for later features, and makes the
  backfill easy to audit (backfilled rows share one timestamp). Drop it if the maintainer
  prefers the smallest possible row.
- **No `CREATE INDEX CONCURRENTLY`, no top-level `BEGIN`/`COMMIT`.** Per `AGENTS.md`. The table
  is new and empty, so building the PK inside the migration transaction takes no time.

### 2.2 Rollout: expand, migrate, contract

Each step is its own PR and its own release. **A step may only be deployed after the previous
step's release is running on every replica.** The invariants that make each step safe are listed
with it.

#### Step 1: create the table and dual-write

- Migration `migrations/<ts>_add_chat_users_table.sql` with the DDL from 2.1. The timestamp must
  be greater than every existing file name.
- `models.ChatUser{ChatID, UserID, CreatedAt}` with `TableName() == "chat_users"`, plus an alias in
  `alita/db/db.go` if the other aliases are kept.
- `UpdateChat`, after the `chats` upsert and **before** the existing array check:
  `INSERT INTO chat_users (chat_id, user_id) VALUES (?, ?) ON CONFLICT DO NOTHING`, skipped when
  `userid <= 0`. Write it with GORM `clause.OnConflict{DoNothing: true}` so SQLite tests run the
  same statement.
  - Running it unconditionally, not only when the array check says "new", means every active
    member lands in the table within one debounce window. That shrinks what the backfill has to
    catch. A conflicting `DO NOTHING` insert is one index probe and writes no heap tuple.
  - On error, return it. `asyncUpdateChat` already logs it and clears the debounce entry, so the
    write is retried. Per `AGENTS.md`, no DB error is discarded on a state-changing path.
- The array path stays exactly as it is. Readers do not change.
- `scripts/validate_orphaned_data.go`: change the `chat_users` check to
  `chat_id NOT IN (SELECT chat_id FROM chats)` only, because there is intentionally no `users`
  FK. This also fixes `make validate-db`, which fails today (row 13).
- Add `&models.ChatUser{}` to the test schemas (see 2.5).
- Update `database-schema.md` to say "`chat_users` is being populated; `chats.users` is still
  authoritative".
- Optionally add `chat_users_pkey` to `verifyIndexes`.

Invariant after step 1: every new `(chat, user)` touch writes both stores. While step 1 is
rolling out, old replicas still write the array only. The backfill in step 2 covers those rows.

#### Step 2: backfill the table from the array

The issue's one-statement backfill is:

```sql
INSERT INTO chat_users (chat_id, user_id)
SELECT chat_id, jsonb_array_elements_text(users)::bigint FROM chats
ON CONFLICT DO NOTHING;
```

It is correct, but it runs as **one transaction** inside the startup migration runner. Two
things make that dangerous at production size:

- `RunMigrations` holds `pg_advisory_lock('alita:schema_migrations')` and runs before `main()`,
  before the HTTP `/health` endpoint is up. Every replica that starts waits on that lock.
  Railway's `healthcheckTimeout` is 100 s (`railway.toml`). A backfill longer than that fails
  the deploy, the killed process rolls the transaction back, and the next deploy tries again from
  zero.
- One long transaction pins `xmin` for its whole run. Autovacuum then cannot clean `users` and
  `chats`, the two tables with the most updates, which already have poor HOT ratios.

The statement also needs guards whatever its size: `jsonb_typeof(users) = 'array'`
(`jsonb_array_elements_text` raises an error on a JSON `null` or a scalar), `> 0` (the new CHECK),
and `DISTINCT` (old races may have left duplicates, and dropping them avoids conflict work).

**Proposed approach (decided by the numbers in "Needed from maintainer"):**

- **Option A: single-transaction migration.** Use this if the total element count is small
  (roughly under 1-2 M rows, `max_len` under roughly 100 k, and `chats` under roughly 1 GB). Rough
  cost: a `chat_users` row is about 40 bytes of heap tuple plus about 30 bytes of PK entry, so
  about 70-80 bytes per member. Postgres inserts on the order of 10^5 rows/s in one transaction on
  modest hardware, so 1 M rows is about 10-20 s and 80 MB, plus WAL of about 2x. These are
  estimates to check against a staging copy, not measurements. The migration file:

  ```sql
  INSERT INTO chat_users (chat_id, user_id)
  SELECT DISTINCT c.chat_id, e.value::bigint
  FROM chats c
  CROSS JOIN LATERAL jsonb_array_elements_text(c.users) AS e(value)
  WHERE jsonb_typeof(c.users) = 'array'
    AND e.value::bigint > 0
  ON CONFLICT DO NOTHING;
  ```

- **Option B: batched, resumable backfill in the app.** This is the default if the numbers are
  large or unknown.
  - Migration `migrations/<ts>_add_data_backfills.sql` creates a small bookkeeping table
    `data_backfills(name TEXT PRIMARY KEY, cursor BIGINT NOT NULL DEFAULT 0, completed_at TIMESTAMPTZ)`
    and inserts the row `('chat_users', 0, NULL)`.
  - After startup, in `main` and not in an `init()`, a goroutine does the following. It starts with
    `defer error_handling.RecoverFromPanic(...)` and is registered as a shutdown drain after
    DB-close so it stops first.
    1. Take `pg_try_advisory_lock('alita:backfill:chat_users')`, so only one replica works on it.
       The others skip.
    2. Loop over `chats` in keyset order on `chats.id > cursor`, taking up to about 500 chats or
       about 50 k array elements per batch. Each batch commits on its own:
       run the Option A `INSERT ... SELECT` restricted to `c.id > $cursor AND c.id <= $batch_end`,
       then `UPDATE data_backfills SET cursor = $batch_end`, in the same short transaction. Sleep
       about 100 ms between batches.
    3. A single chat above the element budget (the `max_len` outlier) gets its own batch. It can
       be split further with `jsonb_array_elements_text(users) WITH ORDINALITY` ranges if one chat
       is still too large for one transaction.
    4. When the cursor passes `max(chats.id)`, set `completed_at = now()`.
  - Resuming is free. The cursor survives restarts, and `ON CONFLICT DO NOTHING` makes a
    repeated batch harmless.
  - It works the same for self-hosters with `AUTO_MIGRATE=true` and needs no manual step.
  - Progress is visible in `SELECT * FROM data_backfills`.

**Verification query** (must return 0 before step 3 relies on the table without a fallback):

```sql
SELECT count(*)
FROM chats c
CROSS JOIN LATERAL jsonb_array_elements_text(c.users) AS e(value)
WHERE jsonb_typeof(c.users) = 'array'
  AND e.value::bigint > 0
  AND NOT EXISTS (SELECT 1 FROM chat_users cu
                  WHERE cu.chat_id = c.chat_id AND cu.user_id = e.value::bigint);
```

Invariant after step 2: the table is a superset of the array's positive IDs. Dual-write keeps it
that way.

#### Step 3: switch every reader to the table

New repository functions in `alita/db/chats/`:

- `AddChatMember(chatID, userID int64) (inserted bool, err error)`: the step-1 insert, returning
  `RowsAffected > 0`.
- `IsChatMember(ctx, chatID, userID int64) (bool, error)`:
  `SELECT EXISTS (SELECT 1 FROM chat_users WHERE chat_id = ? AND user_id = ?)`.
- `ChatMemberIDs(ctx, chatID int64) ([]int64, error)`:
  `SELECT user_id FROM chat_users WHERE chat_id = ? ORDER BY user_id`.
- `ChatsWithMember(ctx, userID int64, chatIDs []int64) ([]int64, error)`:
  `SELECT chat_id FROM chat_users WHERE user_id = ? AND chat_id IN ?`. This uses one PK probe
  per chat.

Reader changes (Part 1 row numbers):

| Row | Before | After |
|-----|--------|-------|
| 1 `UpdateChat` | `GetChatUsersCached` + `slices.Contains`, then append | `AddChatMember` first. **Remove the Redis membership read.** Keep the array append (see below) for rollback safety. |
| 4 `zombies`, 5 `unbanAllCallback` | `GetChatUsersCachedContext` | `ChatMemberIDs` |
| 6 `ChatsContainingUser` | loads every array, loops in Go | delegates to `ChatsWithMember` (keep the exported name in `federations`, or inline it) |
| 7 `GetAllChats` | `SELECT *` | `Select("chat_id, chat_name, is_inactive")`. This can also ship early, on its own. |
| 17 tests | seed `users` | seed `chat_users` |

**Guard against an incomplete backfill.** If Option B was used, every table reader checks
`data_backfills.completed_at IS NOT NULL` (cached in-process once true, re-checked about every
minute until then). **Until the backfill is complete, readers fall back to the array**, so a
half-done backfill never shows a short member list. Delete the guard in step 4. With Option A,
the backfill finished in the same migration run, so no guard is needed.

The array keeps being written in step 3. Rolling back to step-2 code then only means the readers
go back to the array, and the array is still complete.

#### Step 4: contract (two PRs, two releases)

Dropping the column needs two releases, because **old code breaks on a dropped column**: every
`Create(&models.Chat{...})` (rows 1, 2, 8, 9, 10) names `users` explicitly. If one PR removed
the field and dropped the column, the first replica of the new release would run the migration
at startup while old replicas were still serving, and all their chat inserts would fail with
`column "users" does not exist`.

- **4a: stop using the column (code only).**
  - Delete `Chat.Users` from `models.Chat`. GORM then never names the column, and new rows get
    `NULL` there (it is nullable with no default).
  - Delete the array append in `UpdateChat`, the `GetChatUsers*` functions, the `chat_users`
    cache key and its `DeleteCache` calls, the step-3 fallback, and the `data_backfills` checks.
  - Removing the field also removes the `users` column from every SQLite test schema, with no
    `AutoMigrate` list edits (see 2.5).
  - Update tests that use `Update("users", ...)`.
- **4b: drop the column (migration only), in a later release than 4a.**
  - `migrations/<ts>_drop_chats_users_column.sql`:
    1. **Catch-up:** run the Option A `INSERT ... SELECT ... ON CONFLICT DO NOTHING` once more,
       restricted to `c.id > (SELECT cursor FROM data_backfills WHERE name = 'chat_users')` when
       that table exists. On production this is close to a no-op. For a self-hoster who jumps
       from a pre-step-1 version straight to 4b, it is the only backfill that ever runs (their
       startup migrations run 1, 2 and 4b back to back before any app code). Self-hosted
       databases are small, so one transaction is fine there.
    2. `ALTER TABLE chats DROP COLUMN IF EXISTS users;`. This is a metadata-only change and
       takes a brief `ACCESS EXCLUSIVE` lock. The space comes back gradually as rows are
       rewritten, or at once with a manual `VACUUM FULL`/`pg_repack` in a maintenance window.
       Do not put that in the migration.
    3. `DROP TABLE IF EXISTS data_backfills;` (Option B only; it can also stay as a generic
       tool).
  - Update `database-schema.md` (rows for `chats.users` and the "Chat Users" relationship lines).

### 2.3 Cache keys and invalidation

| Key | Steps 1-2 | Step 3 | Step 4 |
|-----|-----------|--------|--------|
| `alita:cache:chat_users:<chatID>` (full array) | unchanged: filled by `GetChatUsersCached`, deleted by `EnsureChatInDb` and by the array append | **no longer read** by any reader. Still deleted on array writes, so a rollback to step 2 never reads stale data. | removed from the code. Leftover entries expire within 30 min (TTL), and `CLEAR_CACHE_ON_STARTUP` also clears them because they sit under `alita:cache:`. |

**No replacement key is proposed.**

- **Hot path (row 1):** today, a known member costs one Redis `GET` that returns and decodes the
  **whole** list. After step 3 it costs one `INSERT ... ON CONFLICT DO NOTHING` on the PK. That is
  O(log n) whatever the group size, writes nothing when the row exists, and stays limited by the
  existing 5-minute per-`(chat, user)` debounce. A cache in front of it would only save a cheap
  indexed probe and add an invalidation duty. The DB-query and Redis-command counters from #863
  (`alita/utils/metrics`) should confirm this after the rollout. If DB queries per update rise
  noticeably, add a positive-only key `chat_member:<chatID>:<userID>`, written after a successful
  insert or a conflict. It never needs invalidating on inserts, only on a chat delete, which the
  app never does.
- **Full list (rows 4 and 5):** rare admin commands whose runtime is one Telegram API call per
  member. The uncached `ChatMemberIDs` read is noise next to that.
- **Federation lookup (row 6):** a fire-and-forget goroutine per `/fban`, already uncached.

If any of these keys is added later, these writes must `cache.DeleteCache` it: `AddChatMember` when
`inserted == true` (list key only), the backfill (do not delete per key; flush or version the
prefix, or simply ship the cache after the backfill completes), and any future "remove member"
path.

### 2.4 Backups

- The per-chat export and import (`alita/db/backup`) **does not contain membership today**, and
  it should stay that way. Membership is tracking data derived from traffic, not settings. It
  would also be wrong to import it into a different chat ID.
- `ensureBackupChat` needs no change until step 4a, when `Chat.Users` disappears and the insert
  simply stops naming the column.
- `backup_test.go` / `roundtrip_test.go` create `models.Chat{}` through the test schema. Nothing
  else changes.
- `scripts/backup_database.sh` (`pg_dump`) picks up `chat_users` automatically. A dump taken
  mid-rollout restores to a consistent point for that step, because both stores are in the same
  snapshot.

### 2.5 `models.Chat`, test schemas and `internal/testdb`

- `internal/testdb/database.go:48` prepends `&models.User{}, &models.Chat{}` to every
  `testdb.Run` schema (admin, aispam, antiflood, antiraid, approvals, blacklists, channels, chats,
  disabling, greetings, lang, locks, monitoring, pins, reactions). **Step 1 adds
  `&models.ChatUser{}` to that prepend**, which covers all those packages in one edit.
- Hand-written `AutoMigrate` lists that include `models.Chat` need `&models.ChatUser{}` added in
  step 1 (per `AGENTS.md` "Adding a module", step 4):
  `alita/db/testmain_test.go`, `alita/db/backup/testmain_test.go`,
  `alita/db/captcha/testmain_test.go`, `alita/db/connections/testmain_test.go`,
  `alita/db/devs/testmain_test.go`, `alita/db/federations/testmain_test.go`,
  `alita/db/filters/testmain_test.go`, `alita/db/logchannels/testmain_test.go`,
  `alita/db/notes/testmain_test.go`, `alita/db/channels/repository_test.go`,
  `alita/db/chats/repository_test.go`, `alita/db/reports/repository_test.go`,
  `alita/db/rules/repository_test.go`, `alita/db/user/repository_test.go`,
  `alita/db/warns/repository_test.go`, `alita/modules/test_harness_test.go`,
  `alita/utils/extraction/extraction_test.go`, `alita/utils/formatting/formatting_test.go`,
  `alita/utils/httpserver/server_test.go`, `alita/utils/monitoring/activity_monitor_test.go`.
  Strictly, only packages that reach `UpdateChat` or the new readers need it. Adding it
  everywhere `Chat` appears avoids a "no such table" failure when code paths move.
- In step 4a, removing `Chat.Users` removes the column from every SQLite schema with no list
  edits. The `ChatUser` entries stay.
- SQLite does not run the `migrations/*.sql` files, so the `CHECK` and the FK only exist on
  Postgres. The GORM model should carry `check:user_id > 0` if the step-1 PR wants SQLite to
  enforce it. The CI migration-chain step applies the real SQL, and the checksum test covers the
  new files.

### 2.6 Risks

| Risk | Effect | Mitigation |
|------|--------|------------|
| **Backfill half done** (process killed, deploy rolled back, Option B cursor mid-way) | Table readers would see a short list: `/zombies` and `/unbanall` skip members, and a federation ban misses chats where the user has not spoken since step 1 | Step 3 readers fall back to the array until `completed_at` is set. Option A is atomic. Each Option B batch is its own transaction and resumes from the cursor. All inserts are idempotent. The 4b catch-up closes any remaining gap before the array is gone. |
| **Backfill blocks startup** (Option A on a large dataset) | The advisory lock and pre-`main` run make every replica wait, health checks time out, and a rollback restarts it from zero | Pick Option A only when the maintainer's numbers are under the threshold. Otherwise use Option B, which runs after `/health` is up and never holds the migration lock. |
| **Long transaction pins `xmin`** | Autovacuum falls behind on `users` and `chats`, bloat grows | Option B uses short batches. Run Option A only off-peak. |
| **Multi-replica, step 1 rolling out** | Old replicas append to the array only | Step 2 runs only after every replica runs step 1, so the backfill picks those rows up. |
| **Multi-replica, step 3 rolling out** | Old (step 2) replicas still read the array, new ones read the table | Both stores are complete and dual-written, so the answers match. |
| **Multi-replica, step 4 rolling out** | Old replicas' `Create(&models.Chat{})` names `users` | 4a and 4b are separate releases. 4b is only deployed after 4a is on every replica. |
| **Version jumps** (a self-hoster upgrades from pre-step-1 directly to step 3 or later) | Migrations 1 and 2 (and 4b) run before any new code. Option B's backfill goroutine exists only in step 2 and 3 code. | The step-3 fallback covers readers until the goroutine finishes. The 4b migration's catch-up covers someone who jumps straight past 3. Release notes for every step must say that `AUTO_MIGRATE=false` operators run `make psql-migrate` before starting the new binary. Otherwise step-1 code logs failed inserts (`relation "chat_users" does not exist`) on every tracked message. The array path still works, so nothing is lost. |
| **Rollback from step 3 to step 2** | none | Step 3 keeps writing the array. |
| **Rollback after 4b** | The array is gone | This one is not reversible without restoring from `pg_dump`. Take a `make backup-db` right before deploying 4b. |
| **Non-array or `NULL` values in `users`** | `jsonb_array_elements_text` raises an error, and the whole Option A migration aborts startup | Every backfill statement filters on `jsonb_typeof(users) = 'array'`. The maintainer's data-shape query shows whether any rows are affected. |
| **FK insert on a chat row that does not exist** | Insert fails | `UpdateChat` upserts `chats` first, and the backfill reads from `chats`. No other writer exists. |
| **Cache stampede after switching** | none expected | The hot path stops reading Redis for membership. No new key is introduced. |

---

## Needed from maintainer

Run on production (read-only, ideally against a read replica) and paste the results on #875. The
first two queries are the issue's own. Queries 3 to 5 are added here because the design depends
on them.

```sql
-- 1 (from the issue): array-size distribution
SELECT count(*) AS chats,
       percentile_cont(ARRAY[0.5,0.9,0.99]) WITHIN GROUP (ORDER BY jsonb_array_length(users)) AS p50_p90_p99,
       max(jsonb_array_length(users)) AS max_len
FROM chats WHERE users IS NOT NULL;

-- 2 (from the issue): table size
SELECT pg_size_pretty(pg_total_relation_size('chats')) AS chats_total_size;

-- 3: data shape. Run this first if query 1 fails with
--    "cannot get array length of a scalar": that means a JSON null or scalar exists.
SELECT coalesce(jsonb_typeof(users), 'sql null') AS kind, count(*)
FROM chats GROUP BY 1;

-- 4: total rows the backfill will insert (upper bound; includes duplicates and non-positive IDs)
SELECT sum(jsonb_array_length(users)) AS total_elements,
       pg_size_pretty(sum(pg_column_size(users))) AS users_column_size
FROM chats WHERE jsonb_typeof(users) = 'array';

-- 5 (optional, scans every array): non-positive IDs that the CHECK will drop
SELECT count(*) AS non_positive_ids
FROM chats c CROSS JOIN LATERAL jsonb_array_elements_text(c.users) AS e(value)
WHERE jsonb_typeof(c.users) = 'array' AND e.value::bigint <= 0;
```

Also needed (not SQL): how many replicas production runs, and on which platform (Railway, Render,
Docker). This decides how long rolling deploys overlap and which health-check timeout applies.

| Query | Result |
|-------|--------|
| 1: `chats` |  |
| 1: `p50_p90_p99` |  |
| 1: `max_len` |  |
| 2: `chats_total_size` |  |
| 3: kinds and counts |  |
| 4: `total_elements` |  |
| 4: `users_column_size` |  |
| 5: `non_positive_ids` |  |
| Replicas / platform |  |

Decisions that depend on these numbers:

- **Option A or Option B for step 2** (2.2): from `total_elements`, `max_len` and `chats_total_size`
  against the rough thresholds (about 1-2 M rows, about 100 k `max_len`, about 1 GB). With Option A,
  the `data_backfills` table and the step-3 fallback are not needed.
- **Batch sizing for Option B** (chats per batch, element budget, whether `WITH ORDINALITY`
  splitting of one chat is needed): from `p99` and `max_len`.
- **Whether the backfill guards matter** (`jsonb_typeof`, SQL `NULL`): from query 3. A non-zero
  `sql null` count also confirms the `UpdateChat` `NULL`-array bug from row 1. Those chats have
  never recorded members, and the new table fixes that as a side effect.
- **Whether the CHECK noticeably shrinks the data**: from query 5.
- **Whether to reclaim `chats` space after 4b** (`pg_repack` in a maintenance window): from
  `users_column_size` against `chats_total_size`.
- **How long the deploy gap between steps must be**: from the replica count and platform.

---

## Follow-up issues

One issue per rollout step. Open them only after this design is approved, and link each one
from #875 and #862.

1. **`perf(db): add chat_users table and dual-write membership (#875 step 1)`**
   Migration creating `chat_users` (DDL from 2.1), `models.ChatUser`, and an unconditional
   `INSERT ... ON CONFLICT DO NOTHING` in `UpdateChat` next to the unchanged array append.
   Add the model to `internal/testdb` and the hand-written `AutoMigrate` lists. Fix the
   `chat_users` orphan check in `scripts/validate_orphaned_data.go`, and update
   `database-schema.md`.

2. **`perf(db): backfill chat_users from chats.users (#875 step 2)`**
   Option A (single-transaction migration) or Option B (`data_backfills` table plus a batched,
   advisory-locked, resumable background job), as chosen from the production numbers. Covers
   the `jsonb_typeof`/`> 0`/`DISTINCT` guards and the verification query, and must ship only after
   step 1 is on every replica.

3. **`perf(db): read chat membership from chat_users (#875 step 3)`**
   Add `AddChatMember`, `IsChatMember`, `ChatMemberIDs` and `ChatsWithMember`. Switch
   `UpdateChat`, `/zombies`, `/unbanall` and `ChatsContainingUser` to them, and narrow
   `GetAllChats`'s `SELECT *`. Keep writing the array. Fall back to the array until the backfill
   is marked complete. Move tests from seeding `users` to seeding `chat_users`.

4. **`perf(db): drop chats.users after chat_users migration (#875 step 4)`**
   Two PRs in two releases. 4a removes `Chat.Users`, the array append, the `GetChatUsers*`
   functions, the `chat_users` cache key and the fallback. 4b, deployed only after 4a is on
   every replica, adds a migration that runs a final idempotent catch-up backfill and then
   `DROP COLUMN users`. Take a `pg_dump` before 4b and update `database-schema.md`.
