---
title: Misc Commands
description: Complete guide to Misc module commands and features
---

# Misc Commands

A collection of utility commands for general use.

:::note[Mostly Available to Everyone]
Most commands in this module are available to all users. `/tell` requires admin permissions and group chat.
:::

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/info` | Get your user info, which can be used as a reply or by passing a User ID or Username. | Yes |
| `/id` | Get the current group id. If used by replying to a message, get that user's id. | Yes |
| `/ping` | Check bot response latency. | Yes |
| `/tr` | Translate the message. | Yes |
| `/stat` | Gets the count of the total number of messages in the chat. | Yes |
| `/removebotkeyboard` | Removes the stuck bot keyboard from your chat. | No |
| `/tell` | Echo a message as a reply (admin only, group only). Reply to a message with `/tell <text>`. | No |

## Usage Examples

```text
/info @username          # Get info about a user
/id                      # Get current chat ID
/ping                    # Check bot latency
/tr en Hola mundo        # Translate to English
/stat                    # Message count for this chat
/removebotkeyboard       # Fix stuck keyboard
```

:::tip[Translation]
Use `/tr <lang code> <text>` or reply to a message with `/tr <lang code>` to translate it. Language codes follow ISO 639-1 format (e.g., `en`, `es`, `fr`, `hi`).
:::

## Module Aliases

This module can be accessed using the following aliases:
`extra`, `extras`

## Required Permissions

Most commands are available to all users. `/tell` requires admin permissions and works only in group chats.
