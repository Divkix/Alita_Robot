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

**Rules of a group from PM:**
In PM, name the group as the first argument, by its ID, its @username, or the title of a group where you are an admin (case does not matter; quote a title with spaces):
`/rules -1001234567890`
`/rules @mygroup`
`/rules "My Group"`

This works without `/connect` and, when you are connected, shows the named group instead of the connected one; the connection stays as it is. The group passes the same checks as `/connect`: admins always may, other members only while `/allowconnect` is on. Titles are looked up only among groups where you are an admin, and only among the connected group and the 50 most recently active groups where you have written, so a member, or anyone naming another group, names it by its @username or ID. A user's or channel's @username is not a group. When two of your groups share a title, the bot lists both with their IDs and asks for the @username or ID. Without an argument, `/rules` in PM shows the connected group; with no connection it answers with the command's help, an example and the groups where you are an admin. In a group the argument is ignored.

**Required Permissions:**
- `/rules`: Available to all users (disableable), but **bot must be admin** in the chat; in PM it needs the group as its first argument or a group connected with `/connect`
- All other commands (`/setrules`, `/resetrules`, `/privaterules`, `/rulesbtn`, `/resetrulesbtn`, etc.): Require admin permissions in the group they act on via `IsUserConnected`: in PM the group named as the first argument, or else the connected group


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
| `/rules` | Any user (disableable) — **bot must be admin**; in PM needs the group as the first argument (`/rules @group` or `/rules -100…`) or a `/connect`ed group |
| `/setrules` | Admin only — requires `IsUserConnected` (bot admin + user admin) |
| `/resetrules`, `/clearrules` | Admin only — requires `IsUserConnected` |
| `/privaterules` | Admin only — requires `IsUserConnected`, user must be admin in the named or connected group |
| `/rulesbtn`, `/rulesbutton` | Admin only — requires `IsUserConnected`, user must be admin in the named or connected group |
| `/resetrulesbtn`, `/resetrulesbutton`, `/clearrulesbtn`, `/clearrulesbutton` | Admin only — requires `IsUserConnected` |
