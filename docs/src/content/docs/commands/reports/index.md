---
title: Reports Commands
description: Complete guide to Reports module commands and features
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# 📦 Reports Commands

We're all busy people who don't have time to monitor our groups 24/7. But how do you react if someone in your group is spamming?

× /report `<reason>`: reply to a message to report it to admins.
- @admin: same as /report but not a command.

*Admins Only:*
× /reports `<on/off/yes/no/true/false>`: change report setting, or view current status.
- If done in PM, toggles your status.
- If in a group, toggles that group's status.
× /reports `block` (via reply only): Block a user from using /report or @admin.
× /reports `unblock` (via reply only): Unblock a user from using /report or @admin.
× /reports `showblocklist`: Check all the blocked users who cannot use /report or @admin.

To report a user, simply reply to his message with @admin or /report; Alita will then reply with a message stating that admins have been notified.
You MUST reply to a message to report a user; you can't just use @admin to tag admins for no reason!

*NOTE:* Neither of these will get triggered if used by admins.

**How It Works:**

**Reporting a Message:**
1. **Reply to the offending message** with `/report` or mention `@admin`
2. The bot sends a notification mentioning all admins who have reports enabled
3. The notification includes action buttons for quick moderation

**Admin Action Buttons:**
- **📩 Message** - Link to the reported message
- **👢 Kick** - Kick the reported user (they can rejoin)
- **🚫 Ban** - Permanently ban the reported user
- **🗑 Delete** - Delete the reported message
- **✅ Resolved** - Mark the report as resolved without action

**Personal vs Group Settings:**
- **In PM:** `/reports on/off` toggles whether YOU receive report notifications
- **In Group:** `/reports on/off` toggles whether reporting is enabled for that group


## Module Aliases

> These are help-menu module names, not command aliases.

This module can be accessed using the following aliases:

- `report`
- `reporting`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/report` | Report a user to chat admins | ✅ |
| `/reports` | Toggle reporting in the group | ❌ |

## Usage Examples

### Basic Usage

```
/report
/reports
```

For detailed command usage, refer to the commands table above.

## Required Permissions

**Who Can Report?**
✅ Regular users
❌ Admins (no need to report to themselves)
❌ Blocked users
❌ Anonymous channels / Telegram system accounts

**Who Can Be Reported?**
✅ Regular users
❌ The bot itself
❌ Admins (protected from reports)
❌ Anonymous channels / Telegram system accounts
