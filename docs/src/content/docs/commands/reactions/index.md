---
title: Reactions Commands
description: Complete guide to Reactions module commands and features
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# 📦 Reactions Commands

**🎭 Auto-Reactions**

Automatically react to messages with specific keywords!

Setup automated emoji reactions that trigger when users send specific words.

**User Commands:**
- /reactions - List configured reactions

**Admin Commands:**
- /addreaction <keyword> <emoji> - Add an auto-reaction
- /removereaction <keyword> - Remove a reaction
- /resetreactions - Clear all reactions

**Example:**
- /addreaction hello 👍 - Bot reacts with 👍 when someone says "hello"
- Only Telegram's standard reaction emoji (for example 👍 ❤ 🔥 🎉) can be used.


## Module Aliases

This module can be accessed using the following aliases:

- `reaction`
- `addreaction`
- `removereaction`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/addreaction` | Add an auto-reaction | ❌ |
| `/reactions` | List configured reactions | ❌ |
| `/removereaction` | Remove a reaction | ❌ |
| `/resetreactions` | Clear all reactions | ❌ |

## Usage Examples

### Basic Usage

```
/addreaction
/reactions
/removereaction
```

For detailed command usage, refer to the commands table above.

## Required Permissions

- `/addreaction`, `/removereaction`, `/resetreactions` — Require `CanUserChangeInfo`
  admin right (admins and group owner only).
- `/reactions` — Available to all users.

## Automatic Message Watcher

The Reactions module includes a background message watcher at **handler group 8**.
When a user sends a text message in the chat, the bot scans the message text for configured
keywords (case-insensitive substring match) and sets the matching emoji reaction automatically.
Media captions are not scanned. A message that mentions @admin or @admins is handled by the
Reports module in the same handler group and gets no reaction. Only the first matching
keyword (in alphabetical order) is used to avoid rate limiting. The watcher runs silently and uses
`ext.ContinueGroups`, so it never stops later handler groups.

## Inline Help Callbacks

The help menu for Reactions includes inline keyboard buttons that show detailed
usage for each command using `reactions_help` prefixed callback data:

| Button | Action |
|--------|--------|
| "Add Reaction" | Shows `/addreaction` usage via `reactions_help\|v1\|action=add` |
| "Remove Reaction" | Shows `/removereaction` usage via `reactions_help\|v1\|action=remove` |
