---
title: Antispam Commands
description: Complete guide to Antispam module commands and features
---

# Antispam Commands

The antispam module provides automatic, behind-the-scenes spam protection for your groups.

Unlike other modules, it operates automatically without requiring any user commands.

:::note[No Configuration Required]
This module has no commands. It works automatically out of the box for all groups.
:::

## How It Works

Per-user-per-chat rate limiting prevents spam automatically.

- **Rate Limit:** 18 messages per second per user per chat
- **Automatic:** No configuration required — works out of the box
- **Transparent:** Silently drops excessive messages without disrupting normal users
- **Per-User Tracking:** Busy groups with many legitimate users will not trigger false positives

## Technical Details

**Rate Limiting Algorithm:**

The module uses a sliding window approach:

1. **First Message:** Starts tracking with count = 1
2. **Subsequent Messages:** Increments counter within the time window
3. **Window Expiry:** Counter resets when the time window (1 second) expires
4. **Threshold Exceeded:** Messages beyond 18/second are silently dropped

**Memory Management:**
- Automatic cleanup: Expired tracking entries are removed every 5 minutes
- No memory leaks: Background goroutine prevents unbounded memory growth
- Efficient storage: Only active users are tracked

## Limitations

:::caution[Limitations]
- **No User Commands:** Cannot be enabled/disabled per chat
- **Fixed Threshold:** 18 messages/second is hardcoded
- **No Logging to Chat:** Spam events are only logged server-side
:::

:::tip
Use **Antispam** for automatic background protection and **[Antiflood](/commands/antiflood/)** for configurable flood control.
:::
