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
| **Validation** | min=1,max=24 |

### `ENABLE_AUTO_CLEANUP`

Whether to automatically mark inactive chats

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |

### `INACTIVITY_THRESHOLD_DAYS`

Days before marking a chat as inactive

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
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

### `ALLOWED_UPDATES`

| Property | Value |
|----------|-------|
| **Type** | `string[]` |
| **Required** | No |

### `DROP_PENDING_UPDATES`

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |

### `VALID_LANG_CODES`

| Property | Value |
|----------|-------|
| **Type** | `string[]` |
| **Required** | No |

## 📂 Core configuration

### `BOT_TOKEN` (Required)

| Property | Value |
|----------|-------|
| **Type** | `string` |
| **Required** | Yes |
| **Validation** | required |

### `API_SERVER`

| Property | Value |
|----------|-------|
| **Type** | `string` |
| **Required** | No |

### `BOT_VERSION`

| Property | Value |
|----------|-------|
| **Type** | `string` |
| **Required** | No |

### `DEBUG`

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |

### `WORKING_MODE`

| Property | Value |
|----------|-------|
| **Type** | `string` |
| **Required** | No |

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
| **Validation** | min=1,max=60 |

### `DB_CONN_MAX_LIFETIME_MIN`

Max lifetime in minutes

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Validation** | min=1,max=1440 |

### `DB_MAX_IDLE_CONNS`

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Validation** | min=1,max=100 |

### `DB_MAX_OPEN_CONNS`

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Validation** | min=1,max=1000 |

## 📂 Database migration settings

### `AUTO_MIGRATE`

Enable automatic database migrations on startup

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |

### `AUTO_MIGRATE_SILENT_FAIL`

Continue running even if migrations fail

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |

### `MIGRATIONS_PATH`

Path to migration files (defaults to migrations)

| Property | Value |
|----------|-------|
| **Type** | `string` |
| **Required** | No |

## 📂 Performance optimization settings

### `BATCH_REQUEST_TIMEOUT_MS`

Batch request timeout in milliseconds

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Validation** | min=10,max=5000 |

### `ENABLE_ASYNC_PROCESSING`

Enable async processing for non-critical operations

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |

### `ENABLE_BATCH_REQUESTS`

Enable batch API requests

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |

### `ENABLE_CACHE_PREWARMING`

Enable cache prewarming on startup

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |

### `ENABLE_HTTP_CONNECTION_POOLING`

Enable HTTP connection pooling

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |

### `ENABLE_QUERY_PREFETCHING`

Enable query batching and prefetching

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |

### `ENABLE_RESPONSE_CACHING`

Enable response caching

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |

### `HTTP_MAX_IDLE_CONNS`

HTTP connection pool size

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Validation** | min=10,max=1000 |

### `HTTP_MAX_IDLE_CONNS_PER_HOST`

HTTP connections per host

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Validation** | min=5,max=500 |

### `RESPONSE_CACHE_TTL`

Response cache TTL in seconds

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Validation** | min=1,max=3600 |

## 📂 Profiling configuration

### `ENABLE_PPROF`

Enable pprof endpoints for performance profiling (development only)

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |

## 📂 Redis configuration

### `REDIS_ADDRESS` (Required)

| Property | Value |
|----------|-------|
| **Type** | `string` |
| **Required** | Yes |
| **Validation** | required |

### `HTTP_PORT`

HTTP Server configuration (unified server for health, metrics, webhook)

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Validation** | min=1,max=65535 |

### `REDIS_DB`

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |

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
| **Validation** | min=100,max=5000 |

### `RESOURCE_MAX_GOROUTINES`

Maximum goroutines before triggering cleanup

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Validation** | min=100,max=10000 |

### `RESOURCE_MAX_MEMORY_MB`

Maximum memory usage in MB

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Validation** | min=100,max=10000 |

## 📂 Safety and performance limits

### `CLEAR_CACHE_ON_STARTUP`

Whether to clear all caches on bot startup

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |

### `DISPATCHER_MAX_ROUTINES`

Max concurrent goroutines for dispatcher

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Validation** | min=1,max=1000 |

### `ENABLE_BACKGROUND_STATS`

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |

### `ENABLE_PERFORMANCE_MONITORING`

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |

### `MAX_CONCURRENT_OPERATIONS`

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Validation** | min=1,max=1000 |

### `OPERATION_TIMEOUT`

Computed from OperationTimeoutSeconds

| Property | Value |
|----------|-------|
| **Type** | `duration` |
| **Required** | No |

### `OPERATION_TIMEOUT_SECONDS`

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Validation** | min=1,max=300 |

## 📂 Webhook configuration

### `USE_WEBHOOKS`

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |

### `WEBHOOK_DOMAIN`

| Property | Value |
|----------|-------|
| **Type** | `string` |
| **Required** | No |

### `WEBHOOK_PORT`

Deprecated: use HTTPPort instead

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Validation** | min=1,max=65535 |

### `WEBHOOK_SECRET`

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
| **Validation** | min=1,max=20 |

### `CACHE_WORKERS`

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Validation** | min=1,max=20 |

### `CHAT_VALIDATION_WORKERS`

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Validation** | min=1,max=100 |

### `DATABASE_WORKERS`

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Validation** | min=1,max=50 |

### `MESSAGE_PIPELINE_WORKERS`

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Validation** | min=1,max=50 |

### `STATS_COLLECTION_WORKERS`

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Validation** | min=1,max=10 |

## Quick Reference

### Required Variables

```bash
BOT_TOKEN=
DATABASE_URL=
MESSAGE_DUMP=
OWNER_ID=
REDIS_ADDRESS=
```

### Optional Variables

```bash
ACTIVITY_CHECK_INTERVAL=# (optional)
ALLOWED_UPDATES=# (optional)
API_SERVER=# (optional)
AUTO_MIGRATE=# (optional)
AUTO_MIGRATE_SILENT_FAIL=# (optional)
BATCH_REQUEST_TIMEOUT_MS=# (optional)
BOT_VERSION=# (optional)
BULK_OPERATION_WORKERS=# (optional)
CACHE_WORKERS=# (optional)
CHAT_VALIDATION_WORKERS=# (optional)
CLEAR_CACHE_ON_STARTUP=# (optional)
DATABASE_WORKERS=# (optional)
DB_CONN_MAX_IDLE_TIME_MIN=# (optional)
DB_CONN_MAX_LIFETIME_MIN=# (optional)
DB_MAX_IDLE_CONNS=# (optional)
DB_MAX_OPEN_CONNS=# (optional)
DEBUG=# (optional)
DISPATCHER_MAX_ROUTINES=# (optional)
DROP_PENDING_UPDATES=# (optional)
ENABLE_ASYNC_PROCESSING=# (optional)
ENABLE_AUTO_CLEANUP=# (optional)
ENABLE_BACKGROUND_STATS=# (optional)
ENABLE_BATCH_REQUESTS=# (optional)
ENABLE_CACHE_PREWARMING=# (optional)
ENABLE_HTTP_CONNECTION_POOLING=# (optional)
ENABLE_PERFORMANCE_MONITORING=# (optional)
ENABLE_PPROF=# (optional)
ENABLE_QUERY_PREFETCHING=# (optional)
ENABLE_RESPONSE_CACHING=# (optional)
HTTP_MAX_IDLE_CONNS=# (optional)
HTTP_MAX_IDLE_CONNS_PER_HOST=# (optional)
HTTP_PORT=# (optional)
INACTIVITY_THRESHOLD_DAYS=# (optional)
MAX_CONCURRENT_OPERATIONS=# (optional)
MESSAGE_PIPELINE_WORKERS=# (optional)
MIGRATIONS_PATH=# (optional)
OPERATION_TIMEOUT=# (optional)
OPERATION_TIMEOUT_SECONDS=# (optional)
REDIS_DB=# (optional)
REDIS_PASSWORD=# (optional)
RESOURCE_GC_THRESHOLD_MB=# (optional)
RESOURCE_MAX_GOROUTINES=# (optional)
RESOURCE_MAX_MEMORY_MB=# (optional)
RESPONSE_CACHE_TTL=# (optional)
STATS_COLLECTION_WORKERS=# (optional)
USE_WEBHOOKS=# (optional)
VALID_LANG_CODES=# (optional)
WEBHOOK_DOMAIN=# (optional)
WEBHOOK_PORT=# (optional)
WEBHOOK_SECRET=# (optional)
WORKING_MODE=# (optional)
```
