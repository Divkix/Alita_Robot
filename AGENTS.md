# Repository Guidelines

Alita Robot is a Telegram group management bot built with Go 1.26+ and the
gotgbot library. Features include user management, filters, greetings,
anti-spam, captcha verification, and multi-language support (en, es, fr, hi, ru, pt, id).

## Project Structure & Module Organization

- **`alita/`** - Core application code
  - `config/` - Configuration management
  - `db/` - Database models, operations, and caching (PostgreSQL + GORM)
  - `i18n/` - Internationalization with embedded YAML locales
  - `modules/` - Bot functionality modules (admin, filters, greetings, etc.)
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
make test               # Run all tests with race detection, coverage (tags: testtools)
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
make check-docs         # Verify docs build integrity
make check-duplicates   # Detect duplicate translation keys

# Database migrations (requires PSQL_DB_* env vars)
make psql-prepare       # Auto-clean Supabase-specific SQL before migrations
make psql-migrate       # Apply pending migrations
make psql-status        # Check migration status
make psql-rollback      # Rollback last migration
make psql-verify        # Verify migration checksums
make psql-reset         # DROP ALL TABLES — destructive, requires confirmation

# Other
make inventory          # List all modules and their priorities
make validate-db        # Validate database schema integrity
make backup-db          # Export chat data per module
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
- **golangci-lint**: Configured in `.golangci.yml` (v2 format). Enabled linters: `godox`, `dupl`, `gocyclo` (min-complexity: 20)
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
- GORM uses `PrepareStmt: true` for prepared statement caching

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
- Key format: `alita:{module}:{identifier}` (e.g., `alita:adminCache:123`). `CacheKey` accepts variadic `...any` for multi-segment keys (`alita:lock:123:photos`)
- Use `singleflight` protection for cache stampede prevention (30-second timeout with `Forget(key)` on timeout)
- Operations: Use `cache.GetMarshal().Get/Set/Delete` for direct cache access; use `getFromCacheOrLoad()` in `alita/db/cache_helpers.go` for DB-backed cached reads
- `cache.GetMarshal()` nil checks: `if m := cache.GetMarshal(); m != nil { ... }` is standard

### Module System
- Modules self-register via `init()` using `RegisterLegacyModule()` or `RegisterModule()`
- `LoadModules()` in `alita/main.go` calls `LoadAllModules()` (priority-sorted) and defers `LoadHelp()`
- Help module loads last to collect all registered modules
- Add translation keys to ALL locale files in `locales/`

### i18n Patterns
- YAML: double quotes for escape sequences (`\n`, `\t`), single quotes preserve literally
- Printf safety: `%d` requires int, not `strconv.Itoa()` output
- Key verification: grep `locales/` to confirm keys exist in ALL files before using
- Parse mode: locale strings use Markdown, bot sends HTML — convert via `tgmd2html.MD2HTMLV2()`

## Testing Guidelines

**Framework:** Standard Go testing with `testing` package

**Coverage:** Run `make test` for full coverage report. CI enforces **78%** coverage threshold.

**Test Naming:**
- Test functions: `TestXxx` (e.g., `TestGetUser`)
- Helper functions: camelCase (e.g., `setupTestDB`)
- Files: `*_test.go` in same package as source code

**Patterns:**
- Use table-driven tests for multiple scenarios
- Database tests use test fixtures in `alita/db/testmain_test.go`
- Race detection enabled by default in test suite
- Build tag: `testtools` used in Makefile test target

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
12. Module loading via `alita.LoadModules(dispatcher)` — `LoadAllModules()` loads priority-sorted registered modules, `LoadHelp()` deferred last

### Module System

Modules live in `alita/modules/`. All modules self-register in `init()` via the
registry system (`alita/modules/registry.go`).

**Registration patterns:**
- **Legacy**: `RegisterLegacyModule(name, priority, loadFunc)` in `init()` — wraps existing `LoadXxx(dispatcher)` functions
- **New interface**: `RegisterModule(m Module)` where `Module` implements `Name()`, `Priority()`, `Load(dispatcher)`
- `bot_updates.go` is the only module using the new `Module` interface directly; all others use `RegisterLegacyModule`

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

**New command registration (preferred):**
- `helpers.WrapCommand(dispatcher, desc, handler)` — declarative pipeline with `CommandDescriptor`
- `helpers.WrapCommandRaw(dispatcher, desc, handler)` — raw handler variant
- `CommandDescriptor` fields: `Name`, `Aliases`, `Group`, `RequiredChecks`, `Disableable`
- Pre-built `CheckFunc` builders: `RequireGroup()`, `RequireBotAdmin()`, `RequireUserAdmin()`, `CanUserRestrict()`, `CanBotRestrict()`, etc.

**Adding a new module:**
1. Create DB operations in `alita/db/*_db.go`
2. Implement handlers in `alita/modules/your_module.go`
3. Create `LoadYourModule(dispatcher)` function
4. Register in `init()`: `RegisterLegacyModule("YourModule", priority, LoadYourModule)`
5. Add translation keys to ALL locale files in `locales/`

### Database Layer

**PostgreSQL + GORM** with connection pooling (configurable via env vars).

**Surrogate key pattern:** All tables use auto-increment `id` as PK. External
IDs (`user_id`, `chat_id`) have unique constraints but aren't primary keys.

**File organization:**
- `alita/db/db.go` — GORM models, connection setup, pool config, OTel-traced DB wrappers (`CreateRecordWithContext`, `UpdateRecordWithContext`, etc.)
- `alita/db/*_db.go` — Domain-specific operations (`Get*`, `Add*`, `Update*`, `Delete*`)
- `alita/db/cache_helpers.go` — TTL management, cache invalidation, `CacheKey()` helper, `getFromCacheOrLoad()`
- `alita/db/optimized_queries.go` — Optimized SELECT queries with minimal column selection, singleflight-protected caching via `getFromCacheOrLoad`, thread-safe singleton query instances (double-checked locking with `sync.RWMutex`)
- `alita/db/migrations.go` — Runtime migration engine (custom SQL runner, not GORM AutoMigrate)
- `alita/db/monitoring.go` — Database pool metrics collection (`StartMonitoring`, `DatabaseMetrics`)
- `alita/db/backup_db.go` — Export/import/clear chat data per module (13 modules supported)
- `alita/db/backup_types.go` — Backup format structs and validation (`BackupFormat`, `BackupFormatVersion = "1.0"`)
- `migrations/*.sql` — Source of truth for schema (timestamped filenames)

**Advanced patterns:**
- `clause.OnConflict` for atomic upserts (e.g., `locks_db.go`)
- `getFromCacheOrLoad` returns `loader()` directly if `cache.GetMarshal() == nil`
- `PrepareStmt: true` in GORM config for prepared statement caching

### Cache Layer

Redis-only via gocache library. Stampede protection via `singleflight` in the
DB caching layer (`alita/db/cache_helpers.go`).

- Key format: `alita:{module}:{identifier}` (e.g., `alita:adminCache:123`)
- `CacheKey(module, ids...)` generates multi-segment keys from variadic `any` args
- Operations: `cache.GetMarshal().Get/Set/Delete` with context (mutex-protected; do not bypass accessors)
- `ClearAllCaches()` — FLUSHDB on startup when `ClearCacheOnStartup` is configured
- Admin cache specialized in `alita/utils/cache/adminCache.go`
- Restricted chat cache: `alita/utils/cache/restrictedCache.go` — `MarkChatRestricted()`, `IsChatRestricted()`, `MarkChatNotRestricted()` (prevents spamming chats where bot lacks rights)
- **Cache must be invalidated on writes** — every DB update function that
  modifies cached data must call the corresponding invalidation

### Permission System (`alita/utils/chat_status/`)

- `RequireGroup()` / `RequirePrivate()` — chat type guards
- `RequireBotAdmin()` / `RequireUserAdmin()` / `RequireUserOwner()` — permission guards (send error messages on failure)
- `IsBotAdmin()` / `IsUserAdmin()` — bool checks (no error messages)
- `IsUserAdmin` returns false for channel IDs (negative numbers < -1000000000000)
- `RequireUser()` / `GetEffectiveUser()` — safe user extraction (nil for channels)
- `IsValidUserId()` / `IsChannelId()` — ID validation
- `IsUserInChat()` / `IsUserBanProtected()` — membership/protection checks
- `IsApproved()` — DB delegation for anti-spam whitelist
- `CanUserChangeInfo()`, `CanUserRestrict()`, `CanBotRestrict()`, `CanUserPromote()`, `CanBotPromote()`, `CanUserPin()`, `CanBotPin()`, `CanUserDelete()`, `CanBotDelete()`, `CanInvite()` — granular permission checks
- `CheckDisabledCmd()` — checks if command is disabled for chat
- Anonymous admin detection with keyboard fallback
- Results cached to reduce Telegram API calls

**PermissionResponder** (`alita/utils/chat_status/permission_responder.go`):
- Centralizes permission-failure messaging: `NewPermissionResponder()`, `Respond()`, `WithReply()`, `WithReplyFallback()`
- Handles callback-query answers vs. chat replies with fallback paths

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

**New error handling utilities:**
- `SendMessageWithErrorHandling()` / `DeleteMessageWithErrorHandling()` — wrappers that mark chats as restricted on permission failures
- `IsPermissionError()` — substring matcher for bot-send permission errors

### Graceful Shutdown (`alita/utils/shutdown/`)

Central coordinator. Handlers registered during setup, executed in LIFO order
on shutdown. Each handler gets panic recovery. Total timeout: 60 seconds.

### Monitoring (`alita/utils/monitoring/`)

- **Activity monitor**: Tracks `last_activity` per chat AND per user (daily/weekly/monthly active users), marks inactive after threshold
- **Auto-remediation**: 4-tier system — warning logs at 80% threshold, GC trigger at 60% memory, aggressive memory cleanup (multiple GC cycles), restart recommendation at 150%+ threshold
- **Background stats**: System stats collected every 30s, DB stats every 1m, summary reported every 5 minutes
- **Database metrics**: `alita/db/monitoring.go` collects pool metrics via `DatabaseMetrics` / `GetCurrentMetrics`

### Additional Utility Packages

- `alita/utils/extraction/` — extracts user IDs, chat IDs, time durations from Telegram messages
- `alita/utils/keyword_matcher/` — Aho-Corasick multi-pattern matching with per-chat caching (used by filters/blacklists)
- `alita/utils/media/` — unified media send interface for notes/filters/greetings
- `alita/utils/tracing/` — OpenTelemetry distributed tracing with OTLP/console exporters, includes cache key sanitization helpers
- `alita/utils/httpserver/` — unified HTTP server (health + metrics + pprof + webhook)
- `alita/utils/async/` — async processing with enable flag
- `alita/utils/constants/` — centralized time/duration constants (cache TTLs, timeouts, intervals)
- `alita/utils/callbackcodec/` — versioned callback data encoding/decoding (`Encode`, `Decode`, `EncodeOrFallback`)
- `alita/utils/helpers/decorators.go` — command decorators: `MultiCommand` (aliases) and `AddCmdToDisableable`
- `alita/utils/helpers/command_pipeline.go` — `WrapCommand` / `WrapCommandRaw` declarative command registration with `CommandDescriptor` and `CheckFunc`
- `alita/utils/formatting/` — `Shtml()`, `Smarkdown()`, `SplitMessage()`, `MentionHtml()`, `ReverseHTML2MD()`, `FormattingReplacer()`
- `alita/utils/keyboard/` — `BuildKeyboard()`, `ChunkKeyboardSlices()`, `MakeLanguageKeyboard()`
- `alita/utils/ratelimit/` — `BackupRateLimiter` with `CanExport/CanImport/CanReset` and cooldown tracking

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
- `.golangci.yml` (v2 format) configures: `godox`, `dupl`, `gocyclo` (min-complexity: 20)

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
- **Callback data**: Use versioned codec (`alita/utils/callbackcodec/`): `Encode(namespace, fields)` produces `<namespace>|v1|<url-encoded fields>`. Use `Decode(data)` to parse. Legacy dot-notation fallback exists for backward compatibility. Avoid raw `strings.Split(data, ".")` — use the codec. Use `EncodeOrFallback` for graceful fallback encoding.
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
- **Upserts**: Use `clause.OnConflict` for atomic upserts where appropriate
- **Nil cache safety**: `getFromCacheOrLoad` returns `loader()` directly when `cache.GetMarshal() == nil`

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

**Additional configuration sections:**
- Connection pool tuning: `DB_MAX_IDLE_CONNS`, `DB_MAX_OPEN_CONNS`, `DB_CONN_MAX_LIFETIME_MIN`, `DB_CONN_MAX_IDLE_TIME_MIN`
- Redis alternatives: `REDIS_PASSWORD`, `REDIS_DB`, `REDIS_URL`
- Worker pools: `CHAT_VALIDATION_WORKERS`, `DATABASE_WORKERS`, `MESSAGE_PIPELINE_WORKERS`, `BULK_OPERATION_WORKERS`, `CACHE_WORKERS`, `STATS_COLLECTION_WORKERS`, `MAX_CONCURRENT_OPERATIONS`, `OPERATION_TIMEOUT_SECONDS`, `DISPATCHER_MAX_ROUTINES`
- Monitoring toggles: `ENABLE_PERFORMANCE_MONITORING`, `ENABLE_BACKGROUND_STATS`, `ENABLE_DB_MONITORING`
- Activity monitoring: `INACTIVITY_THRESHOLD_DAYS`, `ACTIVITY_CHECK_INTERVAL`, `ENABLE_AUTO_CLEANUP`
- Performance optimizations: `ENABLE_QUERY_PREFETCHING`, `ENABLE_CACHE_PREWARMING`, `ENABLE_ASYNC_PROCESSING`, `ENABLE_RESPONSE_CACHING`, `RESPONSE_CACHE_TTL`, `ENABLE_BATCH_REQUESTS`, `BATCH_REQUEST_TIMEOUT_MS`, `ENABLE_HTTP_CONNECTION_POOLING`, `HTTP_MAX_IDLE_CONNS`, `HTTP_MAX_IDLE_CONNS_PER_HOST`
- Resource limits: `RESOURCE_MAX_GOROUTINES`, `RESOURCE_MAX_MEMORY_MB`, `RESOURCE_GC_THRESHOLD_MB`
- Migration settings: `AUTO_MIGRATE_SILENT_FAIL`, `MIGRATIONS_PATH`
- Other: `CLOUDFLARE_TUNNEL_TOKEN`, `ENABLED_LOCALES`, `DROP_PENDING_UPDATES`, `API_SERVER`

## Deployment

**Polling mode** (default): Simple, no external config. Higher latency.
**Webhook mode**: Production. Requires HTTPS (Cloudflare Tunnel supported).

Docker: Multi-stage build → distroless image. Health check via `--health` flag.
CI/CD: GitHub Actions with gosec, govulncheck, golangci-lint, multi-arch Docker.
Releases: GoReleaser to `ghcr.io/divkix/alita_robot`.

**CI/CD workflows:**
- `ci.yml` — lint, test (coverage threshold 78%), build, Docker verify/publish
- `release.yml` — GoReleaser artifact attestation, post-release Trivy scan
- `docs.yml` — Build Astro docs and deploy to Cloudflare Workers
- `dependabot-native-merge.yml` — Auto-approve/merge Dependabot patch/minor updates

## Security Best Practices

- Never commit secrets (API keys, tokens, passwords)
- Pre-commit hooks detect private keys and large files
- Environment variables in `sample.env` only - never commit `.env`
- Use `AUTO_MIGRATE=true` for production database migrations
- Disable `ENABLE_PPROF` in production (exposes memory/profile endpoints)
- Webhook mode requires HTTPS with valid certificates for production
