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
The Default mode for Blacklist is *none*, which will just delete the messages from the chat.

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
