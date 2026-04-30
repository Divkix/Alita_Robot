---
title: Environment Variables
description: Configuration reference for all environment variables
---

# ⚙️ Environment Variables

This page documents all environment variables used to configure Alita Robot.

## 📂 Activity monitoring configuration

### `ACTIVITY_CHECK_INTERVAL`

Hours between activity checks

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Default** | `1` |
| **Validation** | min=1,max=24 |

### `ENABLE_AUTO_CLEANUP`

Whether to automatically mark inactive chats

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |
| **Default** | `true` |

### `INACTIVITY_THRESHOLD_DAYS`

Days before marking a chat as inactive

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Default** | `30` |
| **Validation** | min=1,max=365 |

## 📂 Bot settings

### `MESSAGE_DUMP` (Required)

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | Yes |
| **Validation** | required,min=1 |

### `OWNER_ID` (Required)

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | Yes |
| **Validation** | required,min=1 |

### `DROP_PENDING_UPDATES`

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |
| **Default** | `false` |

### `ENABLED_LOCALES`

Comma-separated list of enabled language codes (e.g., `en,es,fr,hi`). Only these locales will be loaded.

| Property | Value |
|----------|-------|
| **Type** | `string[]` |
| **Required** | No |
| **Default** | `en` |

## 📂 Core configuration

### `BOT_TOKEN` (Required)

| Property | Value |
|----------|-------|
| **Type** | `string` |
| **Required** | Yes |
| **Validation** | required |

### `API_SERVER`

Custom Telegram Bot API server URL. Used with local telegram-bot-api server.

| Property | Value |
|----------|-------|
| **Type** | `string` |
| **Required** | No |
| **Default** | `https://api.telegram.org` |

### `TELEGRAM_API_ID`

Telegram API ID for local telegram-bot-api server. Obtain from https://my.telegram.org/apps.

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |

### `TELEGRAM_API_HASH`

Telegram API hash for local telegram-bot-api server. Obtain from https://my.telegram.org/apps.

| Property | Value |
|----------|-------|
| **Type** | `string` |
| **Required** | No |

### `DEBUG`

Enables debug logging and disables automatic performance monitoring.

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |
| **Default** | `false` |

### `WORKING_MODE`

| Property | Value |
|----------|-------|
| **Type** | `string` |
| **Required** | No |
| **Default** | `worker` |

## 📂 Database configuration

### `DATABASE_URL` (Required)

| Property | Value |
|----------|-------|
| **Type** | `string` |
| **Required** | Yes |
| **Validation** | required |

## 📂 Database connection pool configuration

### `DB_CONN_MAX_IDLE_TIME_MIN`

Max idle time in minutes

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Default** | `60` |
| **Validation** | min=1,max=60 |

### `DB_CONN_MAX_LIFETIME_MIN`

Max lifetime in minutes

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Default** | `240` |
| **Validation** | min=1,max=1440 |

### `DB_MAX_IDLE_CONNS`

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Default** | `50` |
| **Validation** | min=1,max=100 |

### `DB_MAX_OPEN_CONNS`

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Default** | `200` |
| **Validation** | min=1,max=1000 |

## 📂 Database migration settings

### `AUTO_MIGRATE`

Enable automatic database migrations on startup

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |
| **Default** | `false` |

### `AUTO_MIGRATE_SILENT_FAIL`

Continue running even if migrations fail

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |
| **Default** | `false` |

### `MIGRATIONS_PATH`

Path to migration files

| Property | Value |
|----------|-------|
| **Type** | `string` |
| **Required** | No |
| **Default** | `migrations` |

## 📂 Database monitoring configuration

### `ENABLE_DB_MONITORING`

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |
| **Default** | `false` |

## 📂 Performance optimization settings

### `BATCH_REQUEST_TIMEOUT_MS`

Batch request timeout in milliseconds

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Default** | `100` |
| **Validation** | min=10,max=5000 |

### `ENABLE_ASYNC_PROCESSING`

Enable async processing for non-critical operations

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |
| **Default** | `true` |

### `ENABLE_BATCH_REQUESTS`

Enable batch API requests

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |
| **Default** | `true` |

### `ENABLE_CACHE_PREWARMING`

Enable cache prewarming on startup

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |
| **Default** | `true` |

### `ENABLE_HTTP_CONNECTION_POOLING`

Enable HTTP connection pooling

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |
| **Default** | `true` |

### `ENABLE_QUERY_PREFETCHING`

Enable query batching and prefetching

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |
| **Default** | `true` |

### `ENABLE_RESPONSE_CACHING`

Enable response caching

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |
| **Default** | `true` |

### `HTTP_MAX_IDLE_CONNS`

HTTP connection pool size

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Default** | `100` |
| **Validation** | min=10,max=1000 |

### `HTTP_MAX_IDLE_CONNS_PER_HOST`

HTTP connections per host

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Default** | `50` |
| **Validation** | min=5,max=500 |

### `RESPONSE_CACHE_TTL`

Response cache TTL in seconds

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Default** | `30` |
| **Validation** | min=1,max=3600 |

## 📂 Profiling configuration

### `ENABLE_PPROF`

Enable pprof endpoints for performance profiling (development only)

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |
| **Default** | `false` |

## 📂 HTTP Server configuration

### `HTTP_PORT`

Unified HTTP server for health checks, metrics, and webhooks.

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Default** | `8080` |
| **Validation** | min=1,max=65535 |

## 📂 Redis configuration

### `REDIS_ADDRESS`

Redis host:port. Alias for `REDIS_URL` host component. Either `REDIS_ADDRESS` or `REDIS_URL` is required.

| Property | Value |
|----------|-------|
| **Type** | `string` |
| **Required** | Yes* |
| **Default** | `localhost:6379` |

*Required if `REDIS_URL` is not set.

### `REDIS_URL`

Standard Redis URL (`redis://user:password@host:port`). If set, `REDIS_ADDRESS` and `REDIS_PASSWORD` are extracted from it automatically. Takes lower priority than `REDIS_ADDRESS`/`REDIS_PASSWORD` if both are set.

| Property | Value |
|----------|-------|
| **Type** | `string` |
| **Required** | Yes* |

*Required if `REDIS_ADDRESS` is not set.

### `REDIS_DB`

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Default** | `1` |

### `REDIS_PASSWORD`

| Property | Value |
|----------|-------|
| **Type** | `string` |
| **Required** | No |

## 📂 Resource monitoring limits

### `RESOURCE_GC_THRESHOLD_MB`

Memory threshold for triggering GC

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Default** | `400` |
| **Validation** | min=100,max=5000 |

### `RESOURCE_MAX_GOROUTINES`

Maximum goroutines before triggering cleanup

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Default** | `1000` |
| **Validation** | min=100,max=10000 |

### `RESOURCE_MAX_MEMORY_MB`

Maximum memory usage in MB

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Default** | `500` |
| **Validation** | min=100,max=10000 |

## 📂 Safety and performance limits

### `CLEAR_CACHE_ON_STARTUP`

Whether to clear all caches on bot startup

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |
| **Default** | `true` |

### `DISPATCHER_MAX_ROUTINES`

Max concurrent goroutines for dispatcher

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Default** | `200` |
| **Validation** | min=1,max=1000 |

### `ENABLE_BACKGROUND_STATS`

Automatically enabled in production (when `DEBUG=false`). Set to `false` to disable.

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |
| **Default** | `true` (production), `false` (debug) |

### `ENABLE_PERFORMANCE_MONITORING`

Automatically enabled in production (when `DEBUG=false`). Set to `false` to disable.

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |
| **Default** | `true` (production), `false` (debug) |

### `MAX_CONCURRENT_OPERATIONS`

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Default** | `50` |
| **Validation** | min=1,max=1000 |

### `OPERATION_TIMEOUT_SECONDS`

Timeout for operations in seconds. Converted to `time.Duration` internally at `OperationTimeoutSeconds * time.Second`.

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Default** | `30` |
| **Validation** | min=1,max=300 |

## 📂 Webhook configuration

### `USE_WEBHOOKS`

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |
| **Default** | `false` |

### `WEBHOOK_DOMAIN`

Required when `USE_WEBHOOKS=true`.

| Property | Value |
|----------|-------|
| **Type** | `string` |
| **Required** | Conditional |

### `WEBHOOK_PORT`

Deprecated: use `HTTP_PORT` instead.

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Default** | `8081` |
| **Validation** | min=1,max=65535 |

### `WEBHOOK_SECRET`

Required when `USE_WEBHOOKS=true`.

| Property | Value |
|----------|-------|
| **Type** | `string` |
| **Required** | Conditional |

### `CLOUDFLARE_TUNNEL_TOKEN`

Token for Cloudflare Tunnel (cloudflared) when using tunnel mode for webhooks.

| Property | Value |
|----------|-------|
| **Type** | `string` |
| **Required** | No |

## 📂 Worker pool configuration for concurrent processing

### `BULK_OPERATION_WORKERS`

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Default** | `4` |
| **Validation** | min=1,max=20 |

### `CACHE_WORKERS`

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Default** | `3` |
| **Validation** | min=1,max=20 |

### `CHAT_VALIDATION_WORKERS`

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Default** | `10` |
| **Validation** | min=1,max=100 |

### `DATABASE_WORKERS`

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Default** | `5` |
| **Validation** | min=1,max=50 |

### `MESSAGE_PIPELINE_WORKERS`

Default is number of CPU cores, capped at 8.

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Default** | `runtime.NumCPU()` (max 8) |
| **Validation** | min=1,max=50 |

### `STATS_COLLECTION_WORKERS`

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Default** | `2` |
| **Validation** | min=1,max=10 |

## Quick Reference

### Required Variables

```bash
BOT_TOKEN=
DATABASE_URL=
MESSAGE_DUMP=
OWNER_ID=
REDIS_ADDRESS=       # or REDIS_URL
```

### Optional Variables

```bash
ACTIVITY_CHECK_INTERVAL=# hours between activity checks (default: 1)
API_SERVER=# custom API server URL (default: https://api.telegram.org)
AUTO_MIGRATE=# enable automatic database migrations (default: false)
AUTO_MIGRATE_SILENT_FAIL=# continue even if migrations fail (default: false)
BATCH_REQUEST_TIMEOUT_MS=# batch timeout in ms (default: 100)
BULK_OPERATION_WORKERS=# (default: 4)
CACHE_WORKERS=# (default: 3)
CHAT_VALIDATION_WORKERS=# (default: 10)
CLEAR_CACHE_ON_STARTUP=# clear all caches on startup (default: true)
CLOUDFLARE_TUNNEL_TOKEN=# Cloudflare tunnel token for webhooks
DATABASE_WORKERS=# (default: 5)
DB_CONN_MAX_IDLE_TIME_MIN=# max idle time in min (default: 60)
DB_CONN_MAX_LIFETIME_MIN=# max lifetime in min (default: 240)
DB_MAX_IDLE_CONNS=# (default: 50)
DB_MAX_OPEN_CONNS=# (default: 200)
DEBUG=# enable debug logging (default: false)
DISPATCHER_MAX_ROUTINES=# (default: 200)
DROP_PENDING_UPDATES=# (default: false)
ENABLE_ASYNC_PROCESSING=# (default: true)
ENABLE_AUTO_CLEANUP=# (default: true)
ENABLE_BACKGROUND_STATS=# (default: true in prod, false in debug)
ENABLE_BATCH_REQUESTS=# (default: true)
ENABLE_CACHE_PREWARMING=# (default: true)
ENABLE_DB_MONITORING=# (default: false)
ENABLE_HTTP_CONNECTION_POOLING=# (default: true)
ENABLE_PERFORMANCE_MONITORING=# (default: true in prod, false in debug)
ENABLE_PPROF=# enable pprof endpoints (default: false)
ENABLE_QUERY_PREFETCHING=# (default: true)
ENABLE_RESPONSE_CACHING=# (default: true)
ENABLED_LOCALES=# comma-separated language codes (default: en)
HTTP_MAX_IDLE_CONNS=# (default: 100)
HTTP_MAX_IDLE_CONNS_PER_HOST=# (default: 50)
HTTP_PORT=# unified HTTP server port (default: 8080)
INACTIVITY_THRESHOLD_DAYS=# days before marking inactive (default: 30)
MAX_CONCURRENT_OPERATIONS=# (default: 50)
MESSAGE_PIPELINE_WORKERS=# (default: NumCPU, max 8)
MIGRATIONS_PATH=# path to migration files (default: migrations)
OPERATION_TIMEOUT_SECONDS=# timeout in seconds → time.Duration (default: 30)
REDIS_DB=# Redis database number (default: 1)
REDIS_PASSWORD=# Redis password
REDIS_URL=# Redis URL (fallback for REDIS_ADDRESS + REDIS_PASSWORD)
RESOURCE_GC_THRESHOLD_MB=# GC threshold in MB (default: 400)
RESOURCE_MAX_GOROUTINES=# max goroutines (default: 1000)
RESOURCE_MAX_MEMORY_MB=# max memory in MB (default: 500)
RESPONSE_CACHE_TTL=# response cache TTL in seconds (default: 30)
STATS_COLLECTION_WORKERS=# (default: 2)
TELEGRAM_API_HASH=# for local telegram-bot-api server
TELEGRAM_API_ID=# for local telegram-bot-api server
USE_WEBHOOKS=# enable webhook mode (default: false)
WEBHOOK_DOMAIN=# required if USE_WEBHOOKS=true
WEBHOOK_PORT=# deprecated, use HTTP_PORT (default: 8081)
WEBHOOK_SECRET=# required if USE_WEBHOOKS=true
WORKING_MODE=# (default: worker)
```