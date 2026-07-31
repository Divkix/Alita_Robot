---
title: Environment Variables
description: Configuration reference for all environment variables
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

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

Comma-separated list of language codes shown in the language picker (e.g.,
`en,es,fr,hi`). All embedded locale files are loaded regardless of this setting.

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

### `DEBUG`

Enables debug logging and disables automatic performance monitoring.

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |
| **Default** | `false` |

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

## 📂 Profiling configuration

### `ENABLE_PPROF`

Enable pprof endpoints for performance profiling (development only)

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |
| **Default** | `false` |

## 📂 OpenTelemetry tracing configuration

### `OTEL_EXPORTER_OTLP_ENDPOINT`

OTLP gRPC endpoint for trace export (e.g., `localhost:4317`). When set, the bot sends traces to this endpoint.

| Property | Value |
|----------|-------|
| **Type** | `string` |
| **Required** | No |

### `OTEL_EXPORTER_CONSOLE`

Enable console exporter for debugging traces (outputs to stderr).

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |
| **Default** | `false` |

### `OTEL_EXPORTER_OTLP_INSECURE`

Use insecure OTLP gRPC connection (no TLS). Only used when `OTEL_EXPORTER_OTLP_ENDPOINT` is set.

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |
| **Default** | `false` |

### `OTEL_SERVICE_NAME`

Service name for trace identification.

| Property | Value |
|----------|-------|
| **Type** | `string` |
| **Required** | No |
| **Default** | `alita_robot` |

### `OTEL_TRACES_SAMPLE_RATE`

Trace sampling rate from 0.0 (no traces) to 1.0 (all traces).

| Property | Value |
|----------|-------|
| **Type** | `float` |
| **Required** | No |
| **Default** | `1.0` |
| **Validation** | min=0.0,max=1.0 |

## 📂 HTTP Server configuration

### `HTTP_PORT`

Unified HTTP server for health checks, metrics, and webhooks. If `HTTP_PORT` is
unset, the bot uses `PORT` (as injected by Railway) before falling back to 8080.

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Default** | `8080` |
| **Validation** | min=1,max=65535 |

### `METRICS_AUTH_TOKEN`

Bearer token required by `/metrics` and `/db_metrics`. Leaving it empty exposes
those endpoints without authentication and logs a warning.

| Property | Value |
|----------|-------|
| **Type** | `string` |
| **Required** | No |

## 📂 Redis configuration

### `REDIS_ADDRESS`

Redis host and port. It takes precedence over `REDIS_URL`; if neither is set,
the bot connects to `localhost:6379`.

| Property | Value |
|----------|-------|
| **Type** | `string` |
| **Required** | No |
| **Default** | `localhost:6379` |

### `REDIS_URL`

Standard Redis URL (`redis://user:password@host:port/db` or `rediss://...`).
When `REDIS_ADDRESS` is unset, the full URL is used so usernames, TLS, and the
path database are preserved. Explicit `REDIS_PASSWORD` and `REDIS_DB` values
override their URL components.

| Property | Value |
|----------|-------|
| **Type** | `string` |
| **Required** | No |

### `REDIS_DB`

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Default** | `1` |

Set `REDIS_DB=0` explicitly to select database zero.

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

### `WEBHOOK_SECRET`

Required when `USE_WEBHOOKS=true`.

| Property | Value |
|----------|-------|
| **Type** | `string` |
| **Required** | Conditional |

## Quick Reference

### Required Variables

```bash
BOT_TOKEN=
DATABASE_URL=
MESSAGE_DUMP=
OWNER_ID=
```

### Optional Variables

```bash
ACTIVITY_CHECK_INTERVAL=# hours between activity checks (default: 1)
API_SERVER=# custom API server URL (default: https://api.telegram.org)
AUTO_MIGRATE=# enable automatic database migrations (default: false)
AUTO_MIGRATE_SILENT_FAIL=# continue even if migrations fail (default: false)
CLEAR_CACHE_ON_STARTUP=# clear all caches on startup (default: true)
DB_CONN_MAX_IDLE_TIME_MIN=# max idle time in min (default: 60)
DB_CONN_MAX_LIFETIME_MIN=# max lifetime in min (default: 240)
DB_MAX_IDLE_CONNS=# (default: 50)
DB_MAX_OPEN_CONNS=# (default: 200)
DEBUG=# enable debug logging (default: false)
DISPATCHER_MAX_ROUTINES=# (default: 200)
DROP_PENDING_UPDATES=# (default: false)
ENABLE_AUTO_CLEANUP=# (default: true)
ENABLE_BACKGROUND_STATS=# (default: true in prod, false in debug)
ENABLE_DB_MONITORING=# (default: false)
ENABLE_PERFORMANCE_MONITORING=# (default: true in prod, false in debug)
ENABLE_PPROF=# enable pprof endpoints (default: false)
ENABLED_LOCALES=# comma-separated language codes (default: en)
HTTP_MAX_IDLE_CONNS=# (default: 100)
HTTP_MAX_IDLE_CONNS_PER_HOST=# (default: 50)
HTTP_PORT=# unified HTTP server port (PORT fallback, then 8080)
INACTIVITY_THRESHOLD_DAYS=# days before marking inactive (default: 30)
METRICS_AUTH_TOKEN=# protects /metrics and /db_metrics
MIGRATIONS_PATH=# path to migration files (default: migrations)
OTEL_EXPORTER_CONSOLE=# enable console trace exporter (default: false)
OTEL_EXPORTER_OTLP_ENDPOINT=# OTLP gRPC endpoint (e.g., localhost:4317)
OTEL_EXPORTER_OTLP_INSECURE=# use insecure OTLP gRPC (default: false)
OTEL_SERVICE_NAME=# service name for traces (default: alita_robot)
OTEL_TRACES_SAMPLE_RATE=# trace sampling rate 0.0-1.0 (default: 1.0)
REDIS_DB=# Redis database number (default: 1)
REDIS_PASSWORD=# Redis password
REDIS_URL=# Redis URL (fallback for REDIS_ADDRESS + REDIS_PASSWORD)
RESOURCE_GC_THRESHOLD_MB=# GC threshold in MB (default: 400)
RESOURCE_MAX_GOROUTINES=# max goroutines (default: 1000)
RESOURCE_MAX_MEMORY_MB=# max memory in MB (default: 500)
USE_WEBHOOKS=# enable webhook mode (default: false)
WEBHOOK_DOMAIN=# required if USE_WEBHOOKS=true
WEBHOOK_SECRET=# required if USE_WEBHOOKS=true
```
