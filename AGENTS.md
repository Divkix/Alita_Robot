# AGENTS.md

Alita Robot: Telegram group-management bot. Go module `github.com/divkix/Alita_Robot`; PostgreSQL (GORM) + Redis.
Update this file in the same commit as any change that invalidates a rule below.

## Commands

```bash
make test                 # the only valid full test run (tags, race, coverage, skip gate)
make lint                 # golangci-lint v2
make check-translations   # missing locale keys
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

- Group numbers are execution order: `-10` captcha sweeper · `-6` fed-ban · `-5` antiraid · `-2` admin-cache ·
  `-1` users tracker · `0` commands/help/greetings · `3` aispam · `4` antiflood · `5`/`6` locks · `7` blacklists ·
  `8` reports+reactions · `9` filters · `10` pins · `11` log-channel.
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

- `alita/utils/chat_status` predicates return bools and never reply; use `PermissionResponder` to message.
  `helpers.CheckFunc` replies and is valid only inside `WrapCommand`.
- `IsUserAdmin` returns false for channel and non-positive IDs. Never pass a chat ID as a user ID.

## Data

- Repositories live in `alita/db/<domain>/`. Read via `cache.GetFromCacheOrLoad` with `cache.CacheKey` keys.
  **Every write must `cache.DeleteCache` the keys it affects.**
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

- Approved users skip antiflood, locks, blacklists, and captcha.
- Antiflood counters are in-process (per replica). Antiraid is Redis-only and does nothing without Redis.
- Fed-ban lookups cache per `(fed, user)` with a negative sentinel; every ban/unban write must invalidate it.
- A join arrives as both `ChatMemberUpdated` and a service message; dedupe through `claimRecentJoinProcessing`.
- Entity offsets are UTF-16: slice with `extractEntityText`, and match against both `Entities` and `CaptionEntities`.
- Captcha allows one attempt per `(user, chat)`; group `-10` stores the pending user's messages for replay.

## Go rules

- Never discard a DB error on a state-changing path.
- Every fire-and-forget goroutine starts with `defer error_handling.RecoverFromPanic(...)`. Async user/chat writers
  join the WaitGroup drained by `DrainUsersAsyncWrites`.
- Register every new secret with `logredact.RegisterSecret` (≥6 chars), or it is logged verbatim.
- A missing i18n key returns `""` with an error callers ignore; a typo'd key in Go ships an empty message. Copy key
  names from `locales/en.yml`. `locales/config.yml` is the help alt-name pseudo-locale, not a language.
- The docs generator reads only `locales/en.yml`. Pages marked `<!-- MANUALLY MAINTAINED: do not regenerate -->` in
  their first 512 bytes are frozen; `api-reference/lock-types.md` ignores the marker.
- Imports: stdlib, third-party, internal, separated by blank lines. Use `helpers.Ptr[T]` for option pointers.

## Toolchain

- Production builds use `CGO_ENABLED=0`; tests need `CGO_ENABLED=1` (go-sqlite3). Do not unify them.
- Go 1.26.0 with no `toolchain` directive. Do not bump `gotgbot/v2` or `gotg_md2html`; they are pinned on purpose.
- Version strings are shape-locked (`BotVersion:` with two spaces in `alita/config/config.go`, `version =` in
  `main.go`). Run `make bump-version` on a PR branch, merge, then tag. Tags match `^v\d+\.\d+\.\d+(-[0-9A-Za-z.]+)?$`.
- `.env` in cwd auto-loads without overriding real env. Run `--version` smoke checks from `/tmp`.
- Defaults that surprise: `REDIS_DB=1`; `ENABLE_AISPAM=true` (inert without `TYPESAFE_API_KEY`); perf monitoring and
  background stats are on only when `DEBUG=false`; `HTTP_PORT` falls back to `PORT` then 8080. Integer env parsing is
  lenient, so a typo silently becomes the default.

## Testing

- Always pass `-tags testtools`; without it, ~40 files (including helpers in production packages) drop out silently.
- Postgres tests need both `DATABASE_URL` and `ALITA_TEST_DATABASE=true`; otherwise they run on SQLite.
- Use real fixtures: `internal/testdb.Run` (SQLite), miniredis, hand-written `gotgbot.BotClient` fakes. No mock libraries.
- Assert observable behavior (reply sent, row persisted, cache invalidated, gate enforced). Never assert literals,
  source substrings, or test-double internals.
- In CI, keep the migration-chain step before `make test`; its `schema_migrations` rows back the checksum test.

## Commits

- Use Conventional Commits. The changelog drops `docs:`/`test:`/`chore:`/`ci:`/`deps:`, so user-visible changes need
  `feat:` or `fix:`.
