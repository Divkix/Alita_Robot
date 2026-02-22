---
title: Disabling Commands
description: Complete guide to Disabling module commands and features
---

# Disabling Commands

This module allows you to disable some commonly used commands so no one can use them. It also allows you to auto-delete disabled command messages, stopping people from blue-texting.

:::caution[Admin Permissions Required]
`/disable`, `/enable`, and `/disabledel` require admin permissions. `/disableable` and `/disabled` are available to all users.
:::

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/disable` | Stop users from using a command in this group. | No |
| `/enable` | Allow users to use a command in this group. | No |
| `/disableable` | List all disableable commands. | No |
| `/disabled` | List the disabled commands in this chat. | Yes |
| `/disabledel` | Delete disabled commands when used by non-admins. | No |

## Usage Examples

```text
/disable adminlist rules info    # Disable multiple commands at once
/enable adminlist                # Re-enable a command
/disableable                     # See all disableable commands
/disabled                        # See what is disabled in this chat
/disabledel on                   # Auto-delete disabled command usage
/disabledel off                  # Stop auto-deleting
```

:::note[Admin Bypass]
When disabling a command, the command only gets disabled for non-admins. All admins can still use those commands.
:::

:::tip[Disable Multiple Commands at Once]
Both `/disable` and `/enable` support multiple command names in a single message:

```text
/disable adminlist rules info
```

This disables `/adminlist`, `/rules`, and `/info` at once.
:::

## Key Features

- Disable specific bot commands for non-admin users
- Admins can always bypass disabled commands
- Optional automatic deletion of disabled command messages
- Support for disabling multiple commands at once

## Module Aliases

This module can be accessed using the following aliases:
`disable`, `enable`

## Technical Notes

- **Connection Support:** Disabled commands are still accessible through the `/connect` feature for connected chats.
- **Error Handling:** If some commands fail to disable/enable, you will be notified about which ones succeeded and which failed.
