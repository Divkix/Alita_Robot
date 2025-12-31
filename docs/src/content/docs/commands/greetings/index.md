---
title: Greetings Commands
description: Complete guide to Greetings module commands and features
---

# 👋 Greetings Commands

Welcome new members to your groups or say Goodbye after they leave!

*Admin Commands:*
× /setwelcome `<reply/text>`: Sets welcome text for group.
× /welcome `<yes/no/on/off>`: Enables or Disables welcome setting for group.
× /resetwelcome: Resets the welcome message to default.
× /setgoodbye `<reply/text>`: Sets goodbye text for group.
× /goodbye `<yes/no/on/off>`: Enables or Disables goodbye setting for group.
× /resetgoodbye: Resets the goodbye message to default.
× /cleanservice `<yes/no/on/off>`: Delete all service messages such as 'x joined the group' notification.
× /cleanwelcome `<yes/no/on/off>`: Delete the old welcome message, whenever a new member joins.
× /autoapprove `<yes/no/on/off>`: Automatically approve all new members.

## Module Aliases

This module can be accessed using the following aliases:

- `welcome`
- `goodbye`
- `greeting`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/autoapprove` | Automatically approve all new members. | ❌ |
| `/cleangoodbye` | No description available | ❌ |
| `/cleanservice` | Delete all service messages such as 'x joined the group' notification. | ❌ |
| `/cleanwelcome` | Delete the old welcome message, whenever a new member joins. | ❌ |
| `/goodbye` | Enables or Disables goodbye setting for group. | ❌ |
| `/resetgoodbye` | Resets the goodbye message to default. | ❌ |
| `/resetwelcome` | Resets the welcome message to default. | ❌ |
| `/setgoodbye` | Sets goodbye text for group. | ❌ |
| `/setwelcome` | Sets welcome text for group. | ❌ |
| `/welcome` | Enables or Disables welcome setting for group. | ❌ |

## Usage Examples

### Basic Usage

```
/autoapprove
/cleangoodbye
/cleanservice
```

For detailed command usage, refer to the commands table above.

## Captcha Integration

When the [Captcha module](/commands/captcha) is enabled in a group:

1. New members are automatically muted upon joining
2. A captcha challenge is sent instead of the welcome message
3. After successful verification, the configured welcome message is sent
4. If verification fails, the captcha failure action is applied

This provides spam protection while maintaining a welcoming experience.

## Required Permissions

Commands in this module require admin permissions with the ability to change chat info. The bot also needs admin permissions with:

- Delete messages (for clean service/welcome/goodbye features)
- Restrict members (when captcha is enabled)
