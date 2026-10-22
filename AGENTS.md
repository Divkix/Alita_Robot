# AGENTS.md

Alita Robot: Telegram group-management bot. Go module `github.com/divkix/Alita_Robot`; PostgreSQL (GORM) + Redis.
Update this file in the same commit as any change that invalidates a rule below.

## Product

- Additive by default: a new feature must not change what an existing command, setting or default does.
- New behaviour that changes what a chat sees (spam checks, captcha, auto-actions) is off by default and enabled per
  chat.
- Change an existing default or command meaning only to fix a bug; the commit is `fix:` with the old and new behaviour
  in the body.
- Group-only and connection-based usage keep working unchanged when a PM or shortcut form is added, except where the
  PM rule below decides.
- In PM a group the user names wins: a command whose first argument names a group (@username or ID, or a title the
  lookup resolves; `withPMGroupArg`) runs in that group even when the user is connected to another. A command without
  arguments of its own (`PMGroupArgNone`) whose first word does not name exactly one group refuses: with its help when
  the word names no group, with the groups so titled when a title names several (`connections_pm_group_too_many` past
  `pmGroupTitleCheckLimit`), or with `settings_picker_wait` when the picker's pause leaves nothing to look it up in; it
  never falls back to the connected group (`/rules foo` does not show the connected group's rules). Reason: the
  connection is not visible when the message is sent and is easy to forget; acting in a group other than the one named
  is a worse error than a refusal, which costs one retry. Keep this rule when adding PM forms.

## Commands

```bash
make test                 # the only valid full test run (tags, race, coverage, skip gate)
make lint                 # golangci-lint v2
make check-translations   # missing locale keys; values copied from en.yml (allowlist: scripts/check_translations/identical_allowlist.txt)
make check-docs           # generated-docs drift; fix with `make generate-docs`
make bump-version TAG=vX.Y.Z   # the only way to change version strings
go test -tags testtools -race -count=1 -run '^TestName$' ./alita/db/warns   # one test
CGO_ENABLED=0 go build ./...   # compile check; `make build` needs goreleaser v2 + Docker
```

## Startup and shutdown

- `alita/config` `init()` → `alita/db` `init()` connects to Postgres **before `main()`** and runs migrations when
  `AUTO_MIGRATE=true`. Never add DB access to an `init()` that can run before `alita/db`'s.
- `--version`, `--health`, and `*.test` binaries (unless `ALITA_TEST_DATABASE=true`) skip DB init. Guard new pre-`main`
  side effects with `isCliModeActive`.
- Shutdown runs LIFO within 60 s. Register new drains *after* DB-close so they run before it.
- Deploy manifests set `AUTO_MIGRATE=true`; the code default is `false`. Never call `gorm.AutoMigrate` in production code.

## Handlers

- Group numbers are execution order: `-10` captcha sweeper · `-6` fed-ban · `-5` antiraid · `-4` spamdb ·
  `-2` admin-cache · `-1` users tracker + bot-joined (MyChatMember) · `0` commands/help/greetings ·
  `1` connected-chat guard · `3` aispam · `4` antiflood · `5`/`6` locks · `7` blacklists · `8` reports+reactions ·
  `9` filters · `10` pins · `11` log-channel.
- A PM command that acts on the connected group sets `ctx.EffectiveChat` to it, and later groups must not run on
  that ctx. `WrapCommand` ends iteration after such a swap in any group, whatever the handler returns (error and
  panic included). Legacy `dispatcher.AddHandler` commands rely on the group-`1` guard, which only sees swaps made in
  group `0`; a swapping handler registered outside `WrapCommand` in a higher group must return `ext.EndGroups` on
  every path. Keep group `1` for the guard alone: a handler there that matches first would skip it.
- `withPMGroupArg` (`alita/modules/pm_group_arg.go`) lets a legacy PM command take its group as the first argument
  (`-100…` ID, a group's @username, or the title of a group the user administers). A title is matched against the
  connected group and the `settingsCandidateLimit` (50) most recently active groups the user posted in
  (`settingsCandidateChats`, uncached), and admin rights are checked only on the matches (none when more than
  `pmGroupTitleCheckLimit` (3) share the title: the call gets `connections_pm_group_too_many`), outside the `/settings`
  picker's button cap and budget; a group outside those 50 is named by @username or ID only. The lookup takes the
  picker's pause before it scans, so one user scans `chats` at most once per pause: every scan keeps its groups for
  the pause (`settingsScanKey`), and inside the pause a title is matched against them, never against the picker's
  list, which holds only the groups that passed its checks; with no scan kept the call gets `settings_picker_wait`. A
  title that names none there gets the help with the list the last picker build kept, or with the wait notice when
  the pause built none.
  With a saved connection a call without a first word runs on the connection, and so does a `PMGroupArgOwn` call
  whose first word is not a group's @username or ID: that word is the command's own argument and is never looked up
  as a title. The command's help and the picker's list (for a title, built from the lookup's scan) answer a call
  without a first word and without a connection, and a first word looked up as a group that names none (an ID or
  @username of no group, or a title that matches none), so the command needs a `commandHelpModules` entry. It checks
  the group like `/connect`, stores it with `chat_status.SetOneOffGroup` (which `IsUserConnected` prefers over the saved
  connection and checks only for `chatAdmin`/`botAdmin`, not membership, and whose failed checks never disconnect;
  any new caller must run `checkUserConnectToChat` first and, when it reports no live member check (an admin from the
  cached list, which outlives a lost `chat_member` update), look the user up once itself), replaces `ctx.EffectiveMessage` with a copy without the group (entities shifted, UTF-16) and
  ends iteration. `ctx.Message` stays the original: read command text only through `ctx.EffectiveMessage` or
  `ctx.Args()`. Enable it per command at registration; outside PM it does nothing, and a rich message with
  arguments gets `connections_pm_group_plain_text` (blocks cannot be cut). A `WrapCommand` command enables the same through
  `CommandDescriptor.PMGroupArg`: the group is resolved before `RequiredChecks`, so they see it as they would a
  connected group, by the resolver `alita/modules` sets with `helpers.SetPMGroupResolver` in `init()`.
- A group-only command taken to PM gets its group from `groupOnlyChat` (`alita/modules/pm_group_reset.go`; a
  `WrapCommand` command whose checks need the group puts `groupOnlyChatCheck` before them), so its checks see that
  group. A command that checks the user's admin status live itself (`/admincache`) uses `groupOnlyMemberChat`
  instead, which skips the cached admin check. A confirmation button it sends carries the group in its `c` field
  (`confirmData`), and the press acts only through `ownerConfirmChat`, which reads the group with `buttonChat` and
  checks the presser against it: never the PM it was pressed in. A bans button sent from PM carries the group the
  same way: `moderationButtonData` adds it, and an unban or unmute button gets it as `keyboard.UnrestrictButton`'s
  chat ID (`pmButtonChatID`). Its handler reads it with `buttonChat` and checks the presser's right there. A bans or
  unrestrict button sent in a group has no `c` field (`moderationButtonData` adds none, and `UnrestrictButton` gets
  chat ID 0), nor does one sent before PM support; for those `buttonChat` returns the chat the button is in.
  Confirmation buttons always have it (`confirmData` writes `c` wherever they are sent).
- gotgbot stops a group at the first matching handler. Watchers return `ext.ContinueGroups`; commands return
  `ext.EndGroups`. A group-0 watcher returning `nil` silently kills every later group-0 handler.
- Modules register in `init()` via `RegisterLegacyModule(name, priority, load)`. Use a unique name; duplicates are
  dropped silently. Priority sets load order and group-0 precedence; `LoadHelp` loads last.
- New commands use `helpers.WrapCommand(dispatcher, CommandDescriptor{...}, handler)`. Set `Disableable: true` for any
  command chats may disable. Do not add replies for failed checks; the pipeline sends them.
- Anonymous admins bypass `WrapCommand`. Admin commands that must work for them also need
  `RegisterAnonymousAdminHandler` + `anonPipelineHandler`.
- Handler methods use value receivers on `moduleStruct`.

## Callbacks

- Encode/decode only through `alita/utils/callbackcodec` (`<ns>|v1|<url-encoded>`, 64-byte cap). Never `strings.Split`.
- `encodeCallbackData` returns `""` on overflow, which ships a dead button. Put user text in Redis behind a short token.

## Permissions

- `alita/utils/chat_status` predicates return bools and send no refusals; use `PermissionResponder` to message.
  Exceptions: `CanUser*` and `CanInvite` reply with the anonymous-admin proof keyboard (`checkAnonAdmin`) to an
  anonymous admin in a chat with AnonAdmin off, so keep them out of watchers; `IsUserConnected` (returns
  `*gotgbot.Chat`) sends its own refusals.
  `helpers.CheckFunc` replies and needs a `*helpers.CommandContext`: run it only through `WrapCommand`'s
  `RequiredChecks`, or through `helpers.RunChecks` on a `helpers.BuildCommandContext` result in an anonymous-admin
  post-proof handler (`anonPipelineHandler`, pins' `enforcePinChecks`). Never call it from a watcher.
- Given a nil chat, `chat_status` helpers check `ctx.EffectiveChat` once a handler has swapped it to a connected
  group; `PermissionResponder` still answers in the chat the update came from.
- After such a swap `lang.GetLanguage(ctx)` returns the language of the chat the message came from (the user's, in
  PM), since replies go there. Read the group's language with `lang.GetLanguage(&ext.Context{EffectiveChat: group})`.
- `IsUserAdmin` returns false for channel and non-positive IDs. Never pass a chat ID as a user ID.
- `*ForUpdate` predicates are memoised per update. Watchers and command-pipeline checks only, never after a state change in the same update.

## Data

- Repositories live in `alita/db/<domain>/`. Read via `cache.GetFromCacheOrLoad` with `cache.CacheKey` keys.
  **Every write must `cache.DeleteCache` the keys it affects.**
- `GetFromCacheOrLoad` has an in-process layer (`CACHE_LOCAL_TTL`, default 10 s) in front of Redis. `DeleteCache`
  evicts it on this replica only; list freshness-critical keys in `skipLocal` (`alita/db/cache/local.go`). Tests that
  write rows directly must call `cache.ResetLocalForTest()` or `DeleteCache` before reading through the cache.
- Two packages are named `cache`; the loader and generation guards are in `alita/db/cache`, not `alita/utils/cache`.
- Operational Redis keys (`alita:antiraid:*`, `alita:anonAdmin:*`) sit outside the `alita:cache:` prefix;
  `CLEAR_CACHE_ON_STARTUP` does not clear them.
- `UpdateRecord` skips zero values. Use `UpdateRecordWithZeroValues` to write `false`/`0`/`""`. Both return
  `gorm.ErrRecordNotFound` when no row matched.
- Check `TableName()` before raw SQL: `ConnectionSettings→connection` (per user), `ConnectionChatSettings→
  connection_settings` (per chat), `AdminSettings→admin`, `DisableSettings→disable`.

## Migrations

- `migrations/*.sql` is the schema source of truth. Never edit an applied file; the SHA-256 checksum aborts startup.
- Name new files with a timestamp greater than every existing one; order is plain filename sort.
- Each file runs in one transaction: no top-level `BEGIN`/`COMMIT`/`ROLLBACK`, no `CREATE INDEX CONCURRENTLY`.
- Change `alita/db/migrations/runner.go` and `scripts/migrate_psql.sh` together; both appliers must agree.

## Adding a module

1. Add a migration in `migrations/` (rules above).
2. Add the model in `alita/db/models/` with `TableName()` returning the migration's table name.
3. Add the repository in `alita/db/<domain>/`: reads through `cache.GetFromCacheOrLoad`, `cache.DeleteCache` on writes.
4. Add the model to the `AutoMigrate` list in the affected `testmain_test.go` or `internal/testdb.Run` call. Do the same
   whenever you change an existing model.
5. Add `alita/modules/<name>.go` with `LoadXxx`, registered in `init()` via `RegisterLegacyModule`.
6. Register commands with `helpers.WrapCommand`; add `RegisterAnonymousAdminHandler` for admin commands.
7. Add locale keys to all 7 files in `locales/`.
8. Run `make generate-docs`, `make check-translations`, `make test`, `make lint`.

## Subsystem traps

- Approved users skip antiflood, antiraid (neither banned nor counted on join), aispam, locks, blacklists, captcha,
  and the report auto-mute.
- Antiflood counters are in-process (per replica). Antiraid is Redis-only and does nothing without Redis.
- Fed-ban lookups cache per `(fed, user)` with a negative sentinel; every ban/unban write must invalidate it.
- A join arrives as both `ChatMemberUpdated` and a service message; dedupe through `claimRecentJoinProcessing`.
- Entity offsets are UTF-16: slice with `extractEntityText`, and match against both `Entities` and `CaptionEntities`.
- Captcha allows one attempt per `(user, chat)`; group `-10` stores the pending user's messages (type, text,
  file_id) and deletes the originals. Nothing is replayed: a correct answer deletes the stored rows with the attempt
  and posts only a type/count summary (`captcha_stored_messages_summary`, removed after 30 s). Admins list or clear
  them with `/captchapending` and `/captchaclear`.
- `alita:cache:captcha_pending:<chat>` is a per-chat pending flag; any new code that inserts into `captcha_attempts`
  must `DeleteCache` it after commit.
- `warn_entries` cascades from `chats` and `users` like `warns_users`, but has no FK to `warns_users`: any code that
  deletes `warns_users` rows deletes the matching entries in the same transaction, after taking the
  `chats … FOR UPDATE` row lock every warn write holds (without it a first `/warn`, still an uncommitted INSERT, can
  commit between the two DELETEs and lose its entry). Backups export only entries of users with warns, with
  `warned_by` zeroed on anonymous ones and set to `models.WarnedByBot` (-1) on the bot's own, which `/warns` shows as
  automatic. `warns_users.warns` is still written so an older release can read it after a rollback; that release
  writes `warns_users` alone, so reads, warn writes and imports reconcile entries with the reasons through
  `models.ReconcileWarnEntries`.
- Warn expiry (`warns_settings.warn_expiry`): the live count is `num_warns` minus the expired entries; an entry with a
  NULL `warned_at` never expires. Reads filter expired entries after the cache and never prune them; only `WarnUser`
  and `RemoveWarn` delete expired entries, under their row locks.
- A command in `commandHelpModules` answers a call it cannot run with its own entry cut from `<module>_help_msg`; the
  warn commands (/warn, /swarn, /rmwarn, /resetwarns, /warns) do so only from PM, and in a group answer a call without
  a user with `common_no_user_specified` alone, as before PM support. That help keeps one entry per command
  (`× /cmd …`, continuation lines up to a blank line or the next entry) in all 7 locales, with no `"` or unknown HTML
  tags in the entry, and en.yml `<module>_extend_msg` lists `/cmd`; `TestEveryHelpCommandHasItsLineInEveryLocale`
  enforces it. Each such command also needs a `<cmd>_example` in `locales/config.yml` starting with `/<cmd> `, except
  reply-only commands (`replyOnlyCommands`: /dban, /dkick, /dmute), which must have none;
  `TestEveryHelpCommandHasAnExample` enforces it. Help sent to PM shows the optional `<cmd>_pm_example` (the group
  first) in place of `<cmd>_example` when the command has one.
- `menuCommands` (`alita/modules/command_help.go`) lists exactly the commands of the en.yml `<module>_extend_msg`
  texts, each with its module and `AdminOnly` (set only when no form of the command works for a member); a command
  added to or removed from an extend list changes the map in the same commit (`TestMenuCommandsMatchExtendLists`).
  Every module there has a one-line `<module>_section_desc` in all 7 locales. `/commands` (`commands_menu.go`) shows
  members only the commands without `AdminOnly`; it finds admin status in PM through the deep-link group, the
  connected group, then `adminGroupsForUser` (the picker's pause and scan), and never through a scan of its own.
- `moderation_events` gets one row per successful human moderation action, written only by `recordModerationEvent`
  (`alita/modules/moderation.go`) from commands and inline moderation buttons. Never record automated actions
  (spamdb, captcha, antiraid, locks, blacklists, warn limits, report auto-mute): the shared-reputation join check
  reads this table, and a join it blocks must not feed the signal back. A reputation unban clears the signal only
  when the same chat removed the user earlier in the window.
- Activity counters (`alita/utils/stats`) are in-process and flushed additively into `chat_daily_stats` and
  `chat_hourly_stats` (UTC day and hour) in one transaction every 60 s and on shutdown (before DB close). Never add a
  DB write per message; joins count after
  `claimRecentJoinProcessing`, leaves in `leftMember`. Counts are approximate across a crash by design.
- A `/settings` summary toggle (`settingsToggles`, `settings_toggles.go`) is keyed by its row's `Command` and cycles
  the values that row shows. It must check what its command checks (`botAdmin`, and `right`, which returns a
  refusal key and sends nothing) and write through the same repository call, with the command's side effects (`/captcha off` runs
  `DisableCaptchaForChat`); change both when either changes. Where the command has a toggle config
  (`greetingToggleConfig`, `pinToggleConfig`), build the toggle from it. The button carries the value it sets, never
  "flip".
- The weekly digest sends at most once per chat per week through the `digestsettings.Claim` compare-and-set on
  `last_sent_at`. A transient send error `Restore`s the claim to retry; a permanent one
  (`helpers.IsPermanentSendError`) keeps it.
- The admin panel (`alita/panel`, off unless `PANEL_ENABLED`) authenticates only through `VerifyInitData`. Every
  `/chats/{id}` endpoint starts with `chatRequest` (groups only, current members only; non-admins read-only). Gate
  each write with `requirePermission` set to the Telegram right the equivalent command checks. Never call
  `chat_status` `Can*` helpers from the panel: they reply into the chat. Render server data with DOM APIs, never
  `innerHTML`.

## Go rules

- Never discard a DB error on a state-changing path.
- Every fire-and-forget goroutine starts with `defer error_handling.RecoverFromPanic(...)`. Async user/chat writers
  join the WaitGroup drained by `DrainUsersAsyncWrites`.
- Register every new secret with `logredact.RegisterSecret` (≥6 chars), or it is logged verbatim.
- A missing i18n key returns `""` with an error callers ignore; a typo'd key in Go ships an empty message. Copy key
  names from `locales/en.yml`. `locales/config.yml` is the help alt-name pseudo-locale, not a language.
- The docs generator takes help and `*_docs` text only from `locales/en.yml` (other languages are ignored), Module
  Aliases from `alt_names` in `locales/config.yml`, and command tables and lock types from `alita/modules`; run
  `make generate-docs` after changing any of them. Pages marked `<!-- MANUALLY MAINTAINED: do not regenerate -->` in
  their first 512 bytes are frozen; `api-reference/lock-types.md` ignores the marker.
- Imports: stdlib, third-party, internal, separated by blank lines. Use `helpers.Ptr[T]` for option pointers.

## Toolchain

- Production builds use `CGO_ENABLED=0`; tests need `CGO_ENABLED=1` (go-sqlite3). Do not unify them.
- Go 1.27.0 with no `toolchain` directive. Keep dependencies on the latest upstream versions that pass required
  checks. Any intentional pin must document its compatibility reason and recheck trigger.
- Version strings are shape-locked (`BotVersion:` with two spaces in `alita/config/config.go`, `version =` in
  `main.go`). Run `make bump-version` on a PR branch, merge, then tag. Tags match `^v\d+\.\d+\.\d+(-[0-9A-Za-z.]+)?$`.
- `release.yml` runs no checks of its own: it waits for `ci.yml` to pass on the tagged commit (dispatching CI on the
  tag if main's run was cancelled). Put any new release gate in `ci.yml`.
- `.env` in cwd auto-loads without overriding real env. Run `--version` smoke checks from `/tmp`.
- Defaults that surprise: `REDIS_DB=1` (with `REDIS_URL` set and `REDIS_DB` unset, the URL path decides; no path
  means DB 0); `ENABLE_AISPAM=true` (inert without `TYPESAFE_API_KEY`); perf monitoring and background stats default
  on only when `DEBUG=false` (an explicit `true` still enables them in debug); `HTTP_PORT` falls back to `PORT` then
  8080. Integer env parsing is lenient: a typo logs a warning and becomes the default, except `REDIS_DB` (a
  non-integer or a value outside 0-15 fails startup) and the shared-reputation settings `REPUTATION_MIN_CHATS` /
  `REPUTATION_WINDOW_DAYS` (a non-integer or an out-of-range value fails startup). Integers that parse but are out of
  range (`DB_*`, `DISPATCHER_MAX_ROUTINES`, `CACHE_LOCAL_*`, `HTTP_PORT`) also fail startup.

## Testing

- Always pass `-tags testtools`; without it, test files and the helpers in production packages that carry that build tag drop out silently.
- Postgres tests need both `DATABASE_URL` and `ALITA_TEST_DATABASE=true`; otherwise they run on SQLite.
- Use real fixtures: `internal/testdb.Run` (SQLite), miniredis, hand-written `gotgbot.BotClient` fakes. No mock libraries.
- Assert observable behavior (reply sent, row persisted, cache invalidated, gate enforced). Never assert literals,
  source substrings, or test-double internals.
- Handler tests act as `newAdminChatFixture`'s admin, not 777000/1087968824: those skip the admin cache. Use them
  only to test those IDs.
- In CI, keep the migration-chain step before `make test`; its `schema_migrations` rows back the checksum test.

## Commits

- Use Conventional Commits. The changelog drops `docs:`/`test:`/`chore:`/`ci:`/`deps:`, so user-visible changes need
  `feat:` or `fix:`.
