---
title: Disabling Commands
description: Complete guide to Disabling module commands and features
---

# ❌ Disabling Commands

This module allows you to disable some commonly used commands, So, no one can use them. It'll also allow you to autodelete them, stopping people from blue texting.

*Admin commands*:
× /disable `<commandname>`: Stop users from using commandname in this group.
× /enable `<item name>`: Allow users to use commandname in this group.
× /disableable: List all disableable commands.
× /disabledel `<yes/no/on/off>`: Delete disabled commands when used by non-admins.
× /disabled: List the disabled commands in this chat.

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

This module can be accessed using the following aliases:

- `disable`
- `enable`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/disable` | Stop users from using commandname in this group. | ❌ |
| `/disableable` | List all disableable commands. | ❌ |
| `/disabled` | List the disabled commands in this chat. | ✅ |
| `/disabledel` | Delete disabled commands when used by non-admins. | ❌ |
| `/enable` | Allow users to use commandname in this group. | ❌ |

## Usage Examples

### Basic Usage

```
/disable
/disableable
/disabled
```

For detailed command usage, refer to the commands table above.

## Required Permissions

Commands in this module are available to all users unless otherwise specified.

## Technical Notes

**Important Notes:**
- **Admin Bypass:** Disabled commands only affect non-admin users. All admins can still use any command.
- **Connection Support:** Disabled commands are still accessible through the `/connect` feature for connected chats.
- **Multiple Commands:** Both `/disable` and `/enable` support multiple command names in a single message.
- **Error Handling:** If some commands fail to disable/enable, you'll be notified about which ones succeeded and which failed.

**Required Permissions:**
- `/disable`, `/enable`, `/disabledel`: Admin only
- `/disableable`, `/disabled`: All users (but `/disabled` can be disabled)
