---
title: Filters Commands
description: Complete guide to Filters module commands and features
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# 🔍 Filters Commands

Filters are case insensitive; every time someone says your trigger words, Alita will reply something else! This can be used to create your commands, if desired.

Commands:
- /filter <trigger> <reply>: Every time someone says trigger, the bot will reply with sentence. For multiple word filters, quote the trigger.
- /filters: List all chat filters.
- /stop <trigger>: Stop the bot from replying to trigger.
- /stopall: Stop ALL filters in the current chat. This action cannot be undone.

Examples:
- Set a filter:
-> /filter hello Hello there! How are you?
- Set a multiword filter:
-> /filter "hello friend" Hello back! Long time no see!
- To save a file, image, gif, or any other attachment, simply reply to the file with:
-> /filter trigger

**Advanced Features:**

**Random Responses:**
Use `%%%` to separate multiple responses. One will be picked randomly:
`/filter hello Hi there!%%%Hello!%%%Hey, how are you?`

**Media Filters:**
Reply to any media (photo, video, document, sticker, etc.) with `/filter trigger` to create a filter that sends that media.

**Noformat Mode:**
Admins can view the raw filter content (including formatting codes) by adding `noformat` after the trigger:
`hello noformat`
This is useful for debugging filters or seeing button configurations.

**Filter Buttons:**
You can add inline buttons to your filters:
`/filter hello Hello! Check out these links:
[Button 1](buttonurl:https://example.com)
[Button 2](buttonurl:https://example2.com)`

**Filters of a group from PM:**
In PM, `/filters` takes the group as the first argument: its ID, its @username, or the title of a group where you are an admin (case does not matter; quote a title with spaces):
`/filters -1001234567890`
`/filters @mygroup`
`/filters "My Group"`

This works without `/connect` and, when you are connected, lists the named group's filters instead of the connected one's; the connection stays as it is. The group passes the same checks as `/connect`. A title is looked up only among the connected group and the 50 most recently active groups where you have written; name any other group by its @username or ID. When two of your groups share a title, the bot lists both with their IDs and asks for the @username or ID. Without an argument and without a connection, the bot answers with the command's help, an example and the groups where you are an admin. In a group the argument is ignored.


## Module Aliases

> These are help-menu module names, not command aliases.

This module can be accessed using the following aliases:

- `filter`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/addfilter` | Add a message filter trigger | ❌ |
| `/filter` | Add a message filter trigger | ❌ |
| `/filters` | List all active filters | ✅ |
| `/removefilter` | Remove a filter trigger | ❌ |
| `/rmfilter` | Remove a filter trigger | ❌ |
| `/stop` | Remove a filter trigger | ❌ |
| `/stopall` | Remove all filters from the chat | ❌ |

## Usage Examples

### Basic Usage

```
/addfilter
/filter
/filters
```

For detailed command usage, refer to the commands table above.

## Required Permissions

- `/filter`, `/addfilter` — Requires **Change Group Info** admin permission (`CanUserChangeInfo`)
- `/stop`, `/rmfilter`, `/removefilter` — Requires **Change Group Info** admin permission (`CanUserChangeInfo`)
- `/stopall` — Requires **chat owner** (creator)
- `/filters` — Available to all users (disableable)

**Important:** When re-adding an existing filter keyword, the bot shows an inline
confirmation dialog (Yes/No buttons) asking the admin to confirm overwrite. The
confirmation expires after 5 minutes. Each chat can have up to **150 filters**;
exceeding this limit returns an error.
