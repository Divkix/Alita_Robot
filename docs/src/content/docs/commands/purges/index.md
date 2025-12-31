---
title: Purges Commands
description: Complete guide to Purges module commands for bulk message deletion
---

# 🧹 Purges Commands

The Purges module provides powerful tools for group administrators to bulk-delete messages. All commands require admin permissions with message deletion rights.

## Quick Start

*Admin only:*
- `/purge` - Reply to a message to delete all messages from that point up to your command.
- `/del` - Reply to a message to delete just that single message.
- `/purgefrom` + `/purgeto` - Mark a range of messages to delete using two separate commands.

## Module Aliases

This module can be accessed using the following aliases:

- `purge`
- `del`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/del` | Deletes the single message you replied to, along with the command message. | ❌ |
| `/purge [reason]` | Deletes all messages between the replied-to message and the command. Optionally include a reason. | ❌ |
| `/purgefrom` | Marks the replied-to message as the starting point for a range deletion. Valid for 30 seconds. | ❌ |
| `/purgeto [reason]` | Completes the range deletion by deleting all messages between the `/purgefrom` marker and this message. | ❌ |

## Usage Examples

### Delete a Single Message

Reply to any message and use:
```
/del
```
This removes both the replied message and your `/del` command.

### Bulk Delete Messages

Reply to the oldest message you want to delete, then:
```
/purge
```
All messages from the replied message up to (and including) your `/purge` command will be deleted.

With a reason:
```
/purge spam messages
```

### Range Deletion with purgefrom/purgeto

When you need more control over the deletion range:

1. Reply to the **first** message in the range:
   ```
   /purgefrom
   ```
   The bot will confirm the message is marked (expires after 30 seconds).

2. Reply to the **last** message in the range:
   ```
   /purgeto
   ```
   All messages between the two markers (inclusive) will be deleted.

This is useful when you can't easily scroll to the starting point or need to verify the range before deletion.

## Limits and Restrictions

- **Maximum messages per purge:** 1000 messages
- **Message age limit:** Messages older than 48 hours may not be deletable (Telegram API restriction)
- **Permissions required:** Both the user and the bot must have "Delete Messages" permission

## Required Permissions

**User Permissions Required:**
- Administrator status
- Delete messages permission

**Bot Permissions Required:**
- Administrator status
- Delete messages permission

## Notes

- The `/purgefrom` marker expires after 30 seconds if not followed by `/purgeto`
- Deleting large ranges uses concurrent processing for better performance
- Status messages (showing how many messages were deleted) auto-delete after 3 seconds
- The bot handles already-deleted messages gracefully without errors
