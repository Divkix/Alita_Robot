---
title: Admin Commands
description: Complete guide to Admin module commands and features
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# 👑 Admin Commands

Make it easy to promote and demote users with the admin module!

*User Commands:*
× /adminlist: List the admins in the current chat.

*Admin Commands:*
× /promote `<reply/username/mention/userid>` `[title]`: Promote a user, optionally with a custom admin title (max 16 characters).
× /demote `<reply/username/mention/userid>`: Demote a user.
× /title `<reply/username/mention/userid>` `<custom title>`: Set custom title for user
× /setgtitle `<title>`: Change the group title.
× /setgpic: Reply to a photo to set it as the group picture.
× /setgdesc `[text]`: Change the group description.
× /invitelink: Get the chat's invite link.
× /anonadmin `<on/off>`: on lets anonymous admins act without confirming; off (default) makes them confirm with a button (group owner only).
× /admincache: Reload the admin list.
× /clearadmincache: Clear the cached admin list.
× /timezone `[name]`: Show or set the chat's timezone as an IANA name such as Asia/Kolkata (default UTC).

**Anonymous Admin Support**

The `/anonadmin` command allows group owners to toggle anonymous admin recognition:
`/anonadmin on` - Trust anonymous admins without verification
`/anonadmin off` - Require button verification (default)

When disabled (the default), the bot asks for button verification before running admin actions from anonymous accounts.

**How Anonymous Admin Verification Works:**
1. Bot detects sender is GroupAnonymousBot
2. Original message cached with 20-second TTL
3. "Click to prove admin" button sent to chat
4. Clicking user verified as admin, command executed
5. Button expires after 20 seconds if not verified

**Supported Commands:**
Admin: /promote, /demote, /title, /setgtitle, /setgpic, /setgdesc
Bans: /ban, /dban, /sban, /tban, /unban, /baninfo, /banlist, /kick, /dkick, /skick, /restrict, /unrestrict
Mutes: /mute, /smute, /dmute, /tmute, /unmute
Pins: /pin, /unpin, /permapin, /unpinall
Purges: /purge, /del, /purgefrom, /purgeto
Warns: /warn, /swarn, /dwarn


## Module Aliases

> These are help-menu module names, not command aliases.

This module can be accessed using the following aliases:

- `admins`
- `promote`
- `demote`
- `title`
- `setgtitle`
- `setgpic`
- `setgdesc`
- `timezone`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/admincache` | Refresh cached admin list | ❌ |
| `/adminlist` | List chat admins | ✅ |
| `/anonadmin` | Toggle anonymous admin trust (on = no verification button) | ❌ |
| `/clearadmincache` | Clear the cached admin list | ❌ |
| `/demote` | Demote a user from admin | ❌ |
| `/invitelink` | Get group invite link | ❌ |
| `/promote` | Promote a user to admin, optionally with a custom title | ❌ |
| `/setgdesc` | Change the group description (no text clears it); aliases `/setgroupdesc`, `/setdescription` | ❌ |
| `/setgpic` | Set the group photo from a replied-to photo; alias `/setgrouppic` | ❌ |
| `/setgtitle` | Change the group title; alias `/setgrouptitle` | ❌ |
| `/timezone` | Show or set the chat's timezone (IANA name, default UTC); needs the change-info right | ❌ |
| `/title` | Set custom admin title for a user | ❌ |

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

- Promote new admins
- Invite users via link
- Change group info

## Technical Notes

**Security Notes**
- All user-controlled input is HTML-escaped to prevent injection attacks
- Admin permission changes run with proper error handling and panic recovery
