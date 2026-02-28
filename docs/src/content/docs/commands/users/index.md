---
title: Users Module
description: Automatic user and chat tracking module
---

# Users Module

The users module is a passive background module with no user-facing commands. It runs silently on every message the bot processes, automatically tracking all message senders and all chats the bot is active in.

It records user IDs, usernames, and display names for every message sender, and chat IDs and names for every group. Database updates are rate-limited to prevent excessive writes. Other modules rely on this data -- for example, `/info` in the Misc module uses user data collected by this module.

## Module Aliases

> These are help-menu module names, not command aliases.

This module has no help-menu aliases. It operates silently in the background.

## How It Works

- Runs as a message watcher at handler group -1 (fires before all command handlers)
- Tracks: user ID, username, and display name for every message sender
- Tracks: chat ID and chat name for every group the bot is in
- Tracks: channel ID, name, and username for linked channels
- Rate-limited: user updates throttled to one per `UserUpdateInterval`, chat updates to one per `ChatUpdateInterval`
- No user interaction required -- fully automatic

## Required Permissions

This module requires no permissions. It operates automatically on all messages.
