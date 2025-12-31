---
title: Admin Commands
description: Complete guide to Admin module commands and features
---

# 👑 Admin Commands

Make it easy to promote and demote users with the admin module!

*User Commands:*
× /adminlist: List the admins in the current chat.

*Admin Commands:*
× /promote `<reply/username/mention/userid>`: Promote a user.
× /demote `<reply/username/mention/userid>`: Demote a user.
× /title `<reply/username/mention/userid>` `<custom title>`: Set custom title for user

**Anonymous Admin Support**

The `/anonadmin` command allows group owners to toggle anonymous admin recognition:
`/anonadmin on` - Enable anonymous admin checks
`/anonadmin off` - Disable anonymous admin checks

When enabled, the bot will request verification for admin actions from anonymous accounts.

**How Anonymous Admin Verification Works:**
1. Bot detects sender is GroupAnonymousBot
2. Original message cached with 20-second TTL
3. "Verify Admin" button sent to chat
4. Clicking user verified as admin, command executed
5. Button expires after 20 seconds if not verified

**Supported Commands:**
Admin: /promote, /demote, /title
Bans: /ban, /dban, /sban, /tban, /unban
Mutes: /mute, /smute, /dmute, /tmute, /unmute
Pins: /pin, /unpin, /permapin, /unpinall
Purges: /purge, /del
Warns: /warn, /swarn, /dwarn


## Module Aliases

This module can be accessed using the following aliases:

- `admins`
- `promote`
- `demote`
- `title`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/admincache` | No description available | ❌ |
| `/adminlist` | List the admins in the current chat. | ✅ |
| `/anonadmin` | No description available | ❌ |
| `/demote` | Demote a user. | ❌ |
| `/invitelink` | No description available | ❌ |
| `/promote` | Promote a user. | ❌ |
| `/title` | Set custom title for user | ❌ |

## Usage Examples

### Basic Usage

```
/admincache
/adminlist
/anonadmin
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

**Security Notes**
- All user-controlled input is HTML-escaped to prevent injection attacks
- Admin permission changes run with proper error handling and panic recovery
