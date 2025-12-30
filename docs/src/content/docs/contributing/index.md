---
title: Contributing
description: How to contribute to Alita Robot development.
---

# Contributing to Alita Robot

Thank you for your interest in contributing to Alita Robot! This guide will help you get started with development.

## Development Setup

### Prerequisites

- **Go** 1.25+
- **PostgreSQL** 14+
- **Redis** 6+
- **Make** (for running commands)

### Clone and Setup

```bash
git clone https://github.com/divkix/Alita_Robot.git
cd Alita_Robot
cp sample.env .env
# Edit .env with your configuration
```

### Essential Commands

```bash
make run          # Run the bot locally
make build        # Build release artifacts using goreleaser
make lint         # Run golangci-lint for code quality checks
make tidy         # Clean up and download go.mod dependencies
```

### Database Commands

```bash
make psql-migrate  # Apply all pending PostgreSQL migrations
make psql-status   # Check current migration status
make psql-reset    # Reset database (DANGEROUS: drops all tables)
```

## Project Structure

```
Alita_Robot/
├── alita/
│   ├── config/       # Configuration and environment parsing
│   ├── db/           # Database operations and repositories
│   ├── i18n/         # Internationalization
│   ├── modules/      # Command handlers (one file per module)
│   └── utils/        # Utility functions and decorators
├── locales/          # Translation files (YAML)
├── migrations/       # SQL migration files
└── main.go           # Entry point
```

## Adding a New Module

1. **Create database operations** in `alita/db/{module}_db.go`
2. **Implement command handlers** in `alita/modules/{module}.go`
3. **Register commands** in the module's `init()` function
4. **Add translations** to `locales/en.yml` (and other locale files)

### Module Template

```go
package modules

import (
    "github.com/divkix/Alita_Robot/alita/utils/helpers"
    "github.com/PaulSonOfLars/gotgbot/v2"
    "github.com/PaulSonOfLars/gotgbot/v2/ext"
    "github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
)

var myModule = moduleStruct{moduleName: "mymodule"}

func (m moduleStruct) myCommand(b *gotgbot.Bot, ctx *ext.Context) error {
    // Implementation
    return ext.EndGroups
}

func init() {
    helpers.RegisterModule(myModule.moduleName, func(d *ext.Dispatcher) {
        d.AddHandler(handlers.NewCommand("mycommand", myModule.myCommand))
    })
}
```

## Code Style

- Run `make lint` before committing
- Follow Go conventions and idioms
- Use the repository pattern for data access
- Add proper error handling with panic recovery
- Use decorators for common middleware (permissions, error handling)

## Security Best Practices

### HTML Escaping

Always escape user-controlled input before rendering in HTML-formatted messages:

```go
import "github.com/divkix/Alita_Robot/alita/utils/helpers"

// Wrong - vulnerable to HTML injection
text := fmt.Sprintf("Welcome to %s!", chat.Title)

// Correct - escaped
text := fmt.Sprintf("Welcome to %s!", helpers.HtmlEscape(chat.Title))
```

**When to escape:**
- Chat titles and descriptions
- Usernames (when displaying as text, not as @mentions)
- Any user-supplied text in HTML-formatted messages

**Safe alternatives:**
- `helpers.MentionHtml(userId, name)` - Already escapes the name
- `helpers.MentionUrl(url, name)` - Already escapes the name

### Goroutine Error Handling

When running database operations in goroutines, always:

1. Capture variables for closure safety
2. Add panic recovery
3. Handle and log errors

```go
// Correct pattern
chatId := chat.Id  // Capture variable
go func() {
    defer error_handling.RecoverFromPanic("SetAnonAdminMode", "admin")
    if err := db.SetAnonAdminMode(chatId, true); err != nil {
        log.Errorf("[Admin] Failed to set anon admin mode: %v", err)
    }
}()
```

### User Input Validation

- Never trust usernames from user input for security-critical operations
- Validate user IDs against Telegram API when necessary
- Use `extraction.ExtractUserAndText()` for consistent user resolution

## Translation Guidelines

Add help messages to `locales/en.yml`:

```yaml
mymodule_help_msg: |
  Help text for my module.

  *Commands:*
  × /mycommand: Description of command.
```

## Testing

The project uses `golangci-lint` for code quality. Manual testing is done with a test bot and group:

1. Create a test bot via [@BotFather](https://t.me/BotFather)
2. Create a test group
3. Configure your `.env` with the test bot token
4. Run `make run` and test your changes

## Submitting Changes

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/my-feature`)
3. Make your changes
4. Run `make lint` to check for issues
5. Commit with a descriptive message
6. Push to your fork
7. Open a Pull Request

## Getting Help

- **Support Group**: [t.me/DivideSupport](https://t.me/DivideSupport)
- **GitHub Issues**: [Report bugs or request features](https://github.com/divkix/Alita_Robot/issues)
