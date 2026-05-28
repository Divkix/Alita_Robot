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
    if !chat_status.RequireGroup(b, ctx, nil) {
        return ext.EndGroups
    }
    if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id) {
        return ext.EndGroups
    }

    // Business logic here
    text, _ := tr.GetString("example_success_message")
    _, err := msg.Reply(b, text, helpers.Shtml())
    if err != nil {
        log.Error(err)
        // Return ext.EndGroups after user notification, not the error
        return ext.EndGroups
    }

    return ext.EndGroups
}

// Callback handler for inline buttons
func (m moduleStruct) exampleCallback(b *gotgbot.Bot, ctx *ext.Context) error {
    query, ok := callbackQueryFromContext(ctx)
    if !ok {
        return ext.EndGroups
    }
    tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

    // Parse callback data using the modules package wrapper
    // This handles both new codec format and legacy dot-notation fallback
    decoded, ok := decodeCallbackData(query.Data, "example")
    if !ok {
        log.Warn("[ExampleCallback] Invalid callback data format")
        _, _ = query.Answer(b, nil)
        return ext.EndGroups
    }
    action, _ := decoded.Field("action")

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
        callbackquery.Prefix("example"),
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

:::caution[Never ignore DB errors]
Always check `err` returns from database operations. Assigning to `_` causes nil pointer panics when the result is used downstream. This is the single most common source of production crashes.
:::

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

:::note[Surrogate key pattern]
Always use auto-increment `id` as the primary key. External IDs (`chat_id`, `user_id`) should be unique constraints, not primary keys. This decouples internal schema from Telegram's ID space.
:::

### Step 3: Implement Database Operations

Add cache helpers to `alita/db/cache_helpers.go` using the CacheKey helper:

```go
const (
    CacheTTLExampleSettings = 30 * time.Minute
)

// Use the CacheKey helper for consistent key formatting
func exampleSettingsCacheKey(chatID int64) string {
    return CacheKey("example_settings", chatID)
}
```

Update the database operations to use caching with singleflight protection:

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

:::tip[CacheKey helper]
The `CacheKey()` function in `cache_helpers.go` provides consistent key formatting as `alita:{module}:{id}`. Always use it instead of manual string formatting.
:::

:::tip[Cache invalidation is mandatory]
Every function that writes to the database MUST invalidate the corresponding cache key. Forgetting this causes stale data that persists until TTL expiry, which can be up to 1 hour.
:::

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

Add to other locale files (es.yml, fr.yml, hi.yml, id.yml, pt.yml, ru.yml) with appropriate translations.

:::caution[Translation completeness]
You must add keys to ALL locale files, not just `en.yml`. Missing keys cause runtime panics or empty strings. Run `make check-translations` to detect missing keys before committing.
:::

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

:::note[Load order matters]
Place your module after any modules it depends on (e.g., after `LoadUsers` if you need user lookups) and before `LoadHelp` (which is deferred and always loads last).
:::

## Permission Check Functions

Use these functions to validate permissions before executing commands:

| Function | Description | Returns |
|----------|-------------|---------|
| `RequireGroup(b, ctx, chat)` | Ensures command is in group | `bool` |
| `RequirePrivate(b, ctx, chat)` | Ensures command is in PM | `bool` |
| `RequireUserAdmin(b, ctx, chat, userId)` | User must be admin | `bool` |
| `RequireBotAdmin(b, ctx, chat)` | Bot must be admin | `bool` |
| `RequireUserOwner(b, ctx, chat, userId)` | User must be creator | `bool` |
| `CanUserRestrict(b, ctx, chat, userId)` | User can ban/mute | `bool` |
| `CanBotRestrict(b, ctx, chat)` | Bot can ban/mute | `bool` |
| `CanUserDelete(b, ctx, chat, userId)` | User can delete messages | `bool` |
| `CanBotDelete(b, ctx, chat)` | Bot can delete messages | `bool` |
| `CanUserPin(b, ctx, chat, userId)` | User can pin messages | `bool` |
| `CanBotPin(b, ctx, chat)` | Bot can pin messages | `bool` |
| `CanUserPromote(b, ctx, chat, userId)` | User can promote/demote | `bool` |
| `CanBotPromote(b, ctx, chat)` | Bot can promote/demote | `bool` |
| `CanUserChangeInfo(b, ctx, chat, userId)` | User can change chat info | `bool` |
| `CanInvite(b, ctx, chat, msg)` | Can generate invite links | `bool` |

### Permission Error Handling

Permission checking functions are pure and return only boolean values without sending any error messages to users. When error messaging is desired on failure, the caller must explicitly call `PermissionResponder` to send the response.

Example usage:
```go
if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id) {
	chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
	return ext.EndGroups
}
```

### Common Permission Patterns

```go
// Admin-only command
if !chat_status.RequireGroup(b, ctx, nil) {
    return ext.EndGroups
}
if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id) {
    return ext.EndGroups
}

// Command requiring bot to have restrict permissions
if !chat_status.RequireBotAdmin(b, ctx, nil) {
    return ext.EndGroups
}
if !chat_status.CanBotRestrict(b, ctx, nil) {
    return ext.EndGroups
}

// Owner-only command
if !chat_status.RequireUserOwner(b, ctx, nil, user.Id) {
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

:::tip[When to use ContinueGroups]
Use `ext.ContinueGroups` only for monitoring/watcher handlers (handler groups 4-10) that observe traffic without consuming it. Standard command handlers should always return `ext.EndGroups` to prevent duplicate processing.
:::

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

:::caution[Printf type safety]
`%d` requires an `int`, not a `strconv.Itoa()` output (which is a string). Passing the wrong type causes runtime panics. Always match format verbs to argument types.
:::

### Key Naming Convention

Follow the pattern: `module_feature_description`

```yaml
bans_ban_normal_ban: "Banned %s!"
bans_ban_ban_reason: "\nReason: %s"
bans_kick_kicked_user: "Kicked %s!"
bans_unban_unbanned_user: "Unbanned %s!"
```

:::note[Parse mode mismatch]
Locale strings often use Markdown formatting but the bot sends messages in HTML parse mode. Use `tgmd2html.MD2HTMLV2()` to convert before sending. Forgetting this conversion results in raw Markdown symbols appearing in user messages.
:::

## Complete Example: Welcome Toggle Module

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

var welcomeModule = moduleStruct{moduleName: "Welcome"}

// welcomestatus shows whether welcome messages are enabled
func (m moduleStruct) welcomestatus(b *gotgbot.Bot, ctx *ext.Context) error {
    chat := ctx.EffectiveChat
    msg := ctx.EffectiveMessage
    tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

    if !chat_status.RequireGroup(b, ctx, nil) {
        return ext.EndGroups
    }

    settings := db.GetGreetingSettings(chat.Id)
    enabled := false
    if settings.WelcomeSettings != nil {
        enabled = settings.WelcomeSettings.ShouldWelcome
    }

    var text string
    if enabled {
        text, _ = tr.GetString("welcome_status_enabled")
    } else {
        text, _ = tr.GetString("welcome_status_disabled")
    }
    _, err := msg.Reply(b, text, helpers.Shtml())
    if err != nil {
        log.Error(err)
        return err
    }

    return ext.EndGroups
}

// togglewelcome enables or disables welcome messages (admin only)
func (m moduleStruct) togglewelcome(b *gotgbot.Bot, ctx *ext.Context) error {
    chat := ctx.EffectiveChat
    user := ctx.EffectiveSender.User
    msg := ctx.EffectiveMessage
    tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

    if !chat_status.RequireGroup(b, ctx, nil) {
        return ext.EndGroups
    }
    if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id) {
        return ext.EndGroups
    }

    settings := db.GetGreetingSettings(chat.Id)
    current := false
    if settings.WelcomeSettings != nil {
        current = settings.WelcomeSettings.ShouldWelcome
    }

    if err := db.SetWelcomeToggle(chat.Id, !current); err != nil {
        log.Error(err)
        return err
    }

    var text string
    if !current {
        text, _ = tr.GetString("welcome_toggle_enabled")
    } else {
        text, _ = tr.GetString("welcome_toggle_disabled")
    }
    _, err := msg.Reply(b, text, helpers.Shtml())
    if err != nil {
        log.Error(err)
        return err
    }

    return ext.EndGroups
}

func LoadWelcome(dispatcher *ext.Dispatcher) {
    HelpModule.AbleMap.Store(welcomeModule.moduleName, true)

    dispatcher.AddHandler(handlers.NewCommand("welcomestatus", welcomeModule.welcomestatus))
    dispatcher.AddHandler(handlers.NewCommand("togglewelcome", welcomeModule.togglewelcome))
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

:::caution[XSS via Telegram HTML]
Telegram supports a subset of HTML in messages. User-controlled strings (chat titles, usernames, filter keywords) must be escaped with `helpers.HtmlEscape()` before insertion into HTML-formatted messages. Unescaped angle brackets can break message formatting or inject unintended HTML tags.
:::

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

:::caution[Sync before confirm]
Never send a success confirmation to the user before the database write completes. If the write fails, the user has already been told it succeeded. This is a trust-breaking bug that is hard to debug in production.
:::

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

:::tip[Closure variable capture]
Always capture loop/closure variables into local variables before passing them to goroutines. Go closures capture variables by reference, so the value may change by the time the goroutine executes.
:::

### User Extraction

Use `extraction.ExtractUserAndText()` for consistent user identification. It handles:
- Reply messages
- Text mentions
- Username lookups (with Telegram API fallback)
- Numeric user IDs

:::note[Nil sender check]
`ctx.EffectiveSender` can be nil for channel messages. Always check before accessing `.User`. Use `chat_status.RequireUser()` or `chat_status.GetEffectiveUser()` for safe extraction.
:::

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
