# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with
code in this repository.

## Project Overview

Alita Robot is a modern Telegram group management bot built with Go and the
gotgbot library. It provides comprehensive group administration features
including user management, filters, greetings, anti-spam, captcha verification,
and multi-language support.

## Development Commands

### Essential Commands

```bash
make run          # Run the bot locally with current code
make build        # Build release artifacts using goreleaser
make lint         # Run golangci-lint for code quality checks
make tidy         # Clean up and download go.mod dependencies
```

### PostgreSQL Migration Commands

```bash
make psql-migrate  # Apply all pending PostgreSQL migrations
make psql-status   # Check current migration status
make psql-reset    # Reset database - DANGEROUS: drops and recreates all tables
```

### Automatic Database Migrations

The bot now supports automatic database migrations on startup. This feature
eliminates the need to manually run `make psql-migrate`.

#### How It Works

1. **Migration Files**: SQL migrations are stored in `migrations/`
2. **Auto-Cleaning**: Supabase-specific SQL (GRANT statements, RLS policies) is
   automatically removed if present
3. **Version Tracking**: Applied migrations are tracked in `schema_migrations`
   table
4. **Idempotent**: Migrations are only applied once, safe to run multiple times
5. **Transactional**: Each migration runs in a transaction for atomicity

#### Configuration

Enable auto-migration by setting environment variables:

```bash
# Enable automatic migrations on startup
AUTO_MIGRATE=true

# Optional: Continue running even if migrations fail (not recommended for production)
AUTO_MIGRATE_SILENT_FAIL=false

# Optional: Custom migration directory (defaults to migrations)
MIGRATIONS_PATH=migrations
```

#### Migration Process

When `AUTO_MIGRATE=true`, the bot will:

1. Check for pending migrations in `migrations/`
2. Clean Supabase-specific SQL commands automatically if present
3. Apply migrations in alphabetical order
4. Track applied migrations in `schema_migrations` table
5. Log migration status and any errors

#### Manual Migration

If you prefer manual control, keep `AUTO_MIGRATE=false` (default) and use:

- `make psql-migrate` - For any PostgreSQL instance

## High-Level Architecture

### Core Request Flow

1. **Entry Point** (`main.go`): Initializes bot, database, cache, and registers
   all command handlers
2. **Command Registration**: Each module in `alita/modules/` registers its
   handlers via init functions
3. **Middleware Pipeline**:
   - Commands pass through decorators (`alita/utils/decorators/`)
   - Permission checking via `chat_status` utilities
   - Error handling with panic recovery at multiple levels
4. **Data Access Pattern**:
   - Repository pattern with interfaces in `db/repositories/interfaces/`
   - Implementations use GORM with connection pooling
   - Redis caching with stampede protection via singleflight

### Concurrency Architecture

The bot uses several concurrency patterns for performance:

- **Activity Monitor** (`alita/utils/monitoring/activity_monitor.go`): Automatic
  group activity tracking with configurable thresholds
- **Auto Remediation** (`alita/utils/monitoring/auto_remediation.go`): Memory
  monitoring and GC triggering when thresholds exceeded
- **Background Stats** (`alita/utils/monitoring/background_stats.go`): Background
  statistics collection and performance monitoring
- **Dispatcher**: Limited to 100 max goroutines to prevent explosion
- **Async Operations** (`alita/utils/async/`): Utilities for async database
  operations with error handling

### Caching Strategy

Redis-based caching with stampede protection:

1. **Redis Cache**: Distributed, persistent across restarts, handles all caching
2. **Stampede Protection**: Uses `singleflight` pattern to prevent thundering herd
3. **Cache Helpers** (`db/cache_helpers.go`): TTL management, invalidation patterns
4. **Admin Cache** (`alita/utils/cache/adminCache.go`): Specialized caching for admin member lists

### Database Optimization Patterns

- **Batch Prefetching** (`db/optimized_queries.go`): Reduces N+1 queries by
  loading related data
- **Singleton Queries**: Reusable query patterns with caching
- **Bulk Operations**: Generic parallel processing framework
- **Transaction Support** (`db/shared_helpers.go`): Automatic rollback on errors

### Database Schema Design

The database uses a **surrogate key pattern** for all tables:

- **Primary Keys**: Each table has an auto-incremented `id` field as the primary
  key (internal identifier)
- **Business Keys**: External identifiers like `user_id` (Telegram user ID) and
  `chat_id` (Telegram chat ID) are stored with unique constraints
- **Benefits of This Pattern**:
  - Decouples internal schema from external systems (Telegram IDs)
  - Provides stability if external IDs change or new platforms are added
  - Simplifies GORM operations with consistent integer primary keys
  - Better performance for joins and indexing
- **Duplicate Prevention**: Unique constraints on `user_id` and `chat_id`
  prevent duplicates even though they're not primary keys
- **Exception**: The `chat_users` join table uses a composite primary key
  `(chat_id, user_id)` since each pair must be unique

### Module Development Pattern

When adding new features:

1. Create database models and operations in `alita/db/*_db.go`
2. Implement command handlers in `alita/modules/*.go`
3. Register commands in module's init function
4. Use decorators for common middleware (permission checks, error handling)
5. Follow repository pattern for data access
6. Add translations to `locales/` for user-facing strings

### Activity Monitoring System

The bot includes automatic group activity tracking that replaces the manual
dbclean command:

- **Automatic Activity Tracking**: Updates `last_activity` timestamp on every
  message
- **Configurable Thresholds**: Set inactivity period before marking groups as
  inactive
- **Activity Metrics**: Tracks Daily Active Groups (DAG), Weekly Active Groups
  (WAG), and Monthly Active Groups (MAG)
- **Background Processing**: Hourly checks for inactive groups with automatic
  cleanup
- **Smart Reactivation**: Automatically reactivates groups when they become
  active again

### Captcha Module

The captcha module (`alita/modules/captcha.go`) provides CAPTCHA verification for new members:

- **Math Captcha**: Generates secure random arithmetic problems with image
  rendering
- **Text Captcha**: Character recognition from distorted images
- **Refresh Mechanism**: Limited refreshes with cooldown to prevent abuse
- **Pre-Message Storage**: Captures messages sent before captcha completion
- **Security**: Uses crypto/rand for unpredictable challenges
- **Recovery**: Automatic cleanup of orphaned captcha attempts on bot restart

## Environment Configuration

Required environment variables (see sample.env):

```bash
# Core
BOT_TOKEN          # Telegram bot token from @BotFather
DATABASE_URL       # PostgreSQL connection string
REDIS_ADDRESS      # Redis server address
MESSAGE_DUMP       # Log channel ID (must start with -100)
OWNER_ID           # Your Telegram user ID

# HTTP Server (unified health, metrics, webhook on single port)
HTTP_PORT          # HTTP server port (default: 8080)

# Webhook Mode (optional)
USE_WEBHOOKS       # Set to 'true' for webhook mode
WEBHOOK_DOMAIN     # Your webhook domain
WEBHOOK_SECRET     # Random secret for validation
CLOUDFLARE_TUNNEL_TOKEN # For Cloudflare tunnel integration

# Note: WEBHOOK_PORT is deprecated, use HTTP_PORT instead

# Performance Tuning (optional)
WORKER_POOL_SIZE   # Concurrent worker pool size (default: 10)
CACHE_TTL          # Cache time-to-live in seconds (default: 300)

# Activity Monitoring (optional)
INACTIVITY_THRESHOLD_DAYS  # Days before marking chat inactive (default: 30)
ACTIVITY_CHECK_INTERVAL    # Hours between activity checks (default: 1)
ENABLE_AUTO_CLEANUP        # Auto-mark inactive chats (default: true)
```

## Critical Patterns to Understand

### 1. Permission Checking Flow

- All admin commands use `chat_status.RequireUserAdmin()`
- Permissions are cached to reduce API calls
- Bot admin status checked separately with `RequireBotAdmin()`

### 2. Error Handling Hierarchy

- Panic recovery at dispatcher level (main.go)
- Worker-level recovery in pools
- Handler-level recovery in decorators
- Centralized error logging via `error_handling` package

### 3. Graceful Shutdown

- Shutdown manager (`shutdown/graceful.go`) coordinates cleanup
- Handlers registered in order of dependency
- Database connections, cache, and webhooks cleaned up properly

### 4. Resource Monitoring

- Auto-remediation triggers GC when memory exceeds thresholds
- Background stats collection for performance metrics
- Resource monitor tracks memory and goroutine usage

### 5. Migration System

- SQL migrations in `migrations/` are the source of truth
- Auto-cleaning removes Supabase-specific SQL at runtime if present
- Applied to any PostgreSQL instance via `make psql-migrate` or `AUTO_MIGRATE=true`

## Testing Approach

The project uses golangci-lint for comprehensive code quality checks but doesn't
have traditional unit tests. Instead:

- Use `make lint` before commits
- Test handlers manually with a test bot/group
- Check logs in MESSAGE_DUMP channel for errors
- Monitor resource usage via built-in monitoring

## Deployment Modes

### Polling Mode (Default)

- Simple setup, no external configuration needed
- Suitable for development and low-traffic bots
- Higher latency (1-3 second delay)
- HTTP server runs on HTTP_PORT (default 8080) for /health and /metrics

### Webhook Mode

- Real-time updates, better for production
- Requires HTTPS endpoint (use Cloudflare Tunnel)
- Lower resource usage, instant response
- Single HTTP server on HTTP_PORT serves /health, /metrics, and /webhook

### HTTP Endpoints (Unified Server)

All endpoints run on a single port (HTTP_PORT, default 8080):
- `GET /health` - Health check with database/redis status
- `GET /metrics` - Prometheus metrics
- `POST /webhook/{secret}` - Telegram webhook (webhook mode only)

## Build and Release

The project uses GoReleaser for multi-platform builds:

- Binaries for Darwin, Linux, Windows (amd64, arm64)
- Docker images published to ghcr.io/divkix/alita_robot
- GitHub Actions automates releases on version tags
- Supply chain security via attestation

## Important Notes

- The bot maintains backward compatibility with MongoDB (migration tool in
  `cmd/migrate/`)
- All database operations should use the repository pattern for testability
- Worker pools should implement panic recovery and rate limiting
- Cache invalidation must be handled explicitly when data changes
- Performance monitoring is automatic in production (DEBUG=false)

## Development Lessons Learned

### Type Safety in Internationalization (i18n)
- **Issue**: Printf-style formatters (%d, %s) must receive correct data types
- **Fix**: Pass integers directly to %d formatters, not string conversions
- **Pattern**: Use `settings.Timeout` instead of `strconv.Itoa(settings.Timeout)` for %d
- **Testing**: Always test with different languages to catch formatting errors

### YAML Escape Sequences in Translation Files
- **Issue**: Single-quoted strings in YAML preserve escape sequences literally (`\n` appears as text)
- **Fix**: Use double quotes for strings containing escape sequences like `\n`, `\t`, `\"`
- **Pattern**: Change `'Rules for <b>%s</b>:\n\n%s'` to `"Rules for <b>%s</b>:\n\n%s"`
- **Root Cause**: YAML spec treats single quotes as literal strings, double quotes interpret escapes
- **Testing**: Validate YAML parsing and verify escape sequences render correctly in bot messages
- **Hybrid Parameter System**: The translation system supports both named parameters in code (`{"first": value}`) and positional formatters in YAML (`%s`) through intelligent mapping

### Complete Feature Implementation
- **Issue**: Features mentioned in documentation may be partially implemented
- **Solution**: Check for gaps between documented features and actual code
- **Pattern**: When adding user-facing features, implement the full UX flow:
  1. Core functionality
  2. Error handling and user feedback
  3. Cleanup and resource management
  4. Internationalization support

### Database Design for Message Storage
- **Pattern**: Use surrogate keys (auto-increment ID) as primary keys
- **Foreign Keys**: Always include proper foreign key relationships with CASCADE DELETE
- **Indexing**: Add composite indexes for frequent query patterns (user_id, chat_id)
- **Migration Strategy**: Use descriptive timestamps and meaningful migration names

### Message Handler Priority and Interception
- **Critical**: Use negative group numbers (-10) for handlers that need to intercept early
- **Pattern**: Handlers that prevent further processing should return `ext.EndGroups`
- **Safety**: Always check user permissions and attempt existence before processing

### Internationalization Best Practices
- **Consistency**: Use the same parameter passing style across all translations
- **Coverage**: Add translations for ALL user-facing strings, not just core features
- **Languages**: Maintain feature parity across all supported locales
- **Testing**: Verify translation keys exist before using them in production

### Error Handling and User Feedback
- **Transparency**: Always inform users about actions taken by the bot
- **Context**: Include relevant details (attempt counts, time remaining, etc.)
- **Cleanup**: Properly clean up resources and temporary data on both success and failure
- **Timeouts**: Use temporary messages that auto-delete to keep chats clean

### Database Error Handling and Nil Safety
- **Issue**: Functions returning `(value, error)` where error is ignored with `_` can cause nil pointer panics
- **Fix**: Always check errors from database operations; if the function can return nil on error, check before accessing
- **Pattern**:
  ```go
  settings, err := db.GetSettings(chatId)
  if err != nil {
      log.Errorf("Failed to get settings: %v", err)
      settings = &db.Settings{Enabled: false} // Use safe default
  }
  if settings != nil && settings.Enabled { ... } // Safe access
  ```
- **Root Cause**: Go's blank identifier `_` silently discards errors, hiding potential nil returns

### Synchronous Operations Before User Confirmation
- **Issue**: Goroutines for database writes send success messages before operations complete
- **Fix**: Perform database writes synchronously when the user needs confirmation of success
- **Pattern**: Execute DB operation first, then send confirmation message only after it completes
- **When Async is OK**: Background cleanup, logging, or non-critical operations that don't need user confirmation

### Database Schema and Struct Synchronization
- **Issue**: Database functions accepting parameters that the schema cannot store leads to silent data loss
- **Example**: `UpdateChannel(channelId, channelName, username)` received name and username but the `channels` table had no columns for them
- **Fix**: Always verify that:
  1. Database table columns match the GORM struct fields
  2. Function parameters can actually be persisted
  3. Early returns don't prevent legitimate updates
- **Pattern**: When adding parameters to a database function:
  1. Add migration for new columns first
  2. Update GORM struct to include new fields
  3. Update optimized queries SELECT to include new columns
  4. Then update the function to use them
- **Testing**: Verify data is actually stored by querying after insert/update

### Async Database Operations with Error Handling
- **Issue**: Fire-and-forget goroutines (`go db.UpdateX()`) lose errors
- **Fix**: Create wrapper functions that handle errors from async operations:
  ```go
  func asyncUpdateUser(userId int64, username, name string) {
      if err := db.UpdateUser(userId, username, name); err != nil {
          log.Warnf("[Users] Failed to update user %d: %v", userId, err)
      }
  }
  ```
- **Pattern**: Use wrapper functions for all async DB calls to ensure errors are logged
- **Best Practice**: Functions should return errors even if callers discard them for debugging

### Markdown/HTML Parse Mode Mismatch in Help System
- **Issue**: Locale help strings use Markdown formatting (`*bold*`, `` `code` ``) but bot sends with HTML parse mode
- **Symptom**: Help text displays raw asterisks instead of bold text, making help "appear broken"
- **Root Cause**: The `_help_msg` locale strings were written in Markdown, but `getModuleHelpAndKb()` sent messages with `ParseMode: helpers.HTML`
- **Fix**: Use `tgmd2html.MD2HTMLV2()` to convert Markdown to HTML before sending:
  ```go
  helpText = tgmd2html.MD2HTMLV2(fmt.Sprintf(headerTemplate, ModName) + helpMsg)
  ```
- **Pattern**: When working with locale strings that may contain Markdown:
  1. Check if the locale uses Markdown formatting (`*bold*`, `` `code` ``, etc.)
  2. Check the parse mode used when sending the message
  3. Use `tgmd2html.MD2HTMLV2()` to convert if there's a mismatch
- **Prevention**: Consistently use HTML in locale files if the bot primarily uses HTML parse mode

### Callback Query Handler Best Practices
- **Issue**: Callback handlers with `strings.Split(data, ".")[1]` can panic on malformed data
- **Fix**: Always validate callback data format before accessing split results:
  ```go
  parts := strings.Split(query.Data, ".")
  if len(parts) < 2 {
      log.Warnf("[Module] Invalid callback data format: %s", query.Data)
      _, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Invalid selection."})
      return ext.EndGroups
  }
  value := parts[1]
  ```
- **Pattern**: For permission-gated callbacks (e.g., admin-only actions):
  1. Validate callback data format first
  2. Check permissions before any other operations
  3. Return early on permission failure (don't answer callback twice)
  4. Create expensive resources (translators, etc.) only after validation passes
- **Double Answer Bug**: `RequireUserAdmin` with `justCheck=false` already answers the callback with error - don't answer again in calling code
- **Best Practice**: Match the early-return pattern used in command handlers (like `changeLanguage`) for consistency

### Message Handler Sender Safety
- **Issue**: `ctx.EffectiveSender.User` can be nil when message is from a channel, not a user
- **Fix**: Use `ctx.EffectiveSender` with nil check, then `sender.Id()` which works for both users and channels:
  ```go
  sender := ctx.EffectiveSender
  if sender == nil {
      return ext.ContinueGroups
  }
  senderID := sender.Id()
  // IsUserAdmin handles channel IDs safely (returns false for channels)
  if chat_status.IsUserAdmin(b, chat.Id, senderID) {
      return ext.ContinueGroups
  }
  ```
- **Pattern**: Always check for nil sender in message handlers before accessing user properties
- **Note**: `IsUserAdmin` returns false for channel IDs (negative numbers < -1000000000000)

### Cache Invalidation on Updates
- **Issue**: Cached data not invalidated after DB updates causes stale reads (e.g., locks not enforced immediately)
- **Fix**: Add cache invalidation after successful database writes:
  ```go
  func UpdateLock(chatID int64, perm string, val bool) error {
      // ... DB update ...
      InvalidateLockCache(chatID, perm) // Delete cached value
      return nil
  }
  ```
- **Pattern**: When adding caching, always consider the write path - cache must be invalidated on updates
- **Cache Key Format**: Use consistent prefixes like `alita:lock:{chatID}:{lockType}`

### Boolean Logic in Filter Functions
- **Issue**: Complex boolean conditions with `||` can accidentally match most messages
- **Example**: `IsAnonymousChannel() || !IsLinkedChannel()` matches almost everything (NOT linked = most messages)
- **Fix**: Carefully review boolean logic in filter functions:
  ```go
  // Wrong - matches almost all messages
  return sender.IsAnonymousChannel() || !sender.IsLinkedChannel()
  // Correct - matches only channel messages
  return sender.IsAnonymousChannel() || sender.IsLinkedChannel()
  ```
- **Best Practice**: Test filter functions with different message types to verify correct matching

### Message Entity Completeness
- **Issue**: Checking only `msg.Entities` misses entities in captions of media messages
- **Fix**: Also check `msg.CaptionEntities` for completeness:
  ```go
  for _, e := range msg.Entities { /* check */ }
  for _, e := range msg.CaptionEntities { /* check */ }
  ```
- **Applies to**: URL detection, mention detection, any entity-based filtering

### Redundant Context Mutations
- **Issue**: `helpers.IsUserConnected()` already sets `ctx.EffectiveChat` before returning
- **Pattern**: After calling `IsUserConnected()`, use `ctx.EffectiveChat` directly - no need to reassign:
  ```go
  // IsUserConnected sets ctx.EffectiveChat internally
  if helpers.IsUserConnected(b, ctx, true, true) == nil {
      return ext.EndGroups
  }
  chat := ctx.EffectiveChat // Already set correctly
  ```

### Translation Key Mismatches
- **Issue**: Code uses translation keys that don't exist in locale files, causing empty bot responses
- **Symptom**: Bot replies with empty messages or no response at all
- **Root Cause**: Key names in code don't match key names in YAML locale files
- **Example**: Code uses `misc_translate_need_text` but locale has `misc_need_text_and_lang`
- **Fix**: Always verify translation keys exist in ALL locale files before using them:
  ```bash
  # Search for key across all locales
  grep -r "misc_example_key" locales/
  ```
- **Prevention**:
  1. Add translation keys to ALL supported locales simultaneously
  2. Use consistent naming conventions (module_action_context)
  3. Check both en.yml and es.yml when adding new keys
  4. Test the feature in multiple languages
- **Best Practice**: When adding a new command/feature, add all its translation keys to a checklist and verify each exists

### Struct Field Consistency
- **Issue**: Related struct fields (e.g., `Dev` and `IsDev`) not consistently set in all code paths
- **Example**: `GetTeamMemInfo` fallback sets `IsDev: false` but not `Dev: false`
- **Fix**: When a struct has related/alias fields, set both consistently:
  ```go
  // Correct - sets both related fields
  devrc = &DevSettings{UserId: userID, IsDev: false, Dev: false, Sudo: false}

  // Wrong - only sets one field, leaving alias inconsistent
  devrc = &DevSettings{UserId: userID, IsDev: false, Sudo: false}
  ```
- **Pattern**: Audit all places where structs with alias fields are created or modified
