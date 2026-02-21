---
title: Building from Source
description: Compile Alita Robot from source code.
---

# Building from Source

Build Alita Robot from source for development, customization, or when pre-built binaries are not available for your platform.

## Prerequisites

| Tool | Minimum Version | Purpose |
|------|-----------------|---------|
| **Go** | 1.25+ | Compilation |
| **Git** | 2.0+ | Clone repository |
| **Make** | Any | Build automation |
| **golangci-lint** | Latest | Code quality checks |

### Install Go

```bash
# macOS
brew install go

# Ubuntu/Debian
sudo snap install go --classic

# Or download from https://go.dev/dl/
```

### Install golangci-lint

```bash
# macOS
brew install golangci-lint

# Linux/macOS via script
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin

# Go install
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

## Clone the Repository

```bash
git clone https://github.com/divkix/Alita_Robot.git
cd Alita_Robot
```

## Build Commands

### Development Build

Run the bot directly from source:

```bash
# Using Make
make run

# Or using Go directly
go run main.go
```

### Production Build

Build optimized release binaries using GoReleaser:

```bash
# Install GoReleaser
go install github.com/goreleaser/goreleaser@latest

# Build
make build
```

This creates binaries in the `dist/` directory for all supported platforms.

### Manual Build

Build for your current platform:

```bash
go build -o alita_robot .
```

Build with optimizations:

```bash
go build -ldflags="-s -w" -o alita_robot .
```

Cross-compile for other platforms:

```bash
# Linux amd64
GOOS=linux GOARCH=amd64 go build -o alita_robot_linux_amd64 .

# macOS arm64
GOOS=darwin GOARCH=arm64 go build -o alita_robot_darwin_arm64 .

# Windows amd64
GOOS=windows GOARCH=amd64 go build -o alita_robot_windows_amd64.exe .
```

## Makefile Targets

| Target | Command | Description |
|--------|---------|-------------|
| `run` | `make run` | Run the bot from source |
| `build` | `make build` | Build release binaries with GoReleaser |
| `lint` | `make lint` | Run golangci-lint for code quality |
| `tidy` | `make tidy` | Clean and download go.mod dependencies |
| `vendor` | `make vendor` | Vendor dependencies |
| `psql-migrate` | `make psql-migrate` | Apply database migrations |
| `psql-status` | `make psql-status` | Check migration status |
| `psql-reset` | `make psql-reset` | Reset database (DANGEROUS) |
| `test` | `make test` | Run test suite (`go test ./...`) |
| `check-translations` | `make check-translations` | Detect missing translation keys across locale files |
| `generate-docs` | `make generate-docs` | Generate documentation site content |
| `docs-dev` | `make docs-dev` | Start Astro docs dev server |

## Code Quality

### Run Linter

```bash
make lint
```

This runs golangci-lint with the project's configuration. Fix any issues before committing.

### Check Dependencies

```bash
make tidy
```

This:
- Removes unused dependencies
- Downloads missing dependencies
- Updates `go.sum` checksums

## Project Structure

```
Alita_Robot/
├── main.go              # Entry point
├── go.mod               # Go module definition
├── go.sum               # Dependency checksums
├── Makefile             # Build automation
├── .goreleaser.yaml     # GoReleaser configuration
├── sample.env           # Example environment file
├── docker-compose.yml   # Docker Compose configuration
├── alita/               # Main application code
│   ├── config/          # Configuration loading
│   ├── db/              # Database models and operations
│   ├── modules/         # Bot command handlers
│   ├── utils/           # Utility functions
│   ├── i18n/            # Internationalization
│   └── health/          # Health check handlers
├── migrations/          # SQL migration files
├── locales/             # Translation files
└── docs/                # Documentation
```

## Development Mode

For development, run with debug logging:

```bash
DEBUG=true make run
```

Or set in your `.env` file:

```bash
DEBUG=true
```

Debug mode:
- Increases log verbosity
- Disables performance monitoring
- Shows detailed error stack traces

## GoReleaser Configuration

The `.goreleaser.yaml` defines the build process:

```yaml
version: 2
project_name: alita_robot

builds:
  - binary: alita_robot
    env:
      - CGO_ENABLED=0
    goos:
      - darwin
      - linux
      - windows
    goarch:
      - amd64
      - arm64
    flags:
      - -trimpath
    ldflags:
      - -s -w -X main.version={{.Version}} -X main.commit={{.Commit}}
```

### Build Output

After running `make build`, binaries are created in:

```
dist/
├── alita_robot_linux_amd64/
│   └── alita_robot
├── alita_robot_linux_arm64/
│   └── alita_robot
├── alita_robot_darwin_amd64/
│   └── alita_robot
├── alita_robot_darwin_arm64/
│   └── alita_robot
├── alita_robot_windows_amd64/
│   └── alita_robot.exe
└── checksums.txt
```

## Dependencies

Key dependencies from `go.mod`:

| Package | Purpose |
|---------|---------|
| `gotgbot/v2` | Telegram Bot API client |
| `gorm.io/gorm` | ORM for database operations |
| `go-redis/v9` | Redis client |
| `sirupsen/logrus` | Structured logging |
| `spf13/viper` | Configuration management |
| `prometheus/client_golang` | Metrics |

## Testing

While the project primarily uses golangci-lint for quality checks:

```bash
# Run linter (recommended before commits)
make lint

# Test manually with a test bot
DEBUG=true make run
```

## Hot Reload (Development)

Use `air` for automatic reloading during development:

```bash
# Install air
go install github.com/air-verse/air@latest

# Create .air.toml
air init

# Run with hot reload
air
```

Example `.air.toml`:

```toml
root = "."
tmp_dir = "tmp"

[build]
cmd = "go build -o ./tmp/alita_robot ."
bin = "tmp/alita_robot"
include_ext = ["go"]
exclude_dir = ["tmp", "vendor", "docs"]
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make changes
4. Run `make lint` and fix any issues
5. Submit a pull request

See [Contributing Guide](/contributing) for more details.

## Troubleshooting

### Go version too old

```
go: go.mod requires go >= 1.25
```

Update Go to version 1.25 or higher.

### golangci-lint not found

```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Add to PATH
export PATH=$PATH:$(go env GOPATH)/bin
```

### Module download errors

```bash
# Clear module cache
go clean -modcache

# Download dependencies
make tidy
```

### Build fails with CGO errors

The bot is built with `CGO_ENABLED=0` for static binaries. If you need CGO:

```bash
CGO_ENABLED=1 go build -o alita_robot .
```

Note: CGO requires C compiler and platform-specific libraries.
