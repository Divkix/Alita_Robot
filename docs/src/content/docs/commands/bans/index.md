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
× /banme: bans the user who issued the command.

*Ban Commands* (Admin only):
× /ban <userhandle>: bans a user. (via handle, or reply)
× /sban <userhandle>: bans a user silently, does not send message to group and also deletes your command. (via handle, or reply)
× /dban: bans a user and deletes the replied message. (reply only)
× /tban <userhandle> x(m/h/d/w): bans a user for `x` time. (via handle, or reply). m = minutes, h = hours, d = days, w = weeks.
× /unban <userhandle>: unbans a user. (via handle, or reply)
× /unbanall: owner-only confirmation to unban known users.

*Kick Commands* (Admin only):
× /kick <userhandle>: kicks a user. (via handle, or reply)
× /skick <userhandle>: kicks a user silently and deletes your command.
× /dkick: kicks a user and deletes the replied message. (reply only)

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
- `skick`
- `banme`
- `unbanall`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/ban` | Ban a user | ❌ |
| `/banme` | Ban yourself from the group | ✅ |
| `/dban` | Ban a user and delete the replied message | ❌ |
| `/dkick` | Kick a user and delete the replied message | ❌ |
| `/kick` | Kick a user from the group | ❌ |
| `/kickme` | Kick yourself from the group | ✅ |
| `/restrict` | Show restriction options menu | ❌ |
| `/sban` | Ban a user silently and delete your command | ❌ |
| `/skick` | Kick a user silently and delete your command | ❌ |
| `/tban` | Temporarily ban a user for a specified duration | ❌ |
| `/unban` | Unban a user | ❌ |
| `/unbanall` | Unban every known user in the chat (owner only, asks for confirmation) | ❌ |
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
