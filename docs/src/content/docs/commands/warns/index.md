---
title: Warns Commands
description: Complete guide to Warns module commands and features
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# 📦 Warns Commands

Keep your members in check with warnings; stop them getting out of control!
If you're looking for automated warnings, read about the blacklist module!

*User Commands:*
- /warns [user]: See your own warnings, or another user's.

*Admin Commands:*
- /warn <reason>: Warn a user.
- /dwarn <reason>: Warn a user by reply, and delete their message.
- /swarn <reason>: Warn a user and delete your command message; the warning is still posted.
- /rmwarn: Remove a user's latest warning.
- /resetwarns: Reset all of a user's warnings to 0.
- /resetallwarns: Delete all the warnings in a chat. All users return to 0 warns. (creator only)
- /warnings: Get the chat's warning settings.
- /setwarnmode <ban/kick/mute/tban/tmute> [time]: Set the chat's warn mode.
- /setwarntime <time/off>: Set how long a warn counts, e.g. 30d; off keeps warns forever. Warns issued before the bot recorded warn times never expire; /resetwarns removes them.
- /setwarnlimit <number>: Set the number of warnings before users are punished.

*Examples*
- Warn a user.
-> `/warn @user For disobeying the rules`

**Default Settings:**
- **Default warn limit:** 3 warnings
- **Default warn mode:** mute
- **Default warn expiry:** off (warns never expire)

**Available Warn Modes:**
- `ban` - Permanently ban the user from the chat
- `kick` - Kick the user (they can rejoin)
- `mute` - Mute the user (cannot send messages)
- `tban` - Ban the user for the time given with the mode, e.g. `/setwarnmode tban 1d`
- `tmute` - Mute the user for the time given with the mode, e.g. `/setwarnmode tmute 2h`

**Warnings That Delete Messages:**
- `/swarn` - Delete your command; the warning is still posted
- `/dwarn` - Delete the user's message and warn them


## Module Aliases

> These are help-menu module names, not command aliases.

This module can be accessed using the following aliases:

- `warn`
- `warning`
- `warnings`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/dwarn` | Warn a user and delete the replied message | ❌ |
| `/resetallwarns` | Reset all warnings for all users in the chat (chat owner only) | ❌ |
| `/resetwarn` | Alias of `/resetwarns` - reset all warnings for a user | ❌ |
| `/resetwarns` | Reset all warnings for a user | ❌ |
| `/rmwarn` | Remove the last warning from a user | ❌ |
| `/setwarnlimit` | Set maximum warnings before action (range: 1–100) | ❌ |
| `/setwarnmode` | Set action taken when warn limit is reached | ❌ |
| `/setwarntime` | Set how long a warn counts before it expires | ❌ |
| `/swarn` | Warn a user and delete your command (the warning is still posted) | ❌ |
| `/unwarn` | Alias of `/rmwarn` - remove the last warning from a user | ❌ |
| `/warn` | Warn a user | ❌ |
| `/warnings` | Get the chat's warning settings | ❌ |
| `/warns` | Show warning count for a user | ✅ |

## Usage Examples

### Basic Usage

```
/dwarn
/resetallwarns
/resetwarn
```

For detailed command usage, refer to the commands table above.

## Required Permissions

**Permission Requirements:**
- `/warn`: Bot admin + User admin with restrict
- `/dwarn`, `/swarn`: Bot admin + User admin, both with restrict and delete rights
- `/rmwarn`, `/unwarn`: Bot admin + User admin
- `/resetwarn`, `/resetwarns`: Bot admin + User admin
- `/resetallwarns`: Chat owner only
- `/setwarnmode`, `/setwarnlimit`, `/setwarntime`: Bot admin + User admin
- `/warnings`: Bot admin + User admin
- `/warns`: Any user (disableable)


## Technical Notes

**Notes:**
- When a user reaches the warn limit, the configured action (ban/kick/mute/tban/tmute) is applied automatically
- Warning reasons are stored and displayed when checking a user's warns
- The `/warns` command is the only command in this module that can be disabled by admins
- Anonymous channel posts cannot be warned
- Admins cannot be warned
