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
    "github.com/PaulSonOfLars/gotgbot/v2"
    "github.com/PaulSonOfLars/gotgbot/v2/ext"
    "github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
    "github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
    log "github.com/sirupsen/logrus"

    "github.com/divkix/Alita_Robot/alita/db/lang"
    "github.com/divkix/Alita_Robot/alita/i18n"
    "github.com/divkix/Alita_Robot/alita/utils/formatting"
    "github.com/divkix/Alita_Robot/alita/utils/helpers"
)

// Module struct with name for help system
var exampleModule = moduleStruct{moduleName: "Example"}

// Declarative command config. WrapCommand runs RequiredChecks in order; a
// failing check sends its own reply, so the handler adds none.
var exampleDesc = helpers.CommandDescriptor{
    Name:        "example",
    Disableable: true, // lists the command for /disable
    RequiredChecks: []helpers.CheckFunc{
        helpers.CheckDisabled("example"), // enforces /disable
        helpers.RequireGroup(),
        helpers.RequireUserAdmin(),
    },
}

// Command handler method (checks already ran in the pipeline)
func (m moduleStruct) exampleCommand(b *gotgbot.Bot, ctx *ext.Context) error {
    msg := ctx.EffectiveMessage
    tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

    // Business logic here
    text, _ := tr.GetString("example_success_message")
    _, err := msg.Reply(b, text, formatting.Shtml())
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
    tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

    // Parse callback data using the modules package wrapper
    // (versioned codec only — legacy dot-notation callbacks are rejected)
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
    // Register in help system (takes the registry lock)
    SetModuleEnabled(exampleModule.moduleName, true)

    // Register commands through the pipeline
    helpers.WrapCommand(dispatcher, exampleDesc, pipelineHandler(exampleModule.exampleCommand))

    // Register callback handlers
    dispatcher.AddHandler(handlers.NewCallback(
        callbackquery.Prefix("example"),
        exampleModule.exampleCallback,
    ))
}

// Register the module in init() so LoadAllModules picks it up
func init() {
    RegisterLegacyModule("Example", 100, LoadExample)
    // Admin command: also runs for anonymous admins once they prove admin rights
    RegisterAnonymousAdminHandler("example", anonPipelineHandler(exampleDesc, exampleModule.exampleCommand))
}
```

:::note[Command pipeline]
Register new commands with `helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{...}, handler)`. The handler is `func(*helpers.CommandContext) error`; `pipelineHandler` adapts a `(b, ctx)` method. The `helpers.*` CheckFuncs in `RequiredChecks` send their own failure replies, so do not add replies for failed checks. `Disableable: true` only lists the command for `/disable`; `helpers.CheckDisabled("<cmd>")` enforces it. Anonymous admins bypass `WrapCommand`, so admin commands also need `RegisterAnonymousAdminHandler` with `anonPipelineHandler`, which re-runs the same checks.
:::

## Step-by-Step Guide

### Step 1: Create Database Model (If Needed)

Add the model in `alita/db/models/example.go`. `TableName()` must return the migration's table name:

```go
package models

import "time"

// ExampleSettings stores per-chat example settings
type ExampleSettings struct {
    ID        uint      `gorm:"primaryKey;autoIncrement"`
    ChatID    int64     `gorm:"uniqueIndex;not null"`
    Enabled   bool      `gorm:"default:false"`
    Value     string    `gorm:"type:text"`
    CreatedAt time.Time `gorm:"column:created_at"`
    UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (ExampleSettings) TableName() string { return "example_settings" }
```

Add the model to the `AutoMigrate` list in the affected `testmain_test.go` or `internal/testdb.Run` call so tests create its table.

Put the repository in `alita/db/example/repository.go` (package `example`). Every write must invalidate the keys it affects:

```go
package example

import (
    "github.com/divkix/Alita_Robot/alita/db"
    "github.com/divkix/Alita_Robot/alita/db/cache"
    "github.com/divkix/Alita_Robot/alita/db/models"
)

func exampleSettingsCacheKey(chatID int64) string {
    return cache.CacheKey("example_settings", chatID)
}

// SetExampleSettings saves settings for a chat
func SetExampleSettings(chatID int64, enabled bool, value string) error {
    var settings models.ExampleSettings
    // A map (not a struct) so false and "" are written too
    tx := db.DB.Where("chat_id = ?", chatID).
        Assign(map[string]any{"enabled": enabled, "value": value}).
        FirstOrCreate(&settings, models.ExampleSettings{ChatID: chatID})
    if tx.Error != nil {
        return tx.Error
    }

    // Invalidate cache
    cache.DeleteCache(exampleSettingsCacheKey(chatID))
    return nil
}
```

:::caution[Never ignore DB errors]
Always check `err` returns from database operations. Assigning to `_` causes nil pointer panics when the result is used downstream. This is the single most common source of production crashes.
:::

### Step 2: Create Migration File

Create `migrations/<YYYYMMDDHHMMSS>_add_example_settings.sql`, with a timestamp greater than every existing migration (files apply in plain filename order):

```sql
-- Create example_settings table
CREATE TABLE IF NOT EXISTS example_settings (
    id BIGSERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL UNIQUE,
    enabled BOOLEAN DEFAULT FALSE,
    value TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

:::note[Surrogate key pattern]
Always use auto-increment `id` as the primary key. External IDs (`chat_id`, `user_id`) should be unique constraints, not primary keys. This decouples internal schema from Telegram's ID space.
:::

:::caution[Migration rules]
Each file runs in one transaction: no top-level `BEGIN`/`COMMIT`/`ROLLBACK` and no `CREATE INDEX CONCURRENTLY`. Never edit an applied migration; its SHA-256 checksum is checked at startup and a mismatch aborts it.
:::

### Step 3: Implement Database Operations

Add a TTL constant to `alita/db/cache/ttl.go`:

```go
// alita/db/cache/ttl.go
const (
    CacheTTLExampleSettings = 30 * time.Minute
)
```

Read through `cache.GetFromCacheOrLoad`, which shares one database load per key among concurrent callers. Return the default only when the row is missing; return any other error so an outage default is never cached:

```go
// alita/db/example/repository.go (also import "context", "errors" and "gorm.io/gorm")
func GetExampleSettings(ctx context.Context, chatID int64) (*models.ExampleSettings, error) {
    return cache.GetFromCacheOrLoad(ctx, exampleSettingsCacheKey(chatID), cache.CacheTTLExampleSettings,
        func(ctx context.Context) (*models.ExampleSettings, error) {
            var settings models.ExampleSettings
            err := db.DB.WithContext(ctx).Where("chat_id = ?", chatID).First(&settings).Error
            if errors.Is(err, gorm.ErrRecordNotFound) {
                return &models.ExampleSettings{ChatID: chatID}, nil
            }
            if err != nil {
                return nil, err
            }
            return &settings, nil
        })
}
```

:::tip[CacheKey helper]
The `CacheKey()` function in `alita/db/cache/keys.go` provides consistent key formatting as `alita:cache:{module}:{id}`. The `alita:cache:` prefix (`DataCachePrefix`) is what `CLEAR_CACHE_ON_STARTUP` clears. Always use it instead of manual string formatting.
:::

:::tip[Cache invalidation is mandatory]
Every function that writes to the database MUST invalidate the corresponding cache key. Forgetting this causes stale data that persists until TTL expiry, which can be up to 1 hour.
:::

### Step 4: Add Translations

Add to `locales/en.yml`:

```yaml
# Example module: /help reads <module>_help_msg (lowercased module name)
example_help_msg: |
  Example module help.

  *Commands*:

  × /example: Run the example command.
  × /exampleset `<value>`: Set the example value.

example_success_message: "Example command executed successfully!"
example_value_set: "Example value set to: %s"
example_not_enabled: "Example feature is not enabled in this chat."
example_confirmed: "Action confirmed!"
example_cancelled: "Action cancelled."
```

Add to other locale files (es.yml, fr.yml, hi.yml, id.yml, pt.yml, ru.yml) with appropriate translations.

The help key must be named `<module>_help_msg`; /help and the per-command help read only that name. Every command keeps exactly one `× /cmd …` entry in it in all 7 locales (`TestEveryHelpCommandHasItsLineInEveryLocale` enforces this for modules in `commandHelpModules`), and sample calls for those commands go in `<cmd>_example` keys in `locales/config.yml`.

:::caution[Translation completeness]
You must add keys to ALL locale files, not just `en.yml`. Missing keys cause runtime panics or empty strings. Run `make check-translations` to detect missing keys before committing.
:::

### Step 5: Register Module

Modules self-register via `init()` using the registry system. `LoadModules` in `alita/main.go` calls `modules.LoadAllModules` (defined in `alita/modules/registry.go`), which loads all registered modules in priority order:

```go
func LoadModules(dispatcher *ext.Dispatcher) {
    modules.ResetHelpRegistry()
    defer modules.LoadHelp(dispatcher)

    // Loads all modules registered via RegisterLegacyModule
    modules.LoadAllModules(dispatcher)
}
```

There is one registration pattern:

- **`RegisterLegacyModule(name, priority, loadFunc)`** in `init()` — wraps existing `LoadXxx(dispatcher)` functions. There is no newer `RegisterModule` interface; `RegisterLegacyModule` is the only registry entry point.

:::note[Load order matters]
Priority determines load order (lower numbers load first). Place your module at a priority after any modules it depends on (e.g., after `Users` if you need user lookups). `LoadHelp` is deferred and always loads last.
:::

### Step 6: Regenerate Docs and Validate

Run `make generate-docs` (the command pages are generated from `locales/en.yml`), then `make check-translations`, `make check-docs`, `make test` and `make lint`.

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

Inside `WrapCommand`, use the `helpers.*` CheckFunc builders in `RequiredChecks` instead. The raw `chat_status` predicates below are for watchers and legacy handlers, and they do not reply on failure (see above):

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

// Ends this group only; later groups still run
return nil
```

### When to Use Each

| Return | Use When |
|--------|----------|
| `ext.EndGroups` | Command handled successfully, stop processing |
| `ext.ContinueGroups` | Allow other handlers to also process this update |
| `err` | Something went wrong, let error handler deal with it |
| `nil` | Ends only the current group; later handler groups still run on the update. Not the same as `ext.EndGroups`: a group-0 watcher returning `nil` keeps every later group-0 handler from running |

:::tip[When to use ContinueGroups]
Use `ext.ContinueGroups` only for monitoring/watcher handlers that observe traffic without consuming it, in whatever group they sit (the watcher groups are -10, -6, -5, -2, -1 and 3-11, plus any watcher in group 0). A group-0 watcher must return `ext.ContinueGroups`, not `nil`, or later group-0 handlers never run. Standard command handlers should always return `ext.EndGroups` to prevent duplicate processing.
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
    "github.com/PaulSonOfLars/gotgbot/v2"
    "github.com/PaulSonOfLars/gotgbot/v2/ext"
    log "github.com/sirupsen/logrus"

    "github.com/divkix/Alita_Robot/alita/db/greetings"
    "github.com/divkix/Alita_Robot/alita/db/lang"
    "github.com/divkix/Alita_Robot/alita/i18n"
    "github.com/divkix/Alita_Robot/alita/utils/formatting"
    "github.com/divkix/Alita_Robot/alita/utils/helpers"
)

var welcomeModule = moduleStruct{moduleName: "Welcome"}

var (
    welcomeStatusDesc = helpers.CommandDescriptor{
        Name:           "welcomestatus",
        Disableable:    true,
        RequiredChecks: []helpers.CheckFunc{helpers.CheckDisabled("welcomestatus"), helpers.RequireGroup()},
    }
    toggleWelcomeDesc = helpers.CommandDescriptor{
        Name:           "togglewelcome",
        RequiredChecks: []helpers.CheckFunc{helpers.RequireGroup(), helpers.RequireUserAdmin()},
    }
)

// welcomestatus shows whether welcome messages are enabled
func (m moduleStruct) welcomestatus(b *gotgbot.Bot, ctx *ext.Context) error {
    chat := ctx.EffectiveChat
    msg := ctx.EffectiveMessage
    tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

    settings := greetings.GetGreetingSettings(chat.Id)
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
    _, err := msg.Reply(b, text, formatting.Shtml())
    if err != nil {
        log.Error(err)
        return err
    }

    return ext.EndGroups
}

// togglewelcome enables or disables welcome messages (admin only)
func (m moduleStruct) togglewelcome(b *gotgbot.Bot, ctx *ext.Context) error {
    chat := ctx.EffectiveChat
    msg := ctx.EffectiveMessage
    tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

    settings := greetings.GetGreetingSettings(chat.Id)
    current := false
    if settings.WelcomeSettings != nil {
        current = settings.WelcomeSettings.ShouldWelcome
    }

    if err := greetings.SetWelcomeToggle(chat.Id, !current); err != nil {
        log.Error(err)
        return err
    }

    var text string
    if !current {
        text, _ = tr.GetString("welcome_toggle_enabled")
    } else {
        text, _ = tr.GetString("welcome_toggle_disabled")
    }
    _, err := msg.Reply(b, text, formatting.Shtml())
    if err != nil {
        log.Error(err)
        return err
    }

    return ext.EndGroups
}

func LoadWelcome(dispatcher *ext.Dispatcher) {
    SetModuleEnabled(welcomeModule.moduleName, true)

    helpers.WrapCommand(dispatcher, welcomeStatusDesc, pipelineHandler(welcomeModule.welcomestatus))
    helpers.WrapCommand(dispatcher, toggleWelcomeDesc, pipelineHandler(welcomeModule.togglewelcome))
}

func init() {
    RegisterLegacyModule("Welcome", 210, LoadWelcome)
    RegisterAnonymousAdminHandler("togglewelcome", anonPipelineHandler(toggleWelcomeDesc, welcomeModule.togglewelcome))
}
```

## Security Considerations

### HTML Escaping

When displaying user-controlled data in HTML-formatted messages, always escape it:

```go
import "github.com/divkix/Alita_Robot/alita/utils/formatting"

// Escape chat titles, usernames, and user-supplied text
text := fmt.Sprintf("Settings for %s", formatting.HtmlEscape(chat.Title))
```

The `formatting.MentionHtml()` function already handles escaping for user names.

:::caution[XSS via Telegram HTML]
Telegram supports a subset of HTML in messages. User-controlled strings (chat titles, usernames, filter keywords) must be escaped with `formatting.HtmlEscape()` before insertion into HTML-formatted messages. Unescaped angle brackets can break message formatting or inject unintended HTML tags.
:::

### Database Operations and User Feedback

**Prefer synchronous operations when sending success confirmations:**

```go
// CORRECT: Synchronous operation before success message
if err := greetings.SetWelcomeText(chat.Id, db.DefaultWelcome, "", nil, db.TEXT); err != nil {
    log.Error(err)
    return err
}
_, err := msg.Reply(b, "Welcome message reset successfully!", formatting.Shtml())
```

```go
// AVOID: Async operation with premature success message
go func() {
    _ = greetings.SetWelcomeText(chat.Id, db.DefaultWelcome, "", nil, db.TEXT) // May fail silently
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
captchaSettings, err := captcha.GetCaptchaSettings(chat.Id)
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
captchaSettings, _ := captcha.GetCaptchaSettings(chat.Id)
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
- [ ] Implement handler methods on module struct (value receivers)
- [ ] Register commands with `helpers.WrapCommand` and a `helpers.CommandDescriptor`; put permission checks in `RequiredChecks`
- [ ] For disableable commands, set `Disableable: true` and add `helpers.CheckDisabled("<cmd>")`
- [ ] Admin commands: `RegisterAnonymousAdminHandler` + `anonPipelineHandler`
- [ ] Use `i18n.MustNewTranslator(lang.GetLanguage(ctx))` for translations
- [ ] Handle errors properly (log and return)
- [ ] **Escape user-controlled input with `formatting.HtmlEscape()`**
- [ ] **Add panic recovery to goroutines**
- [ ] Create the migration and the model in `alita/db/models/` if needed
- [ ] Add the model to the test `AutoMigrate` list
- [ ] Read through `cache.GetFromCacheOrLoad`; `cache.DeleteCache` on every write
- [ ] Add translations to all 7 locale files, including `<module>_help_msg`
- [ ] Register module in `init()` with `RegisterLegacyModule`
- [ ] Enable the module in help with `SetModuleEnabled(name, true)` (never write `AbleMap` directly)
- [ ] Run `make generate-docs`, `make check-translations`, `make check-docs`, `make test`, `make lint`
- [ ] Test in development environment

## Next Steps

- [Request Flow](/architecture/request-flow) - Understanding the update pipeline
- [Caching](/architecture/caching) - Redis cache integration
- [Project Structure](/architecture/project-structure) - Where files belong
