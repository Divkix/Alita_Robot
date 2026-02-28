---
title: Purges Commands
description: Complete guide to Purges module commands and features
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# 🧹 Purges Commands

*Admin only:*
- /purge: deletes all messages between this and the replied-to message.
- /del: deletes the message you replied to.

*Examples*:
- Delete all messages from the replied message, until now.
-> `/purge`

**Range Deletion with purgefrom/purgeto:**
When you need more control over the deletion range:

1. Reply to the **first** message in the range:
`/purgefrom`
The bot will confirm the message is marked (expires after 30 seconds).

2. Reply to the **last** message in the range:
`/purgeto`
All messages between the two markers (inclusive) will be deleted.

This is useful when you can't easily scroll to the starting point or need to verify the range before deletion.

**Limits and Restrictions:**
- **Maximum messages per purge:** 1000 messages
- **Message age limit:** Messages older than 48 hours may not be deletable (Telegram API restriction)
- **Permissions required:** Both the user and the bot must have "Delete Messages" permission


## Module Aliases

> These are help-menu module names, not command aliases.

This module can be accessed using the following aliases:

- `purge`
- `del`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/del` | Delete the replied message | ❌ |
| `/purge` | Purge messages from replied message to current | ❌ |
| `/purgefrom` | Mark start point for a purge range | ❌ |
| `/purgeto` | Purge messages from marked start to this message | ❌ |

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

## Technical Notes

**Notes:**
- The `/purgefrom` marker expires after 30 seconds if not followed by `/purgeto`
- Deleting large ranges uses concurrent processing for better performance
- Status messages (showing how many messages were deleted) auto-delete after 3 seconds
- The bot handles already-deleted messages gracefully without errors
