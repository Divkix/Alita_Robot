---
title: Users Commands
description: Complete guide to Users module commands and features
---

# 📦 Users Commands

Automatic background user and chat tracker. This module has no user-facing commands — it silently records every message sender and chat the bot is active in.
### What It Tracks:
- User IDs, usernames, and display names for every message sender.
- Chat IDs and names for every group the bot is in.
- Channel IDs, names, and usernames for linked channels.
### Rate Limiting:
- User updates are throttled to one per UserUpdateInterval.
- Chat updates are throttled to one per ChatUpdateInterval.
Other modules, such as /info, depend on data collected by this module.

**Background Tracker:**
The Users module is a passive background module with no user-facing commands. It runs silently on every message the bot processes, automatically tracking all message senders and all chats the bot is active in.

**What It Tracks:**
- User IDs, usernames, and display names for every message sender
- Chat IDs and names for every group the bot is in
- Channel IDs, names, and usernames for linked channels

**Rate Limiting:**
- User updates: throttled to one per `UserUpdateInterval`
- Chat updates: throttled to one per `ChatUpdateInterval`
- This prevents excessive database writes while keeping data reasonably current

**Passive Operation:**
- Runs as a message watcher at handler group -1 (fires before all command handlers)
- No user interaction required — fully automatic
- Other modules, such as `/info`, depend on data collected by this module


