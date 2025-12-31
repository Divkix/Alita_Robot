---
title: Pins Commands
description: Complete guide to Pins module commands and features for managing pinned messages
---

# 📌 Pins Commands

The Pins module provides comprehensive control over pinned messages in your groups. Keep your chat organized and up-to-date with important announcements using pinned messages!

## User Commands

| Command | Description |
|---------|-------------|
| `/pinned` | Get the current pinned message with a convenient button link. |

## Admin Commands

| Command | Description | Permissions Required |
|---------|-------------|---------------------|
| `/pin [loud/notify]` | Pin the message you replied to. Add `loud` or `notify` to send a notification to group members. | Can Pin Messages |
| `/unpin` | Unpin the last pinned message. If used as a reply, unpins the specific replied-to message. | Can Pin Messages |
| `/unpinall` | Unpins all pinned messages in the chat (with confirmation dialog). | Can Pin Messages |
| `/permapin <text>` | Create and pin a custom message through the bot. Supports markdown, buttons, and media attachments. | Can Pin Messages |
| `/antichannelpin [yes/no/on/off]` | Toggle auto-unpinning of linked channel pins. Without arguments, shows current setting. | Admin |
| `/cleanlinked [yes/no/on/off]` | Toggle automatic deletion of messages from the linked channel. Without arguments, shows current setting. | Admin |

## Usage Examples

### Pinning Messages

```
# Pin a message silently (default)
/pin

# Pin a message with notification to all members
/pin loud
/pin notify
```

### Custom Pinned Messages (Permapin)

```
# Pin a text message
/permapin Hello! Welcome to our group, please read the rules.

# Pin with buttons (markdown)
/permapin Check out our website! [Website](buttonurl://example.com)

# Reply to a photo/document/sticker to pin it
/permapin
```

### Managing Channel Pins

```
# Enable anti-channel pin (prevents channel auto-pins)
/antichannelpin on

# Disable anti-channel pin
/antichannelpin off

# Check current setting
/antichannelpin
```

### Cleaning Linked Channel Posts

```
# Enable automatic deletion of linked channel messages
/cleanlinked yes

# Disable it
/cleanlinked no

# Check current setting
/cleanlinked
```

## Features

### Anti-Channel Pin
When enabled, the bot will automatically unpin any message that gets auto-pinned by a linked channel. This is useful when you want to maintain control over what gets pinned in your group.

**Important:** When using anti-channel pins, always use the `/unpin` command instead of unpinning manually through Telegram. Manual unpinning will cause the old message to get re-pinned when the channel sends new messages.

### Clean Linked
When enabled, the bot will automatically delete any messages sent to the group from the linked channel. This keeps your group chat clean from cross-posted channel content.

### Permapin
Create custom pinned messages with:
- Text with HTML/Markdown formatting
- Inline buttons with URLs
- Media attachments (photos, documents, stickers, audio, video, voice notes, video notes)
- All standard message fillings ({first}, {chatname}, etc.)

## Required Permissions

### Bot Permissions
- Must be admin with "Pin Messages" permission

### User Permissions
- Most commands require the user to be an admin with "Pin Messages" permission
- `/antichannelpin` and `/cleanlinked` require admin status (work with connection feature)

## Supported via Connection

The following commands work when connected to a chat via `/connect`:
- `/antichannelpin`
- `/cleanlinked`

## Notes

- The `/unpinall` command shows a confirmation dialog before unpinning all messages
- Permapin supports all media types that Telegram supports for pinning
- Anti-channel pin and clean linked settings are stored per-chat and persist across bot restarts
