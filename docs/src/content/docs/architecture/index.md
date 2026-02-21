---
title: Architecture Overview
description: High-level architecture and design principles of Alita Robot.
---

Alita Robot is a modern Telegram group management bot built with Go and the gotgbot library. This document provides an overview of the architectural decisions, technology stack, and design principles that guide the codebase.

## Technology Stack

| Component | Technology | Purpose |
|-----------|------------|---------|
| **Language** | Go 1.25+ | Core application runtime |
| **Telegram Library** | gotgbot v2 | Telegram Bot API wrapper |
| **Database** | PostgreSQL + GORM | Persistent data storage with ORM |
| **Caching** | Redis + gocache | Distributed caching layer |
| **Monitoring** | Prometheus | Metrics and observability |
| **Build System** | GoReleaser | Multi-platform builds and releases |

## Core Design Principles

### 1. Domain Functions Pattern

All database operations are organized by domain in `alita/db/*_db.go` files. Each file provides Get/Add/Update/Delete functions for its domain, with surrogate key pattern (auto-increment `id` as PK, external IDs as unique constraints):

```go
// Example: Direct domain function calls
settings := db.GetChatSettings(chatId)
db.UpdateChatSettings(chatId, newSettings)
```

### 2. Decorator Pattern

Middleware functionality is implemented through decorators in `alita/utils/decorators/`. Common cross-cutting concerns are handled uniformly:

- Permission checking (admin, restrict, delete rights)
- Error handling with panic recovery
- Logging and metrics collection

### 3. Worker Pools

Concurrent processing uses bounded worker pools with panic recovery:

- **Dispatcher**: Limited to 200 max goroutines by default (configurable via `DISPATCHER_MAX_ROUTINES`)
- **Message Pipeline**: Concurrent validation stages
- **Bulk Operations**: Parallel batch processors with generic framework

### 4. Redis Caching

Redis-based caching with stampede protection:

- Distributed cache using Redis for persistence across restarts
- Singleflight pattern prevents thundering herd on cache misses
- Configurable TTLs per data type (30min - 1hr typically)

## Request Flow Diagram

```
                                    +------------------+
                                    |    Telegram      |
                                    |    Bot API       |
                                    +--------+---------+
                                             |
                          +------------------+------------------+
                          |                                     |
                   Webhook Mode                           Polling Mode
                          |                                     |
                          v                                     v
               +----------+----------+              +-----------+-----------+
               |   HTTP Server       |              |      Updater          |
               |   /webhook/{secret} |              |   GetUpdates loop     |
               +----------+----------+              +-----------+-----------+
                          |                                     |
                          +------------------+------------------+
                                             |
                                             v
                                  +----------+----------+
                                  |     Dispatcher      |
                                  | (configurable routines)|
                                  +----------+----------+
                                             |
                         +-------------------+-------------------+
                         |                   |                   |
                         v                   v                   v
                  +------+------+     +------+------+     +------+------+
                  |   Handler   |     |   Handler   |     |   Handler   |
                  |  (Command)  |     | (Callback)  |     |  (Message)  |
                  +------+------+     +------+------+     +------+------+
                         |                   |                   |
                         +-------------------+-------------------+
                                             |
                              +--------------+--------------+
                              |                             |
                              v                             v
                    +---------+----------+        +---------+----------+
                    |   Redis Cache      |        |   PostgreSQL       |
                    |   (cache lookup)    |        |   (via GORM)       |
                    +--------------------+        +--------------------+
```

## Key Subsystems

### Dispatcher

The dispatcher routes incoming Telegram updates to appropriate handlers:

- Configurable max goroutines (default: 200)
- Enhanced error handler with structured logging
- Recovery from panics in any handler

```go
dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{
    Error: func(b *gotgbot.Bot, ctx *ext.Context, err error) ext.DispatcherAction {
        // Error handling with structured logging
        return ext.DispatcherActionNoop
    },
    MaxRoutines: config.AppConfig.DispatcherMaxRoutines,
})
```

### Module System

Each feature module follows a consistent pattern:

1. Define a `moduleStruct` with module name
2. Implement handler functions as methods
3. Register handlers in a `LoadModule` function
4. Module loaded via `alita/main.go` in specific order

### Cache Layer

Redis caching with stampede protection:

- **Cache Keys**: Prefixed with `alita:` for namespace isolation
- **Stampede Protection**: Singleflight prevents concurrent cache rebuilds
- **TTL Management**: Per-type expiration (settings: 30min, language: 1hr)

### Monitoring

Comprehensive monitoring subsystems:

- **Resource Monitor**: Tracks memory and goroutine usage every 5 minutes
- **Activity Monitor**: Automatic group activity tracking with configurable thresholds
- **Background Stats**: Performance metrics collection
- **Auto-Remediation**: GC triggers when memory exceeds thresholds

## Concurrency Model

### Bounded Concurrency

```go
// Dispatcher limits concurrent handler execution
MaxRoutines: 200  // Configurable via DISPATCHER_MAX_ROUTINES

// Worker pools use bounded parallelism
workerPool := NewWorkerPool(config.AppConfig.WorkerPoolSize)
```

### Singleflight for Cache

```go
// Prevents multiple goroutines from rebuilding same cache entry
var cacheGroup singleflight.Group

result, err, _ := cacheGroup.Do(cacheKey, func() (any, error) {
    // Only one goroutine executes this
    return loadFromDatabase()
})
```

### Graceful Shutdown

```go
// Shutdown manager coordinates cleanup in order
shutdownManager := shutdown.NewManager()
shutdownManager.RegisterHandler(func() error {
    // Cleanup monitoring, database, cache
    return nil
})
```

## Error Handling Strategy

Alita uses a 4-layer error handling hierarchy:

### Layer 1: Dispatcher Level

```go
Error: func(b *gotgbot.Bot, ctx *ext.Context, err error) ext.DispatcherAction {
    defer error_handling.RecoverFromPanic("DispatcherErrorHandler", "Main")
    // Log error with structured fields
    return ext.DispatcherActionNoop
}
```

### Layer 2: Worker Level

Worker pools implement panic recovery:

```go
go func() {
    defer func() {
        if r := recover(); r != nil {
            log.WithField("panic", r).Error("Panic in worker")
        }
    }()
    // Worker logic
}()
```

### Layer 3: Handler Level

Individual handlers use decorators for error handling:

```go
// Permission checks return early on failure
if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id, false) {
    return ext.EndGroups
}
```

### Layer 4: Decorator Level

Command decorators provide permission validation and error context:

```go
// Decorators wrap handlers with cross-cutting concerns
cmdDecorator.MultiCommand(dispatcher, []string{"cmd", "alias"}, handler)
```

## Database Schema Design

The database uses a **surrogate key pattern**:

- **Primary Keys**: Auto-incremented `id` field (internal identifier)
- **Business Keys**: `user_id` and `chat_id` with unique constraints
- **Benefits**:
  - Decouples internal schema from Telegram IDs
  - Stable identifiers if external systems change
  - Better performance for joins and indexing

## Next Steps

- [Project Structure](/architecture/project-structure) - Directory layout and key files
- [Request Flow](/architecture/request-flow) - Detailed update processing pipeline
- [Module Pattern](/architecture/module-pattern) - How to add new modules
- [Caching](/architecture/caching) - Redis caching architecture
