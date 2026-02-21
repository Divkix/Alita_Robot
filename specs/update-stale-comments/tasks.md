# Implementation Tasks: Update Stale Comments

**Date:** 2026-02-20
**Design Source:** design.md
**Total Tasks:** 7
**Slicing Strategy:** vertical (each task = complete feature slice)

All 6 implementation tasks are independent with zero file overlap and can run fully in parallel. TASK-007 is the final verification gate that depends on all prior tasks.

---

## TASK-001: Clean up helpers.go and extraction.go comments

**Complexity:** M
**Files:**
- MODIFY: `/Users/divkix/GitHub/Alita_Robot/alita/utils/helpers/helpers.go` -- replace 4 TODO comments (lines 210, 693-694, 704), fix "statis" typo (line 592), replace "temporary" comment (lines 955-956)
- MODIFY: `/Users/divkix/GitHub/Alita_Robot/alita/utils/extraction/extraction.go` -- fix "pasring" typo (line 94), fix "matchQutes" typo (line 269)
**Dependencies:** None
**Description:**

Make exactly these 7 comment edits across 2 files. No code logic changes.

1. **helpers.go line 210** -- Replace `// TODO: maybe replace in future by msg.GetLink()` with:
   ```go
   // NOTE: msg.GetLink() only works for supergroups/channels. This custom implementation
   // also handles private groups and non-supergroups by constructing the link manually.
   ```

2. **helpers.go lines 693-694** -- Replace the two-line block:
   ```go
   		// TODO: Fix circular dependency with extraction package
   		// For now, use a simple extraction
   ```
   with the single line:
   ```go
   		// Uses inline extraction to avoid circular dependency with the extraction package.
   ```

3. **helpers.go line 704** -- Replace `// TODO: Fix circular dependency with extraction package` with:
   ```go
   		// Uses inline extraction to avoid circular dependency with the extraction package.
   ```

4. **helpers.go line 592** -- Replace `// NOTE: extract statis helper functions` with:
   ```go
   // NOTE: extract status helper functions
   ```

5. **helpers.go lines 955-956** -- Replace the two-line block:
   ```go
   		// temporary variable function until we don't support notes in inline keyboard
   		// will remove non url buttons from keyboard
   ```
   with the single line:
   ```go
   		// buttonUrlFixer filters out non-URL buttons from the keyboard, keeping only valid URL buttons.
   ```

6. **extraction.go line 94** -- Replace:
   ```go
   	// func used to trim newlines from the text, fixes the pasring issues of '\n' before and after text
   ```
   with:
   ```go
   	// trimTextNewline trims leading/trailing newlines to fix parsing issues with '\n' before and after text
   ```

7. **extraction.go line 269** -- Replace:
   ```go
   	// if first character starts with '""' and matchQutes is true
   ```
   with:
   ```go
   	// if first character is a double quote and matchQuotes is true
   ```

**Context to Read:**
- `specs/update-stale-comments/design.md`, sections "File: helpers.go" and "File: extraction.go"
- `/Users/divkix/GitHub/Alita_Robot/alita/utils/helpers/helpers.go` -- lines 208-212, 590-594, 691-706, 953-958
- `/Users/divkix/GitHub/Alita_Robot/alita/utils/extraction/extraction.go` -- lines 92-96, 267-271

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && go build ./... && go vet ./... && grep -rn "pasring\|statis\|matchQutes" alita/ && echo "FAIL: typos still present" || echo "OK: no typos found" && grep -rn "temporary" alita/utils/helpers/helpers.go && echo "FAIL: temporary still present" || echo "OK: no temporary found" && grep -c "TODO" alita/utils/helpers/helpers.go && echo "helpers.go TODO count above (expect 0)"
```

---

## TASK-002: Remove dead commented-out code from notes.go, greetings.go, and manager.go

**Complexity:** S
**Files:**
- MODIFY: `/Users/divkix/GitHub/Alita_Robot/alita/modules/notes.go` -- remove misleading comment (line 33), remove dead code block (lines 958-960)
- MODIFY: `/Users/divkix/GitHub/Alita_Robot/alita/modules/greetings.go` -- remove dead commented-out error handling (lines 752-759)
- MODIFY: `/Users/divkix/GitHub/Alita_Robot/alita/i18n/manager.go` -- remove dead commented-out cache clearing block (lines 136-141)
**Dependencies:** None
**Description:**

Remove 3 dead commented-out code blocks and 1 misleading comment. No code logic changes.

1. **notes.go line 33** -- Remove the comment `// overwriteNotesMap is a sync.Map, initialized to zero value (no make needed)` from inside the `notesModule` struct literal. The variable `overwriteNotesMap` does not exist in this struct; the actual variable is `notesOverwriteMap` in `helpers.go`. The struct literal becomes:
   ```go
   var notesModule = moduleStruct{
       moduleName: "Notes",
   }
   ```

2. **notes.go lines 958-960** -- Remove the 3 commented-out lines:
   ```go
   	// if strings.Contains(err.Error(), "replied message not found") {
   	// 	return ext.EndGroups
   	// }
   ```
   The `if err != nil` block immediately after handles this case. Ensure no consecutive blank lines are left.

3. **greetings.go lines 752-759** -- Remove the 8 commented-out lines (the old error handling block) after `db.SetCleanGoodbyeMsgId(chat.Id, sent.MessageId)`. The result should be:
   ```go
               db.SetCleanGoodbyeMsgId(chat.Id, sent.MessageId)
           }
   ```

4. **manager.go lines 136-141** -- Remove the entire 6-line commented-out block (lines 136-141 inclusive, including the blank line after it if present). The result should flow directly:
   ```go
       lm.localeData = make(map[string][]byte)

       return lm.loadLocaleFiles()
   ```

**Context to Read:**
- `specs/update-stale-comments/design.md`, sections "File: notes.go", "File: greetings.go", "File: manager.go"
- `/Users/divkix/GitHub/Alita_Robot/alita/modules/notes.go` -- lines 30-35, 955-965
- `/Users/divkix/GitHub/Alita_Robot/alita/modules/greetings.go` -- lines 748-762
- `/Users/divkix/GitHub/Alita_Robot/alita/i18n/manager.go` -- lines 132-145

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && go build ./... && go vet ./... && grep -rn "overwriteNotesMap" alita/ --include="*.go" && echo "FAIL: overwriteNotesMap still present" || echo "OK: overwriteNotesMap removed"
```

---

## TASK-003: Fix chat_status.go comments and rename misspelled variable

**Complexity:** S
**Files:**
- MODIFY: `/Users/divkix/GitHub/Alita_Robot/alita/utils/chat_status/chat_status.go` -- rename `anonChatMapExpirartion` to `anonChatMapExpiration` (lines 31, 1021), fix misleading channel ID comment (line 44)
**Dependencies:** None
**Description:**

This is the only task with a code change (variable rename). All other changes are comment-only.

1. **Line 31** -- Rename the variable declaration:
   ```go
   // Before:
   anonChatMapExpirartion = 20 * time.Second
   // After:
   anonChatMapExpiration = 20 * time.Second
   ```

2. **Line 1021** -- Update the usage of the renamed variable:
   ```go
   // Before:
   err := cache.Marshal.Set(cache.Context, fmt.Sprintf("alita:anonAdmin:%d:%d", chatId, msg.MessageId), msg, store.WithExpiration(anonChatMapExpirartion))
   // After:
   err := cache.Marshal.Set(cache.Context, fmt.Sprintf("alita:anonAdmin:%d:%d", chatId, msg.MessageId), msg, store.WithExpiration(anonChatMapExpiration))
   ```

3. **Lines 43-44** -- Fix the misleading channel ID digit count:
   ```go
   // Before:
   // Channel IDs have the format -100XXXXXXXXXX (13 digits starting with -100).
   // After:
   // Channel IDs have the format -100XXXXXXXXXX (-100 prefix followed by 10+ digits).
   ```

The variable is unexported (lowercase) and used only in 2 places within this single file. No other files reference it.

**Context to Read:**
- `specs/update-stale-comments/design.md`, section "File: chat_status.go"
- `/Users/divkix/GitHub/Alita_Robot/alita/utils/chat_status/chat_status.go` -- lines 28-47, 1018-1024

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && go build ./... && go vet ./... && grep -rn "anonChatMapExpirartion" alita/ && echo "FAIL: old name still present" || echo "OK: old name gone" && MATCHES=$(grep -c "anonChatMapExpiration" alita/utils/chat_status/chat_status.go) && [ "$MATCHES" -eq 2 ] && echo "OK: exactly 2 references found" || echo "FAIL: expected 2 references, found $MATCHES"
```

---

## TASK-004: Fix formatting.go TODO and dead code

**Complexity:** S
**Files:**
- MODIFY: `/Users/divkix/GitHub/Alita_Robot/alita/modules/formatting.go` -- remove dead commented-out code (line 73) and TODO comment (line 75), replace with factual comment
**Dependencies:** None
**Description:**

Remove the dead `// help.HELPABLE[ModName],` line, the blank line after it, and the `// TODO: Fix help msg here` comment. Replace with a single factual comment.

**Before (lines 72-76):**
```go
        _, err := msg.Reply(b,
            // help.HELPABLE[ModName],

            // TODO: Fix help msg here
            largeOptionsText,
```

**After:**
```go
        _, err := msg.Reply(b,
            // Uses localized largeOptionsText for the formatting help message.
            largeOptionsText,
```

This eliminates 1 of the 4 godox warnings.

**Context to Read:**
- `specs/update-stale-comments/design.md`, section "File: formatting.go"
- `/Users/divkix/GitHub/Alita_Robot/alita/modules/formatting.go` -- lines 70-78

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && go build ./... && go vet ./... && grep -rn "TODO" alita/modules/formatting.go && echo "FAIL: TODO still present" || echo "OK: no TODOs in formatting.go"
```

---

## TASK-005: Fix antiflood.go and purges.go redundant comments

**Complexity:** S
**Files:**
- MODIFY: `/Users/divkix/GitHub/Alita_Robot/alita/modules/antiflood.go` -- remove duplicate godoc line (line 74)
- MODIFY: `/Users/divkix/GitHub/Alita_Robot/alita/modules/purges.go` -- replace change-tracking comment (line 26)
**Dependencies:** None
**Description:**

1. **antiflood.go line 74** -- Remove the duplicate godoc line. Keep lines 75-77 as the single godoc block.

   **Before (lines 74-77):**
   ```go
   // cleanupLoop periodically cleans up old entries from the flood cache
   // cleanupLoop periodically removes old flood control entries from memory.
   // Runs every 5 minutes to clean entries older than 10 minutes.
   // Accepts a context for graceful shutdown.
   ```

   **After:**
   ```go
   // cleanupLoop periodically removes old flood control entries from memory.
   // Runs every 5 minutes to clean entries older than 10 minutes.
   // Accepts a context for graceful shutdown.
   ```

2. **purges.go line 26** -- Replace the change-tracking comment with a purpose description.

   **Before:**
   ```go
   delMsgs      = sync.Map{} // Changed to sync.Map for concurrent access
   ```

   **After:**
   ```go
   delMsgs      = sync.Map{} // Concurrent-safe map for tracking messages to delete
   ```

**Context to Read:**
- `specs/update-stale-comments/design.md`, sections "File: antiflood.go" and "File: purges.go"
- `/Users/divkix/GitHub/Alita_Robot/alita/modules/antiflood.go` -- lines 72-79
- `/Users/divkix/GitHub/Alita_Robot/alita/modules/purges.go` -- lines 24-28

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && go build ./... && go vet ./... && grep -c "cleanupLoop periodically" alita/modules/antiflood.go | grep -q "1" && echo "OK: single godoc line" || echo "FAIL: duplicate godoc still present" && grep "Changed to" alita/modules/purges.go && echo "FAIL: change-tracking comment still present" || echo "OK: purges.go cleaned"
```

---

## TASK-006: Fix db.go, optimized_queries.go, and captcha_db.go comments

**Complexity:** S
**Files:**
- MODIFY: `/Users/divkix/GitHub/Alita_Robot/alita/db/db.go` -- rewrite misleading deprecated constants comment (line 36)
- MODIFY: `/Users/divkix/GitHub/Alita_Robot/alita/db/optimized_queries.go` -- remove hardcoded profiling numbers (lines 29, 134)
- MODIFY: `/Users/divkix/GitHub/Alita_Robot/alita/db/captcha_db.go` -- update dead function comment (lines 303-304)
**Dependencies:** None
**Description:**

1. **db.go line 36** -- Rewrite the misleading comment that references nonexistent functions.

   **Before:**
   ```go
   // Default greeting messages - deprecated constants, use GetDefaultWelcome/GetDefaultGoodbye instead
   ```

   **After:**
   ```go
   // Default greeting messages used when no custom greetings are configured.
   ```

2. **optimized_queries.go line 29** -- Remove hardcoded profiling number.

   **Before:**
   ```go
   // Optimized for high-frequency calls (319K+ calls) by selecting only the locked column.
   ```

   **After:**
   ```go
   // Optimized for high-frequency lock status checks by selecting only the locked column.
   ```

3. **optimized_queries.go line 134** -- Remove hardcoded profiling number.

   **Before:**
   ```go
   // Optimized for high-frequency calls (123K+ calls) by selecting only necessary fields.
   ```

   **After:**
   ```go
   // Optimized for high-frequency calls by selecting only necessary fields.
   ```

4. **captcha_db.go lines 303-304** -- Update comment to note the function is unwired.

   **Before:**
   ```go
   // CleanupExpiredCaptchaAttempts removes all expired captcha attempts from the database.
   // This should be called periodically to clean up old records.
   ```

   **After:**
   ```go
   // CleanupExpiredCaptchaAttempts removes all expired captcha attempts from the database.
   // NOTE: This function is currently not called from anywhere. Wire it into a periodic
   // cleanup goroutine (e.g., monitoring or antiflood cleanup loop) or remove if unneeded.
   ```

**Context to Read:**
- `specs/update-stale-comments/design.md`, sections "File: db.go", "File: optimized_queries.go", "File: captcha_db.go"
- `/Users/divkix/GitHub/Alita_Robot/alita/db/db.go` -- lines 34-40
- `/Users/divkix/GitHub/Alita_Robot/alita/db/optimized_queries.go` -- lines 27-31, 132-136
- `/Users/divkix/GitHub/Alita_Robot/alita/db/captcha_db.go` -- lines 301-308

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && go build ./... && go vet ./... && grep -rn "GetDefaultWelcome\|GetDefaultGoodbye" alita/ --include="*.go" && echo "FAIL: nonexistent function refs still present" || echo "OK: no phantom function refs" && grep -rn "319K\|123K" alita/ --include="*.go" && echo "FAIL: hardcoded numbers still present" || echo "OK: no hardcoded profiling numbers"
```

---

## TASK-007: Full integration verification

**Complexity:** S
**Files:**
- None (verification only)
**Dependencies:** TASK-001, TASK-002, TASK-003, TASK-004, TASK-005, TASK-006
**Description:**

Run the complete verification suite to confirm all changes are correct and no regressions were introduced.

1. **Build and vet:**
   ```bash
   cd /Users/divkix/GitHub/Alita_Robot && go build ./...
   cd /Users/divkix/GitHub/Alita_Robot && go vet ./...
   ```

2. **Run full test suite:**
   ```bash
   cd /Users/divkix/GitHub/Alita_Robot && make test
   ```

3. **Run linter -- expect godox warnings to drop from 4 to 0:**
   ```bash
   cd /Users/divkix/GitHub/Alita_Robot && make lint
   ```

4. **Verify all stale strings are eliminated:**
   ```bash
   # Typos eliminated
   cd /Users/divkix/GitHub/Alita_Robot && grep -rn "pasring\|statis\|matchQutes" alita/
   # Expected: zero results

   # Old variable name eliminated
   cd /Users/divkix/GitHub/Alita_Robot && grep -rn "anonChatMapExpirartion" alita/
   # Expected: zero results

   # New variable name has exactly 2 references
   cd /Users/divkix/GitHub/Alita_Robot && grep -rn "anonChatMapExpiration" alita/utils/chat_status/chat_status.go
   # Expected: exactly 2 results (line 31 declaration, line ~1021 usage)

   # Nonexistent function references eliminated
   cd /Users/divkix/GitHub/Alita_Robot && grep -rn "GetDefaultWelcome\|GetDefaultGoodbye\|overwriteNotesMap" alita/ --include="*.go"
   # Expected: zero results

   # Hardcoded profiling numbers eliminated
   cd /Users/divkix/GitHub/Alita_Robot && grep -rn "319K\|123K" alita/ --include="*.go"
   # Expected: zero results

   # "temporary" label eliminated from helpers.go
   cd /Users/divkix/GitHub/Alita_Robot && grep -rn "temporary" alita/utils/helpers/helpers.go
   # Expected: zero results
   ```

5. **Verify go doc output for corrected comments:**
   ```bash
   cd /Users/divkix/GitHub/Alita_Robot && go doc ./alita/db DefaultWelcome
   cd /Users/divkix/GitHub/Alita_Robot && go doc ./alita/db CleanupExpiredCaptchaAttempts
   ```

6. **Verify acceptance criteria:**
   - [ ] US-001: `make lint` produces zero `godox` warnings
   - [ ] US-002: Zero commented-out code blocks remain in manager.go, greetings.go, notes.go
   - [ ] US-003: No comments reference `GetDefaultWelcome`, `GetDefaultGoodbye`, or `overwriteNotesMap`
   - [ ] US-004: `grep -rn "pasring\|statis\|matchQutes" alita/` returns zero results
   - [ ] US-005: `antiflood.go` has exactly one godoc for `cleanupLoop`; `purges.go` has purpose comment
   - [ ] US-006: No `319K`, `123K`, or `temporary` in comments
   - [ ] US-007: `CleanupExpiredCaptchaAttempts` comment mentions it is unwired
   - [ ] US-008: `anonChatMapExpirartion` fully replaced with `anonChatMapExpiration`

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && go build ./... && go vet ./... && make test && make lint && grep -rn "pasring\|statis\|matchQutes\|anonChatMapExpirartion\|GetDefaultWelcome\|GetDefaultGoodbye\|overwriteNotesMap" alita/ --include="*.go" && echo "FAIL: stale strings found" || echo "ALL CHECKS PASSED"
```

---

## File Manifest

| Task | Files Touched |
|------|---------------|
| TASK-001 | `alita/utils/helpers/helpers.go`, `alita/utils/extraction/extraction.go` |
| TASK-002 | `alita/modules/notes.go`, `alita/modules/greetings.go`, `alita/i18n/manager.go` |
| TASK-003 | `alita/utils/chat_status/chat_status.go` |
| TASK-004 | `alita/modules/formatting.go` |
| TASK-005 | `alita/modules/antiflood.go`, `alita/modules/purges.go` |
| TASK-006 | `alita/db/db.go`, `alita/db/optimized_queries.go`, `alita/db/captcha_db.go` |
| TASK-007 | None (verification only) |

## Parallelism Matrix

TASK-001 through TASK-006 have **zero file overlap** and can all run in parallel. TASK-007 depends on all 6 completing.

```
TASK-001 ──┐
TASK-002 ──┤
TASK-003 ──┼──> TASK-007 (verification gate)
TASK-004 ──┤
TASK-005 ──┤
TASK-006 ──┘
```

## Risk Register

| Task | Risk | Mitigation |
|------|------|------------|
| TASK-003 | Variable rename misses a reference, causing build failure | Only 2 references exist in one file, verified by grep. `go build ./...` catches any miss instantly. |
| TASK-001 | Replacement comment text accidentally contains godox trigger words (TODO, FIXME, BUG, HACK, XXX) | All replacement text uses NOTE prefix and descriptive language. `make lint` in TASK-007 catches regressions. |
| TASK-002 | Removing commented-out code accidentally deletes an adjacent active code line | Each removal is a precise block. `go build ./...` catches any accidental code deletion. |
| TASK-004 | Removing lines 73-75 in formatting.go shifts argument alignment | The replacement comment maintains the same indentation level. Build verification catches syntax errors. |
| ALL | Comment edit accidentally modifies code on an adjacent line | All edits are surgical. `go build ./...` and `make test` in TASK-007 catch any accidental changes. |

TASKS_COMPLETE
