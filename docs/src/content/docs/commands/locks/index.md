---
title: Locks Commands
description: Complete guide to Locks module commands and features
---

# 🔒 Locks Commands

*Admin only*:
× /lock `<permission>`: Lock Chat permission.
× /unlock `<permission>`: Unlock Chat permission.
× /locks: View Chat permission.
× /locktypes: Check available lock types!

Locks can be used to restrict a group's users.
Locking URLs will auto-delete all messages with URLs, locking stickers will delete all stickers, etc.
Locking bots will stop non-admins from adding bots to the chat.

**Example:**
`/lock media`: this locks all the media messages in the chat.

## Module Aliases

This module can be accessed using the following aliases:

- `lock`
- `unlock`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/lock` | Lock Chat permission. | ❌ |
| `/locks` | View Chat permission. | ✅ |
| `/locktypes` | Check available lock types! | ✅ |
| `/unlock` | Unlock Chat permission. | ❌ |

## Usage Examples

### Basic Usage

```
/lock
/locks
/locktypes
```

For detailed command usage, refer to the commands table above.

## Required Permissions

Most commands in this module require **admin permissions** in the group.

**Bot Permissions Required:**

- Delete messages
- Restrict users

## Technical Notes

**Technical Notes**
- Lock enforcement is real-time
- Admins are exempt from all locks
- Locks persist across bot restarts
- Cache invalidated on updates
