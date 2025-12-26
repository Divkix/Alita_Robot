---
title: Antiflood Commands
description: Complete guide to Antiflood module commands and features
---

# 🌊 Antiflood Commands

You know how sometimes, people join, send 100 messages, and ruin your chat? With antiflood, that happens no more!

Antiflood allows you to take action on users that send more than x messages in a row. Actions are: ban/kick/mute

*Admin commands*:
× /flood: Get the current antiflood settings.
× /setflood `<number/off/no>`: Set the number of messages after which to take action on a user. Set to '0', 'off', or 'no' to disable.
× /setfloodmode `<action type>`: Choose which action to take on a user who has been flooding. Options: ban/kick/mute
× /delflood `<yes/no/on/off>`: If you want bot to delete messages flooded by user.

## Module Aliases

This module can be accessed using the following aliases:

- `flood`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/delflood` | If you want bot to delete messages flooded by user. | ❌ |
| `/flood` | Get the current antiflood settings. | ✅ |
| `/setflood` | Set the number of messages after which to take action on a user. Set to '0', 'off', or 'no' to disable. | ❌ |
| `/setfloodmode` | Choose which action to take on a user who has been flooding. Options: ban/kick/mute | ❌ |

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
- Ban users
- Restrict users
- Pin messages (if applicable)
