---
title: Command Reference
description: Complete reference of all Alita Robot commands
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# Command Reference

This page provides a complete reference of all commands available in Alita Robot.

## Overview

- **Total Modules**: 31 (29 user-facing + 2 internal)
- **Total Commands**: 219

## Commands by Module

### Administration

#### 👑 Admin

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/admincache` | Refresh the admin cache | Admin | ❌ | — |
| `/adminlist` | List chat admins | Everyone | ✅ | — |
| `/anonadmin` | Toggle anonymous admin mode (anyone can view the current mode) | Owner | ❌ | — |
| `/clearadmincache` | Clear the admin cache | Admin | ❌ | — |
| `/demote` | Demote an admin | Admin | ❌ | — |
| `/invitelink` | Get the chat invite link | Everyone (public groups) / users with invite rights (private groups) | ❌ | — |
| `/promote` | Promote a user to admin | Admin | ❌ | — |
| `/setdescription` | Alias of `/setgdesc` | Admin | ❌ | `/setgdesc` |
| `/setgdesc` | Change or clear the group description | Admin | ❌ | `/setgroupdesc`, `/setdescription` |
| `/setgpic` | Set the group photo (reply to a photo) | Admin | ❌ | `/setgrouppic` |
| `/setgroupdesc` | Alias of `/setgdesc` | Admin | ❌ | `/setgdesc` |
| `/setgrouppic` | Alias of `/setgpic` | Admin | ❌ | `/setgpic` |
| `/setgrouptitle` | Alias of `/setgtitle` | Admin | ❌ | `/setgtitle` |
| `/setgtitle` | Change the group title | Admin | ❌ | `/setgrouptitle` |
| `/title` | Set a custom admin title | Admin | ❌ | — |

### Moderation

#### 🌊 Antiflood

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/delflood` | Toggle flood message deletion | Admin | ❌ | `/clearflood` |
| `/clearflood` | Alias of `/delflood` | Admin | ❌ | `/delflood` |
| `/flood` | Show current flood settings | Everyone | ✅ | — |
| `/setflood` | Set the flood trigger limit | Admin | ❌ | — |
| `/setfloodmode` | Set the flood action mode | Admin | ❌ | — |

#### 🚨 AntiRaid

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/antiraid` | Toggle or configure anti-raid mode | Admin | ✅ | — |
| `/raidtime` | Set the raid duration | Admin | ❌ | — |
| `/raidactiontime` | Set the ban duration for raiders | Admin | ❌ | — |
| `/autoantiraid` | Set auto-raid trigger threshold | Admin | ❌ | — |

#### 🤖 Aispam

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/aispam` | Show filter status, or turn it on/off (`on`/`off`) | Admin | ✅ | — |

#### 👤 Approvals

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/approve` | Approve a user in the group | Admin | ❌ | — |
| `/unapprove` | Remove a user from approved list | Admin | ❌ | — |
| `/approval` | Check a user's approval status | Admin | ❌ | — |
| `/approved` | List all approved users | Admin | ❌ | — |
| `/unapproveall` | Remove all approved users | Owner | ❌ | — |

#### 🔨 Bans

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/ban` | Ban a user | Admin | ❌ | — |
| `/banme` | Ban yourself from the group | Everyone | ✅ | — |
| `/dban` | Ban a user and delete their message | Admin | ❌ | — |
| `/dkick` | Kick a user and delete their message | Admin | ❌ | — |
| `/kick` | Kick a user from the group | Admin | ❌ | — |
| `/kickme` | Kick yourself from the group | Everyone | ✅ | — |
| `/restrict` | Restrict a user's permissions | Admin | ❌ | — |
| `/sban` | Silently ban a user | Admin | ❌ | — |
| `/skick` | Silently kick a user and delete the command | Admin | ❌ | — |
| `/tban` | Temporarily ban a user | Admin | ❌ | — |
| `/unban` | Unban a user | Admin | ❌ | — |
| `/unbanall` | Unban all known users in the chat | Owner | ❌ | — |
| `/unrestrict` | Remove restrictions from a user | Admin | ❌ | — |

#### 📦 Blacklists

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/addblacklist` | Add a word to the blacklist | Admin | ❌ | `/blacklist`, `/addblocklist`, `/blocklist` |
| `/addblocklist` | Alias of `/addblacklist` | Admin | ❌ | `/addblacklist` |
| `/blacklist` | Add a word to the blacklist | Admin | ❌ | `/addblacklist`, `/addblocklist`, `/blocklist` |
| `/blacklistaction` | Set the blacklist trigger action | Admin | ❌ | — |
| `/blacklists` | List all blacklisted words | Everyone | ✅ | `/blocklists` |
| `/blaction` | Set the blacklist trigger action | Admin | ❌ | — |
| `/blocklist` | Alias of `/addblacklist` | Admin | ❌ | `/addblacklist` |
| `/blocklists` | Alias of `/blacklists` | Everyone | ✅ | `/blacklists` |
| `/remallbl` | Remove all blacklisted words | Owner | ❌ | `/rmallbl`, `/rmblocklistall`, `/rmallblocklist` |
| `/rmallbl` | Alias of `/remallbl` | Owner | ❌ | `/remallbl` |
| `/rmallblocklist` | Alias of `/remallbl` | Owner | ❌ | `/remallbl` |
| `/rmblacklist` | Remove a word from the blacklist | Admin | ❌ | `/rmblocklist`, `/unblacklist` |
| `/rmblocklist` | Alias of `/rmblacklist` | Admin | ❌ | `/rmblacklist` |
| `/rmblocklistall` | Alias of `/remallbl` | Owner | ❌ | `/remallbl` |
| `/unblacklist` | Alias of `/rmblacklist` | Admin | ❌ | `/rmblacklist` |

#### 🔐 Captcha

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/captcha` | Toggle captcha verification | Admin | ❌ | — |
| `/captchaaction` | Set the captcha failure action | Admin | ❌ | — |
| `/captchaclear` | Clear pending captcha messages | Admin | ❌ | — |
| `/captchamaxattempts` | Set max captcha attempts | Admin | ❌ | — |
| `/captchamode` | Set captcha challenge mode | Admin | ❌ | — |
| `/captchapending` | View a pending user's stored messages | Admin | ❌ | — |
| `/captchatime` | Set captcha timeout duration | Admin | ❌ | — |

#### 🛡️ Federations

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/newfed` | Create a federation (one per user; PM only) | Everyone | ❌ | — |
| `/renamefed` | Rename your federation | Fed owner | ❌ | — |
| `/delfed` | Delete your federation (PM only; asks to confirm) | Fed owner | ❌ | — |
| `/fedpromote` | Promote a user to federation admin | Fed owner | ❌ | — |
| `/feddemote` | Demote a federation admin | Fed owner | ❌ | — |
| `/fedreason` | Require a reason for fbans (on/off) | Fed owner | ❌ | — |
| `/fednotif` | PM the owner on fbans (on/off) | Fed owner | ❌ | — |
| `/subfed` | Subscribe to another federation's bans (max 5) | Fed owner | ❌ | — |
| `/unsubfed` | Unsubscribe from another federation | Fed owner | ❌ | — |
| `/fedsubs` | List the federations yours subscribes to | Fed owner | ✅ | — |
| `/fbanlist` | Export fbans as CSV or JSON (once every 30 minutes) | Fed owner | ❌ | — |
| `/importfbans` | Import fbans from a replied CSV/JSON file | Fed owner | ❌ | — |
| `/setfedlog` | Make this group the federation log chat (groups only) | Fed owner | ❌ | — |
| `/unsetfedlog` | Clear the federation log chat | Fed owner | ❌ | — |
| `/joinfed` | Join this group to a federation | Group creator | ❌ | — |
| `/leavefed` | Leave the federation | Group creator | ❌ | — |
| `/quietfed` | Suppress passive fban notices in this group (on/off) | Admin | ❌ | — |
| `/fban` | Ban a user across the federation | Fed admin | ❌ | — |
| `/unfban` | Lift a federation ban | Fed admin | ❌ | `/funban` |
| `/funban` | Alias of `/unfban` | Fed admin | ❌ | `/unfban` |
| `/feddemoteme` | Step down as a federation admin | Fed admin | ❌ | — |
| `/fedstat` | Check a user's federation bans | Everyone | ✅ | `/fbanstat` |
| `/fbanstat` | Alias of `/fedstat` | Everyone | ❌ | `/fedstat` |
| `/fedinfo` | Show federation info | Everyone | ✅ | — |
| `/fedadmins` | List a federation's admins | Everyone | ✅ | — |
| `/chatfed` | Show this group's federation | Everyone | ❌ | — |
| `/myfeds` | List the federations you own or admin | Everyone | ❌ | — |

#### 🔒 Locks

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/lock` | Lock a permission type | Admin | ❌ | — |
| `/locks` | Show current lock settings | Admin | ✅ | — |
| `/locktypes` | List available lock types | Everyone | ✅ | — |
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
| `/purge` | Purge messages from replied-to onwards (optional reason) | Admin | ❌ | — |
| `/purgefrom` | Set purge start point | Admin | ❌ | — |
| `/purgeto` | Purge to a specific message (optional reason) | Admin | ❌ | — |

#### ⚠️ Warns

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/dwarn` | Warn a user and delete their message | Admin | ❌ | — |
| `/resetallwarns` | Reset all warnings for all users | Owner | ❌ | — |
| `/resetwarn` | Alias of `/resetwarns` | Admin | ❌ | `/resetwarns` |
| `/resetwarns` | Reset warnings for a user | Admin | ❌ | `/resetwarn` |
| `/rmwarn` | Remove a warning from a user | Admin | ❌ | `/unwarn` |
| `/setwarnlimit` | Set the warn limit before action | Admin | ❌ | — |
| `/setwarnmode` | Set the warn action mode | Admin | ❌ | — |
| `/setwarntime` | Set how long a warn counts | Admin | ❌ | — |
| `/swarn` | Warn a user and delete your command | Admin | ❌ | — |
| `/unwarn` | Alias of `/rmwarn` | Admin | ❌ | `/rmwarn` |
| `/warn` | Warn a user | Admin | ❌ | — |
| `/warnings` | Get the chat's warning settings | Admin | ❌ | — |
| `/warns` | Show warning count for a user | Everyone | ✅ | — |

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
| `/stopall` | Remove all filters | Owner | ❌ | — |

#### 📄 Formatting

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/markdownhelp` | Show markdown formatting guide | Everyone | ❌ | `/formatting` |
| `/formatting` | Alias of `/markdownhelp` | Everyone | ❌ | `/markdownhelp` |

#### 🎭 Reactions

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/addreaction` | Add an auto-reaction for a keyword | Admin | ❌ | — |
| `/removereaction` | Remove a reaction for a keyword | Admin | ❌ | — |
| `/reactions` | List configured reactions | Everyone | ❌ | — |
| `/resetreactions` | Clear all reactions | Admin | ❌ | — |

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
| `/addnote` | Save a note | Admin | ❌ | `/save` |
| `/clear` | Remove a note | Admin | ❌ | `/rmnote` |
| `/clearall` | Remove all notes | Owner | ❌ | — |
| `/get` | Retrieve a saved note | Everyone | ✅ | — |
| `/notes` | List all saved notes | Everyone | ✅ | `/saved` |
| `/privnote` | Toggle private note delivery | Admin | ❌ | `/privatenotes` |
| `/privatenotes` | Alias of `/privnote` | Admin | ❌ | `/privnote` |
| `/rmnote` | Remove a note | Admin | ❌ | `/clear` |
| `/save` | Save a note | Admin | ❌ | `/addnote` |
| `/saved` | List all saved notes | Everyone | ❌ | `/notes` |

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
| `/github` | Look up a GitHub user or repository | Everyone | ✅ | — |
| `/id` | Get user or chat ID | Everyone | ✅ | — |
| `/info` | Get user information | Everyone | ✅ | — |
| `/ping` | Check bot response latency | Everyone | ✅ | — |
| `/removebotkeyboard` | Remove a stuck bot keyboard | Everyone | ❌ | — |
| `/runs` | Reply with a random quip | Everyone | ✅ | — |
| `/stat` | Show message count for the chat | Everyone | ✅ | — |
| `/stickerid` | Get a replied sticker's file ID | Everyone | ✅ | — |
| `/tell` | Echo a message via the bot | Admin | ❌ | — |
| `/tr` | Translate text to another language | Everyone | ✅ | — |
| `/ud` | Look up a term on Urban Dictionary | Everyone | ✅ | — |
| `/wiki` | Search English Wikipedia | Everyone | ✅ | — |
| `/zombies` | Kick deleted accounts from the group | Admin | ❌ | — |

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
| `/settings` | Pick a group in PM and show all its settings | Admin | ❌ | — |

#### ❌ Disabling

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/disable` | Disable a command in this chat | Admin | ❌ | — |
| `/disableable` | List commands that can be disabled | Everyone | ❌ | — |
| `/disabled` | List currently disabled commands | Everyone | ✅ | — |
| `/disabledel` | Toggle deletion of disabled commands | Admin | ❌ | — |
| `/enable` | Re-enable a disabled command | Admin | ❌ | — |

#### ❓ Help

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/about` | Show bot information and links | Everyone | ❌ | — |
| `/donate` | Show donation information | Everyone | ❌ | — |
| `/extend` | Show every command in one list (groups get a PM button) | Everyone | ❌ | — |
| `/help` | Show help menu with module list | Everyone | ❌ | — |
| `/start` | Show welcome message with navigation menu | Everyone | ❌ | — |

#### 🌐 Languages

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/lang` | Change the bot language | User/Admin | ❌ | — |

#### 📦 Backup & Restore

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/export` | Export all group settings to a JSON file | Admin | ✅ | — |
| `/import` | Restore settings from a backup file | Owner | ✅ | — |
| `/reset` | Reset all settings to default | Owner | ❌ | — |

#### 📌 Pins

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/antichannelpin` | Toggle anti-channel pin mode | Admin | ❌ | — |
| `/cleanlinked` | Toggle linked channel message cleanup | Admin | ❌ | — |
| `/permapin` | Pin a custom message sent by the bot | Admin | ❌ | — |
| `/pin` | Pin a replied-to message | Admin | ❌ | — |
| `/pinned` | Get the current pinned message | Everyone | ❌ | — |
| `/unpin` | Unpin the current pinned message | Admin | ❌ | — |
| `/unpinall` | Unpin all pinned messages | Admin | ❌ | — |

#### 📜 LogChannels

| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/setlog` | Set the log channel (send it in the channel, then forward that message to the group) | Channel admin + Admin | ❌ | — |
| `/unsetlog` | Remove the log channel | Admin | ❌ | — |
| `/logchannel` | Show the current log channel (groups only) | Everyone | ❌ | — |
| `/logcategories` | List log categories | Everyone | ❌ | — |
| `/log` | Enable one or more log categories | Admin | ❌ | — |
| `/nolog` | Disable one or more log categories | Admin | ❌ | — |

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
| `/addblocklist` | Blacklists | Alias of `/addblacklist` | Admin |
| `/addreaction` | Reactions | Add an auto-reaction for a keyword | Admin |
| `/adddev` | Devs | Grant developer permissions to a user | Owner |
| `/addfilter` | Filters | Add a keyword filter | Admin |
| `/addnote` | Notes | Save a note | Admin |
| `/admincache` | Admin | Refresh the admin cache | Admin |
| `/adminlist` | Admin | List chat admins | Everyone |
| `/addsudo` | Devs | Grant sudo permissions to a user | Owner |
| `/aispam` | Aispam | Show filter status, or turn it on/off | Admin |
| `/allowconnect` | Connections | Toggle connection permissions | Admin |
| `/anonadmin` | Admin | Toggle anonymous admin mode (anyone can view the current mode) | Owner |
| `/antiraid` | AntiRaid | Toggle or configure anti-raid mode | Admin |
| `/antichannelpin` | Pins | Toggle anti-channel pin mode | Admin |
| `/approval` | Approvals | Check a user's approval status | Admin |
| `/approve` | Approvals | Approve a user in the group | Admin |
| `/approved` | Approvals | List all approved users | Admin |
| `/autoantiraid` | AntiRaid | Set auto-raid trigger threshold | Admin |
| `/autoapprove` | Greetings | Toggle auto-approve join requests | Admin |
| `/ban` | Bans | Ban a user | Admin |
| `/banme` | Bans | Ban yourself from the group | Everyone |
| `/blacklist` | Blacklists | Add a word to the blacklist | Admin |
| `/blacklistaction` | Blacklists | Set the blacklist trigger action | Admin |
| `/blacklists` | Blacklists | List all blacklisted words | Everyone |
| `/blaction` | Blacklists | Set the blacklist trigger action | Admin |
| `/blocklist` | Blacklists | Alias of `/addblacklist` | Admin |
| `/blocklists` | Blacklists | Alias of `/blacklists` | Everyone |
| `/captcha` | Captcha | Toggle captcha verification | Admin |
| `/captchaaction` | Captcha | Set the captcha failure action | Admin |
| `/captchaclear` | Captcha | Clear pending captcha messages | Admin |
| `/captchamaxattempts` | Captcha | Set max captcha attempts | Admin |
| `/captchamode` | Captcha | Set captcha challenge mode | Admin |
| `/captchapending` | Captcha | View a pending user's stored messages | Admin |
| `/captchatime` | Captcha | Set captcha timeout duration | Admin |
| `/chatfed` | Federations | Show this group's federation | Everyone |
| `/chatinfo` | Devs | Display detailed information about a chat | Dev/Owner |
| `/chatlist` | Devs | Generate and send a list of all active chats | Dev/Owner |
| `/cleangoodbye` | Greetings | Toggle goodbye message cleanup | Admin |
| `/cleanlinked` | Pins | Toggle linked channel message cleanup | Admin |
| `/cleanservice` | Greetings | Toggle service message deletion | Admin |
| `/cleanwelcome` | Greetings | Toggle welcome message cleanup | Admin |
| `/clear` | Notes | Remove a note | Admin |
| `/clearadmincache` | Admin | Clear the admin cache | Admin |
| `/clearall` | Notes | Remove all notes | Owner |
| `/clearflood` | Antiflood | Alias of `/delflood` | Admin |
| `/clearrules` | Rules | Alias of `/resetrules` | Admin |
| `/clearrulesbtn` | Rules | Reset the rules button text | Admin |
| `/clearrulesbutton` | Rules | Reset the rules button text | Admin |
| `/connect` | Connections | Connect to a group from PM | Everyone |
| `/connection` | Connections | Show current connection status | Everyone |
| `/dban` | Bans | Ban a user and delete their message | Admin |
| `/del` | Purges | Delete a replied-to message | Admin |
| `/delfed` | Federations | Delete your federation (PM only; asks to confirm) | Fed owner |
| `/delflood` | Antiflood | Toggle flood message deletion | Admin |
| `/demote` | Admin | Demote an admin | Admin |
| `/disable` | Disabling | Disable a command in this chat | Admin |
| `/disableable` | Disabling | List commands that can be disabled | Everyone |
| `/disabled` | Disabling | List currently disabled commands | Everyone |
| `/disabledel` | Disabling | Toggle deletion of disabled commands | Admin |
| `/disconnect` | Connections | Disconnect from current group | Everyone |
| `/dkick` | Bans | Kick a user and delete their message | Admin |
| `/dmute` | Mutes | Mute a user and delete their message | Admin |
| `/donate` | Help | Show donation information | Everyone |
| `/dwarn` | Warns | Warn a user and delete their message | Admin |
| `/enable` | Disabling | Re-enable a disabled command | Admin |
| `/export` | Backup | Export all group settings to a JSON file | Admin |
| `/extend` | Help | Show every command in one list | Everyone |
| `/fban` | Federations | Ban a user across the federation | Fed admin |
| `/fbanlist` | Federations | Export fbans as CSV or JSON (once every 30 minutes) | Fed owner |
| `/fbanstat` | Federations | Alias of `/fedstat` | Everyone |
| `/fedadmins` | Federations | List a federation's admins | Everyone |
| `/feddemote` | Federations | Demote a federation admin | Fed owner |
| `/feddemoteme` | Federations | Step down as a federation admin | Fed admin |
| `/fedinfo` | Federations | Show federation info | Everyone |
| `/fednotif` | Federations | PM the owner on fbans (on/off) | Fed owner |
| `/fedpromote` | Federations | Promote a user to federation admin | Fed owner |
| `/fedreason` | Federations | Require a reason for fbans (on/off) | Fed owner |
| `/fedstat` | Federations | Check a user's federation bans | Everyone |
| `/fedsubs` | Federations | List the federations yours subscribes to | Fed owner |
| `/filter` | Filters | Add a keyword filter | Admin |
| `/filters` | Filters | List all active filters | Everyone |
| `/flood` | Antiflood | Show current flood settings | Everyone |
| `/formatting` | Formatting | Alias of `/markdownhelp` | Everyone |
| `/funban` | Federations | Alias of `/unfban` | Fed admin |
| `/get` | Notes | Retrieve a saved note | Everyone |
| `/github` | Misc | Look up a GitHub user or repository | Everyone |
| `/goodbye` | Greetings | Show current goodbye settings | Admin |
| `/help` | Help | Show help menu with module list | Everyone |
| `/id` | Misc | Get user or chat ID | Everyone |
| `/import` | Backup | Restore settings from a backup file | Owner |
| `/importfbans` | Federations | Import fbans from a replied CSV/JSON file | Fed owner |
| `/info` | Misc | Get user information | Everyone |
| `/invitelink` | Admin | Get the chat invite link | Everyone (public groups) / users with invite rights (private groups) |
| `/joinfed` | Federations | Join this group to a federation | Group creator |
| `/kick` | Bans | Kick a user from the group | Admin |
| `/kickme` | Bans | Kick yourself from the group | Everyone |
| `/lang` | Languages | Change the bot language | User/Admin |
| `/leavechat` | Devs | Force the bot to leave a specified chat | Dev/Owner |
| `/leavefed` | Federations | Leave the federation | Group creator |
| `/lock` | Locks | Lock a permission type | Admin |
| `/locks` | Locks | Show current lock settings | Admin |
| `/locktypes` | Locks | List available lock types | Everyone |
| `/log` | LogChannels | Enable one or more log categories | Admin |
| `/logcategories` | LogChannels | List log categories | Everyone |
| `/logchannel` | LogChannels | Show the current log channel | Everyone |
| `/markdownhelp` | Formatting | Show markdown formatting guide | Everyone |
| `/mute` | Mutes | Mute a user | Admin |
| `/myfeds` | Federations | List the federations you own or admin | Everyone |
| `/newfed` | Federations | Create a federation (one per user; PM only) | Everyone |
| `/nolog` | LogChannels | Disable one or more log categories | Admin |
| `/notes` | Notes | List all saved notes | Everyone |
| `/permapin` | Pins | Pin a custom message sent by the bot | Admin |
| `/pin` | Pins | Pin a replied-to message | Admin |
| `/ping` | Misc | Check bot response latency | Everyone |
| `/pinned` | Pins | Get the current pinned message | Everyone |
| `/privatenotes` | Notes | Alias of `/privnote` | Admin |
| `/privaterules` | Rules | Toggle private rules delivery | Admin |
| `/privnote` | Notes | Toggle private note delivery | Admin |
| `/promote` | Admin | Promote a user to admin | Admin |
| `/purge` | Purges | Purge messages from replied-to onwards (optional reason) | Admin |
| `/purgefrom` | Purges | Set purge start point | Admin |
| `/purgeto` | Purges | Purge to a specific message (optional reason) | Admin |
| `/quietfed` | Federations | Suppress passive fban notices in this group (on/off) | Admin |
| `/raidactiontime` | AntiRaid | Set the ban duration for raiders | Admin |
| `/raidtime` | AntiRaid | Set the raid duration | Admin |
| `/reactions` | Reactions | List configured reactions | Everyone |
| `/reconnect` | Connections | Reconnect to last connected group | Everyone |
| `/remallbl` | Blacklists | Remove all blacklisted words | Owner |
| `/removereaction` | Reactions | Remove a reaction for a keyword | Admin |
| `/remdev` | Devs | Revoke developer permissions from a user | Owner |
| `/removebotkeyboard` | Misc | Remove a stuck bot keyboard | Everyone |
| `/removefilter` | Filters | Remove a keyword filter | Admin |
| `/remsudo` | Devs | Revoke sudo permissions from a user | Owner |
| `/renamefed` | Federations | Rename your federation | Fed owner |
| `/report` | Reports | Report a user to admins | Everyone |
| `/reports` | Reports | Toggle reporting for the group | Admin |
| `/resetallwarns` | Warns | Reset all warnings for all users | Owner |
| `/reset` | Backup | Reset all settings to default | Owner |
| `/resetgoodbye` | Greetings | Reset goodbye to default | Admin |
| `/resetreactions` | Reactions | Clear all reactions | Admin |
| `/resetrules` | Rules | Reset all group rules | Admin |
| `/resetrulesbtn` | Rules | Reset the rules button text | Admin |
| `/resetrulesbutton` | Rules | Reset the rules button text | Admin |
| `/resetwarn` | Warns | Reset warnings for a user | Admin |
| `/resetwarns` | Warns | Reset warnings for a user | Admin |
| `/resetwelcome` | Greetings | Reset welcome to default | Admin |
| `/restrict` | Bans | Restrict a user's permissions | Admin |
| `/rmallbl` | Blacklists | Alias of `/remallbl` | Owner |
| `/rmallblocklist` | Blacklists | Alias of `/remallbl` | Owner |
| `/rmblacklist` | Blacklists | Remove a word from the blacklist | Admin |
| `/rmblocklist` | Blacklists | Alias of `/rmblacklist` | Admin |
| `/rmblocklistall` | Blacklists | Alias of `/remallbl` | Owner |
| `/rmfilter` | Filters | Remove a keyword filter | Admin |
| `/rmnote` | Notes | Remove a note | Admin |
| `/rmwarn` | Warns | Remove a warning from a user | Admin |
| `/rules` | Rules | Show the group rules | Everyone |
| `/rulesbtn` | Rules | Set the rules button text | Admin |
| `/rulesbutton` | Rules | Set the rules button text | Admin |
| `/runs` | Misc | Reply with a random quip | Everyone |
| `/save` | Notes | Save a note | Admin |
| `/saved` | Notes | List all saved notes | Everyone |
| `/sban` | Bans | Silently ban a user | Admin |
| `/setdescription` | Admin | Alias of `/setgdesc` | Admin |
| `/setfedlog` | Federations | Make this group the federation log chat (groups only) | Fed owner |
| `/setflood` | Antiflood | Set the flood trigger limit | Admin |
| `/setfloodmode` | Antiflood | Set the flood action mode | Admin |
| `/setgdesc` | Admin | Change or clear the group description | Admin |
| `/setgoodbye` | Greetings | Set the goodbye message | Admin |
| `/setgpic` | Admin | Set the group photo (reply to a photo) | Admin |
| `/setgroupdesc` | Admin | Alias of `/setgdesc` | Admin |
| `/setgrouppic` | Admin | Alias of `/setgpic` | Admin |
| `/setgrouptitle` | Admin | Alias of `/setgtitle` | Admin |
| `/setgtitle` | Admin | Change the group title | Admin |
| `/setlog` | LogChannels | Set the log channel | Channel admin + Admin |
| `/setrules` | Rules | Set the group rules | Admin |
| `/setwarnlimit` | Warns | Set the warn limit before action | Admin |
| `/setwarntime` | Warns | Set how long a warn counts | Admin |
| `/setwarnmode` | Warns | Set the warn action mode | Admin |
| `/settings` | Connections | Pick a group in PM and show all its settings | Admin |
| `/setwelcome` | Greetings | Set the welcome message | Admin |
| `/skick` | Bans | Silently kick a user and delete the command | Admin |
| `/smute` | Mutes | Silently mute a user | Admin |
| `/start` | Help | Show welcome message with navigation menu | Everyone |
| `/stat` | Misc | Show message count for the chat | Everyone |
| `/stats` | Devs | Display bot statistics and system info | Dev/Owner |
| `/stickerid` | Misc | Get a replied sticker's file ID | Everyone |
| `/stop` | Filters | Remove a keyword filter | Admin |
| `/stopall` | Filters | Remove all filters | Owner |
| `/subfed` | Federations | Subscribe to another federation's bans (max 5) | Fed owner |
| `/swarn` | Warns | Warn a user and delete your command | Admin |
| `/tban` | Bans | Temporarily ban a user | Admin |
| `/teamusers` | Devs | List all team members | Team |
| `/tell` | Misc | Echo a message via the bot | Admin |
| `/title` | Admin | Set a custom admin title | Admin |
| `/tmute` | Mutes | Temporarily mute a user | Admin |
| `/tr` | Misc | Translate text to another language | Everyone |
| `/ud` | Misc | Look up a term on Urban Dictionary | Everyone |
| `/unapprove` | Approvals | Remove a user from approved list | Admin |
| `/unapproveall` | Approvals | Remove all approved users | Owner |
| `/unban` | Bans | Unban a user | Admin |
| `/unbanall` | Bans | Unban all known users in the chat | Owner |
| `/unblacklist` | Blacklists | Alias of `/rmblacklist` | Admin |
| `/unfban` | Federations | Lift a federation ban | Fed admin |
| `/unlock` | Locks | Unlock a permission type | Admin |
| `/unmute` | Mutes | Unmute a user | Admin |
| `/unpin` | Pins | Unpin the current pinned message | Admin |
| `/unpinall` | Pins | Unpin all pinned messages | Admin |
| `/unrestrict` | Bans | Remove restrictions from a user | Admin |
| `/unsetfedlog` | Federations | Clear the federation log chat | Fed owner |
| `/unsetlog` | LogChannels | Remove the log channel | Admin |
| `/unsubfed` | Federations | Unsubscribe from another federation | Fed owner |
| `/unwarn` | Warns | Remove a warning from a user | Admin |
| `/warn` | Warns | Warn a user | Admin |
| `/warnings` | Warns | Get the chat's warning settings | Admin |
| `/warns` | Warns | Show warning count for a user | Everyone |
| `/welcome` | Greetings | Show current welcome settings | Admin |
| `/wiki` | Misc | Search English Wikipedia | Everyone |
| `/zombies` | Misc | Kick deleted accounts from the group | Admin |

## Command Registration System

### Legacy Registration

Every module except Help, BotUpdates included (`RegisterLegacyModule("BotUpdates", -10, LoadBotUpdates)`), registers with `RegisterLegacyModule` in its `init()` function. Help is loaded last by `main.go` (`defer modules.LoadHelp(dispatcher)`):

```go
func init() {
    RegisterLegacyModule("Bans", 70, LoadBans)
}
```

### Declarative Command Pipeline

The preferred modern approach uses `WrapCommand` with `CommandDescriptor`:

```go
helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
    Name:  "pin",
    Group: pinsModule.handlerGroup, // 10
    RequiredChecks: []helpers.CheckFunc{
        helpers.RequireGroup(),
        helpers.RequireUserAdmin(),
        helpers.RequireBotAdmin(),
        helpers.CanUserPin(),
        helpers.CanBotPin(),
    },
}, pinsModule.pin)
```

`/permapin` is registered as its own descriptor with the same checks. Set `Aliases` and `Disableable: true` on a descriptor only when the command has aliases or chats may disable it.

**Key types** (from `alita/utils/helpers/command_pipeline.go`):

| Type | Description |
|------|-------------|
| `CommandDescriptor` | Declarative command configuration (Name, Aliases, Group, RequiredChecks, Disableable) |
| `CommandContext` | Context passed through the command pipeline |
| `CheckFunc` | Permission check function signature |
| `WrapCommand` | Main pipeline builder with automatic check execution |

Each CheckFunc handles its own failure: the permission checks send the refusal reply, and `CheckDisabled` stops the command silently (deleting it when the chat has `/disabledel` on), so handlers add no replies for failed checks. `Disableable: true` only lists the command for `/disable`; `CheckDisabled` enforces it.

**Pre-built CheckFunc builders**:

| Function | Description |
|----------|-------------|
| `CheckDisabled(cmd)` | Refuses the command when the chat has disabled it (needed alongside `Disableable: true`) |
| `RequireGroup()` | Ensures command is used in a group |
| `RequireBotAdmin()` | Ensures bot is admin |
| `RequireUserAdmin()` | Ensures user is admin |
| `CanUserPromote()` | Checks user can promote members |
| `CanBotPromote()` | Checks bot can promote members |
| `CanUserPin()` | Checks user can pin messages |
| `CanBotPin()` | Checks bot can pin messages |
| `CanInvite()` | Checks the bot and user can manage invite links |
| `CanUserChangeInfo()` | Checks user can change chat info |
| `CanBotChangeInfo()` | Checks bot can change chat info |
| `CanUserRestrict()` | Checks user can restrict members |
| `CanBotRestrict()` | Checks bot can restrict members |
| `RequireUserOwner()` | Ensures user is the chat owner |
| `CanUserDelete()` | Checks user can delete messages |
| `CanBotDelete()` | Checks bot can delete messages |

`RunChecks(c, checks)` runs a list of CheckFuncs outside `WrapCommand` (used by anonymous-admin handlers).

