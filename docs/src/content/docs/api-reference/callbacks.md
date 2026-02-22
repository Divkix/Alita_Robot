---
title: Callback Queries
description: Complete reference of inline button callback handlers
---

# 🔔 Callback Queries

This page documents all inline button callback handlers in Alita Robot.

## Overview

- **Total Callbacks**: 21
- **Modules with Callbacks**: 14

## Callback Data Format

:::note
Callbacks use a prefix-based routing system. The dispatcher matches the beginning of the callback data string and routes to the corresponding handler.
:::

```
{prefix}{data}
```

For example: `restrict.ban.123456789` routes to the `restrict.` handler with data `ban.123456789`.

## All Callbacks

| Module | Prefix | Handler |
|--------|--------|----------|
| Bans | `restrict.` | restrictButtonHandler |
| Bans | `unrestrict.` | unrestrictButtonHandler |
| Blacklists | `rmAllBlacklist` | buttonHandler |
| Captcha | `captcha_refresh.` | captchaRefreshCallback |
| Captcha | `captcha_verify.` | captchaVerifyCallback |
| Connections | `connbtns.` | connectionButtons |
| Filters | `filters_overwrite.` | filterOverWriteHandler |
| Filters | `rmAllFilters` | filtersButtonHandler |
| Formatting | `formatting.` | formattingHandler |
| Greetings | `join_request.` | joinRequestHandler |
| Help | `about` | about |
| Help | `configuration` | botConfig |
| Help | `helpq` | helpButtonHandler |
| Languages | `change_language.` | langBtnHandler |
| Notes | `notes.overwrite.` | noteOverWriteHandler |
| Notes | `rmAllNotes` | notesButtonHandler |
| Pins | `unpinallbtn` | unpinallCallback |
| Purges | `deleteMsg.` | deleteButtonHandler |
| Reports | `report.` | markResolvedButtonHandler |
| Warns | `rmAllChatWarns` | warnsButtonHandler |
| Warns | `rmWarn` | rmWarnButton |

## Callbacks by Module

### Bans

#### `restrict.`

- **Handler**: `restrictButtonHandler`
- **Source**: `bans.go`

#### `unrestrict.`

- **Handler**: `unrestrictButtonHandler`
- **Source**: `bans.go`

### Blacklists

#### `rmAllBlacklist`

- **Handler**: `buttonHandler`
- **Source**: `blacklists.go`

### Captcha

#### `captcha_refresh.`

- **Handler**: `captchaRefreshCallback`
- **Source**: `captcha.go`

#### `captcha_verify.`

- **Handler**: `captchaVerifyCallback`
- **Source**: `captcha.go`

### Connections

#### `connbtns.`

- **Handler**: `connectionButtons`
- **Source**: `connections.go`

### Filters

#### `filters_overwrite.`

- **Handler**: `filterOverWriteHandler`
- **Source**: `filters.go`

#### `rmAllFilters`

- **Handler**: `filtersButtonHandler`
- **Source**: `filters.go`

### Formatting

#### `formatting.`

- **Handler**: `formattingHandler`
- **Source**: `formatting.go`

### Greetings

#### `join_request.`

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

#### `change_language.`

- **Handler**: `langBtnHandler`
- **Source**: `language.go`

### Notes

#### `notes.overwrite.`

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

#### `deleteMsg.`

- **Handler**: `deleteButtonHandler`
- **Source**: `purges.go`

### Reports

#### `report.`

- **Handler**: `markResolvedButtonHandler`
- **Source**: `reports.go`

### Warns

#### `rmAllChatWarns`

- **Handler**: `warnsButtonHandler`
- **Source**: `warns.go`

#### `rmWarn`

- **Handler**: `rmWarnButton`
- **Source**: `warns.go`

## Registering Callbacks

```go
dispatcher.AddHandler(handlers.NewCallback(
    callbackquery.Prefix("myprefix."),
    myModule.myCallbackHandler,
))
```

## Handling Callbacks

:::caution
Always answer the callback query. If you do not call `query.Answer()`, the user sees a loading spinner indefinitely. Also be careful not to answer twice -- `RequireUserAdmin` with `justCheck=false` already answers the callback.
:::

```go
func (m moduleStruct) myCallbackHandler(b *gotgbot.Bot, ctx *ext.Context) error {
    query := ctx.CallbackQuery

    // Parse callback data
    data := strings.TrimPrefix(query.Data, "myprefix.")

    // Process and answer
    query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
        Text: "Action completed",
    })

    return ext.EndGroups
}
```
