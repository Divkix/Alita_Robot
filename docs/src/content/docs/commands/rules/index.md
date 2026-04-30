---
title: Rules Commands
description: Complete guide to Rules module commands and features
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# 📋 Rules Commands

Every chat works with different rules; this module will help make those rules clearer!
*User commands*:
× /rules: Check the current chat rules.
*Admin commands*:
× /setrules `<text>`: Set the rules for this chat.
× /privaterules `<yes/no/on/off>`: Enable/disable whether the rules should be sent in private.
× /resetrules: Clear all chat rules (sets empty string).
× /rulesbtn `<custom text>`: Sets the custom text for the rules button in private rules link. (max 30 chars)
× /resetrulesbutton: Reset the text of the rules button to default.
× /resetrulesbtn: Same as above.

**Features:**

**Private Rules:**
Enable private rules (`/privaterules on`) to send rules via PM instead of in the group. This keeps the group chat clean.

**Custom Rules Button:**
Set a custom button text (max 30 characters):
`/rulesbtn View Rules`

Reset to default:
`/resetrulesbtn`

**Setting Rules:**
You can set rules by providing text directly or by replying to a message:
`/setrules Please be respectful to all members.`

Or reply to a message:
`/setrules`

**Required Permissions:**
- `/rules`: Available to all users (disableable)
- All other commands (`/setrules`, `/resetrules`, `/privaterules`, `/rulesbtn`, `/resetrulesbtn`, etc.): Require admin permissions in the connected chat via `IsUserConnected`


## Module Aliases

> These are help-menu module names, not command aliases.

This module can be accessed using the following aliases:

- `rule`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/resetrules` | Clear all group rules (sets empty string) | ❌ |
| `/clearrules` | Alias of `/resetrules` — clears all group rules | ❌ |
| `/clearrulesbtn` | Alias of `/resetrulesbtn` — reset button text to default | ❌ |
| `/clearrulesbutton` | Alias of `/resetrulesbtn` — reset button text to default | ❌ |
| `/privaterules` | Toggle sending rules in private messages (admin required) | ❌ |
| `/resetrulesbtn` | Reset rules button text to default | ❌ |
| `/resetrulesbutton` | Alias of `/resetrulesbtn` — reset button text to default | ❌ |
| `/rules` | Show group rules | ✅ |
| `/rulesbtn` | Set custom button text for private rules link | ❌ |
| `/rulesbutton` | Alias of `/rulesbtn` — set custom button text for private rules link | ❌ |
| `/setrules` | Set group rules | ❌ |

## Usage Examples

### Basic Usage

```
/clearrules
/clearrulesbtn
/clearrulesbutton
```

For detailed command usage, refer to the commands table above.

## Required Permissions

| Command | Permissions |
|---------|-------------|
| `/rules` | Available to all users (disableable) |
| `/setrules` | Admin only — requires `IsUserConnected` (bot admin + user admin) |
| `/resetrules`, `/clearrules` | Admin only — requires `IsUserConnected` |
| `/privaterules` | Admin only — requires `IsUserConnected`, user must be admin in connected chat |
| `/rulesbtn`, `/rulesbutton` | Admin only — requires `IsUserConnected`, user must be admin in connected chat |
| `/resetrulesbtn`, `/resetrulesbutton`, `/clearrulesbtn`, `/clearrulesbutton` | Admin only — requires `IsUserConnected` |
