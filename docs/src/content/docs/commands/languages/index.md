---
title: Languages Commands
description: Complete guide to Languages module commands and features
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# Languages Commands

Not able to change language of the bot?
Easily change by using this module!

Just type /lang and use inline keyboard to choose a language for yourself or your group.

You can help us bring bot to more languages by helping on [Crowdin](https://crowdin.com/project/alita_robot)

**How It Works**
- **Private Chats:** Any user can change their personal language
- **Group Chats:** Only admins can change the group language

Language preferences are cached for performance and stored separately for users and groups.


## Module Aliases

> These are help-menu module names, not command aliases.

This module can be accessed using the following aliases:

- `language`
- `lang`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/lang` | Change bot language for yourself or your group | No |

## Usage Examples

### Open language selection

```
/lang
```

The language selection uses an inline keyboard callback. After sending `/lang`, tap the button for your preferred language -- you do not need to type language codes.

## Required Permissions

- **Private chats:** Any user can change their own language
- **Group chats:** Only admins can change the group language
