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
make psql-migrate  # Apply all pending PostgreSQL migrations (auto-cleans Supabase SQL)
make psql-status   # Check current migration status
make psql-reset    # Reset database - DANGEROUS: drops and recreates all tables
```

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
   - Two-tier caching (Ristretto L1 + Redis L2) with stampede protection

### Concurrency Architecture

The bot uses multiple worker pool patterns for performance:

- **Message Pipeline** (`concurrent_processing/message_pipeline.go`): Concurrent
  validation stages
- **Bulk Operations** (`db/bulk_operations.go`): Parallel batch processors with
  generic framework
- **Activity Monitor** (`monitoring/activity_monitor.go`): Automatic group
  activity tracking with configurable thresholds
- **Dispatcher**: Limited to 100 max goroutines to prevent explosion
- **Worker Safety** (`safety/worker_safety.go`): Panic recovery and rate
  limiting

### Caching Strategy

Two-layer cache system with fallback:

1. **L1 Cache** (Ristretto): In-memory, ultra-fast, LFU eviction
2. **L2 Cache** (Redis): Distributed, persistent across restarts
3. **Stampede Protection**: Distributed locking prevents thundering herd
4. **Cache Helpers** (`db/cache_helpers.go`): TTL management, invalidation
   patterns

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
- **Duplicate Prevention**: Unique constraints on `user_id` and `chat_id` prevent
  duplicates even though they're not primary keys
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

### Current Feature Branch: Captcha Module

The `dev/captcha-module` branch adds CAPTCHA verification for new members:

- **Math Captcha**: Generates secure random arithmetic problems with image
  rendering
- **Text Captcha**: Character recognition from distorted images
- **Refresh Mechanism**: Limited refreshes with cooldown to prevent abuse
- **Pre-Message Storage**: Captures messages sent before captcha completion
- **Security**: Uses crypto/rand for unpredictable challenges

## Environment Configuration

Required environment variables (see sample.env):

```bash
# Core
BOT_TOKEN          # Telegram bot token from @BotFather
DATABASE_URL       # PostgreSQL connection string
REDIS_ADDRESS      # Redis server address  
MESSAGE_DUMP       # Log channel ID (must start with -100)
OWNER_ID           # Your Telegram user ID

# Webhook Mode (optional)
USE_WEBHOOKS       # Set to 'true' for webhook mode
WEBHOOK_DOMAIN     # Your webhook domain
WEBHOOK_SECRET     # Random secret for validation
CLOUDFLARE_TUNNEL_TOKEN # For Cloudflare tunnel integration

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

- Supabase migrations are source of truth
- Auto-cleaning removes Supabase-specific SQL at runtime
- Applied to any PostgreSQL instance via `make psql-migrate`

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

### Webhook Mode

- Real-time updates, better for production
- Requires HTTPS endpoint (use Cloudflare Tunnel)
- Lower resource usage, instant response

## Build and Release

The project uses GoReleaser for multi-platform builds:

- Binaries for Darwin, Linux, Windows (amd64, arm64)
- Docker images published to ghcr.io/divideprojects/alita_robot
- GitHub Actions automates releases on version tags
- Supply chain security via attestation

## Important Notes

- The bot maintains backward compatibility with MongoDB (migration tool in
  `cmd/migrate/`)
- All database operations should use the repository pattern for testability
- Worker pools should implement panic recovery and rate limiting
- Cache invalidation must be handled explicitly when data changes
- Performance monitoring is automatic in production (DEBUG=false)
