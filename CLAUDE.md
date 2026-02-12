# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with
code in this repository.

## Project Overview

Alita Robot is a Telegram group management bot built with Go 1.25+ and the
gotgbot library. Features include user management, filters, greetings,
anti-spam, captcha verification, and multi-language support (en, es, fr, hi).

## Development Commands

```bash
make run                # Run the bot locally (go run main.go)
make build              # Multi-platform release build via GoReleaser
make lint               # golangci-lint code quality checks (run before commits)
make test               # go test ./...
make tidy               # go mod tidy
make vendor             # go mod vendor
make check-translations # Detect missing translation keys across locales
make generate-docs      # Generate documentation site content
make docs-dev           # Start Astro docs dev server (uses bun)
```

### Database Migrations

```bash
make psql-migrate       # Apply pending migrations (requires PSQL_DB_* env vars)
make psql-status        # Check migration status
make psql-reset         # DROP ALL TABLES — destructive, requires confirmation
```

Auto-migration on startup: set `AUTO_MIGRATE=true`. Supabase-specific SQL
(GRANT, RLS) is auto-cleaned at runtime. Migrations tracked in
`schema_migrations` table, executed transactionally and idempotently.

## Architecture

### Startup Flow (main.go)

1. Locale manager init (singleton, embedded YAML via `go:embed`)
2. HTTP transport with connection pooling (optional API server rewriting)
3. Bot init + Telegram API connection pre-warming
4. Database (GORM/PostgreSQL) and cache (Redis) initialization
5. Dispatcher creation (configurable max goroutines)
6. Monitoring systems: background stats, auto-remediation (GC triggers), activity monitor
7. Graceful shutdown manager (LIFO handler execution, 60s timeout)
8. Unified HTTP server (health + metrics + webhook on single port)
9. Mode selection: webhook or polling
10. Module loading via `alita.LoadModules(dispatcher)`

### Module System

Modules live in `alita/modules/`. Each module exposes a `LoadXxx(dispatcher)`
function called explicitly from `alita/main.go:LoadModules()` — **not** Go
`init()` functions. Load order matters; help module loads last to collect all
registered modules.

**Module structure pattern:**
- Value receiver `(m moduleStruct)` on handler methods (stateless, no locks)
- Handlers return `ext.EndGroups` (stop propagation) or `error`
- Commands registered via `dispatcher.AddHandler()` with handler groups
- Disableable commands added via `misc.AddCmdToDisableable()`
- Help keyboards registered in `HelpModule.AbleMap` (sync.Map)

**Adding a new module:**
1. Create DB operations in `alita/db/*_db.go`
2. Implement handlers in `alita/modules/your_module.go`
3. Create `LoadYourModule(dispatcher)` function
4. Call it from `LoadModules()` in `alita/main.go`
5. Add translation keys to ALL locale files in `locales/`

### Database Layer

**PostgreSQL + GORM** with connection pooling (configurable via env vars).

**Surrogate key pattern:** All tables use auto-increment `id` as PK. External
IDs (`user_id`, `chat_id`) have unique constraints but aren't primary keys.
Exception: `chat_users` join table uses composite PK `(chat_id, user_id)`.

**File organization:**
- `alita/db/db.go` — GORM models, connection setup, pool config
- `alita/db/*_db.go` — Domain-specific operations (`Get*`, `Add*`, `Update*`, `Delete*`)
- `alita/db/cache_helpers.go` — TTL management, cache invalidation
- `alita/db/optimized_queries.go` — Batch prefetching, bulk parallel operations
- `alita/db/shared_helpers.go` — Transaction support with auto-rollback
- `alita/db/migrations.go` — Runtime migration engine
- `migrations/*.sql` — Source of truth for schema (timestamped filenames)

### Cache Layer

Redis-only via gocache library. Stampede protection via `singleflight`.

- Key format: `alita:{module}:{identifier}` (e.g., `alita:adminCache:123`)
- Operations: `cache.Marshal.Get/Set/Delete` with context
- Admin cache specialized in `alita/utils/cache/adminCache.go`
- **Cache must be invalidated on writes** — every DB update function that
  modifies cached data must call the corresponding invalidation

### Permission System (`alita/utils/chat_status/`)

- `RequireGroup()` / `RequirePrivate()` — chat type guards
- `RequireBotAdmin()` / `RequireUserAdmin()` — permission guards
- `RequireUser()` — extracts valid user (nil for channels)
- `IsUserAdmin()` — bool check (returns false for channel IDs)
- Anonymous admin detection with keyboard fallback
- Results cached to reduce Telegram API calls

### Internationalization (`alita/i18n/`)

Singleton `LocaleManager` with per-language `Translator` instances. YAML locale
files embedded via `go:embed`. Supports named parameters in code
(`{"user": value}`) mapped to positional formatters (`%s`) in YAML.

### Error Handling

Four-layer recovery: dispatcher → worker pool → decorator → handler. The
`error_handling` package provides `RecoverFromPanic()`, `HandleErr()`, and
`CaptureError()`. Expected Telegram API errors (bot not admin, chat closed)
are filtered via `helpers.IsExpectedTelegramError()`.

### Graceful Shutdown (`alita/utils/shutdown/`)

Central coordinator. Handlers registered during setup, executed in LIFO order
on shutdown. Each handler gets panic recovery. Total timeout: 60 seconds.

### Monitoring (`alita/utils/monitoring/`)

- **Activity monitor**: Tracks `last_activity` per chat, marks inactive after threshold
- **Auto-remediation**: GC triggers when memory/goroutine thresholds exceeded
- **Background stats**: 5-minute collection of goroutines, memory, GC metrics

## Environment Configuration

See `sample.env` for all variables. Critical ones:

- `BOT_TOKEN`, `DATABASE_URL`, `REDIS_ADDRESS`, `MESSAGE_DUMP`, `OWNER_ID` (required)
- `HTTP_PORT` (default 8080) — unified for health, metrics, webhook
- `USE_WEBHOOKS`, `WEBHOOK_DOMAIN`, `WEBHOOK_SECRET` — webhook mode
- `AUTO_MIGRATE` — enable startup migrations
- `DEBUG` — verbose logging (performance monitoring auto-disabled when true)

## Deployment

**Polling mode** (default): Simple, no external config. Higher latency.
**Webhook mode**: Production. Requires HTTPS (Cloudflare Tunnel supported).

Docker: Multi-stage build → distroless image. Health check via `--health` flag.
CI/CD: GitHub Actions with gosec, govulncheck, golangci-lint, multi-arch Docker.
Releases: GoReleaser to `ghcr.io/divkix/alita_robot`.

## Critical Rules

These are hard-won patterns from past bugs. Violating them causes real issues.

### Go Patterns
- **Never ignore DB errors with `_`**: Always check `err` — nil returns cause panics
- **Nil sender check**: `ctx.EffectiveSender` can be nil (channel messages). Check before accessing `.User`
- **`IsUserAdmin` returns false for channel IDs** (negative numbers < -1000000000000)
- **Sync before confirm**: DB writes that need user confirmation must be synchronous, not goroutines
- **Async DB wrappers**: Fire-and-forget `go db.X()` loses errors. Wrap in functions that log errors
- **Struct alias fields**: When a struct has related fields (e.g., `Dev` and `IsDev`), set both consistently everywhere

### Handler Patterns
- **Handler groups**: Use negative numbers (-10) for early interception handlers
- **Return values**: `ext.EndGroups` stops propagation, `ext.ContinueGroups` continues
- **Callback data validation**: Always check `len(strings.Split(data, "."))` before indexing
- **Double-answer bug**: `RequireUserAdmin` with `justCheck=false` already answers the callback — don't answer again
- **`IsUserConnected()`** sets `ctx.EffectiveChat` internally — no need to reassign after calling it
- **Entity completeness**: Check both `msg.Entities` AND `msg.CaptionEntities` for URL/mention detection

### i18n Patterns
- **YAML quoting**: Use double quotes for strings with escape sequences (`\n`, `\t`). Single quotes preserve them literally
- **Printf type safety**: `%d` requires int, not `strconv.Itoa()` output
- **Key verification**: Always grep `locales/` to confirm keys exist in ALL locale files before using them
- **Parse mode**: Locale strings use Markdown but bot sends HTML. Convert via `tgmd2html.MD2HTMLV2()`

### Database Patterns
- **Schema-struct sync**: Every DB function parameter must map to an actual column. Add migration → update struct → update optimized queries → update function
- **Cache invalidation on writes**: Every update function must invalidate the corresponding cache key
- **Surrogate keys**: Always use auto-increment `id` as PK, external IDs as unique constraints
- **Composite indexes**: Add for frequent query patterns on `(user_id, chat_id)`

### Boolean Logic
- **Filter functions**: `IsAnonymousChannel() || !IsLinkedChannel()` matches almost everything. Test filter logic with multiple message types before shipping
