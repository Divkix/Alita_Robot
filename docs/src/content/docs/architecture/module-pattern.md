---
title: Module Pattern
description: How to create new feature modules for Alita Robot.
---

This guide explains how to add new feature modules to Alita Robot, following the established patterns and conventions.

## Module Structure Template

Every module follows this structure:

```go
package modules

import (
    "fmt"
    "strings"

    "github.com/PaulSonOfLars/gotgbot/v2"
    "github.com/PaulSonOfLars/gotgbot/v2/ext"
    "github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
    "github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
    log "github.com/sirupsen/logrus"

    "github.com/divkix/Alita_Robot/alita/db"
    "github.com/divkix/Alita_Robot/alita/i18n"
    "github.com/divkix/Alita_Robot/alita/utils/chat_status"
    "github.com/divkix/Alita_Robot/alita/utils/extraction"
    "github.com/divkix/Alita_Robot/alita/utils/helpers"
)

// Module struct with name for help system
var exampleModule = moduleStruct{moduleName: "Example"}

// Command handler method
func (m moduleStruct) exampleCommand(b *gotgbot.Bot, ctx *ext.Context) error {
    chat := ctx.EffectiveChat
    user := ctx.EffectiveSender.User
    msg := ctx.EffectiveMessage
    tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

    // Permission checks
    if !chat_status.RequireGroup(b, ctx, nil, false) {
        return ext.EndGroups
    }
    if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id, false) {
        return ext.EndGroups
    }

    // Business logic here
    text, _ := tr.GetString("example_success_message")
    _, err := msg.Reply(b, text, helpers.Shtml())
    if err != nil {
        log.Error(err)
        return err
    }

    return ext.EndGroups
}

// Callback handler for inline buttons
func (m moduleStruct) exampleCallback(b *gotgbot.Bot, ctx *ext.Context) error {
    query := ctx.CallbackQuery
    tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

    // Parse callback data
    args := strings.Split(query.Data, ".")
    action := args[1]

    // Handle action
    var responseText string
    switch action {
    case "confirm":
        responseText, _ = tr.GetString("example_confirmed")
    case "cancel":
        responseText, _ = tr.GetString("example_cancelled")
    }

    // Answer callback
    _, err := query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
        Text: responseText,
    })
    if err != nil {
        log.Error(err)
        return err
    }

    return ext.EndGroups
}

// LoadExample registers all handlers for this module
func LoadExample(dispatcher *ext.Dispatcher) {
    // Register in help system
    HelpModule.AbleMap.Store(exampleModule.moduleName, true)

    // Register command handlers
    dispatcher.AddHandler(handlers.NewCommand("example", exampleModule.exampleCommand))

    // Register callback handlers
    dispatcher.AddHandler(handlers.NewCallback(
        callbackquery.Prefix("example."),
        exampleModule.exampleCallback,
    ))
}
```

## Step-by-Step Guide

### Step 1: Create Database Model (If Needed)

Create a new file `alita/db/example_db.go`:

```go
package db

import (
    "gorm.io/gorm"
)

// ExampleSettings stores per-chat example settings
type ExampleSettings struct {
    ID        uint   `gorm:"primaryKey;autoIncrement"`
    ChatID    int64  `gorm:"uniqueIndex;not null"`
    Enabled   bool   `gorm:"default:false"`
    Value     string `gorm:"type:text"`
    CreatedAt int64  `gorm:"autoCreateTime"`
    UpdatedAt int64  `gorm:"autoUpdateTime"`
}

// GetExampleSettings retrieves settings for a chat
func GetExampleSettings(chatID int64) *ExampleSettings {
    var settings ExampleSettings
    tx := db.Session(&gorm.Session{}).Where("chat_id = ?", chatID).First(&settings)
    if tx.Error != nil {
        return &ExampleSettings{ChatID: chatID, Enabled: false}
    }
    return &settings
}

// SetExampleSettings saves settings for a chat
func SetExampleSettings(chatID int64, enabled bool, value string) error {
    settings := ExampleSettings{
        ChatID:  chatID,
        Enabled: enabled,
        Value:   value,
    }

    tx := db.Session(&gorm.Session{}).Where("chat_id = ?", chatID).
        Assign(settings).FirstOrCreate(&settings)

    if tx.Error != nil {
        return tx.Error
    }

    // Invalidate cache
    deleteCache(exampleSettingsCacheKey(chatID))
    return nil
}
```

### Step 2: Create Migration File

Create `migrations/XXX_add_example_settings.sql`:

```sql
-- Create example_settings table
CREATE TABLE IF NOT EXISTS example_settings (
    id SERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL UNIQUE,
    enabled BOOLEAN DEFAULT FALSE,
    value TEXT,
    created_at BIGINT,
    updated_at BIGINT
);

-- Create index for faster lookups
CREATE INDEX IF NOT EXISTS idx_example_settings_chat_id ON example_settings(chat_id);
```

### Step 3: Implement Database Operations

Add cache helpers to `alita/db/cache_helpers.go`:

```go
const (
    CacheTTLExampleSettings = 30 * time.Minute
)

func exampleSettingsCacheKey(chatID int64) string {
    return fmt.Sprintf("alita:example_settings:%d", chatID)
}
```

Update the database operations to use caching:

```go
func GetExampleSettings(chatID int64) *ExampleSettings {
    result, err := getFromCacheOrLoad(
        exampleSettingsCacheKey(chatID),
        CacheTTLExampleSettings,
        func() (*ExampleSettings, error) {
            var settings ExampleSettings
            tx := db.Session(&gorm.Session{}).Where("chat_id = ?", chatID).First(&settings)
            if tx.Error != nil {
                return &ExampleSettings{ChatID: chatID, Enabled: false}, nil
            }
            return &settings, nil
        },
    )
    if err != nil {
        return &ExampleSettings{ChatID: chatID, Enabled: false}
    }
    return result
}
```

### Step 4: Add Translations

Add to `locales/en.yml`:

```yaml
# Example module
example_help: |
  <b>Example Module</b>

  Commands:
  - /example: Run the example command
  - /exampleset <value>: Set the example value

example_success_message: "Example command executed successfully!"
example_value_set: "Example value set to: %s"
example_not_enabled: "Example feature is not enabled in this chat."
example_confirmed: "Action confirmed!"
example_cancelled: "Action cancelled."
```

Add to other locale files (de.yml, etc.) with appropriate translations.

### Step 5: Register Module

Add to `alita/main.go` in `LoadModules`:

```go
func LoadModules(dispatcher *ext.Dispatcher) {
    modules.HelpModule.AbleMap.Init()
    defer modules.LoadHelp(dispatcher)

    // ... existing modules ...
    modules.LoadExample(dispatcher)  // Add your module
}
```

## Permission Check Functions

Use these functions to validate permissions before executing commands:

| Function | Description | Returns |
|----------|-------------|---------|
| `RequireGroup(b, ctx, chat, justCheck)` | Ensures command is in group | `bool` |
| `RequirePrivate(b, ctx, chat, justCheck)` | Ensures command is in PM | `bool` |
| `RequireUserAdmin(b, ctx, chat, userId, justCheck)` | User must be admin | `bool` |
| `RequireBotAdmin(b, ctx, chat, justCheck)` | Bot must be admin | `bool` |
| `RequireUserOwner(b, ctx, chat, userId, justCheck)` | User must be creator | `bool` |
| `CanUserRestrict(b, ctx, chat, userId, justCheck)` | User can ban/mute | `bool` |
| `CanBotRestrict(b, ctx, chat, justCheck)` | Bot can ban/mute | `bool` |
| `CanUserDelete(b, ctx, chat, userId, justCheck)` | User can delete messages | `bool` |
| `CanBotDelete(b, ctx, chat, justCheck)` | Bot can delete messages | `bool` |
| `CanUserPin(b, ctx, chat, userId, justCheck)` | User can pin messages | `bool` |
| `CanBotPin(b, ctx, chat, justCheck)` | Bot can pin messages | `bool` |
| `CanUserPromote(b, ctx, chat, userId, justCheck)` | User can promote/demote | `bool` |
| `CanBotPromote(b, ctx, chat, justCheck)` | Bot can promote/demote | `bool` |
| `CanUserChangeInfo(b, ctx, chat, userId, justCheck)` | User can change chat info | `bool` |
| `Caninvite(b, ctx, chat, msg, justCheck)` | Can generate invite links | `bool` |

### justCheck Parameter

- `justCheck = false`: Sends error message to user if check fails
- `justCheck = true`: Silently returns false without messaging

### Common Permission Patterns

```go
// Admin-only command
if !chat_status.RequireGroup(b, ctx, nil, false) {
    return ext.EndGroups
}
if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id, false) {
    return ext.EndGroups
}

// Command requiring bot to have restrict permissions
if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
    return ext.EndGroups
}
if !chat_status.CanBotRestrict(b, ctx, nil, false) {
    return ext.EndGroups
}

// Owner-only command
if !chat_status.RequireUserOwner(b, ctx, nil, user.Id, false) {
    return ext.EndGroups
}
```

## Handler Return Values

```go
// Stop processing - no more handlers will run
return ext.EndGroups

// Continue to next handler in same group
return ext.ContinueGroups

// Error - propagates to dispatcher error handler
return err

// Success with no error
return nil
```

### When to Use Each

| Return | Use When |
|--------|----------|
| `ext.EndGroups` | Command handled successfully, stop processing |
| `ext.ContinueGroups` | Allow other handlers to also process this update |
| `err` | Something went wrong, let error handler deal with it |
| `nil` | Same as `ext.EndGroups` for most purposes |

## Translation Best Practices

### Parameter Passing

Use positional formatters in YAML with named parameters in code:

```yaml
# locales/en.yml
example_user_action: "User %s performed action: %s"
```

```go
// In handler
text, _ := tr.GetString("example_user_action")
formattedText := fmt.Sprintf(text, userName, actionName)
```

### Escape Sequences

Always use double quotes for strings with escape sequences:

```yaml
# Correct - double quotes interpret \n
example_multiline: "Line 1\nLine 2\nLine 3"

# Wrong - single quotes preserve \n literally
example_multiline: 'Line 1\nLine 2\nLine 3'
```

### Key Naming Convention

Follow the pattern: `module_feature_description`

```yaml
bans_ban_normal_ban: "Banned %s!"
bans_ban_ban_reason: "\nReason: %s"
bans_kick_kicked_user: "Kicked %s!"
bans_unban_unbanned_user: "Unbanned %s!"
```

## Complete Example: Greeting Counter Module

Here's a complete example showing all patterns together:

```go
package modules

import (
    "fmt"

    "github.com/PaulSonOfLars/gotgbot/v2"
    "github.com/PaulSonOfLars/gotgbot/v2/ext"
    "github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
    log "github.com/sirupsen/logrus"

    "github.com/divkix/Alita_Robot/alita/db"
    "github.com/divkix/Alita_Robot/alita/i18n"
    "github.com/divkix/Alita_Robot/alita/utils/chat_status"
    "github.com/divkix/Alita_Robot/alita/utils/helpers"
)

var counterModule = moduleStruct{moduleName: "Counter"}

// getcount shows the current greeting count
func (m moduleStruct) getcount(b *gotgbot.Bot, ctx *ext.Context) error {
    chat := ctx.EffectiveChat
    msg := ctx.EffectiveMessage
    tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

    if !chat_status.RequireGroup(b, ctx, nil, false) {
        return ext.EndGroups
    }

    count := db.GetGreetingCount(chat.Id)
    text, _ := tr.GetString("counter_current_count")
    _, err := msg.Reply(b, fmt.Sprintf(text, count), helpers.Shtml())
    if err != nil {
        log.Error(err)
        return err
    }

    return ext.EndGroups
}

// resetcount resets the greeting count (admin only)
func (m moduleStruct) resetcount(b *gotgbot.Bot, ctx *ext.Context) error {
    chat := ctx.EffectiveChat
    user := ctx.EffectiveSender.User
    msg := ctx.EffectiveMessage
    tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

    if !chat_status.RequireGroup(b, ctx, nil, false) {
        return ext.EndGroups
    }
    if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id, false) {
        return ext.EndGroups
    }

    err := db.ResetGreetingCount(chat.Id)
    if err != nil {
        log.Error(err)
        text, _ := tr.GetString("counter_reset_error")
        _, _ = msg.Reply(b, text, nil)
        return err
    }

    text, _ := tr.GetString("counter_reset_success")
    _, err = msg.Reply(b, text, helpers.Shtml())
    if err != nil {
        log.Error(err)
        return err
    }

    return ext.EndGroups
}

func LoadCounter(dispatcher *ext.Dispatcher) {
    HelpModule.AbleMap.Store(counterModule.moduleName, true)

    dispatcher.AddHandler(handlers.NewCommand("getcount", counterModule.getcount))
    dispatcher.AddHandler(handlers.NewCommand("resetcount", counterModule.resetcount))
}
```

## Security Considerations

### HTML Escaping

When displaying user-controlled data in HTML-formatted messages, always escape it:

```go
import "github.com/divkix/Alita_Robot/alita/utils/helpers"

// Escape chat titles, usernames, and user-supplied text
text := fmt.Sprintf("Settings for %s", helpers.HtmlEscape(chat.Title))
```

The `helpers.MentionHtml()` function already handles escaping for user names.

### Database Operations and User Feedback

**Prefer synchronous operations when sending success confirmations:**

```go
// CORRECT: Synchronous operation before success message
db.SetWelcomeText(chat.Id, db.DefaultWelcome, "", nil, db.TEXT)
_, err := msg.Reply(b, "Welcome message reset successfully!", helpers.Shtml())
```

```go
// AVOID: Async operation with premature success message
go func() {
    db.SetWelcomeText(chat.Id, db.DefaultWelcome, "", nil, db.TEXT) // May fail silently
}()
_, err := msg.Reply(b, "Success!") // User sees success even if DB write fails
```

**When async operations are necessary**, only use them for non-critical background tasks that don't require user confirmation.

### Handling Functions That Return Errors

**Always check errors from database operations that can fail:**

```go
// CORRECT: Check error and handle nil case
captchaSettings, err := db.GetCaptchaSettings(chat.Id)
if err != nil {
    log.Errorf("Failed to get captcha settings: %v", err)
    captchaSettings = &db.CaptchaSettings{Enabled: false} // Use safe default
}
if captchaSettings != nil && captchaSettings.Enabled {
    // Safe to access
}
```

```go
// AVOID: Ignoring errors can cause nil pointer panics
captchaSettings, _ := db.GetCaptchaSettings(chat.Id)
if captchaSettings.Enabled { // May panic if captchaSettings is nil!
    // ...
}
```

### Async Database Operations (When Appropriate)

When running DB operations in goroutines for non-critical background tasks, follow this pattern:

```go
// 1. Capture loop/closure variables
chatId := chat.Id

go func() {
    // 2. Add panic recovery
    defer error_handling.RecoverFromPanic("FunctionName", "module")

    // 3. Handle errors explicitly
    if err := db.SomeOperation(chatId); err != nil {
        log.Errorf("[Module] Operation failed for chat %d: %v", chatId, err)
    }
}()
```

### User Extraction

Use `extraction.ExtractUserAndText()` for consistent user identification. It handles:
- Reply messages
- Text mentions
- Username lookups (with Telegram API fallback)
- Numeric user IDs

## Checklist for New Modules

- [ ] Create module struct with `moduleName`
- [ ] Implement handler methods on module struct
- [ ] Add appropriate permission checks
- [ ] Use `i18n.MustNewTranslator(db.GetLanguage(ctx))` for translations
- [ ] Handle errors properly (log and return)
- [ ] **Escape user-controlled input with `helpers.HtmlEscape()`**
- [ ] **Add panic recovery to goroutines**
- [ ] Create database models if needed
- [ ] Create migration file if needed
- [ ] Add cache helpers if needed
- [ ] Add translations to all locale files
- [ ] Register module in `LoadModules`
- [ ] Store module in help system with `HelpModule.AbleMap.Store`
- [ ] Test in development environment

## Next Steps

- [Request Flow](/architecture/request-flow) - Understanding the update pipeline
- [Caching](/architecture/caching) - Redis cache integration
- [Project Structure](/architecture/project-structure) - Where files belong
