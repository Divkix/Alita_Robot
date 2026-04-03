# Architecture

**What belongs here:** High-level architecture of the Alita Robot system.
**What does NOT belong here:** Implementation details, specific bug fixes.

---

## System Overview

Alita Robot is a Telegram group management bot built with Go 1.25+ using the gotgbot library.

## Architecture Layers

### 1. Entry Point (main.go)
- Configuration loading
- Service initialization (DB, cache, i18n, tracing)
- Bot setup and connection
- Dispatcher creation
- HTTP server setup
- Module loading
- Graceful shutdown handling

### 2. Module System (alita/modules/)
- Handler-based architecture using gotgbot dispatcher
- Module loading via `LoadXxx(dispatcher)` functions
- Handler groups for command routing
- Callback handling for inline keyboards
- Each module: commands, handlers, callbacks

### 3. Database Layer (alita/db/)
- PostgreSQL with GORM
- Connection pooling
- Surrogate key pattern (auto-increment id)
- Cache integration for reads
- Cache invalidation on writes

### 4. Cache Layer (alita/utils/cache/)
- Redis-based caching
- Singleflight protection for stampede prevention
- Key format: `alita:{module}:{identifier}`
- Traced operations for OpenTelemetry

### 5. Utilities (alita/utils/)
- **chat_status**: Permission checking, admin verification
- **helpers**: Common helper functions, decorators
- **extraction**: Message parsing, user ID extraction
- **error_handling**: Panic recovery
- **callbackcodec**: Callback data encoding/decoding
- **keyword_matcher**: Pattern matching with Aho-Corasick
- **media**: Unified media sending
- **monitoring**: Activity tracking, stats, auto-remediation
- **shutdown**: Graceful shutdown coordination
- **tracing**: OpenTelemetry integration

### 6. i18n (alita/i18n/)
- YAML-based translations
- Per-language Translator instances
- Markdown in YAML, HTML output

## Data Flow

1. Telegram update received (webhook or polling)
2. Dispatcher routes to handler based on filters
3. Handler processes command/action
4. DB/cache operations as needed
5. Response sent back to Telegram

## Critical Patterns

### Error Handling
- Four-layer recovery: dispatcher → worker pool → decorator → handler
- Expected Telegram errors filtered via helpers
- DB errors never ignored

### Nil Safety
- EffectiveSender can be nil (channel messages)
- GetChat() can return nil (deleted messages)
- DB results must be checked before access

### Concurrency
- Dispatcher uses configurable goroutine pool
- Singleflight for cache stampede protection
- Background workers for monitoring
- Graceful shutdown with LIFO handler execution
