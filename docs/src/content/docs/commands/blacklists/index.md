---
title: Blacklists Commands
description: Complete guide to Blacklists module commands and features
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# 📦 Blacklists Commands

*User Commands:*
× /blacklists: Check all the blacklists in chat.

*Admin Commands:*
× /addblacklist `<trigger>`: Blacklists the word in the current chat.
× /rmblacklist `<trigger>`: Removes the word from current Blacklisted Words in Chat.
× /blaction `<mute/kick/ban/warn/none>`: Sets the action to be performed by bot when a blacklist word is detected.
× /blacklistaction: Same as above

*Owner Only:*
× /remallbl /rmallbl: Removes all the blacklisted words from chat

*Note:*
The Default mode for Blacklist is **warn**, which will delete the message and issue a warning to the user.

**Available Actions:**
The following actions can be set using `/blaction`:
- `none` - Just deletes the message without any further action
- `warn` - Deletes message and issues a warning to the user (default)
- `mute` - Deletes message and mutes the user
- `kick` - Deletes message and kicks the user (they can rejoin)
- `ban` - Deletes message and permanently bans the user

**Commands:**
- `/addblacklist &lt;trigger&gt;` - Blacklists the word in the current chat
- `/rmblacklist &lt;trigger&gt;` - Removes the word from current Blacklisted Words in Chat
- `/blaction &lt;mute/kick/ban/warn/none&gt;` - Sets the action to be performed by bot when a blacklist word is detected
- `/remallbl` / `/rmallbl` - Removes all the blacklisted words from chat (Owner Only)


## Module Aliases

> These are help-menu module names, not command aliases.

This module can be accessed using the following aliases:

- `blacklist`
- `unblacklist`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/addblacklist` | Add a trigger to the blacklist | ❌ |
| `/blacklist` | Add a trigger to the blacklist | ❌ |
| `/blacklistaction` | Set action for blacklist triggers | ❌ |
| `/blacklists` | List all blacklisted triggers | ✅ |
| `/blaction` | Set action for blacklist triggers | ❌ |
| `/remallbl` | Remove all blacklist triggers | ❌ |
| `/rmallbl` | Alias of `/remallbl` | ❌ |
| `/rmblacklist` | Remove a trigger from the blacklist | ❌ |

## Usage Examples

### Basic Usage

```
/addblacklist
/blacklist
/blacklistaction
```

For detailed command usage, refer to the commands table above.

## Required Permissions

- `/addblacklist`, `/blacklist` — Requires admin + **Restrict Members** permission (`CanUserRestrict`)
- `/rmblacklist` — Requires admin + **Restrict Members** permission (`CanUserRestrict`)
- `/blaction`, `/blacklistaction` — Requires admin + **Restrict Members** permission (`CanUserRestrict`)
- `/remallbl`, `/rmallbl` — Requires **chat owner** (creator)
- `/blacklists` — Available to all users (disableable)

**Bot must also be admin** with Restrict Members permission for add/remove/action commands.
