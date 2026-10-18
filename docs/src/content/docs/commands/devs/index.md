---
title: Devs Commands
description: Complete guide to Devs module commands and features
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# 📦 Devs Commands

Bot management and diagnostic commands restricted to the bot owner and trusted developers. These commands are not surfaced to regular users.
### Diagnostic Commands (Owner or Dev):
- `/stats`: Display bot statistics and system info.
- `/chatinfo <chat_id>`: Display detailed information about a chat (requires numeric chat ID argument).
- `/chatlist`: Generate and send a list of all active chats.
- `/leavechat <chat_id>`: Force the bot to leave a specified chat.
### Team Commands:
- `/teamusers`: List dev and sudo users. The owner can run it but is not listed, and a user with both roles appears only under Dev.
### Owner Commands:
- `/addsudo <reply/@username/user_id>`: Grant sudo permissions to a user.
- `/adddev <reply/@username/user_id>`: Grant developer permissions to a user.
- `/remsudo <reply/@username/user_id>`: Revoke sudo permissions from a user.
- `/remdev <reply/@username/user_id>`: Revoke developer permissions from a user.

**Team Hierarchy:**
The Devs module provides bot management and diagnostic commands restricted to the bot's internal team. The team has three tiers with descending privilege levels.

**Tier 1 — Owner:** Set via `OWNER_ID` environment variable at deployment. Full control: manage team roster, all diagnostic commands, all team commands.

**Tier 2 — Dev:** Assigned by owner via `/adddev`. Has diagnostic command access: `/stats`, `/chatinfo`, `/chatlist`, `/leavechat`. Also listed in `/teamusers`, and marked as a dev user in `/info`.

**Tier 2 — Sudo:** Assigned by owner via `/addsudo`. Listed in `/teamusers` but does NOT have diagnostic command access.

**Promotions:** When a Dev or Sudo user who is a chat admin with the right to add admins uses `/promote`, the new admin gets every admin right the bot holds, even ones the promoter lacks. The chat's creator gets the same treatment. Other admins can only pass on rights they hold themselves.

**Tier 3 — Team Member:** Automatically includes owner + all sudo + all dev users. View team roster only via `/teamusers`.

**Note:** Of the commands on this page, Sudo users can use only `/teamusers`. `/stats`, `/chatinfo`, `/chatlist`, and `/leavechat` require Dev or Owner.

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
/adddev @username
/addsudo 123456789
/chatinfo -1001234567890
```

For detailed command usage, refer to the commands table above.

## Required Permissions

| Command | Permissions |
|---------|-------------|
| `/addsudo`, `/adddev`, `/remsudo`, `/remdev` | **Owner only** — verified `user.Id == ownerId` |
| `/stats`, `/chatinfo`, `/chatlist`, `/leavechat` | **Owner or Dev** — checked via `ownerId OR isDev`; sudo excluded |
| `/teamusers` | **Any team member** — owner, dev, or sudo |

Unauthorized users are silently ignored: the bot does not reply. These commands are not visible in the public help menu.
