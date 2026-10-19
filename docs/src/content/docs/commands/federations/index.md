---
title: Federations Commands
description: Complete guide to Federations module commands and features
---

# 🛡️ Federations Commands

Federations share a ban list across groups you own.

### Owner (PM):
- `/newfed <name>`: Create a federation (one per user).
- `/renamefed <name>`: Rename your federation.
- `/delfed`: Delete your federation (confirm).
- `/fedpromote /feddemote <user>`: Promote or demote a federation admin (reply, ID or @username).
- `/fedreason on/off`: Require a reason for fbans.
- `/fednotif on/off`: PM the owner on fbans.
- `/subfed /unsubfed <fed_id>`: Subscribe to or unsubscribe from another federation's bans (max 5).
- `/fedsubs`: List the federations yours subscribes to.
- `/fbanlist [csv|json|minicsv]`: Export fbans.
- `/importfbans`: Import from a replied CSV/JSON file.
- `/unsetfedlog`: Clear the federation log chat.

### Owner (in a group):
- `/setfedlog`: Make this group your federation's log chat.

### Group owner:
- `/joinfed <fed_id>`: Join this chat to a federation.
- `/leavefed`: Leave the federation.

### Group admins:
- `/quietfed on/off`: Suppress passive fban notices.

### Fed admins:
- `/fban <user> [reason]`: Ban a user across the federation (in a fed group; the owner can also use it in PM).
- `/unfban /funban <user>`: Lift a federation ban.
- `/feddemoteme <fed_id>`: Step down as a federation admin.

### Anyone:
- `/fedstat /fbanstat [user] [fed_id]`: Check a user's fbans (yours by default); with a fed_id, show that ban's reason.
- `/fedinfo [fed_id]`: Show federation info.
- `/fedadmins [fed_id]`: List a federation's admins.
- `/chatfed`: Show this group's federation (groups only).
- `/myfeds`: List the federations you own or admin.


## Module Aliases

This module can be accessed using the following aliases:

- `fed`
- `feds`
- `fban`
- `unfban`
- `newfed`
- `joinfed`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/chatfed` | Show this group's federation (groups only). | ✅ |
| `/delfed` | Delete your federation (confirm). | ❌ |
| `/fban` | Ban a user across the federation (in a fed group; the owner can also use it in PM). | ❌ |
| `/fbanlist` | Export fbans. | ❌ |
| `/fbanstat` | Check a user's fbans (yours by default); with a fed_id, show that ban's reason. | ❌ |
| `/fedadmins` | List a federation's admins. | ✅ |
| `/feddemote` | Promote or demote a federation admin (reply, ID or @username). | ❌ |
| `/feddemoteme` | Step down as a federation admin. | ❌ |
| `/fedinfo` | Show federation info. | ✅ |
| `/fednotif` | PM the owner on fbans. | ❌ |
| `/fedpromote` | Promote or demote a federation admin (reply, ID or @username). | ❌ |
| `/fedreason` | Require a reason for fbans. | ❌ |
| `/fedstat` | Check a user's fbans (yours by default); with a fed_id, show that ban's reason. | ✅ |
| `/fedsubs` | List the federations yours subscribes to. | ✅ |
| `/funban` | Lift a federation ban. (Aliases: `unfban, funban`) | ❌ |
| `/importfbans` | Import from a replied CSV/JSON file. | ❌ |
| `/joinfed` | Join this chat to a federation. | ❌ |
| `/leavefed` | Leave the federation. | ❌ |
| `/myfeds` | List the federations you own or admin. | ❌ |
| `/newfed` | Create a federation (one per user). | ❌ |
| `/quietfed` | Suppress passive fban notices. | ❌ |
| `/renamefed` | Rename your federation. | ❌ |
| `/setfedlog` | Make this group your federation's log chat. | ❌ |
| `/subfed` | Subscribe to or unsubscribe from another federation's bans (max 5). | ❌ |
| `/unfban` | Lift a federation ban. (Aliases: `unfban, funban`) | ❌ |
| `/unsetfedlog` | Clear the federation log chat. | ❌ |
| `/unsubfed` | Subscribe to or unsubscribe from another federation's bans (max 5). | ❌ |

## Usage Examples

### Basic Usage

```text
/chatfed
/delfed
/fban
```

For detailed command usage, refer to the commands table above.

## Required Permissions

Most commands are limited to the federation owner or federation admins, not group admins. `/joinfed` and `/leavefed` require the group **creator**, and `/quietfed` requires a group admin. `/newfed` (in PM), `/fedinfo`, `/fedadmins`, `/chatfed`, `/myfeds` and `/fedstat` are open to everyone.

**Bot Permissions Required:**

- Ban users (to enforce and lift federation bans)

