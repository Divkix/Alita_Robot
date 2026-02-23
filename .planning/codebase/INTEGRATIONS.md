# External Integrations

**Analysis Date:** 2026-02-23

## APIs & External Services

**Telegram Bot API:**
- Service: Telegram Bot API — core bot communication (send/receive messages, manage chats, handle callbacks)
  - SDK/Client: `github.com/PaulSonOfLars/gotgbot/v2 v2.0.0-rc.33`
  - Auth: `BOT_TOKEN` env var
  - Default endpoint: `https://api.telegram.org`
  - Custom endpoint: `API_SERVER` env var (supports self-hosted local Bot API server)
  - HTTP transport: pooled with configurable `HTTP_MAX_IDLE_CONNS` (default 100) and `HTTP_MAX_IDLE_CONNS_PER_HOST` (default 50), HTTP/2 enabled

**Cloudflare Tunnel (optional):**
- Service: Cloudflare Tunnel — provides HTTPS termination for webhook mode without exposing server directly
  - Auth: `CLOUDFLARE_TUNNEL_TOKEN` env var
  - Used in: `docker-compose.yml` when `USE_WEBHOOKS=true`

## Data Storage

**Databases:**
- Type: PostgreSQL (primary data store for all bot state)
  - Connection: `DATABASE_URL` env var (format: `postgres://user:pass@host:port/db?sslmode=disable`)
  - Client: GORM v1.31.1 with pgx/v5 driver (`gorm.io/driver/postgres v1.6.0`)
  - Connection pool: configurable via `DB_MAX_IDLE_CONNS` (default 50), `DB_MAX_OPEN_CONNS` (default 200), `DB_CONN_MAX_LIFETIME_MIN` (default 240), `DB_CONN_MAX_IDLE_TIME_MIN` (default 60)
  - Schema managed via timestamped SQL migrations in `migrations/` directory
  - Migration tracking: `schema_migrations` table
  - DB operations: `alita/db/db.go` (models + connection), `alita/db/*_db.go` (domain operations)

**File Storage:**
- Local filesystem only — CAPTCHA images generated in-memory via `github.com/mojocn/base64Captcha`, sent as base64 directly to Telegram; no external file storage

**Caching:**
- Service: Redis (sole caching layer — no local/in-memory cache)
  - Connection: `REDIS_ADDRESS` env var (format: `host:port`) OR `REDIS_URL` (Heroku/Railway format `redis://user:pass@host:port`)
  - Auth: `REDIS_PASSWORD` env var
  - DB: `REDIS_DB` env var (default: 1)
  - Client: `github.com/redis/go-redis/v9 v9.18.0` (direct) + `github.com/eko/gocache/store/redis/v4 v4.2.6` (abstraction layer)
  - Key format: `alita:{module}:{identifier}` (e.g., `alita:adminCache:123`)
  - Traced operations via: `TracedGet()`, `TracedSet()`, `TracedDelete()` in `alita/utils/cache/`
  - Stampede protection: `singleflight` in `alita/db/cache_helpers.go`
  - Startup flush: `ClearCacheOnStartup` defaults to `true` (`CLEAR_CACHE_ON_STARTUP` env var)

## Authentication & Identity

**Auth Provider:**
- Custom — no external auth provider
  - Bot owner identified by `OWNER_ID` env var (Telegram user ID)
  - Admin permissions resolved via Telegram API (`GetChatAdministrators`) and cached in Redis for 30 minutes (`alita/utils/cache/adminCache.go`)
  - Permission system: `alita/utils/chat_status/` package with `RequireBotAdmin()`, `RequireUserAdmin()`, `RequireUserOwner()`, etc.
  - Anonymous admin detection with inline keyboard fallback

## Monitoring & Observability

**Distributed Tracing:**
- Service: OpenTelemetry (OTLP over gRPC or stdout/console)
  - SDK: `go.opentelemetry.io/otel v1.40.0`
  - Export to OTLP backend: `OTEL_EXPORTER_OTLP_ENDPOINT` env var (format: `host:port`)
  - Console export: `OTEL_EXPORTER_CONSOLE=true` env var (debug only)
  - TLS: `OTEL_EXPORTER_OTLP_INSECURE=true` to disable
  - Sample rate: `OTEL_TRACES_SAMPLE_RATE` env var (0.0–1.0, default: 1.0)
  - Service name: `OTEL_SERVICE_NAME` env var (default: `alita_robot`)
  - Implementation: `alita/utils/tracing/tracing.go`

**Metrics:**
- Service: Prometheus (self-hosted scrape endpoint)
  - Client: `github.com/prometheus/client_golang v1.23.2`
  - Endpoint: `GET /metrics` on `HTTP_PORT` (default 8080)
  - Metrics defined in: `alita/metrics/prometheus.go`
  - Tracked: commands processed, messages processed, DB query durations, cache hit/miss rates, active users/chats, errors, response times, goroutine count, memory usage

**Logs:**
- Structured JSON via `github.com/sirupsen/logrus v1.9.4`
- Output: stdout (captured by container runtime)
- Level: `DEBUG` when `DEBUG=true`, else `INFO`
- Stack traces: only in debug mode (`log.SetReportCaller(cfg.Debug)`)
- No external log aggregation integrated — logs consumed by deployment platform

**Profiling:**
- pprof endpoints: `GET /debug/pprof/*` — enabled only when `ENABLE_PPROF=true` (dev/staging only)

## CI/CD & Deployment

**Hosting:**
- Bot: Docker container on Dokploy (self-hosted) or any Docker-compatible host
- Container registry: `ghcr.io/divkix/alita_robot` (GitHub Container Registry)
- Docs site: Cloudflare Workers (deployed via Wrangler v4)

**CI Pipeline:**
- Service: GitHub Actions (`.github/workflows/ci.yml`, `.github/workflows/release.yml`, `.github/workflows/docs.yml`)
- CI jobs: security scan (gosec → SARIF upload to CodeQL), govulncheck, golangci-lint v2.9.0, go build, go test (with PostgreSQL service container), Docker build (amd64 only), Docker publish (amd64+arm64 on main push)
- Release: GoReleaser v2.13.0 on git tag push — builds darwin/linux/windows + amd64/arm64, publishes to GitHub Releases + GHCR
- Post-release: Trivy container scan (`aquasecurity/trivy-action@0.34.1`)
- Build provenance: `actions/attest-build-provenance@v3` on releases
- Coverage threshold: 15% minimum enforced in CI

**Security Scanning:**
- gosec (SARIF → GitHub Code Scanning) — every CI run and release
- govulncheck — Go vulnerability database check
- Trivy — post-release container vulnerability scan (CRITICAL+HIGH)

## Webhooks & Callbacks

**Incoming (Telegram → Bot):**
- Webhook endpoint: `POST /webhook/{token}` on `HTTP_PORT` (default 8080)
- Enabled when: `USE_WEBHOOKS=true`
- Domain: `WEBHOOK_DOMAIN` env var (e.g., `https://your-bot-domain.com`)
- Secret validation: `WEBHOOK_SECRET` env var (required when webhooks enabled)
- Request body size limit: 10MB (DoS protection in `alita/utils/httpserver/server.go`)
- Handled by: `alita/utils/httpserver/server.go`

**Outgoing (Bot → Telegram):**
- All Telegram API calls via gotgbot HTTP client to `https://api.telegram.org` (or `API_SERVER`)
- No other outgoing webhook destinations

**Health Check:**
- `GET /health` returns JSON with status, version, uptime, and liveness checks for database and Redis
- Used by Docker healthcheck: `./alita_robot --health` (polls `http://localhost:{HTTP_PORT}/health`)

## Environment Configuration

**Required env vars:**
- `BOT_TOKEN` — Telegram bot token from @BotFather
- `OWNER_ID` — Telegram user ID of the bot owner
- `MESSAGE_DUMP` — Telegram chat ID for logging/dumping messages
- `DATABASE_URL` — PostgreSQL connection string
- `REDIS_ADDRESS` or `REDIS_URL` — Redis connection

**Secrets location:**
- `.env` file (local dev, not committed — `.env` is in `.gitignore`)
- Dokploy UI environment variables (production Docker deployment)
- GitHub Actions secrets: `CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID` (docs deployment), `GITHUB_TOKEN` (auto-provided for GHCR + releases)

---

*Integration audit: 2026-02-23*
