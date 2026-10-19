---
title: Aispam Commands
description: Complete guide to the Aispam module — the per-chat AI spam filter
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# 🤖 Aispam Commands

The Aispam module is an opt-in, per-chat AI spam filter. When it is on, every
new message in the chat is judged by TypeSafe's Jev decision model against the
chat's own title, description and rules, and messages the model is confident
are spam are deleted.

It is the only module that deletes on a judgement rather than a rule, and it is
off in every chat until an admin turns it on.

## Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/aispam` | Show whether the filter is on, and exactly what is sent to the model | ✅ |
| `/aispam on` | Turn the filter on for this chat | ✅ |
| `/aispam off` | Turn the filter off for this chat | ✅ |

`on` also accepts `yes`, `true` or `enable`, and `off` also accepts `no`, `false`
or `disable` (case-insensitive). A bare `/aispam` reports status and lists the
data sent to the model.

## What Happens To A Flagged Message

Only one action exists: the message is deleted. No warning, no mute, no
restriction, no reply, and no notice to the sender. Every deletion is mirrored
to `MESSAGE_DUMP` with the probability, category, model version and the message
text, which is the audit trail and the only feedback loop for tuning.

## Thresholds

| The model reads the message as | Minimum probability to delete |
|-------------------------------|------------------------------|
| English | 0.80 |
| Anything else, including too short or mixed to tell | 0.90 |

Jev is English-first, so only a message it reads as English takes the lower bar.
The message's own language decides, not the chat's configured language: an
English chat still receives posts written in other languages, and those are held
to the higher bar. If the language answer is missing or unreadable, the higher
bar applies.

## What Is Skipped

These messages are never sent to the model:

- Anything from an admin or an approved user.
- Members still solving a captcha challenge.
- This bot's own commands: a bare `/command`, or `/command@thisbot`. A slash
  message aimed at another bot is judged like any other text.
- Messages with no text or caption, and messages from bots, channels, anonymous
  admins or linked channels.
- Edits to a message already posted: only a message as first posted is judged,
  so a message edited into spam afterwards is not checked again.

## What The Model Receives

For each checked message:

- The chat's title, description, rules and language.
- The sender's profile: days since the bot first saw them, messages in the last
  hour, warn count.
- The message text or caption, the domains it links to, and whether it is a
  forward or a reply.
- The sender's few most recent messages in this chat, and how many messages they
  posted in the last hour.

The sender window is kept in Redis, keyed per chat and user. The message count
covers only the last hour; the sender's last five texts are kept until they have
posted nothing in the chat for an hour, then expire on their own. Nothing else
about a message is stored. A message joins the
window even when the check that follows it fails, and losing Redis costs the
window rather than the check: the message is still judged, with no history.

No names, usernames, user IDs, phone numbers, or messages from other chats are
sent. `/aispam` prints this list in the chat.

## Failure Behaviour

The filter fails open: a timeout, rate limit, server error or unparseable
answer produces no verdict and no action. `429` and `5xx` get at most one retry,
after the provider's `Retry-After` (0.5 s if it sends none). If it asks for more
than 2 s, or the check's 8 s budget runs out first, there is no retry and the
check fails open. After five consecutive failures in a chat, checks for
that chat pause for five minutes and the chat gets one notice; the stored
setting is untouched, so protection resumes on its own.

Checks run on a bounded worker queue. When the queue is full — a raid, or a
provider brownout — checks are shed rather than queued unbounded. A shed check
means "not checked", and antiflood still owns floods.

## Required Permissions

| Command | Permissions |
|---------|-------------|
| `/aispam`, `/aispam on`, `/aispam off` | Admin only — requires `IsUserAdmin` |

**Bot Permissions Required:** the bot must be an admin with the Delete Messages permission. Enabling does not check this. Without it, messages are still sent to the model (and still use the operator's API quota) and deletions are still mirrored to `MESSAGE_DUMP`, but nothing is removed from the chat.

Enabling is refused, with a message explaining why, when the bot operator has
not configured an API key or has switched the filter off globally, so an admin is
not told their chat is protected when the operator has not set the filter up. It
does not check the bot's own rights; see above.

## Operator Setup

| Variable | Meaning |
|----------|---------|
| `TYPESAFE_API_KEY` | TypeSafe API key used for every check. One credential for the whole bot; chats cannot bring their own. Empty disables the feature. |
| `ENABLE_AISPAM` | Global kill switch. Default `true`; the filter still stays off in chats that never enabled it. |

The key is registered with `logredact`, so it never appears in logs.
