# Alita Robot 🤖

<p align='center'>
  <a href="https://github.com/divkix/Alita_Robot/actions/workflows/ci.yml"><img src="https://github.com/divkix/Alita_Robot/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/divkix/Alita_Robot/actions/workflows/release.yml"> <img src="https://github.com/divkix/Alita_Robot/actions/workflows/release.yml/badge.svg" alt="Release"/> </a>
  <a href="https://goreportcard.com/report/github.com/divkix/Alita_Robot"><img src="https://goreportcard.com/badge/github.com/divkix/Alita_Robot" alt="Go Report Card"></a>
  <a href="https://pkg.go.dev/github.com/divkix/Alita_Robot"><img src="https://pkg.go.dev/badge/github.com/divkix/Alita_Robot.svg" alt="Go Reference"></a>
</p>

<p align='center'>
  <img src="https://img.shields.io/github/forks/divkix/Alita_Robot?style=flat-square" alt="Forks">
  <img src="https://img.shields.io/github/stars/divkix/Alita_Robot?style=flat-square" alt="Stars">
  <img src="https://img.shields.io/github/issues/divkix/Alita_Robot?style=flat-square" alt="Issues">
  <img src="https://img.shields.io/github/license/divkix/Alita_Robot?style=flat-square" alt="LICENSE">
  <img src="https://img.shields.io/github/contributors/divkix/Alita_Robot?style=flat-square" alt="Contributors">
  <img src="https://img.shields.io/github/repo-size/divkix/Alita_Robot?style=flat-square" alt="Repo Size">
  <img src="https://img.shields.io/github/v/release/divkix/Alita_Robot?style=flat-square" alt="Release Version">
</p>

<p align='center'>
  <a href="https://go.dev/"> <img src="https://img.shields.io/badge/Made%20with-Go-1f425f.svg?style=flat-square&logo=Go&color=00ADD8" /> </a>
  <a href="https://www.postgresql.org/"> <img src="https://img.shields.io/badge/Database-PostgreSQL-336791?style=flat-square&logo=postgresql&logoColor=white" /> </a>
  <a href="https://redis.io/"> <img src="https://img.shields.io/badge/Cache-Redis-DC382D?style=flat-square&logo=redis&logoColor=white" /> </a>
  <a href="https://makeapullrequest.com"> <img src="https://img.shields.io/badge/PRs-welcome-brightgreen.svg?style=flat-square" /> </a>
</p>

> **Alita** is a powerful, modern Telegram group management bot built with Go
> and the Gotgbot library. Designed for speed, reliability, and extensive
> customization, Alita provides comprehensive moderation tools for Telegram
> communities of any size.

## 📋 Table of Contents

- [Features](#-features)
- [Quick Start](#-quick-start)
- [Installation](#-installation)
  - [Docker (Recommended)](#docker-recommended)
  - [Binary Release](#binary-release)
  - [Build from Source](#build-from-source)
- [Configuration](#️-configuration)
- [Bot Commands](#-bot-commands)
- [Development](#-development)
- [Contributing](#-contributing)
- [License](#-license)

## ✨ Features

### 📊 **Performance & Optimization**

- **Parallel Bulk Processing**: High-performance batch operations for filters,
  blacklists, and warnings
- **Worker Pool Architecture**: Efficient concurrent task processing with rate
  limiting
- **Smart Caching**: Two-tier caching with stampede protection and TTL
  management
- **Batch Prefetching**: Optimized data loading for reduced database queries
- **Resource Monitoring**: Automatic detection and alerting for memory/goroutine
  issues
- **Performance Analytics**: Built-in statistics collection and performance
  tracking

### 🛡️ **Admin & Moderation**

- **User Management**: Ban, mute, kick, and warn users with customizable actions
- **Permission System**: Granular permission control for admins
- **Anti-Spam**: Configurable flood control and spam detection
- **Blacklist**: Word and sticker filtering with pattern matching

### 💬 **Messaging & Content**

- **Welcome/Goodbye**: Customizable greeting messages with variables
- **Filters**: Keyword-triggered auto-responses with regex support
- **Notes**: Save and retrieve formatted messages
- **Pins**: Manage pinned messages with anti-spam protection
- **Locks**: Control message types (links, forwards, media, etc.)

### 🔧 **Technical Excellence**

- **Performance**: Built with Go for blazing-fast response times
- **Dual-Layer Cache**: Redis + Ristretto for optimal performance
- **Database**: PostgreSQL with connection pooling and batch operations
- **Deployment Modes**: Support for both polling and webhook modes
- **Multi-Language**: i18n support with YAML locale files
- **Monitoring**: Built-in resource monitoring and health checks

### 🚀 **Modern Architecture**

- **Fully Asynchronous**: Non-blocking operations throughout
- **Repository Pattern**: Clean separation of concerns
- **Middleware System**: Extensible command decorators
- **Graceful Shutdown**: Proper cleanup and connection handling
- **Docker Ready**: Multi-architecture images for easy deployment
- **Worker Pools**: Concurrent processing with configurable worker pools
- **Batch Operations**: Optimized bulk database operations with parallel
  processing
- **Performance Monitoring**: Built-in metrics collection and analysis

## 🚀 Quick Start

Get Alita running in under 5 minutes!

### Prerequisites

- Docker and Docker Compose installed
- PostgreSQL database (or use the included one)
- Redis instance (or use the included one)
- Telegram Bot Token from [@BotFather](https://t.me/BotFather)

### Step 1: Clone the Repository

```bash
git clone https://github.com/divkix/Alita_Robot.git
cd Alita_Robot
```

### Step 2: Configure Environment

```bash
cp sample.env .env
# Edit .env with your configuration
nano .env
```

**Required variables:**

```env
BOT_TOKEN=your_bot_token_from_botfather
OWNER_ID=your_telegram_user_id
MESSAGE_DUMP=-100xxxxxxxxx  # Your log channel
DATABASE_URL=postgres://postgres:password@postgres:5432/alita_robot
REDIS_ADDRESS=redis:6379
```

### Step 3: Run with Docker

```bash
docker-compose up -d
```

That's it! Your bot should now be running. Check the logs:

```bash
docker-compose logs -f alita
```

### Interact with Your Bot

Open Telegram and search for your bot username to start using it!

## 💻 Installation

### Docker (Recommended)

We provide official Docker images at `ghcr.io/divkix/alita_robot` for easy
deployment.

#### Using Docker Compose (Full Stack)

This includes PostgreSQL, Redis, and the bot:

```bash
# Clone the repository
git clone https://github.com/divkix/Alita_Robot.git
cd Alita_Robot

# Configure environment
cp sample.env .env
# Edit .env with your settings

# Start all services
docker-compose up -d

# View logs
docker-compose logs -f alita

# Stop services
docker-compose down
```

Note: Database migrations run automatically in Docker (AUTO_MIGRATE=true).

Optional: To run a local Telegram Bot API server for faster file handling, use
the compose profile and set API_SERVER in your `.env`:

```bash
# .env
API_SERVER=http://telegram-bot-api:8081
TELEGRAM_API_ID=your_api_id
TELEGRAM_API_HASH=your_api_hash

# Start with profile
docker compose --profile local-bot-api up -d
```

#### Using Docker Run (Bot Only)

If you have existing PostgreSQL and Redis instances:

```bash
docker run -d \
  --name alita-bot \
  --restart unless-stopped \
  -e BOT_TOKEN="your_bot_token" \
  -e DATABASE_URL="your_postgres_url" \
  -e REDIS_ADDRESS="your_redis_address" \
  -e OWNER_ID="your_telegram_id" \
  -e MESSAGE_DUMP="-100xxxxxxxxx" \
  ghcr.io/divkix/alita_robot:latest
```

### Binary Release

Download pre-built binaries for your platform:

1. Visit the [Releases](https://github.com/divkix/Alita_Robot/releases) page
2. Download the appropriate binary for your OS/architecture:
   - **Linux**: `alita_robot_*_linux_amd64.tar.gz` or `_arm64`
   - **macOS**: `alita_robot_*_darwin_amd64.tar.gz` or `_arm64`
   - **Windows**: `alita_robot_*_windows_amd64.zip`

3. Extract and run:

```bash
# Linux/macOS
tar -xzf alita_robot_*.tar.gz
chmod +x alita_robot
./alita_robot

# Windows
# Extract the zip file and run alita_robot.exe
```

### Build from Source

#### Prerequisites

- Go 1.21 or higher
- PostgreSQL 14+
- Redis 6+
- Make (optional)

#### Build Steps

```bash
# Clone the repository
git clone https://github.com/divkix/Alita_Robot.git
cd Alita_Robot

# Install dependencies
go mod download

# Build the binary
go build -o alita_robot .

# Or use make
make build

# Run the bot
./alita_robot

# (Recommended) Run database migrations before the first run
# Provide your Postgres connection via env vars:
#   PSQL_DB_HOST, PSQL_DB_NAME, PSQL_DB_USER, PSQL_DB_PASSWORD
# Optional: PSQL_DB_PORT (default 5432), PSQL_DB_SSLMODE (default require)
make psql-migrate
```

#### Development Build

```bash
# Run with hot reload (requires air)
go install github.com/cosmtrek/air@latest
air

# Or use make
make run
```

## ⚙️ Configuration

### Environment Variables

Alita uses environment variables for configuration. Create a `.env` file in the
project root:

#### Required Variables

| Variable        | Description                                                  | Example                        |
| --------------- | ------------------------------------------------------------ | ------------------------------ |
| `BOT_TOKEN`     | Telegram Bot Token from [@BotFather](https://t.me/BotFather) | `123456:ABC-DEF...`            |
| `DATABASE_URL`  | PostgreSQL connection string                                 | `postgres://user:pass@host/db` |
| `REDIS_ADDRESS` | Redis server address                                         | `redis:6379`                   |
| `OWNER_ID`      | Your Telegram user ID                                        | `123456789`                    |
| `MESSAGE_DUMP`  | Log channel ID (must start with -100)                        | `-100123456789`                |

#### Optional Variables

| Variable           | Description                      | Default      |
| ------------------ | -------------------------------- | ------------ |
| `REDIS_PASSWORD`   | Redis password                   | (empty)      |
| `ENABLED_LOCALES`  | Comma-separated locale codes     | `en`         |
| `USE_WEBHOOKS`     | Enable webhook mode              | `false`      |
| `WEBHOOK_DOMAIN`   | Webhook domain (if enabled)      | -            |
| `WEBHOOK_SECRET`   | Webhook security token           | -            |
| `WEBHOOK_PORT`     | Webhook server port              | `8080`       |
| `MAX_DB_POOL_SIZE` | Database connection pool size    | (calculated) |
| `CACHE_TTL`        | Cache time-to-live (seconds)     | `300`        |
| `CACHE_SIZE`       | In-memory cache size (MB)        | `100`        |
| `WORKER_POOL_SIZE` | Concurrent worker pool size      | `10`         |
| `QUERY_TIMEOUT`    | Database query timeout (seconds) | `30`         |

See `sample.env` for a complete list of configuration options.

### Webhook Mode (Production)

For production deployments, webhook mode provides better performance and lower
resource usage than polling. Alita supports webhooks with Cloudflare Tunnel for
easy setup behind firewalls.

#### Prerequisites

- Cloudflare account with a domain added to Cloudflare
- Docker and Docker Compose installed

#### Step 1: Create Cloudflare Tunnel

1. Go to [Cloudflare Zero Trust Dashboard](https://one.dash.cloudflare.com)
2. Navigate to **Networks > Tunnels**
3. Click **Create a tunnel** → Choose **Cloudflared**
4. Name your tunnel (e.g., `alita-telegram-bot`)
5. **Copy the tunnel token** from the command shown (the long string after
   `--token`)

#### Step 2: Configure Public Hostname

1. In your tunnel dashboard, go to **Public Hostnames** tab
2. Click **Add a public hostname**
3. Configure:
   - **Subdomain**: `alita-bot` (or your preference)
   - **Domain**: Select your domain
   - **Service**: `http://alita:8080`
   - **Path**: `/webhook/your-secret` (replace with your actual
     `WEBHOOK_SECRET`)

#### Step 3: Environment Configuration

Create your `.env` file with webhook settings:

```bash
# Bot Configuration
BOT_TOKEN=your_bot_token_here
OWNER_ID=your_telegram_user_id
MESSAGE_DUMP=-100xxxxxxxxx

# Database Configuration
DATABASE_URL=postgres://postgres:password@postgres:5432/alita_robot?sslmode=disable
REDIS_ADDRESS=redis:6379
REDIS_PASSWORD=your_redis_password

# Webhook Configuration
USE_WEBHOOKS=true
WEBHOOK_DOMAIN=https://alita-bot.yourdomain.com
WEBHOOK_SECRET=your-random-secret-string-here
WEBHOOK_PORT=8080

# Cloudflare Tunnel
CLOUDFLARE_TUNNEL_TOKEN=eyJhIjoiNzU1...your-tunnel-token-here
```

#### Step 4: Enable Cloudflare Tunnel in Docker

Uncomment the `cloudflared` service in your `docker-compose.yml`:

```yaml
# Uncomment this section for webhook mode
cloudflared:
  image: cloudflare/cloudflared:latest
  container_name: alita-cloudflared
  environment:
    - TUNNEL_TOKEN=${CLOUDFLARE_TUNNEL_TOKEN}
  command: tunnel --no-autoupdate run
  restart: unless-stopped
  depends_on:
    - alita
  deploy:
    resources:
      limits:
        memory: 128M
        cpus: "0.1"
```

#### Step 5: Register Webhook with Telegram

After your bot is running, register the webhook URL with Telegram:

```bash
# Replace YOUR_BOT_TOKEN with your actual bot token
# Replace the URL with your actual webhook URL
curl -X POST "https://api.telegram.org/botYOUR_BOT_TOKEN/setWebhook" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://alita-bot.yourdomain.com/webhook/your-secret",
    "secret_token": "your-secret"
  }'
```

#### Step 6: Deploy

```bash
docker-compose up -d
```

#### Verify Setup

Check webhook status:

```bash
curl "https://api.telegram.org/botYOUR_BOT_TOKEN/getWebhookInfo"
```

#### Switch Back to Polling

To disable webhooks and return to polling mode:

```bash
# Clear webhook
curl -X POST "https://api.telegram.org/botYOUR_BOT_TOKEN/setWebhook" -d "url="

# Update environment
USE_WEBHOOKS=false
```

#### Webhook vs Polling Comparison

| Feature               | Webhook Mode             | Polling Mode              |
| --------------------- | ------------------------ | ------------------------- |
| **Performance**       | ⚡ Real-time updates     | 🐌 1-3 second delay       |
| **Resource Usage**    | 💚 Lower CPU/bandwidth   | 🟡 Higher CPU/bandwidth   |
| **Setup Complexity**  | 🔧 Requires domain setup | ✅ Simple, works anywhere |
| **Production Ready**  | ✅ Recommended           | ⚠️ Development only       |
| **Firewall Friendly** | ✅ Works behind NAT      | ❌ Needs outbound access  |

## 🤖 Bot Commands

### Admin Commands

- `/promote` - Promote user to admin
- `/demote` - Demote admin to user
- `/ban` - Ban a user
- `/unban` - Unban a user
- `/mute` - Mute a user
- `/unmute` - Unmute a user
- `/kick` - Kick a user
- `/warn` - Warn a user
- `/unwarn` - Remove warnings
- `/setwarnlimit` - Set warning limit
- `/lock` - Lock message types
- `/unlock` - Unlock message types

### User Commands

- `/start` - Start the bot
- `/help` - Get help
- `/info` - User information
- `/id` - Get IDs
- `/ping` - Check bot response

### Content Management

- `/filter` - Add keyword filter
- `/filters` - List filters
- `/stop` - Remove filter
- `/save` - Save a note
- `/get` - Get a note
- `/notes` - List notes
- `/clear` - Delete a note

### Group Settings

- `/setwelcome` - Set welcome message
- `/setgoodbye` - Set goodbye message
- `/resetwelcome` - Reset welcome
- `/resetgoodbye` - Reset goodbye
- `/cleanwelcome` - Auto-delete welcomes
- `/cleanservice` - Auto-delete service messages
- `/setflood` - Configure antiflood
- `/blacklist` - Add blacklisted words

## 📖 Documentation

- **Comprehensive Code Documentation**: All 774+ functions across 83 Go files
  are fully documented
- **GoDoc Compatible**: Documentation follows Go standards for automatic
  documentation generation
- **Developer Guide**: See [CLAUDE.md](CLAUDE.md) for detailed architecture and
  development guidelines
- **API Reference**: Run `go doc` or visit
  [pkg.go.dev](https://pkg.go.dev/github.com/divkix/Alita_Robot) for API
  documentation

## 🔨 Development

### Project Structure

```
Alita_Robot/
├── alita/              # Core bot code
│   ├── config/         # Configuration management
│   ├── db/             # Database layer
│   ├── modules/        # Command handlers
│   ├── utils/          # Utility packages
│   └── i18n/           # Internationalization
├── cmd/                # Executables
│   ├── alita/          # Main bot
│   └── migrate/        # Migration tool
├── locales/            # Language files
├── supabase/           # Database migrations
└── docker/             # Docker configurations
```

### Development Setup

1. **Install Go 1.21+**
   ```bash
   # macOS
   brew install go

   # Linux
   wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz
   sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz
   ```

2. **Setup PostgreSQL and Redis**
   ```bash
   # Using Docker
   docker run -d --name postgres -e POSTGRES_PASSWORD=password -p 5432:5432 postgres:14
   docker run -d --name redis -p 6379:6379 redis:7-alpine
   ```

3. **Clone and Build**
   ```bash
   git clone https://github.com/divkix/Alita_Robot.git
   cd Alita_Robot
   go mod download
   make build
   ```

4. **Install Pre-commit Hooks** (Optional but recommended)
   ```bash
   pip install pre-commit
   pre-commit install
   ```

   This will run automatic checks before each commit:
   - Code formatting (gofmt)
   - Linting (golangci-lint)
   - Security checks
   - File cleanup (trailing whitespace, EOF)

5. **Run Database Migrations**

   Supabase is the source of truth for schema files (`supabase/migrations`).
   Migrations are applied to any PostgreSQL by auto-cleaning Supabase-specific
   statements at runtime.

   - Required environment variables for migrations:
     - `PSQL_DB_HOST`, `PSQL_DB_NAME`, `PSQL_DB_USER`, `PSQL_DB_PASSWORD`
     - Optional: `PSQL_DB_PORT` (default: 5432), `PSQL_DB_SSLMODE` (default:
       require)

   ```bash
   # Example: local Postgres
   export PSQL_DB_HOST="localhost"
   export PSQL_DB_NAME="alita_robot"
   export PSQL_DB_USER="postgres"
   export PSQL_DB_PASSWORD="password"
   # export PSQL_DB_PORT="5432"       # optional
   # export PSQL_DB_SSLMODE="require" # optional

   # Apply migrations (auto-cleans Supabase SQL for generic Postgres)
   make psql-migrate
   ```

   Optional: generate cleaned SQL for inspection (not required to run
   migrations):

   ```bash
   make psql-prepare PSQL_MIGRATIONS_DIR=tmp/migrations_cleaned
   ls -1 tmp/migrations_cleaned
   ```

6. **Start Development**
   ```bash
   make run
   ```

### Available Make Commands

```bash
make run          # Run bot locally
make build        # Build release binaries
make lint         # Run linters
make tidy         # Clean dependencies
make vendor       # Vendor dependencies
make psql-migrate # Run migrations
make psql-prepare # Generate cleaned SQL into tmp/migrations_cleaned
make psql-status  # Check migration status
make psql-reset   # Reset database (DANGEROUS)
```

### Adding New Features

1. **Database Model** - Add to `alita/db/`
2. **Repository** - Implement in `alita/db/repositories/`
3. **Handler** - Create in `alita/modules/`
4. **Register** - Add to module's init function
5. **Localize** - Add strings to `locales/`

### Code Quality

```bash
# Run linters
make lint

# Format code
gofmt -w .

# Run tests (when available)
go test ./...
```

### Verifying Releases

All releases are cryptographically attested using GitHub's attestation feature
for supply chain security. To verify:

```bash
# Using GitHub CLI (gh)
gh attestation verify alita_robot_*.tar.gz \
  --owner divkix \
  --repo Alita_Robot
```

This verification ensures:

- The artifact was built by our official GitHub Actions workflow
- The binary hasn't been tampered with since build
- Full build provenance and supply chain integrity

## 🤝 Contributing

We welcome contributions! Here's how to get started:

### Guidelines

1. **Fork the repository** and create your branch from `main`
2. **Write clean code** that follows Go best practices
3. **Test your changes** thoroughly
4. **Update documentation** if needed
5. **Submit a Pull Request** with a clear description

### Development Workflow

```bash
# Fork and clone
git clone https://github.com/YOUR_USERNAME/Alita_Robot.git
cd Alita_Robot

# Create feature branch
git checkout -b feature/amazing-feature

# Make changes and test
make run
make lint

# Commit with conventional commits
git commit -m "feat: add amazing feature"

# Push and create PR
git push origin feature/amazing-feature
```

### Commit Convention

We use [Conventional Commits](https://www.conventionalcommits.org/):

- `feat:` New feature
- `fix:` Bug fix
- `docs:` Documentation
- `refactor:` Code refactoring
- `test:` Testing
- `chore:` Maintenance

### Need Help?

- Join our [Support Group](https://t.me/DivideSupport)
- Check [existing issues](https://github.com/divkix/Alita_Robot/issues)
- Read the [CLAUDE.md](CLAUDE.md) for codebase details

## 🌟 Acknowledgments

### Special Thanks

- **[Paul Larsen](https://github.com/PaulSonOfLars)** - For the amazing
  [Gotgbot](https://github.com/PaulSonOfLars/gotgbot) library and inspiration
  from Marie
- **[ÁÑÑÍHÌLÅTØR SPÄRK](https://github.com/annihilatorrrr)** - Continuous
  motivation and contributions
- **[jayantkageri](https://github.com/jayantkageri)** - Support and
  encouragement
- **[Anony](https://github.com/anonyindian)** - Bug fixes and improvements
- **All Contributors** - Everyone who has helped improve this project!

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file
for details.

```
Copyright (C) 2020-2025 Divkix

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:
```

---

<p align="center">
  Made with ❤️ by <a href="https://divkix.me">Divkix</a> and contributors
</p>

<p align="center">
  <a href="https://t.me/Alita_Robot">Try Alita</a> •
  <a href="https://t.me/DivideSupport">Support Group</a> •
  <a href="https://t.me/DivideProjects">Updates Channel</a>
</p>
