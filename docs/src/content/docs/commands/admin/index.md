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

## Anonymous Admin Support

The `/anonadmin` command allows group owners to toggle anonymous admin recognition:

```
/anonadmin on    # Enable anonymous admin checks
/anonadmin off   # Disable anonymous admin checks
```

When enabled, the bot will request verification for admin actions from anonymous accounts.

### How Anonymous Admin Verification Works

When an anonymous admin (posting as the group) runs a command like `/ban`:

1. **Command Interception**: The bot detects the sender is `GroupAnonymousBot` (Telegram's ID for anonymous group posts)
2. **Cache Storage**: The original message is cached with key `alita:anonAdmin:{chatId}:{msgId}` (20-second TTL)
3. **Verification Button**: A "Verify Admin" button is sent to the chat
4. **Button Click**: When clicked, the handler:
   - Verifies the clicking user is actually an admin via `IsUserAdmin()`
   - Retrieves the original command from cache via `getAnonAdminCache()`
   - Executes the command as if the admin had sent it directly
5. **Expiration**: If 20 seconds pass without verification, the button expires

**Supported Commands for Anonymous Verification:**
- Admin: `/promote`, `/demote`, `/title`
- Bans: `/ban`, `/dban`, `/sban`, `/tban`, `/unban`, `/restrict`, `/unrestrict`
- Mutes: `/mute`, `/smute`, `/dmute`, `/tmute`, `/unmute`
- Pins: `/pin`, `/unpin`, `/permapin`, `/unpinall`
- Purges: `/purge`, `/del`
- Warns: `/warn`, `/swarn`, `/dwarn`

## User Lookup Behavior

Admin commands accept multiple input formats to identify target users:

| Input Type | Example | Resolution Method |
|------------|---------|-------------------|
| Reply | Reply to message | Direct from message |
| User ID | `/promote 123456789` | Trusted numeric ID |
| Username | `/promote @username` | DB lookup → Telegram API fallback |
| Text Mention | Click on inline mention | Direct from entity |

**Telegram API Fallback**: When a username isn't found in the local database, the bot queries Telegram's API directly. This ensures admin commands work on any valid user, not just those the bot has previously seen.

## Required Permissions

Most commands in this module require **admin permissions** in the group.

**Bot Permissions Required:**

- Delete messages
- Ban users
- Restrict users
- Pin messages (if applicable)

## Security Notes

- All user-controlled input (chat titles, usernames) is HTML-escaped before rendering in messages to prevent injection attacks
- Admin permission changes run in background goroutines with proper error handling and panic recovery
