---
title: Locks Commands
description: Complete guide to Locks module commands and features
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# 🔒 Locks Commands

*Admin only:*
× /lock `<type> [type ...]`: Lock one or more lock types (see /locktypes).
× /unlock `<type> [type ...]`: Unlock one or more lock types.
× /locks: View Chat permission.
× /nightmode `<start> <end> <types|all>`: Also apply these lock types every day from start to end (24-hour HH:MM in the chat's /timezone; the window may cross midnight). `/nightmode off` turns it off; /nightmode alone shows it.

*User Commands:*
× /locktypes: Check available lock types!

Locks can be used to restrict a group's users.
Locking URLs will auto-delete all messages with URLs, locking stickers will delete all stickers, etc.
Locking bots will stop users who are neither admins nor approved from adding bots to the chat.

**Example:**
`/lock media`: this locks all the media messages in the chat.

## Module Aliases

> These are help-menu module names, not command aliases.

This module can be accessed using the following aliases:

- `lock`
- `unlock`
- `nightmode`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/lock` | Lock one or more lock types in the group | ❌ |
| `/locks` | Show current lock settings | ✅ |
| `/locktypes` | List all available lock types | ✅ |
| `/nightmode` | Apply extra lock types every day between two times in the chat's timezone | ❌ |
| `/unlock` | Unlock one or more lock types in the group | ❌ |

## Usage Examples

### Basic Usage

```
/lock
/locks
/locktypes
/nightmode 23:00 07:00 media,url
/nightmode off
```

For detailed command usage, refer to the commands table above.

## Required Permissions

Most commands in this module require **admin permissions** in the group.

**Bot Permissions Required:**

- Delete messages
- Ban users
- Restrict users
- Pin messages (if applicable)

## Technical Notes

**Technical Notes**
- Lock enforcement is real-time
- Admins and approved users are exempt from all locks
- Locks persist across bot restarts
- Night mode adds its lock types to the day locks only inside its window, in the chat's timezone (`/timezone`, default UTC); it never changes the locks set with `/lock`. It cannot apply `bots`. A window crossing midnight runs past it; a start or end skipped by a daylight-saving jump takes effect at the next minute that exists
- Cache invalidated on updates
