---
title: Self-Hosting Overview
description: Host your own instance of Alita Robot.
---

# Self-Hosting Alita Robot

This guide covers everything you need to deploy your own instance of Alita Robot.

## Prerequisites

Before you begin, ensure you have:

| Component | Minimum Version | Purpose |
|-----------|-----------------|---------|
| **PostgreSQL** | 14+ | Primary database |
| **Redis** | 6+ | Caching layer |
| **Docker** (recommended) | 20.10+ | Container deployment |

For building from source:
- **Go** 1.25+

## Deployment Options

### Docker Compose (Recommended)

The easiest way to deploy Alita. Includes PostgreSQL and Redis.

```bash
git clone https://github.com/divkix/Alita_Robot.git
cd Alita_Robot
cp sample.env .env
# Edit .env with your configuration
docker-compose up -d
```

[Full Docker Guide →](/self-hosting/docker)

### Binary Deployment

Download pre-built binaries for your platform:

- Linux (amd64, arm64)
- macOS (amd64, arm64)
- Windows (amd64)

[Binary Installation →](/self-hosting/binary)

### Build from Source

Compile the bot yourself:

```bash
git clone https://github.com/divkix/Alita_Robot.git
cd Alita_Robot
go build -o alita_robot .
```

[Building from Source →](/self-hosting/source)

## Essential Configuration

At minimum, you need these environment variables:

```bash
BOT_TOKEN=your_bot_token_from_botfather
OWNER_ID=your_telegram_user_id
MESSAGE_DUMP=-100xxxxxxxxxx  # Log channel ID
DATABASE_URL=postgres://user:pass@host:5432/db
REDIS_ADDRESS=localhost:6379
```

[Full Environment Reference →](/api-reference/environment)

## Webhook vs Polling

| Mode | Use Case | Setup Complexity |
|------|----------|------------------|
| **Webhook** | Production | Requires HTTPS domain |
| **Polling** | Development | Simple, works anywhere |

[Webhook Setup →](/self-hosting/webhooks)

## Database Setup

Alita uses PostgreSQL with automatic migrations:

```bash
# Enable auto-migration
AUTO_MIGRATE=true

# Or run manually
make psql-migrate
```

[Database Guide →](/self-hosting/database)

## Monitoring

Built-in observability endpoints:

- `GET /health` - Health check with DB/Redis status
- `GET /metrics` - Prometheus metrics

[Monitoring Guide →](/self-hosting/monitoring)

## Next Steps

1. [Docker Deployment](/self-hosting/docker) - Quick start with containers
2. [Environment Variables](/api-reference/environment) - Full configuration reference
3. [Troubleshooting](/self-hosting/troubleshooting) - Common issues and solutions
