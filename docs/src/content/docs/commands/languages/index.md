---
title: Languages Commands
description: Complete guide to Languages module commands and features
---

# Languages Commands

The Languages module allows users and group administrators to change the bot's interface language. The bot supports multiple languages and provides an intuitive inline keyboard for language selection.

## How It Works

### Private Chats
- Any user can change the bot's language for their private conversations
- Use `/lang` to see current language and available options
- Click any language button to switch immediately

### Group Chats
- Only **group administrators** can change the group's language
- Non-admins will receive an error message when attempting to change
- The language setting affects all bot messages in that group

## Module Aliases

This module can be accessed using the following aliases:

- `language`
- `lang`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/lang` | Display current language and show language selection keyboard | No |

## Usage Examples

### Check Current Language
```
/lang
```
Shows your current language (in private) or the group's language (in groups) along with a keyboard to select a different language.

### Changing Language
1. Type `/lang` in any chat
2. An inline keyboard appears with available languages
3. Click your preferred language
4. The bot confirms the language change

## Required Permissions

| Context | Required Permission |
|---------|---------------------|
| Private Chat | None (all users) |
| Group Chat | Administrator |

## Callback Data Format

The language selection buttons use the callback format:
```
change_language.<language_code>
```

For example: `change_language.en` for English, `change_language.es` for Spanish.

## Supported Languages

The bot supports the following languages (configured via `ENABLED_LOCALES` environment variable):

| Code | Language |
|------|----------|
| `en` | English |
| `es` | Spanish |

Want to see your language here? Help us translate on [Crowdin](https://crowdin.com/project/alita_robot)!

## Technical Details

- Language preferences are cached with a 1-hour TTL for performance
- User language preferences are stored separately from group preferences
- The bot automatically falls back to English if a translation key is missing
- Invalid callback data is safely handled with appropriate error messages
