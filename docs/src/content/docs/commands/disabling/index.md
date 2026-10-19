---
title: Disabling Commands
description: Complete guide to Disabling module commands and features
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# ❌ Disabling Commands

This module allows you to disable some commonly used commands, So, no one can use them. It'll also allow you to autodelete them, stopping people from blue texting.

*User commands*:
× /disableable: List all disableable commands.
× /disabled: List the disabled commands in this chat.

*Admin commands*:
× /disable `<commandname>`: Stop users from using commandname in this group.
× /enable `<commandname>`: Allow users to use commandname in this group again.
× /disabledel `<yes/no/on/off>`: Delete disabled commands when used by non-admins.

Note:
When disabling a command, the command only gets disabled for non-admins. All admins can still use those commands.
Disabled commands are still accessible through the /connect feature. If you would be interested to see this disabled too, let me know in the support chat.

**Key Features:**
- Disable specific bot commands for non-admin users
- Admins can always bypass disabled commands
- Optional automatic deletion of disabled command messages
- Support for disabling multiple commands at once

**Disable Multiple Commands:**
`/disable adminlist rules info`
Disables `/adminlist`, `/rules`, and `/info` commands at once.

**Toggle Command Deletion:**
`/disabledel on` - Disabled messages will now be deleted.
`/disabledel off` - Disabled messages will no longer be deleted.
`/disabledel` - Check the current deletion setting.


## Module Aliases

> These are help-menu module names, not command aliases.

This module can be accessed using the following aliases:

- `disable`
- `enable`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/disable` | Disable a command in this chat | ❌ |
| `/disableable` | List all commands that can be disabled | ❌ |
| `/disabled` | List all currently disabled commands | ✅ |
| `/disabledel` | Toggle deletion of disabled command messages | ❌ |
| `/enable` | Re-enable a disabled command | ❌ |

## Usage Examples

### Basic Usage

```
/disable
/disableable
/disabled
```

For detailed command usage, refer to the commands table above.

## Required Permissions

- `/disable`, `/enable`, `/disabledel` — **Group admins only**, and the bot must be an admin in the group. In PM they act on the group you connected with `/connect`.
- `/disabled` — Available to all users, but the bot must be an admin in the group. In PM it needs an active `/connect`. `/disabled` can itself be disabled.
- `/disableable` — Available to all users, in any chat, including PM.

## Technical Notes

**Important Notes:**
- **Admin Bypass:** Disabled commands only affect non-admin users. All admins can still use any command.
- **Only listed commands:** `/disable` accepts only the commands `/disableable` lists. Backup's `/export`, `/import` and `/reset` are not on that list.
- **Connection Support:** Disabled commands are still accessible through the `/connect` feature for connected chats.
- **Multiple Commands:** Both `/disable` and `/enable` support multiple command names in a single message.
