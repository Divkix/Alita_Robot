---
title: Mutes Commands
description: Complete guide to Mutes module commands and features
---

# Mutes Commands

Sometimes users can be annoying and you might want to restrict them from sending messages. This module lets you mute members in your group.

:::caution[Admin Permissions Required]
All mute commands require admin permissions with restrict rights in groups.
:::

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/mute` | Mutes a user (via handle or reply). | No |
| `/smute` | Mutes a user silently — no group message, deletes your command. | No |
| `/dmute` | Mutes a user and deletes the replied message. | No |
| `/tmute` | Mutes a user for a specified time (via handle or reply). | No |
| `/unmute` | Unmutes a user (via handle or reply). | No |

## Usage Examples

```text
/mute @username          # Mute a user
/mute @username Spamming # Mute with reason
/smute @username         # Mute silently
/dmute                   # Reply to mute and delete their message
/tmute @username 2h      # Mute for 2 hours
/unmute @username        # Unmute a user
```

:::tip[Time Format for Temporary Mutes]
Use `/tmute` with a time suffix: `m` = minutes, `h` = hours, `d` = days.

```text
/tmute @user 30m    # Mute for 30 minutes
/tmute @user 2h     # Mute for 2 hours
/tmute @user 1d     # Mute for 1 day
```
:::

## Mute Variants

| Variant | Behavior |
|---------|----------|
| `/mute` | Standard mute with optional reason |
| `/smute` | Silent mute (deletes your command message) |
| `/dmute` | Delete-mute (mutes user and deletes their message) |
| `/tmute` | Temporary mute with specified duration |

## Module Aliases

This module can be accessed using the following aliases:
`mute`, `unmute`, `tmute`, `smute`, `dmute`

## Required Permissions

**Bot Permissions Required:**
- Restrict members

**User must have:**
- Admin status in the chat
- Permission to restrict members
