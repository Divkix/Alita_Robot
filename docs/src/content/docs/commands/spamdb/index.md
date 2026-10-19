---
title: Spamdb Commands
description: Complete guide to the Spamdb module — the per-chat CAS spam database check on join
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# 🛡 Spamdb Commands

The Spamdb module is an opt-in, per-chat check of new members against the
[CAS](https://cas.chat) (Combot Anti-Spam) database. When it is on, every member
who joins is looked up in CAS, and one that CAS lists is banned, muted or only
reported, as the chat chooses.

It is off in every chat until an admin turns it on.

## Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/spamdb` | Show whether the check is on, its action, and what is sent to CAS | ✅ |
| `/spamdb on` | Turn the check on for this chat | ✅ |
| `/spamdb off` | Turn the check off for this chat | ✅ |
| `/spamdb action <ban/mute/log>` | Choose what happens to a listed member | ✅ |

`on`/`off` also accept `yes`/`no`. The action defaults to `ban` and is kept when
the check is turned off and on again.

## Actions

| Action | What happens to a member CAS lists |
|--------|------------------------------------|
| `ban` | Banned from the chat (default) |
| `mute` | Restricted from sending messages |
| `log` | Nothing; the join is only reported |

Every action is reported to the chat's [log channel](/commands/logchannels/)
under the `automated` category, never in the group, so a raid does not flood
the chat with notices. Without a log channel the action still happens silently.
`ban` and `mute` need the bot to be an admin allowed to ban users.

## What Is Skipped

- Admins and approved users.
- Bots, including this bot.
- Anything but a group or supergroup.

## What CAS Receives

The Telegram user ID of each new member, and nothing else. `/spamdb` says so in
the chat.

Verdicts are cached in Redis (and in-process) so a returning member is not
looked up again at once: a listing for six hours, a clean answer for thirty
minutes.

## Failure Behaviour

The check fails open: a timeout, an error from CAS or an answer the bot does not
recognise lets the member in, and is not cached, so the next join asks again.
Unrecognised CAS answers are logged with the start of the body so an API change
shows up in the logs.

## Required Permissions

| Command | Permissions |
|---------|-------------|
| `/spamdb` and all its options | Admin only — requires `IsUserAdmin` |

Enabling is refused, with a message explaining why, when the bot operator has
switched the check off.

## Operator Setup

| Variable | Meaning |
|----------|---------|
| `ENABLE_SPAMDB` | Kill switch. Default `true`; the check still stays off in chats that never enabled it. `false` stops every lookup. |
| `CAS_API_KEY` | Optional CAS API key, sent as `Authorization: Bearer`. CAS still accepts requests without one. |

The key is registered with `logredact`, so it never appears in logs.
