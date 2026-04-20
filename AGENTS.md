# Repository Guidelines

Alita Robot is a Telegram group management bot built with Go 1.25+ and the
gotgbot library. Features include user management, filters, greetings,
anti-spam, captcha verification, and multi-language support (en, es, fr, hi).

## Project Structure & Module Organization

- **`alita/`** - Core application code
  - `config/` - Configuration management
  - **`db/`** - Database models, operations, and caching (PostgreSQL + GORM) — [see @db/AGENTS.md](./alita/db/AGENTS.md)
  - `i18n/` - Internationalization with embedded YAML locales
  - **`modules/`** - Bot functionality modules (admin, filters, greetings, etc.) — [see @modules/AGENTS.md](./alita/modules/AGENTS.md)
  - `utils/` - Shared utilities (permissions, error handling, monitoring)
- **`locales/`** - Translation files (en, es, fr, hi, ru, pt, id)
- **`migrations/`** - Database schema migrations (timestamped SQL files)
- **`scripts/`** - Support scripts (translation checks, docs generation)
- **`docs/`** - Documentation site (Astro-based)

## Build, Test & Development Commands

```bash
# Development
make run                # Run the bot locally (go run main.go)
make build              # Multi-platform release build via GoReleaser

# Code quality (run before commits)
make lint               # golangci-lint code quality checks (run before commits)
make test               # Run all tests with race detection, coverage
make tidy               # go mod tidy
make vendor             # go mod vendor

# Single test execution patterns:
go test -v -run TestFunctionName ./package       # Run specific test
go test -v -run "^TestXxx$" ./alita/db          # Run tests matching pattern
go test -v -count=1 -timeout 10m ./alita/db      # Run all tests in package

# Translations & docs
make check-translations # Detect missing translation keys across locales
make generate-docs      # Generate documentation site content
make docs-dev           # Start Astro docs dev server (uses bun)

# Database migrations (requires PSQL_DB_* env vars)
make psql-migrate       # Apply pending migrations
make psql-status        # Check migration status
make psql-reset         # DROP ALL TABLES — destructive, requires confirmation
```

**Auto-migration on startup:** set `AUTO_MIGRATE=true`. Supabase-specific SQL
(GRANT, RLS) is auto-cleaned at runtime. Migrations tracked in
`schema_migrations` table, executed transactionally and idempotently.

## Code Style & Conventions

### Imports
Order: standard library → third-party → internal. Group with blank lines:

```go
import (
    "context"
    "fmt"

    "github.com/PaulSonOfLars/gotgbot/v2"
    log "github.com/sirupsen/logrus"
    "gorm.io/gorm"

    "github.com/divkix/Alita_Robot/alita/db"
    "github.com/divkix/Alita_Robot/alita/i18n"
)
```

### Formatting
- **gofmt**: `gofmt -l -w` enforced via pre-commit hooks
- Line length: keep under 100 chars where reasonable
- Comments: full sentences with period, start with `// FunctionName`

### Naming Conventions
- **Exported**: PascalCase (`GetUser`, `LoadModules`)
- **Unexported**: camelCase (`getUser`, `moduleEnabled`)
- **Structs**: PascalCase, descriptive (`moduleStruct`, `ButtonArray`)
- **Constants**: PascalCase or UPPER_SNAKE for package-level (`DefaultWelcome`)
- **Interface suffixes**: `er` pattern (`Scanner`, `Valuer`)
- **Test files**: `_test.go` suffix, test functions: `TestXxx`, helpers: `helperName`

### Types & Structs
- Use surrogate keys: auto-increment `id` as PK, external IDs as unique constraints
- Custom GORM types implement `Scan(value any) error` and `Value() (driver.Value, error)`
- JSONB arrays: define as custom types with driver interface implementations

### Error Handling
- **Never ignore DB errors with `_`** — nil returns cause panics
- Wrap errors with context: `errors.Wrap(err, "context")` or `Wrapf` for formatting
- Four-layer recovery: dispatcher → worker pool → decorator → handler
- Expected Telegram API errors filtered via `helpers.IsExpectedTelegramError()`
- Panic recovery: `defer error_handling.RecoverFromPanic("context", "func")`

### Handler Patterns
- Value receiver on methods: `(moduleStruct)` or `(m moduleStruct)` when field access needed
- Return values: `ext.EndGroups` (stop), `ext.ContinueGroups` (continue), or `error`
- Handler groups: negative (-1) for early interception, positive (4-10) for watchers, 0 for standard
- Callback data: use `callbackcodec.Encode/Decode`, never raw `strings.Split`

### Database & Cache
- **Cache invalidation on writes**: every update must invalidate corresponding cache key
- Key format: `alita:{module}:{identifier}` (e.g., `alita:adminCache:123`)
- Use `singleflight` protection for cache stampede prevention
- Operations: Use `cache.Marshal.Get/Set/Delete` for direct cache access; use `getFromCacheOrLoad()` in `alita/db/cache_helpers.go` for DB-backed cached reads

### Module System
- Create `LoadXxx(dispatcher)` function per module
- Register in `alita/main.go:LoadModules()` — load order matters
- Help module loads last to collect all registered modules
- Add translation keys to ALL locale files in `locales/`

### i18n Patterns
- YAML: double quotes for escape sequences (`\n`, `\t`), single quotes preserve literally
- Printf safety: `%d` requires int, not `strconv.Itoa()` output
- Key verification: grep `locales/` to confirm keys exist in ALL files before using
- Parse mode: locale strings use Markdown, bot sends HTML — convert via `tgmd2html.MD2HTMLV2()`

## Testing Guidelines

**Framework:** Standard Go testing with `testing` package

**Coverage:** Run `make test` for full coverage report. Aim to maintain current coverage levels.

**Test Naming:**
- Test functions: `TestXxx` (e.g., `TestGetUser`)
- Helper functions: camelCase (e.g., `setupTestDB`)
- Files: `*_test.go` in same package as source code

**Patterns:**
- Use table-driven tests for multiple scenarios
- Database tests use test fixtures in `alita/db/testmain_test.go`
- Race detection enabled by default in test suite

## Architecture Overview

### Startup Flow (main.go)

1. Locale manager init (singleton, embedded YAML via `go:embed`)
2. OpenTelemetry tracing initialization (`tracing.InitTracing()`)
3. HTTP transport with connection pooling (optional API server rewriting)
4. Bot init + Telegram API connection pre-warming
5. Database (GORM/PostgreSQL) and cache (Redis) initialization
6. Async processing initialization (if `EnableAsyncProcessing` is configured)
7. Dispatcher creation (configurable max goroutines)
8. Monitoring systems: background stats, auto-remediation (GC triggers), activity monitor
9. Graceful shutdown manager (LIFO handler execution, 60s timeout)
10. Unified HTTP server (health + metrics + pprof + webhook on single port)
11. Mode selection: webhook or polling
12. Module loading via `alita.LoadModules(dispatcher)`

### Module System

Modules live in `alita/modules/`. Most modules expose a `LoadXxx(dispatcher)`
function called explicitly from `alita/main.go:LoadModules()`. Note:
`antiflood` and `antispam` modules also use Go `init()` functions for
background goroutine startup. Load order matters; help module loads last to
collect all registered modules.

**Non-module files in `alita/modules/`:** `helpers.go` (defines `moduleStruct`,
shared help utilities), `moderation_input.go` (text extraction for
filters/blacklists), `callback_codec.go` and `callback_parse_overwrite.go`
(callback data encoding), `chat_permissions.go` (permission helpers),
`connections_auth.go` (connection auth helper), `rules_format.go` (HTML
formatting for rules).

**Module structure pattern:**
- Value receiver on handler methods — typically unnamed `(moduleStruct)`,
  named `(m moduleStruct)` when method body needs struct field access
- `moduleStruct` fields: `moduleName`, `handlerGroup`, `permHandlerGroup`,
  `restrHandlerGroup`, `defaultRulesBtn`, `AbleMap`, `AltHelpOptions`,
  `helpableKb`
- Handlers return `ext.EndGroups` (stop propagation), `ext.ContinueGroups`
  (for monitoring/watcher handlers that should not block downstream), or `error`
- Commands registered via `dispatcher.AddHandler()` with handler groups
- Multiple command aliases registered via `helpers.MultiCommand(dispatcher, aliases, handler)`
- Disableable commands added via `helpers.AddCmdToDisableable()`
- Module enablement tracked in `HelpModule.AbleMap` (custom `moduleEnabled`
  struct wrapping `map[string]bool` with Store/Load methods — not sync.Map)
- Help keyboard buttons stored separately in `HelpModule.helpableKb`
  (`map[string][][]gotgbot.InlineKeyboardButton`)

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

**File organization:**
- `alita/db/db.go` — GORM models, connection setup, pool config
- `alita/db/*_db.go` — Domain-specific operations (`Get*`, `Add*`, `Update*`, `Delete*`)
- `alita/db/cache_helpers.go` — TTL management, cache invalidation
- `alita/db/optimized_queries.go` — Optimized SELECT queries with minimal column selection, singleflight-protected caching via `getFromCacheOrLoad`, thread-safe singleton query instances
- `alita/db/migrations.go` — Runtime migration engine
- `migrations/*.sql` — Source of truth for schema (timestamped filenames)

### Cache Layer

Redis-only via gocache library. Stampede protection via `singleflight` in the
DB caching layer (`alita/db/cache_helpers.go`).

- Key format: `alita:{module}:{identifier}` (e.g., `alita:adminCache:123`)
- Operations: `cache.Marshal.Get/Set/Delete` with context
- `ClearAllCaches()` — FLUSHDB on startup when `ClearCacheOnStartup` is configured
- Admin cache specialized in `alita/utils/cache/adminCache.go`
- **Cache must be invalidated on writes** — every DB update function that
  modifies cached data must call the corresponding invalidation

### Permission System (`alita/utils/chat_status/`)

- `RequireGroup()` / `RequirePrivate()` — chat type guards
- `RequireBotAdmin()` / `RequireUserAdmin()` / `RequireUserOwner()` — permission guards (send error messages on failure)
- `IsBotAdmin()` / `IsUserAdmin()` — bool checks (no error messages)
- `RequireUser()` / `GetEffectiveUser()` — safe user extraction (nil for channels)
- `IsValidUserId()` / `IsChannelId()` — ID validation
- `IsUserInChat()` / `IsUserBanProtected()` — membership/protection checks
- `CanUserChangeInfo()`, `CanUserRestrict()`, `CanBotRestrict()`, `CanUserPromote()`, `CanBotPromote()`, `CanUserPin()`, `CanBotPin()`, `CanUserDelete()`, `CanBotDelete()` — granular permission checks
- `CheckDisabledCmd()` — checks if command is disabled for chat
- Anonymous admin detection with keyboard fallback
- Results cached to reduce Telegram API calls

### Internationalization (`alita/i18n/`)

Singleton `LocaleManager` with per-language `Translator` instances. YAML locale
files embedded via `go:embed`. Supports named parameters in code
(`{"user": value}`) mapped to positional formatters (`%s`) in YAML.

### Error Handling

Four-layer recovery: dispatcher → worker pool → decorator → handler. The
`error_handling` package provides `RecoverFromPanic()` and `SetOnErrorCallback()`.
Expected Telegram API errors (bot not admin, chat closed) are filtered via
`helpers.IsExpectedTelegramError()`. Custom error wrapping with file/line/function
metadata via `alita/utils/errors/` (`Wrap()`/`Wrapf()` using `runtime.Caller`).

### Graceful Shutdown (`alita/utils/shutdown/`)

Central coordinator. Handlers registered during setup, executed in LIFO order
on shutdown. Each handler gets panic recovery. Total timeout: 60 seconds.

### Monitoring (`alita/utils/monitoring/`)

- **Activity monitor**: Tracks `last_activity` per chat AND per user (daily/weekly/monthly active users), marks inactive after threshold
- **Auto-remediation**: 4-tier system — warning logs at 80% threshold, GC trigger at 60% memory, aggressive memory cleanup (multiple GC cycles), restart recommendation at 150%+ threshold
- **Background stats**: System stats collected every 30s, DB stats every 1m, summary reported every 5 minutes

### Additional Utility Packages

- `alita/utils/extraction/` — extracts user IDs, chat IDs, time durations from Telegram messages
- `alita/utils/keyword_matcher/` — Aho-Corasick multi-pattern matching with per-chat caching (used by filters/blacklists)
- `alita/utils/media/` — unified media send interface for notes/filters/greetings
- `alita/utils/tracing/` — OpenTelemetry distributed tracing with OTLP/console exporters, includes cache key sanitization helpers
- `alita/utils/httpserver/` — unified HTTP server (health + metrics + pprof + webhook)
- `alita/utils/async/` — async processing with enable flag
- `alita/utils/constants/` — centralized time/duration constants (cache TTLs, timeouts, intervals)
- `alita/utils/callbackcodec/` — versioned callback data encoding/decoding
- `alita/utils/helpers/decorators.go` — command decorators: MultiCommand (aliases) and AddCmdToDisableable

## Commit & Pull Request Guidelines

**Commit Message Format:** Follow conventional commits
- `feat:` - New features
- `fix:` - Bug fixes
- `refactor:` - Code restructuring without behavior changes
- `perf:` - Performance improvements
- `test:` - Adding or updating tests
- `docs:` - Documentation changes
- `chore:` - Maintenance tasks
- `deps:` - Dependency updates

**Examples from project history:**
```
feat(i18n): add Russian, Portuguese, and Indonesian locales
fix: use process start time for /health uptime
refactor: Phase 1 code simplification - consolidate packages
perf: optimize critical loops for 60-95% performance gains
```

**Pre-commit Requirements:**
- Pre-commit hooks run automatically: `golangci-lint`, `gofmt`, `go mod tidy`
- Install: `pip install pre-commit && pre-commit install`
- Checks include: trailing whitespace, YAML validation, large file detection

**Pull Request Requirements:**
- Link related issues in PR description
- Ensure all tests pass (`make test`)
- Run linting (`make lint`) and fix issues
- Add translation keys to ALL locale files in `locales/` for any user-facing changes

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
- **Handler groups**: Negative numbers (e.g., -1) for early interception, positive numbers (4-10) for message watchers/monitors. Default group (0) for standard command handlers
- **Return values**: `ext.EndGroups` stops propagation, `ext.ContinueGroups` continues
- **Callback data**: Use versioned codec (`alita/utils/callbackcodec/`): `Encode(namespace, fields)` produces `<namespace>|v1|<url-encoded fields>`. Use `Decode(data)` to parse. Legacy dot-notation fallback exists for backward compatibility. Avoid raw `strings.Split(data, ".")` — use the codec
- **Double-answer bug**: `RequireUserAdmin` with `justCheck=false` already answers the callback — don't answer again
- **`IsUserConnected()`**: After calling, use the returned `connectedChat` value for the effective chat
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

## Pre-commit Hooks

Repository uses pre-commit with:
- `trailing-whitespace`, `end-of-file-fixer`, `check-yaml`
- `check-added-large-files` (max 1000KB)
- `check-merge-conflict`, `detect-private-key`
- `golangci-lint --timeout=5m`
- `gofmt -l -w`
- `go mod tidy`

Install: `pip install pre-commit && pre-commit install`

## Environment Configuration

See `sample.env` for all variables. Critical ones:

- `BOT_TOKEN`, `DATABASE_URL`, `REDIS_ADDRESS`, `MESSAGE_DUMP`, `OWNER_ID` (required)
- `HTTP_PORT` (default 8080) — unified for health, metrics, webhook
- `USE_WEBHOOKS`, `WEBHOOK_DOMAIN`, `WEBHOOK_SECRET` — webhook mode
- `AUTO_MIGRATE` — enable startup migrations
- `DEBUG` — verbose logging (performance monitoring auto-disabled when true)
- `ENABLE_PPROF` — enables `/debug/pprof/*` endpoints (dangerous in production)
- `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_SERVICE_NAME`, `OTEL_TRACES_SAMPLE_RATE` — OpenTelemetry tracing (set via environment, not in `sample.env`)

## Deployment

**Polling mode** (default): Simple, no external config. Higher latency.
**Webhook mode**: Production. Requires HTTPS (Cloudflare Tunnel supported).

Docker: Multi-stage build → distroless image. Health check via `--health` flag.
CI/CD: GitHub Actions with gosec, govulncheck, golangci-lint, multi-arch Docker.
Releases: GoReleaser to `ghcr.io/divkix/alita_robot`.

## Security Best Practices

- Never commit secrets (API keys, tokens, passwords)
- Pre-commit hooks detect private keys and large files
- Environment variables in `sample.env` only - never commit `.env`
- Use `AUTO_MIGRATE=true` for production database migrations
- Disable `ENABLE_PPROF` in production (exposes memory/profile endpoints)
- Webhook mode requires HTTPS with valid certificates for production
