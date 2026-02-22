# Technical Design: Fix 13 Bugs in User-Facing Command Handlers

**Date:** 2026-02-22
**Requirements Source:** requirements.md
**Total Files Modified:** 26 files, 0 new files, 0 database migrations

---

## Design Overview

13 bugs across `alita/modules/` and `alita/db/`. All fixes follow existing codebase patterns:
- Nil guards: same pattern as `antiflood.go`, `antispam.go`, `blacklists.go`
- DB error returns: same pattern as `captcha_db.go:SetCaptchaEnabled`, `devs_db.go:AddSudo`
- Goroutine safety: same pattern as `connections.go:54-57`

Largest change: US-005 (21 void-to-error DB functions + 38+ call sites). Must be atomic.

---

## Design Decisions

| Decision | Chosen | Rationale |
|----------|--------|-----------|
| US-005: i18n key strategy | Single `common_settings_save_failed` | Users don't care which subsystem failed. 21 per-module keys × 4 locales = 84 strings for zero benefit. |
| US-006: config.yml loading | Remove skip block in `loader.go` | `extractLangCode("config.yml")` → `"config"`. `MustNewTranslator("config")` finds it. `check-translations` already skips it. 3-line removal. |
| US-007: ExtractUserAndText sentinel | **Keep -1** (deviation from requirements) | `-1` means "lookup failed AND error already sent to user." Changing to `0` creates ambiguity with "use self." Only fix: `IdFromReply` nil guard. |
| US-008: IsUserConnected approach | Remove mutation, fix only 2 Pattern D callers in `locks.go` | 28/40 callers already assign explicitly. 10 use `connectedChat` directly. 2 discard result. Only 2 in `locks.go` depend on implicit mutation. |
| US-010: HtmlEscape approach | Delegate to `html.EscapeString` | One-line body change. Preserves all call sites. Superset of current escaping. |
| US-012: secureIntn fallback | Return 0 (not math/rand) | `math/rand` fallback gives false sense of security. Return 0 is honestly biased. Error log alerts operators. |
| US-013: time.Sleep replacement | `time.AfterFunc` | Most concise stdlib pattern. Functionally equivalent to goroutine+timer. |

---

## Component Designs

### US-001: logUsers Nil Sender Guard (users.go)

```go
func (moduleStruct) logUsers(bot *gotgbot.Bot, ctx *ext.Context) error {
    // ... existing variable assignments ...
    user := ctx.EffectiveSender
    if user == nil {
        return ext.ContinueGroups  // channel posts have nil sender
    }
    // ... existing logic ...
    // Also guard repliedMsg.GetSender() for nil before calling methods on it
}
```

### US-002: Purge user.Id Fix (purges.go)

3 lines change from `ctx.EffectiveUser.Id` to `user.Id` at lines 149, 335, 419. `user` already exists from `RequireUser()` call above each line.

### US-003: info Nil Sender Guard (misc.go)

```go
case 0:
    if sender == nil {
        return ext.EndGroups
    }
    userId = sender.Id()
```

### US-004: echomsg Nil From Guard (misc.go)

```go
if msg.From == nil {
    return ext.EndGroups
}
```

### US-005: DB Void-to-Error Refactor

**DB function transformation pattern (21 functions across 6 files):**
1. Add `error` return type
2. Change bare `return` after error to `return err`
3. Add `return nil` at function end
4. Early-return no-ops become `return nil`

**Caller pattern (38+ call sites):**
```go
// User-facing commands:
if err := db.SetFlood(chat.Id, 0); err != nil {
    log.Errorf("[Antiflood] SetFlood failed for chat %d: %v", chat.Id, err)
    errText, _ := tr.GetString("common_settings_save_failed")
    _, _ = msg.Reply(bot, errText, helpers.Shtml())
    return ext.EndGroups
}
// ... existing success path ...

// Non-user-facing (join/leave cleanup):
if err := db.SetCleanWelcomeMsgId(chat.Id, sent.MessageId); err != nil {
    log.Warnf("[Greetings] Failed to store clean welcome msg ID for chat %d: %v", chat.Id, err)
}
```

**Affected DB files:**
| File | Functions |
|------|-----------|
| `greetings_db.go` | SetWelcomeText, SetWelcomeToggle, SetGoodbyeText, SetGoodbyeToggle, SetShouldCleanService, SetShouldAutoApprove, SetCleanWelcomeSetting, SetCleanGoodbyeSetting, SetCleanWelcomeMsgId, SetCleanGoodbyeMsgId (10) |
| `antiflood_db.go` | SetFlood, SetFloodMode, SetFloodMsgDel (3) |
| `lang_db.go` | ChangeUserLanguage, ChangeGroupLanguage (2) |
| `pin_db.go` | SetAntiChannelPin, SetCleanLinked (2) |
| `warns_db.go` | SetWarnLimit, SetWarnMode (2) |
| `filters_db.go` | AddFilter, RemoveFilter (2) |

**Affected caller files:** greetings.go, antiflood.go, language.go, pins.go, warns.go, filters.go

### US-006: config.yml Loading (loader.go)

Remove 3-line skip block:
```go
// REMOVE:
if fileName == "config.yml" || fileName == "config.yaml" {
    continue
}
```

### US-007: IdFromReply Nil Guard (extraction.go)

```go
// Line 251:
replySender := prevMessage.GetSender()
if replySender == nil {
    return 0, ""
}
userId = replySender.Id()
```

Keep `-1` sentinel at line 130 unchanged.

### US-008: IsUserConnected Mutation Removal (helpers.go + locks.go)

Remove `ctx.EffectiveChat = chat` at helpers.go:307.

Fix 2 Pattern D callers in `locks.go` (~lines 197, 283):
```go
// BEFORE:
if helpers.IsUserConnected(b, ctx, false, true) == nil { return ext.EndGroups }
// AFTER:
connectedChat := helpers.IsUserConnected(b, ctx, false, true)
if connectedChat == nil { return ext.EndGroups }
ctx.EffectiveChat = connectedChat
```

28 Category A callers already have explicit `ctx.EffectiveChat = connectedChat` (no change needed). 10 Category B callers use `connectedChat` directly (no change needed).

### US-009: helpers.go:260 Goroutine Safety

```go
// BEFORE: go db.ConnectId(user.Id, cochat.Id)
// AFTER:
go func() {
    defer error_handling.RecoverFromPanic("ConnectId", "helpers")
    db.ConnectId(user.Id, cochat.Id)
}()
```

connections.go goroutines already have recovery wrappers -- no changes needed there.

### US-010: HtmlEscape → html.EscapeString (helpers.go)

```go
func HtmlEscape(s string) string {
    return html.EscapeString(s)
}
```

### US-011: chatlist.txt Race Fix (devs.go)

Replace fixed `"chatlist.txt"` with `os.CreateTemp("", "chatlist-*.txt")`. Add `defer os.Remove(tmpFile.Name())`. Check `os.Open` error (currently ignored with `_`).

### US-012: secureIntn Bounded Retry (captcha.go)

```go
const maxRetries = 10
for i := 0; i < maxRetries; i++ {
    n, err := crand.Int(crand.Reader, big.NewInt(int64(max)))
    if err == nil { return int(n.Int64()) }
}
log.Error("[Captcha] secureIntn: exhausted retries for crypto/rand.Int, returning 0")
return 0
```

### US-013: removeBotKeyboard Non-blocking (misc.go)

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

---

## i18n Keys (2 new keys, all 4 locales)

```yaml
# en.yml:
common_settings_save_failed: "Failed to save settings. Please try again later."
devs_chatlist_error: "Failed to generate chat list. Please try again."

# es.yml:
common_settings_save_failed: "Error al guardar la configuracion. Intentalo de nuevo mas tarde."
devs_chatlist_error: "Error al generar la lista de chats. Intentalo de nuevo."

# fr.yml:
common_settings_save_failed: "Echec de la sauvegarde des parametres. Veuillez reessayer plus tard."
devs_chatlist_error: "Echec de la generation de la liste des chats. Veuillez reessayer."

# hi.yml:
common_settings_save_failed: "Settings save karne mein error. Baad mein try karein."
devs_chatlist_error: "Chat list generate karne mein error. Phir try karein."
```

---

## Parallelization Analysis (4 Independent Streams)

### Stream 1: Handler Nil Fixes + misc.go (US-001, US-002, US-003, US-004, US-013)
Files: `users.go`, `purges.go`, `misc.go`

### Stream 2: DB Void-to-Error (US-005) -- LARGEST
Files: 6 DB files + 6 module caller files + 4 locale files (16 files total)
**Must be atomic** -- partial apply breaks `go build`.

### Stream 3: helpers.go Cluster (US-008, US-009, US-010)
Files: `alita/utils/helpers/helpers.go`, `alita/modules/helpers.go`, `alita/modules/locks.go`
Serialize within stream (same file for US-008 and US-010).

### Stream 4: Standalone Fixes (US-006, US-007, US-011, US-012)
Files: `loader.go`, `extraction.go`, `devs.go`, `captcha.go`
Each file unique. All independent.

### Execution Order
Phase 1 (parallel): Streams 1 + 4 (no overlap)
Phase 2: Stream 2 (large, atomic)
Phase 3: Stream 3 (touches module files also in Stream 2)

---

## Complete File Change Inventory

### Stream 1 (5 bugs, 3 files)
| File | Changes |
|------|---------|
| `alita/modules/users.go` | Nil guard for EffectiveSender + repliedMsg.GetSender() |
| `alita/modules/purges.go` | 3x `ctx.EffectiveUser.Id` → `user.Id` |
| `alita/modules/misc.go` | Nil guards in info/echomsg + time.AfterFunc in removeBotKeyboard |

### Stream 2 (1 bug, 16 files)
| File | Changes |
|------|---------|
| `alita/db/greetings_db.go` | 10 functions add `error` return |
| `alita/db/antiflood_db.go` | 3 functions add `error` return |
| `alita/db/lang_db.go` | 2 functions add `error` return |
| `alita/db/pin_db.go` | 2 functions add `error` return |
| `alita/db/warns_db.go` | 2 functions add `error` return |
| `alita/db/filters_db.go` | 2 functions add `error` return |
| `alita/modules/greetings.go` | 16+ call sites: wrap with error check |
| `alita/modules/antiflood.go` | 5 call sites |
| `alita/modules/language.go` | 2 call sites |
| `alita/modules/pins.go` | 4 call sites |
| `alita/modules/warns.go` | 4 call sites |
| `alita/modules/filters.go` | 5 call sites |
| `locales/en.yml` | Add 2 i18n keys |
| `locales/es.yml` | Add 2 i18n keys |
| `locales/fr.yml` | Add 2 i18n keys |
| `locales/hi.yml` | Add 2 i18n keys |

### Stream 3 (3 bugs, 3 files)
| File | Changes |
|------|---------|
| `alita/utils/helpers/helpers.go` | Remove ctx mutation (line 307) + HtmlEscape body |
| `alita/modules/helpers.go` | Wrap `go db.ConnectId` with recovery |
| `alita/modules/locks.go` | 2 callers: add explicit ctx.EffectiveChat assignment |

### Stream 4 (4 bugs, 4 files)
| File | Changes |
|------|---------|
| `alita/i18n/loader.go` | Remove 3-line config.yml skip |
| `alita/utils/extraction/extraction.go` | IdFromReply nil guard |
| `alita/modules/devs.go` | os.CreateTemp + defer cleanup + error checks |
| `alita/modules/captcha.go` | Bounded retry (max 10) |

### Total: 26 files modified

---

## Testing Strategy

| Test | File | Verification |
|------|------|-------------|
| logUsers nil sender | `users_test.go` | No panic, returns ContinueGroups |
| logUsers nil reply sender | `users_test.go` | No panic |
| info nil sender + userId=0 | `misc_test.go` | No panic, returns EndGroups |
| echomsg nil From | `misc_test.go` | No panic, returns EndGroups |
| SetFlood error return | DB integration test | Returns error on failure |
| HtmlEscape quotes | `helpers_test.go` | `"` and `'` properly escaped |
| secureIntn bounded | `captcha_test.go` | Returns 0 after 10 retries |
| IdFromReply nil sender | `extraction_test.go` | Returns (0, ""), no panic |
| IsUserConnected no mutation | `helpers_test.go` | ctx.EffectiveChat unchanged |
| chatlist concurrent | `devs_test.go` | No race under `-race` |

**Verification commands:**
```bash
go build ./...
make lint
make test
make check-translations
grep -r "time.Sleep" alita/modules/  # expect zero results
grep -n "ctx.EffectiveUser.Id" alita/modules/purges.go  # expect zero results
go vet ./...
```

---

## Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| US-005: Missing a caller | LOW | Build failure | `go build ./...` catches immediately |
| US-005: Atomic commit required (16 files) | MEDIUM | Partial apply breaks build | Single commit, build gate before push |
| US-008: Caller relies on implicit mutation | LOW | Wrong chat context | All 40 callers audited; only 2 need fix |
| US-007: Keeping -1 confuses future callers | LOW | Missed error case | Document in function godoc |

DESIGN_COMPLETE
