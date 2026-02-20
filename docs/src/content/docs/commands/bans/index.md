---
title: Bans Commands
description: Complete guide to Bans module commands and features
---

# 🔨 Bans Commands

Sometimes users can be annoying and you might want to remove them from your chat, this module exactly helps you to deal with that!.
Ban/kick users from your chat, and unban them later on if they're behaving themselves.

*User Commands:*
× /kickme: kicks the user who issued the command.

*Ban Commands* (Admin only):
× /ban <userhandle>: bans a user. (via handle, or reply)
× /sban <userhandle>: bans a user silently, does not send message to group and also deletes your command. (via handle, or reply)
× /dban <userhandle>: bans a user and delete the replied message. (via handle, or reply)
× /tban <userhandle> x(m/h/d): bans a user for `x` time. (via handle, or reply). m = minutes, h = hours, d = days.
× /unban <userhandle>: unbans a user. (via handle, or reply)

*Restrict Commands:* (Admin only)
× /restrict: Shows an InlineKeyboard to choose options from kick, ban and mute
× /unrestrict: Shows an InlineKeyboard to choose options from unmute and unban.

## Module Aliases

This module can be accessed using the following aliases:

- `ban`
- `kick`
- `dkick`
- `restrict`
- `kickme`
- `unrestrict`
- `sban`
- `dban`
- `tban`
- `unban`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/ban` | bans a user. (via handle, or reply) | ❌ |
| `/dban` | bans a user and delete the replied message. (via handle, or reply) | ❌ |
| `/dkick` | Delete the replied message and kick the sender. | ❌ |
| `/kick` | Kick a user from the group (by reply, @handle, or user ID). | ❌ |
| `/kickme` | kicks the user who issued the command. | ❌ |
| `/restrict` | Shows an InlineKeyboard to choose options from kick, ban and mute | ❌ |
| `/sban` | bans a user silently, does not send message to group and also deletes your command. (via handle, or reply) | ❌ |
| `/tban` | bans a user for `x` time. (via handle, or reply). m = minutes, h = hours, d = days. | ❌ |
| `/unban` | unbans a user. (via handle, or reply) | ❌ |
| `/unrestrict` | Shows an InlineKeyboard to choose options from unmute and unban. | ❌ |

## Usage Examples

### Basic Usage

```
/ban
/dban
/dkick
```

For detailed command usage, refer to the commands table above.

## Required Permissions

Most commands require admin with ban/restrict permissions. `/kickme` is available to all non-admin users.

**Bot Permissions Required:**

- Ban users
- Delete messages (for /dban, /dkick, /sban)
- Restrict members (for /restrict)
