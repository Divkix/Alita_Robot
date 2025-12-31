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
# Mute a user permanently by replying to their message
/mute

# Mute a user by username
/mute @username

# Mute a user with a reason
/mute @username spamming

# Temporary mute for 30 minutes
/tmute @username 30m

# Temporary mute for 2 hours
/tmute @username 2h

# Temporary mute for 1 day
/tmute @username 1d

# Silent mute (deletes your command message)
/smute @username

# Delete-mute (mutes user and deletes their message)
/dmute

# Unmute a user
/unmute @username
```

For detailed command usage, refer to the commands table above.

## Required Permissions

**Admin only commands.** Users executing these commands must have:
- Admin status in the chat
- Permission to restrict members

The bot must also have admin privileges with permission to restrict members.
