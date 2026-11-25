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
× /allowlist `<host or invite link> [item ...]`: Except these hosts (the url lock) or private Telegram invites (the invitelink lock) from deletion; each link in a message is checked on its own, so an allowed host never exempts another link.
× /rmallowlist `<host or invite link> [item ...]`: Remove allowlist entries.
× /rmallowlistall: Remove all allowlist entries (owner only, asks for confirmation).
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
| `/allowlist` | Except hosts (url lock) or private Telegram invites (invitelink lock) from deletion | ❌ |
| `/rmallowlist` | Remove allowlist entries | ❌ |
| `/rmallowlistall` | Remove all allowlist entries (owner only) | ❌ |
| `/nightmode` | Apply extra lock types every day between two times in the chat's timezone | ❌ |
| `/unlock` | Unlock one or more lock types in the group | ❌ |

## Usage Examples

### Basic Usage

```
/lock
/locks
/locktypes
/allowlist example.com t.me/+AbCdEf
/rmallowlist example.com
/rmallowlistall
/nightmode 23:00 07:00 media,url
/nightmode off
```

For detailed command usage, refer to the commands table above.

## Required Permissions

Most commands in this module require **admin permissions** in the group.

`/locktypes` is member-readable; `/locks`, `/lock`, `/unlock`, `/allowlist` and `/rmallowlist` require a group admin. `/rmallowlistall` requires the group owner. Disabling `/locktypes` also silences members querying that group from PM, without deleting their PM messages; admins remain exempt.

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
- The `url` and `invitelink` locks honour `/allowlist` per link entity: a host entry covers that host and its subdomains, an invite entry covers one private invite hash (`+HASH`, `joinchat/HASH` or `tg://join?invite=HASH`, case-sensitive). Public usernames, previews, discussion links, phone lookups and phone-number profiles such as `t.me/+15551234567` are not invites. Every link in a message or caption is checked on its own, so an allowed host or invite never exempts another unapproved link, and a lookalike host such as `example.com.evil.test` is not covered. Invite exceptions stay local to the chat: they name a live private join credential, so chat backups carry only host entries and a restore never adds, removes or replaces the destination's invite exceptions. `/rmallowlistall` is a one-use owner confirmation.
