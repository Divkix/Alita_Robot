---
title: Notes Commands
description: Complete guide to Notes module commands and features
---

# 📝 Notes Commands

Save data for future users with notes!
Notes are great to save random tidbits of information; a phone number, a nice gif, a funny picture - anything!

## Features

- **Text & Media Support**: Save text messages, photos, videos, documents, and more
- **Custom Buttons**: Add inline keyboard buttons to notes
- **Formatting Options**: Control note visibility and behavior
- **Admin-Only Notes**: Restrict certain notes to administrators only
- **Private Notes**: Send notes privately to users via DM
- **Web Preview Control**: Enable or disable link previews
- **Protected Content**: Prevent note content from being forwarded

## User Commands

| Command | Description |
|---------|-------------|
| `/get <notename>` | Get a note by name |
| `#notename` | Same as `/get` - quick way to fetch notes |

## Admin Commands

| Command | Description |
|---------|-------------|
| `/save <notename> <content>` | Save a new note. Reply to a message to save that message as a note |
| `/addnote <notename> <content>` | Alias for `/save` |
| `/clear <notename>` | Delete a note |
| `/rmnote <notename>` | Alias for `/clear` |
| `/notes` | List all notes in the current chat |
| `/saved` | Alias for `/notes` |
| `/clearall` | Delete ALL notes in a chat (Owner only) |
| `/privatenotes <on/off>` | Toggle private notes mode |
| `/privnote <on/off>` | Alias for `/privatenotes` |

## Note Formatting Options

When saving a note, you can use special tags to control its behavior:

| Tag | Description |
|-----|-------------|
| `{private}` | Note will only be sent via DM, never in group |
| `{noprivate}` | Note will only be sent in group, never via DM |
| `{admin}` | Note can only be accessed by admins |
| `{protect}` | Content cannot be forwarded |
| `{nonotif}` | Send without notification sound |
| `{nopreview}` | Disable link preview |

### Example Usage

```
/save rules Welcome to the group!

Please follow our rules:
1. Be respectful
2. No spam

{admin}
```

This saves an admin-only note called "rules".

## Module Aliases

This module can be accessed using the following aliases:

- `note`
- `notes`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/addnote` | Save a new note (alias for /save) | ❌ |
| `/clear` | Delete the associated note | ❌ |
| `/clearall` | Delete ALL notes in a chat. This cannot be undone | ❌ |
| `/get` | Get a note | ✅ |
| `/notes` | List all notes in the current chat | ✅ |
| `/rmnote` | Delete a note (alias for /clear) | ❌ |
| `/save` | Save a new note called "word". Replying to a message will save that message. Even works on media! | ❌ |
| `/saved` | Same as /notes | ❌ |

## Usage Examples

### Basic Text Note

```
/save welcome Hello! Welcome to our group.
```

### Note with Reply

Reply to any message and use:
```
/save faq
```

This saves the replied message as a note called "faq".

### Note with Buttons

```
/save links Check out these links!
[Website](buttonurl://example.com)
[Support](buttonurl://t.me/support_group)
```

### Admin-Only Note

```
/save secretinfo This information is for admins only.
{admin}
```

### Private Note

```
/save rules Please read these rules carefully.
{private}
```

### Getting Notes

```
/get welcome
```
Or simply:
```
#welcome
```

### Raw Note (No Formatting)

To see a note's raw content without formatting (for editing):
```
/get notename noformat
```

## Private Notes Mode

When private notes are enabled (`/privatenotes on`), all notes are sent to users via DM instead of in the group. This keeps the group chat clean.

Individual notes can override this behavior:
- `{private}` - Always send privately
- `{noprivate}` - Always send in group

## Required Permissions

| Action | Required Permission |
|--------|---------------------|
| Save/Clear notes | Change Group Info |
| View admin notes | Admin status |
| Clear all notes | Chat Owner |
| Get regular notes | Any user |

## Database Fields

Notes support the following properties:

| Property | Description | Default |
|----------|-------------|---------|
| `admin_only` | Only admins can view | `false` |
| `private_only` | Only sent via DM | `false` |
| `group_only` | Only sent in group | `false` |
| `web_preview` | Show link previews | `true` |
| `is_protected` | Prevent forwarding | `false` |
| `no_notif` | Silent notification | `false` |
