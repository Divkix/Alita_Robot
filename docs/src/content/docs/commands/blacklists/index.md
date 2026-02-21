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

**Available Actions:**
The following actions can be set using `/blaction`:
- `none` - Just deletes the message without any further action
- `warn` - Deletes message and issues a warning to the user (default)
- `mute` - Deletes message and mutes the user
- `kick` - Deletes message and kicks the user (they can rejoin)
- `ban` - Deletes message and permanently bans the user

**Note:**
The Default mode for Blacklist is **warn**, which will delete the message and issue a warning to the user.

**Commands:**
- `/addblacklist &lt;trigger&gt;` - Blacklists the word in the current chat
- `/rmblacklist &lt;trigger&gt;` - Removes the word from current Blacklisted Words in Chat
- `/blaction &lt;mute/kick/ban/warn/none&gt;` - Sets the action to be performed by bot when a blacklist word is detected
- `/remallbl` - Removes all the blacklisted words from chat (Owner Only)


## Module Aliases

This module can be accessed using the following aliases:

- `blacklist`
- `unblacklist`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/addblacklist` | Blacklists the word in the current chat. | ❌ |
| `/blacklist` | Add words to the blacklist. Alias for /addblacklist. | ❌ |
| `/blacklistaction` | Set the action to take when a blacklisted word is detected. | ❌ |
| `/blacklists` | Check all the blacklists in chat. | ✅ |
| `/blaction` | Sets the action to be performed by bot when a blacklist word is detected. | ❌ |
| `/rmallbl` | Remove all blacklisted words from the chat. | ❌ |
| `/remallbl` | Remove all blacklisted words from the chat. Alias for /rmallbl. | ❌ |
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

Most commands require admin with restrict permissions. `/blacklists` (list command) is available to all users. `/rmallbl` requires chat owner.
