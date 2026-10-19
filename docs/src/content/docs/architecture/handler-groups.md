---
title: Handler Group Precedence
description: Which message watchers fire first and how propagation works in Alita Robot.
---

# Handler Group Precedence

gotgbot dispatches incoming updates to handlers in ascending group number order. Lower group numbers fire first. Within the same group, handlers fire in registration order. A handler returning `ext.EndGroups` stops all further group processing for that update. `ext.ContinueGroups` allows downstream groups to continue firing.

This page documents every handler group the bot registers outside the default group 0, with its exact group number, verified from source code.

## Precedence Table

| Group | Module | Handler | Filter | Return on Match | Notes |
|-------|--------|---------|--------|-----------------|-------|
| -10 | Captcha | `handlePendingCaptchaMessage` | nil (all messages) | `EndGroups` | Intercepts messages from users with pending captcha; stores and deletes message |
| -6 | Federations | `enforceFedBan` | `message.All` | `ContinueGroups` (always) | Bans senders and new members who are banned in the chat's federation; later groups still run |
| -5 | AntiRaid | `onJoin` | Message with `new_chat_members` | `ContinueGroups` (always) | While raid mode is on (or once the auto-antiraid join threshold is reached, which turns it on), temp-bans each new joiner for the raid action time; skips the bot, admins and approved users; does nothing if the bot cannot restrict |
| -2 | BotUpdates | `adminCacheAutoUpdate` | MyChatMember (bot's own admin status change) | `ContinueGroups` | Invalidates and reloads the admin cache when the bot is promoted or demoted |
| -1 | BotUpdates | `botJoinedGroup` | MyChatMember (bot joined; channel joins are filtered out and never reach it) | `EndGroups` for private chats and basic groups (it leaves basic groups), `ContinueGroups` for supergroups | Early interceptor for bot group joins; leaves basic groups and stays in channels |
| -1 | Users | `logUsers` | `message.All` | `ContinueGroups` | Logs user activity; never blocks propagation |
| 0 | All modules | Commands, callbacks | Various | varies | Default group; standard command handlers |
| 1 | Connections | `connectedChatGuard` | update whose `EffectiveChat` a group-0 PM command swapped to the connected group | `EndGroups` | Keeps later groups from running on the swapped context; group 1 is reserved for this guard |
| 3 | AISpam | `checkAISpam` | `message.All` | `ContinueGroups` (always) | Opt-in AI spam filter; enqueues a check and never blocks propagation |
| 4 | Antiflood | `checkFlood` | `message.All` | `ContinueGroups` (always) | Flood control; on flood deletes the message(s), punishes the sender and posts a notice without stopping later groups; skips anon admins, linked channels, approved users and media groups |
| 5 | Locks | `permHandler` | `message.All` | `ContinueGroups` (always) | Content-type locks (sticker, gif, url, forward, rtl, anonchannel, ...); deletes the offending message but does not block later groups |
| 6 | Locks | `restHandler` | `message.All` | `ContinueGroups` (always) | Restriction locks (messages, comments, media, other, previews, all); deletes the offending message but does not block later groups |
| 7 | Blacklists | `blacklistWatcher` | text or caption present | `EndGroups` after a warn (the default action); `ContinueGroups` otherwise (an error moves on to the next group) | Checks words against blacklist; after a warn, later groups (reports, filters, pins, log channel) do not run |
| 8 | Reports | `report` | text mentioning `@admin`/`@admins` (case-insensitive, whole word) | `EndGroups` (or error) | Treats a message that mentions @admin/@admins as a /report to the chat's admins |
| 8 | Reactions | `checkReactions` | `message.All` (acts only on text messages) | `ContinueGroups` (always) | Sets the chat's configured emoji reaction on messages whose text contains a keyword (case-insensitive substring; first matching keyword in sorted order wins); never mutes, bans, or blocks propagation |
| 9 | Filters | `filtersWatcher` | text or caption present | `ContinueGroups`; `EndGroups` when a non-admin sends `<keyword> noformat` | Matches filter patterns; sends configured response but does not block |
| 10 | Pins | `checkPinned` | linked-channel automatic forwards (`GetSender().IsLinkedChannel()`) | `ContinueGroups` | Deletes the post if /cleanlinked is on, otherwise unpins it if /antichannelpin is on. Group 10 also holds the /pin, /unpin, /unpinall, /permapin and /pinned commands (WrapCommand, `Group: 10`) |
| 11 | LogChannels | `captureSetLogForward` | `message.All` | `ContinueGroups` (always) | Captures the channel post forwarded to finish /setlog |

## Propagation Behavior

:::caution[Critical propagation rules]
Understanding these rules is essential for debugging why a message did or did not trigger a particular watcher.

- **Captcha is first (group -10).** Any message from a user with a pending captcha is stored for moderator review and deleted. The message never reaches antiraid, antiflood, blacklists, or filters.
- **AntiRaid bans raid joiners (group -5).** During raid mode, users who join (seen through the join service message) are temp-banned for the raid action time. The handler always returns `ContinueGroups`, so later groups still see the join message.
- **Filters use ContinueGroups; Blacklists depend on the action.** Filters send their reply and never block later watchers. A blacklist `mute`/`ban`/`kick`/`tban`/`tmute`/`delete`/`none` action returns `ContinueGroups`, so the message is still checked by filters. The `warn` action (the default) returns `EndGroups`, so reports, filters, pins and the log channel skip that message. One exception: when a non-admin sends `<keyword> noformat`, Filters replies with the admin-only error and returns `EndGroups`, so pins and the log channel skip that update.
- **Locks use ContinueGroups (groups 5-6).** A locked message is deleted, but blacklists, reports/reactions, filters and pins still see the update.
:::

## Interaction Examples

### Scenario 1: User with pending captcha sends a message

1. **Captcha (group -10)** intercepts the message, stores it in the pending queue, and deletes it from the chat.
2. Returns `EndGroups`. Nothing else fires.
3. After successful verification, the stored messages are deleted and the bot sends a type/count summary; their contents are not replayed.

### Scenario 2: User sends a flood of messages

1. Captcha (group -10) passes through (no pending captcha).
2. AntiRaid (group -5) does not match (it only handles join service messages).
3. Users (group -1) logs activity, continues.
4. Commands (group 0) fire normally for earlier messages.
5. **Antiflood (group 4)** triggers once the flood threshold is reached: it deletes the flooding message(s), applies the configured action (mute/kick/ban, or tmute/tban from older data), posts a notice, and returns `ContinueGroups`.
6. Locks, blacklists, reports/reactions, filters, pins and the log channel still receive the update, although the message is already deleted from the chat.

### Scenario 3: Message matches both a blacklist word and a filter

1. Captcha (group -10) passes through.
2. AntiRaid (group -5) does not match (it only handles join service messages).
3. Users (group -1) logs activity, continues.
4. Commands (group 0) — not a command, passes.
5. Antiflood (group 4) — not flooding, continues.
6. **Locks (groups 5-6)** check the chat's content-type and restriction locks. Message passes (no lock violation).
7. **Blacklists (group 7)** matches the word, takes the configured action (e.g., mute user), returns `ContinueGroups`.
8. Group 8: Reports does nothing (the message does not mention @admin/@admins); Reactions adds an emoji reaction if a configured keyword matches, and returns `ContinueGroups`.
9. **Filters (group 9)** matches the filter pattern, sends the configured response, returns `ContinueGroups`.
10. Pins (group 10) does not match (the message is not a linked-channel post).

Both the blacklist action and the filter response are triggered for the same message.

If the blacklist action is `warn` (the default), the watcher returns `EndGroups` at step 7: steps 8-10 do not run and the filter does not reply.

## Related Pages

- [Request Flow](/architecture/request-flow) — Full update processing pipeline
- [Module Pattern](/architecture/module-pattern) — How modules register handlers
