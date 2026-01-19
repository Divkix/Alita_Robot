---
title: Project Structure
description: Directory layout and key files in the Alita Robot codebase.
---

This document explains the directory structure of Alita Robot, helping developers navigate the codebase efficiently.

## Directory Tree

```
Alita_Robot/
├── main.go                     # Application entry point
├── CLAUDE.md                   # AI assistant guidance document
├── sample.env                  # Environment variable template
├── Makefile                    # Build and development commands
├── go.mod                      # Go module definition
├── go.sum                      # Go dependency checksums
├── .goreleaser.yaml            # Release configuration
│
├── alita/                      # Core application code
│   ├── main.go                 # Module loader and initialization
│   │
│   ├── config/                 # Configuration management
│   │   └── config.go           # Environment variable parsing
│   │
│   ├── db/                     # Database layer
│   │   ├── db.go               # GORM connection and setup
│   │   ├── cache_helpers.go    # Cache key patterns and TTLs
│   │   ├── *_db.go             # Per-feature database operations
│   │   └── shared_helpers.go   # Transaction support utilities
│   │
│   ├── health/                 # Health check endpoints
│   │   └── health.go           # /health handler implementation
│   │
│   ├── i18n/                   # Internationalization
│   │   ├── i18n.go             # Translation manager
│   │   └── translator.go       # Per-request translator
│   │
│   ├── metrics/                # Prometheus metrics
│   │   └── metrics.go          # Metric definitions and /metrics handler
│   │
│   ├── modules/                # Command handlers (features)
│   │   ├── help.go             # Help system and module registry
│   │   ├── bans.go             # Ban/kick/restrict commands
│   │   ├── mutes.go            # Mute/unmute commands
│   │   ├── filters.go          # Message filters
│   │   ├── notes.go            # Saved notes/messages
│   │   ├── greetings.go        # Welcome/goodbye messages
│   │   ├── warns.go            # Warning system
│   │   ├── captcha.go          # CAPTCHA verification
│   │   └── ...                 # Other feature modules
│   │
│   └── utils/                  # Utility packages
│       ├── async/              # Async processing utilities
│       ├── cache/              # Redis cache initialization
│       │   └── cache.go        # Cache client setup
│       ├── chat_status/        # Permission checking
│       │   ├── chat_status.go  # Admin/permission checks
│       │   └── helpers.go      # Helper functions
│       ├── cleanup/            # Resource cleanup utilities
│       ├── constants/          # Application constants
│       ├── decorators/         # Handler decorators
│       ├── error_handling/     # Panic recovery and logging
│       ├── errors/             # Custom error types
│       ├── extraction/         # User/text extraction
│       ├── helpers/            # General helper functions
│       ├── httpserver/         # Unified HTTP server
│       ├── keyword_matcher/    # Keyword matching utilities
│       ├── media/              # Media handling utilities
│       ├── monitoring/         # Activity and resource monitoring
│       ├── shutdown/           # Graceful shutdown manager
│       ├── string_handling/    # String manipulation
│       └── webhook/            # Webhook configuration
│
├── migrations/                 # SQL migration files
│   ├── 001_initial.sql         # Initial schema
│   ├── 002_*.sql               # Feature migrations
│   └── ...                     # Numbered migrations
│
├── locales/                    # Translation files
│   ├── en.yml                  # English translations
│   ├── de.yml                  # German translations
│   └── ...                     # Other languages
│
├── cmd/                        # Additional CLI tools
│   └── migrate/                # MongoDB migration tool
│       └── main.go             # Migration entry point
│
├── docker/                     # Docker-related files
│   └── Dockerfile              # Container build definition
│
└── docs/                       # Documentation site (Starlight)
    ├── astro.config.mjs        # Astro configuration
    ├── src/content/docs/       # Documentation pages
    └── ...                     # Static assets
```

## Key Directories Explained

### `/alita/db/` - Database Layer

The database layer implements the repository pattern:

```
db/
├── db.go               # Database connection, GORM setup
├── cache_helpers.go    # Cache key generators, TTL constants
├── shared_helpers.go   # Transaction helpers, bulk operations
├── bans_db.go          # Ban-related queries
├── users_db.go         # User storage operations
├── chats_db.go         # Chat settings operations
├── filters_db.go       # Filter storage
├── notes_db.go         # Notes storage
├── warns_db.go         # Warning system storage
└── ...                 # Other feature-specific files
```

Each `*_db.go` file contains:
- GORM model definitions
- Query functions (Get, Set, Delete)
- Cache invalidation calls

### `/alita/modules/` - Command Handlers

Feature modules contain command handlers and business logic:

```
modules/
├── help.go             # Help system, module registry
├── bans.go             # /ban, /kick, /restrict, etc.
├── mutes.go            # /mute, /unmute, /tmute
├── admin.go            # /promote, /demote, /adminlist
├── filters.go          # /filter, /stop, /filters
├── notes.go            # /save, /get, /notes
├── greetings.go        # /welcome, /goodbye, /setwelcome
├── warns.go            # /warn, /warns, /resetwarns
├── captcha.go          # CAPTCHA verification system
├── locks.go            # /lock, /unlock, /locks
├── blacklists.go       # /blacklist, /unblacklist
├── antiflood.go        # Anti-flood protection
├── reports.go          # /report, /reports
├── rules.go            # /rules, /setrules
├── users.go            # User tracking
├── connections.go      # Chat connections
├── disabling.go        # Command disabling
├── language.go         # Language settings
├── dev.go              # Developer commands
├── pin.go              # Pin management
├── purges.go           # Message purging
├── misc.go             # Miscellaneous commands
├── bot_updates.go      # Bot status tracking
├── antispam.go         # Anti-spam measures
└── mkdcmd.go           # Markdown commands
```

### `/alita/utils/chat_status/` - Permission Checking

Central permission validation:

```go
// Key functions available:
chat_status.RequireUserAdmin(b, ctx, chat, userId, justCheck)
chat_status.RequireBotAdmin(b, ctx, chat, justCheck)
chat_status.CanUserRestrict(b, ctx, chat, userId, justCheck)
chat_status.CanBotRestrict(b, ctx, chat, justCheck)
chat_status.CanUserDelete(b, ctx, chat, userId, justCheck)
chat_status.CanBotDelete(b, ctx, chat, justCheck)
chat_status.IsUserAdmin(b, chatId, userId)
chat_status.IsUserInChat(b, chat, userId)
chat_status.IsUserBanProtected(b, ctx, chat, userId)
```

### `/migrations/` - SQL Migrations

Database migrations follow naming convention:

```
migrations/
├── 001_initial_schema.sql
├── 002_add_users_table.sql
├── 003_add_chats_table.sql
├── 004_add_filters_table.sql
└── ...
```

Migrations are:
- Applied automatically on startup if `AUTO_MIGRATE=true`
- Tracked in `schema_migrations` table
- Idempotent (safe to run multiple times)
- Auto-cleaned of Supabase-specific SQL if present

### `/locales/` - Translation Files

YAML-based translations:

```yaml
# locales/en.yml
bans_ban_normal_ban: "Banned %s!"
bans_ban_ban_reason: "\nReason: %s"
bans_kick_kicked_user: "Kicked %s!"
common_no_user_specified: "You need to specify a user!"
chat_status_user_admin_cmd_error: "You need to be an admin to use this command!"
```

Key patterns:
- Use double quotes for strings with escape sequences (`\n`, `\t`)
- Use `%s`, `%d` for formatting placeholders
- Prefix keys with module name for organization

## Important Files

### `main.go` (Root)

Application entry point handling:
- Health check mode (`--health` flag)
- HTTP transport configuration
- Dispatcher setup with error handling
- Monitoring system initialization
- Graceful shutdown coordination
- Webhook/polling mode selection

### `alita/main.go`

Module loading and initialization:
- `LoadModules()` - Loads all feature modules in order
- `ListModules()` - Returns list of loaded modules
- `InitialChecks()` - Validates configuration, initializes cache
- `ResourceMonitor()` - Background resource tracking

### `alita/config/config.go`

Environment variable parsing:
- Required: `BOT_TOKEN`, `DATABASE_URL`, `REDIS_ADDRESS`
- Optional: Performance tuning, monitoring, webhooks
- Validation and default values

### `alita/db/db.go`

Database connection management:
- GORM configuration with connection pooling
- Auto-migration support
- Connection health checking
- Graceful connection closing

## Module Loading Order

Modules are loaded in a specific order in `alita/main.go`:

```go
func LoadModules(dispatcher *ext.Dispatcher) {
    modules.HelpModule.AbleMap.Init()
    defer modules.LoadHelp(dispatcher)  // Help loaded LAST

    modules.LoadBotUpdates(dispatcher)
    modules.LoadAntispam(dispatcher)
    modules.LoadLanguage(dispatcher)
    modules.LoadAdmin(dispatcher)
    modules.LoadPin(dispatcher)
    modules.LoadMisc(dispatcher)
    modules.LoadBans(dispatcher)
    modules.LoadMutes(dispatcher)
    modules.LoadPurges(dispatcher)
    modules.LoadUsers(dispatcher)
    modules.LoadReports(dispatcher)
    modules.LoadDev(dispatcher)
    modules.LoadLocks(dispatcher)
    modules.LoadFilters(dispatcher)
    modules.LoadAntiflood(dispatcher)
    modules.LoadNotes(dispatcher)
    modules.LoadConnections(dispatcher)
    modules.LoadDisabling(dispatcher)
    modules.LoadRules(dispatcher)
    modules.LoadWarns(dispatcher)
    modules.LoadGreetings(dispatcher)
    modules.LoadCaptcha(dispatcher)
    modules.LoadBlacklists(dispatcher)
    modules.LoadMkdCmd(dispatcher)
}
```

The order matters for:
- Handler priority (first registered wins for same trigger)
- Dependency resolution (users before bans)
- Help module collecting all registered commands

## Next Steps

- [Request Flow](/architecture/request-flow) - How updates are processed
- [Module Pattern](/architecture/module-pattern) - Adding new features
- [Caching](/architecture/caching) - Redis cache architecture
