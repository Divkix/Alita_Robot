---
title: Command Reference
description: Complete reference of all Alita Robot commands
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# Command Reference

This page provides a complete reference of all commands available in Alita Robot.

## Overview

- **Total Modules**: 25 (24 user-facing + 1 internal)
- **Total Commands**: 142

## Commands by Module

### Administration

#### 👑 Admin

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/admincache` | Refresh the admin cache | Admin | ❌ | — |
| `/adminlist` | List chat admins | Everyone | ✅ | — |
| `/anonadmin` | Toggle anonymous admin mode | Admin | ❌ | — |
| `/clearadmincache` | Clear the admin cache | Admin | ❌ | — |
| `/demote` | Demote an admin | Admin | ❌ | — |
| `/invitelink` | Get the chat invite link | Admin | ❌ | — |
| `/promote` | Promote a user to admin | Admin | ❌ | — |
| `/title` | Set a custom admin title | Admin | ❌ | — |

### Moderation

#### 🌊 Antiflood

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/delflood` | Toggle flood message deletion | Admin | ❌ | — |
| `/flood` | Show current flood settings | Admin | ✅ | — |
| `/setflood` | Set the flood trigger limit | Admin | ❌ | — |
| `/setfloodmode` | Set the flood action mode | Admin | ❌ | — |

#### 🛡️ Antispam

This module has no user-facing commands. It runs as a passive background watcher.

#### 🔨 Bans

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/ban` | Ban a user | Admin | ❌ | — |
| `/dban` | Ban a user and delete their message | Admin | ❌ | — |
| `/dkick` | Kick a user and delete their message | Admin | ❌ | — |
| `/kick` | Kick a user from the group | Admin | ❌ | — |
| `/kickme` | Kick yourself from the group | Everyone | ❌ | — |
| `/restrict` | Restrict a user's permissions | Admin | ❌ | — |
| `/sban` | Silently ban a user | Admin | ❌ | — |
| `/tban` | Temporarily ban a user | Admin | ❌ | — |
| `/unban` | Unban a user | Admin | ❌ | — |
| `/unrestrict` | Remove restrictions from a user | Admin | ❌ | — |

#### 📦 Blacklists

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/addblacklist` | Add a word to the blacklist | Admin | ❌ | — |
| `/blacklist` | Add a word to the blacklist | Admin | ❌ | — |
| `/blacklistaction` | Set the blacklist trigger action | Admin | ❌ | — |
| `/blacklists` | List all blacklisted words | Admin | ✅ | — |
| `/blaction` | Set the blacklist trigger action | Admin | ❌ | — |
| `/remallbl` | Remove all blacklisted words | Admin | ❌ | `/rmallbl` |
| `/rmallbl` | Alias of `/remallbl` | Admin | ❌ | `/remallbl` |
| `/rmblacklist` | Remove a word from the blacklist | Admin | ❌ | — |

#### 🔐 Captcha

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/captcha` | Toggle captcha verification | Admin | ❌ | — |
| `/captchaaction` | Set the captcha failure action | Admin | ❌ | — |
| `/captchaclear` | Clear pending captcha messages | Admin | ❌ | — |
| `/captchamaxattempts` | Set max captcha attempts | Admin | ❌ | — |
| `/captchamode` | Set captcha challenge mode | Admin | ❌ | — |
| `/captchapending` | View pending captcha users | Admin | ❌ | — |
| `/captchatime` | Set captcha timeout duration | Admin | ❌ | — |

#### 🔒 Locks

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/lock` | Lock a permission type | Admin | ❌ | — |
| `/locks` | Show current lock settings | Admin | ✅ | — |
| `/locktypes` | List available lock types | Admin | ✅ | — |
| `/unlock` | Unlock a permission type | Admin | ❌ | — |

#### 🔇 Mutes

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/dmute` | Mute a user and delete their message | Admin | ❌ | — |
| `/mute` | Mute a user | Admin | ❌ | — |
| `/smute` | Silently mute a user | Admin | ❌ | — |
| `/tmute` | Temporarily mute a user | Admin | ❌ | — |
| `/unmute` | Unmute a user | Admin | ❌ | — |

#### 🧹 Purges

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/del` | Delete a replied-to message | Admin | ❌ | — |
| `/purge` | Purge messages from replied-to onwards | Admin | ❌ | — |
| `/purgefrom` | Set purge start point | Admin | ❌ | — |
| `/purgeto` | Purge to a specific message | Admin | ❌ | — |

#### ⚠️ Warns

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/dwarn` | Warn a user and delete their message | Admin | ❌ | — |
| `/resetallwarns` | Reset all warnings for all users | Admin | ❌ | — |
| `/resetwarn` | Reset warnings for a user | Admin | ❌ | — |
| `/resetwarns` | Reset warnings for a user | Admin | ❌ | — |
| `/rmwarn` | Remove a warning from a user | Admin | ❌ | — |
| `/setwarnlimit` | Set the warn limit before action | Admin | ❌ | — |
| `/setwarnmode` | Set the warn action mode | Admin | ❌ | — |
| `/swarn` | Silently warn a user | Admin | ❌ | — |
| `/unwarn` | Remove a warning from a user | Admin | ❌ | — |
| `/warn` | Warn a user | Admin | ❌ | — |
| `/warnings` | Show warnings for a user | Everyone | ❌ | — |
| `/warns` | Show current warn settings | Everyone | ✅ | — |

### Content Management

#### 🔍 Filters

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/addfilter` | Add a keyword filter | Admin | ❌ | — |
| `/filter` | Add a keyword filter | Admin | ❌ | — |
| `/filters` | List all active filters | Everyone | ✅ | — |
| `/removefilter` | Remove a keyword filter | Admin | ❌ | — |
| `/rmfilter` | Remove a keyword filter | Admin | ❌ | — |
| `/stop` | Remove a keyword filter | Admin | ❌ | — |
| `/stopall` | Remove all filters | Admin | ❌ | — |

#### 📄 Formatting

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/markdownhelp` | Show markdown formatting guide | Everyone | ❌ | `/formatting` |
| `/formatting` | Alias of `/markdownhelp` | Everyone | ❌ | `/markdownhelp` |

#### 👋 Greetings

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/autoapprove` | Toggle auto-approve join requests | Admin | ❌ | — |
| `/cleangoodbye` | Toggle goodbye message cleanup | Admin | ❌ | — |
| `/cleanservice` | Toggle service message deletion | Admin | ❌ | — |
| `/cleanwelcome` | Toggle welcome message cleanup | Admin | ❌ | — |
| `/goodbye` | Show current goodbye settings | Admin | ❌ | — |
| `/resetgoodbye` | Reset goodbye to default | Admin | ❌ | — |
| `/resetwelcome` | Reset welcome to default | Admin | ❌ | — |
| `/setgoodbye` | Set the goodbye message | Admin | ❌ | — |
| `/setwelcome` | Set the welcome message | Admin | ❌ | — |
| `/welcome` | Show current welcome settings | Admin | ❌ | — |

#### 📝 Notes

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/addnote` | Save a note | Admin | ❌ | — |
| `/clear` | Remove a note | Admin | ❌ | — |
| `/clearall` | Remove all notes | Admin | ❌ | — |
| `/get` | Retrieve a saved note | Everyone | ✅ | — |
| `/notes` | List all saved notes | Everyone | ✅ | — |
| `/privnote` | Toggle private note delivery | Admin | ❌ | `/privatenotes` |
| `/privatenotes` | Alias of `/privnote` | Admin | ❌ | `/privnote` |
| `/rmnote` | Remove a note | Admin | ❌ | — |
| `/save` | Save a note | Admin | ❌ | — |
| `/saved` | List all saved notes | Everyone | ❌ | — |

#### 📋 Rules

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/clearrulesbtn` | Reset the rules button text | Admin | ❌ | — |
| `/clearrulesbutton` | Reset the rules button text | Admin | ❌ | — |
| `/privaterules` | Toggle private rules delivery | Admin | ❌ | — |
| `/resetrules` | Reset all group rules | Admin | ❌ | `/clearrules` |
| `/clearrules` | Alias of `/resetrules` | Admin | ❌ | `/resetrules` |
| `/resetrulesbtn` | Reset the rules button text | Admin | ❌ | — |
| `/resetrulesbutton` | Reset the rules button text | Admin | ❌ | — |
| `/rules` | Show the group rules | Everyone | ✅ | — |
| `/rulesbtn` | Set the rules button text | Admin | ❌ | — |
| `/rulesbutton` | Set the rules button text | Admin | ❌ | — |
| `/setrules` | Set the group rules | Admin | ❌ | — |

### User Tools

#### 🔧 Misc

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/id` | Get user or chat ID | Everyone | ✅ | — |
| `/info` | Get user information | Everyone | ✅ | — |
| `/ping` | Check bot response latency | Everyone | ✅ | — |
| `/removebotkeyboard` | Remove a stuck bot keyboard | Everyone | ❌ | — |
| `/stat` | Show message count for the chat | Everyone | ✅ | — |
| `/tell` | Echo a message via the bot | Everyone | ❌ | — |
| `/tr` | Translate text to another language | Everyone | ✅ | — |

#### 📢 Reports

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/report` | Report a user to admins | Everyone | ✅ | — |
| `/reports` | Toggle reporting for the group | Admin | ❌ | — |

### Bot Management

#### 🔗 Connections

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/allowconnect` | Toggle connection permissions | Admin | ❌ | — |
| `/connect` | Connect to a group from PM | Everyone | ❌ | — |
| `/connection` | Show current connection status | Everyone | ❌ | — |
| `/disconnect` | Disconnect from current group | Everyone | ❌ | — |
| `/reconnect` | Reconnect to last connected group | Everyone | ❌ | — |

#### ❌ Disabling

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/disable` | Disable a command in this chat | Admin | ❌ | — |
| `/disableable` | List commands that can be disabled | Admin | ❌ | — |
| `/disabled` | List currently disabled commands | Admin | ✅ | — |
| `/disabledel` | Toggle deletion of disabled commands | Admin | ❌ | — |
| `/enable` | Re-enable a disabled command | Admin | ❌ | — |

#### ❓ Help

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/about` | Show bot information and links | Everyone | ❌ | — |
| `/donate` | Show donation information | Everyone | ❌ | — |
| `/help` | Show help menu with module list | Everyone | ❌ | — |
| `/start` | Show welcome message with navigation menu | Everyone | ❌ | — |

#### 🌐 Languages

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/lang` | Change the bot language | User/Admin | ❌ | — |

#### 📌 Pins

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/antichannelpin` | Toggle anti-channel pin mode | Admin | ❌ | — |
| `/cleanlinked` | Toggle linked channel message cleanup | Admin | ❌ | — |
| `/permapin` | Pin a message permanently | Admin | ❌ | — |
| `/pin` | Pin a replied-to message | Admin | ❌ | — |
| `/pinned` | Get the current pinned message | Admin | ❌ | — |
| `/unpin` | Unpin the current pinned message | Admin | ❌ | — |
| `/unpinall` | Unpin all pinned messages | Admin | ❌ | — |

### Developer

#### 🔧 Devs

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/addsudo` | Grant sudo permissions to a user | Owner | ❌ | — |
| `/adddev` | Grant developer permissions to a user | Owner | ❌ | — |
| `/chatinfo` | Display detailed information about a chat | Dev/Owner | ❌ | — |
| `/chatlist` | Generate and send a list of all active chats | Dev/Owner | ❌ | — |
| `/leavechat` | Force the bot to leave a specified chat | Dev/Owner | ❌ | — |
| `/remdev` | Revoke developer permissions from a user | Owner | ❌ | — |
| `/remsudo` | Revoke sudo permissions from a user | Owner | ❌ | — |
| `/stats` | Display bot statistics and system info | Dev/Owner | ❌ | — |
| `/teamusers` | List all team members | Team | ❌ | — |

### Internal

#### 👤 Users

No commands — passive background tracker. See [module page](/commands/users/) for details.

## Alphabetical Index

| Command | Module | Description | Permission |
|---------|--------|-------------|------------|
| `/about` | Help | Show bot information and links | Everyone |
| `/addblacklist` | Blacklists | Add a word to the blacklist | Admin |
| `/adddev` | Devs | Grant developer permissions to a user | Owner |
| `/addfilter` | Filters | Add a keyword filter | Admin |
| `/addnote` | Notes | Save a note | Admin |
| `/admincache` | Admin | Refresh the admin cache | Admin |
| `/adminlist` | Admin | List chat admins | Everyone |
| `/addsudo` | Devs | Grant sudo permissions to a user | Owner |
| `/allowconnect` | Connections | Toggle connection permissions | Admin |
| `/anonadmin` | Admin | Toggle anonymous admin mode | Admin |
| `/antichannelpin` | Pins | Toggle anti-channel pin mode | Admin |
| `/autoapprove` | Greetings | Toggle auto-approve join requests | Admin |
| `/ban` | Bans | Ban a user | Admin |
| `/blacklist` | Blacklists | Add a word to the blacklist | Admin |
| `/blacklistaction` | Blacklists | Set the blacklist trigger action | Admin |
| `/blacklists` | Blacklists | List all blacklisted words | Admin |
| `/blaction` | Blacklists | Set the blacklist trigger action | Admin |
| `/captcha` | Captcha | Toggle captcha verification | Admin |
| `/captchaaction` | Captcha | Set the captcha failure action | Admin |
| `/captchaclear` | Captcha | Clear pending captcha messages | Admin |
| `/captchamaxattempts` | Captcha | Set max captcha attempts | Admin |
| `/captchamode` | Captcha | Set captcha challenge mode | Admin |
| `/captchapending` | Captcha | View pending captcha users | Admin |
| `/captchatime` | Captcha | Set captcha timeout duration | Admin |
| `/chatinfo` | Devs | Display detailed information about a chat | Dev/Owner |
| `/chatlist` | Devs | Generate and send a list of all active chats | Dev/Owner |
| `/cleangoodbye` | Greetings | Toggle goodbye message cleanup | Admin |
| `/cleanlinked` | Pins | Toggle linked channel message cleanup | Admin |
| `/cleanservice` | Greetings | Toggle service message deletion | Admin |
| `/cleanwelcome` | Greetings | Toggle welcome message cleanup | Admin |
| `/clear` | Notes | Remove a note | Admin |
| `/clearadmincache` | Admin | Clear the admin cache | Admin |
| `/clearall` | Notes | Remove all notes | Admin |
| `/clearrules` | Rules | Alias of `/resetrules` | Admin |
| `/clearrulesbtn` | Rules | Reset the rules button text | Admin |
| `/clearrulesbutton` | Rules | Reset the rules button text | Admin |
| `/connect` | Connections | Connect to a group from PM | Everyone |
| `/connection` | Connections | Show current connection status | Everyone |
| `/dban` | Bans | Ban a user and delete their message | Admin |
| `/del` | Purges | Delete a replied-to message | Admin |
| `/delflood` | Antiflood | Toggle flood message deletion | Admin |
| `/demote` | Admin | Demote an admin | Admin |
| `/disable` | Disabling | Disable a command in this chat | Admin |
| `/disableable` | Disabling | List commands that can be disabled | Admin |
| `/disabled` | Disabling | List currently disabled commands | Admin |
| `/disabledel` | Disabling | Toggle deletion of disabled commands | Admin |
| `/disconnect` | Connections | Disconnect from current group | Everyone |
| `/dkick` | Bans | Kick a user and delete their message | Admin |
| `/dmute` | Mutes | Mute a user and delete their message | Admin |
| `/donate` | Help | Show donation information | Everyone |
| `/dwarn` | Warns | Warn a user and delete their message | Admin |
| `/enable` | Disabling | Re-enable a disabled command | Admin |
| `/filter` | Filters | Add a keyword filter | Admin |
| `/filters` | Filters | List all active filters | Everyone |
| `/flood` | Antiflood | Show current flood settings | Admin |
| `/formatting` | Formatting | Alias of `/markdownhelp` | Everyone |
| `/get` | Notes | Retrieve a saved note | Everyone |
| `/goodbye` | Greetings | Show current goodbye settings | Admin |
| `/help` | Help | Show help menu with module list | Everyone |
| `/id` | Misc | Get user or chat ID | Everyone |
| `/info` | Misc | Get user information | Everyone |
| `/invitelink` | Admin | Get the chat invite link | Admin |
| `/kick` | Bans | Kick a user from the group | Admin |
| `/kickme` | Bans | Kick yourself from the group | Everyone |
| `/lang` | Languages | Change the bot language | User/Admin |
| `/leavechat` | Devs | Force the bot to leave a specified chat | Dev/Owner |
| `/lock` | Locks | Lock a permission type | Admin |
| `/locks` | Locks | Show current lock settings | Admin |
| `/locktypes` | Locks | List available lock types | Admin |
| `/markdownhelp` | Formatting | Show markdown formatting guide | Everyone |
| `/mute` | Mutes | Mute a user | Admin |
| `/notes` | Notes | List all saved notes | Everyone |
| `/permapin` | Pins | Pin a message permanently | Admin |
| `/pin` | Pins | Pin a replied-to message | Admin |
| `/ping` | Misc | Check bot response latency | Everyone |
| `/pinned` | Pins | Get the current pinned message | Admin |
| `/privatenotes` | Notes | Alias of `/privnote` | Admin |
| `/privaterules` | Rules | Toggle private rules delivery | Admin |
| `/privnote` | Notes | Toggle private note delivery | Admin |
| `/promote` | Admin | Promote a user to admin | Admin |
| `/purge` | Purges | Purge messages from replied-to onwards | Admin |
| `/purgefrom` | Purges | Set purge start point | Admin |
| `/purgeto` | Purges | Purge to a specific message | Admin |
| `/reconnect` | Connections | Reconnect to last connected group | Everyone |
| `/remallbl` | Blacklists | Remove all blacklisted words | Admin |
| `/remdev` | Devs | Revoke developer permissions from a user | Owner |
| `/removebotkeyboard` | Misc | Remove a stuck bot keyboard | Everyone |
| `/removefilter` | Filters | Remove a keyword filter | Admin |
| `/remsudo` | Devs | Revoke sudo permissions from a user | Owner |
| `/report` | Reports | Report a user to admins | Everyone |
| `/reports` | Reports | Toggle reporting for the group | Admin |
| `/resetallwarns` | Warns | Reset all warnings for all users | Admin |
| `/resetgoodbye` | Greetings | Reset goodbye to default | Admin |
| `/resetrules` | Rules | Reset all group rules | Admin |
| `/resetrulesbtn` | Rules | Reset the rules button text | Admin |
| `/resetrulesbutton` | Rules | Reset the rules button text | Admin |
| `/resetwarn` | Warns | Reset warnings for a user | Admin |
| `/resetwarns` | Warns | Reset warnings for a user | Admin |
| `/resetwelcome` | Greetings | Reset welcome to default | Admin |
| `/restrict` | Bans | Restrict a user's permissions | Admin |
| `/rmallbl` | Blacklists | Alias of `/remallbl` | Admin |
| `/rmblacklist` | Blacklists | Remove a word from the blacklist | Admin |
| `/rmfilter` | Filters | Remove a keyword filter | Admin |
| `/rmnote` | Notes | Remove a note | Admin |
| `/rmwarn` | Warns | Remove a warning from a user | Admin |
| `/rules` | Rules | Show the group rules | Everyone |
| `/rulesbtn` | Rules | Set the rules button text | Admin |
| `/rulesbutton` | Rules | Set the rules button text | Admin |
| `/save` | Notes | Save a note | Admin |
| `/saved` | Notes | List all saved notes | Everyone |
| `/sban` | Bans | Silently ban a user | Admin |
| `/setflood` | Antiflood | Set the flood trigger limit | Admin |
| `/setfloodmode` | Antiflood | Set the flood action mode | Admin |
| `/setgoodbye` | Greetings | Set the goodbye message | Admin |
| `/setrules` | Rules | Set the group rules | Admin |
| `/setwarnlimit` | Warns | Set the warn limit before action | Admin |
| `/setwarnmode` | Warns | Set the warn action mode | Admin |
| `/setwelcome` | Greetings | Set the welcome message | Admin |
| `/smute` | Mutes | Silently mute a user | Admin |
| `/start` | Help | Show welcome message with navigation menu | Everyone |
| `/stat` | Misc | Show message count for the chat | Everyone |
| `/stats` | Devs | Display bot statistics and system info | Dev/Owner |
| `/stop` | Filters | Remove a keyword filter | Admin |
| `/stopall` | Filters | Remove all filters | Admin |
| `/swarn` | Warns | Silently warn a user | Admin |
| `/tban` | Bans | Temporarily ban a user | Admin |
| `/teamusers` | Devs | List all team members | Team |
| `/tell` | Misc | Echo a message via the bot | Everyone |
| `/title` | Admin | Set a custom admin title | Admin |
| `/tmute` | Mutes | Temporarily mute a user | Admin |
| `/tr` | Misc | Translate text to another language | Everyone |
| `/unban` | Bans | Unban a user | Admin |
| `/unlock` | Locks | Unlock a permission type | Admin |
| `/unmute` | Mutes | Unmute a user | Admin |
| `/unpin` | Pins | Unpin the current pinned message | Admin |
| `/unpinall` | Pins | Unpin all pinned messages | Admin |
| `/unrestrict` | Bans | Remove restrictions from a user | Admin |
| `/unwarn` | Warns | Remove a warning from a user | Admin |
| `/warn` | Warns | Warn a user | Admin |
| `/warnings` | Warns | Show warnings for a user | Everyone |
| `/warns` | Warns | Show current warn settings | Everyone |
| `/welcome` | Greetings | Show current welcome settings | Admin |
