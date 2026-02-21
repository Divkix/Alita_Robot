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
make test         # Run test suite
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
3. **Register commands** in a `LoadXxx(dispatcher)` function
4. **Add translations** to `locales/en.yml` (and other locale files)
5. **Call `LoadMyModule(dispatcher)`** from `LoadModules()` in `alita/main.go`

### Module Template

```go
package modules

import (
    "github.com/PaulSonOfLars/gotgbot/v2"
    "github.com/PaulSonOfLars/gotgbot/v2/ext"
    "github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
)

var myModule = moduleStruct{moduleName: "mymodule"}

func (m moduleStruct) myCommand(b *gotgbot.Bot, ctx *ext.Context) error {
    // Implementation
    return ext.EndGroups
}

func LoadMyModule(d *ext.Dispatcher) {
    d.AddHandler(handlers.NewCommand("mycommand", myModule.myCommand))
}
```

## Code Style

- Run `make lint` before committing
- Run `make test` before committing
- Follow Go conventions and idioms
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

Automated regression tests are required for all contributions:

1. Run `make test` (or `go test ./...`) and make sure all tests pass
2. Run `make lint` and fix any lint findings
3. Manually verify behavior with a test bot/group for user-facing changes

## Submitting Changes

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/my-feature`)
3. Make your changes
4. Run `make test` and `make lint`
5. Commit with a descriptive message
6. Push to your fork
7. Open a Pull Request

## Common Pitfalls

These are common bugs to avoid when developing modules:

### Nil Pointer on User Extraction

**Problem:** `ctx.EffectiveSender.User` can be nil for channel posts.

```go
// Wrong - will panic on channel posts
user := ctx.EffectiveSender.User
userId := user.Id

// Correct - use the safe helper
user := chat_status.RequireUser(b, ctx, false)
if user == nil {
    return ext.EndGroups
}
```

### Shadow Variables in Conditionals

**Problem:** Using `:=` inside conditionals creates a new variable that shadows the outer one.

```go
// Wrong - shadows outer userId
if condition {
    userId := someValue  // New variable!
}
// userId here is still the original value

// Correct - reassigns the outer variable
if condition {
    userId = someValue  // Reassigns existing variable
}
```

### Empty Slice Access

**Problem:** Accessing slice elements without bounds checking.

```go
// Wrong - panics if args is empty
args := strings.Fields(input)
firstArg := args[0]

// Correct - check length first
args := strings.Fields(input)
if len(args) == 0 {
    // Handle empty case
    return
}
firstArg := args[0]
```

### Callback Data Validation

**Problem:** Not validating callback query data before parsing.

```go
// Wrong - panics on malformed data
args := strings.Split(query.Data, ".")
action := args[1]
userId, _ := strconv.Atoi(args[2])

// Correct - validate first
args := strings.Split(query.Data, ".")
if len(args) < 3 {
    log.Error("Malformed callback data")
    return ext.EndGroups
}
action := args[1]
userId, err := strconv.Atoi(args[2])
if err != nil {
    log.Error("Invalid userId in callback")
    return ext.EndGroups
}
```

### Goroutine Variable Capture

**Problem:** Goroutines capturing loop variables or mutable state.

```go
// Risky - captures variables by reference
go func() {
    doSomething(userId)  // userId might change
}()

// Safer - pass as parameters
go func(uid int64) {
    doSomething(uid)
}(userId)
```

### Markdown/HTML Parse Mode Mismatch

**Problem:** Locale strings use Markdown formatting (`*bold*`, `` `code` ``) but the bot sends messages with HTML parse mode, causing raw asterisks to appear instead of formatted text.

```go
// Wrong - locale uses Markdown, but sending with HTML parse mode
helpMsg, _ := tr.GetString("module_help_msg")  // Contains *bold*
b.SendMessage(chatId, helpMsg, &gotgbot.SendMessageOpts{
    ParseMode: helpers.HTML,  // Markdown won't render!
})

// Correct - convert Markdown to HTML before sending
helpMsg, _ := tr.GetString("module_help_msg")
htmlMsg := tgmd2html.MD2HTMLV2(helpMsg)  // Converts *bold* to <b>bold</b>
b.SendMessage(chatId, htmlMsg, &gotgbot.SendMessageOpts{
    ParseMode: helpers.HTML,
})
```

The `tgmd2html` library provides conversion functions:
- `tgmd2html.MD2HTMLV2(text)` - Converts Markdown formatting to HTML
- `tgmd2html.MD2HTMLButtonsV2(text)` - Converts and extracts inline buttons

## Getting Help

- **Support Group**: [t.me/DivideSupport](https://t.me/DivideSupport)
- **GitHub Issues**: [Report bugs or request features](https://github.com/divkix/Alita_Robot/issues)
