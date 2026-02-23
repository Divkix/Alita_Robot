# Codebase Structure

**Analysis Date:** 2026-02-23

## Directory Layout

```
Alita_Robot/
├── main.go                     # Root entry point — bootstrap, mode selection, shutdown
├── go.mod / go.sum             # Go module definition
├── Makefile                    # Dev commands: run, build, lint, test, migrate
├── sample.env                  # All supported environment variables documented
├── alita/                      # Core application code
│   ├── main.go                 # Module loader (LoadModules, InitialChecks)
│   ├── config/                 # Typed config struct from env vars
│   │   ├── config.go           # AppConfig global, all env var parsing
│   │   └── types.go            # Supporting types for config
│   ├── db/                     # PostgreSQL/GORM layer
│   │   ├── db.go               # All GORM models, DB init, generic CRUD helpers
│   │   ├── *_db.go             # Domain-specific DB operations (one file per domain)
│   │   ├── cache_helpers.go    # Cache key generators, TTL constants, singleflight wrapper
│   │   ├── optimized_queries.go # Minimal-column SELECT queries, singleton instances
│   │   └── migrations.go       # Runtime migration engine (reads migrations/*.sql)
│   ├── i18n/                   # Internationalization
│   │   ├── manager.go          # Singleton LocaleManager
│   │   ├── translator.go       # Per-language Translator with named param substitution
│   │   ├── loader.go           # YAML locale file loading from embed.FS
│   │   └── types.go            # TranslationParams, ManagerConfig types
│   ├── health/                 # Health check logic
│   │   └── health.go           # DB + cache connectivity checks
│   ├── metrics/                # Prometheus metrics
│   │   └── prometheus.go       # Metric registration and collection
│   ├── modules/                # Feature modules (one file per domain)
│   │   ├── helpers.go          # moduleStruct, moduleEnabled, shared in-process state
│   │   ├── help.go             # HelpModule, help menu rendering (loaded last)
│   │   ├── admin.go            # Admin management
│   │   ├── antiflood.go        # Message flood detection and action
│   │   ├── antispam.go         # Spam detection (uses init() for background goroutine)
│   │   ├── bans.go             # Ban/kick/unban commands
│   │   ├── blacklists.go       # Blacklisted word detection and action
│   │   ├── bot_updates.go      # Bot status update handlers (added to group, etc.)
│   │   ├── captcha.go          # Captcha verification for new members
│   │   ├── connections.go      # PM-to-group connection management
│   │   ├── devs.go             # Developer/sudo commands
│   │   ├── disabling.go        # Per-chat command disabling
│   │   ├── filters.go          # Keyword-triggered auto-reply filters
│   │   ├── formatting.go       # Text formatting commands
│   │   ├── greetings.go        # Welcome/goodbye messages
│   │   ├── language.go         # Bot language selection per chat/user
│   │   ├── locks.go            # Message type locking (stickers, gifs, etc.)
│   │   ├── misc.go             # Miscellaneous commands
│   │   ├── mute.go             # Mute/unmute commands
│   │   ├── notes.go            # Saved note management
│   │   ├── pins.go             # Message pinning management
│   │   ├── purges.go           # Message bulk deletion
│   │   ├── reports.go          # User reporting system
│   │   ├── rules.go            # Chat rules management
│   │   ├── users.go            # User tracking handlers
│   │   ├── warns.go            # Warning system
│   │   ├── callback_codec.go   # Callback data encode/decode wrapper for modules
│   │   ├── callback_parse_overwrite.go # Overwrite confirmation callback handling
│   │   ├── chat_permissions.go # Permission bit helpers for modules
│   │   ├── connections_auth.go # Connection authentication helper
│   │   ├── moderation_input.go # Text/target extraction for bans/filters/blacklists
│   │   └── rules_format.go     # HTML formatting for rules display
│   └── utils/                  # Cross-cutting utility packages
│       ├── async/              # Async processing wrapper
│       │   └── async_processor.go
│       ├── cache/              # Redis cache client
│       │   ├── adminCache.go   # Admin list cache management
│       │   └── sanitize.go     # Cache key sanitization for OTel spans
│       ├── callbackcodec/      # Versioned callback data encoding
│       │   └── callbackcodec.go # Encode/Decode functions, ErrInvalidFormat, etc.
│       ├── chat_status/        # Permission guards and user extraction
│       │   └── chat_status.go  # Require*, Is*, Can* functions
│       ├── constants/          # Centralized time/duration constants
│       ├── debug_bot/          # Debug utilities
│       ├── decorators/
│       │   ├── cmdDecorator/   # MultiCommand alias registration
│       │   │   └── cmdDecorator.go
│       │   └── misc/           # AddCmdToDisableable and handler vars
│       │       └── handler_vars.go
│       ├── error_handling/     # Panic recovery and error capture
│       │   └── error_handling.go
│       ├── errors/             # Wrapped errors with file/line/function metadata
│       ├── extraction/         # User ID, chat ID, duration extraction from messages
│       │   └── extraction.go
│       ├── helpers/            # Telegram message helpers
│       │   ├── helpers.go      # Shtml(), HTML constant, chunked keyboard, etc.
│       │   ├── telegram_helpers.go # IsExpectedTelegramError, bot API wrappers
│       │   └── channel_helpers.go  # Channel/linked chat detection helpers
│       ├── httpserver/         # Unified HTTP server
│       │   └── server.go       # Health, metrics, pprof, webhook on one port
│       ├── keyword_matcher/    # Aho-Corasick pattern matcher
│       │   ├── matcher.go      # Per-chat matcher construction and matching
│       │   └── cache.go        # Matcher result caching
│       ├── media/              # Unified media sending
│       │   └── sender.go       # SendMedia for notes/filters/greetings
│       ├── monitoring/         # Background system monitoring
│       │   ├── activity_monitor.go  # Chat/user activity tracking and cleanup
│       │   ├── background_stats.go  # Periodic system stats collection
│       │   └── auto_remediation.go  # GC trigger and memory pressure response
│       ├── shutdown/           # Graceful shutdown coordination
│       │   └── graceful.go     # LIFO handler registry, signal handler
│       ├── string_handling/    # Slice search utilities
│       ├── tracing/            # OpenTelemetry setup
│       │   └── tracing.go      # InitTracing, StartSpan, TracingProcessor
│       └── webhook/            # Webhook validation utilities
├── locales/                    # Embedded YAML translation files
│   ├── en.yml                  # English (source of truth for keys)
│   ├── es.yml                  # Spanish
│   ├── fr.yml                  # French
│   ├── hi.yml                  # Hindi
│   └── config.yml              # Locale loader configuration
├── migrations/                 # SQL migration files
│   └── YYYYMMDDHHMMSS_*.sql    # Timestamped SQL migrations (source of truth for schema)
├── docker/                     # Docker build files
├── docs/                       # Astro documentation site
├── scripts/                    # Build/tooling scripts
├── specs/                      # Git worktree specs for parallel work sessions
├── supabase/                   # Supabase CLI project files
└── .github/workflows/          # CI/CD — lint, test, security scan, Docker publish
```

## Directory Purposes

**`alita/modules/`:**
- Purpose: One Go file per bot feature domain; each file defines a `moduleStruct` var and a `Load*()` function
- Contains: Handler methods on `moduleStruct`, module-level state (e.g., in-process sync.Map for antiflood counters)
- Key files: `helpers.go` (shared state, `moduleStruct` definition), `help.go` (last loaded, aggregates all modules)

**`alita/db/`:**
- Purpose: All database interaction — models, domain CRUD, caching, migrations
- Contains: 50 files — `db.go` with all GORM struct definitions, paired `*_db.go` + `*_db_test.go` for each domain
- Key files: `db.go`, `cache_helpers.go`, `optimized_queries.go`, `migrations.go`

**`alita/i18n/`:**
- Purpose: Multi-language string resolution from embedded YAML files
- Contains: Singleton manager, per-language translators, YAML loader
- Key files: `manager.go` (GetManager singleton), `translator.go` (GetString with named params)

**`alita/utils/chat_status/`:**
- Purpose: All permission checking logic in one package — prevents permission check duplication across modules
- Contains: `Require*` (with error messages), `Is*` (bool checks), `Can*` (granular permission checks)

**`alita/config/`:**
- Purpose: Centralized env var parsing — `AppConfig` global used everywhere
- Contains: All env var names, defaults, and validation

**`migrations/`:**
- Purpose: Source of truth for database schema; executed in timestamp order
- Contains: Timestamped `.sql` files; tracked in `schema_migrations` table
- Generated: No — hand-authored SQL
- Committed: Yes

**`locales/`:**
- Purpose: Translation strings embedded into binary at compile time via `go:embed`
- Contains: One YAML per language; keys must be present in ALL language files
- Committed: Yes — changes require updating all locale files

**`specs/`:**
- Purpose: Git worktree directories for parallel feature development sessions
- Contains: Subdirectories matching branch names for `.claude worktrees` workflow

## Key File Locations

**Entry Points:**
- `main.go`: Process entry point — bootstrap, mode selection, signal handling
- `alita/main.go`: `LoadModules()` — explicit ordered module registration

**Configuration:**
- `alita/config/config.go`: `AppConfig` global `Config` struct with all env vars
- `sample.env`: Documentation of all supported environment variables with defaults

**Core Logic:**
- `alita/modules/helpers.go`: `moduleStruct` and `HelpModule` definitions
- `alita/db/db.go`: All GORM models, DB init with retry, generic CRUD functions
- `alita/db/cache_helpers.go`: Cache key functions and TTL constants
- `alita/utils/chat_status/chat_status.go`: All permission guard functions

**Database Migrations:**
- `migrations/*.sql`: SQL migration files (timestamped `YYYYMMDDHHMMSS_description.sql`)
- `alita/db/migrations.go`: Runtime migration runner

**Testing:**
- `alita/db/*_db_test.go`: DB-level integration tests (require live PostgreSQL)
- `alita/db/testmain_test.go`: Test suite setup
- `alita/modules/*_test.go`: Module-level unit tests (callback codec, helpers, etc.)
- `alita/i18n/i18n_test.go`: Translator unit tests
- `alita/config/config_test.go`: Config parsing tests

## Naming Conventions

**Files:**
- Module files: `<feature>.go` (e.g., `bans.go`, `captcha.go`, `greetings.go`)
- DB files: `<domain>_db.go` and paired `<domain>_db_test.go` (e.g., `warns_db.go`, `warns_db_test.go`)
- Utility files: descriptive `snake_case.go` (e.g., `chat_status.go`, `keyword_matcher.go`)
- Test files: `<name>_test.go` co-located with the file under test

**Directories:**
- Packages: `snake_case` (e.g., `chat_status`, `keyword_matcher`, `error_handling`, `callbackcodec`)

**Functions:**
- DB operations: `Get*`, `Add*`, `Update*`, `Delete*` verbs (e.g., `GetWarnSettings`, `AddWarnUser`, `UpdateWarnSettings`)
- Permission guards: `Require*` (sends error message on fail), `Is*` (bool check), `Can*` (capability check)
- Module loaders: `Load<ModuleName>(dispatcher *ext.Dispatcher)` (e.g., `LoadBans`, `LoadGreetings`)
- Handler methods: value receivers on `moduleStruct` — unnamed `(moduleStruct)` or named `(m moduleStruct)` if body needs struct fields

**Variables:**
- Module singleton: `var <feature>Module = moduleStruct{moduleName: "<Name>"}` at package level
- Global DB: `var DB *gorm.DB` in `alita/db/db.go`
- Global config: `var AppConfig Config` in `alita/config/config.go`

## Where to Add New Code

**New Feature Module:**
1. DB operations: Create `alita/db/<feature>_db.go` and `alita/db/<feature>_db_test.go`
2. DB model: Add GORM struct to `alita/db/db.go` with `TableName()` method
3. SQL migration: Add `migrations/YYYYMMDDHHMMSS_add_<feature>.sql` — include CREATE TABLE and indexes
4. Module handlers: Create `alita/modules/<feature>.go` with `moduleStruct` var and `Load<Feature>(dispatcher)` function
5. Module registration: Add `modules.Load<Feature>(dispatcher)` call in `alita/main.go` `LoadModules()`
6. Translation keys: Add all keys to ALL locale files in `locales/` (en, es, fr, hi)
7. Tests: Add `alita/db/<feature>_db_test.go` and `alita/modules/<feature>_test.go` as needed

**New Command in Existing Module:**
- Add handler method on `moduleStruct`
- Register via `dispatcher.AddHandler()` or `cmdDecorator.MultiCommand()` inside the module's `Load*()` function
- Add to disableable list via `misc.AddCmdToDisableable()` if applicable

**New DB Operation:**
- Add function to `alita/db/<domain>_db.go`
- Cache reads: use `getFromCacheOrLoad(key, ttl, loader)` pattern in `cache_helpers.go`
- Cache writes: add corresponding `invalidate<Domain>Cache(chatID)` call after every mutation

**New Utility Package:**
- Create `alita/utils/<package-name>/` directory
- Use `snake_case` for package name

**New Translation String:**
- Add to ALL four locale files: `locales/en.yml`, `locales/es.yml`, `locales/fr.yml`, `locales/hi.yml`
- Run `make check-translations` to verify completeness
- Use double-quoted YAML strings when the value contains `\n` or `\t`

**New DB Migration:**
- Create `migrations/<timestamp>_<description>.sql` (use `date +%Y%m%d%H%M%S`)
- Update corresponding GORM struct in `alita/db/db.go`
- Update `optimized_queries.go` if the struct is used there
- Update domain `*_db.go` functions for new columns

## Special Directories

**`specs/`:**
- Purpose: Git worktree working directories for parallel branch sessions
- Generated: Yes (by `.claude` worktree commands)
- Committed: No (contents are separate git worktrees, not committed to main)

**`.planning/codebase/`:**
- Purpose: Codebase analysis documents for GSD planning workflow
- Generated: Yes (by `/gsd:map-codebase` agent)
- Committed: Yes

**`docs/`:**
- Purpose: Astro-based documentation website
- Generated: Partially (`make generate-docs` generates content from source)
- Committed: Yes

**`supabase/`:**
- Purpose: Supabase CLI project configuration for managed PostgreSQL deployment
- Generated: No
- Committed: Yes

---

*Structure analysis: 2026-02-23*
