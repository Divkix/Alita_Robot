---
title: Misc Commands
description: Complete guide to Misc module commands and features
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# 🔧 Misc Commands

× /info: Get your user info, which can be used as a reply or by passing a User Id or Username.
× /id [user]: Get your ID, and the group's ID when used in a group. Pass a username or user ID, or reply to a message, to get that user's ID (a reply also shows forwarded-from, GIF and sticker IDs).
× /ping: Ping the Telegram Server!
× /tr <lang code> <text>: Translate the text. Reply to a message with /tr [lang code] to translate it (English if no code is given).
× /removebotkeyboard: Removes the stuck bot keyboard from your chat.
× /stat: Gets the count of the total number of messages in the chat.
× /tell <text>: (admin, groups only) Reply to a message to make the bot send <text> as a reply to it; your command message is deleted.
× /stickerid: Reply to a sticker to get its file ID.
× /zombies: Kick deleted accounts the bot knows about (admin).
× /github <user|user/repo>: Look up a GitHub user or repository.
× /wiki <search>: Search English Wikipedia.
× /ud <term>: Look up Urban Dictionary.
× /runs: A random quip.

## Module Aliases

> These are help-menu module names, not command aliases.

This module can be accessed using the following aliases:

- `extra`
- `extras`
- `stickerid`
- `github`
- `wiki`
- `ud`
- `runs`
- `zombies`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/github` | Look up a GitHub user or repository | ✅ |
| `/id` | Get chat or user ID | ✅ |
| `/info` | Get user info | ✅ |
| `/ping` | Ping the Telegram server | ✅ |
| `/removebotkeyboard` | Remove stuck bot keyboard | ❌ |
| `/runs` | Reply with a random quip | ✅ |
| `/stat` | Show message count for the chat | ✅ |
| `/stickerid` | Get a replied sticker's file ID | ✅ |
| `/tell` | Echo a message through the bot | ❌ |
| `/tr` | Translate a message | ✅ |
| `/ud` | Look up a term on Urban Dictionary | ✅ |
| `/wiki` | Search English Wikipedia | ✅ |
| `/zombies` | Kick deleted accounts from the group | ❌ |

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
- `/zombies` — Requires **Admin**, works in groups only (`RequireGroup` +
  `RequireUserAdmin`), and the bot needs the right to restrict members
  (`CanBotRestrict`). Kicks tracked members whose first name is empty or
  'Deleted Account'.
- All other commands (`/github`, `/id`, `/info`, `/ping`, `/removebotkeyboard`,
  `/runs`, `/stat`, `/stickerid`, `/tr`, `/ud`, `/wiki`) — Available to all users.

## `/ping` Metrics

`/ping` reports three latency measurements:

| Metric | Source | Description |
|--------|--------|-------------|
| **API RTT** | `getMe` call | Baseline network round-trip to Telegram API |
| **Send msg** | `sendMessage` RTT | Full latency including Telegram message processing |
| **Overhead** | `Send - API RTT` | Estimated server-side processing overhead |

## `/stat` Details

- `/stat` is **group-only** (`RequireGroup`); in a private chat the bot replies that the command must be used in a group.
- The message count is **approximate**: it uses `MessageId + 1` of the received
  command message, which may differ from actual message count if messages were
  deleted by users or admins.
