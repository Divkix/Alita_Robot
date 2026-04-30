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

Commands in this module are available to all users.

## Configuration Wizard

Accessible from the `/about` keyboard via the **Configuration** button
(`configuration` callback prefix), the wizard walks new users through
setting up Alita in three steps:

1. **Step 1** — Add Alita to a group (deep-link button + "Done").
2. **Step 2** — Promote Alita to admin with recommended permissions
   (instructions + "Done").
3. **Step 3** — Continue to Help module to explore commands.

The wizard is PM-only and uses English regardless of user language setting.

## Deep-Link Support in `/start`

When `/start` is used with a parameter in private chat, the bot resolves it
as a deep link:

| Prefix | Example | Behavior |
|--------|---------|----------|
| `help_` | `start=help_admin` | Opens help for a specific module |
| `connect_` | `start=connect_-1001234` | Connects user to that group in PM |
| `rules_` | `start=rules_-1001234` | Shows the group's rules |
| `notes_` | `start=notes_-1001234` | Lists all notes in the group (admin-only notes hidden from non-admins) |
| `note_` | `start=note_-1001234_welcome` | Shows a specific note (respects admin-only flag) |

