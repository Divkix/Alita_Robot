# Implementation Tasks: Fix 13 Bugs in User-Facing Command Handlers

**Date:** 2026-02-22
**Design Source:** design.md
**Total Tasks:** 14
**Slicing Strategy:** vertical (each task = complete feature slice)

---

## TASK-001: Fix logUsers Nil Sender Panic (US-001)

**Complexity:** M
**Files:**
- MODIFY: `alita/modules/users.go` -- Add nil guard for `ctx.EffectiveSender` at top of `logUsers`, guard `repliedMsg.GetSender()` for nil before calling methods on it
**Dependencies:** None
**Description:**
The `logUsers` handler at `users.go:73` runs on EVERY message (handler group -1) but accesses `ctx.EffectiveSender` without a nil check. Channel posts have nil sender, causing a panic that crashes the entire message processing pipeline.

Changes to `alita/modules/users.go`:
1. At line 76, immediately after `user := ctx.EffectiveSender`, add:
   ```go
   if user == nil {
       return ext.ContinueGroups // channel posts have nil sender
   }
   ```
2. At line 116-135, the `repliedMsg` block calls `repliedMsg.GetSender()` multiple times without checking for nil. Extract `replySender := repliedMsg.GetSender()` once, add `if replySender == nil` guard before accessing `.IsAnonymousChannel()`, `.Id()`, `.Name()`, `.Username()`. Refactor the block to:
   ```go
   if repliedMsg != nil {
       replySender := repliedMsg.GetSender()
       if replySender != nil {
           if replySender.IsAnonymousChannel() {
               // ... channel update logic using replySender ...
           } else {
               // ... user update logic using replySender ...
           }
       }
   }
   ```

Edge cases covered: EC-001a (channel post), EC-001c (reply to channel post), EC-001e (linked channel), EC-001g (repliedMsg.GetSender() nil).

**Context to Read:**
- design.md, section "US-001: logUsers Nil Sender Guard"
- `alita/modules/users.go` -- full file, understand existing flow
- requirements.md, section "US-001" for acceptance criteria and edge cases
**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot/.claude/worktrees/tender-seeking-hinton && go vet ./alita/modules/ && go build ./...
```

---

## TASK-002: Fix Purge Callback user.Id and misc.go Nil Guards (US-002, US-003, US-004, US-013)

**Complexity:** M
**Files:**
- MODIFY: `alita/modules/purges.go` -- Replace 3x `ctx.EffectiveUser.Id` with `user.Id` at lines 149, 335, 419
- MODIFY: `alita/modules/misc.go` -- Add nil sender guard in `info`, nil From guard in `echomsg`, replace `time.Sleep` with `time.AfterFunc` in `removeBotKeyboard`
**Dependencies:** None
**Description:**
Four bugs across two files, all nil-guard and handler-blocking fixes.

**purges.go changes (US-002):**
1. Line 149: Change `ctx.EffectiveUser.Id` to `user.Id` in `RequireUserAdmin` call within `purge()`.
2. Line 335: Change `ctx.EffectiveUser.Id` to `user.Id` in `RequireUserAdmin` call within `purgeFrom()`.
3. Line 419: Change `ctx.EffectiveUser.Id` to `user.Id` in `RequireUserAdmin` call within `purgeTo()`.

In all three cases, `user` is already declared via `user := chat_status.RequireUser(bot, ctx, false)` earlier in the function, with a nil check immediately after. The `ctx.EffectiveUser` can be nil for anonymous admins but `user` from `RequireUser()` is the correctly resolved user.

**misc.go changes (US-003 -- info nil sender):**
At line 282-284, the `case 0:` block calls `sender.Id()` without checking if `sender` is nil. Add:
```go
case 0:
    if sender == nil {
        return ext.EndGroups
    }
    userId = sender.Id()
```

**misc.go changes (US-004 -- echomsg nil From):**
At line 79, `msg.From.Id` is accessed without a nil check. Add before line 79:
```go
if msg.From == nil {
    return ext.EndGroups
}
```

**misc.go changes (US-013 -- removeBotKeyboard non-blocking):**
Replace lines 461-466 (the `time.Sleep` + synchronous delete) with:
```go
time.AfterFunc(1*time.Second, func() {
    defer error_handling.RecoverFromPanic("removeBotKeyboard", "misc")
    _, err := rMsg.Delete(b, nil)
    if err != nil {
        log.WithFields(log.Fields{"chat_id": rMsg.Chat.Id, "message_id": rMsg.MessageId}).
            Debug("[Misc] Failed to delete keyboard removal message")
    }
})
return ext.EndGroups
```
This requires adding `"github.com/divkix/Alita_Robot/alita/utils/error_handling"` to the imports in misc.go. Remove the `return err` line after the delete since `removeBotKeyboard` now returns immediately.

**Context to Read:**
- design.md, sections "US-002", "US-003", "US-004", "US-013"
- `alita/modules/purges.go` -- lines 130-155, 315-340, 400-425
- `alita/modules/misc.go` -- lines 72-114 (echomsg), 275-346 (info), 444-469 (removeBotKeyboard)
- `alita/utils/error_handling/error_handling.go` -- verify `RecoverFromPanic` signature
**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot/.claude/worktrees/tender-seeking-hinton && go vet ./alita/modules/ && go build ./... && grep -r "time.Sleep" alita/modules/ && grep -n "ctx.EffectiveUser.Id" alita/modules/purges.go
```
Both grep commands should return zero results.

---

## TASK-003: Fix config.yml Loading for alt_names Resolution (US-006)

**Complexity:** S
**Files:**
- MODIFY: `alita/i18n/loader.go` -- Remove 3-line config.yml skip block (lines 37-39)
**Dependencies:** None
**Description:**
The `loadLocaleFiles()` function at `loader.go:37-39` explicitly skips `config.yml` and `config.yaml`. This prevents `MustNewTranslator("config")` from finding the config data, breaking module alternative name resolution in the help system (`getModuleNameFromAltName` in `helpers.go`).

Remove lines 37-39:
```go
// REMOVE THESE 3 LINES:
if fileName == "config.yml" || fileName == "config.yaml" {
    continue
}
```

After removal, `extractLangCode("config.yml")` returns `"config"`, `loadSingleLocaleFile` processes it, and `MustNewTranslator("config")` will find the config viper instance.

The `check-translations` script already skips "config" -- verified by design. The `GetLanguage()` function will never return "config" because it only returns user/group language preferences from DB.

**Context to Read:**
- design.md, section "US-006: config.yml Loading"
- `alita/i18n/loader.go` -- full file
- `alita/modules/helpers.go` -- lines 194-206 (`getModuleNameFromAltName`) to understand the consumer
- `locales/config.yml` -- to verify it has alt_names data
**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot/.claude/worktrees/tender-seeking-hinton && go build ./... && make check-translations
```

---

## TASK-004: Fix IdFromReply Nil Sender Guard (US-007)

**Complexity:** S
**Files:**
- MODIFY: `alita/utils/extraction/extraction.go` -- Add nil guard for `prevMessage.GetSender()` in `IdFromReply`
**Dependencies:** None
**Description:**
At `extraction.go:251`, `IdFromReply` calls `prevMessage.GetSender().Id()` without checking if `GetSender()` returns nil. When replying to a service message or channel post with no author signature, this panics.

Change line 251 from:
```go
userId = prevMessage.GetSender().Id()
```
to:
```go
replySender := prevMessage.GetSender()
if replySender == nil {
    return 0, ""
}
userId = replySender.Id()
```

Keep the `-1` sentinel at line 130 unchanged (design decision: `-1` means "lookup failed AND error already sent to user").

**Context to Read:**
- design.md, section "US-007: IdFromReply Nil Guard"
- `alita/utils/extraction/extraction.go` -- lines 241-258 (`IdFromReply`) and lines 120-130 (the -1 sentinel)
- requirements.md, section "US-007" for edge cases EC-007a through EC-007g
**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot/.claude/worktrees/tender-seeking-hinton && go vet ./alita/utils/extraction/ && go build ./...
```

---

## TASK-005: Fix chatlist.txt Race Condition (US-011)

**Complexity:** M
**Files:**
- MODIFY: `alita/modules/devs.go` -- Replace fixed filename with `os.CreateTemp`, add defer cleanup, check `os.Open` error
**Dependencies:** None
**Description:**
At `devs.go:104`, the `/chatlist` handler uses a fixed filename `"chatlist.txt"` which causes race conditions when two devs invoke simultaneously. Also at `devs.go:123`, `os.Open` error is ignored with `_`.

Changes to the `chatlist` handler (around lines 103-154):
1. Replace `fileName := "chatlist.txt"` and `os.WriteFile(fileName, ...)` with `os.CreateTemp`:
   ```go
   tmpFile, err := os.CreateTemp("", "chatlist-*.txt")
   if err != nil {
       log.Error(err)
       return err
   }
   defer os.Remove(tmpFile.Name())
   ```
2. Write content to `tmpFile` using `tmpFile.WriteString(writeString)` instead of `os.WriteFile`.
3. Close `tmpFile` after writing, then reopen for sending:
   ```go
   if err := tmpFile.Close(); err != nil {
       log.Error(err)
       return err
   }
   openedFile, err := os.Open(tmpFile.Name())
   if err != nil {
       log.Error(err)
       // Send error message to user
       errText, _ := tr.GetString("devs_chatlist_error")
       _, _ = msg.Reply(b, errText, nil)
       return ext.EndGroups
   }
   defer openedFile.Close()
   ```
4. Remove the manual `openedFile.Close()` and `os.Remove()` at the bottom since `defer` handles cleanup.
5. Update the `gotgbot.InputFileByReader` call to use `"chatlist.txt"` as the display name but `openedFile` as the reader (the display name does NOT need to match the temp file name).

This requires adding the `devs_chatlist_error` i18n key, but that is handled in TASK-008 (the i18n keys task). For now, use a hardcoded fallback:
```go
errText, _ := tr.GetString("devs_chatlist_error")
if errText == "" {
    errText = "Failed to generate chat list. Please try again."
}
```

**Context to Read:**
- design.md, section "US-011: chatlist.txt Race Fix"
- `alita/modules/devs.go` -- lines 80-157 (the `chatlist` handler)
- requirements.md, section "US-011" for edge cases
**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot/.claude/worktrees/tender-seeking-hinton && go vet ./alita/modules/ && go build ./...
```

---

## TASK-006: Fix secureIntn Bounded Retry (US-012)

**Complexity:** S
**Files:**
- MODIFY: `alita/modules/captcha.go` -- Add bounded retry loop (max 10) to `secureIntn`, log on exhaustion
**Dependencies:** None
**Description:**
At `captcha.go:240-245`, `secureIntn` has an infinite `for {}` loop that retries `crand.Int()` on error. If the entropy source fails persistently (e.g., in a container without `/dev/urandom`), this causes CPU starvation.

Replace lines 240-245:
```go
func secureIntn(max int) int {
    if max <= 0 {
        return 0
    }
    const maxRetries = 10
    for i := 0; i < maxRetries; i++ {
        n, err := crand.Int(crand.Reader, big.NewInt(int64(max)))
        if err == nil {
            return int(n.Int64())
        }
    }
    log.Error("[Captcha] secureIntn: exhausted retries for crypto/rand.Int, returning 0")
    return 0
}
```

**Context to Read:**
- design.md, section "US-012: secureIntn Bounded Retry"
- `alita/modules/captcha.go` -- lines 232-246 (the `secureIntn` function)
- requirements.md, section "US-012" for edge cases
**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot/.claude/worktrees/tender-seeking-hinton && go vet ./alita/modules/ && go build ./...
```

---

## TASK-007: Fix IsUserConnected Mutation, Goroutine Safety, HtmlEscape (US-008, US-009, US-010)

**Complexity:** M
**Files:**
- MODIFY: `alita/utils/helpers/helpers.go` -- Remove `ctx.EffectiveChat = chat` mutation (line 307), replace `HtmlEscape` body with `html.EscapeString(s)`
- MODIFY: `alita/modules/helpers.go` -- Wrap `go db.ConnectId` at line 260 with recovery closure
- MODIFY: `alita/modules/locks.go` -- Fix 2 Pattern D callers of `IsUserConnected` at `locktypes` and `locks` functions
**Dependencies:** None
**Description:**
Three related bugs in the helpers + locks cluster.

**US-008: IsUserConnected mutation removal (`alita/utils/helpers/helpers.go`):**
Remove line 307: `ctx.EffectiveChat = chat`. The function should return the chat without mutating context. After this change, callers that depended on the implicit mutation must assign explicitly.

28 Category A callers already have `ctx.EffectiveChat = connectedChat` (no change needed). 10 Category B callers use `connectedChat` directly (no change needed). Only 2 Pattern D callers in locks.go need fixing.

**US-008: Fix 2 Pattern D callers (`alita/modules/locks.go`):**
In `locktypes()` at ~line 149:
```go
// BEFORE:
if helpers.IsUserConnected(b, ctx, false, true) == nil {
    return ext.EndGroups
}
// AFTER:
connectedChat := helpers.IsUserConnected(b, ctx, false, true)
if connectedChat == nil {
    return ext.EndGroups
}
ctx.EffectiveChat = connectedChat
```

In `locks()` at ~line 174:
```go
// BEFORE:
if helpers.IsUserConnected(b, ctx, true, true) == nil {
    return ext.EndGroups
}
chat := ctx.EffectiveChat
// AFTER:
connectedChat := helpers.IsUserConnected(b, ctx, true, true)
if connectedChat == nil {
    return ext.EndGroups
}
ctx.EffectiveChat = connectedChat
chat := ctx.EffectiveChat
```

**US-009: Goroutine safety (`alita/modules/helpers.go`):**
At line 260, replace:
```go
go db.ConnectId(user.Id, cochat.Id)
```
with:
```go
go func() {
    defer error_handling.RecoverFromPanic("ConnectId", "helpers")
    db.ConnectId(user.Id, cochat.Id)
}()
```
Add `"github.com/divkix/Alita_Robot/alita/utils/error_handling"` to the imports.

**US-010: HtmlEscape (`alita/utils/helpers/helpers.go`):**
Replace the body of `HtmlEscape` (lines 147-150) with:
```go
func HtmlEscape(s string) string {
    return html.EscapeString(s)
}
```
The `"html"` import already exists in the file.

**Context to Read:**
- design.md, sections "US-008", "US-009", "US-010"
- `alita/utils/helpers/helpers.go` -- lines 146-151 (HtmlEscape), 240-309 (IsUserConnected)
- `alita/modules/helpers.go` -- lines 255-265 (the connect deep link handler), verify import list
- `alita/modules/locks.go` -- lines 142-186 (locktypes + locks functions)
- `alita/utils/error_handling/error_handling.go` -- `RecoverFromPanic` signature
**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot/.claude/worktrees/tender-seeking-hinton && go vet ./alita/utils/helpers/ ./alita/modules/ && go build ./...
```

---

## TASK-008: Add i18n Keys for DB Error Messages (US-005 prerequisite)

**Complexity:** S
**Files:**
- MODIFY: `locales/en.yml` -- Add 2 i18n keys: `common_settings_save_failed`, `devs_chatlist_error`
- MODIFY: `locales/es.yml` -- Add same 2 keys in Spanish
- MODIFY: `locales/fr.yml` -- Add same 2 keys in French
- MODIFY: `locales/hi.yml` -- Add same 2 keys in Hindi
**Dependencies:** None
**Description:**
Add the 2 new i18n keys needed by the DB void-to-error refactoring (US-005) and the chatlist error (US-011). These must exist in ALL 4 locale files before the module caller tasks can use them.

Add to each locale file (at the end or in alphabetical position among existing keys):

**en.yml:**
```yaml
common_settings_save_failed: "Failed to save settings. Please try again later."
devs_chatlist_error: "Failed to generate chat list. Please try again."
```

**es.yml:**
```yaml
common_settings_save_failed: "Error al guardar la configuracion. Intentalo de nuevo mas tarde."
devs_chatlist_error: "Error al generar la lista de chats. Intentalo de nuevo."
```

**fr.yml:**
```yaml
common_settings_save_failed: "Echec de la sauvegarde des parametres. Veuillez reessayer plus tard."
devs_chatlist_error: "Echec de la generation de la liste des chats. Veuillez reessayer."
```

**hi.yml:**
```yaml
common_settings_save_failed: "Settings save karne mein error. Baad mein try karein."
devs_chatlist_error: "Chat list generate karne mein error. Phir try karein."
```

**Context to Read:**
- design.md, section "i18n Keys (2 new keys, all 4 locales)"
- `locales/en.yml` -- to find appropriate insertion point and verify formatting style
**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot/.claude/worktrees/tender-seeking-hinton && make check-translations
```

---

## TASK-009: DB Void-to-Error -- greetings_db.go + greetings.go (US-005, part 1)

**Complexity:** L
**Files:**
- MODIFY: `alita/db/greetings_db.go` -- Change 10 functions to return `error`: SetWelcomeText, SetWelcomeToggle, SetGoodbyeText, SetGoodbyeToggle, SetShouldCleanService, SetShouldAutoApprove, SetCleanWelcomeSetting, SetCleanGoodbyeSetting, SetCleanWelcomeMsgId, SetCleanGoodbyeMsgId
- MODIFY: `alita/modules/greetings.go` -- Wrap ~16 call sites with error checking
**Dependencies:** TASK-008
**Description:**
The largest sub-task of US-005. 10 greeting DB functions currently have void return types. They log errors internally but return nothing, so callers show success messages even when DB writes fail.

**DB function transformation pattern (apply to all 10):**
For each function, change signature to return `error`, change bare `return` after error to `return err`, add `return nil` at function end.

Example for `SetWelcomeText`:
```go
// BEFORE:
func SetWelcomeText(chatID int64, welcometxt, fileId string, buttons []Button, welcType int) {
    // ...
    err := UpdateRecord(...)
    if err != nil {
        log.Errorf(...)
        return
    }
}

// AFTER:
func SetWelcomeText(chatID int64, welcometxt, fileId string, buttons []Button, welcType int) error {
    // ...
    err := UpdateRecord(...)
    if err != nil {
        log.Errorf(...)
        return err
    }
    return nil
}
```

Apply the same pattern to all 10 functions. For functions that use `DB.Model(...).Updates(...)`, capture the error and return it. For no-op early returns (e.g., when update is not needed), change bare `return` to `return nil`.

**Caller transformation pattern (greetings.go):**
For user-facing call sites (command handlers), wrap with:
```go
if err := db.SetWelcomeToggle(chat.Id, true); err != nil {
    log.Errorf("[Greetings] SetWelcomeToggle failed for chat %d: %v", chat.Id, err)
    errText, _ := tr.GetString("common_settings_save_failed")
    _, _ = msg.Reply(bot, errText, helpers.Shtml())
    return ext.EndGroups
}
```

For non-user-facing call sites (join/leave event handlers where the user did not initiate the action), use warning-only:
```go
if err := db.SetCleanWelcomeMsgId(chat.Id, sent.MessageId); err != nil {
    log.Warnf("[Greetings] Failed to store clean welcome msg ID for chat %d: %v", chat.Id, err)
}
```

Identify all call sites by grepping greetings.go for `db.SetWelcome`, `db.SetGoodbye`, `db.SetShouldClean`, `db.SetShouldAutoApprove`, `db.SetCleanWelcome`, `db.SetCleanGoodbye`. Each must be wrapped appropriately.

IMPORTANT: This task will break `go build` until ALL DB function callers are updated. The verification for this individual task only checks its own two files compile together.

**Context to Read:**
- design.md, section "US-005: DB Void-to-Error Refactor"
- `alita/db/greetings_db.go` -- full file
- `alita/modules/greetings.go` -- full file, find all call sites
- requirements.md, section "US-005" for edge cases
**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot/.claude/worktrees/tender-seeking-hinton && go vet ./alita/db/ ./alita/modules/ 2>&1 | head -20
```
Note: Full `go build` will only succeed after ALL US-005 sub-tasks are complete.

---

## TASK-010: DB Void-to-Error -- antiflood_db.go + antiflood.go (US-005, part 2)

**Complexity:** M
**Files:**
- MODIFY: `alita/db/antiflood_db.go` -- Change 3 functions to return `error`: SetFlood, SetFloodMode, SetFloodMsgDel
- MODIFY: `alita/modules/antiflood.go` -- Wrap 5 call sites with error checking
**Dependencies:** TASK-008
**Description:**
Apply the same void-to-error transformation as TASK-009 to 3 antiflood DB functions.

**DB changes (antiflood_db.go):**
- `SetFlood`: Change `func SetFlood(chatID int64, limit int)` to `func SetFlood(chatID int64, limit int) error`. For early returns where `floodSrc.Limit == limit`, return `nil`. Change the error path from `log.Errorf(...)` to `return err`. Add `return nil` at end. Keep cache invalidation before the final `return nil`.
- `SetFloodMode`: Same pattern. Early return for `floodSrc.Action == mode` becomes `return nil`.
- `SetFloodMsgDel`: Same pattern. Early return for `floodSrc.DeleteAntifloodMessage == val` becomes `return nil`.

**Caller changes (antiflood.go):**
Find 5 call sites: `db.SetFlood(`, `db.SetFloodMode(`, `db.SetFloodMsgDel(`. Wrap each with:
```go
if err := db.SetFlood(chat.Id, 0); err != nil {
    log.Errorf("[Antiflood] SetFlood failed for chat %d: %v", chat.Id, err)
    errText, _ := tr.GetString("common_settings_save_failed")
    _, _ = msg.Reply(bot, errText, helpers.Shtml())
    return ext.EndGroups
}
```

**Context to Read:**
- design.md, section "US-005: DB Void-to-Error Refactor" (antiflood rows)
- `alita/db/antiflood_db.go` -- full file
- `alita/modules/antiflood.go` -- grep for `db.SetFlood`, `db.SetFloodMode`, `db.SetFloodMsgDel`
**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot/.claude/worktrees/tender-seeking-hinton && go vet ./alita/db/ ./alita/modules/ 2>&1 | head -20
```

---

## TASK-011: DB Void-to-Error -- lang_db.go + language.go + pin_db.go + pins.go (US-005, part 3)

**Complexity:** M
**Files:**
- MODIFY: `alita/db/lang_db.go` -- Change 2 functions to return `error`: ChangeUserLanguage, ChangeGroupLanguage
- MODIFY: `alita/db/pin_db.go` -- Change 2 functions to return `error`: SetAntiChannelPin, SetCleanLinked
- MODIFY: `alita/modules/language.go` -- Wrap 2 call sites
- MODIFY: `alita/modules/pins.go` -- Wrap 4 call sites
**Dependencies:** TASK-008
**Description:**
Apply void-to-error transformation to 4 DB functions across 2 DB files and 2 module files.

**DB changes (lang_db.go):**
- `ChangeUserLanguage`: Multiple return paths. Each early `return` (after error log, after no-op check) becomes `return err` or `return nil`. Add `return nil` at end.
- `ChangeGroupLanguage`: Same pattern.

IMPORTANT: These functions have multiple exit paths with `return` after error logging. Every bare `return` must be changed to either `return err` or `return nil` as appropriate. Do not miss the early returns for "language already set" (no-op case) -- those should be `return nil`.

**DB changes (pin_db.go):**
- `SetCleanLinked`: Currently calls `UpdateRecordWithZeroValues` and logs error. Change to return the error:
  ```go
  func SetCleanLinked(chatID int64, pref bool) error {
      err := UpdateRecordWithZeroValues(...)
      if err != nil {
          log.Errorf(...)
          return err
      }
      return nil
  }
  ```
- `SetAntiChannelPin`: Same pattern.

**Caller changes (language.go):**
Find 2 call sites: `db.ChangeUserLanguage(`, `db.ChangeGroupLanguage(`. Wrap with error checking using `common_settings_save_failed`.

**Caller changes (pins.go):**
Find 4 call sites: `db.SetAntiChannelPin(`, `db.SetCleanLinked(`. Wrap with error checking.

**Context to Read:**
- design.md, section "US-005" (lang, pin rows)
- `alita/db/lang_db.go` -- lines 80-152 (ChangeUserLanguage, ChangeGroupLanguage)
- `alita/db/pin_db.go` -- lines 31-47 (SetCleanLinked, SetAntiChannelPin)
- `alita/modules/language.go` -- grep for `db.ChangeUserLanguage`, `db.ChangeGroupLanguage`
- `alita/modules/pins.go` -- grep for `db.SetAntiChannelPin`, `db.SetCleanLinked`
**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot/.claude/worktrees/tender-seeking-hinton && go vet ./alita/db/ ./alita/modules/ 2>&1 | head -20
```

---

## TASK-012: DB Void-to-Error -- warns_db.go + warns.go + filters_db.go + filters.go (US-005, part 4)

**Complexity:** M
**Files:**
- MODIFY: `alita/db/warns_db.go` -- Change 2 functions to return `error`: SetWarnLimit, SetWarnMode
- MODIFY: `alita/db/filters_db.go` -- Change 2 functions to return `error`: AddFilter, RemoveFilter
- MODIFY: `alita/modules/warns.go` -- Wrap 4 call sites
- MODIFY: `alita/modules/filters.go` -- Wrap 5 call sites
**Dependencies:** TASK-008
**Description:**
Final batch of the US-005 void-to-error transformation: 4 DB functions across 2 DB files and 2 module files.

**DB changes (warns_db.go):**
- `SetWarnLimit` (line 212): `func SetWarnLimit(chatId int64, warnLimit int) error`. Return `err` on failure, `nil` on success. Keep cache invalidation before `return nil`.
- `SetWarnMode` (line 226): Same pattern.

**DB changes (filters_db.go):**
- `AddFilter` (line 54): `func AddFilter(chatID int64, keyWord, replyText, fileID string, buttons []Button, filtType int) error`. Multiple exit paths: early return when filter exists becomes `return nil`, early return on existence-check error becomes `return err`, CreateRecord error becomes `return err`, success becomes `return nil`. Keep cache invalidation.
- `RemoveFilter` (line 90): `func RemoveFilter(chatID int64, keyWord string) error`. Return `result.Error` on failure, `nil` on success. Keep cache invalidation.

**Caller changes (warns.go):**
Find 4 call sites: `db.SetWarnLimit(`, `db.SetWarnMode(`. Wrap each with error checking using `common_settings_save_failed`.

**Caller changes (filters.go):**
Find 5 call sites: `db.AddFilter(`, `db.RemoveFilter(`. Wrap each with error checking. For `AddFilter`, the handler should show an error message on failure. For `RemoveFilter`, same pattern.

**Context to Read:**
- design.md, section "US-005" (warns, filters rows)
- `alita/db/warns_db.go` -- lines 212-236 (SetWarnLimit, SetWarnMode)
- `alita/db/filters_db.go` -- lines 54-103 (AddFilter, RemoveFilter)
- `alita/modules/warns.go` -- grep for `db.SetWarnLimit`, `db.SetWarnMode`
- `alita/modules/filters.go` -- grep for `db.AddFilter`, `db.RemoveFilter`
**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot/.claude/worktrees/tender-seeking-hinton && go vet ./alita/db/ ./alita/modules/ 2>&1 | head -20
```

---

## TASK-013: US-005 Build Verification Gate

**Complexity:** S
**Files:**
(no file changes -- verification only)
**Dependencies:** TASK-009, TASK-010, TASK-011, TASK-012
**Description:**
After all 4 US-005 sub-tasks are complete, verify that the entire codebase compiles. The US-005 changes are atomic across 12 files (6 DB + 6 module). Individual sub-tasks may not compile alone because changing a DB function signature without updating ALL callers breaks the build. This task confirms the full build succeeds.

If the build fails, identify which call site was missed and fix it. Common causes:
- A caller in a module file that was not in the expected list (grep for the function name across all Go files)
- An early return path that was not converted to `return nil`
- A missing `error` in the return type

**Context to Read:**
- All 6 DB files modified in TASK-009 through TASK-012
- All 6 module files modified in TASK-009 through TASK-012
**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot/.claude/worktrees/tender-seeking-hinton && go build ./... && go vet ./...
```

**Rollback:** If build fails and the issue is in a single file, fix it directly. If the issue is systemic, `git stash` the incomplete changes and re-examine the design.

---

## TASK-014: Full Integration Verification

**Complexity:** S
**Files:**
(no file changes -- verification only)
**Dependencies:** TASK-001, TASK-002, TASK-003, TASK-004, TASK-005, TASK-006, TASK-007, TASK-008, TASK-009, TASK-010, TASK-011, TASK-012, TASK-013
**Description:**
Run the complete verification suite to confirm all 13 bugs are fixed, no regressions introduced, and all acceptance criteria from requirements.md are met.

Check each specific verification from the design:
1. `go build ./...` -- clean build
2. `make lint` -- golangci-lint passes (same pre-existing warnings only: 63 dupl, 4 godox)
3. `make test` -- all tests pass
4. `make check-translations` -- all i18n keys present in all 4 locales
5. `go vet ./...` -- no vet issues
6. `grep -r "time.Sleep" alita/modules/` -- zero results (US-013 verified)
7. `grep -n "ctx.EffectiveUser.Id" alita/modules/purges.go` -- zero results (US-002 verified)

**Context to Read:**
- design.md, section "Testing Strategy" and "Verification commands"
- requirements.md, all "Definition of Done" sections
**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot/.claude/worktrees/tender-seeking-hinton && \
  go build ./... && \
  make lint && \
  make test && \
  make check-translations && \
  go vet ./... && \
  echo "=== US-013 check ===" && \
  grep -r "time.Sleep" alita/modules/ ; \
  echo "=== US-002 check ===" && \
  grep -n "ctx.EffectiveUser.Id" alita/modules/purges.go ; \
  echo "VERIFICATION COMPLETE"
```

---

## File Manifest

| Task | Files | Operation |
|------|-------|-----------|
| TASK-001 | `alita/modules/users.go` | MODIFY |
| TASK-002 | `alita/modules/purges.go` | MODIFY |
| TASK-002 | `alita/modules/misc.go` | MODIFY |
| TASK-003 | `alita/i18n/loader.go` | MODIFY |
| TASK-004 | `alita/utils/extraction/extraction.go` | MODIFY |
| TASK-005 | `alita/modules/devs.go` | MODIFY |
| TASK-006 | `alita/modules/captcha.go` | MODIFY |
| TASK-007 | `alita/utils/helpers/helpers.go` | MODIFY |
| TASK-007 | `alita/modules/helpers.go` | MODIFY |
| TASK-007 | `alita/modules/locks.go` | MODIFY |
| TASK-008 | `locales/en.yml` | MODIFY |
| TASK-008 | `locales/es.yml` | MODIFY |
| TASK-008 | `locales/fr.yml` | MODIFY |
| TASK-008 | `locales/hi.yml` | MODIFY |
| TASK-009 | `alita/db/greetings_db.go` | MODIFY |
| TASK-009 | `alita/modules/greetings.go` | MODIFY |
| TASK-010 | `alita/db/antiflood_db.go` | MODIFY |
| TASK-010 | `alita/modules/antiflood.go` | MODIFY |
| TASK-011 | `alita/db/lang_db.go` | MODIFY |
| TASK-011 | `alita/db/pin_db.go` | MODIFY |
| TASK-011 | `alita/modules/language.go` | MODIFY |
| TASK-011 | `alita/modules/pins.go` | MODIFY |
| TASK-012 | `alita/db/warns_db.go` | MODIFY |
| TASK-012 | `alita/db/filters_db.go` | MODIFY |
| TASK-012 | `alita/modules/warns.go` | MODIFY |
| TASK-012 | `alita/modules/filters.go` | MODIFY |
| TASK-013 | (none -- verification only) | -- |
| TASK-014 | (none -- verification only) | -- |

**Total unique files modified:** 22 (plus 4 locale files = 26)

---

## Parallelism Matrix

Tasks with **no file overlap** can execute in parallel:

| Parallel Group | Tasks | Rationale |
|---------------|-------|-----------|
| Group A | TASK-001, TASK-002, TASK-003, TASK-004, TASK-005, TASK-006, TASK-008 | All touch unique files, zero overlap |
| Group B | TASK-007, TASK-009, TASK-010, TASK-011, TASK-012 | TASK-007 touches helpers.go/locks.go (no overlap with DB tasks). TASK-009-012 each touch unique DB+module file pairs. All depend on TASK-008 for i18n keys (except TASK-007). |
| Group C | TASK-013 | Depends on TASK-009-012 |
| Group D | TASK-014 | Depends on ALL |

Maximum parallelism: 7 tasks simultaneously in Group A.

---

## Risk Register

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| US-005: Missing a DB caller in a module file | LOW | `go build` fails | TASK-013 is a dedicated build gate. Grep for all function names across the entire codebase before finalizing. |
| US-005: Partial completion breaks build | HIGH (by design) | Build broken until all 4 sub-tasks complete | TASK-013 verifies build. Sub-tasks use `go vet` not `go build` for individual verification. |
| US-008: Undiscovered caller relying on implicit mutation | LOW | Wrong chat context in edge case | Design audited all 40 callers. Only 2 Pattern D callers in locks.go need fixing. |
| US-006: config.yml breaks check-translations | LOW | CI failure | The check-translations script already skips non-language codes. Verify with `make check-translations`. |
| US-013: time.AfterFunc goroutine leak under high load | LOW | Goroutine accumulation | `time.AfterFunc` goroutines are short-lived (1 delete call). Standard Go timer pool management. |
| i18n keys missing in a locale | MEDIUM | `make check-translations` fails | TASK-008 adds to all 4 locales atomically. TASK-014 verifies. |
| TASK-005 chatlist temp file not cleaned on panic | LOW | Temp file orphaned in /tmp | `defer os.Remove` handles normal paths. OS /tmp cleanup handles orphans. |
