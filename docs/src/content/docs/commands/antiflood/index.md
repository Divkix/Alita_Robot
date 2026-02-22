---
title: Antiflood Commands
description: Complete guide to Antiflood module commands and features
---

# Antiflood Commands

You know how sometimes, people join, send 100 messages, and ruin your chat? With antiflood, that happens no more!

Antiflood allows you to take action on users that send more than x messages in a row. Actions are: ban/kick/mute.

:::caution[Admin Permissions Required]
All commands in this module require admin permissions in groups.
:::

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/flood` | Get the current antiflood settings. | Yes |
| `/setflood` | Set the number of messages after which to take action on a user (limit: 3-100). Set to `0`, `off`, `no`, or `false` to disable. | No |
| `/setfloodmode` | Choose which action to take on a user who has been flooding. Options: ban/kick/mute. | No |
| `/delflood` | Toggle whether the bot should delete messages from flooding users. | No |

## Usage Examples

```text
/flood                    # Check current settings
/setflood 10              # Take action after 10 messages
/setflood off             # Disable antiflood
/setfloodmode ban         # Ban flooding users
/delflood yes             # Delete flood messages
```

:::tip[Recommended Setup]
Start with a threshold of 10-15 messages and adjust based on your group's activity level. Use `mute` mode first — it is less disruptive than `ban` while still stopping the flood.
:::

:::note[Default Behavior]
Antiflood is **disabled by default**. You must explicitly enable it using `/setflood <number>`.
:::

## Module Aliases

This module can be accessed using the following aliases:
`flood`

## Required Permissions

**Bot Permissions Required:**
- Delete messages
- Ban users
- Restrict members
