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
| **Recommended range** | 1-24 (not enforced) |

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
| **Recommended range** | 1-365 (not enforced) |

## 📂 AI spam filter configuration

### `AISPAM_DAILY_LIMIT`

Most AI spam checks (Jev calls) the bot makes per UTC day, across every chat.
A check past the limit is not made: the message stays unchecked and nothing is
deleted, as for a check shed under load, until the next UTC day. The count is
kept in Redis (`alita:aispam:daily:<YYYY-MM-DD>`), so replicas sharing Redis
share it; without Redis, or while a Redis call fails, each replica counts on
its own, and the bot as a whole can make up to the limit times its replica
count. A retried request counts as one check. Unset means no cap; use
`ENABLE_AISPAM=false` to stop checks altogether.

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Default** | unset (no cap) |
| **Validation** | integer, at least 1 (a non-integer, `0` or a negative value fails startup) |

### `ENABLE_AISPAM`

Global kill switch for the AI spam filter. When `false`, no chat runs AI spam
checks regardless of its per-chat setting. The filter is inert without
`TYPESAFE_API_KEY` either way.

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |
| **Default** | `true` |

### `TYPESAFE_API_KEY`

TypeSafe API key used by every AI spam check. One credential for the whole bot;
chats cannot bring their own. Leaving it empty disables the feature and makes
`/aispam on` refuse.

| Property | Value |
|----------|-------|
| **Type** | `string` |
| **Required** | No |

## 📂 Spam database configuration

### `ENABLE_SPAMDB`

Operator kill switch for the spam database join check (`/spamdb`). When
`false`, no chat sends new members' user IDs to CAS or lols.bot regardless of
its per-chat settings, no shared reputation lookup runs, and `/spamdb on`
refuses. The check is off in every chat until an admin turns it on; CAS is its
default database and lols.bot and the bot's own Shared bans network are opt-in
per chat.

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |
| **Default** | `true` |

### `CAS_API_KEY`

Optional [CAS](https://cas.chat/api) API key, sent as `Authorization: Bearer`.
CAS still accepts requests without a key but has announced that keys will
become required. The key is redacted from logs.

| Property | Value |
|----------|-------|
| **Type** | `string` |
| **Required** | No |

## 📂 Shared reputation configuration

### `ENABLE_REPUTATION`

Operator kill switch for the shared reputation lookup. When `false`, the shared
source is unavailable and a chat cannot enable it. The lookup lists a user that
at least `REPUTATION_MIN_CHATS` distinct chats banned or kicked inside the
rolling window, clears the signal when a human unban in that window reverses a
ban or kick the same chat recorded earlier in the window, and treats a failed
lookup as not listed so a join is never blocked. An unban in a chat that never
removed the user changes nothing. Only user IDs and moderation actions are
aggregated; no message content is stored. The feature is off by default and
every chat stays opted out until its spam database settings enable the shared
source with `/spamdb db shared on`; `/spamdb db shared off` opts out. A chat
that opts out stops being looked up at once, while its older events stay part
of the aggregate until they age out (180 days). See
[Spamdb commands](/commands/spamdb/#shared-bans) for the policy.

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |
| **Default** | `false` |

### `REPUTATION_MIN_CHATS`

Number of distinct chats that must have banned or kicked a user inside the
window before the shared source lists them. Ten bans in one chat still count as
one chat.

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Default** | `3` |
| **Validation** | integer, at least 2 (a non-integer or a lower value fails startup) |

### `REPUTATION_WINDOW_DAYS`

Rolling window, in days, the shared reputation count looks back. A human unban
inside the window clears the signal only when the same chat banned or kicked
the user earlier in the window. The maximum is `180`, the moderation event
retention: older events are deleted, so a longer window could never be observed
in full.

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Default** | `30` |
| **Validation** | integer, 1-180 (a non-integer or an out-of-range value fails startup) |

## 📂 Weekly digest configuration

### `ENABLE_DIGEST`

Operator kill switch for the weekly digest and the `/digest` command. When
`false`, no digest is sent and `/digest` refuses; the daily counters keep
recording because the dashboard overview uses the same data, and each chat's
digest settings are left untouched. With it on, a digest still goes only to
chats whose admins ran `/digest on`.

The scheduler posts the previous week's totals, with the change from the week
before, after Monday 00:00 UTC. A chat's digest goes to its log channel by
default and to the group when it has none; admins choose with `/digest here`
and `/digest log`.

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |
| **Default** | `true` |

### `DIGEST_INTERVAL`

How often the digest scheduler looks for chats due a weekly digest, as a Go
duration (`30m`, `1h`, `6h`). It is a test and staging knob; leave it unset in
production. A value that is not a positive duration fails startup.

| Property | Value |
|----------|-------|
| **Type** | `duration` |
| **Required** | No |
| **Default** | `1h` |

## 📂 Bot settings

### `MESSAGE_DUMP` (Required)

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | Yes |
| **Validation** | required, non-zero (log channel ID, starts with -100) |

### `OWNER_ID` (Required)

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | Yes |
| **Validation** | required, non-zero |

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
| **Recommended range** | 10-1000 (not enforced) |

### `HTTP_MAX_IDLE_CONNS_PER_HOST`

HTTP connections per host

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Default** | `50` |
| **Recommended range** | 5-500 (not enforced) |

## 📂 Profiling configuration

### `ENABLE_PPROF`

Enable pprof endpoints for performance profiling (development only)

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |
| **Default** | `false` |

## 📂 OpenTelemetry configuration

### `OTEL_EXPORTER_OTLP_ENDPOINT`

OTLP gRPC endpoint (e.g., `localhost:4317`). When set, the bot sends traces, logs and its Prometheus metrics to this endpoint. Each update's span carries `update.type`, `chat.id`, `chat.type`, `user.id`, `bot.command` (the first word of a message starting with `/`) and `callback.namespace`; a handler error sets the span to Error with `error.type` and `error.message` (expected Telegram errors only set `error.expected=true`). Log records are scrubbed by the same redaction as stdout logs.

| Property | Value |
|----------|-------|
| **Type** | `string` |
| **Required** | No |

### `OTEL_EXPORTER_CONSOLE`

Enable console exporter for debugging traces (outputs to stderr). Only the exact lowercase value `true` enables it; unlike other boolean variables, `1`, `yes` and `TRUE` are ignored.

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |
| **Default** | `false` |

### `OTEL_EXPORTER_OTLP_INSECURE`

Use insecure OTLP gRPC connection (no TLS). Only used when `OTEL_EXPORTER_OTLP_ENDPOINT` is set. Only `true` (any letter case) enables it; unlike other boolean variables, `1` and `yes` are ignored.

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |
| **Default** | `false` |

### `OTEL_EXPORTER_OTLP_HEADERS`

Headers sent with every OTLP export, as `key1=value1,key2=value2` (for ClickStack/HyperDX: `authorization=<ingestion API key>`). Values are redacted from logs.

| Property | Value |
|----------|-------|
| **Type** | `string` |
| **Required** | No |

### `OTEL_LOGS_EXPORTER`

Set to `none` to stop exporting logs over OTLP while traces and metrics keep flowing. Only used when `OTEL_EXPORTER_OTLP_ENDPOINT` is set.

| Property | Value |
|----------|-------|
| **Type** | `string` |
| **Required** | No |
| **Default** | `otlp` |

### `OTEL_METRICS_EXPORTER`

Set to `none` to stop exporting the Prometheus registry over OTLP. Only used when `OTEL_EXPORTER_OTLP_ENDPOINT` is set; `/metrics` is unaffected.

| Property | Value |
|----------|-------|
| **Type** | `string` |
| **Required** | No |
| **Default** | `otlp` |

### `OTEL_METRIC_EXPORT_INTERVAL`

Milliseconds between OTLP metric exports.

| Property | Value |
|----------|-------|
| **Type** | `int` |
| **Required** | No |
| **Default** | `60000` |

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

## 📂 Admin panel configuration

The Telegram Mini App admin panel is served by the bot's own HTTP server under
`/panel`. It is off by default; with it disabled those paths return 404.

### `PANEL_ENABLED`

Enable the admin panel. Requires `PANEL_PUBLIC_URL`.

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |
| **Default** | `false` |

### `PANEL_PUBLIC_URL`

Public HTTPS URL Telegram opens for the panel, for example
`https://bot.example.com/panel`. The bot serves the panel under `/panel` and the
shell calls `/panel/api/v1/*`, so the path must reach the bot unchanged. Must be
`https://`; `http://` is accepted only when `DEBUG=true`. A trailing slash is
trimmed. Required when `PANEL_ENABLED=true`.

| Property | Value |
|----------|-------|
| **Type** | `string` |
| **Required** | Conditional |

### `PANEL_MINIAPP_SHORTNAME`

Optional BotFather Mini App short name. When set, group buttons open
`https://t.me/<bot>/<short_name>?startapp=chat_<chat_id>` directly; otherwise
they use the `/start panel_<chat_id>` PM deep link.

| Property | Value |
|----------|-------|
| **Type** | `string` |
| **Required** | No |

### `PANEL_SESSION_MAX_AGE`

How old a Mini App `initData` payload may be before the panel rejects it. Go
duration syntax such as `30m` or `24h`; an invalid value fails startup.

| Property | Value |
|----------|-------|
| **Type** | `duration` |
| **Required** | No |
| **Default** | `24h` |

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
| **Default** | `1` with `REDIS_ADDRESS`; with `REDIS_URL` alone, the URL's path database (`0` if the URL has no path) |

In `REDIS_ADDRESS` mode, set `REDIS_DB=0` explicitly to select database zero. In `REDIS_URL` mode, a set `REDIS_DB` overrides the URL path; when it is unset, the path decides, and a URL with no path uses database 0.

### `REDIS_PASSWORD`

| Property | Value |
|----------|-------|
| **Type** | `string` |
| **Required** | No |

### `REDIS_TLS_INSECURE_SKIP_VERIFY`

Skip TLS certificate verification when `REDIS_URL` is a `rediss://` URL. Set it
to `true` on Heroku: Heroku Redis serves TLS with a self-signed certificate, so
verification fails and the bot cannot start. The Heroku button (`app.json`) sets
it for you. It has no effect with a `redis://` URL or with `REDIS_ADDRESS`. The
bot logs a warning at startup when it skips verification.

Leave it unset elsewhere. With verification off, the connection is still
encrypted, but the bot does not check who it is talking to, so anyone who can
intercept traffic to Redis can impersonate it.

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |
| **Default** | `false` |

## 📂 Local cache layer

Read-through lookups keep a short-lived copy in process memory in front of
Redis. See [Caching Architecture](/architecture/caching/#local-layer).

### `CACHE_LOCAL_TTL`

Seconds a replica serves a cached value from memory before reading Redis
again. Other replicas can see a change up to this long after a write; the
replica that wrote sees it immediately. `0` disables the local layer. An
unparsable value falls back to the default.

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Default** | `10` |
| **Validation** | min=0,max=300 |

### `CACHE_LOCAL_MAX_ENTRIES`

Maximum number of entries in the local layer; the least recently used entry is
evicted first.

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Default** | `50000` |
| **Validation** | min=100,max=1000000 |

### `DISABLE_CACHE`

Bypasses the read-through cache: every cached lookup reads PostgreSQL directly, the in-process layer is turned off, and writes skip Redis cache deletes. The bot also starts when Redis is unreachable or misconfigured; it logs a warning and runs degraded (Redis-backed state such as antiraid does nothing). Intended for load-testing PostgreSQL, not normal operation.

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |
| **Default** | `false` |

## 📂 Resource monitoring limits

### `RESOURCE_GC_THRESHOLD_MB`

Memory threshold for triggering GC

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Default** | `400` |
| **Recommended range** | 100-5000 (not enforced) |

### `RESOURCE_MAX_GOROUTINES`

Maximum goroutines before triggering cleanup

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Default** | `1000` |
| **Recommended range** | 100-10000 (not enforced) |

### `RESOURCE_MAX_MEMORY_MB`

Maximum memory usage in MB

| Property | Value |
|----------|-------|
| **Type** | `integer` |
| **Required** | No |
| **Default** | `500` |
| **Recommended range** | 100-10000 (not enforced) |

## 📂 Safety and performance limits

### `CLEAR_CACHE_ON_STARTUP`

Whether to clear disposable `alita:cache:*` entries on bot startup. Active anti-raid state and pending confirmations are preserved.

| Property | Value |
|----------|-------|
| **Type** | `boolean` |
| **Required** | No |
| **Default** | `false` |

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
AISPAM_DAILY_LIMIT=# most AI spam checks per UTC day (default: unset, no cap)
API_SERVER=# custom API server URL (default: https://api.telegram.org)
AUTO_MIGRATE=# enable automatic database migrations (default: false)
AUTO_MIGRATE_SILENT_FAIL=# continue even if migrations fail (default: false)
CAS_API_KEY=# optional CAS API key for the /spamdb join check
CLEAR_CACHE_ON_STARTUP=false # clear disposable cached data on startup
DB_CONN_MAX_IDLE_TIME_MIN=# max idle time in min (default: 60)
DB_CONN_MAX_LIFETIME_MIN=# max lifetime in min (default: 240)
DB_MAX_IDLE_CONNS=# (default: 50)
DB_MAX_OPEN_CONNS=# (default: 200)
DEBUG=# enable debug logging (default: false)
DISABLE_CACHE=# bypass the cache; bot starts without Redis, degraded (default: false)
DIGEST_INTERVAL=# how often the digest scheduler checks for due chats, Go duration (default: 1h)
DISPATCHER_MAX_ROUTINES=# (default: 200)
DROP_PENDING_UPDATES=# (default: false)
ENABLE_AISPAM=# global AI spam filter kill switch (default: true)
ENABLE_AUTO_CLEANUP=# (default: true)
ENABLE_BACKGROUND_STATS=# (default: true in prod, false in debug)
ENABLE_DB_MONITORING=# (default: false)
ENABLE_DIGEST=# operator kill switch for the weekly digest and /digest (default: true)
ENABLE_PERFORMANCE_MONITORING=# (default: true in prod, false in debug)
ENABLE_PPROF=# enable pprof endpoints (default: false)
ENABLE_REPUTATION=# operator kill switch for shared reputation lookups (default: false)
ENABLE_SPAMDB=# operator kill switch for the /spamdb join check (default: true)
ENABLED_LOCALES=# comma-separated language codes (default: en)
HTTP_MAX_IDLE_CONNS=# (default: 100)
HTTP_MAX_IDLE_CONNS_PER_HOST=# (default: 50)
HTTP_PORT=# unified HTTP server port (PORT fallback, then 8080)
INACTIVITY_THRESHOLD_DAYS=# days before marking inactive (default: 30)
METRICS_AUTH_TOKEN=# protects /metrics and /db_metrics
MIGRATIONS_PATH=# path to migration files (default: migrations)
OTEL_EXPORTER_CONSOLE=# enable console trace exporter (default: false)
OTEL_EXPORTER_OTLP_ENDPOINT=# OTLP gRPC endpoint for traces, logs and metrics (e.g., localhost:4317)
OTEL_EXPORTER_OTLP_HEADERS=# OTLP headers, e.g. authorization=<ingestion key>
OTEL_EXPORTER_OTLP_INSECURE=# use insecure OTLP gRPC (default: false)
OTEL_SERVICE_NAME=# service name for traces (default: alita_robot)
OTEL_TRACES_SAMPLE_RATE=# trace sampling rate 0.0-1.0 (default: 1.0)
OTEL_LOGS_EXPORTER=# none disables OTLP log export
OTEL_METRICS_EXPORTER=# none disables OTLP metric export
OTEL_METRIC_EXPORT_INTERVAL=# ms between OTLP metric exports (default: 60000)
PANEL_ENABLED=# enable the Telegram Mini App admin panel (default: false)
PANEL_MINIAPP_SHORTNAME=# optional BotFather Mini App short name for direct group links
PANEL_PUBLIC_URL=# public HTTPS URL Telegram opens for the panel, e.g. https://host/panel; required when PANEL_ENABLED=true
PANEL_SESSION_MAX_AGE=# maximum initData age, Go duration (default: 24h)
REDIS_DB=# Redis database number (default: 1 with REDIS_ADDRESS; REDIS_URL uses its path, else 0)
REDIS_PASSWORD=# Redis password
REDIS_URL=# Redis URL (fallback for REDIS_ADDRESS + REDIS_PASSWORD)
REPUTATION_MIN_CHATS=# distinct chats needed to list a user (default: 3, minimum 2)
REPUTATION_WINDOW_DAYS=# rolling window in days (default: 30, 1-180)
RESOURCE_GC_THRESHOLD_MB=# GC threshold in MB (default: 400)
RESOURCE_MAX_GOROUTINES=# max goroutines (default: 1000)
RESOURCE_MAX_MEMORY_MB=# max memory in MB (default: 500)
TYPESAFE_API_KEY=# TypeSafe API key for the AI spam filter
USE_WEBHOOKS=# enable webhook mode (default: false)
WEBHOOK_DOMAIN=# required if USE_WEBHOOKS=true
WEBHOOK_SECRET=# required if USE_WEBHOOKS=true
```
