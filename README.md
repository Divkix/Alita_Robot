# Alita Robot 🤖

<p align='center'>
  <a href="https://github.com/divkix/Alita_Robot/actions/workflows/ci.yml"><img src="https://github.com/divkix/Alita_Robot/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/divkix/Alita_Robot/actions/workflows/release.yml"><img src="https://github.com/divkix/Alita_Robot/actions/workflows/release.yml/badge.svg" alt="Release"></a>
  <a href="https://goreportcard.com/report/github.com/divkix/Alita_Robot"><img src="https://goreportcard.com/badge/github.com/divkix/Alita_Robot" alt="Go Report Card"></a>
  <a href="https://pkg.go.dev/github.com/divkix/Alita_Robot"><img src="https://pkg.go.dev/badge/github.com/divkix/Alita_Robot.svg" alt="Go Reference"></a>
</p>

Alita is a Telegram group-management bot written in Go. It provides moderation,
filters, notes, greetings, anti-spam, captcha, backups, and multi-language
support.

## Quick Start

Prerequisites: Docker, Docker Compose, a Telegram bot token from
[@BotFather](https://t.me/BotFather), and a Telegram user ID plus log-channel ID.

```bash
git clone https://github.com/divkix/Alita_Robot.git
cd Alita_Robot
cp sample.env .env
# Set BOT_TOKEN, OWNER_ID, and MESSAGE_DUMP in .env.
docker compose up -d
docker compose logs -f alita
```

PostgreSQL and Redis are included in the Compose stack.

## Documentation

For setup, configuration, deployment, and command references, see
[alita-docs.divkix.me](https://alita-docs.divkix.me). For contributor and
architecture guidance, see [AGENTS.md](AGENTS.md).

## Development

```bash
make run
make lint
make test
```

## Contributing

Open an issue or pull request on GitHub. Please use conventional commits and
run the relevant checks before submitting changes.

## License

[MIT](LICENSE) © 2020-2026 Divkix and contributors.

<p align="center">
  <a href="https://t.me/Alita_Robot">Try Alita</a> •
  <a href="https://t.me/DivideSupport">Support Group</a> •
  <a href="https://t.me/DivideProjects">Updates Channel</a>
</p>
