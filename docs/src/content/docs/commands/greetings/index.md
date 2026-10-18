---
title: Greetings Commands
description: Complete guide to Greetings module commands and features
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# 👋 Greetings Commands

Welcome new members to your groups or say Goodbye after they leave!

*Admin Commands:*
× /setwelcome `<reply/text>`: Sets welcome text for group.
× /welcome `<yes/no/on/off/noformat>`: With no argument, shows the welcome settings and a preview of the welcome message. on/off enables or disables welcome; noformat shows the preview with placeholders unfilled and button codes instead of buttons.
× /resetwelcome: Resets the welcome message to default.
× /setgoodbye `<reply/text>`: Sets goodbye text for group.
× /goodbye `<yes/no/on/off/noformat>`: With no argument, shows the goodbye settings and a preview of the goodbye message. on/off enables or disables goodbye; noformat shows the preview with placeholders unfilled and button codes instead of buttons.
× /resetgoodbye: Resets the goodbye message to default.
× /cleanservice `<yes/no/on/off>`: Delete join and leave service messages, such as the 'x joined the group' notification.
× /cleanwelcome `<yes/no/on/off>`: Delete the old welcome message, whenever a new member joins.
× /cleangoodbye `<yes/no/on/off>`: Delete the old goodbye message, whenever a member leaves.
× /autoapprove `<yes/no/on/off>`: Automatically approve chat join requests.

**Captcha Integration**
When Captcha module is enabled:
1. New members are muted upon joining
2. Captcha challenge sent instead of welcome
3. After verification, welcome message is sent
4. Failed verification applies captcha action

**Join Requests**
When the group requires admin approval to join and `/autoapprove` is off, the bot posts each join request in the group with the user's name and ID and **Approve**, **Decline** and **Ban** buttons. Only group admins can use them:
- Approve / Decline: both the admin and the bot need the **Invite Users** permission.
- Ban: both the admin and the bot need **Invite Users** and **Ban Users**. Ban bans the user and declines the request.

After an admin acts, the message is edited to show the result (or that the request was already handled or withdrawn). With Redis available, a repeat request from the same user within 5 minutes is not posted again. When `/autoapprove` is on, requests are approved immediately and no message is posted.


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

- `/setwelcome`, `/setgoodbye` — Requires **Change Group Info** admin permission (`CanUserChangeInfo`)
- `/resetwelcome`, `/resetgoodbye` — Requires **Change Group Info** admin permission (`CanUserChangeInfo`)
- `/welcome on/off`, `/goodbye on/off` — Requires **admin** permission
- `/welcome`, `/goodbye` (no args, view settings) — Available to any admin
- `/cleanwelcome`, `/cleangoodbye` — Requires **Change Group Info** admin permission
- `/cleanservice`, `/autoapprove` — Requires **Change Group Info** admin permission, and the bot must be an admin in the group
- Join-request buttons — Approve/Decline need **Invite Users** (admin and bot); Ban also needs **Ban Users** (admin and bot)

**Preview without placeholders:** Append `noformat` to `/welcome` or `/goodbye`
(e.g., `/welcome noformat`) to preview the stored message with `{placeholders}` left
unfilled and buttons shown as `[text](buttonurl://…)` codes instead of a keyboard.
Text formatting (bold, links, etc.) still renders. Admin only, like the plain view.
