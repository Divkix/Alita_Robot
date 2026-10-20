---
title: Spamdb Commands
description: Complete guide to the Spamdb module — the per-chat CAS and lols.bot spam database check on join
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# 🛡 Spamdb Commands

The Spamdb module is an opt-in, per-chat check of new members against spam
databases: [CAS](https://cas.chat) (Combot Anti-Spam), checked by default, and
[lols.bot](https://lols.bot), which a chat can add. When the check is on, every
member who joins is looked up in the chat's databases, and one that any of them
lists is banned, muted or only reported, as the chat chooses.

It is off in every chat until an admin turns it on.

## Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/spamdb` | Show whether the check is on, its databases, its action, and what is sent where | ✅ |
| `/spamdb on` | Turn the check on for this chat | ✅ |
| `/spamdb off` | Turn the check off for this chat | ✅ |
| `/spamdb db <cas/lols> <on/off>` | Choose which databases the check asks | ✅ |
| `/spamdb action <ban/mute/log>` | Choose what happens to a listed member | ✅ |

`on`/`off` also accept `yes`/`no`. The databases and the action are kept when
the check is turned off and on again.

## Databases

| Database | Default | Notes |
|----------|---------|-------|
| `cas` — [CAS](https://cas.chat) | On | Lists accounts reported for spam across many chats. |
| `lols` — [lols.bot](https://lols.bot) | Off | Lists accounts more aggressively than CAS, so expect more false positives. Consider `/spamdb action log` until you trust it. |

At least one database stays on: switching off the last one is refused, and the
reply says how to turn the check off instead. With both on, a member is acted on
when either lists them.

## Actions

| Action | What happens to a member a database lists |
|--------|-------------------------------------------|
| `ban` | Banned from the chat (default) |
| `mute` | Restricted from sending messages |
| `log` | Nothing; the join is only reported |

Every action is reported to the chat's [log channel](/commands/logchannels/)
under the `automated` category, never in the group, so a raid does not flood
the chat with notices. The notice names the database that listed the member.
Without a log channel the action still happens silently.
`ban` and `mute` need the bot to be an admin allowed to ban users.

## What Is Skipped

- Admins and approved users.
- Bots, including this bot.
- Anything but a group or supergroup.

## What The Databases Receive

The Telegram user ID of each new member, sent only to the databases the chat
selected, and nothing else. `/spamdb` names them in the chat. lols.bot needs no
API key, and the CAS key is never sent to it.

Verdicts are cached per database in Redis (and in-process) so a returning
member is not looked up again at once: a listing for six hours, a clean answer
for thirty minutes. A chat that asks only CAS never reads a lols.bot verdict.

## Failure Behaviour

The databases are asked in parallel. A listing from one is acted on at once,
without waiting for the other. A member counts as clean only when every
selected database answered clean.

The check fails open: a timeout, an error from a database or an answer the bot
does not recognise lets the member in (unless the other database lists them),
and is not cached, so the next join asks again. Unrecognised answers are logged
with the start of the body so an API change shows up in the logs.

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
