---
title: Reports Commands
description: Complete guide to Reports module commands and features
---

# 📦 Reports Commands

We're all busy people who don't have time to monitor our groups 24/7. But how do you react if someone in your group is spamming?

× /report `<reason>`: reply to a message to report it to admins.
- @admin: same as /report but not a command.

*Admins Only:*
× /reports `<on/off/yes/no>`: change report setting, or view current status.
- If done in PM, toggles your status.
- If in a group, toggles that group's status.
× /reports `block` (via reply only): Block a user from using /report or @admin.
× /reports `unblock` (via reply only): Unblock a user from using /report or @admin.
× /reports `showblocklist`: Check all the blocked users who cannot use /report or @admin.

To report a user, simply reply to his message with @admin or /report; Natalie will then reply with a message stating that admins have been notified.
You MUST reply to a message to report a user; you can't just use @admin to tag admins for no reason!

*NOTE:* Neither of these will get triggered if used by admins.

## Module Aliases

This module can be accessed using the following aliases:

- `report`
- `reporting`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/report` | reply to a message to report it to admins. | ✅ |
| `/reports` | change report setting, or view current status. | ❌ |

## Usage Examples

### Basic Usage

```
/report
/reports
```

For detailed command usage, refer to the commands table above.

## Required Permissions

Commands in this module are available to all users unless otherwise specified.
