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
× /resetrules: Reset the chat rules to default
× /rulesbtn `<custom text>`: Sets the text of the rules button.
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
- User commands: Available to all users
- Admin commands: Require admin permissions in the chat


## Module Aliases

> These are help-menu module names, not command aliases.

This module can be accessed using the following aliases:

- `rule`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/resetrules` | Clear all group rules | ❌ |
| `/clearrules` | Alias of `/resetrules` | ❌ |
| `/clearrulesbtn` | Toggle button for clearing rules | ❌ |
| `/clearrulesbutton` | Toggle button for clearing rules | ❌ |
| `/privaterules` | Toggle sending rules in private messages | ❌ |
| `/resetrulesbtn` | Toggle button for resetting rules | ❌ |
| `/resetrulesbutton` | Toggle button for resetting rules | ❌ |
| `/rules` | Show group rules | ✅ |
| `/rulesbtn` | Toggle rules button on welcome message | ❌ |
| `/rulesbutton` | Toggle rules button on welcome message | ❌ |
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

Commands in this module are available to all users unless otherwise specified.
