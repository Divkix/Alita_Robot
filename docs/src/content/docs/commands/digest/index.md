---
title: Digest Commands
description: Complete guide to Digest module commands and features
---

# 📦 Digest Commands

**📊 Weekly digest**

The bot counts messages, joins and leaves in this chat every day, in UTC, and keeps the totals for a year. Admins can turn on a weekly digest that posts the previous week's totals, with the change from the week before, every Monday after 00:00 UTC. It goes to the log channel when this chat has one, otherwise to the group.

### Admin commands:

- `/digest`: Show whether the digest is on and where it goes.
- `/digest on`: Turn the weekly digest on.
- `/digest off`: Turn the weekly digest off.
- `/digest here`: Send the digest to this group.
- `/digest log`: Send the digest to the log channel (needs a log channel).
- `/digest now`: Preview this week so far with the same rendering as the digest.

The counts are kept per chat and never include message text.


## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/digest` | Show whether the digest is on and where it goes. | ❌ |

## Usage Examples

### Basic Usage

```text
/digest
```

For detailed command usage, refer to the commands table above.

## Required Permissions

Every `/digest` form requires **admin permissions** in the group. The digest itself is sent by the bot.

**Bot Permissions Required:**

- Send messages (in the group, or in the log channel when the digest goes there)

