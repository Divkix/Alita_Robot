---
title: Formatting Commands
description: Complete guide to Formatting module commands and features
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# 📄 Formatting Commands

Alita supports a large number of formatting options to make your messages more expressive. In a private chat with the bot, `/markdownhelp` (or `/formatting`) opens the guide with three sections: Markdown Formatting, Fillings and Random Content. In a group, the command replies with a **Markdown Help** button that opens the guide in the bot's PM; if you send it as a reply, the bot posts that button as a reply to that message.

## Module Aliases

> These are help-menu module names, not command aliases.

This module can be accessed using the following aliases:

- `markdownhelp`
- `mdhelp`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/markdownhelp` | Open the formatting guide (in a group, replies with a button that opens it in PM) | ❌ |
| `/formatting` | Alias of `/markdownhelp` | ❌ |

## Markdown Formatting

- `` `code words` ``: monospace.
- `_italic words_`: italic.
- `*bold words*`: bold.
- `~strikethrough~`: strikethrough.
- `||spoiler||`: spoiler.
- ```` ```pre``` ````: preformatted; other formatting characters inside are ignored.
- `__underline__`: underline. Some clients show it as italic.
- `[hyperlink](example.com)`: a hyperlink.
- `[My Button](buttonurl://https://example.com)`: a button that opens the link. The URL must start with `http://` or `https://`, or no button is created.

Add `:same` to put a button on the same row as the previous one:

```
[button 1](buttonurl:https://example.com)
[button 2](buttonurl://https://example.com:same)
[button 3](buttonurl://https://example.com)
```

Buttons 1 and 2 share a row; button 3 goes underneath.

## Fillings

These placeholders work in welcome/goodbye messages, filters and notes:

- `{first}`: the user's first name.
- `{last}`: the user's last name.
- `{fullname}`: the user's full name.
- `{username}`: the user's @username, or a mention if they have none.
- `{mention}`: the same as `{username}`.
- `{id}`: the user's ID.
- `{chatname}`: the chat's name.
- `{count}`: the chat's member count.
- `{rules}`: adds a Rules button, only if the chat has rules set. `{rules:same}` puts it on the previous button's row; `{rules:up}` puts it first.

Notes only:

- `{protect}`: the content can't be forwarded or saved.
- `{preview}`: enables link previews.
- `{nonotif}`: sends the message without a notification.

## Random Content

`%%%` separates alternative replies in notes and filters, and the bot picks one at random. It does not work in welcome messages.

```
/filter "hey"
hello there {first}!
%%%
Ooooh, {first} how are you?
%%%
Sup? {first}
```

## Usage Examples

### Basic Usage

```
/formatting
/markdownhelp
```

For detailed command usage, refer to the commands table above.

## Required Permissions

Commands in this module are available to all users unless otherwise specified.
