---
title: Warns Commands
description: Complete guide to Warns module commands and features
---

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

## Module Aliases

This module can be accessed using the following aliases:

- `warn`
- `warning`
- `warnings`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/dwarn` | Warn a user by reply, and delete their message. | ❌ |
| `/resetallwarns` | Delete all the warnings in a chat. All users return to 0 warns. | ❌ |
| `/resetwarn` | Reset all of a user's warnings to 0. | ❌ |
| `/resetwarns` | No description available | ❌ |
| `/rmwarn` | Remove a user's latest warning. | ❌ |
| `/setwarnlimit` | Set the number of warnings before users are punished. | ❌ |
| `/setwarnmode` | Set the chat's warn mode. | ❌ |
| `/swarn` | Silently warn a user, and delete your message. | ❌ |
| `/unwarn` | No description available | ❌ |
| `/warn` | Warn a user. | ❌ |
| `/warnings` | Get the chat's warning settings. | ❌ |
| `/warns` | See a user's warnings. | ✅ |

## Usage Examples

### Basic Usage

```
/dwarn
/resetallwarns
/resetwarn
```

For detailed command usage, refer to the commands table above.

## Required Permissions

Commands in this module are available to all users unless otherwise specified.
