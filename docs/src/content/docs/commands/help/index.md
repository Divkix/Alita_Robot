---
title: Help Commands
description: Complete guide to Help module commands and features
---

# 📦 Help Commands

The main entry point and navigation system for Alita Robot.
### Navigation Commands:
- `/start`: Show the welcome message with navigation menu.
- `/help`: Show help menu with module list; use /help <module> for details.
- `/about`: Show bot information, version, and useful links.
- `/donate`: Show donation information for supporting the project.

**Navigation System:**
The Help module provides the bot's main entry points and navigation. When a user sends `/start` in a private chat, the bot displays a welcome message with an inline keyboard for navigating to different modules.

The `/help` command shows a module list. Clicking a module button displays that module's commands and usage information. When `/help` is used in a group chat, the bot redirects the user to a private message for detailed help to avoid cluttering the group.

`/about` shows bot information, version, and links, while `/donate` shows donation information for supporting the project.


## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/about` | Show bot information, version, and useful links. | ❌ |
| `/donate` | Show donation information for supporting the project. | ❌ |
| `/help` | Show help menu with module list; use /help <module> for details. | ❌ |
| `/start` | Show the welcome message with navigation menu. | ❌ |

## Usage Examples

### Basic Usage

```
/about
/donate
/help
```

For detailed command usage, refer to the commands table above.

## Required Permissions

Commands in this module are available to all users unless otherwise specified.

