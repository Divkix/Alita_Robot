# Technical Design: Update Stale Comments

**Date:** 2026-02-20
**Requirements Source:** specs/update-stale-comments/requirements.md
**Codebase Conventions:** Comment-only and naming changes. Follows existing Go comment style (godoc for exported symbols, `//` inline comments, `// NOTE:` for explanatory callouts). No new patterns introduced.

## Design Overview

This is a hygiene pass across 11 source files. The work eliminates all 4 `godox` lint warnings, removes 3 blocks of dead commented-out code, corrects 3 misleading comments, fixes 3 comment typos, cleans up 2 redundant comments, replaces 3 stale labels/numbers, updates 1 dead-function comment, and renames 1 misspelled unexported variable.

There are zero behavioral changes. The only code-level change is renaming `anonChatMapExpirartion` to `anonChatMapExpiration` in `chat_status.go` (2 sites: declaration on line 31, usage on line 1021). Everything else is comment text.

The work touches 9 distinct files with no overlapping changes between user stories, making full parallelization possible. All changes are verified against the actual source as of commit `656984f`.

## Component Architecture

There are no new components. This section documents the exact change per file.

### File: `/Users/divkix/GitHub/Alita_Robot/alita/modules/formatting.go`

**Requirement:** US-001 (godox warning removal)
**Changes:**

Line 73 -- Remove the dead commented-out code `// help.HELPABLE[ModName],` and the blank line after it.

Lines 75 -- Remove the TODO comment `// TODO: Fix help msg here`. The code already uses `largeOptionsText` from the locale system; no comment is needed. If context is desired, replace with:

```go
// Uses localized largeOptionsText for the formatting help message.
```

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

---

### File: `/Users/divkix/GitHub/Alita_Robot/alita/utils/helpers/helpers.go`

**Requirement:** US-001, US-004, US-006
**Changes:**

**Line 210 (US-001):** Replace the TODO with a NOTE explaining the limitation.

Before:
```go
// TODO: maybe replace in future by msg.GetLink()
```

After:
```go
// NOTE: msg.GetLink() only works for supergroups/channels. This custom implementation
// also handles private groups and non-supergroups by constructing the link manually.
```

**Lines 693-694 (US-001):** Replace the TODO with factual explanation.

Before:
```go
        // TODO: Fix circular dependency with extraction package
        // For now, use a simple extraction
```

After:
```go
        // Uses inline extraction to avoid circular dependency with the extraction package.
```

**Line 704 (US-001):** Replace the TODO.

Before:
```go
        // TODO: Fix circular dependency with extraction package
```

After:
```go
        // Uses inline extraction to avoid circular dependency with the extraction package.
```

**Line 592 (US-004):** Fix "statis" typo.

Before:
```go
// NOTE: extract statis helper functions
```

After:
```go
// NOTE: extract status helper functions
```

**Lines 955-956 (US-006):** Replace "temporary" label.

Before:
```go
        // temporary variable function until we don't support notes in inline keyboard
        // will remove non url buttons from keyboard
```

After:
```go
        // buttonUrlFixer filters out non-URL buttons from the keyboard, keeping only valid URL buttons.
```

---

### File: `/Users/divkix/GitHub/Alita_Robot/alita/utils/extraction/extraction.go`

**Requirement:** US-004
**Changes:**

**Line 94:** Fix "pasring" typo and improve wording.

Before:
```go
    // func used to trim newlines from the text, fixes the pasring issues of '\n' before and after text
```

After:
```go
    // trimTextNewline trims leading/trailing newlines to fix parsing issues with '\n' before and after text
```

**Line 269:** Fix "matchQutes" typo and clean up description.

Before:
```go
    // if first character starts with '""' and matchQutes is true
```

After:
```go
    // if first character is a double quote and matchQuotes is true
```

---

### File: `/Users/divkix/GitHub/Alita_Robot/alita/i18n/manager.go`

**Requirement:** US-002
**Changes:**

**Lines 136-141:** Remove entire commented-out block.

Before:
```go
    // Clear external cache if available
    // Note: This would clear all cache, not just i18n entries
    // In production, you might want to implement selective clearing
    // if lm.cacheClient != nil {
    //     // TODO: Implement selective cache clearing
    // }
```

After: (blank line removed entirely -- the `lm.localeData = make(...)` line flows directly to `return lm.loadLocaleFiles()`)

```go
    lm.localeData = make(map[string][]byte)

    return lm.loadLocaleFiles()
```

---

### File: `/Users/divkix/GitHub/Alita_Robot/alita/modules/greetings.go`

**Requirement:** US-002
**Changes:**

**Lines 752-759:** Remove commented-out error handling block.

Before:
```go
            db.SetCleanGoodbyeMsgId(chat.Id, sent.MessageId)
            // if err.Error() == "unable to deleteMessage: Bad Request: message to delete not found" {
            // 	log.WithFields(
            // 		log.Fields{
            // 			"chat": chat.Id,
            // 		},
            // 	).Error("error deleting message")
            // 	return ext.EndGroups
            // }
        }
```

After:
```go
            db.SetCleanGoodbyeMsgId(chat.Id, sent.MessageId)
        }
```

---

### File: `/Users/divkix/GitHub/Alita_Robot/alita/modules/notes.go`

**Requirement:** US-002, US-003
**Changes:**

**Line 33 (US-003):** Remove the misleading comment referencing nonexistent `overwriteNotesMap`.

Before:
```go
var notesModule = moduleStruct{
    moduleName: "Notes",
    // overwriteNotesMap is a sync.Map, initialized to zero value (no make needed)
}
```

After:
```go
var notesModule = moduleStruct{
    moduleName: "Notes",
}
```

**Lines 958-960 (US-002):** Remove commented-out error check.

Before:
```go
    })
    // if strings.Contains(err.Error(), "replied message not found") {
    // 	return ext.EndGroups
    // }
    if err != nil {
```

After:
```go
    })
    if err != nil {
```

---

### File: `/Users/divkix/GitHub/Alita_Robot/alita/db/db.go`

**Requirement:** US-003
**Changes:**

**Line 36:** Rewrite misleading comment that references nonexistent functions.

Before:
```go
// Default greeting messages - deprecated constants, use GetDefaultWelcome/GetDefaultGoodbye instead
```

After:
```go
// Default greeting messages used when no custom greetings are configured.
```

---

### File: `/Users/divkix/GitHub/Alita_Robot/alita/utils/chat_status/chat_status.go`

**Requirement:** US-003, US-008
**Changes:**

**Line 31 (US-008):** Rename variable.

Before:
```go
    anonChatMapExpirartion = 20 * time.Second
```

After:
```go
    anonChatMapExpiration = 20 * time.Second
```

**Line 1021 (US-008):** Update usage of renamed variable.

Before:
```go
    err := cache.Marshal.Set(cache.Context, fmt.Sprintf("alita:anonAdmin:%d:%d", chatId, msg.MessageId), msg, store.WithExpiration(anonChatMapExpirartion))
```

After:
```go
    err := cache.Marshal.Set(cache.Context, fmt.Sprintf("alita:anonAdmin:%d:%d", chatId, msg.MessageId), msg, store.WithExpiration(anonChatMapExpiration))
```

**Lines 43-44 (US-003):** Fix misleading godoc on `IsChannelId`.

Before:
```go
// IsChannelId checks if an ID represents a Telegram channel.
// Channel IDs have the format -100XXXXXXXXXX (13 digits starting with -100).
```

After:
```go
// IsChannelId checks if an ID represents a Telegram channel.
// Channel IDs have the format -100XXXXXXXXXX (-100 prefix followed by 10+ digits).
```

---

### File: `/Users/divkix/GitHub/Alita_Robot/alita/modules/antiflood.go`

**Requirement:** US-005
**Changes:**

**Lines 74-77:** Remove duplicate godoc line.

Before:
```go
// cleanupLoop periodically cleans up old entries from the flood cache
// cleanupLoop periodically removes old flood control entries from memory.
// Runs every 5 minutes to clean entries older than 10 minutes.
// Accepts a context for graceful shutdown.
```

After:
```go
// cleanupLoop periodically removes old flood control entries from memory.
// Runs every 5 minutes to clean entries older than 10 minutes.
// Accepts a context for graceful shutdown.
```

---

### File: `/Users/divkix/GitHub/Alita_Robot/alita/modules/purges.go`

**Requirement:** US-005
**Changes:**

**Line 26:** Replace change-tracking comment with purpose description.

Before:
```go
    delMsgs      = sync.Map{} // Changed to sync.Map for concurrent access
```

After:
```go
    delMsgs      = sync.Map{} // Concurrent-safe map for tracking messages to delete
```

---

### File: `/Users/divkix/GitHub/Alita_Robot/alita/db/optimized_queries.go`

**Requirement:** US-006
**Changes:**

**Line 29:** Remove hardcoded profiling number.

Before:
```go
// Optimized for high-frequency calls (319K+ calls) by selecting only the locked column.
```

After:
```go
// Optimized for high-frequency lock status checks by selecting only the locked column.
```

**Line 134:** Remove hardcoded profiling number.

Before:
```go
// Optimized for high-frequency calls (123K+ calls) by selecting only necessary fields.
```

After:
```go
// Optimized for high-frequency calls by selecting only necessary fields.
```

---

### File: `/Users/divkix/GitHub/Alita_Robot/alita/db/captcha_db.go`

**Requirement:** US-007
**Changes:**

**Lines 303-304:** Update comment to note the function is unwired.

Before:
```go
// CleanupExpiredCaptchaAttempts removes all expired captcha attempts from the database.
// This should be called periodically to clean up old records.
```

After:
```go
// CleanupExpiredCaptchaAttempts removes all expired captcha attempts from the database.
// NOTE: This function is currently not called from anywhere. Wire it into a periodic
// cleanup goroutine (e.g., monitoring or antiflood cleanup loop) or remove if unneeded.
```

---

## Data Models

No data model changes. No migrations. No schema changes.

## Data Flow

No data flow changes. All handler logic, database queries, cache operations, and API calls remain identical.

## API Contracts

No API changes. No public interface changes. The only symbol change is the unexported variable `anonChatMapExpirartion` -> `anonChatMapExpiration`, which is package-internal (2 references in the same file).

## Testing Strategy

### Unit Tests

No new tests needed. This is comment/naming hygiene with zero behavioral changes.

| Component | Test | Verification |
|-----------|------|-------------|
| All files | Existing test suite | `make test` passes -- confirms no accidental breakage from edits |

### Integration Tests

No new integration tests needed.

| Flow | Test | Verification |
|------|------|-------------|
| Full build | `go build ./...` | Confirms renamed variable compiles correctly |
| Full vet | `go vet ./...` | Confirms no new vet warnings |

### Verification Commands

```bash
# Step 1: Verify compilation (critical for US-008 variable rename)
cd /Users/divkix/GitHub/Alita_Robot && go build ./...

# Step 2: Verify vet passes
cd /Users/divkix/GitHub/Alita_Robot && go vet ./...

# Step 3: Run full test suite
cd /Users/divkix/GitHub/Alita_Robot && make test

# Step 4: Run lint -- expect godox count to drop from 4 to 0
cd /Users/divkix/GitHub/Alita_Robot && make lint

# Step 5: Verify all stale strings are eliminated
cd /Users/divkix/GitHub/Alita_Robot && grep -rn "pasring\|statis\|matchQutes" alita/
# Expected: zero results

cd /Users/divkix/GitHub/Alita_Robot && grep -rn "anonChatMapExpirartion" alita/
# Expected: zero results

cd /Users/divkix/GitHub/Alita_Robot && grep -rn "anonChatMapExpiration" alita/utils/chat_status/chat_status.go
# Expected: exactly 2 results (line 31 declaration, line 1021 usage)

cd /Users/divkix/GitHub/Alita_Robot && grep -rn "GetDefaultWelcome\|GetDefaultGoodbye\|overwriteNotesMap" alita/
# Expected: zero results

cd /Users/divkix/GitHub/Alita_Robot && grep -rn "319K\|123K" alita/ --include="*.go"
# Expected: zero results

cd /Users/divkix/GitHub/Alita_Robot && grep -rn "temporary" alita/utils/helpers/helpers.go
# Expected: zero results

# Step 6: Verify go doc output for corrected comments
cd /Users/divkix/GitHub/Alita_Robot && go doc ./alita/db DefaultWelcome
cd /Users/divkix/GitHub/Alita_Robot && go doc ./alita/db/captcha_db CleanupExpiredCaptchaAttempts
```

## Parallelization Analysis

### Independent Streams

Every file is modified by at most one user story (no overlapping file edits across stories), so ALL work can be parallelized into independent streams:

- **Stream A (US-001):** `formatting.go`, `helpers.go` (lines 210, 693, 704)
- **Stream B (US-002):** `manager.go`, `greetings.go`, `notes.go` (lines 958-960)
- **Stream C (US-003):** `db.go`, `notes.go` (line 33), `chat_status.go` (lines 43-44)
- **Stream D (US-004):** `helpers.go` (line 592), `extraction.go`
- **Stream E (US-005):** `antiflood.go`, `purges.go`
- **Stream F (US-006):** `helpers.go` (lines 955-956), `optimized_queries.go`
- **Stream G (US-007):** `captcha_db.go`
- **Stream H (US-008):** `chat_status.go` (lines 31, 1021)

**However**, some files appear in multiple streams:
- `helpers.go` -- Streams A, D, F (lines 210/693/704, 592, 955-956)
- `notes.go` -- Streams B, C (lines 958-960, 33)
- `chat_status.go` -- Streams C, H (lines 43-44, 31/1021)

### Shared Resources (Serialization Points)

Given the shared files, a practical grouping is:

| Stream | Files | User Stories |
|--------|-------|-------------|
| **Stream 1** | `helpers.go`, `extraction.go` | US-001 (partial), US-004, US-006 (partial) |
| **Stream 2** | `notes.go`, `greetings.go`, `manager.go` | US-002, US-003 (partial) |
| **Stream 3** | `chat_status.go`, `db.go` | US-003 (partial), US-008 |
| **Stream 4** | `formatting.go` | US-001 (partial) |
| **Stream 5** | `antiflood.go`, `purges.go` | US-005 |
| **Stream 6** | `optimized_queries.go`, `captcha_db.go` | US-006 (partial), US-007 |

All 6 streams can run in parallel with zero file conflicts.

### Sequential Dependencies

None. No stream depends on the output of another. All changes are independent.

## Design Decisions

### Decision: Replace TODOs with factual comments rather than deleting them entirely

- **Context:** US-001 requires eliminating 4 godox warnings. The simplest approach is deleting the TODO lines. The question is whether to leave replacement comments.
- **Options considered:** (a) Delete TODO lines entirely. (b) Replace with factual `// NOTE:` comments explaining the current state.
- **Chosen:** Option (b) -- replace with factual comments. The TODOs exist because the code has non-obvious design decisions (circular dependency workaround, API limitation). Deleting them loses that context.
- **Trade-offs:** Slightly more work, but future developers will not rediscover and re-investigate these decisions.

### Decision: Rename variable rather than leaving a comment about the typo

- **Context:** US-008 involves the misspelled variable `anonChatMapExpirartion`. This is the only non-comment code change.
- **Options considered:** (a) Add a `// NOTE: intentionally misspelled` comment. (b) Rename the variable.
- **Chosen:** Option (b) -- rename. The variable is unexported, used in exactly 2 places in the same file, and has zero reflection risk.
- **Trade-offs:** Tiny risk of missing a reference. Verified: only 2 references exist, both in the same file.

### Decision: Keep CleanupExpiredCaptchaAttempts, update comment only

- **Context:** US-007 notes the function is never called. Options are: delete it, wire it up, or annotate it.
- **Options considered:** (a) Delete the function. (b) Wire it into a cleanup loop. (c) Annotate it as unwired.
- **Chosen:** Option (c) -- annotate. Wiring it up is a functional change (out of scope). Deleting loses a working implementation. Annotating makes the status explicit for the next developer.
- **Trade-offs:** Dead code remains, but is clearly labeled.

## Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Variable rename misses a reference | LOW | Build failure (caught immediately) | Verified only 2 references via grep. `go build ./...` will catch any miss. |
| New comment text accidentally contains godox trigger words (TODO, FIXME, BUG, HACK, XXX) | LOW | Lint regression | Review all replacement text. None of the proposed replacements contain trigger words. `make lint` verification catches regressions. |
| Comment edit accidentally modifies code on an adjacent line | LOW | Runtime behavioral change | All edits are surgical single-line or block replacements. `make test` and `go build ./...` catch accidental code changes. |
| `make lint` godox count does not reach 0 due to undiscovered TODOs | LOW | US-001 acceptance criteria not met | The 4 findings were obtained by running `make lint` against the current codebase. If new TODOs were introduced after research, they are separate work. |
| Removing commented-out code blocks changes line numbers referenced in error logging | NONE | N/A | No runtime log statements reference line numbers of the removed blocks. Go's `runtime.Caller` produces line numbers dynamically. |

## Files Modified Summary

| File (absolute path) | Lines Changed | Type of Change |
|---|---|---|
| `/Users/divkix/GitHub/Alita_Robot/alita/modules/formatting.go` | 73-75 | Comment removal (dead code + TODO) |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/helpers/helpers.go` | 210, 592, 693-694, 704, 955-956 | Comment rewrite (TODO, typo, stale label) |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/extraction/extraction.go` | 94, 269 | Comment typo fix |
| `/Users/divkix/GitHub/Alita_Robot/alita/i18n/manager.go` | 136-141 | Dead commented-out code removal |
| `/Users/divkix/GitHub/Alita_Robot/alita/modules/greetings.go` | 752-759 | Dead commented-out code removal |
| `/Users/divkix/GitHub/Alita_Robot/alita/modules/notes.go` | 33, 958-960 | Misleading comment removal + dead code removal |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/db.go` | 36 | Misleading comment rewrite |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/chat_status/chat_status.go` | 31, 43-44, 1021 | Variable rename + misleading comment fix |
| `/Users/divkix/GitHub/Alita_Robot/alita/modules/antiflood.go` | 74 | Duplicate godoc removal |
| `/Users/divkix/GitHub/Alita_Robot/alita/modules/purges.go` | 26 | Change-tracking comment rewrite |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/optimized_queries.go` | 29, 134 | Hardcoded number removal |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/captcha_db.go` | 303-304 | Dead function comment update |

**Total:** 12 files, ~30 lines changed (mostly deletions and rewrites).

## Commit Strategy

Per NFR-003, use conventional commit format. Recommended: a single commit covering all changes since they are all part of the same cleanup initiative and individually trivial.

```
refactor(comments): remove stale TODOs, dead code, and fix misleading comments

- Remove all 4 godox lint warnings (formatting.go, helpers.go)
- Remove 3 dead commented-out code blocks (manager.go, greetings.go, notes.go)
- Fix 3 misleading comments (db.go, notes.go, chat_status.go)
- Fix 3 comment typos: statis->status, pasring->parsing, matchQutes->matchQuotes
- Remove duplicate godoc on cleanupLoop (antiflood.go)
- Replace change-tracking comment with purpose description (purges.go)
- Remove hardcoded profiling numbers from comments (optimized_queries.go)
- Update CleanupExpiredCaptchaAttempts comment to note it is unwired
- Rename anonChatMapExpirartion -> anonChatMapExpiration (typo fix)
```

DESIGN_COMPLETE
