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
- /notes: List all notes in the current chat.
- /saved: Same as /notes.
Admin commands:
- /save /addnote <notename> <note text>: Save a new note called "word". Replying to a message will save that message. Even works on media!
- /clear /rmnote <notename>: Delete the associated note.
- /clearall: Delete ALL notes in a chat (chat owner only). This cannot be undone.
- /privatenotes /privnote <on/off>: Whether or not to send notes in PM. Will send a message with a button which users can click to get the note in PM. Without an argument, shows the current setting.

**Features:**
- **Text &amp; Media Support**: Save text messages, photos, videos, documents, and more
- **Custom Buttons**: Add inline keyboard buttons to notes
- **Formatting Options**: Control note visibility and behavior
- **Admin-Only Notes**: Restrict certain notes to administrators only
- **Private Notes**: Send notes privately to users via DM
- **Web Preview Control**: Link previews are off by default; add `{preview}` to turn them on
- **Protected Content**: Prevent note content from being forwarded

**Note Formatting Options:**
When saving a note, you can use special tags to control its behavior:
- `{private}` - Note will only be sent via DM, never in group
- `{noprivate}` - When fetched in a group, the note is sent in the group even if private notes are on (fetching it from PM still delivers it in PM)
- `{admin}` - Note can only be accessed by admins
- `{protect}` - Content cannot be forwarded
- `{nonotif}` - Send without notification sound
- `{preview}` - Show link previews (they are off by default)

**Private Notes Mode:**
When private notes are enabled (`/privatenotes on`), all notes are sent to users via DM instead of in the group. This keeps the group chat clean.

Individual notes can override this behavior:
- `{private}` - Always send privately
- `{noprivate}` - Always send in group

**Raw Note (No Formatting):**
Admins can see a note's raw content without formatting (for editing):
`/get notename noformat` or `#notename noformat`

**Notes of a group from PM:**
In PM, `/notes` and `/saved` take the group as the first argument: its ID, its @username, or the title of a group where you are an admin (case does not matter; quote a title with spaces):
`/notes -1001234567890`
`/notes @mygroup`
`/notes "My Group"`

This works without `/connect` and, when you are connected, lists the named group's notes instead of the connected one's; the connection stays as it is. The group passes the same checks as `/connect`. When two of your groups share a title, the bot lists both with their IDs and asks for the @username or ID. Without an argument and without a connection, the bot answers with the command's help, an example and the groups where you are an admin. In a group the argument is ignored.


## Module Aliases

> These are help-menu module names, not command aliases.

This module can be accessed using the following aliases:

- `note`
- `notes`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/addnote` | Alias of `/save` | ❌ |
| `/clear` | Remove a saved note | ❌ |
| `/clearall` | Remove all notes from the chat | ❌ |
| `/get` | Retrieve a saved note by keyword | ✅ |
| `/notes` | List all saved notes | ✅ |
| `/privnote` | Toggle sending notes in private messages | ❌ |
| `/privatenotes` | Alias of `/privnote` | ❌ |
| `/rmnote` | Alias of `/clear` | ❌ |
| `/save` | Save a note with a keyword | ❌ |
| `/saved` | Alias of `/notes` | ❌ |

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
- Bot: must be an admin for /save, /addnote, /clear, /rmnote, /notes and /saved (/get and #notename work without it)
