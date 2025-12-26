---
title: Commands Overview
description: Overview of all command modules and categories
---

# 📚 Commands Overview

Alita Robot provides a comprehensive set of commands organized into modules. Each module handles a specific aspect of group management.

## Quick Stats

- **Total Modules**: 20
- **Total Commands**: 120

## Module Categories

### 👑 Administration

#### [👑 Admin](./admin/)

Make it easy to promote and demote users with the admin module!.

**Commands**: 7 | **Aliases**: admins, promote, demote, title

### 🛡️ Moderation

#### [🌊 Antiflood](./antiflood/)

You know how sometimes, people join, send 100 messages, and ruin your chat? With antiflood, that happens no more!.

**Commands**: 4 | **Aliases**: flood

#### [🔨 Bans](./bans/)

Sometimes users can be annoying and you might want to remove them from your chat, this module exactly helps you to deal with that!.

**Commands**: 10 | **Aliases**: ban, kick, dkick, restrict, kickme, unrestrict, sban, dban, tban, unban

#### [🔐 Captcha](./captcha/)

Protect your group from bots and spammers with CAPTCHA verification!.

**Commands**: 7

#### [🔒 Locks](./locks/)

*Admin only*:.

**Commands**: 4 | **Aliases**: lock, unlock

#### [🧹 Purges](./purges/)

*Admin only:*.

**Commands**: 4 | **Aliases**: purge, del

### 📝 Content

#### [🔍 Filters](./filters/)

Filters are case insensitive; every time someone says your trigger words, Alita will reply something else! This can be used to create your commands, if desired.

**Commands**: 7 | **Aliases**: filter

#### [👋 Greetings](./greetings/)

Welcome new members to your groups or say Goodbye after they leave!.

**Commands**: 10 | **Aliases**: welcome, goodbye, greeting

#### [📝 Notes](./notes/)

Save data for future users with notes!.

**Commands**: 8 | **Aliases**: note, notes

#### [📋 Rules](./rules/)

Every chat works with different rules; this module will help make those rules clearer!.

**Commands**: 9 | **Aliases**: rule

### 🔧 User Tools

#### [📄 Formatting](./formatting/)

Alita supports a large number of formatting options to make your messages more expressive.

**Commands**: 0 | **Aliases**: markdownhelp, mdhelp

### ⚙️ Bot Management

#### [📦 Blacklists](./blacklists/)

*User Commands:*.

**Commands**: 6 | **Aliases**: blacklist, unblacklist

#### [📦 Connections](./connections/)

This module allows you to connect to a chat's database, and add things to it without the chat knowing about it! For obvious reasons, you need to be an admin to add things; but any member can view your data.

**Commands**: 5 | **Aliases**: connection, connect

#### [❌ Disabling](./disabling/)

This module allows you to disable some commonly used commands, So, no one can use them.

**Commands**: 5 | **Aliases**: disable, enable

#### [📦 Languages](./languages/)

Not able to change language of the bot?.

**Commands**: 1 | **Aliases**: language, lang

#### [🔧 Misc](./misc/)

× /info: Get your user info, which can be used as a reply or by passing a User Id or Username.

**Commands**: 7 | **Aliases**: extra, extras

#### [📦 Mutes](./mutes/)

Sometimes users can be annoying and you might want to restrict them from sending a message to chat, this module is here to help, you can use this module to mute members in your group.

**Commands**: 5 | **Aliases**: mute, unmute, tmute, smute, dmute

#### [📦 Pins](./pins/)

All the pin-related commands can be found here; keep your chat up to date on the latest news with a simple pinned message!.

**Commands**: 7 | **Aliases**: antichannelpin, cleanlinked, pins

#### [📦 Reports](./reports/)

We're all busy people who don't have time to monitor our groups 24/7.

**Commands**: 2 | **Aliases**: report, reporting

#### [📦 Warns](./warns/)

Keep your members in check with warnings; stop them getting out of control!.

**Commands**: 12 | **Aliases**: warn, warning, warnings

## Getting Started

### Basic Command Syntax

All commands follow this format:

```
/command [required_argument] [optional_argument]
```

### Command Prefixes

Commands can be used with or without the bot username:

- `/start` - Works in private chat or group
- `/start@AlitaRobot` - Explicitly targets this bot in groups

### Getting Help

- `/help` - Show general help and module list
- `/help <module>` - Show detailed help for a specific module
- `/cmds <module>` - List all commands in a module

### Permission Levels

Commands require different permission levels:

- **Everyone**: All group members can use
- **Admin**: Requires admin rights in the group
- **Owner**: Requires group creator/owner status
- **Dev**: Requires bot developer access
