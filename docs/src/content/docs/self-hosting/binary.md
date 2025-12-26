---
title: Binary Installation
description: Deploy Alita Robot using pre-built binaries.
---

# Binary Installation

Pre-built binaries are available for all major platforms, making it easy to deploy Alita Robot without Docker or building from source.

## Available Platforms

| Platform | Architecture | Binary Name |
|----------|-------------|-------------|
| Linux | amd64 | `alita_robot_linux_amd64` |
| Linux | arm64 | `alita_robot_linux_arm64` |
| macOS | amd64 (Intel) | `alita_robot_macOS_amd64` |
| macOS | arm64 (Apple Silicon) | `alita_robot_macOS_arm64` |
| Windows | amd64 | `alita_robot_windows_amd64.exe` |
| Windows | arm64 | `alita_robot_windows_arm64.exe` |

## Download

Download the latest release from GitHub:

```bash
# Get the latest release version
VERSION=$(curl -s https://api.github.com/repos/divkix/Alita_Robot/releases/latest | grep tag_name | cut -d '"' -f 4)

# Download for Linux amd64
curl -LO "https://github.com/divkix/Alita_Robot/releases/download/${VERSION}/alita_robot_${VERSION}_linux_amd64.tar.gz"

# Extract
tar -xzf alita_robot_${VERSION}_linux_amd64.tar.gz

# Make executable
chmod +x alita_robot
```

Or visit the [GitHub Releases](https://github.com/divkix/Alita_Robot/releases) page to download manually.

## Verify Checksums

Each release includes a `checksums.txt` file with SHA256 hashes:

```bash
# Download checksums
curl -LO "https://github.com/divkix/Alita_Robot/releases/download/${VERSION}/checksums.txt"

# Verify
sha256sum -c checksums.txt --ignore-missing
```

## Quick Start

1. Download and extract the binary
2. Create a configuration file:

```bash
cat > .env << EOF
BOT_TOKEN=your_bot_token
OWNER_ID=your_telegram_id
MESSAGE_DUMP=-100xxxxxxxxxx
DATABASE_URL=postgres://user:pass@localhost:5432/alita
REDIS_ADDRESS=localhost:6379
EOF
```

3. Run the bot:

```bash
./alita_robot
```

## Systemd Service (Linux)

For production deployments on Linux, run Alita Robot as a systemd service.

### Create Service File

```bash
sudo nano /etc/systemd/system/alita-robot.service
```

Add the following content:

```ini
[Unit]
Description=Alita Robot Telegram Bot
After=network.target postgresql.service redis.service
Wants=postgresql.service redis.service

[Service]
Type=simple
User=alita
Group=alita
WorkingDirectory=/opt/alita-robot
ExecStart=/opt/alita-robot/alita_robot
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

# Environment file
EnvironmentFile=/opt/alita-robot/.env

# Resource limits
LimitNOFILE=65535
MemoryMax=1G
CPUQuota=100%

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
PrivateDevices=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictSUIDSGID=true
RestrictNamespaces=true

[Install]
WantedBy=multi-user.target
```

### Setup

```bash
# Create user and directory
sudo useradd -r -s /bin/false alita
sudo mkdir -p /opt/alita-robot
sudo chown alita:alita /opt/alita-robot

# Copy binary and config
sudo cp alita_robot /opt/alita-robot/
sudo cp .env /opt/alita-robot/
sudo chmod 600 /opt/alita-robot/.env
sudo chown alita:alita /opt/alita-robot/*

# Copy migrations if using auto-migrate
sudo cp -r migrations /opt/alita-robot/

# Reload systemd and enable service
sudo systemctl daemon-reload
sudo systemctl enable alita-robot
```

### Service Management

```bash
# Start the service
sudo systemctl start alita-robot

# Check status
sudo systemctl status alita-robot

# View logs
sudo journalctl -u alita-robot -f

# Stop the service
sudo systemctl stop alita-robot

# Restart the service
sudo systemctl restart alita-robot
```

## Log Rotation

Configure log rotation for journald logs:

```bash
sudo nano /etc/systemd/journald.conf.d/alita-robot.conf
```

```ini
[Journal]
SystemMaxUse=500M
SystemMaxFileSize=50M
MaxRetentionSec=1month
```

Or use a dedicated log file with logrotate:

```bash
sudo nano /etc/logrotate.d/alita-robot
```

```
/var/log/alita-robot/*.log {
    daily
    rotate 7
    compress
    delaycompress
    missingok
    notifempty
    create 640 alita alita
    postrotate
        systemctl reload alita-robot > /dev/null 2>&1 || true
    endscript
}
```

## Environment File

Create `/opt/alita-robot/.env`:

```bash
# Required
BOT_TOKEN=your_bot_token
OWNER_ID=your_telegram_id
MESSAGE_DUMP=-100xxxxxxxxxx
DATABASE_URL=postgres://user:pass@localhost:5432/alita
REDIS_ADDRESS=localhost:6379

# Optional but recommended
AUTO_MIGRATE=true
HTTP_PORT=8080
DEBUG=false

# Webhook mode (optional)
# USE_WEBHOOKS=true
# WEBHOOK_DOMAIN=https://your-domain.com
# WEBHOOK_SECRET=your-secret

# Performance tuning
DB_MAX_OPEN_CONNS=100
DISPATCHER_MAX_ROUTINES=200
```

## macOS Installation

### Using Homebrew (if available)

The binary can be installed manually:

```bash
# Download for Apple Silicon
curl -LO "https://github.com/divkix/Alita_Robot/releases/latest/download/alita_robot_macOS_arm64.tar.gz"
tar -xzf alita_robot_macOS_arm64.tar.gz
chmod +x alita_robot

# Run
./alita_robot
```

### Create a Launch Agent (Optional)

For automatic startup on macOS:

```bash
mkdir -p ~/Library/LaunchAgents
nano ~/Library/LaunchAgents/com.alita-robot.plist
```

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.alita-robot</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/alita_robot</string>
    </array>
    <key>WorkingDirectory</key>
    <string>/usr/local/etc/alita-robot</string>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/usr/local/var/log/alita-robot.log</string>
    <key>StandardErrorPath</key>
    <string>/usr/local/var/log/alita-robot.log</string>
</dict>
</plist>
```

Load the agent:

```bash
launchctl load ~/Library/LaunchAgents/com.alita-robot.plist
```

## Windows Installation

### Download and Extract

1. Download the Windows binary from [GitHub Releases](https://github.com/divkix/Alita_Robot/releases)
2. Extract the ZIP file to a directory (e.g., `C:\alita-robot`)
3. Create a `.env` file in the same directory

### Run as a Service

Use [NSSM](https://nssm.cc/) (Non-Sucking Service Manager) to run as a Windows service:

```powershell
# Install NSSM
choco install nssm

# Install service
nssm install AlitaRobot C:\alita-robot\alita_robot.exe
nssm set AlitaRobot AppDirectory C:\alita-robot
nssm set AlitaRobot AppEnvironmentExtra BOT_TOKEN=your_token
nssm set AlitaRobot Start SERVICE_AUTO_START

# Start service
nssm start AlitaRobot
```

## Updating

To update the binary:

1. Stop the service
2. Download the new binary
3. Replace the existing binary
4. Start the service

```bash
# Linux example
sudo systemctl stop alita-robot
curl -LO "https://github.com/divkix/Alita_Robot/releases/latest/download/alita_robot_linux_amd64.tar.gz"
tar -xzf alita_robot_linux_amd64.tar.gz
sudo cp alita_robot /opt/alita-robot/
sudo systemctl start alita-robot
```

## Troubleshooting

### Permission denied

```bash
chmod +x alita_robot
```

### Library not found

The binary is statically compiled, so no external libraries are needed. If you see library errors, ensure you downloaded the correct platform.

### Cannot connect to database

Verify PostgreSQL is running and accessible:

```bash
psql -h localhost -U postgres -d alita -c "SELECT 1"
```

### Service won't start

Check the logs:

```bash
sudo journalctl -u alita-robot -n 50
```

Common issues:
- Invalid environment variables
- Database connection failed
- Redis not running
