---
title: Inline Button Callbacks
description: Documentation for callback query handling patterns, callback data formats, and inline button implementations.
---

Alita Robot uses Telegram's inline keyboard buttons extensively for interactive features. This document covers callback data patterns, handler registration, and security considerations.

## Callback Data Format

Callback data follows a consistent pattern across modules:

```
{module}_{action}_{parameters}
```

or with dot separators:

```
{module}.{action}.{parameters}
```

Examples:
- `restrict.ban.123456789` - Ban action for user ID 123456789
- `unrestrict.unban.123456789` - Unban action for user ID
- `helpq.Admin` - Help query for Admin module
- `captcha_verify.5.123456789.A` - Captcha verification with attempt ID, user ID, and answer

## Common Callback Patterns

### Anonymous Admin Verification

When an anonymous admin uses a command, Alita presents a verification button:

```
alita:anonAdmin:{chatId}:{messageId}
```

- **chatId**: The chat where the command was issued
- **messageId**: The original message ID for context retrieval

The verification flow:
1. Anonymous admin sends a command (e.g., `/ban`)
2. Bot stores the message context in cache with 20-second expiration
3. Bot presents an inline keyboard with "Prove Admin" button
4. Admin clicks the button
5. Bot verifies admin status via `IsUserAdmin()`
6. If valid, executes the original command with cached context

### Restriction Actions

For ban, kick, and mute operations:

```go
// Restrict command keyboard
restrict.ban.{userId}    // Permanent ban
restrict.kick.{userId}   // Kick (remove from group)
restrict.mute.{userId}   // Mute (restrict sending)

// Unrestrict command keyboard
unrestrict.unban.{userId}   // Remove ban
unrestrict.unmute.{userId}  // Remove mute restrictions
```

### Help Navigation

Help menu uses a hierarchical callback structure:

```go
helpq.{module}     // Show help for specific module
helpq.Help         // Return to main help menu
helpq.BackStart    // Return to start menu
helpq.Languages    // Show language selection
```

### Settings Toggles

Settings and configuration callbacks:

```go
configuration.step1   // First setup step
configuration.step2   // Second setup step
configuration.step3   // Third setup step
about.main            // Main about section
about.me              // Bot information
```

### Confirmation Dialogs

For destructive operations, confirmation patterns:

```go
rmAllFilters.yes     // Confirm remove all filters
rmAllFilters.no      // Cancel operation
rmAllNotes.yes       // Confirm remove all notes
rmAllNotes.no        // Cancel operation
rmAllBlacklist.yes   // Confirm remove all blacklist entries
rmAllBlacklist.no    // Cancel operation
rmAllChatWarns.yes   // Confirm remove all warnings
rmAllChatWarns.no    // Cancel operation
unpinallbtn(yes)     // Confirm unpin all messages
unpinallbtn(no)      // Cancel operation
```

### Pagination

For list navigation:

```go
helpq.{ModuleName}   // Navigate to module help page
```

### Language Selection

```go
change_language.{langCode}   // e.g., change_language.en, change_language.de
```

### Connection Management

```go
connbtns.Main    // Main connection menu
connbtns.Admin   // Admin connection options
connbtns.User    // User connection options
```

### Captcha Verification

```go
captcha_verify.{attemptId}.{userId}.{answer}  // Submit captcha answer
captcha_refresh.{attemptId}.{userId}          // Request new captcha
```

### Report Actions

```go
report.{chatId}.{userId}.kick      // Kick reported user
report.{chatId}.{userId}.ban       // Ban reported user
report.{chatId}.{userId}.delete    // Delete reported message
report.{chatId}.{userId}.resolved  // Mark report as resolved
```

### Warn Management

```go
rmWarn.{userId}   // Remove a warning from user
```

### Join Request Handling

```go
join_request.accept.{userId}   // Accept join request
join_request.decline.{userId}  // Decline join request
join_request.ban.{userId}      // Ban user who requested
```

### Content Management

```go
filters_overwrite.{filterWord}  // Overwrite existing filter
filters_overwrite.cancel        // Cancel overwrite
notes.overwrite.yes.{noteName}  // Confirm note overwrite
notes.overwrite.no.{noteName}   // Cancel note overwrite
deleteMsg.{messageId}           // Delete specific message
```

### Formatting Help

```go
formatting.md_formatting   // Show markdown formatting help
formatting.fillings        // Show template fillings help
formatting.random          // Show random content help
```

## Handler Registration

Callback handlers are registered using the `handlers.NewCallback` function with a prefix filter:

```go
import (
    "github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
    "github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
)

func LoadMyModule(dispatcher *ext.Dispatcher) {
    // Register callback handler with prefix filter
    dispatcher.AddHandler(handlers.NewCallback(
        callbackquery.Prefix("myprefix."),
        myModule.myCallbackHandler,
    ))
}
```

Multiple callbacks can be registered for different prefixes:

```go
func LoadBans(dispatcher *ext.Dispatcher) {
    // Restrict actions (ban, kick, mute)
    dispatcher.AddHandler(handlers.NewCallback(
        callbackquery.Prefix("restrict."),
        bansModule.restrictButtonHandler,
    ))

    // Unrestrict actions (unban, unmute)
    dispatcher.AddHandler(handlers.NewCallback(
        callbackquery.Prefix("unrestrict."),
        bansModule.unrestrictButtonHandler,
    ))
}
```

## Callback Handler Pattern

A typical callback handler follows this structure:

```go
func (moduleStruct) myCallbackHandler(b *gotgbot.Bot, ctx *ext.Context) error {
    query := ctx.CallbackQuery
    chat := ctx.EffectiveChat
    user := ctx.EffectiveSender.User

    // 1. Parse callback data
    args := strings.Split(query.Data, ".")
    action := args[1]
    userId, _ := strconv.Atoi(args[2])

    // 2. Check permissions
    if !chat_status.CanUserRestrict(b, ctx, chat, user.Id, false) {
        return ext.EndGroups
    }

    // 3. Process the action
    switch action {
    case "ban":
        _, err := chat.BanMember(b, int64(userId), nil)
        if err != nil {
            log.Error(err)
            return err
        }
    case "kick":
        // Handle kick action
    }

    // 4. Update the message (optional)
    _, _, err := query.Message.EditText(b,
        "Action completed!",
        &gotgbot.EditMessageTextOpts{
            ParseMode: helpers.HTML,
        },
    )
    if err != nil {
        log.Error(err)
        return err
    }

    // 5. Answer the callback query
    _, err = query.Answer(b, nil)
    if err != nil {
        log.Error(err)
        return err
    }

    return ext.EndGroups
}
```

## Callback Answer Types

### Simple Acknowledgment

Silent acknowledgment without visible feedback:

```go
_, err := query.Answer(b, nil)
```

### Toast Message

Brief notification that appears at the top of the screen:

```go
_, err := query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
    Text: "Action completed!",
})
```

### Alert Popup

Modal dialog requiring user acknowledgment:

```go
_, err := query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
    Text:      "Important: This action cannot be undone!",
    ShowAlert: true,
})
```

### URL Redirect

Open a URL when the button is pressed:

```go
_, err := query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
    Url: "https://example.com/result",
})
```

## Creating Inline Keyboards

### Single Button

```go
keyboard := gotgbot.InlineKeyboardMarkup{
    InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
        {
            {
                Text:         "Click Me",
                CallbackData: "action.data",
            },
        },
    },
}
```

### Multiple Buttons in Row

```go
keyboard := gotgbot.InlineKeyboardMarkup{
    InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
        {
            {Text: "Ban", CallbackData: fmt.Sprintf("restrict.ban.%d", userId)},
            {Text: "Kick", CallbackData: fmt.Sprintf("restrict.kick.%d", userId)},
        },
    },
}
```

### Multiple Rows

```go
keyboard := gotgbot.InlineKeyboardMarkup{
    InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
        {
            {Text: "Ban", CallbackData: fmt.Sprintf("restrict.ban.%d", userId)},
            {Text: "Kick", CallbackData: fmt.Sprintf("restrict.kick.%d", userId)},
        },
        {
            {Text: "Mute", CallbackData: fmt.Sprintf("restrict.mute.%d", userId)},
        },
    },
}
```

### With URL Button (No Callback)

```go
keyboard := gotgbot.InlineKeyboardMarkup{
    InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
        {
            {
                Text: "Visit Website",
                Url:  "https://example.com",
            },
        },
    },
}
```

## Security Considerations

### Permission Verification

Always verify user permissions before processing callback actions:

```go
func (moduleStruct) restrictButtonHandler(b *gotgbot.Bot, ctx *ext.Context) error {
    query := ctx.CallbackQuery
    chat := ctx.EffectiveChat
    user := ctx.EffectiveSender.User

    // Verify user has restrict permission
    if !chat_status.CanUserRestrict(b, ctx, chat, user.Id, false) {
        _, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
            Text:      "You don't have permission to do this!",
            ShowAlert: true,
        })
        return ext.EndGroups
    }

    // Process action...
}
```

### Data Validation

Always validate callback data format and parameters:

```go
args := strings.Split(query.Data, ".")
if len(args) < 3 {
    log.Error("Invalid callback data format")
    return ext.EndGroups
}

userId, err := strconv.Atoi(args[2])
if err != nil {
    log.Error("Invalid user ID in callback data")
    return ext.EndGroups
}
```

### Expiration for Sensitive Actions

Use cache with expiration for time-sensitive callbacks:

```go
// Set cache with 20-second expiration
err := cache.Marshal.Set(
    cache.Context,
    fmt.Sprintf("alita:anonAdmin:%d:%d", chatId, msg.MessageId),
    msg,
    store.WithExpiration(20 * time.Second),
)
```

### Prevent Button Reuse

For one-time actions, edit the message to remove buttons after processing:

```go
// After processing the action
_, _, err := query.Message.EditText(b,
    "Action completed. This message cannot be used again.",
    &gotgbot.EditMessageTextOpts{
        ParseMode: helpers.HTML,
        // No ReplyMarkup removes the keyboard
    },
)
```

### Cross-Chat Validation

For callbacks that reference specific chats, verify the action is in the correct context:

```go
args := strings.Split(query.Data, ".")
chatId, _ := strconv.ParseInt(args[1], 10, 64)

// Verify the callback is from the correct chat
if chat.Id != chatId {
    _, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
        Text:      "This button is for a different chat!",
        ShowAlert: true,
    })
    return ext.EndGroups
}
```

## Best Practices

1. **Keep callback data short**: Telegram limits callback data to 64 bytes
2. **Use consistent separators**: Choose either `.` or `_` and stick with it
3. **Include context IDs**: Add chat/user/message IDs to prevent cross-context misuse
4. **Always answer callbacks**: Unanswered callbacks show a loading indicator indefinitely
5. **Handle expiration gracefully**: Provide user feedback when cached data expires
6. **Use descriptive prefixes**: Makes filtering and debugging easier
7. **Validate all parsed data**: Never trust callback data blindly
