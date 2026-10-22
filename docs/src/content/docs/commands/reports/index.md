---
title: Reports Commands
description: Complete guide to Reports module commands and features
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# 📦 Reports Commands

We're all busy people who don't have time to monitor our groups 24/7. But how do you react if someone in your group is spamming?

× /report `<reason>`: reply to a message to report it to admins.
- @admin / @admins: same as /report but not a command.

*Admins Only:*
× /reports `<on/off/yes/no/true/false>`: change report setting, or view current status.
- If done in PM, toggles your status. This needs an active /connect to a group where both you and the bot are admins.
- If in a group, toggles that group's status.
× /reports `block` (via reply only): Block a user from using /report or @admin.
× /reports `unblock` (via reply only): Unblock a user from using /report or @admin.
× /reports `showblocklist`: Check all the blocked users who cannot use /report or @admin.
× /reports `mute <N>` / `mute off`: mute a user once N (2-20) different members have open reports against them, until an admin decides and for at most 24 hours. Off by default; never applies to admins or approved users.

To report a user, simply reply to his message with @admin or /report; Alita will then reply with a message stating that admins have been notified.
You MUST reply to a message to report a user; you can't just use @admin to tag admins for no reason!

*NOTE:* Neither of these will get triggered if used by admins.

**How It Works:**

**Reporting a Message:**
1. **Reply to the offending message** with `/report` or mention `@admin`
2. The bot sends a notification mentioning all admins who have reports enabled
3. The notification includes action buttons for quick moderation

**Admin Action Buttons:**
- **➡ Message** - Link to the reported message
- **⚠ Kick** - Kick the reported user (they can rejoin)
- **⛔️ Ban** - Permanently ban the reported user
- **❎ Delete Message** - Delete the reported message
- **✔️ Mark Resolved** - Mark the report as resolved without action

**Log Channel Card:**
If the group has a log channel (`/setlog`) with the `reports` category on, each reported message also gets one card there, with its own **Ban**, **Mute**, **Delete** and **Dismiss** buttons (below). The group still gets the full report with admin mentions and the buttons described above.

In the log channel:
- A copy of the reported message, so the evidence survives if the original is deleted. Messages that cannot be copied are quoted as text instead.
- A card replying to that copy with the reported user, the number of members who reported the message, the last reporter and their reason, and a link to the original message.
- Further reports of the same message update the same card; a member reporting the same message twice is counted once.
- Buttons to **Ban** or **Mute** the reported user, **Delete** the message, or **Dismiss** the report. Only admins of the reported group can use them, with the same rights as in the group; the first decision closes the report and is written on the card.
- Once a report is closed, the same member reporting that message again does not reopen it or post a new card.

Once admins have decided a member's report, that member reporting the same message again gets a reply that the admins already reviewed it, with no admin mentions or buttons, whether or not the group has a log channel.

Every report is also saved in the database with its reporter, target, message ID, the first 100 characters of the message text, and its status, which the buttons above record; the full message lives only in the log channel. Closed reports are deleted 90 days after they were closed, and reports nobody closed are deleted 90 days after they were made. Reports are not included in `/export`. A card's buttons keep working while any report on it is still stored and open; once none is, pressing them answers that the report has expired.

**Auto-mute (off by default):**
`/reports mute <N>` (N from 2 to 20) mutes a user once N different members have open reports against them, until an admin decides. A member who reports several of the user's messages counts once.
- The mute ends on its own after 24 hours at most, even if no admin acts.
- It never applies to admins, approved users, the bot, or anonymous and channel senders, and never replaces a restriction an admin already set.
- Dismissing the report or deleting the message, on the log channel card or with the group buttons, lifts the mute unless the user's other open reports still reach N. Ban, kick and mute leave the user as that action put them.
- A user is muted once until an admin decides: the same reports do not mute them again when the 24 hours end.
- With the `automated` log category on, each auto-mute is logged to the log channel as `#REPORT_MUTE`.
- `/reports mute off` turns it off; `/reports mute` shows the current setting. Changing it needs the right to restrict members, for you and for the bot.

**Personal vs Group Settings:**
- **In PM:** `/reports on/off` toggles whether YOU receive report notifications (needs an active /connect to a group where both you and the bot are admins)
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
