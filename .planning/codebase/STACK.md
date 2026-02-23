# Technology Stack

**Analysis Date:** 2026-02-23

## Languages

**Primary:**
- Go 1.25.0 - All bot logic, database layer, HTTP server, modules, utilities
- SQL - Database migrations in `migrations/*.sql`

**Secondary:**
- TypeScript/JavaScript - Documentation site only (`docs/`)
- YAML - Locale/i18n files in `locales/`, config files

## Runtime

**Environment:**
- Go 1.25.0 (specified in `go.mod`)
- CGO disabled for all builds (`CGO_ENABLED=0`)
- No C dependencies — pure Go, statically compiled

**Package Manager:**
- Go modules (`go mod`) — lockfile: `go.sum` (present)
- Bun — docs site only (`docs/bun.lock` present)

## Frameworks

**Core:**
- `github.com/PaulSonOfLars/gotgbot/v2 v2.0.0-rc.33` — Telegram Bot API framework (dispatcher, handlers, types)
- `gorm.io/gorm v1.31.1` — ORM for PostgreSQL
- `gorm.io/driver/postgres v1.6.0` — GORM PostgreSQL driver (uses `jackc/pgx/v5` underneath)

**Caching:**
- `github.com/eko/gocache/lib/v4 v4.2.3` — Cache abstraction layer
- `github.com/eko/gocache/store/redis/v4 v4.2.6` — Redis store backend

**Observability:**
- `go.opentelemetry.io/otel v1.40.0` — OpenTelemetry core
- `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.40.0` — OTLP gRPC trace exporter
- `go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.40.0` — Console trace exporter
- `go.opentelemetry.io/otel/sdk v1.40.0` — OTel SDK
- `github.com/prometheus/client_golang v1.23.2` — Prometheus metrics

**Logging:**
- `github.com/sirupsen/logrus v1.9.4` — Structured JSON logging

**Testing:**
- Go standard `testing` package — no third-party test framework

**Build/Dev:**
- GoReleaser v2.13.0 — cross-platform release builds (config: `.goreleaser.yaml`)
- golangci-lint v2.9.0 — linting (config: `.golangci.yml`)
- `make` — all development commands (`Makefile`)

**Documentation Site:**
- Astro v5.17.3 with Starlight v0.37.6 — docs in `docs/`
- Bun — package manager and script runner for docs

## Key Dependencies

**Critical:**
- `github.com/PaulSonOfLars/gotgbot/v2 v2.0.0-rc.33` — Bot framework; all handlers, dispatcher, Telegram types
- `github.com/redis/go-redis/v9 v9.18.0` — Redis client (used directly and via gocache)
- `gorm.io/gorm v1.31.1` — All database operations
- `github.com/joho/godotenv v1.5.1` — `.env` file loading at startup
- `github.com/spf13/viper v1.21.0` — Configuration management (indirect, used by viper internals)

**Infrastructure:**
- `github.com/cloudflare/ahocorasick v0.0.0-20240916140611-054963ec9396` — Multi-pattern string matching for filters/blacklists (`alita/utils/keyword_matcher/`)
- `github.com/mojocn/base64Captcha v1.3.8` — CAPTCHA image generation (`alita/modules/captcha.go`)
- `github.com/dustin/go-humanize v1.0.1` — Human-readable sizes/numbers in bot responses
- `github.com/google/uuid v1.6.0` — UUID generation for callback codec
- `github.com/PaulSonOfLars/gotg_md2html v0.0.0-20260214100625-69ffd2817536` — Telegram Markdown v2 to HTML conversion for i18n strings
- `golang.org/x/sync v0.19.0` — `singleflight` for cache stampede protection
- `gopkg.in/yaml.v3 v3.0.1` — YAML parsing for locale files
- `golang.org/x/text v0.34.0` — Unicode/text utilities

## Configuration

**Environment:**
- Loaded from `.env` file (optional) via `github.com/joho/godotenv`
- All config in `alita/config/config.go` — single `AppConfig` global struct
- Required vars: `BOT_TOKEN`, `OWNER_ID`, `MESSAGE_DUMP`, `DATABASE_URL`, `REDIS_ADDRESS`
- Reference: `sample.env` documents all available variables with defaults

**Build:**
- `Makefile` — development commands
- `.goreleaser.yaml` — cross-platform release builds (darwin, linux, windows; amd64, arm64)
- `docker/alpine` — production Dockerfile (multi-stage: `golang:alpine` builder → `gcr.io/distroless/static-debian12` runtime)
- `docker/goreleaser` — GoReleaser Docker image build
- `.golangci.yml` — linting config (enables `godox` and `dupl` linters only; threshold 100)

## Platform Requirements

**Development:**
- Go 1.25.0+
- PostgreSQL (any recent version; tests use postgres:16)
- Redis (any recent version)
- `golangci-lint` for `make lint`
- `goreleaser` for `make build`
- Bun for docs development (`make docs-dev`)

**Production:**
- Deployment via Docker Compose (`docker-compose.yml`) targeting Dokploy
- PostgreSQL 18 Alpine (`docker-compose.yml`) or any PostgreSQL-compatible DB
- Redis (latest)
- Images published to `ghcr.io/divkix/alita_robot`
- Runtime image: `gcr.io/distroless/static-debian12` (non-root user 65532)
- Memory limit: 1G bot container, 512M PostgreSQL, 256M Redis
- Supports Cloudflare Tunnel for webhook HTTPS (`CLOUDFLARE_TUNNEL_TOKEN`)
- Docs site deployed to Cloudflare Workers via Wrangler v4

---

*Stack analysis: 2026-02-23*
