---
title: Handler Group Precedence
description: Which message watchers fire first and how propagation works in Alita Robot.
---

# Handler Group Precedence

gotgbot dispatches incoming updates to handlers in ascending group number order. Lower group numbers fire first. Within the same group, handlers fire in registration order. A handler returning `ext.EndGroups` stops all further group processing for that update. `ext.ContinueGroups` allows downstream groups to continue firing.

This page documents every registered message watcher with its exact handler group number, verified from source code.

## Precedence Table

| Group | Module | Handler | Filter | Return on Match | Notes |
|-------|--------|---------|--------|-----------------|-------|
| -10 | Captcha | `handlePendingCaptchaMessage` | nil (all messages) | `EndGroups` | Intercepts messages from users with pending captcha; stores and deletes message |
| -2 | Antispam | (inline closure) | `message.All` | `EndGroups` | Rate-limits spamming users; passes through if not spamming |
| -1 | Users | `logUsers` | `message.All` | `ContinueGroups` | Logs user activity; never blocks propagation |
| 0 | All modules | Commands, callbacks | Various | varies | Default group; standard command handlers |
| 4 | Antiflood | `checkFlood` | `message.All` | `EndGroups` on flood, `ContinueGroups` otherwise | Flood control; skips anon admins and media groups |
| 5 | Locks | `permHandler` | `message.All` | `EndGroups` on violation | Permission-based locks (can_send_messages, etc.) |
| 6 | Locks | `restHandler` | `message.All` | `EndGroups` on violation | Restriction-based locks (stickers, animations, etc.) |
| 7 | Blacklists | `blacklistWatcher` | non-command, non-media-group | `ContinueGroups` (always) | Checks words against blacklist; action taken but processing continues |
| 8 | Reports | (handler) | `message.All` | varies | Report tracking |
| 9 | Filters | `filtersWatcher` | non-command, non-media-group | `ContinueGroups` (always) | Matches filter patterns; sends configured response but does not block |
| 10 | Pins | (handler) | `message.All` | `EndGroups` | Anti-channel-pin watcher |

## Propagation Behavior

:::caution[Critical propagation rules]
Understanding these rules is essential for debugging why a message did or did not trigger a particular watcher.

- **Captcha is first (group -10).** Any message from a user with a pending captcha is intercepted, stored for delivery after verification, and deleted. The message never reaches antiflood, blacklists, or filters.
- **Antispam EndGroups stops all further processing.** When antispam (group -2) returns `EndGroups` for a rate-limited user, every downstream handler is skipped. A rate-limited message never reaches any other watcher.
- **Blacklists and Filters use ContinueGroups.** Both take their configured action (warn, mute, ban for blacklists; send reply for filters) but do not block downstream watchers. A message matching a blacklist word still gets checked by filters.
:::

## Interaction Examples

### Scenario 1: User with pending captcha sends a message

1. **Captcha (group -10)** intercepts the message, stores it in the pending queue, and deletes it from the chat.
2. Returns `EndGroups`. Nothing else fires.
3. After the user completes captcha verification, stored messages are replayed.

### Scenario 2: User sends a flood of messages

1. Captcha (group -10) passes through (no pending captcha).
2. Antispam (group -2) passes through (not yet rate-limited).
3. Users (group -1) logs activity, continues.
4. Commands (group 0) fire normally for earlier messages.
5. **Antiflood (group 4)** triggers after the flood threshold is reached, returns `EndGroups`.
6. Locks, blacklists, filters, and pins are all skipped for that message.

### Scenario 3: Message matches both a blacklist word and a filter

1. Captcha (group -10) passes through.
2. Antispam (group -2) passes through.
3. Users (group -1) logs activity, continues.
4. Commands (group 0) — not a command, passes.
5. Antiflood (group 4) — not flooding, continues.
6. **Locks (groups 5-6)** check permissions. Message passes (no lock violation).
7. **Blacklists (group 7)** matches the word, takes configured action (e.g., warn user), returns `ContinueGroups`.
8. Reports (group 8) tracks the message.
9. **Filters (group 9)** matches the filter pattern, sends the configured response, returns `ContinueGroups`.
10. Pins (group 10) checks for anti-channel-pin conditions.

Both the blacklist action and the filter response are triggered for the same message.

## Related Pages

- [Request Flow](/architecture/request-flow) — Full update processing pipeline
- [Module Pattern](/architecture/module-pattern) — How modules register handlers
