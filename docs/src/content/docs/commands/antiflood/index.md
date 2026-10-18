---
title: Antiflood Commands
description: Complete guide to Antiflood module commands and features
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# 🌊 Antiflood Commands

You know how sometimes, people join, send 100 messages, and ruin your chat? With antiflood, that happens no more!

Antiflood allows you to take action on users that send more than x messages with no gap longer than 60 seconds between them (other members' messages don't reset the count). Actions are: ban/kick/mute

*User commands*:
× /flood: Get the current antiflood settings.

*Admin commands*:
× /setflood `<number/off/no>`: Set the number of messages after which to take action on a user. Valid range is **3-100**. Set to '0', 'off', 'no', or 'false' to disable.
× /setfloodmode `<action type>`: Choose which action to take on a user who has been flooding. Options: ban/kick/mute
× /delflood `<yes/no/on/off>`: If you want bot to delete messages flooded by user.

## Module Aliases

> These are help-menu module names, not command aliases.

This module can be accessed using the following aliases:

- `flood`
- `clearflood`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/clearflood` | Alias of `/delflood` — toggle deletion of flood messages | ❌ |
| `/delflood` | Toggle deletion of flood messages | ❌ |
| `/flood` | Show current flood settings | ✅ |
| `/setflood` | Set flood message count threshold | ❌ |
| `/setfloodmode` | Set action taken on flood detection | ❌ |

## Usage Examples

### Basic Usage

```
/delflood
/flood
/setflood
```

For detailed command usage, refer to the commands table above.

## Required Permissions

Most commands in this module require **admin permissions** in the group.

**Bot Permissions Required:**

- Delete messages
- Ban users (for kick/ban actions)
- Restrict users (for mute action)

**Command Permissions:**
- `/setflood`, `/setfloodmode`, `/delflood` (`/clearflood`) — Require admin permission
- `/flood` — Available to all users (disableable)

**Defaults:**
- Flood action defaults to **mute** if not explicitly set
- Message deletion on flood (`/delflood`) defaults to **off**; when off, only the message that triggers antiflood is deleted, and when on, every tracked message of the flood is deleted

## Technical Notes

**Default Behavior**
Antiflood is **disabled by default**. You must explicitly enable it using `/setflood &lt;number&gt;`.
