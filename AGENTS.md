# AGENTS.md

Alita Robot: Telegram group-management bot. Go module `github.com/divkix/Alita_Robot`; PostgreSQL (GORM) + Redis.

This file holds only rules that span packages. A rule about one package lives in the doc comment of that package
or of the symbol it governs, so it changes in the same diff as the code: read the doc comments of the files you
touch, and put a new package rule there. `internal/agentsmd` fails `make test` when a name or path in this file no
longer exists, or when the file outgrows its budget.

## Product

- Additive by default: a new feature leaves what existing commands, settings and defaults do unchanged.
- New behaviour a chat can see (spam checks, captcha, auto-actions) ships off and is enabled per chat.
- Change an existing default or command meaning only to fix a bug; commit it as `fix:` with the old and new
  behaviour in the body.
- Group-only and connection-based usage keep working when a PM or shortcut form is added.
- In PM a group the user names wins over the connected group. A command without arguments of its own
  (`PMGroupArgNone`) whose first word does not name exactly one group refuses (its help, the matching groups, or
  `settings_picker_wait`) and never falls back to the connection: the connection is invisible when the message is
  sent, and acting in the wrong group is worse than a refusal that costs one retry. Mechanics: `withPMGroupArg`.
- A PM reply about a group names it: end the text with `withGroupLine`.

## Commands

```bash
make test                 # the only valid full run: tags, race, coverage, skip gate
make lint                 # golangci-lint v2 over the whole tree (.golangci.yml); CI also gates new code on dupl/gocyclo
make nilaway              # nil-panic analysis; no suppression comments exist, so make the nil flow explicit
make check-translations   # missing locale keys and values copied from en.yml
make generate-docs        # after changing en.yml help/*_docs, config.yml alt_names, or command/lock tables
make check-formal         # TLC on formal/ (Docker)
make bump-version TAG=vX.Y.Z   # the only way to change version strings
go test -tags testtools -race -count=1 -run '^TestName$' ./alita/db/warns   # one test
CGO_ENABLED=0 go build ./...   # compile check
```

## Startup and shutdown

- `alita/db`'s `init()` connects to Postgres before `main()`, after the `alita/config` and `alita/utils/tracing`
  inits. Keep DB access out of any `init()` that can run before it, and guard new pre-`main` side effects with
  `isCliModeActive` (`--version`, `--health` and test binaries without `ALITA_TEST_DATABASE=true` skip DB init).
- Once config has loaded, exit through `log.Fatal*` or `log.StandardLogger().Exit`: its exit handler flushes
  telemetry, which `os.Exit` skips.
- Shutdown runs LIFO within 60 s: register a new drain after DB-close so it runs before it.

## Handlers

- Handler groups run in order: `-10` captcha sweeper · `-6` fed-ban · `-5` antiraid · `-4` spamdb · `-2`
  admin-cache · `-1` users tracker + bot-joined · `0` commands/help/greetings · `1` connected-chat guard ·
  `2` probation · `3` aispam · `4` antiflood · `5`/`6` locks · `7` blacklists · `8` reports+reactions ·
  `9` filters · `10` pins · `11` log-channel.
- gotgbot stops a group at its first matching handler: watchers return `ext.ContinueGroups`, commands
  `ext.EndGroups`. A group-0 watcher returning `nil` silently kills every later group-0 handler.
- Modules register in `init()` with `RegisterLegacyModule(name, priority, load)`. Names must be unique (a duplicate
  is dropped silently); priority sets load order and group-0 precedence.
- New commands use `helpers.WrapCommand(dispatcher, CommandDescriptor{...}, handler)`, with `Disableable: true` when
  chats may disable them. The pipeline sends the reply for a failed check.
- Anonymous admins bypass `WrapCommand`: an admin command that must work for them also registers
  `RegisterAnonymousAdminHandler` with `anonPipelineHandler` or `anonCommandContextHandler`.
- A PM command acting on a group points `ctx.EffectiveChat` at it, and later groups must not run on that ctx.
  `WrapCommand` ends iteration after such a swap. A legacy command relies on the group-1 guard, which sees only
  group-0 swaps, so one registered in a higher group returns `ext.EndGroups` on every path. Group 1 holds the guard
  alone.
- Read command text through `ctx.EffectiveMessage` or `ctx.Args()`: a PM group argument is cut from those, while
  `ctx.Message` keeps it. Group-only commands taken to PM and the buttons they send: `groupOnlyChat`.

## Callbacks

- Encode and decode callback data only through `alita/utils/callbackcodec` (`<ns>|v1|<url-encoded>`, 64-byte cap).
  `encodeCallbackData` returns `""` on overflow, which ships a dead button: keep user text in Redis behind a short
  token.

## Permissions and replies

- `chat_status` predicates return bools and send nothing; refuse through `PermissionResponder`. These reply
  themselves, so keep them out of watchers: `CanUser*` and `CanInvite` (the anonymous-admin proof keyboard),
  `IsUserConnected`, `UserHasPermissionForUpdate`.
- `helpers.CheckFunc` replies and needs a `*helpers.CommandContext`: run it through `WrapCommand`'s `RequiredChecks`,
  or `helpers.RunChecks` in an anonymous-admin post-proof handler.
- After a swap, `chat_status` helpers given a nil chat check the swapped group, and replies still go to the chat the
  update came from, in its language: `lang.GetLanguage(ctx)` returns that language, and `PermissionResponder` takes
  it through `chat_status.WithTranslator`. Read the group's own language with
  `lang.GetLanguage(&ext.Context{EffectiveChat: group})`.
- `IsUserAdmin` returns false for channel and non-positive IDs: pass a user ID, never a chat ID.
- `*ForUpdate` predicates are memoised per update: call them in watchers and pipeline checks, before any state
  change in the same update.

## Data

- Repositories live in `alita/db/<domain>/`. Reads go through `cache.GetFromCacheOrLoad` with `cache.CacheKey` keys;
  every write calls `cache.DeleteCache` on each key it affects. The loader is `alita/db/cache`, not
  `alita/utils/cache`; its in-process layer is described in `alita/db/cache/local.go`.
- Operational Redis keys (`alita:antiraid:*`, `alita:anonAdmin:*`, `alita:probation:*`, `alita:aispam:*`) sit outside
  the `alita:cache:` prefix, so `CLEAR_CACHE_ON_STARTUP` keeps them.
- `UpdateRecord` skips zero values; write `false`/`0`/`""` with `UpdateRecordWithZeroValues`. Both return
  `gorm.ErrRecordNotFound` when no row matched.
- Check `TableName()` before raw SQL: `ConnectionSettings` → `connection` (per user), `ConnectionChatSettings` →
  `connection_settings` (per chat), `AdminSettings` → `admin`, `DisableSettings` → `disable`.
- Return every DB error on a state-changing path.

## Migrations

- `migrations/*.sql` is the schema. Applied files are immutable (a SHA-256 checksum mismatch aborts startup); a new
  file's timestamp sorts after every existing name.
- Each file runs in one transaction: no top-level `BEGIN`/`COMMIT`/`ROLLBACK`, no `CREATE INDEX CONCURRENTLY`.
- `alita/db/migrations/runner.go` and `scripts/migrate_psql.sh` both apply migrations: change them together.

## Adding a module

1. Migration in `migrations/`; model in `alita/db/models/` whose `TableName()` returns the migration's table.
2. Repository in `alita/db/<domain>/`, following the cache rules above.
3. Add the model to the `AutoMigrate` list of each affected `testmain_test.go` or `internal/testdb.Run` call; do the
   same when changing an existing model.
4. `alita/modules/<name>.go` with `LoadXxx`, registered through `RegisterLegacyModule`; commands through
   `helpers.WrapCommand`, plus `RegisterAnonymousAdminHandler` for admin commands.
5. Locale keys in all 7 `locales/*.yml`. Help entries, examples and the `/commands` menu follow `commandHelpModules`
   and `menuCommands`; their tests name any missing entry.
6. `make generate-docs`, `make check-translations`, `make test`, `make lint`.

## Cross-cutting traps

- Approved users skip every automatic check (antiflood, antiraid, aispam, spam filter, locks, blacklists, captcha,
  probation, report auto-mute); a new one skips them too.
- A join arrives as both a `ChatMemberUpdated` and a service message: dedupe through `claimRecentJoinProcessing`.
- Entity offsets are UTF-16: slice with `extraction.ExtractEntityText`, and match both `Entities` and
  `CaptionEntities`.
- Messages the bot sends on its own, not as a reply, go through `ratelimit.GlobalSendLimiter().Do`.
- A missing i18n key returns `""` with an error callers ignore, so a typo ships an empty message: copy key names
  from `locales/en.yml`. `locales/config.yml` is the help alt-name pseudo-locale, not a language.

## Go rules

- A goroutine you start fire-and-forget begins with `defer error_handling.RecoverFromPanic(...)`; async user/chat
  writers join the WaitGroup that `DrainUsersAsyncWrites` drains.
- Each tick of a background job doing I/O runs through `jobs.Run(ctx, "fixedName", fn)` (own root trace, recovered
  panics). Log with `log.WithContext(ctx)` where a ctx exists, so lines carry `trace_id`.
- Register every new secret with `logredact.RegisterSecret` (6+ chars), or it is logged verbatim.
- Bot API, GORM and Redis calls get spans only on the update context: pass `tracing.UpdateContext(ctx)` to the
  `*WithContext` method or `DB.WithContext`. Spans carry no message text, SQL, bound values, Redis arguments or Bot
  API URL (it holds the token).

## Toolchain

- Production builds use `CGO_ENABLED=0`; tests need `CGO_ENABLED=1` (go-sqlite3). Keep them split.
- `go.mod` sets the Go version with no toolchain directive. Keep dependencies on the latest versions that pass CI;
  document any pin with its reason and recheck trigger.
- Release: `make bump-version` on a PR branch, merge, then tag `vX.Y.Z[-pre]`. `release.yml` runs no checks of its
  own and waits for `ci.yml` on the tagged commit, so a new release gate goes in `ci.yml`.
- `.env` in cwd auto-loads without overriding real env: run `--version` smoke checks from `/tmp`. Surprising config
  defaults are listed in the `alita/config` package doc.

## Testing

- Pass `-tags testtools` to every `go test`: without it, tagged test files and helpers drop out silently.
- Postgres tests run only when both `DATABASE_URL` and `ALITA_TEST_DATABASE=true` are set, else SQLite; a stale shell
  export of either re-targets the suite.
- Any skipped test fails `make test` unless `scripts/check_test_results` allowlists it.
- Fixtures are real: `internal/testdb.Run` (SQLite), miniredis, hand-written `gotgbot.BotClient` fakes.
- Assert observable behaviour (reply sent, row persisted, cache invalidated, gate enforced), not literals, source
  substrings or test-double internals.
- Handler tests act as `newAdminChatFixture`'s admin. 777000 and 1087968824 skip the admin cache; use them only to
  test those IDs.

## Commits

- Conventional Commits. The changelog drops `docs:`/`test:`/`chore:`/`ci:`/`deps:`, so a user-visible change needs
  `feat:` or `fix:`.
