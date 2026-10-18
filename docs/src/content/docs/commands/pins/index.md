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
× /pin: Pin the message you replied to. Add 'loud', 'notify', or 'violent' to send a notification to group members.
× /pinned: Gets the latest pinned message in the current Chat.
× /permapin <text>: Pin a custom message through the bot. This message can contain markdown, buttons, and all the other cool features.
× /unpin: Unpin the current pinned message. If used as a reply, unpins the replied-to message.
× /unpinall: Unpins all pinned messages.
× /antichannelpin <yes/no/on/off>: Don't let telegram auto-pin linked channels. If no arguments are given, show the current setting.
× /cleanlinked <yes/no/on/off>: Delete messages sent by the linked channel.

**Features:**

**Anti-Channel Pin:**
When enabled, the bot will automatically unpin any message that gets auto-pinned by a linked channel. This is useful when you want to maintain control over what gets pinned in your group.

**Clean Linked:**
When enabled, the bot will automatically delete any messages sent to the group from the linked channel. This keeps your group chat clean from cross-posted channel content.

**Permapin:**
Create custom pinned messages with:
- Text with HTML/Markdown formatting
- Inline buttons with URLs
- Media attachments (photos, documents, stickers, audio, video, voice notes, video notes)

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
| `/permapin` | Send a custom message (or a copy of the replied message) from the bot and pin it | ❌ |
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

- `/pin`, `/permapin`, `/unpin`, `/unpinall` — Require admin with **Pin Messages** permission
- `/antichannelpin`, `/cleanlinked` — Require admin permission (+ works via `/connect`); `/cleanlinked` also needs the bot to have **Delete Messages** permission, otherwise linked-channel posts are silently left in place
- `/pinned` — Any user can use, but **bot must be admin** to fetch pinned message

## Technical Notes

**Notes:**
- The `/unpinall` command shows a confirmation dialog before unpinning all messages
- Permapin supports text, photos, documents and GIFs, stickers, audio, voice messages, videos and video notes; other message types (polls, locations, contacts, dice, etc.) are rejected
- Anti-channel pin and clean linked settings are stored per-chat and persist across bot restarts

**Required Permissions:**
**Bot:** Must be admin with "Pin Messages" permission (and "Delete Messages" for `/cleanlinked`)
**User:** Most commands require admin with "Pin Messages" permission
