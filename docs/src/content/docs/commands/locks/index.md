---
title: Locks Commands
description: Complete guide to Locks module commands and features
---

# 🔒 Locks Commands

*Admin only*:
× /lock `<permission>`: Lock Chat permission.
× /unlock `<permission>`: Unlock Chat permission.
× /locks: View Chat permission.
× /locktypes: Check available lock types!

Locks can be used to restrict a group's users.
Locking URLs will auto-delete all messages with URLs, locking stickers will delete all stickers, etc.
Locking bots will stop non-admins from adding bots to the chat.

**Example:**
`/lock media`: this locks all the media messages in the chat.

## Module Aliases

This module can be accessed using the following aliases:

- `lock`
- `unlock`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/lock <type>` | Lock a specific content type in the chat | ❌ |
| `/unlock <type>` | Unlock a specific content type in the chat | ❌ |
| `/locks` | View all current lock settings in the chat | ✅ |
| `/locktypes` | View all available lock types | ✅ |

## Available Lock Types

### Permission Locks
These locks delete messages containing specific content types:

| Lock Type | Description |
|-----------|-------------|
| `sticker` | Deletes sticker messages |
| `audio` | Deletes audio files |
| `voice` | Deletes voice messages |
| `document` | Deletes documents (excluding GIFs) |
| `video` | Deletes video messages |
| `videonote` | Deletes video notes (round videos) |
| `contact` | Deletes contact shares |
| `photo` | Deletes photo messages |
| `gif` | Deletes GIF/animation messages |
| `url` | Deletes messages containing URLs |
| `bots` | Prevents non-admins from adding bots |
| `forward` | Deletes forwarded messages |
| `game` | Deletes game messages |
| `location` | Deletes location shares |
| `rtl` | Deletes messages with Arabic/RTL text |
| `anonchannel` | Deletes messages from anonymous/linked channels |

### Restriction Locks
These locks delete broader categories of content:

| Lock Type | Description |
|-----------|-------------|
| `messages` | Deletes all text messages |
| `comments` | Deletes comments from users not in the group (discussion comments) |
| `media` | Deletes all media (audio, documents, video notes, videos, voices, photos) |
| `other` | Deletes games, stickers, and animations |
| `previews` | Deletes messages with URL previews |
| `all` | Deletes all messages (use with caution!) |

## Usage Examples

### Locking Multiple Types

```
/lock sticker gif audio
```
Locks stickers, GIFs, and audio files simultaneously.

### Unlocking Multiple Types

```
/unlock sticker gif audio
```
Unlocks stickers, GIFs, and audio files simultaneously.

### View Current Locks

```
/locks
```
Shows a list of all locks with their current status (true/false).

## Required Permissions

Most commands in this module require **admin permissions** in the group.

**Bot Permissions Required:**
- Delete messages (for deleting locked content)
- Ban users (for banning bots when `bots` lock is enabled)

## Technical Notes

- Lock enforcement is applied to all messages in real-time
- Admins are exempt from all locks
- Locks are stored per-chat and persist across bot restarts
- The `comments` lock is designed for linked discussion groups to filter comments from non-members
- Cache is automatically invalidated when locks are updated for immediate enforcement
