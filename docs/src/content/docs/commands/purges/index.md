---
title: Purges Commands
description: Complete guide to Purges module commands and features
---

# 🧹 Purges Commands

*Admin only:*
- /purge: deletes all messages between this and the replied-to message.
- /del: deletes the message you replied to.

*Examples*:
- Delete all messages from the replied message, until now.
-> `/purge`

## Module Aliases

This module can be accessed using the following aliases:

- `purge`
- `del`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/del` | deletes the message you replied to. | ❌ |
| `/purge` | deletes all messages between this and the replied-to message. | ❌ |
| `/purgefrom` | No description available | ❌ |
| `/purgeto` | No description available | ❌ |

## Usage Examples

### Basic Usage

```
/del
/purge
/purgefrom
```

For detailed command usage, refer to the commands table above.

## Required Permissions

Most commands in this module require **admin permissions** in the group.

**Bot Permissions Required:**

- Delete messages
- Ban users
- Restrict users
- Pin messages (if applicable)
