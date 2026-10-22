---
title: Spamdb Commands
description: Complete guide to the Spamdb module — the per-chat spam database check on join, against CAS, lols.bot and the bot's Shared bans network
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# 🛡 Spamdb Commands

The Spamdb module is an opt-in, per-chat check of new members against spam
databases: [CAS](https://cas.chat) (Combot Anti-Spam), checked by default,
[lols.bot](https://lols.bot), and the bot's own **Shared bans** network. When
the check is on, every member who joins is looked up in the chat's databases,
and one that any of them lists is banned, muted or only reported, as the chat
chooses.

It is off in every chat until an admin turns it on, and Shared bans is off
until the chat opts in with `/spamdb db shared on`.

## Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/spamdb` | Show whether the check is on, its databases, its action, and what is sent where | ✅ |
| `/spamdb on` | Turn the check on for this chat | ✅ |
| `/spamdb off` | Turn the check off for this chat | ✅ |
| `/spamdb db <cas/lols/shared> <on/off>` | Choose which databases the check asks | ✅ |
| `/spamdb action <ban/mute/log>` | Choose what happens to a listed member | ✅ |

`on`/`off` also accept `yes`/`no`. The databases and the action are kept when
the check is turned off and on again.

## Databases

| Database | Default | Notes |
|----------|---------|-------|
| `cas` — [CAS](https://cas.chat) | On | Lists accounts reported for spam across many chats. |
| `lols` — [lols.bot](https://lols.bot) | Off | Lists accounts more aggressively than CAS, so expect more false positives. Consider `/spamdb action log` until you trust it. |
| `shared` — Shared bans | Off | The bot's own aggregate of human moderation actions. Lists a user that several distinct chats recently banned or kicked, until a human unbans them. Sends nothing anywhere. |

At least one database stays on: switching off the last one is refused, and the
reply says how to turn the check off instead. With more than one on, a member
is acted on when any of them lists them.

## Shared bans

**Shared bans** is the bot's own reputation network, not a third-party
database. The bot records every human ban, kick, mute and unban as one
moderation event, and the shared source lists a user that at least
`REPUTATION_MIN_CHATS` distinct chats (default `3`, minimum `2`) banned or
kicked inside the last `REPUTATION_WINDOW_DAYS` days (default `30`, at most
`180`). Ten bans from one chat still count as one chat. A human unban inside
the same window clears the signal, so a correction propagates. Mutes never
count as removals.

It is off in every chat. An admin opts in with `/spamdb db shared on` and opts
out with `/spamdb db shared off`; turning the check off or switching the
database off stops new lookups immediately. Historical events from the chat
stay part of the aggregate until they age out, so opting out stops the chat
being *looked up*, not its past actions being *counted*.

The signal is computed from `moderation_events`: a chat ID, the actor's user
ID, the target's user ID, the action and the time. No message text, message ID
or reason content is used for the signal, and nothing about a chat or its
members leaves the bot for it. Automated actions — spamdb itself, captcha,
antiraid, locks, blacklists, federations — are not recorded, so a join block
never feeds the network. Events older than 180 days are deleted. Turning the
operator's `ENABLE_REPUTATION` off stops every shared lookup at once, and
`/spamdb db shared on` then refuses.

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

The Telegram user ID of each new member, sent only to the external databases
the chat selected, and nothing else. `/spamdb` names them in the chat. lols.bot
needs no API key, and the CAS key is never sent to it. Shared bans sends
nothing: it is computed inside the bot.

Verdicts are cached per database in Redis (and in-process) so a returning
member is not looked up again at once: a listing for six hours, a clean answer
for thirty minutes. A chat that asks only CAS never reads a lols.bot verdict.
Shared bans carries its own ten-minute cache, so the second join of a member is
served without another database query.

## Failure Behaviour

The databases are asked in parallel. A listing from one is acted on at once,
without waiting for the others. A member counts as clean only when every
selected database answered clean.

The check fails open: a timeout, an error from a database or an answer the bot
does not recognise lets the member in (unless another database lists them), and
is not cached, so the next join asks again. Unrecognised answers are logged
with the start of the body so an API change shows up in the logs. A failing
shared lookup is treated the same way: logged and not listed.

## Required Permissions

| Command | Permissions |
|---------|-------------|
| `/spamdb` and all its options | Admin only — requires `IsUserAdmin` |

Enabling is refused, with a message explaining why, when the bot operator has
switched the check off. Switching Shared bans on is refused the same way while
the operator has not enabled the shared reputation network.

## Operator Setup

| Variable | Meaning |
|----------|---------|
| `ENABLE_SPAMDB` | Kill switch. Default `true`; the check still stays off in chats that never enabled it. `false` stops every lookup. |
| `CAS_API_KEY` | Optional CAS API key, sent as `Authorization: Bearer`. CAS still accepts requests without one. |
| `ENABLE_REPUTATION` | Kill switch for Shared bans. Default `false`; while it is off, `/spamdb db shared on` refuses and no shared lookup runs. |
| `REPUTATION_MIN_CHATS` | Distinct chats that must have removed a user before Shared bans lists them. Default `3`, minimum `2`. |
| `REPUTATION_WINDOW_DAYS` | Rolling window of the shared signal. Default `30`, at most `180` (the event retention). |

The key is registered with `logredact`, so it never appears in logs. The
reputation settings are validated at startup: a non-integer or out-of-range
value fails startup rather than silently using a default.
