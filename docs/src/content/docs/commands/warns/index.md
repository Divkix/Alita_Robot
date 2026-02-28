---
title: Warns Commands
description: Complete guide to Warns module commands and features
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# 📦 Warns Commands

Keep your members in check with warnings; stop them getting out of control!
If you're looking for automated warnings, read about the blacklist module!

*Admin Commands:*
- /warn <reason>: Warn a user.
- /dwarn <reason>: Warn a user by reply, and delete their message.
- /swarn <reason>: Silently warn a user, and delete your message.
- /warns: See a user's warnings.
- /rmwarn: Remove a user's latest warning.
- /resetwarn: Reset all of a user's warnings to 0.
- /resetallwarns: Delete all the warnings in a chat. All users return to 0 warns.
- /warnings: Get the chat's warning settings.
- /setwarnmode <ban/kick/mute>: Set the chat's warn mode.
- /setwarnlimit <number>: Set the number of warnings before users are punished.

*Examples*
- Warn a user.
-> `/warn @user For disobeying the rules`

**Default Settings:**
- **Default warn limit:** 3 warnings
- **Default warn mode:** mute

**Available Warn Modes:**
- `ban` - Permanently ban the user from the chat
- `kick` - Kick the user (they can rejoin)
- `mute` - Mute the user (cannot send messages)

**Silent and Delete Warnings:**
- `/swarn` - Delete your command and warn silently
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
| `/resetallwarns` | Reset all warnings for all users in the chat | ❌ |
| `/resetwarn` | Reset all warnings for a user | ❌ |
| `/resetwarns` | Reset all warnings for a user | ❌ |
| `/rmwarn` | Remove the last warning from a user | ❌ |
| `/setwarnlimit` | Set maximum warnings before action | ❌ |
| `/setwarnmode` | Set action taken when warn limit is reached | ❌ |
| `/swarn` | Warn a user silently and delete your command | ❌ |
| `/unwarn` | Remove the last warning from a user | ❌ |
| `/warn` | Warn a user | ❌ |
| `/warnings` | Show warning count for a user | ❌ |
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
- `/warn`, `/dwarn`, `/swarn`: Bot admin + User admin with restrict
- `/rmwarn`, `/unwarn`: Bot admin + User admin
- `/resetwarn`, `/resetwarns`: Bot admin + User admin
- `/resetallwarns`: Chat owner only
- `/setwarnmode`, `/setwarnlimit`: Bot admin + User admin
- `/warnings`: Bot admin + User admin
- `/warns`: Any user (disableable)


## Technical Notes

**Notes:**
- When a user reaches the warn limit, the configured action (ban/kick/mute) is applied automatically
- Warning reasons are stored and displayed when checking a user's warns
- The `/warns` command is the only command in this module that can be disabled by admins
- Anonymous channel posts cannot be warned
- Admins cannot be warned
