# Repository Guidelines

Alita Robot is an English-only Telegram group-management bot written in Go using
`gotgbot/v2`. `AGENTS.md` is the source of truth for contributor guidance;
`CLAUDE.md` and `GEMINI.md` are symlinks to it.

Keep this file focused on non-obvious, repository-wide constraints. Update an
existing section when a change invalidates it; do not duplicate facts that are
easy to discover from code, tests, `sample.env`, or workflow files.

## Project map

- `main.go`: process bootstrap, polling/webhook selection, HTTP transport, and
  shutdown wiring.
- `alita/modules/`: Telegram commands, callbacks, watchers, and module registry.
- `alita/db/models/`: GORM models; `alita/db/<domain>/`: domain repositories.
- `alita/db/cache/`: repository read-through cache; `alita/utils/cache/`: admin
  cache; `alita/utils/state/`: the in-process TTL store both build on.
- `alita/i18n/` and `locales/`: embedded English strings and command metadata.
- `alita/db/migrations/sqlite/`: embedded SQLite baseline and forward-only
  migrations.
- `internal/repo_checks/`: source-structure assertions that may need updates after
  renames or refactors.
- `scripts/check_translations/`: separate Go module for locale validation.

## Commands

```bash
make run
make build
make lint
make test
make check-translations
make check-duplicates
make tidy

# Focused tests
go test -v -run TestXxx ./path/to/package

# Database tooling
make validate-db
```

The normal test suite uses temporary SQLite files via the embedded baseline.
Race tests require CGO and a C toolchain. Consult `.github/workflows/ci.yml` for
the current coverage gate and exact CI versions.

## Architecture and startup

- Importing `alita/config` loads and validates global configuration; importing
  `alita/db` opens SQLite at `SQLITE_PATH` (default `/data/alita.db`) and applies
  embedded migrations. Both happen in package `init()` and short-circuit for
  supported CLI flags or missing test configuration. Do not move them into
  `main()` without redesigning all import-time behavior.
- `main()` initializes locales and modules, then the bot, DB anchors, dispatcher,
  shutdown, the health server, and polling or webhook delivery.
- Neither transport asks Telegram to discard queued updates on startup: a restart
  must not drop moderation actions buffered while the bot was down.
- Modules self-register in `init()` and load by ascending priority. Help is
  deliberately loaded last so all module metadata is available.
- Handler groups define update flow: negative groups are early interceptors,
  group 0 is commands, and positive groups are watchers. Commands normally return
  `ext.EndGroups`; watchers return `ext.ContinueGroups`.
- Shutdown handlers run LIFO under one overall timeout. Register them in inverse
  dependency order and give every background service a stop-and-join path.

## Handlers, callbacks, and permissions

- Prefer the declarative command pipeline in
  `alita/utils/helpers/command_pipeline.go` for new commands. Legacy handlers
  remain valid where connection or anonymous-admin behavior requires them.
- `ctx.EffectiveSender` and callback messages may be nil. Use
  `callbackQueryFromContext`, `chat_status.GetEffectiveUser`, or
  `chat_status.RequireUser` instead of unchecked access.
- `IsUserAdmin` intentionally rejects channel IDs and all non-positive user IDs.
  Do not weaken this or pass chat IDs as user IDs.
- After `IsUserConnected`, assign the returned chat to `ctx.EffectiveChat`.
- Anonymous-admin redispatch bypasses command-pipeline checks; wrappers on that
  path must enforce permissions explicitly.
- Chat-scoped deep links must re-check `chat_status.IsUserInChat`; admin-only
  content must also re-check admin status.
- Use `alita/utils/callbackcodec` for callback data. Telegram limits it to 64
  bytes. Never parse raw callback data with `strings.Split`; store large or
  user-controlled payloads in the in-process TTL store behind a short,
  user-bound token.
- Permission helpers may already answer callback queries. Do not answer a second
  time after a failed check.
- State changes that gate a confirmation or success response must complete
  synchronously. Never report success after a persistence error.
- Every fire-and-forget goroutine needs
  `defer error_handling.RecoverFromPanic(function, module)`.

## Database and migrations

- Embedded SQL in `alita/db/migrations/sqlite/*.sql` is the production schema
  source of truth. For schema changes, add a newer timestamped embedded
  migration, update the model, optimized query columns, and repository behavior.
- Applied migrations are immutable because the runner verifies SHA-256 checksums
  of raw bytes. Always add a newer timestamped migration.
- Models use surrogate `ID` primary keys; Telegram IDs are separate unique
  columns. Confirm each model's `TableName()` and actual migration columns before
  writing raw SQL.
- GORM struct updates omit zero values. Persist `false`, `0`, or `""` with
  `UpdateRecordWithZeroValues` and a map.
- Repository writes must invalidate every affected cache using the exact existing
  key prefix. Prefixes are not consistently the same as package or table names,
  so follow nearby repository code rather than guessing.
- Open connections through `db.OpenSQLite` so tests share the runtime PRAGMAs
  and pool limits. Never enable GORM's `PrepareStmt` for SQLite: cached
  statements stay open past a transaction and go-sqlite3 then rejects the
  COMMIT with "SQL statements in progress".
- Use existing conflict-safe upsert and `db.RetryOnLock` patterns for
  concurrently writable data. Do not replace them with read-then-create logic.
- Repository reads often return safe defaults instead of propagating errors; do
  not use those defaults as proof that a row exists.

## Cache and ephemeral state

- Retained repository caching (`alita/db/cache`) and the Telegram administrator
  cache live in the in-process TTL store (`alita/utils/state`). Cached values are
  shared between callers: treat reads as read-only, or return a copy as
  `AdminCache` does.
- Do not bypass repository `DeleteCache` or `InvalidateAdminCache`: both bump an
  invalidation generation that prevents an in-flight load from restoring stale
  data, and `InvalidateAdminCache` also forgets the in-flight singleflight load.
- Keep independent named caches for filters and blacklists. Sharing them causes
  cross-module eviction.
- One-use confirmation payloads use the state store's consume-once
  `GetAndDelete`; preserve user binding, TTLs, and replay protection.
- Tests that must observe the database directly disable repository caching with
  `dbcache.SetEnabled(false)` and reset in-process entries with
  `state.SimulateRestart()`.

## Locale and message content

- The bot is English-only. User-facing strings belong in `locales/en.yml`;
  `locales/config.yml` is a pseudo-locale containing command aliases and DB
  defaults and must remain embedded.
- `GetString` supports named and legacy positional placeholders. Extend the
  ordered-key mapping when introducing positional parameters with new names.
- Most locale text is Markdown but Telegram sends use HTML; follow neighboring
  code and apply `tgmd2html.MD2HTMLV2` where required.
- Moderation matching must inspect both `Entities` and `CaptionEntities`. Telegram
  entity offsets are UTF-16 units; extract them with the existing helper rather
  than byte or rune slicing.
- Stored note/filter buttons support URLs only. Callback buttons are discarded.
- `extraction.ExtractUserAndText` uses `-1` for an error already shown to the user
  and `0` for no target; keep those cases distinct.

## Operational and security invariants

- Required and optional environment variables are documented in `sample.env` and
  loaded manually in `alita/config/config.go`; struct validation/env tags are not
  authoritative.
- Add every new secret configuration field to the `logredact.RegisterSecret`
  call. Never log tokens, credentials, webhook secrets, or authorization headers.
- `BOT_TOKEN` and `OWNER_ID` are the only required variables. `SQLITE_PATH`,
  `HTTP_PORT`, `LOG_LEVEL`, `MESSAGE_DUMP`, `USE_WEBHOOKS`, and `OWNER_CHAT_IDS` are optional;
  `WEBHOOK_DOMAIN` and `WEBHOOK_SECRET` are validated only in webhook mode.
- The HTTP server exposes `/health` (and `/webhook` in webhook mode) and nothing
  else. There is no OpenTelemetry, Prometheus, `/metrics`, `/db_metrics`, pprof,
  background-statistics, remediation, or activity-worker stack; `internal/repo_checks`
  fails the build if one returns.
- Webhook requests use a static `/webhook` path and authenticate via the secret
  header. Preserve constant-time secret comparisons and the request-size limit.
- Build identity is `config.Commit`: `dev` locally, and the short commit SHA when
  injected via `-ldflags "-X .../alita/config.Commit=<sha>"`. `--version` and
  `/health` both report it. There is no semver to bump.
- `docker/alpine` is the only image definition and `docker-compose.yml` the only
  deployment: one non-root `linux/amd64` container, one `alita_data` volume
  mounted at `/data`. Debug, PR-build, and GoReleaser images and the local Bot
  API service are gone; `internal/repo_checks` fails the build if one returns.
  `scripts/docker-acceptance/run.sh` proves the image starts from an empty
  volume, becomes healthy, and keeps its SQLite database across a recreation.
- Deployment on the Docker host is manual: snapshot the volume, pull, and
  recreate the container. No workflow touches the host.
- PR verification is handled by `.github/workflows/ci.yml`. Release behavior is
  defined by `.github/workflows/release.yml`; images are built and pushed to GHCR
  when a tag is pushed, injecting `config.Commit` through the `COMMIT` build argument.
- Treat `gotgbot/v2` release-candidate changes and `gotg_md2html` pseudo-version
  changes as compatibility-sensitive; do not auto-merge them without review.

## Go conventions

- Run `gofmt`; group imports as standard library, third-party, then internal.
- Use exported PascalCase and unexported camelCase names. Keep tests in the same
  package and name them `TestXxx`.
- Use `helpers.Ptr[T]` for pointer literals in gotgbot options.
- Add user-facing strings to `locales/en.yml` and run the relevant focused tests,
  `make lint`, `make test`, and `make check-translations` as appropriate.
- Never commit secrets or `.env` files. Preserve unrelated worktree changes and
  stage only files relevant to the task.
