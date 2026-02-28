---
title: Connections Commands
description: Complete guide to Connections module commands and features
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# 📦 Connections Commands

This module allows you to connect to a chat's database, and add things to it without the chat knowing about it! For obvious reasons, you need to be an admin to add things; but any member can view your data. (banned/kicked users can't!)

*Commands*:
× /connect `<chatid>`: Connect to the specified chat, allowing you to view/edit contents.
× /disconnect: Disconnect from the current chat.
× /reconnect: Reconnect to the previously connect chat
× /connection: See information about the currently connected chat.

*Admin Commands:*
× /allowconnect <yes/no>: Allow users to connect to chats or not.

You can retrieve the chat id by using the /id command in your chat. Don't be surprised if the id is negative; all super groups have negative ids.

## Module Aliases

> These are help-menu module names, not command aliases.

This module can be accessed using the following aliases:

- `connection`
- `connect`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/allowconnect` | Toggle whether users can connect to this chat | ❌ |
| `/connect` | Connect to a group chat from private messages | ❌ |
| `/connection` | Show current connection status | ❌ |
| `/disconnect` | Disconnect from the connected group chat | ❌ |
| `/reconnect` | Reconnect to the last connected group chat | ❌ |

## Usage Examples

### Basic Usage

```
/allowconnect
/connect
/connection
```

For detailed command usage, refer to the commands table above.

## Required Permissions

Commands in this module are available to all users unless otherwise specified.
