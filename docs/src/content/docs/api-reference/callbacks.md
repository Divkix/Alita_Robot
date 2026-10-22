---
title: Callback Queries
description: Complete reference of inline button callback handlers
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# Callback Queries

This page documents all inline button callback handlers in Alita Robot.

## Overview

- **Total Callbacks**: 30
- **Modules with Callbacks**: 19

## Callback Data Format

Callbacks use a versioned codec with URL-encoded fields:

```
<namespace>|v1|<url-encoded-fields>
```

For example: `restrict|v1|a=ban&u=123456789` routes to the `restrict` namespace handler.

When the payload has no fields, `_` is used as placeholder: `helpq|v1|_`.

**Maximum length**: 64 bytes (Telegram's `callback_data` limit).

### Strict Decoding

Handlers accept only the versioned format above. Malformed payloads, unsupported
versions, and legacy unversioned dot-notation payloads are rejected.

## All Callbacks

| Module | Prefix | Handler |
|--------|--------|----------|
| Backup | `backup` | backupCallbackHandler |
| Bans | `restrict` | restrictButtonHandler |
| Bans | `unrestrict` | unrestrictButtonHandler |
| Bans | `unbanall` | unbanAllCallback |
| Bans | `banlist` | banListCallback |
| Blacklists | `rmAllBlacklist` | buttonHandler |
| Bot Updates | `anon_admin` | verifyAnonymousAdmin |
| AntiRaid | `antiraid` | callbackHandler |
| Approvals | `rmAllApprovals` | unapproveAllCallback |
| Captcha | `captcha_refresh` | captchaRefreshCallback |
| Captcha | `captcha_verify` | captchaVerifyCallback |
| Connections | `connbtns` | connectionButtons |
| Connections | `settings` | settingsCallback |
| Federations | `fed` | fedCallback |
| Filters | `filters_overwrite` | filterOverWriteHandler |
| Filters | `rmAllFilters` | filtersButtonHandler |
| Formatting | `formatting` | formattingHandler |
| Greetings | `join_request` | joinRequestHandler |
| Help | `about` | about |
| Help | `configuration` | botConfig |
| Help | `helpq` | helpButtonHandler |
| Languages | `change_language` | langBtnHandler |
| Notes | `notes.overwrite` | noteOverWriteHandler |
| Notes | `rmAllNotes` | notesButtonHandler |
| Pins | `unpinallbtn` | unpinallCallback |
| Reactions | `reactions_help` | reactionsHelpHandler |
| Reports | `report` | markResolvedButtonHandler |
| Reports | `rpt` | reportCardHandler |
| Warns | `rmAllChatWarns` | warnsButtonHandler |
| Warns | `rmWarn` | rmWarnButton |

## Callbacks by Module

### Backup

#### `backup`

- **Handler**: `backupCallbackHandler`
- **Source**: `backup.go`

Confirms or cancels a pending /import or /reset via inline buttons (actions ci/xi for import, cr/xr for reset). Only the group creator can press them, and pending requests expire after 10 minutes. /export sends the backup file directly and uses no callback.

### Bans

#### `restrict`

- **Handler**: `restrictButtonHandler`
- **Source**: `bans.go`

#### `unrestrict`

- **Handler**: `unrestrictButtonHandler`
- **Source**: `bans.go`

#### `unbanall`

- **Handler**: `unbanAllCallback`
- **Source**: `bans.go`

Confirms or cancels `/unbanall`. Only the chat owner may press it.

#### `banlist`

- **Handler**: `banListCallback`
- **Source**: `bans_list.go`
- **Data**: `banlist|v1|p=<page>[&t=<search token>][&c=<group id>]`

Turns a `/banlist` page. The presser must be able to restrict members in the group the list shows (`c` when the list was sent from PM). A search is kept in Redis behind the 16-hex token `t` for an hour; a press after that is told to send /banlist again.

### Blacklists

#### `rmAllBlacklist`

- **Handler**: `buttonHandler`
- **Source**: `blacklists.go`

### Bot Updates

#### `anon_admin`

- **Handler**: `verifyAnonymousAdmin`
- **Source**: `bot_updates.go`

Verifies that the user who pressed the button is an admin of the group, then replays the command an anonymous group admin sent (fields: c = chat ID, m = message ID of the cached command).

### AntiRaid

#### `antiraid`

- **Handler**: `callbackHandler`
- **Source**: `antiraid.go`

Handles anti-raid mode toggle callbacks (enable/disable raid protection).

### Approvals

#### `rmAllApprovals`

- **Handler**: `unapproveAllCallback`
- **Source**: `approvals.go`

Handles confirmation callback for removing all approved users.

### Captcha

#### `captcha_refresh`

- **Handler**: `captchaRefreshCallback`
- **Source**: `captcha.go`

#### `captcha_verify`

- **Handler**: `captchaVerifyCallback`
- **Source**: `captcha.go`

### Connections

#### `connbtns`

- **Handler**: `connectionButtons`
- **Source**: `connections.go`

#### `settings`

- **Handler**: `settingsCallback`
- **Source**: `settings.go` (registered in `connections.go`)

Actions (field `a`):
- `a=p&c=<chat_id>`: picks a group from the `/settings` list. Re-checks that the presser may connect to it and is an admin there, connects them to it and shows the settings summary.
- `a=r&c=<chat_id>`: **Refresh**. Re-checks admin rights and redraws the summary.
- `a=l`: **Another group**. Shows the group list again. During the picker cooldown it shows the stored list, or answers with the wait notice when there is none.
- `a=t&c=<chat_id>&k=<key>&v=<value>`: a summary toggle (`settings_toggles.go`). Sets the setting `k` names to `v` after the checks its command runs (admin, bot admin, and change-info, creator or federation membership where the command needs them), writing through the command's repository function, then redraws the summary. A refused tap answers with an alert and changes nothing; an unknown key, or a value the command does not take, is answered as an invalid callback.

Works only in a private chat. A press anywhere else, or a payload that does not decode, is answered as an invalid callback.

### Federations

#### `fed`

- **Handler**: `fedCallback`
- **Source**: `federations.go`

Handles the `/delfed` confirmation buttons. `a=del` deletes the federation owned by the user who pressed it (anyone else is refused); `a=noop` cancels. Any other action is answered as a stale button.

### Filters

#### `filters_overwrite`

- **Handler**: `filterOverWriteHandler`
- **Source**: `filters.go`

#### `rmAllFilters`

- **Handler**: `filtersButtonHandler`
- **Source**: `filters.go`

### Formatting

#### `formatting`

- **Handler**: `formattingHandler`
- **Source**: `formatting.go`

### Greetings

#### `join_request`

- **Handler**: `joinRequestHandler`
- **Source**: `greetings.go`

### Help

#### `about`

- **Handler**: `about`
- **Source**: `help.go`

#### `configuration`

- **Handler**: `botConfig`
- **Source**: `help.go`

#### `helpq`

- **Handler**: `helpButtonHandler`
- **Source**: `help.go`

### Languages

#### `change_language`

- **Handler**: `langBtnHandler`
- **Source**: `language.go`

### Notes

#### `notes.overwrite`

- **Handler**: `noteOverWriteHandler`
- **Source**: `notes.go`

#### `rmAllNotes`

- **Handler**: `notesButtonHandler`
- **Source**: `notes.go`

### Pins

#### `unpinallbtn`

- **Handler**: `unpinallCallback`
- **Source**: `pins.go`

### Reactions

#### `reactions_help`

- **Handler**: `reactionsHelpHandler`
- **Source**: `reactions.go`

Displays help information for reaction commands.

### Reports

#### `report`

- **Handler**: `markResolvedButtonHandler`
- **Source**: `reports.go`

#### `rpt`

- **Handler**: `reportCardHandler`
- **Source**: `reports_callbacks.go`
- **Data**: `rpt|v1|a=<ban|mute|delete|dismiss>&i=<report id>`

Acts on a report card in the log channel. The presser must be an admin of the group the report came from, with the right for the action (restrict for ban/mute, delete for delete); admin rights in the log channel alone are not enough. The report id must belong to the card that was pressed. The first decision closes every open report of the message; later presses get "already handled".

### Warns

#### `rmAllChatWarns`

- **Handler**: `warnsButtonHandler`
- **Source**: `warns.go`

#### `rmWarn`

- **Handler**: `rmWarnButton`
- **Source**: `warns.go`

## Code Example

### Encoding and Decoding Callback Data

```go
// Encode callback data
data, err := callbackcodec.Encode("restrict", map[string]string{
    "a": "ban",
    "u": "123456789",
})
// -> "restrict|v1|a=ban&u=123456789"

// Empty payload
data, err := callbackcodec.Encode("helpq", map[string]string{})
// -> "helpq|v1|_"

// Decode callback data
decoded, err := callbackcodec.Decode("restrict|v1|a=ban&u=123456789")
namespace := decoded.Namespace  // "restrict"
action, _ := decoded.Field("a") // "ban"
userID, _ := decoded.Field("u") // "123456789"
```

### EncodeOrFallback

```go
data := callbackcodec.EncodeOrFallback("restrict", map[string]string{
    "a": "ban",
}, "fallback_data")
```

Encodes callback data with graceful fallback. If encoding fails, returns the fallback string instead of an error.

### Registering a Callback Handler

```go
dispatcher.AddHandler(handlers.NewCallback(
    callbackquery.Prefix("restrict"),
    myModule.restrictHandler,
))
```
