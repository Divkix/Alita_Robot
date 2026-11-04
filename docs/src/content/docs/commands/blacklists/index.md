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
× /blaction `<mute/kick/ban/warn/none/tban/tmute> [duration]`: Sets the action to be performed by bot when a blacklist word is detected. Temporary actions require a duration, such as `/blaction tban 1d` or `/blaction tmute 2h`. With no arguments, shows the current action and duration.
× /blacklistaction: Same as above

*Owner Only:*
× /remallbl /rmallbl /rmblocklistall /rmallblocklist: Removes all the blacklisted words from chat

*Note:*
The Default mode for Blacklist is **warn**, which will delete the message and issue a warning to the user.

**Available Actions:**
The following actions can be set using `/blaction`:
- `none` - Just deletes the message without any further action
- `warn` - Deletes message and issues a warning to the user (default)
- `mute` - Deletes message and mutes the user
- `kick` - Deletes message and kicks the user (they can rejoin)
- `ban` - Deletes message and permanently bans the user
- `tban` - Deletes message and bans the user for the required duration
- `tmute` - Deletes message and mutes the user for the required duration

Temporary durations range from **1 minute to 366 days**, using `m`, `h`, `d`, or `w`. The action and duration apply to every word, including words added later. Switching to an untimed action clears the duration. Older temporary-action settings without a usable duration keep the **one-hour** fallback; temporary actions do not punish channel senders.

**Commands:**
- `/addblacklist &lt;trigger&gt;` - Blacklists the word in the current chat
- `/rmblacklist &lt;trigger&gt;` - Removes the word from current Blacklisted Words in Chat
- `/blaction &lt;mute/kick/ban/warn/none/tban/tmute&gt; [duration]` - Sets the action to be performed by bot when a blacklist word is detected
- `/remallbl` / `/rmallbl` / `/rmblocklistall` / `/rmallblocklist` - Removes all the blacklisted words from chat (Owner Only)


## Module Aliases

> These are help-menu module names, not command aliases.

This module can be accessed using the following aliases:

- `blacklist`
- `unblacklist`
- `blocklist`
- `addblocklist`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/addblacklist` | Add a trigger to the blacklist | ❌ |
| `/addblocklist` | Alias of `/addblacklist` | ❌ |
| `/blacklist` | Add a trigger to the blacklist | ❌ |
| `/blacklistaction` | Set action for blacklist triggers | ❌ |
| `/blacklists` | List all blacklisted triggers | ✅ |
| `/blaction` | Set action for blacklist triggers | ❌ |
| `/blocklist` | Alias of `/addblacklist` | ❌ |
| `/blocklists` | Alias of `/blacklists` | ✅ |
| `/remallbl` | Remove all blacklist triggers | ❌ |
| `/rmallbl` | Alias of `/remallbl` | ❌ |
| `/rmallblocklist` | Alias of `/remallbl` | ❌ |
| `/rmblacklist` | Remove a trigger from the blacklist | ❌ |
| `/rmblocklist` | Alias of `/rmblacklist` | ❌ |
| `/rmblocklistall` | Alias of `/remallbl` | ❌ |
| `/unblacklist` | Alias of `/rmblacklist` | ❌ |

## Usage Examples

### Basic Usage

```
/addblacklist spam
/blaction tban 1d
/blaction tmute 2h
/blaction
```

For detailed command usage, refer to the commands table above.

## Required Permissions

- `/addblacklist`, `/blacklist`, `/addblocklist`, `/blocklist` — Requires admin + **Restrict Members** permission (`CanUserRestrict`)
- `/rmblacklist`, `/rmblocklist`, `/unblacklist` — Requires admin + **Restrict Members** permission (`CanUserRestrict`)
- `/blaction`, `/blacklistaction` — Requires admin + **Restrict Members** permission (`CanUserRestrict`)
- `/remallbl`, `/rmallbl`, `/rmblocklistall`, `/rmallblocklist` — Requires **chat owner** (creator)
- `/blacklists`, `/blocklists` — Available to all users (disableable)

**Bot must also be admin** with Restrict Members permission for add/remove/action commands.
