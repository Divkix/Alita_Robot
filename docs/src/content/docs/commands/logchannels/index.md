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
- `/log <category>`: Enable a category.
- `/nolog <category>`: Disable a category.

### Anyone:
- `/logchannel`: Show the current log channel (groups only).
- `/logcategories`: List categories.

Categories: settings (log channel setting changes), admin (bans, kicks, mutes, unbans and unmutes), user (/kickme and /banme), reports (user reports), automated, other. Nothing is logged under automated or other yet.


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
| `/logchannel` | Show the current log channel (groups only). | ❌ |
| `/nolog` | Disable a category. | ❌ |
| `/setlog` | Set the log channel (send it in the channel, then forward the message to the group). | ❌ |
| `/unsetlog` | Remove the log channel. | ❌ |

## Usage Examples

**Set up a log channel:**
1. Add the bot to the channel as an admin that can post messages.
2. Send `/setlog` in the channel.
3. Within an hour, forward that /setlog message to your group.

**Choose what gets logged:**
- `/logcategories` - list the categories
- `/log admin user` - turn on two categories
- `/nolog settings` - turn off one category


## Required Permissions

**User Permissions:**
- `/setlog` is sent in the channel itself, where only channel admins can post.
- Only group admins can bind the channel by forwarding that message, and only they can use `/unsetlog`, `/log` and `/nolog`.
- Anyone can use `/logchannel` (groups only) and `/logcategories`.

**Bot Permissions:**
- The bot must be an admin of the log channel with permission to post messages, both to see `/setlog` and to send log entries. Entries it cannot send are dropped without an error.


