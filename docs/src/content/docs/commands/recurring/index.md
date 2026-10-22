---
title: Recurring Commands
description: Complete guide to Recurring module commands and features
---

# 📦 Recurring Commands

**🔁 Recurring messages**

Admins can have the bot post a message in this chat again and again, for example a reminder of the rules every day. The content works like a note: text with formatting, buttons and fillings such as the chat name, or the replied message, media included. Times are in UTC. A chat can have up to 5 recurring messages, and the shortest interval is 1 hour.

### Admin commands:

- `/recurring add <interval> <reply/text>`: Post the text, or the replied message, every interval (for example 6h, 1d or 1w) in the topic the command is sent in.
Options go between the interval and the text: `start=HH:MM` posts first at this time (UTC), then every interval; `end=YYYY-MM-DD` stops after that day (`end=YYYY-MM-DDTHH:MM` at that time); `delete=on` deletes the previous post when the next one goes out; `topic=ID` posts in that topic instead.
- `/recurring list`: List this chat's recurring messages with their IDs.
- `/recurring rm <id>`: Remove a recurring message.

A message the bot can no longer post (it was removed or the topic is gone) is stopped and stays in the list until it is removed. If the bot may not write in the chat, it skips the posts until it may again.


## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/recurring` | Post the text, or the replied message, every interval (for example 6h, 1d or 1w) in the topic the command is sent in. | ❌ |

## Usage Examples

### Basic Usage

```text
/recurring
```

For detailed command usage, refer to the commands table above.

## Required Permissions

Every `/recurring` form requires **admin permissions** with the right to change group info, the same right `/save` and `/setwelcome` need.

**Bot Permissions Required:**

- Send messages (and send media, for media content)
- Delete messages (only for `delete=on` when the previous post is older than 48 hours)

