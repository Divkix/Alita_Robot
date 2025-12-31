---
title: Reports Commands
description: Complete guide to Reports module commands and features
---

# 📦 Reports Commands

We're all busy people who don't have time to monitor our groups 24/7. But how do you react if someone in your group is spamming?

The Reports module allows regular users to report problematic messages to admins, who then receive actionable buttons to quickly take action.

## User Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/report` | Reply to a message to report it to admins | ✅ |
| `@admin` | Same as /report but not a command - mention anywhere in your message | N/A |

## Admin Commands

| Command | Description |
|---------|-------------|
| `/reports` | View current report settings status |
| `/reports on/yes` | Enable reports in the chat |
| `/reports off/no` | Disable reports in the chat |
| `/reports block` | Reply to block a user from reporting |
| `/reports unblock` | Reply to unblock a user |
| `/reports showblocklist` | List all blocked users |

## How It Works

### Reporting a Message

1. **Reply to the offending message** with `/report` or mention `@admin`
2. The bot sends a notification mentioning all admins who have reports enabled
3. The notification includes action buttons for quick moderation

### Admin Action Buttons

When a report is submitted, admins see these action buttons:

| Button | Action |
|--------|--------|
| **📩 Message** | Link to the reported message |
| **👢 Kick** | Kick the reported user (they can rejoin) |
| **🚫 Ban** | Permanently ban the reported user |
| **🗑 Delete** | Delete the reported message |
| **✅ Resolved** | Mark the report as resolved without action |

### Personal vs Group Settings

- **In PM:** `/reports on/off` toggles whether YOU receive report notifications
- **In Group:** `/reports on/off` toggles whether reporting is enabled for that group

## Usage Examples

### Report a Message
```
# Reply to a message
/report

# Or simply mention admins
This message is spam @admin
```

### Block Abusive Reporters
```
# Reply to the abuser's message
/reports block

# Later unblock them
/reports unblock
```

### Check Blocked Users
```
/reports showblocklist
```

## Who Can Report?

- ✅ Regular users
- ❌ Admins (no need to report to themselves)
- ❌ Blocked users
- ❌ Anonymous channels / Telegram system accounts

## Who Can Be Reported?

- ✅ Regular users
- ❌ The bot itself
- ❌ Admins (protected from reports)
- ❌ Anonymous channels / Telegram system accounts

## Required Permissions

| Action | Permission Required |
|--------|---------------------|
| `/report` | None (regular users) |
| `/reports` settings | Admin |
| Action buttons | Admin |

## Module Aliases

This module can be accessed using the following aliases:

- `report`
- `reporting`

## Technical Notes

The reports module uses callback-based inline buttons for admin actions. Each report stores the reported user's ID and message ID in the callback data for processing when admins click action buttons.
