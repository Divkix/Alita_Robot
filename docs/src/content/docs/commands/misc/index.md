---
title: Misc Commands
description: Complete guide to Misc module commands and features
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# 🔧 Misc Commands

× /info: Get your user info, which can be used as a reply or by passing a User Id or Username.
× /id: Get the current group id. If used by replying to a message, get that user's id.
× /ping: Ping the Telegram Server!
× /tr <lang code> <msg/reply to message>: Translate the message.
× /removebotkeyboard: Removes the stuck bot keyboard from your chat.
× /stat: Gets the count of the total number of messages in the chat.

## Module Aliases

> These are help-menu module names, not command aliases.

This module can be accessed using the following aliases:

- `extra`
- `extras`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/id` | Get chat or user ID | ✅ |
| `/info` | Get user info | ✅ |
| `/ping` | Ping the Telegram server | ✅ |
| `/removebotkeyboard` | Remove stuck bot keyboard | ❌ |
| `/stat` | Show message count for the chat | ✅ |
| `/tell` | Echo a message through the bot | ❌ |
| `/tr` | Translate a message | ✅ |

## Usage Examples

### Basic Usage

```
/id
/info
/ping
```

For detailed command usage, refer to the commands table above.

## Required Permissions

- `/tell` — Requires **Admin** (`RequireGroup` + `IsUserAdmin`). Must be used
  as a reply to another message. The bot deletes the original command message
  and echoes the command arguments as a reply to the target message.
- All other commands (`/id`, `/info`, `/ping`, `/removebotkeyboard`, `/stat`,
  `/tr`) — Available to all users.

## `/ping` Metrics

`/ping` reports three latency measurements:

| Metric | Source | Description |
|--------|--------|-------------|
| **API RTT** | `getMe` call | Baseline network round-trip to Telegram API |
| **Send msg** | `sendMessage` RTT | Full latency including Telegram message processing |
| **Overhead** | `Send - API RTT` | Estimated server-side processing overhead |

## `/stat` Details

- `/stat` is **group-only** (`RequireGroup`); it does nothing in private chats.
- The message count is **approximate**: it uses `MessageId + 1` of the received
  command message, which may differ from actual message count if messages were
  deleted by users or admins.
