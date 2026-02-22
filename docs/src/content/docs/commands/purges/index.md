---
title: Purges Commands
description: Complete guide to Purges module commands and features
---

# Purges Commands

Bulk-delete messages between two points, or delete individual messages by reply.

:::caution[Admin Permissions Required]
All commands in this module require admin permissions with "Delete Messages" permission.
:::

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/purge` | Deletes all messages between this and the replied-to message. | No |
| `/del` | Deletes the message you replied to. | No |
| `/purgefrom` | Mark a message as the starting point for range deletion. | No |
| `/purgeto` | Mark the ending point and delete all messages in the range. | No |

## Usage Examples

```text
/purge          # Reply to a message to delete everything from there to now
/del            # Reply to a message to delete just that one
```

### Range Deletion with purgefrom/purgeto

When you need more control over the deletion range:

1. Reply to the **first** message in the range:
   ```text
   /purgefrom
   ```
   The bot will confirm the message is marked (expires after 30 seconds).

2. Reply to the **last** message in the range:
   ```text
   /purgeto
   ```
   All messages between the two markers (inclusive) will be deleted.

:::tip
Range deletion with `/purgefrom` and `/purgeto` is useful when you cannot easily scroll to the starting point or need to verify the range before deletion.
:::

:::note[Limits and Restrictions]
- **Maximum messages per purge:** 1000 messages
- **Message age limit:** Messages older than 48 hours may not be deletable (Telegram API restriction)
- **Permissions required:** Both the user and the bot must have "Delete Messages" permission
:::

## Module Aliases

This module can be accessed using the following aliases:
`purge`, `del`

## Required Permissions

**Bot Permissions Required:**
- Delete messages

## Technical Notes

- The `/purgefrom` marker expires after 30 seconds if not followed by `/purgeto`
- Deleting large ranges uses concurrent processing for better performance
- Status messages (showing how many messages were deleted) auto-delete after 3 seconds
- The bot handles already-deleted messages gracefully without errors
