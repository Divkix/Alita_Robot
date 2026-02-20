---
title: Mutes Commands
description: Complete guide to Mutes module commands and features
---

# 📦 Mutes Commands

Sometimes users can be annoying and you might want to restrict them from sending a message to chat, this module is here to help, you can use this module to mute members in your group.

*Mute Commands:* (Admin only)
× /mute <userhandle>: mutes a user, (via a handle, or reply)
× /smute <userhandle>: mutes a user silently, does not send a message to the group, and also deletes your command. (via a handle, or reply)
× /dmute <userhandle>: mutes a user and deletes the replied message. (via a handle, or reply)
× /tmute <userhandle> x(m/h/d): mutes a user for `x` time. (via a handle, or reply). m = minutes, h = hours, d = days.
× /unmute <userhandle>: unmutes a user. (via a handle, or reply)

**Time Format for Temporary Mutes:**
- `m` = minutes (e.g., `30m`)
- `h` = hours (e.g., `2h`)
- `d` = days (e.g., `1d`)

**Mute Variants:**
- `/mute` - Standard mute with optional reason
- `/smute` - Silent mute (deletes your command message)
- `/dmute` - Delete-mute (mutes user and deletes their message)
- `/tmute` - Temporary mute with specified duration

**Required Permissions:**
**Admin only commands.** Users executing these commands must have:
- Admin status in the chat
- Permission to restrict members

The bot must also have admin privileges with permission to restrict members.


## Module Aliases

This module can be accessed using the following aliases:

- `mute`
- `unmute`
- `tmute`
- `smute`
- `dmute`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/dmute` | mutes a user and deletes the replied message. (via a handle, or reply) | ❌ |
| `/mute` | mutes a user, (via a handle, or reply) | ❌ |
| `/smute` | mutes a user silently, does not send a message to the group, and also deletes your command. (via a handle, or reply) | ❌ |
| `/tmute` | mutes a user for `x` time. (via a handle, or reply). m = minutes, h = hours, d = days. | ❌ |
| `/unmute` | unmutes a user. (via a handle, or reply) | ❌ |

## Usage Examples

### Basic Usage

```
/dmute
/mute
/smute
```

For detailed command usage, refer to the commands table above.

## Required Permissions

All mute commands require admin with restrict permissions in groups.
