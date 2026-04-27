---
title: Devs Commands
description: Complete guide to Devs module commands and features
---

# 📦 Devs Commands

Bot management and diagnostic commands restricted to the bot owner and trusted developers. These commands are not surfaced to regular users.
### Team Commands:
- `/stats`: Display bot statistics and system info.
- `/teamusers`: List all team members.
### Owner Commands:
- `/addsudo`: Grant sudo permissions to a user.
- `/adddev`: Grant developer permissions to a user.
- `/remsudo`: Revoke sudo permissions from a user.
- `/remdev`: Revoke developer permissions from a user.
### Diagnostic Commands:
- `/chatinfo`: Display detailed information about a chat.
- `/chatlist`: Generate and send a list of all active chats.
- `/leavechat`: Force the bot to leave a specified chat.

**Team Hierarchy:**
The Devs module provides bot management and diagnostic commands restricted to the bot's internal team. The team has three tiers with descending privilege levels.

**Tier 1 — Owner:** Set via `OWNER_ID` environment variable at deployment. Full control: manage team roster, all diagnostic commands, all team commands.

**Tier 2 — Sudo / Dev:** Assigned by owner via `/addsudo` or `/adddev`. Diagnostic commands: bot stats, chat info, chat management.

**Tier 3 — Team Member:** Automatically includes owner + all sudo + all dev users. View team roster only via `/teamusers`.

**Note:** Sudo and Dev have identical command access. The distinction exists for organizational purposes.

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

Commands in this module are available to all users unless otherwise specified.

