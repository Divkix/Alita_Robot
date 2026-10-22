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
- `/setlog`: Set the log channel (send it in the channel, then forward the message to the group). In PM, name the group and then the channel, if you are an admin of both and the bot can post in the channel.
- `/unsetlog`: Remove the log channel.
- `/log <category>`: Enable a category.
- `/nolog <category>`: Disable a category.

### Anyone:
- `/logchannel`: Show the current log channel.
- `/logcategories`: List categories.

Categories: settings (log channel setting changes), admin (bans, kicks, mutes, unbans and unmutes), user (/kickme and /banme), reports (user reports), automated, other. Nothing is logged under automated or other yet.

From PM, name the group first in /setlog, /unsetlog, /log, /nolog and /logchannel: its title, @username or ID.


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
| `/setlog` | Set the log channel (send it in the channel, then forward the message to the group). In PM, name the group and then the channel, if you are an admin of both and the bot can post in the channel. | ❌ |
| `/unsetlog` | Remove the log channel. | ❌ |

## Usage Examples

**Set up a log channel:**
1. Add the bot to the channel as an admin that can post messages.
2. Send `/setlog` in the channel.
3. Within an hour, forward that /setlog message to your group.

**Set up and manage it from PM:**
In PM, name the group first: its ID, its @username, or the title of a group where you are an admin (case does not matter; quote a title with spaces). Nothing is posted in the group, and the channel gets only the usual settings entry.
- `/setlog @mygroup @mychannel` - bind a channel by its @username
- `/setlog -1001234567890 -1009876543210` - bind it by IDs
- `/log @mygroup admin` - turn on a category
- `/nolog -1001234567890 settings` - turn off a category
- `/logchannel @mygroup` - show the log channel
- `/unsetlog @mygroup` - remove it
You must be an admin of the group and either own the channel or be its admin with permission to post messages, as posting /setlog there would need; the bot must be an admin of the channel that can post messages. When you are connected with /connect, the group may be left out: `/setlog @mychannel`. While you are connected, /setlog, /log and /nolog do not look up a title: name another group by its @username or ID. The group passes the same checks as /connect, and the connection stays as it is.

**Choose what gets logged:**
- `/logcategories` - list the categories
- `/log admin user` - turn on two categories
- `/nolog settings` - turn off one category


## Required Permissions

**User Permissions:**
- `/setlog` is sent in the channel itself, where only channel admins can post.
- Only group admins can bind the channel by forwarding that message, and only they can use `/unsetlog`, `/log` and `/nolog`.
- `/setlog` in PM binds the channel only for an admin of the group who owns the channel or is its admin with permission to post messages.
- Anyone can use `/logchannel` and `/logcategories`; in PM, `/logchannel` works for a group you could connect to with `/connect`.

**Bot Permissions:**
- The bot must be an admin of the log channel with permission to post messages, both to see `/setlog` and to send log entries. Entries it cannot send are dropped without an error.


