---
title: Disabling Commands
description: Complete guide to Disabling module commands and features
---

# Disabling Commands

This module allows you to disable commonly used commands in your group, preventing non-admin users from using them. You can also configure automatic deletion of disabled command messages to keep your chat clean.

## Key Features

- Disable specific bot commands for non-admin users
- Admins can always bypass disabled commands
- Optional automatic deletion of disabled command messages
- Support for disabling multiple commands at once

## Admin Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/disable <command> [command2...]` | Disable one or more commands in this group | No |
| `/enable <command> [command2...]` | Re-enable one or more disabled commands | No |
| `/disableable` | List all commands that can be disabled | No |
| `/disabled` | List currently disabled commands in this chat | Yes |
| `/disabledel <on/off>` | Toggle automatic deletion of disabled commands | No |

## Usage Examples

### Disable a Single Command

```
/disable adminlist
```

This prevents non-admin users from using the `/adminlist` command.

### Disable Multiple Commands

```
/disable adminlist rules info
```

Disables `/adminlist`, `/rules`, and `/info` commands at once.

### Enable Commands

```
/enable adminlist rules
```

Re-enables the `/adminlist` and `/rules` commands.

### Check Disabled Commands

```
/disabled
```

Shows all currently disabled commands in the chat.

### Toggle Command Deletion

```
/disabledel on
```

When enabled, messages containing disabled commands from non-admins will be automatically deleted.

```
/disabledel off
```

Disabled commands will be blocked but messages won't be deleted.

```
/disabledel
```

Check the current deletion setting.

## Important Notes

- **Admin Bypass**: Disabled commands only affect non-admin users. All admins can still use any command.
- **Connection Support**: Disabled commands are still accessible through the `/connect` feature for connected chats.
- **Multiple Commands**: Both `/disable` and `/enable` support multiple command names in a single message.
- **Error Handling**: If some commands fail to disable/enable, you'll be notified about which ones succeeded and which failed.

## Required Permissions

- `/disable`, `/enable`, `/disabledel`: Admin only
- `/disableable`, `/disabled`: All users (but `/disabled` can be disabled)
