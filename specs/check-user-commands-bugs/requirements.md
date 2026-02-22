# Requirements: Bug Fixes for User-Facing Command Handlers

**Date:** 2026-02-22
**Goal:** Fix all 13 identified bugs in user-facing command handlers across `alita/modules/` to eliminate nil pointer panics, silent data loss, and undefined behavior.
**Source:** research.md (2026-02-22)

---

## Scope

### In Scope
- Fix 4 nil pointer dereference panics (Bugs 1-4)
- Refactor 21 void DB write functions to return `error` and propagate failures to callers (Bug 5)
- Fix i18n config.yml loading so `alt_names` module resolution works (Bug 6)
- Normalize `ExtractUserAndText` return value semantics (Bug 7)
- Eliminate `IsUserConnected` context mutation side effect (Bug 8)
- Replace fire-and-forget goroutine DB calls with error-propagating wrappers (Bug 9)
- Replace custom `HtmlEscape` with stdlib `html.EscapeString` (Bug 10)
- Fix `chatlist.txt` race condition in devs module (Bug 11)
- Add bounded retry with backoff to `secureIntn` (Bug 12)
- Replace blocking `time.Sleep` in `removeBotKeyboard` with non-blocking pattern (Bug 13)
- Edge case analysis for all 13 bugs
- Performance NFRs for logUsers handler
- Backwards compatibility analysis for DB void-to-error refactoring
- Testing requirements for validating all fixes

### Out of Scope
- Adding handler-level integration tests beyond regression tests for fixes
- Changing CI quality gate configuration (gosec, govulncheck exit codes)
- Raising golangci-lint strictness (adding errcheck, nilaway, staticcheck)
- Coverage threshold changes beyond minimum regression tests
- Callback codec 64-byte limit redesign
- New user-facing features or commands
- Performance optimization of existing working code

---

## User Stories

### US-001: Prevent logUsers Panic on Channel Posts (P0)

As the bot runtime, I want the `logUsers` watcher to handle nil `ctx.EffectiveSender` gracefully, so that channel posts do not crash the entire message processing pipeline.

**Acceptance Criteria:**
- [ ] GIVEN a channel post (where `ctx.EffectiveSender` is nil) WHEN the `logUsers` handler at `users.go:73` executes THEN it SHALL return `ext.ContinueGroups` without panic.
- [ ] GIVEN a normal user message WHEN `logUsers` executes THEN existing user tracking behavior is preserved.
- [ ] GIVEN an anonymous admin message WHEN `logUsers` executes THEN the anonymous channel tracking path works as before.

**Edge Cases:**
- EC-001a: Channel post in a group (no sender) -> return `ext.ContinueGroups`
- EC-001b: Channel post with forwarded origin from another channel -> handle nil sender AND forward origin
- EC-001c: Channel post that is a reply to a user message -> skip sender block, safely process `repliedMsg.GetSender()`
- EC-001d: Anonymous admin message (sender exists but `sender.User` is nil) -> `IsAnonymousChannel()` returns false; `user.Id()` works because EffectiveSender is non-nil
- EC-001e: Message from linked channel (auto-forwarded) -> `ctx.EffectiveSender` may be nil
- EC-001g: `repliedMsg.GetSender()` returns nil (reply to channel post with no author signature) -> guard for nil
- EC-001h: `msg.ForwardOrigin.MergeMessageOrigin()` returns struct with both Chat and SenderUser nil -> skip without panic

**NFR: Performance** -- nil-check path SHALL complete in < 1ms, zero heap allocations on nil-sender path.

**Definition of Done:**
- [ ] Nil check for `ctx.EffectiveSender` is the first operation in `logUsers`.
- [ ] Regression test passes nil `EffectiveSender` context and asserts no panic + `ext.ContinueGroups` return.
- [ ] `make lint` passes.

---

### US-002: Prevent purge Callback Handlers Panic on Nil EffectiveUser (P0)

As a group admin using purge functionality, I want the handlers to use the safe `user.Id` from `RequireUser()` return value instead of `ctx.EffectiveUser.Id`.

**Acceptance Criteria:**
- [ ] Lines 149, 335, 419 in `purges.go` use `user.Id` instead of `ctx.EffectiveUser.Id`.
- [ ] Existing purge behavior is identical for normal admin users.

**Edge Cases:**
- EC-002a: Anonymous admin triggers purge -> `user.Id` from `RequireUser()` is the correct resolved user
- EC-002b: Callback from channel account -> `RequireUser()` returns nil first, handler exits before reaching permission check
- EC-002d: `user.Id` differs from `ctx.EffectiveUser.Id` for anonymous admin resolution -> correct ID for permission checks is `user.Id`

**Definition of Done:**
- [ ] All three `ctx.EffectiveUser.Id` replaced with `user.Id`.
- [ ] Regression test verifies no panic when `ctx.EffectiveUser` is nil but `RequireUser()` returns valid user.
- [ ] `make lint` passes.

---

### US-003: Prevent info Command Panic on Channel Messages (P1)

As a user invoking `/info`, I want the bot to respond gracefully when sender is nil.

**Acceptance Criteria:**
- [ ] GIVEN `ctx.EffectiveSender` is nil AND userId == 0 WHEN the `info` handler executes THEN it returns `ext.EndGroups` without panic.
- [ ] GIVEN a valid user invoking `/info` with no arguments THEN handler uses sender's own ID as before.

**Edge Cases:**
- EC-003a: Channel posts the /info command -> sender is nil -> handler returns error/exits
- EC-003d: `/info @nonexistent_user` -> ExtractUser returns -1, handler exits before sender access
- EC-003e: `/info` replying to a channel post -> `IdFromReply` calls `GetSender().Id()` which may panic

**Definition of Done:**
- [ ] Nil sender check before `sender.Id()` at `misc.go:284`.
- [ ] Regression test verifies no panic with nil sender and userId=0.
- [ ] `make lint` passes.

---

### US-004: Prevent echomsg Panic on Nil msg.From (P1)

As a group admin using `/tell`, I want the handler to handle nil `msg.From` gracefully.

**Acceptance Criteria:**
- [ ] GIVEN `msg.From` is nil WHEN `echomsg` handler executes THEN it returns `ext.EndGroups` without panic.

**Edge Cases:**
- EC-004a: Channel-forwarded message triggers /tell -> `msg.From` is nil -> handler exits
- EC-004b: Anonymous admin sends /tell -> `msg.From` is set to group's fake user
- EC-004c: /tell in private chat -> RequireGroup fails first; never reaches line 79

**Definition of Done:**
- [ ] Nil check on `msg.From` before `msg.From.Id` access at `misc.go:79`.
- [ ] Regression test.
- [ ] `make lint` passes.

---

### US-005: DB Write Functions Shall Return Errors (P1)

As a group admin configuring bot settings, I want the bot to inform me when a settings change fails to persist.

**Acceptance Criteria:**
- [ ] All 21 void DB write functions changed to return `error`.
- [ ] All callers in `alita/modules/` check returned error.
- [ ] On error, callers send a generic localized error message and return `ext.EndGroups`.
- [ ] Success messages only sent when DB function returns nil.

**Affected DB functions (21):**
- `greetings_db.go`: SetWelcomeText, SetWelcomeToggle, SetGoodbyeText, SetGoodbyeToggle, SetShouldCleanService, SetShouldAutoApprove, SetCleanWelcomeSetting, SetCleanGoodbyeSetting, SetCleanWelcomeMsgId, SetCleanGoodbyeMsgId (10)
- `antiflood_db.go`: SetFlood, SetFloodMode, SetFloodMsgDel (3)
- `lang_db.go`: ChangeUserLanguage, ChangeGroupLanguage (2)
- `pin_db.go`: SetAntiChannelPin, SetCleanLinked (2)
- `warns_db.go`: SetWarnLimit, SetWarnMode (2)
- `filters_db.go`: AddFilter, RemoveFilter (2)

**Edge Cases:**
- EC-005a: DB connection drops mid-write -> function returns error, user sees error message (not false success)
- EC-005b: Unique constraint violation -> error returned and surfaced
- EC-005d: Disk full on PostgreSQL -> all writes fail with errors surfaced to users
- EC-005e: Cache invalidation succeeds but DB write fails -> self-correcting on next cache expiry

**Migration Path (Backwards Compatibility):**
- All 21 function signatures and all call sites change atomically in one commit
- No DB schema changes; only Go function signatures change
- No external consumers (internal package)

**Definition of Done:**
- [ ] All 21 functions return `error`.
- [ ] All callers check error and handle gracefully.
- [ ] New i18n key `common_settings_save_failed` added to all 4 locale files.
- [ ] `go build ./...` succeeds. `make test` passes. `make check-translations` passes.

---

### US-006: Load config.yml for alt_names Module Resolution (P1)

As a user requesting help by module alternative name (e.g., `/start help_flood`), I want the help system to resolve the alternative name to the canonical module name.

**Acceptance Criteria:**
- [ ] `config.yml` data loadable via `MustNewTranslator("config")`.
- [ ] `getModuleNameFromAltName("flood")` returns `"Antiflood"`.

**Edge Cases:**
- EC-006a: "en" fallback has no `alt_names` -> empty slice -> module resolution fails
- EC-006b: `/start help_filter` (lowercase) -> "filter" != "filters" -> returns "" -> falls through
- EC-006d: Fix loads config.yml as non-language translator -> `check-translations` script must not treat "config" as a language

**Definition of Done:**
- [ ] `loader.go` no longer skips `config.yml`.
- [ ] `MustNewTranslator("config")` returns translator backed by `config.yml` data.
- [ ] Unit test verifies alt_names resolution.
- [ ] `make check-translations` still passes.

---

### US-007: Normalize ExtractUserAndText Return Values (P2)

As a module handler, I want consistent "user not found" semantics (0 instead of -1).

**Acceptance Criteria:**
- [ ] `ExtractUserAndText` returns `(0, "")` for username-not-found (not -1).
- [ ] All callers that check for -1 are updated.

**Edge Cases:**
- EC-007a: Reply to channel post -> channel ID returned (negative), callers must validate
- EC-007b: Reply to service message -> `GetSender()` returns nil -> panic
- EC-007d: `/ban 0` -> interpreted as "self" in some handlers
- EC-007f: UUID-like argument -> UUID parse succeeds, wrong user extracted
- EC-007g: Caller checks `userId == 0` but function returns -1 -> missed error case

**Definition of Done:**
- [ ] Return value changed from -1 to 0 at `extraction.go:130`.
- [ ] All callers audited and updated.
- [ ] Unit tests cover username found, not found, numeric ID, no args, UUID argument.

---

### US-008: Eliminate IsUserConnected Context Mutation (P1)

As a module handler, I want `IsUserConnected` to return the connected chat without mutating `ctx.EffectiveChat`.

**Acceptance Criteria:**
- [ ] Line `ctx.EffectiveChat = chat` at `helpers.go:307` is removed.
- [ ] All callers explicitly assign `ctx.EffectiveChat = connectedChat` at the call site.

**Edge Cases:**
- EC-008a: Handler calls `IsUserConnected()` then logs `ctx.EffectiveChat.Id` -> without fix, logs wrong chat
- EC-008c: `IsUserConnected()` called twice -> without fix, second call sees mutated context
- EC-008d: Connected chat deleted -> `GetChat()` fails, but ctx may already be mutated

**Definition of Done:**
- [ ] Mutation line removed from `IsUserConnected`.
- [ ] All callers in `alita/modules/` verified and updated.
- [ ] Unit test verifies `ctx.EffectiveChat` unchanged after calling `IsUserConnected`.

---

### US-009: Replace Fire-and-Forget Goroutine DB Calls (P1)

As the system, I want all DB write goroutines to have error logging and panic recovery.

**Acceptance Criteria:**
- [ ] `helpers.go:260` wraps `db.ConnectId` with panic recovery and error logging.
- [ ] `connections.go:110-119, 170-173, 326-329` all have panic recovery and error logging.

**Edge Cases:**
- EC-009a: `/connect` succeeds but `go db.ConnectId()` fails -> error logged (previously silent)
- EC-009d: App shutdown during goroutine -> DB write may be interrupted

**Definition of Done:**
- [ ] All 4 fire-and-forget patterns have `defer error_handling.RecoverFromPanic(...)` and error logging.
- [ ] `make lint` passes.

---

### US-010: Replace Custom HtmlEscape with stdlib html.EscapeString (P2)

**Acceptance Criteria:**
- [ ] `helpers.HtmlEscape` replaced with `html.EscapeString` or delegated to it.
- [ ] `"` and `'` now properly escaped.

**Definition of Done:**
- [ ] Function body changed or all call sites updated.
- [ ] Existing tests updated.
- [ ] `make test` passes.

---

### US-011: Fix chatlist.txt Race Condition (P2)

**Acceptance Criteria:**
- [ ] Unique temporary file per invocation (e.g., `os.CreateTemp`).
- [ ] `defer os.Remove(tempFile)` for cleanup.
- [ ] `os.Open` error at `devs.go:123` checked.

**Edge Cases:**
- EC-011a: Two devs invoke simultaneously -> unique temp files prevent corruption
- EC-011b: Read-only filesystem -> error returned to user
- EC-011d: `os.Open` fails -> handled instead of nil reader

**Definition of Done:**
- [ ] `devs.go` uses `os.CreateTemp` instead of fixed filename.
- [ ] Cleanup via `defer`. Error checks added.
- [ ] `make lint` passes.

---

### US-012: Add Bounded Retry to secureIntn (P2)

**Acceptance Criteria:**
- [ ] Maximum 10 retries, then return 0 with error log.

**Edge Cases:**
- EC-012a: No entropy source in container -> bounded loop prevents CPU starvation
- EC-012c: Fails once then succeeds -> returns successful value

**Definition of Done:**
- [ ] Bounded loop (max 10) at `captcha.go:240`.
- [ ] `log.Error` on exhaustion.
- [ ] Unit test verifies termination.

---

### US-013: Replace Blocking time.Sleep in removeBotKeyboard (P2)

**Acceptance Criteria:**
- [ ] Handler returns immediately after sending keyboard-removal message.
- [ ] Deletion scheduled via `time.AfterFunc` or background goroutine with panic recovery.

**Edge Cases:**
- EC-013a: 100 concurrent invocations -> no goroutine pool blocking
- EC-013c: Message deleted before timer fires -> error logged and ignored

**Definition of Done:**
- [ ] `time.Sleep` at `misc.go:461` replaced with non-blocking pattern.
- [ ] `grep -r "time.Sleep" alita/modules/` returns zero results.
- [ ] `make lint` passes.

---

## Non-Functional Requirements

### NFR-001: No New Panics
Zero unrecovered panics in any modified handler for any message type. Verification: `go vet ./...`, `make lint`, manual test with channel posts.

### NFR-002: logUsers Performance
Nil-check path < 1ms p99, zero heap allocations. Verification: benchmark test.

### NFR-003: Panic as DoS Vector
No user-triggerable input shall cause a panic. Fixes at source, not masked by `recover()`.

### NFR-004: DB Error Propagation Latency
p99 latency increase < 100us. Error return is stack-allocated nil in success case.

### NFR-005: DB Signature Change Compatibility
All callers compile. `go build ./...` succeeds. No DB schema changes.

### NFR-006: Concurrent Safety (chatlist)
Safe for concurrent invocation. No race condition. `-race` test passes.

### NFR-007: secureIntn Bounded
Max 10 retries. WARN log on exhaustion.

### NFR-008: No time.Sleep in Handlers
No handler blocks with `time.Sleep`. Delayed operations use `time.AfterFunc` or background goroutine.

### NFR-009: Error Observability
Every error is logged with structured fields (chat ID, user ID).

### NFR-010: Handler Test Coverage
>= 5% handler coverage in `alita/modules/` after fixes (up from 0%). Each fix includes at least one regression test.

### NFR-011: Localized Error Messages
Any new error message uses i18n key present in all 4 locale files. `make check-translations` passes.

---

## Testing Requirements

### TR-001: Nil Sender Regression Tests
Table-driven tests for Bugs 1-4 with nil `EffectiveSender`, `EffectiveUser`, `msg.From`. Verify no panic and correct return value.

### TR-002: DB Error Propagation Tests
At least 3 representative void-to-error functions tested: `SetFlood`, `ChangeUserLanguage`, `SetAntiChannelPin`.

### TR-003: Extraction Edge Case Tests
Tests for `IdFromReply` nil sender, `ExtractUserAndText` sentinel values, negative channel IDs.

### TR-004: Context Mutation Test
Test that `IsUserConnected()` does not mutate `ctx.EffectiveChat`.

### TR-005: Concurrency Tests
`/chatlist` race test with `-race` flag. `secureIntn` bounded retry test. `logUsers` rate limiting concurrency test.

---

## Dependencies

| Dependency | Required By | Risk |
|-----------|------------|------|
| `RequireUser()` pattern | US-002, US-003 | LOW -- exists and works |
| i18n `LocaleManager` singleton | US-006 | LOW -- loader logic change |
| GORM error propagation | US-005 | LOW -- GORM already returns errors |
| `html.EscapeString` (stdlib) | US-010 | NONE |
| `os.CreateTemp` (stdlib) | US-011 | NONE |
| All 4 locale files for new i18n keys | US-005 | MEDIUM -- must add keys |

## Assumptions

1. `ctx.EffectiveSender` CAN be nil for channel posts (defensive checks warranted regardless).
2. `ctx.EffectiveUser` CAN be nil in callback contexts.
3. `msg.From` CAN be nil for channel-authored messages.
4. GORM `DB.Save()` and `DB.Update()` return meaningful errors (confirmed by docs).
5. `config.yml` is intended for i18n module config (evidenced by `MustNewTranslator("config")` calls).
6. No external system depends on `ExtractUserAndText` returning -1 (confirmed internal-only).

## Open Questions

- [ ] **Q1:** Do any callers of `IsUserConnected` rely on ctx mutation without explicit assignment? -> Blocks US-008.
- [ ] **Q2:** Should `config.yml` be loaded as "config" language or via separate mechanism? -> Blocks US-006 approach.
- [ ] **Q3:** Single generic i18n key vs per-module keys for DB error messages? -> Impacts i18n workload.
- [ ] **Q4:** `secureIntn` fallback: `math/rand` (less secure) or 0 (biased)? -> Blocks US-012 implementation.
- [ ] **Q5:** Does `db.ConnectId` return error? -> Partially blocks US-009.
