---
title: LogChannels Commands
description: Complete guide to LogChannels module commands and features
---

# 📜 LogChannels Commands

Send admin actions to a dedicated log channel.

1. Add the bot to a channel as admin.
2. In that channel, send /setlog.
3. Forward that message to the group.

### Admin commands:
- `/setlog`: Set the log channel (send it in the channel, then forward the message to the group).
- `/unsetlog`: Remove the log channel.
- `/logchannel`: Show the current log channel.
- `/logcategories`: List categories.
- `/log <category>`: Enable a category.
- `/nolog <category>`: Disable a category.

Categories: settings, admin, user, automated, reports, other.


## Module Aliases

This module can be accessed using the following aliases:

- `setlog`
- `logchannel`
- `logcategories`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/log` | Enable a category. | ❌ |
| `/logcategories` | List categories. | ❌ |
| `/logchannel` | Show the current log channel. | ❌ |
| `/nolog` | Disable a category. | ❌ |
| `/setlog` | Set the log channel (send it in the channel, then forward the message to the group). | ❌ |
| `/unsetlog` | Remove the log channel. | ❌ |

## Usage Examples

### Basic Usage

```text
/log
/logcategories
/logchannel
```

For detailed command usage, refer to the commands table above.

## Required Permissions

`/setlog` is posted in the log channel by one of its admins, then that post is forwarded to the group by a group admin. `/unsetlog`, `/log` and `/nolog` require a group admin. `/logchannel` and `/logcategories` are open to everyone.

**Bot Permissions Required:**

- Admin in the log channel with permission to post messages (to receive `/setlog` and send the logs)

