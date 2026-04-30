---
title: Bans Commands
description: Complete guide to Bans module commands and features
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

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

*Kick Commands* (Admin only):
× /kick <userhandle>: kicks a user. (via handle, or reply)
× /dkick <userhandle>: deletes the replied message and kicks a user. (via reply)

*Restrict Commands:* (Admin only)
× /restrict: Shows an InlineKeyboard to choose options from kick, ban and mute
× /unrestrict: Shows an InlineKeyboard to choose options from unmute and unban.

## Module Aliases

> These are help-menu module names, not command aliases.

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
| `/ban` | Ban a user | ❌ |
| `/dban` | Ban a user and delete the replied message | ❌ |
| `/dkick` | Kick a user and delete the replied message | ❌ |
| `/kick` | Kick a user from the group | ❌ |
| `/kickme` | Kick yourself from the group | ❌ |
| `/restrict` | Show restriction options menu | ❌ |
| `/sban` | Ban a user silently and delete your command | ❌ |
| `/tban` | Temporarily ban a user for a specified duration | ❌ |
| `/unban` | Unban a user | ❌ |
| `/unrestrict` | Show unrestriction options menu | ❌ |

## Usage Examples

### Basic Usage

```
/ban
/dban
/dkick
```

For detailed command usage, refer to the commands table above.

## Required Permissions

Most commands in this module require **admin permissions** in the group.

**Bot Permissions Required:**

- Delete messages
- Ban users
- Restrict users
