---
title: Callback Queries
description: Complete reference of inline button callback handlers
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# Callback Queries

This page documents all inline button callback handlers in Alita Robot.

## Overview

- **Total Callbacks**: 21
- **Modules with Callbacks**: 14

## Callback Data Format

Callbacks use a versioned codec with URL-encoded fields:

```
<namespace>|v1|<url-encoded-fields>
```

For example: `restrict|v1|a=ban&uid=123456789` routes to the `restrict` namespace handler.

When the payload has no fields, `_` is used as placeholder: `helpq|v1|_`.

**Maximum length**: 64 bytes (Telegram's `callback_data` limit).

### Backward Compatibility

Legacy dot-notation (`prefix.field1.field2`) is accepted by handlers for backward compatibility but is deprecated. All new callbacks use the versioned format.

## All Callbacks

| Module | Prefix | Handler |
|--------|--------|----------|
| Bans | `restrict` | restrictButtonHandler |
| Bans | `unrestrict` | unrestrictButtonHandler |
| Blacklists | `rmAllBlacklist` | buttonHandler |
| Captcha | `captcha_refresh` | captchaRefreshCallback |
| Captcha | `captcha_verify` | captchaVerifyCallback |
| Connections | `connbtns` | connectionButtons |
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
| Purges | `deleteMsg` | deleteButtonHandler |
| Reports | `report` | markResolvedButtonHandler |
| Warns | `rmAllChatWarns` | warnsButtonHandler |
| Warns | `rmWarn` | rmWarnButton |

## Callbacks by Module

### Bans

#### `restrict`

- **Handler**: `restrictButtonHandler`
- **Source**: `bans.go`

#### `unrestrict`

- **Handler**: `unrestrictButtonHandler`
- **Source**: `bans.go`

### Blacklists

#### `rmAllBlacklist`

- **Handler**: `buttonHandler`
- **Source**: `blacklists.go`

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

### Purges

#### `deleteMsg`

- **Handler**: `deleteButtonHandler`
- **Source**: `purges.go`

### Reports

#### `report`

- **Handler**: `markResolvedButtonHandler`
- **Source**: `reports.go`

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
    "a":   "ban",
    "uid": "123456789",
})
// -> "restrict|v1|a=ban&uid=123456789"

// Empty payload
data, err := callbackcodec.Encode("helpq", map[string]string{})
// -> "helpq|v1|_"

// Decode callback data
decoded, err := callbackcodec.Decode("restrict|v1|a=ban&uid=123456789")
namespace := decoded.Namespace  // "restrict"
action, _ := decoded.Field("a") // "ban"
uid, _ := decoded.Field("uid")  // "123456789"
```

### Registering a Callback Handler

```go
dispatcher.AddHandler(handlers.NewCallback(
    callbackquery.Prefix("restrict"),
    myModule.restrictHandler,
))
```
