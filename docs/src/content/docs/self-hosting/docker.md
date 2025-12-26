---
title: Docker Deployment
description: Deploy Alita Robot using Docker and docker-compose.
---

# Docker Deployment

Docker is the recommended way to deploy Alita Robot. The provided `docker-compose.yml` includes all required services with optimized configurations for production use.

## Prerequisites

- **Docker** 20.10 or higher
- **docker-compose** v2 (comes with Docker Desktop)
- A Telegram bot token from [@BotFather](https://t.me/BotFather)
- Your Telegram user ID (get it from [@userinfobot](https://t.me/userinfobot))

## Quick Start

```bash
# Clone the repository
git clone https://github.com/divkix/Alita_Robot.git
cd Alita_Robot

# Copy and configure environment variables
cp sample.env .env

# Edit .env with your configuration
# At minimum, set: BOT_TOKEN, OWNER_ID, MESSAGE_DUMP

# Start all services
docker-compose up -d
```

## Docker Compose Configuration

The `docker-compose.yml` deploys three services:

```yaml
# Production Docker Compose for Alita Robot
# Uses pre-built images from GHCR

services:
  alita:
    image: ghcr.io/divkix/alita_robot:latest
    container_name: alita-robot
    restart: always
    environment:
      DATABASE_URL: postgresql://alita:alita@postgres:5432/alita
      REDIS_ADDRESS: redis:6379
      REDIS_PASSWORD: redis
      AUTO_MIGRATE: "false"
      USE_WEBHOOKS: "true"
      HTTP_PORT: "8080"
      DROP_PENDING_UPDATES: "true"
      BOT_TOKEN: ${BOT_TOKEN}
      OWNER_ID: ${OWNER_ID}
      MESSAGE_DUMP: ${MESSAGE_DUMP}
      WEBHOOK_DOMAIN: ${WEBHOOK_DOMAIN}
      WEBHOOK_SECRET: ${WEBHOOK_SECRET}
    ports:
      - "8080:8080"
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_started
    healthcheck:
      test: ["CMD", "/alita_robot", "--health"]
      interval: 30s
      timeout: 10s
      retries: 3
    deploy:
      resources:
        limits:
          memory: 1G
          cpus: "1.0"
        reservations:
          memory: 256M
          cpus: "0.25"

  postgres:
    image: postgres:18-alpine
    container_name: alita-postgres
    restart: always
    environment:
      POSTGRES_USER: alita
      POSTGRES_PASSWORD: alita
      POSTGRES_DB: alita
      POSTGRES_INITDB_ARGS: "--encoding=UTF8 --locale=C"
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U alita -d alita"]
      interval: 10s
      timeout: 5s
      retries: 5
    deploy:
      resources:
        limits:
          memory: 512M
          cpus: "0.5"

  redis:
    image: redis:latest
    container_name: alita-redis
    restart: always
    command: redis-server --requirepass redis
    deploy:
      resources:
        limits:
          memory: 256M
          cpus: "0.25"

volumes:
  postgres_data:
```

## Service Descriptions

### alita (Bot Service)

The main Telegram bot service.

| Configuration | Value | Description |
|--------------|-------|-------------|
| **Image** | `ghcr.io/divkix/alita_robot:latest` | Pre-built image from GitHub Container Registry |
| **Port** | 8080 | HTTP server for health, metrics, and webhooks |
| **Memory Limit** | 1GB | Maximum memory allocation |
| **Memory Reservation** | 256MB | Guaranteed minimum memory |
| **CPU Limit** | 1.0 | Maximum CPU cores |
| **CPU Reservation** | 0.25 | Guaranteed minimum CPU |

The health check uses the built-in `--health` flag, which is ideal for distroless container images that lack curl or wget.

### postgres (Database)

PostgreSQL 18 with Alpine Linux for a minimal footprint.

| Configuration | Value | Description |
|--------------|-------|-------------|
| **Image** | `postgres:18-alpine` | PostgreSQL 18 on Alpine Linux |
| **Memory Limit** | 512MB | Maximum memory allocation |
| **Encoding** | UTF8 | Database character encoding |
| **Locale** | C | Collation locale |

Data is persisted in a Docker volume named `postgres_data`.

### redis (Cache)

Redis for caching and session management.

| Configuration | Value | Description |
|--------------|-------|-------------|
| **Image** | `redis:latest` | Latest Redis server |
| **Memory Limit** | 256MB | Maximum memory allocation |
| **Authentication** | Password protected | Uses `REDIS_PASSWORD` env var |

## Environment Variables

Set these in your `.env` file or pass them to docker-compose:

```bash
# Required
BOT_TOKEN=your_bot_token_here
OWNER_ID=your_telegram_id
MESSAGE_DUMP=-100xxxxxxxxxx

# For webhook mode (production)
WEBHOOK_DOMAIN=https://your-domain.com
WEBHOOK_SECRET=your-random-secret
```

For a complete list of environment variables, see the [Environment Reference](/api-reference/environment).

## Resource Limits

The compose file includes recommended resource limits:

| Service | Memory Limit | CPU Limit | Use Case |
|---------|-------------|-----------|----------|
| alita | 1GB | 1.0 | Handles bot traffic |
| postgres | 512MB | 0.5 | Database operations |
| redis | 256MB | 0.25 | Caching |

For high-traffic deployments, increase these limits:

```yaml
deploy:
  resources:
    limits:
      memory: 2G
      cpus: "2.0"
```

## Updating the Bot

Pull the latest image and recreate containers:

```bash
# Pull latest images
docker-compose pull

# Recreate containers with new images
docker-compose up -d

# Optional: Remove old images
docker image prune -f
```

## Viewing Logs

```bash
# View all service logs
docker-compose logs -f

# View only bot logs
docker-compose logs -f alita

# View last 100 lines
docker-compose logs --tail=100 alita

# View PostgreSQL logs
docker-compose logs -f postgres
```

## Common Operations

### Restart the bot

```bash
docker-compose restart alita
```

### Stop all services

```bash
docker-compose down
```

### Stop and remove volumes (WARNING: deletes all data)

```bash
docker-compose down -v
```

### Access PostgreSQL shell

```bash
docker-compose exec postgres psql -U alita -d alita
```

### Access Redis CLI

```bash
docker-compose exec redis redis-cli -a redis
```

## Health Checks

The bot includes built-in health checks accessible at `http://localhost:8080/health`:

```json
{
  "status": "healthy",
  "checks": {
    "database": true,
    "redis": true
  },
  "version": "1.0.0",
  "uptime": "24h30m15s"
}
```

## Troubleshooting

### Container keeps restarting

Check the logs for error messages:

```bash
docker-compose logs alita
```

Common issues:
- Invalid `BOT_TOKEN`
- `MESSAGE_DUMP` channel ID doesn't start with `-100`
- Database connection failed (wait for postgres to be ready)

### Database connection errors

Ensure postgres is healthy before alita starts:

```bash
docker-compose ps
```

The postgres container should show `healthy` status.

### Webhook not working

1. Ensure `WEBHOOK_DOMAIN` is set correctly (must include `https://`)
2. Check that `WEBHOOK_SECRET` is configured
3. Verify port 8080 is accessible externally

See [Webhooks Guide](/self-hosting/webhooks) for detailed setup.
