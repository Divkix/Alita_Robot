---
title: Notes Commands
description: Complete guide to Notes module commands and features
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# 📝 Notes Commands

Save data for future users with notes!
Notes are great to save random tidbits of information; a phone number, a nice gif, a funny picture - anything!
*User commands:*
- /get <notename>: Get a note.
- #notename: Same as /get.
Admin commands:
- /save <notename> <note text>: Save a new note called "word". Replying to a message will save that message. Even works on media!
- /clear <notename>: Delete the associated note.
- /notes: List all notes in the current chat.
- /saved: Same as /notes.
- /clearall: Delete ALL notes in a chat. This cannot be undone.
- /privatenotes: Whether or not to send notes in PM. Will send a message with a button which users can click to get the note in PM.

**Features:**
- **Text &amp; Media Support**: Save text messages, photos, videos, documents, and more
- **Custom Buttons**: Add inline keyboard buttons to notes
- **Formatting Options**: Control note visibility and behavior
- **Admin-Only Notes**: Restrict certain notes to administrators only
- **Private Notes**: Send notes privately to users via DM
- **Web Preview Control**: Enable or disable link previews
- **Protected Content**: Prevent note content from being forwarded

**Note Formatting Options:**
When saving a note, you can use special tags to control its behavior:
- `{private}` - Note will only be sent via DM, never in group
- `{noprivate}` - Note will only be sent in group, never via DM
- `{admin}` - Note can only be accessed by admins
- `{protect}` - Content cannot be forwarded
- `{nonotif}` - Send without notification sound
- `{nopreview}` - Disable link preview

**Private Notes Mode:**
When private notes are enabled (`/privatenotes on`), all notes are sent to users via DM instead of in the group. This keeps the group chat clean.

Individual notes can override this behavior:
- `{private}` - Always send privately
- `{noprivate}` - Always send in group

**Raw Note (No Formatting):**
To see a note's raw content without formatting (for editing):
`/get notename noformat`


## Module Aliases

> These are help-menu module names, not command aliases.

This module can be accessed using the following aliases:

- `note`
- `notes`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/addnote` | Save a note with a keyword | ❌ |
| `/clear` | Remove a saved note | ❌ |
| `/clearall` | Remove all notes from the chat | ❌ |
| `/get` | Retrieve a saved note by keyword | ✅ |
| `/notes` | List all saved notes | ✅ |
| `/privnote` | Toggle sending notes in private messages | ❌ |
| `/privatenotes` | Alias of `/privnote` | ❌ |
| `/rmnote` | Remove a saved note | ❌ |
| `/save` | Save a note with a keyword | ❌ |
| `/saved` | List all saved notes | ❌ |

## Usage Examples

### Basic Usage

```
/addnote
/clear
/clearall
```

For detailed command usage, refer to the commands table above.

## Required Permissions

**Required Permissions:**
- Save/Clear notes: Change Group Info
- View admin notes: Admin status
- Clear all notes: Chat Owner
- Get regular notes: Any user
