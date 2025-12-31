---
title: Warns Commands
description: Complete guide to Warns module commands and features
---

# Warns Commands

Keep your members in check with warnings; stop them getting out of control!
If you're looking for automated warnings, read about the blacklist module!

## Admin Commands

| Command | Description | Permission Required |
|---------|-------------|---------------------|
| `/warn <user> [reason]` | Warn a user with an optional reason | Admin with restrict permission |
| `/dwarn <reason>` | Warn a user by reply and delete their message | Admin with restrict permission |
| `/swarn <reason>` | Silently warn a user and delete your command message | Admin with restrict permission |
| `/rmwarn <user>` | Remove the latest warning from a user | Admin |
| `/unwarn <user>` | Alias for `/rmwarn` | Admin |
| `/resetwarn <user>` | Reset all warnings for a user to 0 | Admin |
| `/resetwarns <user>` | Alias for `/resetwarn` | Admin |
| `/resetallwarns` | Delete all warnings for all users in the chat | Owner only |
| `/setwarnmode <ban/kick/mute>` | Set the action when warn limit is reached | Admin |
| `/setwarnlimit <1-100>` | Set the number of warnings before punishment | Admin |
| `/warnings` | Display current warning settings (limit and mode) | Admin |

## User Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/warns [user]` | Check your own or another user's warnings | Yes |

## Default Settings

- **Default warn limit**: 3 warnings
- **Default warn mode**: mute

## Available Warn Modes

| Mode | Action |
|------|--------|
| `ban` | Permanently ban the user from the chat |
| `kick` | Kick the user (they can rejoin) |
| `mute` | Mute the user (cannot send messages) |

## Usage Examples

### Warning a User

```
# Warn by reply
/warn For breaking the rules

# Warn by username
/warn @username Spamming links

# Warn by user ID
/warn 123456789 Inappropriate behavior
```

### Silent and Delete Warnings

```
# Delete your command and warn silently
/swarn Stop spamming

# Delete the user's message and warn them
/dwarn No advertising allowed
```

### Managing Warnings

```
# Check a user's warnings
/warns @username

# Remove the latest warning
/rmwarn @username

# Reset all warnings for a user
/resetwarn @username

# Reset all warnings in the chat (owner only)
/resetallwarns
```

### Configuring Warn Settings

```
# Set warn limit to 5
/setwarnlimit 5

# Set action to ban when limit is reached
/setwarnmode ban

# View current settings
/warnings
```

## Permission Requirements

| Command | Required Permission |
|---------|---------------------|
| `/warn`, `/dwarn`, `/swarn` | Bot admin + User admin with restrict |
| `/rmwarn`, `/unwarn` | Bot admin + User admin |
| `/resetwarn`, `/resetwarns` | Bot admin + User admin |
| `/resetallwarns` | Chat owner only |
| `/setwarnmode`, `/setwarnlimit` | Bot admin + User admin |
| `/warnings` | Bot admin + User admin |
| `/warns` | Any user (disableable) |

## Notes

- When a user reaches the warn limit, the configured action (ban/kick/mute) is applied automatically
- Warning reasons are stored and displayed when checking a user's warns
- The `/warns` command is the only command in this module that can be disabled by admins
- Anonymous channel posts cannot be warned
- Admins cannot be warned
