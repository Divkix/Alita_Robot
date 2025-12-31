---
title: Formatting Commands
description: Complete guide to Formatting module commands and features
---

# 📄 Formatting Commands

Alita supports a large number of formatting options to make your messages more expressive. This guide covers markdown syntax, fillings (dynamic placeholders), and random content.

## Commands

| Command | Description |
|---------|-------------|
| `/markdownhelp` | Shows formatting help menu with options for markdown, fillings, and random content |
| `/formatting` | Alias for `/markdownhelp` |

## Module Aliases

This module can be accessed using the following aliases:

- `markdownhelp`
- `formatting`

## Markdown Formatting

You can format your message using **bold**, *italics*, __underline__, and much more.

### Supported Markdown

| Syntax | Result | Example |
|--------|--------|---------|
| `` `code` `` | Monospace | `code words` |
| `_text_` | Italic | _italic words_ |
| `*text*` | Bold | **bold words** |
| `~text~` | Strikethrough | ~~strikethrough~~ |
| `\|\|text\|\|` | Spoiler | (hidden until clicked) |
| ` ```text``` ` | Code block | Preserves formatting inside |
| `__text__` | Underline | (note: some clients interpret as italic) |
| `[text](url)` | Hyperlink | [example](https://example.com) |

### Button Syntax

Create inline buttons using this format:

```
[Button Text](buttonurl://example.com)
```

To place buttons on the same row, add `:same`:

```
[Button 1](buttonurl://example.com)
[Button 2](buttonurl://example.com:same)
[Button 3](buttonurl://example.com)
```

This displays Button 1 and Button 2 on the same row, with Button 3 underneath.

## Fillings (Dynamic Placeholders)

Customize message content with contextual data. These work in notes, filters, welcome/goodbye messages.

| Placeholder | Description |
|-------------|-------------|
| `{first}` | User's first name |
| `{last}` | User's last name |
| `{fullname}` | User's full name |
| `{username}` | User's @username (or mention if none) |
| `{mention}` | Mentions the user with their first name |
| `{id}` | User's Telegram ID |
| `{chatname}` | Chat's title |
| `{count}` | Chat's member count |
| `{rules}` | Adds a Rules button to the message |
| `{protect}` | Protects content from being shared/forwarded |
| `{preview}` | Enables link previews |
| `{nonotif}` | Sends without notification |

## Random Content

Randomize message content using the `%%%` separator:

```
Hello there!
%%%
Greetings!
%%%
Welcome aboard!
```

Each time the message is triggered, one of the three options is randomly selected. This works in filters and notes.

## Required Permissions

Commands in this module are available to all users. The formatting features themselves require no special permissions.
