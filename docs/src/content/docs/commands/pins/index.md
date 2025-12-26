---
title: Pins Commands
description: Complete guide to Pins module commands and features
---

# 📦 Pins Commands

All the pin-related commands can be found here; keep your chat up to date on the latest news with a simple pinned message!

*User commands:*
× /pinned: Get the current pinned message.

*Admin commands:*
× /pin: Pin the message you replied to. Add 'loud' or 'notify' to send a notification to group members.
× /pinned: Gets the latest pinned message in the current Chat.
× /permapin <text>: Pin a custom message through the bot. This message can contain markdown, buttons, and all the other cool features.
× /unpin: Unpin the current pinned message. If used as a reply, unpins the replied-to message.
× /unpinall: Unpins all pinned messages.
× /antichannelpin <yes/no/on/off>: Don't let telegram auto-pin linked channels. If no arguments are given, show the current setting.
× /cleanlinked <yes/no/on/off>: Delete messages sent by the linked channel.
Note: When using anti channel pins, make sure to use the /unpin command, instead of doing it manually. Otherwise, the old message will get re-pinned when the channel sends any messages.

## Module Aliases

This module can be accessed using the following aliases:

- `antichannelpin`
- `cleanlinked`
- `pins`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/antichannelpin` | Don't let telegram auto-pin linked channels. If no arguments are given, show the current setting. | ❌ |
| `/cleanlinked` | Delete messages sent by the linked channel. | ❌ |
| `/permapin` | Pin a custom message through the bot. This message can contain markdown, buttons, and all the other cool features. | ❌ |
| `/pin` | Get the current pinned message. | ❌ |
| `/pinned` | Get the current pinned message. | ❌ |
| `/unpin` | Unpin the current pinned message. If used as a reply, unpins the replied-to message. | ❌ |
| `/unpinall` | Unpins all pinned messages. | ❌ |

## Usage Examples

### Basic Usage

```
/antichannelpin
/cleanlinked
/permapin
```

For detailed command usage, refer to the commands table above.

## Required Permissions

Commands in this module are available to all users unless otherwise specified.
