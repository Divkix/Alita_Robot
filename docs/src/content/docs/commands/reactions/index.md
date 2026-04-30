---
title: Reactions Commands
description: Complete guide to Reactions module commands and features
---

# 📦 Reactions Commands

**🎭 Auto-Reactions**

Automatically react to messages with specific keywords!

Setup automated emoji reactions that trigger when users send specific words.

**Admin Commands:**
- /addreaction <keyword> <emoji> - Add an auto-reaction
- /removereaction <keyword> - Remove a reaction
- /reactions - List configured reactions
- /resetreactions - Clear all reactions

**Example:**
- /addreaction hello 👋 - Bot reacts with 👋 when someone says "hello"


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
When any user sends a message in the chat, the bot scans the text for configured
keywords and sets the matching emoji reaction automatically. Only the first matching
keyword is used to avoid rate limiting. The watcher runs silently and uses
`ext.ContinueGroups` so it never blocks other handlers.

## Inline Help Callbacks

The help menu for Reactions includes inline keyboard buttons that show detailed
usage for each command using `reactions_help` prefixed callback data:

| Button | Action |
|--------|--------|
| "Add Reaction" | Shows `/addreaction` usage via `reactions_help.add` |
| "Remove Reaction" | Shows `/removereaction` usage via `reactions_help.remove` |

