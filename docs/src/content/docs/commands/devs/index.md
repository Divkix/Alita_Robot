---
title: Devs Commands
description: Complete guide to developer and owner commands
---

# Developer and Owner Commands

:::caution[Not surfaced to regular users]
These commands are restricted to the bot's internal team. They do not appear in the `/help` menu, are not listed in public command references, and are invisible to group admins and regular users. Unauthorized users who attempt these commands receive **no response** -- the bot silently ignores the message.
:::

The devs module provides bot management and diagnostic commands for the bot's internal team. The team has three tiers with descending privilege levels. Commands are protected by guard conditions that check the user's team membership before execution.

## Team Hierarchy

| Tier | Role | How to Assign | Capabilities |
|------|------|---------------|-------------|
| 1 | **Owner** | Set via `OWNER_ID` environment variable at deployment | Full control: manage team roster, all diagnostic commands, all team commands |
| 2 | **Sudo / Dev** | Assigned by owner via `/addsudo` or `/adddev` | Diagnostic commands: bot stats, chat info, chat management |
| 3 | **Any Team Member** | Automatically includes owner + all sudo + all dev users | View team roster only |

:::tip
Sudo and Dev have identical command access. The distinction exists for organizational purposes -- sudo users are typically trusted operators, dev users are typically developers with debug access.
:::

## Access Level Table

Each command is mapped to its access level with the exact guard condition from the source code:

| Command | Description | Access Level | Guard Condition |
|---------|-------------|-------------|-----------------|
| `/addsudo` | Grant sudo permissions to a user | Owner Only | `user.Id != config.AppConfig.OwnerId` |
| `/adddev` | Grant developer permissions to a user | Owner Only | `user.Id != config.AppConfig.OwnerId` |
| `/remsudo` | Revoke sudo permissions from a user | Owner Only | `user.Id != config.AppConfig.OwnerId` |
| `/remdev` | Revoke developer permissions from a user | Owner Only | `user.Id != config.AppConfig.OwnerId` |
| `/stats` | Display bot statistics and system info | Owner or Dev | `user.Id != OwnerId && !memStatus.Dev` |
| `/chatinfo` | Display detailed information about a chat | Owner or Dev | `user.Id != OwnerId && !memStatus.Dev` |
| `/chatlist` | Generate and send a list of all active chats | Owner or Dev | `user.Id != OwnerId && !memStatus.Dev` |
| `/leavechat` | Force the bot to leave a specified chat | Owner or Dev | `user.Id != OwnerId && !memStatus.Dev` |
| `/teamusers` | List all team members (owner, sudo, dev) | Any Team Member | User must be in `teamint64Slice` |

## Silent Ignore Behavior

:::note
Unlike admin commands which respond with "you need to be an admin", dev commands use a silent-ignore pattern. When the guard condition fails, the handler simply returns without sending any response. This is intentional: it prevents information leakage about which commands exist and avoids giving non-team users any feedback about restricted functionality.
:::

## Module Aliases

> These are help-menu module names, not command aliases.

This module has no help-menu aliases. It is not listed in the help menu.

## Disableable Status

None of the developer commands are disableable. They cannot be disabled via the `/disable` command because they are not registered with `AddCmdToDisableable()`.

## Usage Examples

### View bot statistics

```
/stats
```

### Get information about a chat

```
/chatinfo -100123456789
```

### Add a sudo user

```
/addsudo <reply to user or user_id>
```

### Add a dev user

```
/adddev <reply to user or user_id>
```

### List team members

```
/teamusers
```

### Force bot to leave a chat

```
/leavechat -100123456789
```
