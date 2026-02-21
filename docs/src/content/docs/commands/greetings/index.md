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

**Captcha Integration**
When Captcha module is enabled:
1. New members are muted upon joining
2. Captcha challenge sent instead of welcome
3. After verification, welcome message is sent
4. Failed verification applies captcha action


## Module Aliases

This module can be accessed using the following aliases:

- `welcome`
- `goodbye`
- `greeting`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/autoapprove` | Automatically approve all new members. | ❌ |
| `/cleangoodbye` | Delete the old goodbye message when a member leaves. | ❌ |
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

## Required Permissions

Most commands require admin with 'Change Group Info' permission.
