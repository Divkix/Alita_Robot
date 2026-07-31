---
title: Antispam Commands
description: Complete guide to Antispam module commands and features
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# 📦 Antispam Commands

The Antispam module is a telemetry-only detector. It operates automatically and
has no user commands or per-chat configuration.

## How It Works

- **Fixed window:** Tracks each non-admin, non-approved user separately in each
  chat for one-second windows.
- **Threshold:** The 18th and later messages in the same window produce a
  server-side debug log.
- **Non-blocking:** The module always continues update processing. It does not
  delete or hide messages, restrict users, or prevent other handlers from
  running.
- **Automatic cleanup:** A background task removes stale counters every five
  minutes.

## Limitations

- The one-second window and threshold of 18 are hardcoded.
- Detection events are only written to debug logs; nothing is posted in the
  chat.
- The module does not enforce a spam penalty.

Use **Antiflood** when you need configurable flood detection and enforcement.
