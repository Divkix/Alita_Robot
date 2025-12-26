---
title: Permission System
description: Documentation for the permission checking system, admin verification, and special Telegram ID handling.
---

Alita Robot implements a comprehensive permission system to ensure commands are only executed by authorized users. This document covers permission check functions, the anonymous admin system, and special Telegram accounts.

## Permission Check Functions

The `chat_status` package provides functions to verify user and bot permissions. All functions accept a `justCheck` parameter:
- `true`: Only check permissions, don't send error messages
- `false`: Send localized error messages to the user if check fails

### Complete Function Reference

| Function | Checks | Returns | Error Behavior |
|----------|--------|---------|----------------|
| `IsUserAdmin` | User is admin or creator | `bool` | None (silent check) |
| `RequireUserAdmin` | User is admin or creator | `bool` | Sends error message if false |
| `RequireUserOwner` | User is chat creator | `bool` | Sends error message if false |
| `RequireBotAdmin` | Bot is admin | `bool` | Sends error message if false |
| `IsBotAdmin` | Bot is admin | `bool` | None (silent check) |
| `CanUserRestrict` | User can restrict members | `bool` | Sends error message if false |
| `CanBotRestrict` | Bot can restrict members | `bool` | Sends error message if false |
| `CanUserDelete` | User can delete messages | `bool` | Sends error message if false |
| `CanBotDelete` | Bot can delete messages | `bool` | Sends error message if false |
| `CanUserPin` | User can pin messages | `bool` | Sends error message if false |
| `CanBotPin` | Bot can pin messages | `bool` | Sends error message if false |
| `CanUserPromote` | User can promote/demote | `bool` | Sends error message if false |
| `CanBotPromote` | Bot can promote/demote | `bool` | Sends error message if false |
| `CanUserChangeInfo` | User can change chat info | `bool` | Sends error message if false |
| `Caninvite` | Both can create invites | `bool` | Sends error message if false |
| `RequireGroup` | Chat is group/supergroup | `bool` | Sends error message if false |
| `RequirePrivate` | Chat is private | `bool` | Sends error message if false |
| `IsUserInChat` | User is member of chat | `bool` | None (silent check) |
| `IsUserBanProtected` | User cannot be banned | `bool` | None (silent check) |

### Function Signatures

```go
// Silent checks (no error messages)
func IsUserAdmin(b *gotgbot.Bot, chatID, userId int64) bool
func IsBotAdmin(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool
func IsUserInChat(b *gotgbot.Bot, chat *gotgbot.Chat, userId int64) bool
func IsUserBanProtected(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool

// Requirement checks (send error messages when justCheck=false)
func RequireUserAdmin(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64, justCheck bool) bool
func RequireUserOwner(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64, justCheck bool) bool
func RequireBotAdmin(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, justCheck bool) bool
func RequireGroup(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, justCheck bool) bool
func RequirePrivate(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, justCheck bool) bool

// Permission checks (send error messages when justCheck=false)
func CanUserRestrict(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64, justCheck bool) bool
func CanBotRestrict(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, justCheck bool) bool
func CanUserDelete(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64, justCheck bool) bool
func CanBotDelete(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, justCheck bool) bool
func CanUserPin(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64, justCheck bool) bool
func CanBotPin(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, justCheck bool) bool
func CanUserPromote(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64, justCheck bool) bool
func CanBotPromote(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, justCheck bool) bool
func CanUserChangeInfo(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64, justCheck bool) bool
func Caninvite(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, msg *gotgbot.Message, justCheck bool) bool
```

## Command Permission Matrix

Each command requires specific permissions. Here's the breakdown by module:

### Admin Module

| Command | User Permission | Bot Permission |
|---------|-----------------|----------------|
| `/promote` | CanPromoteMembers | CanPromoteMembers |
| `/demote` | CanPromoteMembers | CanPromoteMembers |
| `/title` | CanPromoteMembers | CanPromoteMembers |
| `/adminlist` | None (any user) | None |

### Bans Module

| Command | User Permission | Bot Permission |
|---------|-----------------|----------------|
| `/ban` | CanRestrictMembers | CanRestrictMembers |
| `/sban` | CanRestrictMembers + CanDeleteMessages | CanRestrictMembers + CanDeleteMessages |
| `/tban` | CanRestrictMembers | CanRestrictMembers |
| `/dban` | CanRestrictMembers + CanDeleteMessages | CanRestrictMembers + CanDeleteMessages |
| `/unban` | CanRestrictMembers | CanRestrictMembers |
| `/kick` | CanRestrictMembers | CanRestrictMembers |
| `/dkick` | CanRestrictMembers + CanDeleteMessages | CanRestrictMembers + CanDeleteMessages |
| `/kickme` | None (self-service) | CanRestrictMembers |
| `/restrict` | CanRestrictMembers | CanRestrictMembers |
| `/unrestrict` | CanRestrictMembers | CanRestrictMembers |

### Mutes Module

| Command | User Permission | Bot Permission |
|---------|-----------------|----------------|
| `/mute` | CanRestrictMembers | CanRestrictMembers |
| `/tmute` | CanRestrictMembers | CanRestrictMembers |
| `/unmute` | CanRestrictMembers | CanRestrictMembers |

### Pins Module

| Command | User Permission | Bot Permission |
|---------|-----------------|----------------|
| `/pin` | CanPinMessages | CanPinMessages |
| `/unpin` | CanPinMessages | CanPinMessages |
| `/unpinall` | Creator only | CanPinMessages |
| `/permapin` | CanPinMessages | CanPinMessages |

### Purges Module

| Command | User Permission | Bot Permission |
|---------|-----------------|----------------|
| `/purge` | CanDeleteMessages | CanDeleteMessages |
| `/del` | CanDeleteMessages | CanDeleteMessages |
| `/spurge` | CanDeleteMessages | CanDeleteMessages |

### Warns Module

| Command | User Permission | Bot Permission |
|---------|-----------------|----------------|
| `/warn` | CanRestrictMembers | CanRestrictMembers |
| `/dwarn` | CanRestrictMembers + CanDeleteMessages | CanRestrictMembers + CanDeleteMessages |
| `/swarn` | CanRestrictMembers | CanRestrictMembers |
| `/resetwarns` | CanRestrictMembers | None |
| `/warns` | Admin status | None |
| `/rmwarn` | CanRestrictMembers | None |
| `/warnlimit` | Admin status | None |
| `/warnmode` | Admin status | None |

### Settings Module

| Command | User Permission | Bot Permission |
|---------|-----------------|----------------|
| `/setlang` | Admin status | None |
| `/settings` | Admin status | None |

## Usage Examples

### Basic Permission Check

```go
func (m moduleStruct) ban(b *gotgbot.Bot, ctx *ext.Context) error {
    chat := ctx.EffectiveChat
    user := ctx.EffectiveSender.User
    msg := ctx.EffectiveMessage

    // Check if command is used in a group
    if !chat_status.RequireGroup(b, ctx, nil, false) {
        return ext.EndGroups
    }

    // Check if user is admin
    if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id, false) {
        return ext.EndGroups
    }

    // Check if bot is admin
    if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
        return ext.EndGroups
    }

    // Check specific permissions
    if !chat_status.CanUserRestrict(b, ctx, nil, user.Id, false) {
        return ext.EndGroups
    }
    if !chat_status.CanBotRestrict(b, ctx, nil, false) {
        return ext.EndGroups
    }

    // Proceed with ban logic...
}
```

### Silent Permission Check

```go
// For conditional logic without error messages
if chat_status.IsUserAdmin(b, chat.Id, user.Id) {
    // User is admin, show admin-only options
} else {
    // User is not admin, show regular options
}
```

### Target User Validation

```go
// Check if target user can be banned
if chat_status.IsUserBanProtected(b, ctx, nil, targetUserId) {
    _, _ = msg.Reply(b, "This user cannot be banned!", nil)
    return ext.EndGroups
}
```

## Anonymous Admin Handling

Telegram groups can have "anonymous admins" - admins who post as the group itself rather than their personal account. Alita implements a verification system for these users.

### Special User ID

```go
const groupAnonymousBot = 1087968824  // Group Anonymous Bot ID
```

When a message is from an anonymous admin, `ctx.EffectiveSender.User.Id` equals `1087968824`.

### Detection

```go
func (sender *Sender) IsAnonymousAdmin() bool {
    return sender.User != nil && sender.User.Id == 1087968824
}
```

### Verification Flow

1. Anonymous admin issues a command
2. Bot detects anonymous sender via `sender.IsAnonymousAdmin()`
3. Bot checks `AnonAdmin` setting in database:
   - If bypass enabled: Trust the anonymous admin, proceed with command
   - If bypass disabled: Present verification keyboard
4. Message context is cached with 20-second expiration
5. Inline keyboard is shown with "Prove Admin" button
6. When clicked, callback handler:
   - Verifies clicker is actually an admin
   - Retrieves cached message context
   - Executes the original command

### Verification Keyboard

```go
func sendAnonAdminKeyboard(b *gotgbot.Bot, msg *gotgbot.Message, chat *gotgbot.Chat) (*gotgbot.Message, error) {
    return msg.Reply(b,
        "Please verify you are an admin",
        &gotgbot.SendMessageOpts{
            ReplyMarkup: gotgbot.InlineKeyboardMarkup{
                InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
                    {{
                        Text:         "Prove Admin Status",
                        CallbackData: fmt.Sprintf("alita:anonAdmin:%d:%d", chat.Id, msg.MessageId),
                    }},
                },
            },
        },
    )
}
```

### Cache Management

```go
var anonChatMapExpirartion = 20 * time.Second

func setAnonAdminCache(chatId int64, msg *gotgbot.Message) {
    err := cache.Marshal.Set(
        cache.Context,
        fmt.Sprintf("alita:anonAdmin:%d:%d", chatId, msg.MessageId),
        msg,
        store.WithExpiration(anonChatMapExpirartion),
    )
    if err != nil {
        log.Errorf("Failed to set anonymous admin cache: %v", err)
    }
}
```

### AnonAdmin Bypass Setting

Groups can enable "AnonAdmin bypass" to skip verification:

```go
if db.GetAdminSettings(chat.Id).AnonAdmin {
    return true, true  // Trust anonymous admin
}
```

When enabled, anonymous admins are trusted without clicking the verification button.

## Special Telegram IDs

Certain Telegram user IDs have special significance and require special handling:

### System Accounts

| ID | Account | Description |
|----|---------|-------------|
| `1087968824` | Group Anonymous Bot | Anonymous admin posts |
| `777000` | Telegram | System notifications, forwarded channel posts |
| `136817688` | Channel Bot | Users sending as channels (deprecated) |

### Code Constants

```go
const (
    groupAnonymousBot = 1087968824
    tgUserId          = 777000
)

var tgAdminList = []int64{groupAnonymousBot, tgUserId}
```

### Ban Protection Logic

These special accounts cannot be banned:

```go
func IsUserBanProtected(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool {
    // Protected in private chats
    if chat.Type == "private" {
        return true
    }

    // Admins are ban-protected
    if IsUserAdmin(b, ctx.EffectiveChat.Id, userId) {
        return true
    }

    // Special Telegram accounts are ban-protected
    if string_handling.FindInInt64Slice(tgAdminList, userId) {
        return true
    }

    return false
}
```

### ID Validation

```go
// Check if ID is a valid user (not channel or group)
func IsValidUserId(id int64) bool {
    return id > 0  // User IDs are always positive
}

// Check if ID is a channel
func IsChannelId(id int64) bool {
    return id < -1000000000000  // Channel format: -100XXXXXXXXXX
}
```

## Admin Cache System

To reduce API calls, admin lists are cached:

### Cache Lookup

```go
func IsUserAdmin(b *gotgbot.Bot, chatID, userId int64) bool {
    // Check special accounts first
    if string_handling.FindInInt64Slice(tgAdminList, userId) {
        return true
    }

    // Try cache first
    adminsAvail, admins := cache.GetAdminCacheList(chatID)
    if adminsAvail && admins.Cached {
        for i := range admins.UserInfo {
            if admins.UserInfo[i].User.Id == userId {
                return true
            }
        }
        return false
    }

    // Cache miss - load from API
    adminList := cache.LoadAdminCache(b, chatID)
    // Check admin list...
}
```

### Cache Auto-Update

When admin status changes in a chat, the cache is automatically refreshed:

```go
func adminCacheAutoUpdate(b *gotgbot.Bot, ctx *ext.Context) error {
    chat := ctx.EffectiveChat

    adminsAvail, _ := cache.GetAdminCacheList(chat.Id)
    if !adminsAvail {
        cache.LoadAdminCache(b, chat.Id)
        log.Info(fmt.Sprintf("Reloaded admin cache for %d (%s)", chat.Id, chat.Title))
    }

    return ext.ContinueGroups
}
```

## Error Handling

### Command Context Errors

Permission functions handle both command and callback contexts:

```go
func CanUserRestrict(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64, justCheck bool) bool {
    // ... permission check ...

    if !userMember.CanRestrictMembers && userMember.Status != "creator" {
        query := ctx.CallbackQuery
        if query != nil {
            // Error via callback answer
            if !justCheck {
                _, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
                    Text: "You don't have permission to restrict users!",
                })
            }
            return false
        }

        // Error via message reply
        if !justCheck {
            _, _ = b.SendMessage(chat.Id, "You need restrict permission!", nil)
        }
        return false
    }
    return true
}
```

### Context Extraction

Functions automatically extract chat from various context types:

```go
func extractChatFromContext(ctx *ext.Context, chat *gotgbot.Chat) *gotgbot.Chat {
    if chat != nil {
        return chat
    }
    if ctx.CallbackQuery != nil {
        chatValue := ctx.CallbackQuery.Message.GetChat()
        return &chatValue
    }
    if ctx.Message != nil {
        return &ctx.Message.Chat
    }
    if ctx.MyChatMember != nil {
        return &ctx.MyChatMember.Chat
    }
    return nil
}
```

## Best Practices

1. **Check group context first**: Use `RequireGroup` before other checks to fail fast
2. **Check bot permissions early**: Verify bot can perform action before processing
3. **Use silent checks for conditional logic**: Pass `justCheck=true` for non-critical checks
4. **Handle anonymous admins**: All permission checks automatically handle anonymous admin verification
5. **Validate target users**: Always check `IsUserBanProtected` before restriction actions
6. **Cache awareness**: Rely on the cache system but be aware of propagation delays
7. **Creator vs Admin**: Use `RequireUserOwner` for destructive operations like `/unpinall`
