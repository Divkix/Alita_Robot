---
title: Blacklists Commands
description: Complete guide to Blacklists module commands and features
---

# 📦 Blacklists Commands

*User Commands:*
× /blacklists: Check all the blacklists in chat.

*Admin Commands:*
× /addblacklist `<trigger>`: Blacklists the word in the current chat.
× /rmblacklist `<trigger>`: Removes the word from current Blacklisted Words in Chat.
× /blaction `<mute/kick/ban/warn/none>`: Sets the action to be performed by bot when a blacklist word is detected.
× /blacklistaction: Same as above

*Owner Only:*
× /remallbl: Removes all the blacklisted words from chat

*Note:*
The Default mode for Blacklist is *warn*, which will delete the message and issue a warning to the user.

## Available Actions

The following actions can be set using `/blaction`:

| Action | Description |
|--------|-------------|
| `none` | Just deletes the message without any further action |
| `warn` | Deletes message and issues a warning to the user (default) |
| `mute` | Deletes message and mutes the user |
| `kick` | Deletes message and kicks the user (they can rejoin) |
| `ban` | Deletes message and permanently bans the user |

## Module Aliases

This module can be accessed using the following aliases:

- `blacklist`
- `unblacklist`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/addblacklist` | Blacklists the word in the current chat. | ❌ |
| `/blacklist` | Check all the blacklists in chat. | ❌ |
| `/blacklistaction` | Same as above | ❌ |
| `/blacklists` | Check all the blacklists in chat. | ✅ |
| `/blaction` | Sets the action to be performed by bot when a blacklist word is detected. | ❌ |
| `/rmblacklist` | Removes the word from current Blacklisted Words in Chat. | ❌ |
| `/rmallbl` | Removes all blacklisted words from chat. (Owner only) | ❌ |
| `/remallbl` | Alias for `/rmallbl`. (Owner only) | ❌ |

## Usage Examples

### Basic Usage

```
/addblacklist
/blacklist
/blacklistaction
```

For detailed command usage, refer to the commands table above.

## Required Permissions

Commands in this module are available to all users unless otherwise specified.
