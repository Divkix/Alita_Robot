---
title: Antispam
description: Automatic spam protection system for Alita Robot
---

# Antispam

The antispam module provides automatic, behind-the-scenes spam protection for your groups. Unlike other modules, it operates automatically without requiring any user commands.

## How It Works

The antispam system implements per-user-per-chat rate limiting to prevent spam:

- **Rate Limit**: 18 messages per second per user per chat
- **Automatic**: No configuration required - works out of the box
- **Transparent**: Silently drops excessive messages without disrupting normal users
- **Per-User Tracking**: Busy groups with many legitimate users won't trigger false positives

## Technical Details

### Rate Limiting Algorithm

The module uses a sliding window approach:

1. **First Message**: Starts tracking with count = 1
2. **Subsequent Messages**: Increments counter within the time window
3. **Window Expiry**: Counter resets when the time window (1 second) expires
4. **Threshold Exceeded**: Messages beyond 18/second are silently dropped

### Memory Management

- **Automatic Cleanup**: Expired tracking entries are removed every 5 minutes
- **No Memory Leaks**: Background goroutine prevents unbounded memory growth
- **Efficient Storage**: Only active users are tracked

### Concurrency Safety

The module uses proper mutex locking to ensure thread-safe operation:

- **Atomic Operations**: Read-modify-write operations are protected
- **No Race Conditions**: Proper lock ordering prevents data corruption
- **Scalable**: Designed for high-traffic groups

## What Gets Blocked

When spam is detected, the bot:

1. Logs the event (at debug level)
2. Stops processing the message
3. Does **not** notify the user or delete the message

This silent approach prevents:

- Spammers from knowing they've been detected
- Cluttering the chat with spam notifications
- Potential abuse of notification messages

## Limitations

- **No User Commands**: Cannot be enabled/disabled per chat
- **Fixed Threshold**: 18 messages/second is hardcoded
- **No Logging to Chat**: Spam events are only logged server-side

## Comparison with Antiflood

| Feature | Antispam | Antiflood |
|---------|----------|-----------|
| Scope | Per-user | Per-chat configurable |
| Configuration | Automatic | Admin configurable |
| Action | Silent drop | Configurable (mute/ban/kick) |
| Threshold | Fixed (18/sec) | Configurable |
| User notification | No | Yes |

Use **Antispam** for automatic background protection and **Antiflood** for configurable flood control with admin-defined actions.

## Implementation Details

The antispam module:

- Registers at handler group `-2` (high priority, runs before other handlers)
- Uses composite keys `(chatId, userId)` for accurate per-user tracking
- Skips channel posts (no `EffectiveUser`)
- Returns `ext.EndGroups` when spam is detected to stop further processing
