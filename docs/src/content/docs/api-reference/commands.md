---
title: Command Reference
description: Complete reference of all Alita Robot commands
---

# 🤖 Command Reference

This page provides a complete reference of all commands available in Alita Robot.

## Overview

- **Total Modules**: 20
- **Total Commands**: 120

## Commands by Module

### 👑 Admin

| Command | Handler | Disableable | Aliases |
|---------|---------|-------------|----------|
| `/admincache` | `adminCache` | ❌ | — |
| `/adminlist` | `adminlist` | ✅ | — |
| `/anonadmin` | `anonAdmin` | ❌ | — |
| `/demote` | `demote` | ❌ | — |
| `/invitelink` | `getinvitelink` | ❌ | — |
| `/promote` | `promote` | ❌ | — |
| `/title` | `setTitle` | ❌ | — |

### 🌊 Antiflood

| Command | Handler | Disableable | Aliases |
|---------|---------|-------------|----------|
| `/delflood` | `setFloodDeleter` | ❌ | — |
| `/flood` | `flood` | ✅ | — |
| `/setflood` | `setFlood` | ❌ | — |
| `/setfloodmode` | `setFloodMode` | ❌ | — |

### 🔨 Bans

| Command | Handler | Disableable | Aliases |
|---------|---------|-------------|----------|
| `/ban` | `ban` | ❌ | — |
| `/dban` | `dBan` | ❌ | — |
| `/dkick` | `dkick` | ❌ | — |
| `/kick` | `kick` | ❌ | — |
| `/kickme` | `kickme` | ❌ | — |
| `/restrict` | `restrict` | ❌ | — |
| `/sban` | `sBan` | ❌ | — |
| `/tban` | `tBan` | ❌ | — |
| `/unban` | `unban` | ❌ | — |
| `/unrestrict` | `unrestrict` | ❌ | — |

### 📦 Blacklists

| Command | Handler | Disableable | Aliases |
|---------|---------|-------------|----------|
| `/addblacklist` | `addBlacklist` | ❌ | — |
| `/blacklist` | `addBlacklist` | ❌ | — |
| `/blacklistaction` | `setBlacklistAction` | ❌ | — |
| `/blacklists` | `listBlacklists` | ✅ | — |
| `/blaction` | `setBlacklistAction` | ❌ | — |
| `/rmblacklist` | `removeBlacklist` | ❌ | — |

### 🔐 Captcha

| Command | Handler | Disableable | Aliases |
|---------|---------|-------------|----------|
| `/captcha` | `captchaCommand` | ❌ | — |
| `/captchaaction` | `captchaActionCommand` | ❌ | — |
| `/captchaclear` | `clearPendingMessages` | ❌ | — |
| `/captchamaxattempts` | `captchaMaxAttemptsCommand` | ❌ | — |
| `/captchamode` | `captchaModeCommand` | ❌ | — |
| `/captchapending` | `viewPendingMessages` | ❌ | — |
| `/captchatime` | `captchaTimeCommand` | ❌ | — |

### 📦 Connections

| Command | Handler | Disableable | Aliases |
|---------|---------|-------------|----------|
| `/allowconnect` | `allowConnect` | ❌ | — |
| `/connect` | `connect` | ❌ | — |
| `/connection` | `connection` | ❌ | — |
| `/disconnect` | `disconnect` | ❌ | — |
| `/reconnect` | `reconnect` | ❌ | — |

### ❌ Disabling

| Command | Handler | Disableable | Aliases |
|---------|---------|-------------|----------|
| `/disable` | `disable` | ❌ | — |
| `/disableable` | `disableable` | ❌ | — |
| `/disabled` | `disabled` | ✅ | — |
| `/disabledel` | `disabledel` | ❌ | — |
| `/enable` | `enable` | ❌ | — |

### 🔍 Filters

| Command | Handler | Disableable | Aliases |
|---------|---------|-------------|----------|
| `/addfilter` | `addFilter` | ❌ | — |
| `/filter` | `addFilter` | ❌ | — |
| `/filters` | `filtersList` | ✅ | — |
| `/removefilter` | `rmFilter` | ❌ | — |
| `/rmfilter` | `rmFilter` | ❌ | — |
| `/stop` | `rmFilter` | ❌ | — |
| `/stopall` | `rmAllFilters` | ❌ | — |

### 👋 Greetings

| Command | Handler | Disableable | Aliases |
|---------|---------|-------------|----------|
| `/autoapprove` | `autoApprove` | ❌ | — |
| `/cleangoodbye` | `cleanGoodbye` | ❌ | — |
| `/cleanservice` | `delJoined` | ❌ | — |
| `/cleanwelcome` | `cleanWelcome` | ❌ | — |
| `/goodbye` | `goodbye` | ❌ | — |
| `/resetgoodbye` | `resetGoodbye` | ❌ | — |
| `/resetwelcome` | `resetWelcome` | ❌ | — |
| `/setgoodbye` | `setGoodbye` | ❌ | — |
| `/setwelcome` | `setWelcome` | ❌ | — |
| `/welcome` | `welcome` | ❌ | — |

### 📦 Languages

| Command | Handler | Disableable | Aliases |
|---------|---------|-------------|----------|
| `/lang` | `changeLanguage` | ❌ | — |

### 🔒 Locks

| Command | Handler | Disableable | Aliases |
|---------|---------|-------------|----------|
| `/lock` | `lockPerm` | ❌ | — |
| `/locks` | `locks` | ✅ | — |
| `/locktypes` | `locktypes` | ✅ | — |
| `/unlock` | `unlockPerm` | ❌ | — |

### 🔧 Misc

| Command | Handler | Disableable | Aliases |
|---------|---------|-------------|----------|
| `/id` | `getId` | ✅ | — |
| `/info` | `info` | ✅ | — |
| `/ping` | `ping` | ✅ | — |
| `/removebotkeyboard` | `removeBotKeyboard` | ❌ | — |
| `/stat` | `stat` | ✅ | — |
| `/tell` | `echomsg` | ❌ | — |
| `/tr` | `translate` | ✅ | — |

### 📦 Mutes

| Command | Handler | Disableable | Aliases |
|---------|---------|-------------|----------|
| `/dmute` | `dMute` | ❌ | — |
| `/mute` | `mute` | ❌ | — |
| `/smute` | `sMute` | ❌ | — |
| `/tmute` | `tMute` | ❌ | — |
| `/unmute` | `unmute` | ❌ | — |

### 📝 Notes

| Command | Handler | Disableable | Aliases |
|---------|---------|-------------|----------|
| `/addnote` | `addNote` | ❌ | — |
| `/clear` | `rmNote` | ❌ | — |
| `/clearall` | `rmAllNotes` | ❌ | — |
| `/get` | `getNotes` | ✅ | — |
| `/notes` | `notesList` | ✅ | — |
| `/rmnote` | `rmNote` | ❌ | — |
| `/save` | `addNote` | ❌ | — |
| `/saved` | `notesList` | ❌ | — |

### 📦 Pins

| Command | Handler | Disableable | Aliases |
|---------|---------|-------------|----------|
| `/antichannelpin` | `antichannelpin` | ❌ | — |
| `/cleanlinked` | `cleanlinked` | ❌ | — |
| `/permapin` | `permaPin` | ❌ | — |
| `/pin` | `pin` | ❌ | — |
| `/pinned` | `pinned` | ❌ | — |
| `/unpin` | `unpin` | ❌ | — |
| `/unpinall` | `unpinAll` | ❌ | — |

### 🧹 Purges

| Command | Handler | Disableable | Aliases |
|---------|---------|-------------|----------|
| `/del` | `delCmd` | ❌ | — |
| `/purge` | `purge` | ❌ | — |
| `/purgefrom` | `purgeFrom` | ❌ | — |
| `/purgeto` | `purgeTo` | ❌ | — |

### 📦 Reports

| Command | Handler | Disableable | Aliases |
|---------|---------|-------------|----------|
| `/report` | `report` | ✅ | — |
| `/reports` | `reports` | ❌ | — |

### 📋 Rules

| Command | Handler | Disableable | Aliases |
|---------|---------|-------------|----------|
| `/clearrulesbtn` | `resetRulesBtn` | ❌ | — |
| `/clearrulesbutton` | `resetRulesBtn` | ❌ | — |
| `/privaterules` | `privaterules` | ❌ | — |
| `/resetrulesbtn` | `resetRulesBtn` | ❌ | — |
| `/resetrulesbutton` | `resetRulesBtn` | ❌ | — |
| `/rules` | `sendRules` | ✅ | — |
| `/rulesbtn` | `rulesBtn` | ❌ | — |
| `/rulesbutton` | `rulesBtn` | ❌ | — |
| `/setrules` | `setRules` | ❌ | — |

### 📦 Warns

| Command | Handler | Disableable | Aliases |
|---------|---------|-------------|----------|
| `/dwarn` | `dWarnUser` | ❌ | — |
| `/resetallwarns` | `resetAllWarns` | ❌ | — |
| `/resetwarn` | `resetWarns` | ❌ | — |
| `/resetwarns` | `resetWarns` | ❌ | — |
| `/rmwarn` | `removeWarn` | ❌ | — |
| `/setwarnlimit` | `setWarnLimit` | ❌ | — |
| `/setwarnmode` | `setWarnMode` | ❌ | — |
| `/swarn` | `sWarnUser` | ❌ | — |
| `/unwarn` | `removeWarn` | ❌ | — |
| `/warn` | `warnUser` | ❌ | — |
| `/warnings` | `warnings` | ❌ | — |
| `/warns` | `warns` | ✅ | — |

## Alphabetical Index

| Command | Module | Handler |
|---------|--------|----------|
| `/addblacklist` | Blacklists | `addBlacklist` |
| `/addfilter` | Filters | `addFilter` |
| `/addnote` | Notes | `addNote` |
| `/admincache` | Admin | `adminCache` |
| `/adminlist` | Admin | `adminlist` |
| `/allowconnect` | Connections | `allowConnect` |
| `/anonadmin` | Admin | `anonAdmin` |
| `/antichannelpin` | Pins | `antichannelpin` |
| `/autoapprove` | Greetings | `autoApprove` |
| `/ban` | Bans | `ban` |
| `/blacklist` | Blacklists | `addBlacklist` |
| `/blacklistaction` | Blacklists | `setBlacklistAction` |
| `/blacklists` | Blacklists | `listBlacklists` |
| `/blaction` | Blacklists | `setBlacklistAction` |
| `/captcha` | Captcha | `captchaCommand` |
| `/captchaaction` | Captcha | `captchaActionCommand` |
| `/captchaclear` | Captcha | `clearPendingMessages` |
| `/captchamaxattempts` | Captcha | `captchaMaxAttemptsCommand` |
| `/captchamode` | Captcha | `captchaModeCommand` |
| `/captchapending` | Captcha | `viewPendingMessages` |
| `/captchatime` | Captcha | `captchaTimeCommand` |
| `/cleangoodbye` | Greetings | `cleanGoodbye` |
| `/cleanlinked` | Pins | `cleanlinked` |
| `/cleanservice` | Greetings | `delJoined` |
| `/cleanwelcome` | Greetings | `cleanWelcome` |
| `/clear` | Notes | `rmNote` |
| `/clearall` | Notes | `rmAllNotes` |
| `/clearrulesbtn` | Rules | `resetRulesBtn` |
| `/clearrulesbutton` | Rules | `resetRulesBtn` |
| `/connect` | Connections | `connect` |
| `/connection` | Connections | `connection` |
| `/dban` | Bans | `dBan` |
| `/del` | Purges | `delCmd` |
| `/delflood` | antiflood | `setFloodDeleter` |
| `/demote` | Admin | `demote` |
| `/disable` | Disabling | `disable` |
| `/disableable` | Disabling | `disableable` |
| `/disabled` | Disabling | `disabled` |
| `/disabledel` | Disabling | `disabledel` |
| `/disconnect` | Connections | `disconnect` |
| `/dkick` | Bans | `dkick` |
| `/dmute` | Mutes | `dMute` |
| `/dwarn` | Warns | `dWarnUser` |
| `/enable` | Disabling | `enable` |
| `/filter` | Filters | `addFilter` |
| `/filters` | Filters | `filtersList` |
| `/flood` | antiflood | `flood` |
| `/get` | Notes | `getNotes` |
| `/goodbye` | Greetings | `goodbye` |
| `/id` | Misc | `getId` |
| `/info` | Misc | `info` |
| `/invitelink` | Admin | `getinvitelink` |
| `/kick` | Bans | `kick` |
| `/kickme` | Bans | `kickme` |
| `/lang` | Languages | `changeLanguage` |
| `/lock` | Locks | `lockPerm` |
| `/locks` | Locks | `locks` |
| `/locktypes` | Locks | `locktypes` |
| `/mute` | Mutes | `mute` |
| `/notes` | Notes | `notesList` |
| `/permapin` | Pins | `permaPin` |
| `/pin` | Pins | `pin` |
| `/ping` | Misc | `ping` |
| `/pinned` | Pins | `pinned` |
| `/privaterules` | Rules | `privaterules` |
| `/promote` | Admin | `promote` |
| `/purge` | Purges | `purge` |
| `/purgefrom` | Purges | `purgeFrom` |
| `/purgeto` | Purges | `purgeTo` |
| `/reconnect` | Connections | `reconnect` |
| `/removebotkeyboard` | Misc | `removeBotKeyboard` |
| `/removefilter` | Filters | `rmFilter` |
| `/report` | Reports | `report` |
| `/reports` | Reports | `reports` |
| `/resetallwarns` | Warns | `resetAllWarns` |
| `/resetgoodbye` | Greetings | `resetGoodbye` |
| `/resetrulesbtn` | Rules | `resetRulesBtn` |
| `/resetrulesbutton` | Rules | `resetRulesBtn` |
| `/resetwarn` | Warns | `resetWarns` |
| `/resetwarns` | Warns | `resetWarns` |
| `/resetwelcome` | Greetings | `resetWelcome` |
| `/restrict` | Bans | `restrict` |
| `/rmblacklist` | Blacklists | `removeBlacklist` |
| `/rmfilter` | Filters | `rmFilter` |
| `/rmnote` | Notes | `rmNote` |
| `/rmwarn` | Warns | `removeWarn` |
| `/rules` | Rules | `sendRules` |
| `/rulesbtn` | Rules | `rulesBtn` |
| `/rulesbutton` | Rules | `rulesBtn` |
| `/save` | Notes | `addNote` |
| `/saved` | Notes | `notesList` |
| `/sban` | Bans | `sBan` |
| `/setflood` | antiflood | `setFlood` |
| `/setfloodmode` | antiflood | `setFloodMode` |
| `/setgoodbye` | Greetings | `setGoodbye` |
| `/setrules` | Rules | `setRules` |
| `/setwarnlimit` | Warns | `setWarnLimit` |
| `/setwarnmode` | Warns | `setWarnMode` |
| `/setwelcome` | Greetings | `setWelcome` |
| `/smute` | Mutes | `sMute` |
| `/stat` | Misc | `stat` |
| `/stop` | Filters | `rmFilter` |
| `/stopall` | Filters | `rmAllFilters` |
| `/swarn` | Warns | `sWarnUser` |
| `/tban` | Bans | `tBan` |
| `/tell` | Misc | `echomsg` |
| `/title` | Admin | `setTitle` |
| `/tmute` | Mutes | `tMute` |
| `/tr` | Misc | `translate` |
| `/unban` | Bans | `unban` |
| `/unlock` | Locks | `unlockPerm` |
| `/unmute` | Mutes | `unmute` |
| `/unpin` | Pins | `unpin` |
| `/unpinall` | Pins | `unpinAll` |
| `/unrestrict` | Bans | `unrestrict` |
| `/unwarn` | Warns | `removeWarn` |
| `/warn` | Warns | `warnUser` |
| `/warnings` | Warns | `warnings` |
| `/warns` | Warns | `warns` |
| `/welcome` | Greetings | `welcome` |
