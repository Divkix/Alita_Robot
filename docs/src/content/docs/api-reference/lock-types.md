---
title: Lock Types
description: Complete reference of all available lock types in the Locks module
---

# 🔒 Lock Types Reference

This page documents all available lock types in the Locks module. Locks allow administrators to restrict specific types of content or actions in their groups.

## Overview

- **Total Lock Types**: 22
- **Permission Locks**: 16 (specific content types)
- **Restriction Locks**: 6 (broad categories)

## How Locks Work

Locks prevent non-admin users from posting specific types of content. When a lock is enabled, the bot will automatically delete matching messages from regular users.

### Usage

```
/lock <lock_type> [lock_type2 ...]
/unlock <lock_type> [lock_type2 ...]
/locks - View all currently enabled locks
/locktypes - View all available lock types
```

## Restriction Locks

Restriction locks affect broad categories of messages. These are powerful locks that can block multiple content types at once.

| Lock Type | Description |
|-----------|-------------|
| `all` | Blocks all messages from non-admins |
| `comments` | Blocks messages from non-members (discussion comments) |
| `media` | Blocks all media files (audio, document, video, photo, video note, voice) |
| `messages` | Blocks all text and media messages |
| `other` | Blocks games, stickers, and GIFs |
| `previews` | Blocks messages with URL previews |

## Permission Locks

Permission locks target specific types of content or actions. Use these for fine-grained control over what users can post.

| Lock Type | Description |
|-----------|-------------|
| `anonchannel` | Blocks messages from anonymous channels and linked channel posts |
| `audio` | Blocks audio file messages |
| `bots` | Prevents non-admins from adding bots to the group |
| `contact` | Blocks contact card messages |
| `document` | Blocks document files (excludes GIFs/animations) |
| `forward` | Blocks forwarded messages |
| `game` | Blocks game messages |
| `gif` | Blocks GIF/animation messages |
| `location` | Blocks location/venue messages |
| `photo` | Blocks photo messages |
| `rtl` | Blocks messages containing right-to-left (Arabic) text |
| `sticker` | Blocks sticker messages |
| `url` | Blocks messages containing URLs |
| `video` | Blocks video messages |
| `videonote` | Blocks video note messages (round videos) |
| `voice` | Blocks voice messages |

## Media Type Locks

These locks control specific types of media content:

- **`photo`**: Blocks photo messages
- **`video`**: Blocks video messages
- **`audio`**: Blocks audio file messages
- **`voice`**: Blocks voice messages
- **`document`**: Blocks document files (excludes GIFs/animations)
- **`gif`**: Blocks GIF/animation messages
- **`sticker`**: Blocks sticker messages
- **`videonote`**: Blocks video note messages (round videos)

## Content Behavior Locks

These locks control how content behaves:

- **`forward`**: Blocks forwarded messages
- **`url`**: Blocks messages containing URLs
- **`previews`**: Blocks messages with URL previews
- **`rtl`**: Blocks messages containing right-to-left (Arabic) text
- **`anonchannel`**: Blocks messages from anonymous channels and linked channel posts
- **`comments`**: Blocks messages from non-members (discussion comments)

## Special Locks

### `bots`

Prevents non-admins from adding bots to the group

**Behavior**: When enabled, the bot will automatically ban any bot added by non-admins.

### `all`

Blocks all messages from non-admins

**Use Case**: Useful for temporarily freezing chat activity or creating read-only channels.

## Examples

### Prevent Media Spam

```
/lock media
```
Blocks all media files (audio, documents, videos, photos, video notes, and voice messages).

### Create Read-Only Chat

```
/lock all
```
Prevents all non-admin users from sending any messages.

### Block Forwarded Content

```
/lock forward
```
Prevents users from forwarding messages from other chats.

### Prevent Bot Addition

```
/lock bots
```
Only admins can add bots to the group.

### Multiple Locks at Once

```
/lock sticker gif video
```
Lock multiple content types in a single command.

## Important Notes

### Admin Exemption

All locks automatically exempt administrators. Admins can always post any content type, regardless of which locks are enabled.

### Bot Permissions

The bot requires the following permissions to enforce locks:

- **Delete Messages**: Required to remove locked content
- **Ban Users**: Only for the `bots` lock (to ban unauthorized bot additions)

### Interaction with Other Modules

Locks work independently but complement other moderation modules:

- **Antiflood**: Locks control content types, antiflood controls message frequency
- **Filters**: Custom filters can block specific words/patterns, locks block content types
- **Blacklist**: Blacklist blocks specific words, locks block entire categories

## Related Commands

- `/lock <type>` - Enable one or more locks
- `/unlock <type>` - Disable one or more locks
- `/locks` - View currently enabled locks
- `/locktypes` - View all available lock types

For more information, see the [Locks module documentation](/commands/locks/).
