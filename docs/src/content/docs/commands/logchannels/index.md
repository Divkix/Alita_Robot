---
title: Logchannels Commands
description: Complete guide to Logchannels module commands and features
---

# 📜 Logchannels Commands

Send admin actions to a dedicated log channel.

1. Add the bot to a channel as admin.
2. In that channel, send /setlog.
3. Forward that message to the group.

### Admin commands:
- `/unsetlog`: Remove the log channel.
- `/logchannel`: Show the current log channel.
- `/logcategories`: List categories.
- `/log <category>`: Enable a category.
- `/nolog <category>`: Disable a category.

Categories: settings, admin, user, automated, reports, other.


## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/log` | Enable a category. | ❌ |
| `/logcategories` | List categories. | ❌ |
| `/logchannel` | Show the current log channel. | ❌ |
| `/nolog` | Disable a category. | ❌ |
| `/setlog` | No description available | ❌ |
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

Most commands in this module require **admin permissions** in the group.

**Bot Permissions Required:**

- Delete messages
- Ban users
- Restrict users
- Pin messages (if applicable)

