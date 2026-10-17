---
title: Connections Commands
description: Complete guide to Connections module commands and features
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# 📦 Connections Commands

This module allows you to connect to a chat's database, and add things to it without the chat knowing about it! For obvious reasons, you need to be an admin to add things; but any member can view your data. (banned/kicked users can't!)

*Commands*:
× /connect `<chatid>`: Connect to the specified chat, allowing you to view/edit contents.
× /disconnect: Disconnect from the current chat.
× /reconnect: Reconnect to the previously connect chat
× /connection: See information about the currently connected chat.
× /settings: Pick a group where you are an admin and see all of its settings; values that differ from the default are marked with •. Only groups where you have written since the bot joined are listed; for others use /connect `<chatid>` or send /settings in the group.

*Admin Commands:*
× /allowconnect <yes/no>: Allow users to connect to chats or not.
While connected, admins can also run /captcha, /antiraid, /aispam, /anonadmin, /addreaction, /allowconnect and /lang group from PM.

You can retrieve the chat id by using the /id command in your chat. Don't be surprised if the id is negative; all super groups have negative ids.

## Module Aliases

> These are help-menu module names, not command aliases.

This module can be accessed using the following aliases:

- `connection`
- `connect`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/allowconnect` | Toggle whether users can connect to this chat | ❌ |
| `/connect` | Connect to a group chat from private messages | ❌ |
| `/connection` | Show current connection status | ❌ |
| `/disconnect` | Disconnect from the connected group chat | ❌ |
| `/reconnect` | Reconnect to the last connected group chat | ❌ |
| `/settings` | Pick a group in PM and show all of its settings | ❌ |

## Usage Examples

### Basic Usage

```
/allowconnect
/connect
/connection
```

For detailed command usage, refer to the commands table above.

## Required Permissions

- `/allowconnect` — Requires **Admin** (`IsUserAdmin`) in the target group.
  From PM it acts on the connected group.
- `/settings` — Lists and connects only groups where you are an **Admin**;
  rights are checked again when you tap a group.
- `/connect`, `/disconnect`, `/connection`, `/reconnect` — Available to all
  users, but connection authorization depends on the target chat's settings
  (admins always allowed; non-admins need `allow_connect` enabled and must
  be current members, not kicked/left).

## Settings Summary

`/settings` in PM lists up to 20 groups where you are an admin: the
connected group first, then groups where you have written since the bot
joined, most recently active first. Only the 50 most recently active of
those groups are checked, so an admin group past them is not listed; use
`/connect <chatid>` or `/settings` in that group. Tapping a group re-checks
your rights, connects you and shows every setting of that group, one line
per command (for example `/captcha — on`). Lines that differ from what a new group gets
are marked with `•`. **Refresh** re-reads the settings and **Another group**
returns to the list. Showing the summary never writes settings.

`/settings` in a group (admins only) replies with a button that opens the
same summary in PM. Admins who never wrote in the group, and anonymous
admins, are not in the list; they can use `/connect <chatid>` or `/settings`
in the group.

## Inline Keyboard Feature

When a user is connected via PM, the `/connection` command and the connect
flow show an inline keyboard with buttons driven by `connbtns`-prefixed
callback data:

| Button | Target | Shows |
|--------|--------|-------|
| **Admin Commands** | Admins only | List of admin-level commands usable via connection |
| **User Commands** | All connected users | List of user-level commands usable via connection |

A **Back** button (→ `connbtns.Main`) returns to the main connection view.

## Two Connection Modes

`/connect` behaves differently depending on where it's used:

- **Private chat (PM)**: Extracts a chat argument, authorizes the user,
  and connects immediately. Shows the inline keyboard for available commands.
- **Group chat**: Does not connect directly. Instead shows a deep-link button
  `t.me/<bot>?start=connect_<chat_id>` that the user must tap to connect from PM.
