---
title: Rules Commands
description: Complete guide to Rules module commands and features
---

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

This module can be accessed using the following aliases:

- `rule`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/clearrulesbtn` | No description available | ❌ |
| `/clearrulesbutton` | No description available | ❌ |
| `/privaterules` | Enable/disable whether the rules should be sent in private. | ❌ |
| `/resetrulesbtn` | Same as above. | ❌ |
| `/resetrulesbutton` | Reset the text of the rules button to default. | ❌ |
| `/rules` | Check the current chat rules. | ✅ |
| `/rulesbtn` | Sets the text of the rules button. | ❌ |
| `/rulesbutton` | No description available | ❌ |
| `/resetrules` | Reset the group rules. Alias for /clearrules. | ❌ |
| `/clearrules` | Clear the group rules. | ❌ |
| `/setrules` | Set the rules for this chat. | ❌ |

## Usage Examples

### Basic Usage

```
/clearrulesbtn
/clearrulesbutton
/privaterules
```

For detailed command usage, refer to the commands table above.

## Required Permissions

Admin commands require admin permissions. `/rules` is available to all users.
