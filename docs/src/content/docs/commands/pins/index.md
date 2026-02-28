---
title: Pins Commands
description: Complete guide to Pins module commands and features
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# 📦 Pins Commands

All the pin-related commands can be found here; keep your chat up to date on the latest news with a simple pinned message!

*User commands:*
× /pinned: Get the current pinned message.

*Admin commands:*
× /pin: Pin the message you replied to. Add 'loud' or 'notify' to send a notification to group members.
× /pinned: Gets the latest pinned message in the current Chat.
× /permapin <text>: Pin a custom message through the bot. This message can contain markdown, buttons, and all the other cool features.
× /unpin: Unpin the current pinned message. If used as a reply, unpins the replied-to message.
× /unpinall: Unpins all pinned messages.
× /antichannelpin <yes/no/on/off>: Don't let telegram auto-pin linked channels. If no arguments are given, show the current setting.
× /cleanlinked <yes/no/on/off>: Delete messages sent by the linked channel.
Note: When using anti channel pins, make sure to use the /unpin command, instead of doing it manually. Otherwise, the old message will get re-pinned when the channel sends any messages.

**Features:**

**Anti-Channel Pin:**
When enabled, the bot will automatically unpin any message that gets auto-pinned by a linked channel. This is useful when you want to maintain control over what gets pinned in your group.

**Important:** When using anti channel pins, always use the `/unpin` command instead of unpinning manually through Telegram. Manual unpinning will cause the old message to get re-pinned when the channel sends new messages.

**Clean Linked:**
When enabled, the bot will automatically delete any messages sent to the group from the linked channel. This keeps your group chat clean from cross-posted channel content.

**Permapin:**
Create custom pinned messages with:
- Text with HTML/Markdown formatting
- Inline buttons with URLs
- Media attachments (photos, documents, stickers, audio, video, voice notes, video notes)
- All standard message fillings ({first}, {chatname}, etc.)

**Supported via Connection:**
The following commands work when connected to a chat via `/connect`:
- `/antichannelpin`
- `/cleanlinked`


## Module Aliases

> These are help-menu module names, not command aliases.

This module can be accessed using the following aliases:

- `antichannelpin`
- `cleanlinked`
- `pins`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/antichannelpin` | Toggle automatic unpinning of channel posts | ❌ |
| `/cleanlinked` | Toggle deletion of linked channel messages | ❌ |
| `/permapin` | Pin a message permanently (re-pins if unpinned) | ❌ |
| `/pin` | Pin the replied message | ❌ |
| `/pinned` | Get the current pinned message | ❌ |
| `/unpin` | Unpin the current pinned message | ❌ |
| `/unpinall` | Unpin all pinned messages | ❌ |

## Usage Examples

### Basic Usage

```
/antichannelpin
/cleanlinked
/permapin
```

For detailed command usage, refer to the commands table above.

## Required Permissions

Commands in this module are available to all users unless otherwise specified.

## Technical Notes

**Notes:**
- The `/unpinall` command shows a confirmation dialog before unpinning all messages
- Permapin supports all media types that Telegram supports for pinning
- Anti-channel pin and clean linked settings are stored per-chat and persist across bot restarts

**Required Permissions:**
**Bot:** Must be admin with "Pin Messages" permission
**User:** Most commands require admin with "Pin Messages" permission
