---
title: Devs Commands
description: Complete guide to Devs module commands and features
---

# 📦 Devs Commands

Bot management and diagnostic commands restricted to the bot owner and trusted developers. These commands are not surfaced to regular users.
### Diagnostic Commands (Owner or Dev):
- `/stats`: Display bot statistics and system info.
- `/chatinfo <chat_id>`: Display detailed information about a chat (requires numeric chat ID argument).
- `/chatlist`: Generate and send a list of all active chats.
- `/leavechat <chat_id>`: Force the bot to leave a specified chat.
### Team Commands:
- `/teamusers`: List all team members (includes owner, devs, sudo users).
### Owner Commands:
- `/addsudo`: Grant sudo permissions to a user.
- `/adddev`: Grant developer permissions to a user.
- `/remsudo`: Revoke sudo permissions from a user.
- `/remdev`: Revoke developer permissions from a user.

**Team Hierarchy:**
The Devs module provides bot management and diagnostic commands restricted to the bot's internal team. The team has three tiers with descending privilege levels.

**Tier 1 — Owner:** Set via `OWNER_ID` environment variable at deployment. Full control: manage team roster, all diagnostic commands, all team commands.

**Tier 2 — Dev:** Assigned by owner via `/adddev`. Has diagnostic command access: `/stats`, `/chatinfo`, `/chatlist`, `/leavechat`. Also listed in `/teamusers`.

**Tier 2 — Sudo:** Assigned by owner via `/addsudo`. Listed in `/teamusers` but does NOT have diagnostic command access.

**Tier 3 — Team Member:** Automatically includes owner + all sudo + all dev users. View team roster only via `/teamusers`.

**Note:** Sudo users only appear in `/teamusers`. `/stats`, `/chatinfo`, `/chatlist`, and `/leavechat` require Dev or Owner.

**Important:** These commands are not visible in the public help menu and silently ignore unauthorized users.


## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/adddev` | Grant developer permissions to a user. | ❌ |
| `/addsudo` | Grant sudo permissions to a user. | ❌ |
| `/chatinfo` | Display detailed information about a chat. | ❌ |
| `/chatlist` | Generate and send a list of all active chats. | ❌ |
| `/leavechat` | Force the bot to leave a specified chat. | ❌ |
| `/remdev` | Revoke developer permissions from a user. | ❌ |
| `/remsudo` | Revoke sudo permissions from a user. | ❌ |
| `/stats` | Display bot statistics and system info. | ❌ |
| `/teamusers` | List all team members. | ❌ |

## Usage Examples

### Basic Usage

```
/adddev
/addsudo
/chatinfo
```

For detailed command usage, refer to the commands table above.

## Required Permissions

| Command | Permissions |
|---------|-------------|
| `/addsudo`, `/adddev`, `/remsudo`, `/remdev` | **Owner only** — verified `user.Id == ownerId` |
| `/stats`, `/chatinfo`, `/chatlist`, `/leavechat` | **Owner or Dev** — checked via `ownerId OR isDev`; sudo excluded |
| `/teamusers` | **Any team member** — owner, dev, or sudo |

Unauthorized users are silently ignored (`ext.ContinueGroups`). These commands are not visible in the public help menu.

