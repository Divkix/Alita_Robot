# Repository Guidelines

Alita Robot — a Telegram group-management bot. **Go 1.26**, gotgbot/v2 `v2.0.0-rc.36`, PostgreSQL (GORM) + Redis.
~30 feature modules, 7 locales. One binary serves long-polling *or* webhook plus `/health`, `/metrics`, `/db_metrics`.

> Couplings and traps only — nothing a quick file read already shows. Update this file in the same commit as the change it describes.

## Project Overview

- Single Go module `github.com/divkix/Alita_Robot`. `main.go` + `alita/` is the binary, `scripts/` is tooling, no `vendor/`.
- `migrations/*.sql` is the schema source of truth; `gorm.AutoMigrate` is never used in production.
- Deploy manifests all set `AUTO_MIGRATE=true` even though the code default is `false`.
- `CONTEXT.md` is the domain glossary; `docs/agents/` holds process rules.

## Architecture & Data Flow

```
Telegram (polling | POST /webhook) → dispatcher → handler groups ascending → module → repo → Postgres ⇄ Redis → reply
```

**Init order is load-bearing.** `alita/config` `init()` (env, logging, `logredact` secrets) → `alita/db` `init()`
**connects to Postgres before `main()` runs** and applies migrations when `AUTO_MIGRATE=true` → `main()`: cache → i18n →
tracing → bot → dispatcher → HTTP server → `postInit` (modules → captcha lifecycle → `WorkingMode` → commands).
`--version`/`--health` short-circuit all of it, as does a `*.test` binary unless `ALITA_TEST_DATABASE=true`.

**Shutdown is LIFO** (reverse registration, 60 s budget). Drains (`DrainUsersAsyncWrites`, `DrainAISpamChecks`, …) must
happen before DB close — that's why DB-close is registered first.

**Handler groups** — the numbers are ordering, not labels:
`-10` captcha message sweeper · `-6` federations fed-ban · `-5` antiraid · `-2` admin-cache refresh · `-1` users tracker ·
`0` commands/help/greetings · `3` aispam (after -1 so rows exist, before 4 so floods don't burn model calls) · `4` antiflood ·
`5`/`6` locks perm/restr · `7` blacklists · `8` reports + reactions · `9` filters · `10` pins · `11` log-channel capture.

⚠️ **gotgbot ends the group at the first matching handler.** Watchers must return `ext.ContinueGroups`; commands
`ext.EndGroups`. A group-0 watcher returning `nil` silently disables every later group-0 handler for that update.

**Modules** self-register in `init()` via `RegisterLegacyModule(name, priority, load)` — dedupe by name (duplicates are
silently ignored), loaded ascending by priority, `LoadHelp` last. Priority also sets group-0 precedence.

**Commands:** `helpers.WrapCommand(dispatcher, CommandDescriptor{...}, handler)`; its checks send their own failure
replies, and `Disableable: true` is what makes per-chat disabling possible. Legacy path: `handlers.NewCommand` +
`helpers.MultiCommand` + `AddCmdToDisableable`. **Anonymous admins bypass the pipeline** — new admin commands that must
work for them need `RegisterAnonymousAdminHandler` + `anonPipelineHandler`, which re-runs `helpers.RunChecks`.

**Callbacks:** `alita/utils/callbackcodec` only — `<ns>|v1|<url-encoded>`, 64-byte cap; `encodeCallbackData` returns `""`
on overflow, which ships a dead button. Never `strings.Split` callback data. User text goes in Redis behind a short token.

**Permissions** (`alita/utils/chat_status`): predicates return bools and never reply; `PermissionResponder` messages;
`helpers.CheckFunc` replies and is only valid inside `WrapCommand`. `IsUserAdmin` is false for channel and non-positive
IDs — never pass a chat ID where a user ID is expected.

**Reads/writes:** repositories in `alita/db/<domain>/`; reads via `cache.GetFromCacheOrLoad` with keys from
`cache.CacheKey` (`alita:cache:` prefix, 30-minute TTLs). **Every write must `cache.DeleteCache` its keys.** Two packages
are named `cache` — the loader and its generation guards live only in `alita/db/cache`. Operational keys
(`alita:antiraid:*`, `alita:anonAdmin:*`, …) sit outside the prefix, so `CLEAR_CACHE_ON_STARTUP` can't wipe them.

**Migrations are immutable** — SHA-256 over raw bytes, so editing an applied file aborts startup. Apply order is plain
filename sort: always add a greater timestamp. One transaction per file, so no top-level `BEGIN`/`COMMIT`/`ROLLBACK` and
no `CREATE INDEX CONCURRENTLY`. Both appliers (`alita/db/migrations/runner.go`, `scripts/migrate_psql.sh`) must agree.

**Table names ≠ struct names** — check `TableName()` before raw SQL:
`ConnectionSettings→connection` (per user) vs `ConnectionChatSettings→connection_settings` (per chat),
`AdminSettings→admin`, `DisableSettings→disable`.

**Subsystem traps**, one line each: approvals short-circuit antiflood/locks/blacklists/captcha · antiflood counters are
in-process, so flood state is per replica · antiraid is Redis-only and silently inert without Redis · fed-ban lookups are
cached per `(fed, user)` with a negative sentinel, so ban/unban writes must invalidate · a join arrives as *both* a
`ChatMemberUpdated` and a service message, deduped via `claimRecentJoinProcessing` · entity offsets are UTF-16 (slice with
`extractEntityText`) and matching must read `Entities` **and** `CaptionEntities` · captcha allows one attempt per
`(user, chat)` and group `-10` stores the pending user's messages for replay.

## Key Directories

`alita/config` (env) · `alita/db` (+ `<domain>/` repos, `models/`, `cache/`, `migrations/`, `backup/`) · `alita/modules`
(feature modules) · `alita/i18n` · `alita/utils` (`helpers`, `chat_status`, `cache`, `callbackcodec`, `media`, `tracing`,
`monitoring`, `shutdown`, `httpserver`, `error_handling`, `logredact`) · `locales/` (7 locales + `config.yml`
pseudo-locale) · `migrations/` · `internal/testdb/` (SQLite fixture) · `scripts/` · `docs/` (Blume site) · `docker/`.

## Development Commands

```bash
make test                     # full suite; gated by scripts/check_test_results (see Testing & QA)
make lint                     # golangci-lint v2
make build                    # goreleaser snapshot — needs goreleaser v2 + Docker, NOT plain `go build`
make test-postgres-integrity  # needs DATABASE_URL AND ALITA_TEST_DATABASE=true
make generate-docs            # after module/help changes; `make check-docs` fails on drift
make check-translations       # missing-key gate (separate Go module)
make psql-migrate             # manual migrations; PSQL_DB_* vars, reads scripts/.env
make bump-version TAG=vX.Y.Z  # the only safe way to change version strings
go test -tags testtools -race -count=1 -run '^TestName$' ./alita/db/warns   # single test
```

## Code Conventions & Common Patterns

- `gofmt`; imports stdlib → third-party → internal, blank-line separated; `helpers.Ptr[T]` for option pointers.
- Handler methods take value receivers on `moduleStruct`.
- Never discard a DB error on a state-changing path. Every fire-and-forget goroutine needs
  `defer error_handling.RecoverFromPanic(...)`, and async user/chat writers must join the WaitGroup drained by
  `DrainUsersAsyncWrites`.
- `UpdateRecord` skips zero values → use `UpdateRecordWithZeroValues` for `false`/`0`/`""`; both return
  `gorm.ErrRecordNotFound` when nothing matched.
- Register new secrets with `logredact.RegisterSecret` (≥6 chars) or they get logged verbatim.
- i18n: new keys go in **all 7 locales**. A missing key returns `""` plus an error that callers ignore, so a typo ships an
  empty message. `locales/config.yml` is the `"config"` pseudo-locale (help alt-names) and is not selectable.
- Commits use Conventional Commits; the changelog drops `docs:`/`test:`/`chore:`/`ci:`/`deps:`, so user-visible work needs
  `feat:`/`fix:`.
- Adding a module: migration → model → repo → `LoadXxx` with `RegisterLegacyModule` → locale keys → `make generate-docs`.
  Changing a model also means updating the `AutoMigrate` lists in the relevant `testmain_test.go` or `internal/testdb.Run` call.

## Important Files

| File | Why it matters |
|---|---|
| `main.go` | startup/shutdown order, `postInit`, webhook vs polling |
| `alita/config/config.go` | env defaults, validation, `BotVersion` literal |
| `alita/db/conn.go` | pre-`main()` connect, `isCliModeActive` guards |
| `alita/db/db.go` | generic CRUD helpers, `RowsAffected == 0` semantics |
| `alita/db/migrations/runner.go` | ordering, advisory lock, checksums |
| `alita/db/cache/loader.go` | `GetFromCacheOrLoad` and generation invalidation |
| `alita/utils/callbackcodec/` | callback encoding and the 64-byte cap |
| `alita/modules/registry.go` / `core.go` | registration, priorities, help registry |
| `alita/modules/users.go` | group -1 tracker whose rows every other group assumes |
| `alita/utils/helpers/command_pipeline.go` | the `WrapCommand` command standard |
| `Makefile` | the only sanctioned entry point for every gate |

## Runtime/Tooling Preferences

- **CGO split — do not unify:** production builds `CGO_ENABLED=0`; tests need `CGO_ENABLED=1` (go-sqlite3 fixtures).
- Go 1.26.0, no `toolchain` directive. gotgbot and `gotg_md2html` are pinned deliberately — Dependabot won't auto-merge them.
- Versions live in two string-shape-locked places (`BotVersion:` with two spaces in `alita/config/config.go`, `version =`
  in `main.go`); release tags must match `^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$`.
- `.env` is auto-loaded from cwd and never overrides real env vars; CI runs `--version` from `/tmp` to avoid it.
- Surprising defaults: `REDIS_DB=1`; `ENABLE_AISPAM=true` (inert without `TYPESAFE_API_KEY`); performance monitoring and
  background stats default true only when `DEBUG=false`; `HTTP_PORT` falls back to `PORT` then 8080. `typeConvertor` is
  lenient, so a typo'd integer silently becomes the default.

## Testing & QA

- `make test` is not plain `go test`: it pipes `go test -tags testtools -json -race -coverprofile=coverage.out
  -coverpkg=<alita/...> -count=1 -timeout 10m ./...` into `scripts/check_test_results`, which fails on failures **and on
  any `t.Skip` outside a two-entry allowlist** (`alita/db/migrations`; `alita/db/chats`/`TestUpdateChat`). A stray skip is a build break.
- `-tags testtools` is effectively mandatory: ~40 files carry it, including three non-test helper files inside production
  packages. Plain `go test ./...` silently runs a subset.
- Fixtures are real, there is no mock library: SQLite via `internal/testdb.Run`, miniredis or an in-memory marshaler, and
  hand-written `gotgbot.BotClient` fakes.
- Postgres needs `DATABASE_URL` **and** `ALITA_TEST_DATABASE=true`; a `*.test` binary otherwise stays on SQLite even when
  `DATABASE_URL` is exported.
- Coverage gate is **78%**, measured over `./alita/...` only (root `main` and `scripts/` are excluded from the number).
- CI runs the migration-chain test *before* `make test`; the `schema_migrations` rows it leaves are what keep the checksum
  test meaningful. Don't reorder those steps.
- Assert observable behavior (reply sent, row persisted, cache invalidated, gate enforced) — never literals, source
  substrings, or test-double internals.
- Lint: `godox`, `dupl` (100), `gocyclo` (20) with `issues.new: true`, so only new issues fail; `_test.go` is exempt from
  the complexity linters.
- Docs drift is a gate: changing module commands or `en.yml` help keys requires `make generate-docs`. The generator reads
  **only `locales/en.yml`**. Frozen pages need `<!-- MANUALLY MAINTAINED: do not regenerate -->` in their first 512 bytes
  (`api-reference/lock-types.md` ignores the sentinel entirely).
