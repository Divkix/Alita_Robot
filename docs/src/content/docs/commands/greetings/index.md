---
title: Greetings Commands
description: Complete guide to Greetings module commands and features
---

# Greetings Commands

Welcome new members to your groups or say goodbye after they leave!

:::caution[Admin Permissions Required]
All commands in this module require admin with "Change Group Info" permission.
:::

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/welcome` | Enables or disables welcome setting for group. | No |
| `/setwelcome` | Sets welcome text for group. | No |
| `/resetwelcome` | Resets the welcome message to default. | No |
| `/goodbye` | Enables or disables goodbye setting for group. | No |
| `/setgoodbye` | Sets goodbye text for group. | No |
| `/resetgoodbye` | Resets the goodbye message to default. | No |
| `/cleanservice` | Delete all service messages such as "x joined the group" notification. | No |
| `/cleanwelcome` | Delete the old welcome message whenever a new member joins. | No |
| `/cleangoodbye` | Delete the old goodbye message when a member leaves. | No |
| `/autoapprove` | Automatically approve all new members. | No |

## Usage Examples

```text
/welcome on                          # Enable welcome messages
/setwelcome Hello, {first}!          # Set custom welcome text
/goodbye on                          # Enable goodbye messages
/setgoodbye Goodbye, {first}!        # Set custom goodbye text
/cleanservice on                     # Delete join/leave service messages
/cleanwelcome on                     # Delete old welcome messages
/autoapprove on                      # Auto-approve new members
```

:::tip[Dynamic Variables]
Use variables like `{first}`, `{last}`, `{fullname}`, `{username}`, `{id}`, `{chatname}`, and `{count}` in your welcome/goodbye messages to personalize them.
:::

:::note[Captcha Integration]
When the Captcha module is enabled:
1. New members are muted upon joining
2. Captcha challenge sent instead of welcome
3. After verification, welcome message is sent
4. Failed verification applies captcha action
:::

## Module Aliases

This module can be accessed using the following aliases:
`welcome`, `goodbye`, `greeting`

## Required Permissions

**Bot Permissions Required:**
- Delete messages (for clean service/welcome/goodbye)
- Restrict members (for auto-approve)
