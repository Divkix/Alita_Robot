---
title: Greetings Commands
description: Complete guide to Greetings module commands and features
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

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

> These are help-menu module names, not command aliases.

This module can be accessed using the following aliases:

- `welcome`
- `goodbye`
- `greeting`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/autoapprove` | Toggle auto-approve for join requests | ❌ |
| `/cleangoodbye` | Toggle deletion of previous goodbye messages | ❌ |
| `/cleanservice` | Toggle deletion of service messages | ❌ |
| `/cleanwelcome` | Toggle deletion of previous welcome messages | ❌ |
| `/goodbye` | Show current goodbye message settings | ❌ |
| `/resetgoodbye` | Reset goodbye message to default | ❌ |
| `/resetwelcome` | Reset welcome message to default | ❌ |
| `/setgoodbye` | Set a custom goodbye message | ❌ |
| `/setwelcome` | Set a custom welcome message | ❌ |
| `/welcome` | Show current welcome message settings | ❌ |

## Usage Examples

### Basic Usage

```
/autoapprove
/cleangoodbye
/cleanservice
```

For detailed command usage, refer to the commands table above.

## Required Permissions

Commands in this module are available to all users unless otherwise specified.
