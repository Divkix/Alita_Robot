---
title: Help Commands
description: Complete guide to help, navigation, and bot information commands
---

# Help Commands

The help module provides the bot's main entry points and navigation system. When a user sends `/start` in a private chat, the bot displays a welcome message with an inline keyboard for navigating to different modules.

The `/help` command shows a module list. Clicking a module button displays that module's commands and usage information. When `/help` is used in a group chat, the bot redirects the user to a private message for detailed help to avoid cluttering the group.

`/about` shows bot information and links, while `/donate` shows donation information for supporting the project.

## Module Aliases

> These are help-menu module names, not command aliases.

This module has no help-menu aliases. These commands are always available.

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/about` | Show bot information and links | No |
| `/donate` | Show donation information | No |
| `/help` | Show help menu with module list | No |
| `/start` | Show welcome message with navigation menu | No |

## Usage Examples

### Start the bot

```
/start
```

### View help menu

```
/help
```

### Get help for a specific module

```
/help admin
```

### View bot information

```
/about
```

## Required Permissions

All commands in this module are available to everyone. No admin or owner permissions required.
