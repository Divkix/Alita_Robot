---
title: Permission System
description: Complete reference of permission checking functions
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# 🔐 Permission System

This page documents all permission checking functions in Alita Robot.

## Overview

- **Total Functions**: 35
- **Location**: `alita/utils/chat_status/ (chat_status.go, access.go, update_memo.go, permission_responder.go)`

## Function Summary

| Function | Returns | Description |
|----------|---------|-------------|
| `CanBotDelete` | `bool` | CanBotDelete checks if the bot has permission to delete m... |
| `CanBotPin` | `bool` | CanBotPin checks if the bot has permission to pin message... |
| `CanBotPromote` | `bool` | CanBotPromote checks if the bot has permission to promote... |
| `CanBotRestrict` | `bool` | CanBotRestrict checks if the bot has permission to restri... |
| `CanBotChangeInfo` | `bool` | CanBotChangeInfo checks the bot can change chat info |
| `CanBotInvite` | `bool` | CanBotInvite checks the bot can invite users |
| `IsBotAdmin` | `bool` | IsBotAdmin checks if the bot has administrator privileges... |
| `IsChannelId` | `bool` | IsChannelId checks if an ID represents a Telegram channel... |
| `IsValidUserId` | `bool` | IsValidUserId checks if an ID represents a valid Telegram... |
| `IsApproved` | `bool` | IsApproved checks if a user is in the approved whitelist ... |
| `IsApprovedForUpdate` | `bool` | IsApproved memoised for the current update (watchers only) |
| `RequireBotAdmin` | `bool` | RequireBotAdmin ensures the bot has administrator privile... |
| `RequireGroup` | `bool` | RequireGroup ensures the command is being used in a group... |
| `RequirePrivate` | `bool` | RequirePrivate ensures the command is being used in a pri... |
| `RequireUserAdmin` | `bool` | RequireUserAdmin ensures a user has administrator privile... |
| `RequireUserOwner` | `bool` | RequireUserOwner ensures a user is the chat creator/owner... |
| `RequireUser` | `*gotgbot.User` | RequireUser extracts the effective user from the update context safely... |
| `CanInvite` | `bool` | CanInvite checks if the bot and user have permissions to ... |
| `CanUserInvite` | `bool` | CanUserInvite checks a user has CanInviteUsers |
| `CanUserChangeInfo` | `bool` | CanUserChangeInfo checks if a user has permission to chan... |
| `CanUserDelete` | `bool` | CanUserDelete checks if a user has permission to delete m... |
| `CanUserPin` | `bool` | CanUserPin checks if a user has permission to pin message... |
| `CanUserPromote` | `bool` | CanUserPromote checks if a user has permission to promote... |
| `CanUserRestrict` | `bool` | CanUserRestrict checks if a user has permission to restri... |
| `IsUserAdmin` | `bool` | IsUserAdmin checks if a user has administrator privileges... |
| `IsUserAdminCached` | `(bool, bool)` | Answers from the stored admin list only; known=false when there is no usable list |
| `IsUserAdminWithContext` | `(bool, error)` | IsUserAdmin bounded by a context |
| `IsUserAdminForUpdate` | `bool` | IsUserAdmin memoised for the current update (watchers only) |
| `IsUserBanProtected` | `bool` | IsUserBanProtected checks if a user is protected from bei... |
| `IsUserInChat` | `bool` | IsUserInChat checks if a user is currently a member of th... |
| `IsUserInChatWithError` | `(bool, error)` | Membership check that reports lookup errors separately |
| `IsUserConnected` | `*gotgbot.Chat` | Resolves the chat a command acts on (the connected group in PM) |
| `GetEffectiveUser` | `*gotgbot.User` | GetEffectiveUser safely extracts the user from the context without nil panics... |
| `GetChat` | `*gotgbot.Chat` | GetChat safely retrieves a chat by ID with caching... |
| `CheckDisabledCmd` | `bool` | CheckDisabledCmd checks if a command is disabled in the c... |

## Functions by Category

### 🤖 Bot Permission Checks

#### `CanBotDelete`

```go
func CanBotDelete(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool
```

CanBotDelete checks if the bot has permission to delete messages in the chat. Validates the bot's CanDeleteMessages permission.

**Parameters:**
- `b`
- `ctx`
- `chat`

#### `CanBotPin`

```go
func CanBotPin(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool
```

CanBotPin checks if the bot has permission to pin messages in the chat. Validates the bot's CanPinMessages permission.

**Parameters:**
- `b`
- `ctx`
- `chat`

#### `CanBotPromote`

```go
func CanBotPromote(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool
```

CanBotPromote checks if the bot has permission to promote/demote members in the chat. Validates the bot's CanPromoteMembers permission.

**Parameters:**
- `b`
- `ctx`
- `chat`

#### `CanBotRestrict`

```go
func CanBotRestrict(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool
```

CanBotRestrict checks if the bot has permission to restrict members in the chat. Validates the bot's CanRestrictMembers permission.

**Parameters:**
- `b`
- `ctx`
- `chat`

#### `CanBotChangeInfo`

```go
func CanBotChangeInfo(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool
```

CanBotChangeInfo checks if the bot has permission to change chat information. Validates the bot's CanChangeInfo permission.

**Parameters:**
- `b`
- `ctx`
- `chat`

#### `CanBotInvite`

```go
func CanBotInvite(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool
```

CanBotInvite checks if the bot has permission to invite users to the chat. Validates the bot's CanInviteUsers permission.

**Parameters:**
- `b`
- `ctx`
- `chat`

#### `IsBotAdmin`

```go
func IsBotAdmin(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool
```

IsBotAdmin checks if the bot has administrator privileges in the specified chat. Returns true for private chats (bot is always "admin" in private). For groups, verifies the bot's actual admin status.

**Parameters:**
- `b`
- `ctx`
- `chat`

### PermissionResponder

Located in `alita/utils/chat_status/permission_responder.go`.

Centralizes permission-failure messaging with support for callback-query answers and chat replies.

```go
responder := chat_status.NewPermissionResponder(b)

// Answer a callback, or send cmdKey's text as a plain message
responder.Respond(ctx, "chat_status_owner_cmd_error", "chat_status_owner_button_error")

// Reply to the original message instead
responder.Respond(ctx, cmdKey, btnKey, chat_status.WithReply())
```

**Functions**:

| Function | Description |
|----------|-------------|
| `NewPermissionResponder(b)` | Creates a responder |
| `Respond(ctx, cmdKey, btnKey, opts...)` | Sends the failure message (btnKey's text as a callback answer, cmdKey's text otherwise) |
| `WithReply()` | Option: send as a reply to the original message |
| `WithReplyFallback()` | Option: reply, falling back to a plain message if the reply fails |
| `WithTranslator(tr)` | Option: use tr instead of the chat's language |

### 🔢 ID Validation

#### `IsChannelId`

```go
func IsChannelId(id int64) bool
```

IsChannelId checks if an ID represents a Telegram channel. Channel IDs have the format -100XXXXXXXXXX (-100 prefix followed by 10+ digits).

**Parameters:**
- `id`

#### `IsValidUserId`

```go
func IsValidUserId(id int64) bool
```

IsValidUserId checks if an ID represents a valid Telegram user. User IDs are always positive (> 0). Channel IDs are negative with format -100XXXXXXXXXX (< -1000000000000). Regular chat/group IDs are negative but in a different range.

**Parameters:**
- `id`

### 📋 Other

#### `IsApproved`

```go
func IsApproved(b *gotgbot.Bot, chatID, userID int64) bool
```

IsApproved checks if a user is in the approved whitelist for a chat. Approved users are immune to anti-spam measures (antiflood, antiraid, AI spam filter, blacklists, locks, captcha). This is a simple delegation to the DB layer for consistent usage in watcher handlers.

**Parameters:**
- `b`
- `chatID`
- `userID`

### ✅ Requirement Checks

#### `RequireBotAdmin`

```go
func RequireBotAdmin(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool
```

RequireBotAdmin ensures the bot has administrator privileges in the chat. Uses IsBotAdmin internally to perform the check.

**Parameters:**
- `b`
- `ctx`
- `chat`

#### `RequireGroup`

```go
func RequireGroup(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool
```

RequireGroup ensures the command is being used in a group chat. Returns false for private chats.  nolint:dupl // RequirePrivate/RequireGroup have symmetric logic

**Parameters:**
- `b`
- `ctx`
- `chat`

#### `RequirePrivate`

```go
func RequirePrivate(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool
```

RequirePrivate ensures the command is being used in a private chat. Returns false for group chats and supergroups.  nolint:dupl // RequirePrivate/RequireGroup have symmetric logic

**Parameters:**
- `b`
- `ctx`
- `chat`

#### `RequireUserAdmin`

```go
func RequireUserAdmin(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool
```

RequireUserAdmin ensures a user has administrator privileges in the chat. Uses IsUserAdmin internally to perform the check.

**Parameters:**
- `b`
- `ctx`
- `chat`
- `userId`

#### `RequireUserOwner`

```go
func RequireUserOwner(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool
```

RequireUserOwner ensures a user is the chat creator/owner. Checks for "creator" status specifically, not just administrator.

**Parameters:**
- `b`
- `ctx`
- `chat`
- `userId`

#### `RequireUser`

```go
func RequireUser(b *gotgbot.Bot, ctx *ext.Context) *gotgbot.User
```

RequireUser extracts the effective user from the update context. Returns nil for channel messages where `ctx.EffectiveSender` is nil. Always check the return value before accessing fields.

**Parameters:**
- `b`
- `ctx`

### 👮 User Permission Checks

#### `CanInvite`

```go
func CanInvite(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, msg *gotgbot.Message) bool
```

CanInvite checks if the bot and user have permissions to generate invite links. Returns true immediately if the chat has a public username. Validates both bot and user permissions for invite link generation.

**Parameters:**
- `b`
- `ctx`
- `chat`
- `msg`

#### `CanUserInvite`

```go
func CanUserInvite(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool
```

CanUserInvite checks if a user has permission to invite users to the chat. Validates the CanInviteUsers permission.

**Parameters:**
- `b`
- `ctx`
- `chat`
- `userId`

#### `CanUserChangeInfo`

```go
func CanUserChangeInfo(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool
```

CanUserChangeInfo checks if a user has permission to change chat information. Handles anonymous admins and validates the CanChangeInfo permission.

**Parameters:**
- `b`
- `ctx`
- `chat`
- `userId`

#### `CanUserDelete`

```go
func CanUserDelete(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool
```

CanUserDelete checks if a user has permission to delete messages in the chat. Handles anonymous admins and validates the CanDeleteMessages permission.

**Parameters:**
- `b`
- `ctx`
- `chat`
- `userId`

#### `CanUserPin`

```go
func CanUserPin(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool
```

CanUserPin checks if a user has permission to pin messages in the chat. Handles anonymous admins and validates the CanPinMessages permission.

**Parameters:**
- `b`
- `ctx`
- `chat`
- `userId`

#### `CanUserPromote`

```go
func CanUserPromote(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool
```

CanUserPromote checks if a user has permission to promote/demote other members. Handles anonymous admins and validates the CanPromoteMembers permission.

**Parameters:**
- `b`
- `ctx`
- `chat`
- `userId`

#### `CanUserRestrict`

```go
func CanUserRestrict(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool
```

CanUserRestrict checks if a user has permission to restrict other members. Handles anonymous admins and validates the CanRestrictMembers permission.

**Parameters:**
- `b`
- `ctx`
- `chat`
- `userId`

### 👤 User Status Checks

#### `IsUserAdmin`

```go
func IsUserAdmin(b *gotgbot.Bot, chatID, userId int64) bool
```

IsUserAdmin checks if a user has administrator privileges in a chat. Uses caching system to avoid repeated API calls and handles special Telegram admin accounts. Returns true if the user is an admin, creator, or special Telegram account.

**Parameters:**
- `b`
- `chatID`
- `userId`

#### `IsUserAdminCached`

```go
func IsUserAdminCached(chatID, userId int64) (isAdmin, known bool)
```

IsUserAdminCached answers from the stored admin list without calling Telegram. known is false when the list is missing, negative or empty, and the caller must then fall back to IsUserAdmin.

**Parameters:**
- `chatID`
- `userId`

#### `IsUserAdminWithContext`

```go
func IsUserAdminWithContext(ctx context.Context, b *gotgbot.Bot, chatID, userId int64) (bool, error)
```

IsUserAdminWithContext is IsUserAdmin bounded by ctx. When ctx ends the check it returns false with ctx.Err() and caches nothing. A panic in the admin-list load comes back as `*cache.AdminLoadPanicError`. IsUserAdmin calls it with context.Background() and re-panics on that error.

**Parameters:**
- `ctx`
- `b`
- `chatID`
- `userId`

#### `IsUserAdminForUpdate`

```go
func IsUserAdminForUpdate(b *gotgbot.Bot, ctx *ext.Context, chatID, userID int64) bool
```

IsUserAdminForUpdate is IsUserAdmin memoised for the current update. Use it only in watchers, never after a state change in the same update.

**Parameters:**
- `b`
- `ctx`
- `chatID`
- `userID`

#### `IsApprovedForUpdate`

```go
func IsApprovedForUpdate(b *gotgbot.Bot, ctx *ext.Context, chatID, userID int64) bool
```

IsApprovedForUpdate is IsApproved memoised for the current update. Use it only in watchers, never after a state change in the same update.

**Parameters:**
- `b`
- `ctx`
- `chatID`
- `userID`

#### `IsUserBanProtected`

```go
func IsUserBanProtected(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool
```

IsUserBanProtected checks if a user is protected from being banned. Returns true for private chats, admins, and special Telegram accounts. Used to prevent banning of administrators and system accounts.

**Parameters:**
- `b`
- `ctx`
- `chat`
- `userId`

#### `IsUserInChat`

```go
func IsUserInChat(b *gotgbot.Bot, chat *gotgbot.Chat, userId int64) bool
```

IsUserInChat checks if a user is currently a member of the specified chat. Returns false for special Telegram accounts and users with "left" or "kicked" status.

**Parameters:**
- `b`
- `chat`
- `userId`

#### `IsUserInChatWithError`

```go
func IsUserInChatWithError(b *gotgbot.Bot, chat *gotgbot.Chat, userId int64) (bool, error)
```

IsUserInChatWithError tells a definite non-member apart from a Telegram lookup error. IsUserInChat wraps it and returns false on error.

**Parameters:**
- `b`
- `chat`
- `userId`

#### `IsUserConnected`

```go
func IsUserConnected(b *gotgbot.Bot, ctx *ext.Context, chatAdmin, botAdmin bool) *gotgbot.Chat
```

IsUserConnected returns the effective chat in a group, or the connected group in PM. With chatAdmin/botAdmin it also requires the user/bot to be admin there. Unlike the other predicates, it replies itself (or answers the callback) on failure and returns nil.

**Parameters:**
- `b`
- `ctx`
- `chatAdmin`
- `botAdmin`

#### `GetEffectiveUser`

```go
func GetEffectiveUser(ctx *ext.Context) *gotgbot.User
```

GetEffectiveUser safely extracts the user from the context without nil panics. Returns nil for channel messages where `ctx.EffectiveSender` is nil. Use this instead of direct `ctx.EffectiveSender.User` access to avoid nil pointer dereferences.

**Parameters:**
- `ctx`

### 🔧 Utility Functions

#### `CheckDisabledCmd`

```go
func CheckDisabledCmd(bot *gotgbot.Bot, msg *gotgbot.Message, cmd string) bool
```

CheckDisabledCmd checks if a command is disabled in the chat and handles deletion if configured. Returns true if the command should be blocked, false if it should proceed. Skips checks for private chats and admin users. If command is disabled for non-admin users, optionally deletes the message based on chat settings.

**Parameters:**
- `bot`
- `msg`
- `cmd`

## Special Telegram IDs

| ID | Description |
|----|-------------|
| `1087968824` | Anonymous Admin Bot (GroupAnonymousBot) |
| `777000` | Telegram System Account |
| `136817688` | SendAsChannel Bot (for users sending messages as channel) |

## Usage Example

```go
func (m moduleStruct) myCommand(b *gotgbot.Bot, ctx *ext.Context) error {
    chat := ctx.EffectiveChat
    user := ctx.EffectiveSender.User
    
    // Check if user is admin
    if !chat_status.RequireUserAdmin(b, ctx, chat, user.Id) {
        return ext.EndGroups
    }
    
    // Check if bot can restrict
    if !chat_status.CanBotRestrict(b, ctx, chat) {
        return ext.EndGroups
    }
    
    // Proceed with action...
    return ext.EndGroups
}
```

