---
title: Permission System
description: Complete reference of permission checking functions
---

# 🔐 Permission System

This page documents all permission checking functions in Alita Robot.

## Overview

- **Total Functions**: 22
- **Location**: `alita/utils/chat_status/chat_status.go`

## Function Summary

| Function | Returns | Description |
|----------|---------|-------------|
| `CanBotDelete` | `bool` | CanBotDelete checks if the bot has permission to delete m... |
| `CanBotPin` | `bool` | CanBotPin checks if the bot has permission to pin message... |
| `CanBotPromote` | `bool` | CanBotPromote checks if the bot has permission to promote... |
| `CanBotRestrict` | `bool` | CanBotRestrict checks if the bot has permission to restri... |
| `IsBotAdmin` | `bool` | IsBotAdmin checks if the bot has administrator privileges... |
| `IsChannelId` | `bool` | IsChannelId checks if an ID represents a Telegram channel... |
| `IsValidUserId` | `bool` | IsValidUserId checks if an ID represents a valid Telegram... |
| `RequireBotAdmin` | `bool` | RequireBotAdmin ensures the bot has administrator privile... |
| `RequireGroup` | `bool` | RequireGroup ensures the command is being used in a group... |
| `RequirePrivate` | `bool` | RequirePrivate ensures the command is being used in a pri... |
| `RequireUserAdmin` | `bool` | RequireUserAdmin ensures a user has administrator privile... |
| `RequireUserOwner` | `bool` | RequireUserOwner ensures a user is the chat creator/owner... |
| `CanUserChangeInfo` | `bool` | CanUserChangeInfo checks if a user has permission to chan... |
| `CanUserDelete` | `bool` | CanUserDelete checks if a user has permission to delete m... |
| `CanUserPin` | `bool` | CanUserPin checks if a user has permission to pin message... |
| `CanUserPromote` | `bool` | CanUserPromote checks if a user has permission to promote... |
| `CanUserRestrict` | `bool` | CanUserRestrict checks if a user has permission to restri... |
| `Caninvite` | `bool` | Caninvite checks if the bot and user have permissions to ... |
| `IsUserAdmin` | `bool` | IsUserAdmin checks if a user has administrator privileges... |
| `IsUserBanProtected` | `bool` | IsUserBanProtected checks if a user is protected from bei... |
| `IsUserInChat` | `bool` | IsUserInChat checks if a user is currently a member of th... |
| `CheckDisabledCmd` | `bool` | CheckDisabledCmd checks if a command is disabled in the c... |

## Functions by Category

### 🤖 Bot Permission Checks

#### `CanBotDelete`

```go
func CanBotDelete(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, justCheck bool) bool
```

CanBotDelete checks if the bot has permission to delete messages in the chat. Validates the bot's CanDeleteMessages permission. If justCheck is false, sends error messages explaining the missing permission.

**Parameters:**
- `b`
- `ctx`
- `chat`
- `justCheck`

#### `CanBotPin`

```go
func CanBotPin(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, justCheck bool) bool
```

CanBotPin checks if the bot has permission to pin messages in the chat. Validates the bot's CanPinMessages permission. If justCheck is false, sends error messages explaining the missing permission.

**Parameters:**
- `b`
- `ctx`
- `chat`
- `justCheck`

#### `CanBotPromote`

```go
func CanBotPromote(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, justCheck bool) bool
```

CanBotPromote checks if the bot has permission to promote/demote members in the chat. Validates the bot's CanPromoteMembers permission. If justCheck is false, sends error messages explaining the missing permission.

**Parameters:**
- `b`
- `ctx`
- `chat`
- `justCheck`

#### `CanBotRestrict`

```go
func CanBotRestrict(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, justCheck bool) bool
```

CanBotRestrict checks if the bot has permission to restrict members in the chat. Validates the bot's CanRestrictMembers permission. If justCheck is false, sends error messages explaining the missing permission.

**Parameters:**
- `b`
- `ctx`
- `chat`
- `justCheck`

#### `IsBotAdmin`

```go
func IsBotAdmin(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool
```

IsBotAdmin checks if the bot has administrator privileges in the specified chat. Returns true for private chats (bot is always "admin" in private). For groups, verifies the bot's actual admin status.

**Parameters:**
- `b`
- `ctx`
- `chat`

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

### ✅ Requirement Checks

#### `RequireBotAdmin`

```go
func RequireBotAdmin(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, justCheck bool) bool
```

RequireBotAdmin ensures the bot has administrator privileges in the chat. Uses IsBotAdmin internally to perform the check. If justCheck is false, sends error messages when bot is not admin.

**Parameters:**
- `b`
- `ctx`
- `chat`
- `justCheck`

#### `RequireGroup`

```go
func RequireGroup(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, justCheck bool) bool
```

RequireGroup ensures the command is being used in a group chat. Returns false for private chats. If justCheck is false, sends error messages explaining the command is for group use only.

**Parameters:**
- `b`
- `ctx`
- `chat`
- `justCheck`

#### `RequirePrivate`

```go
func RequirePrivate(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, justCheck bool) bool
```

RequirePrivate ensures the command is being used in a private chat. Returns false for group chats and supergroups. If justCheck is false, sends error messages explaining the command is for private use only.

**Parameters:**
- `b`
- `ctx`
- `chat`
- `justCheck`

#### `RequireUserAdmin`

```go
func RequireUserAdmin(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64, justCheck bool) bool
```

RequireUserAdmin ensures a user has administrator privileges in the chat. Uses IsUserAdmin internally to perform the check. If justCheck is false, sends error messages when user is not admin.

**Parameters:**
- `b`
- `ctx`
- `chat`
- `userId`
- `justCheck`

#### `RequireUserOwner`

```go
func RequireUserOwner(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64, justCheck bool) bool
```

RequireUserOwner ensures a user is the chat creator/owner. Checks for "creator" status specifically, not just administrator. If justCheck is false, sends error messages when user is not the creator.

**Parameters:**
- `b`
- `ctx`
- `chat`
- `userId`
- `justCheck`

### 👮 User Permission Checks

#### `CanUserChangeInfo`

```go
func CanUserChangeInfo(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64, justCheck bool) bool
```

CanUserChangeInfo checks if a user has permission to change chat information. Handles anonymous admins and validates the CanChangeInfo permission. If justCheck is false, sends error messages to user.

**Parameters:**
- `b`
- `ctx`
- `chat`
- `userId`
- `justCheck`

#### `CanUserDelete`

```go
func CanUserDelete(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64, justCheck bool) bool
```

CanUserDelete checks if a user has permission to delete messages in the chat. Handles anonymous admins and validates the CanDeleteMessages permission. If justCheck is false, sends error messages to user.

**Parameters:**
- `b`
- `ctx`
- `chat`
- `userId`
- `justCheck`

#### `CanUserPin`

```go
func CanUserPin(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64, justCheck bool) bool
```

CanUserPin checks if a user has permission to pin messages in the chat. Handles anonymous admins and validates the CanPinMessages permission. If justCheck is false, sends error messages to user.

**Parameters:**
- `b`
- `ctx`
- `chat`
- `userId`
- `justCheck`

#### `CanUserPromote`

```go
func CanUserPromote(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64, justCheck bool) bool
```

CanUserPromote checks if a user has permission to promote/demote other members. Handles anonymous admins and validates the CanPromoteMembers permission. If justCheck is false, sends error messages to user.

**Parameters:**
- `b`
- `ctx`
- `chat`
- `userId`
- `justCheck`

#### `CanUserRestrict`

```go
func CanUserRestrict(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64, justCheck bool) bool
```

CanUserRestrict checks if a user has permission to restrict other members. Handles anonymous admins and validates the CanRestrictMembers permission. If justCheck is false, sends error messages to user.

**Parameters:**
- `b`
- `ctx`
- `chat`
- `userId`
- `justCheck`

#### `Caninvite`

```go
func Caninvite(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, msg *gotgbot.Message, justCheck bool) bool
```

Caninvite checks if the bot and user have permissions to generate invite links. Returns true immediately if the chat has a public username. Validates both bot and user permissions for invite link generation.

**Parameters:**
- `b`
- `ctx`
- `chat`
- `msg`
- `justCheck`

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
| `136817688` | Channel Bot (deprecated) |

## Usage Example

```go
func (m moduleStruct) myCommand(b *gotgbot.Bot, ctx *ext.Context) error {
    chat := ctx.EffectiveChat
    user := ctx.EffectiveSender.User

    // Check if user is admin
    if !chat_status.RequireUserAdmin(b, ctx, chat, user.Id, false) {
        return ext.EndGroups
    }

    // Check if bot can restrict
    if !chat_status.CanBotRestrict(b, ctx, chat, false) {
        return ext.EndGroups
    }

    // Proceed with action...
    return ext.EndGroups
}
```
