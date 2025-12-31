---
title: Rules Commands
description: Complete guide to Rules module commands and features
---

# 📋 Rules Commands

Every chat works with different rules; this module will help make those rules clearer!

## User Commands

| Command | Description |
|---------|-------------|
| `/rules` | Check the current chat rules. |

## Admin Commands

| Command | Description |
|---------|-------------|
| `/setrules <text>` | Set the rules for this chat. Reply to a message or provide text. |
| `/privaterules <yes/no/on/off>` | Enable/disable whether the rules should be sent in private. |
| `/clearrules` | Clear all rules from the chat. |
| `/resetrules` | Same as `/clearrules`. |
| `/rulesbtn <custom text>` | Sets the text of the rules button (max 30 characters). |
| `/rulesbutton <custom text>` | Same as `/rulesbtn`. |
| `/resetrulesbtn` | Reset the text of the rules button to default. |
| `/resetrulesbutton` | Same as `/resetrulesbtn`. |
| `/clearrulesbtn` | Same as `/resetrulesbtn`. |
| `/clearrulesbutton` | Same as `/resetrulesbtn`. |

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/rules` | Check the current chat rules. | ✅ |
| `/setrules` | Set the rules for this chat. | ❌ |
| `/privaterules` | Enable/disable whether the rules should be sent in private. | ❌ |
| `/clearrules` | Clear all rules from the chat. | ❌ |
| `/resetrules` | Same as `/clearrules`. | ❌ |
| `/rulesbtn` | Sets the text of the rules button. | ❌ |
| `/rulesbutton` | Same as `/rulesbtn`. | ❌ |
| `/resetrulesbtn` | Reset the text of the rules button to default. | ❌ |
| `/resetrulesbutton` | Same as `/resetrulesbtn`. | ❌ |
| `/clearrulesbtn` | Same as `/resetrulesbtn`. | ❌ |
| `/clearrulesbutton` | Same as `/resetrulesbtn`. | ❌ |

## Usage Examples

### Setting Rules

```
/setrules Please be respectful to all members.
```

Or reply to a message:
```
/setrules
```

### Private Rules

Enable private rules (rules sent via PM):
```
/privaterules on
```

Disable private rules:
```
/privaterules off
```

### Custom Rules Button

Set a custom button text:
```
/rulesbtn View Rules
```

Reset to default:
```
/resetrulesbtn
```

### Clear Rules

```
/clearrules
```

## Required Permissions

- User commands: Available to all users
- Admin commands: Require admin permissions in the chat
