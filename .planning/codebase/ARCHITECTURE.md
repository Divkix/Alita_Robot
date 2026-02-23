# Architecture

**Analysis Date:** 2026-02-23

## Pattern Overview

**Overall:** Event-driven modular monolith with layered separation

**Key Characteristics:**
- Telegram update events are dispatched by gotgbot's `ext.Dispatcher` with configurable max goroutines
- All feature domains are isolated as modules under `alita/modules/`, each registering their own handlers into the dispatcher
- Database access is mediated through a domain-specific function layer (`alita/db/*_db.go`) — modules never call GORM directly
- Redis cache sits in front of every hot DB read; every write function is responsible for invalidating its own cache key
- OpenTelemetry tracing is injected at the dispatcher level via `tracing.TracingProcessor`, meaning every update carries a trace span automatically

## Layers

**Entry Point (main.go):**
- Purpose: Bootstrap ordering — locale → tracing → HTTP → bot → DB → cache → monitoring → dispatcher → modules
- Location: `main.go` (root package)
- Contains: Startup sequence, signal handling delegation, shutdown registration, webhook/polling mode selection
- Depends on: All packages below
- Used by: OS process start

**Application Orchestration:**
- Purpose: Module loading coordination and resource monitoring
- Location: `alita/main.go` (`package alita`)
- Contains: `LoadModules()`, `InitialChecks()`, `ResourceMonitor()`, `ListModules()`
- Depends on: `alita/modules`, `alita/db`, `alita/utils/cache`
- Used by: `main.go`

**Module Layer:**
- Purpose: Feature implementation — command handlers, callback handlers, message watchers
- Location: `alita/modules/`
- Contains: One file per feature domain (e.g., `bans.go`, `filters.go`, `captcha.go`); shared helpers in `helpers.go`
- Depends on: `alita/db`, `alita/i18n`, `alita/utils/chat_status`, `alita/utils/extraction`, `alita/utils/helpers`, `alita/utils/media`, `alita/utils/keyword_matcher`
- Used by: `alita/main.go` via explicit `Load*()` calls

**Permission/Chat Status Layer:**
- Purpose: Centralized permission enforcement and anonymous admin handling
- Location: `alita/utils/chat_status/`
- Contains: `RequireGroup`, `RequireUserAdmin`, `RequireBotAdmin`, `IsBotAdmin`, `IsUserAdmin`, `CanBotRestrict`, etc.
- Depends on: `alita/db`, `alita/utils/cache`
- Used by: Every module handler that enforces access control

**Database Access Layer:**
- Purpose: Domain-specific CRUD operations wrapping GORM
- Location: `alita/db/` — `db.go` (models + connection), `*_db.go` (domain operations), `optimized_queries.go`, `cache_helpers.go`, `migrations.go`
- Contains: All GORM models, `CreateRecord`, `UpdateRecord`, `DeleteRecord`, `GetRecord` generic helpers plus domain-specific `Get*`, `Add*`, `Update*`, `Delete*` functions
- Depends on: `alita/config`, `alita/utils/cache`, `alita/utils/tracing`
- Used by: Module layer, `alita/utils/chat_status`, `alita/utils/httpserver`

**Cache Layer:**
- Purpose: Redis-backed TTL cache with stampede protection
- Location: `alita/utils/cache/` — `adminCache.go`, `sanitize.go`, and core cache init
- Contains: `InitCache()`, `LoadAdminCache()`, `TracedGet()`, `TracedSet()`, `TracedDelete()`, `ClearAllCaches()`
- Depends on: `alita/utils/constants` (TTL values), `alita/utils/tracing`
- Used by: `alita/db/cache_helpers.go`, `alita/utils/chat_status`, modules directly for hot paths

**Internationalization Layer:**
- Purpose: Multi-language string resolution with named parameter substitution
- Location: `alita/i18n/` — `manager.go` (singleton), `translator.go`, `loader.go`, `types.go`
- Contains: `LocaleManager` singleton, `Translator` per-language instances, YAML locale file loading via `go:embed`
- Depends on: `alita/utils/cache` (optional translation caching)
- Used by: All module handlers, `main.go` for bot command descriptions

**Infrastructure Utilities:**
- Purpose: Cross-cutting concerns — HTTP server, tracing, shutdown, monitoring, error handling
- Location: `alita/utils/` subdirectories
- Contains:
  - `httpserver/server.go` — unified HTTP mux (health, metrics, pprof, webhook on single port)
  - `tracing/tracing.go` — OpenTelemetry OTLP/console exporter init
  - `shutdown/graceful.go` — LIFO shutdown handler registry with 60s timeout
  - `monitoring/` — `background_stats.go`, `auto_remediation.go`, `activity_monitor.go`
  - `error_handling/error_handling.go` — `RecoverFromPanic`, `HandleErr`, `CaptureError`
  - `errors/` — `Wrap`/`Wrapf` with `runtime.Caller` metadata
- Depends on: `alita/config`, `alita/db`, `alita/utils/cache`, `alita/utils/tracing`
- Used by: `main.go`, module handlers, DB layer

**Configuration Layer:**
- Purpose: Environment variable parsing and validation into a typed `Config` struct
- Location: `alita/config/config.go`, `alita/config/types.go`
- Contains: `AppConfig` global, all env var parsing with defaults, Redis URL parsing helpers
- Depends on: `godotenv`, `logrus`
- Used by: Every layer that needs runtime configuration

## Data Flow

**Command Handler Flow:**

1. Telegram sends update → bot receives via webhook POST or long-polling GET
2. `ext.Dispatcher` receives the update, injects trace context via `tracing.TracingProcessor`
3. Dispatcher routes to matching handler group(s) based on filters (command text, message type, etc.)
4. Handler function runs permission checks via `alita/utils/chat_status/` — returns `ext.EndGroups` on failure
5. Handler calls domain-specific DB functions in `alita/db/*_db.go` with cache-first reads
6. DB function checks Redis cache via `getFromCacheOrLoad` (singleflight-protected); on cache miss, queries PostgreSQL via GORM
7. Handler sends Telegram API response (message, keyboard, edit) using helpers from `alita/utils/helpers/`
8. Write operations (DB mutations) invalidate corresponding cache keys

**Callback Query Flow:**

1. User clicks inline keyboard → Telegram sends `callback_query` update
2. Handler decodes payload via `alita/utils/callbackcodec/` (`Decode(data)` → `Decoded{Namespace, Fields}`)
3. Handler dispatches based on namespace/fields, performs action, edits or replies to the callback
4. NOTE: `RequireUserAdmin` with `justCheck=false` already answers the callback — do not answer again

**Message Watcher Flow (filters/blacklists/antiflood):**

1. Every message triggers watcher handlers in high-numbered groups (handlerGroup 9+)
2. `keyword_matcher` (Aho-Corasick) matches message text against per-chat filter/blacklist trigger lists
3. On match, handler executes configured action (reply, restrict, warn, delete)

**State Management:**
- Bot settings per chat: PostgreSQL via GORM, cached in Redis for 30 minutes
- Admin lists: Redis-only via `LoadAdminCache`, refreshed on demand
- Antiflood counters: In-process `sync.Map` keyed by `{chatId, userId}` struct
- Language preferences: PostgreSQL, cached in Redis for 1 hour
- Captcha state: PostgreSQL `captcha_attempts` + `stored_messages` tables

## Key Abstractions

**`moduleStruct`:**
- Purpose: Shared state container for every module — name, handler groups, module registry
- Examples: `var bansModule = moduleStruct{moduleName: "Bans"}` in `alita/modules/bans.go`
- Pattern: Value receiver on all handler methods; singleton var per module file

**`moduleEnabled` (AbleMap):**
- Purpose: Thread-safe module enable/disable registry used by help system
- Examples: `modules.HelpModule.AbleMap.Init()` in `alita/main.go`
- Pattern: Custom struct wrapping `map[string]bool` with `Store`/`Load` methods (not `sync.Map`)

**`moduleStruct.helpableKb`:**
- Purpose: Per-module inline keyboard buttons appended to help navigation
- Examples: Set during `Load*()` functions, consumed by `help.go`
- Pattern: `map[string][][]gotgbot.InlineKeyboardButton`

**GORM Models:**
- Purpose: Database schema representation with surrogate auto-increment `id` PK
- Examples: `alita/db/db.go` — `User`, `Chat`, `WarnSettings`, `ChatFilters`, `Notes`, `CaptchaSettings`, etc.
- Pattern: Every struct implements `TableName() string`, external IDs (`user_id`, `chat_id`) are `uniqueIndex` not PK

**Cache Keys:**
- Purpose: Namespaced Redis keys preventing key collisions
- Examples: `alita:filter_list:{chatID}`, `alita:adminCache:{chatID}`, `alita:user_lang:{userID}`
- Pattern: `alita:{module}:{identifier}` — generators are private functions in `alita/db/cache_helpers.go`

**Callback Codec:**
- Purpose: Versioned Telegram callback data encoding within the 64-byte limit
- Examples: `alita/utils/callbackcodec/callbackcodec.go` — `Encode(namespace, fields)` → `<ns>|v1|<url-encoded>`
- Pattern: `Decode(data)` returns `Decoded{Namespace, Fields}`; legacy dot-notation fallback exists

## Entry Points

**HTTP Server (`/health`, `/metrics`, `/debug/pprof/*`, webhook):**
- Location: `alita/utils/httpserver/server.go`
- Triggers: HTTP requests on `HTTP_PORT` (default 8080)
- Responsibilities: Health checks (DB + cache connectivity), Prometheus metrics exposure, optional webhook update ingestion, optional pprof

**Telegram Polling Entry:**
- Location: `main.go` — `updater.StartPolling()`
- Triggers: Long-poll to Telegram GetUpdates API
- Responsibilities: Fetches updates and feeds them to `ext.Dispatcher`

**Telegram Webhook Entry:**
- Location: `alita/utils/httpserver/server.go` via `RegisterWebhook()`
- Triggers: POST to `/{WEBHOOK_SECRET}` from Telegram servers
- Responsibilities: Validates secret, parses update JSON, feeds to `ext.Dispatcher`

**Module Loader:**
- Location: `alita/main.go` — `LoadModules(dispatcher)`
- Triggers: Called once during startup after dispatcher creation
- Responsibilities: Registers all handler functions with the dispatcher in defined order; `LoadHelp` always runs last via `defer`

## Error Handling

**Strategy:** Four-layer recovery preventing cascading failures

**Patterns:**
- Dispatcher level: `Error` callback in `ext.DispatcherOpts` — logs and returns `DispatcherActionNoop`; expected Telegram errors filtered via `helpers.IsExpectedTelegramError()`
- Handler level: `defer error_handling.RecoverFromPanic(name, pkg)` in critical paths
- Error wrapping: `errors.Wrap(err, "context")` via `alita/utils/errors/` attaches file/line/function via `runtime.Caller`
- DB level: Never ignore errors with `_`; nil DB returns cause panics — always check `err`

## Cross-Cutting Concerns

**Logging:** `github.com/sirupsen/logrus` — structured `log.WithFields()` for DB and monitoring, plain `log.Info/Error/Fatal` elsewhere

**Validation:** Permission checks centralized in `alita/utils/chat_status/` — handlers call `Require*` functions at the top of each handler and return `ext.EndGroups` on failure

**Authentication:** Anonymous admin detection in `chat_status.go` with keyboard fallback; admin lists cached from Telegram API via `LoadAdminCache`

**Tracing:** `tracing.StartSpan(ctx, "operation")` wraps DB operations; `TracingProcessor` injects trace context into every dispatched update; OTLP gRPC export or console fallback

---

*Architecture analysis: 2026-02-23*
