---
title: Locks Commands
description: Complete guide to Locks module commands and features
---

# Locks Commands

Locks can be used to restrict a group's users. Locking URLs will auto-delete all messages with URLs, locking stickers will delete all stickers, etc. Locking bots will stop non-admins from adding bots to the chat.

:::caution[Admin Permissions Required]
Most commands require admin permissions. `/locks` and `/locktypes` are available to all users.
:::

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/lock` | Lock a chat permission. | No |
| `/unlock` | Unlock a chat permission. | No |
| `/locks` | View current chat permissions. | Yes |
| `/locktypes` | Check available lock types. | Yes |

## Usage Examples

```text
/lock media          # Lock all media messages
/lock url            # Lock all URLs
/lock sticker        # Lock all stickers
/unlock media        # Unlock media messages
/locks               # View all current locks
/locktypes           # See all available lock types
```

:::tip[Check Available Types First]
Use `/locktypes` to see all the available lock types before trying to lock something.
:::

:::note
Admins are exempt from all locks. Only non-admin users will have their messages deleted.
:::

## Module Aliases

This module can be accessed using the following aliases:
`lock`, `unlock`

## Required Permissions

**Bot Permissions Required:**
- Delete messages
- Restrict users

## Technical Notes

- Lock enforcement is real-time
- Admins are exempt from all locks
- Locks persist across bot restarts
- Cache is invalidated on lock updates
