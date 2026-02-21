# Research: Update Stale Comments

**Date:** 2026-02-20
**Goal:** Identify and catalog all stale, outdated, incorrect, or misleading comments across the Alita_Robot codebase
**Confidence:** HIGH -- Comprehensive search using multiple strategies (TODO/FIXME patterns, typo detection, godoc mismatch, commented-out code, section headers, numeric references, URL checks, dead code detection)

## Executive Summary

The codebase contains 17 distinct comment issues across 11 files. The problems break down into: 4 TODO/FIXME comments (some actionable, some stale), 3 typos in comments, 3 pieces of commented-out dead code, 2 misleading/incorrect comments referencing nonexistent functions or wrong variable names, 2 comments with hardcoded profiling numbers that will become stale, 1 duplicate godoc comment, 1 change-tracking comment that belongs in git history, and 1 "temporary" comment on permanent code. None of these are blocking bugs, but they degrade code comprehension and create confusion for new contributors.

## Findings

### 1. Deprecated constants comment references nonexistent functions

- **File:** `/Users/divkix/GitHub/Alita_Robot/alita/db/db.go`
- **Line:** 36
- **Comment:** `// Default greeting messages - deprecated constants, use GetDefaultWelcome/GetDefaultGoodbye instead`
- **Why stale:** `GetDefaultWelcome` and `GetDefaultGoodbye` functions do not exist anywhere in the codebase. The constants `DefaultWelcome` and `DefaultGoodbye` are actively used in 12+ locations across `greetings_db.go` and `greetings.go`.
- **Suggested action:** REWRITE -- Remove "deprecated" label and the nonexistent function references. Replace with: `// Default greeting messages used when no custom greetings are configured.`

### 2. TODO: Fix help msg here

- **File:** `/Users/divkix/GitHub/Alita_Robot/alita/modules/formatting.go`
- **Line:** 75
- **Comment:** `// TODO: Fix help msg here`
- **Why stale:** This TODO has no description of what needs fixing. The code uses `largeOptionsText` from translations which appears to be working as intended.
- **Suggested action:** INVESTIGATE and either fix the underlying issue or REMOVE the TODO with a descriptive comment about what the current behavior is.

### 3. TODO: maybe replace by msg.GetLink()

- **File:** `/Users/divkix/GitHub/Alita_Robot/alita/utils/helpers/helpers.go`
- **Line:** 210
- **Comment:** `// TODO: maybe replace in future by msg.GetLink()`
- **Why stale:** `gotgbot.Message.GetLink()` exists and is available. However, it returns an empty string for private/group chats (only works for supergroups/channels), while the custom `GetMessageLinkFromMessageId` handles non-supergroups too. The TODO is actionable but needs a caveat.
- **Suggested action:** UPDATE to: `// NOTE: gotgbot's msg.GetLink() only works for supergroups/channels. This custom implementation also handles private groups and non-supergroups.`

### 4. TODO: Fix circular dependency (appears twice)

- **File:** `/Users/divkix/GitHub/Alita_Robot/alita/utils/helpers/helpers.go`
- **Lines:** 693, 704
- **Comment:** `// TODO: Fix circular dependency with extraction package` / `// For now, use a simple extraction`
- **Why stale:** The "simple extraction" is the current implementation. This TODO has been here since the circular dependency was worked around. The workaround is now the de facto implementation.
- **Suggested action:** REWRITE -- Replace both with a factual comment: `// Uses inline extraction to avoid circular dependency with the extraction package.`

### 5. Commented-out TODO for selective cache clearing

- **File:** `/Users/divkix/GitHub/Alita_Robot/alita/i18n/manager.go`
- **Lines:** 138-141
- **Comment:** Commented-out code block with `// TODO: Implement selective cache clearing`
- **Why stale:** This is dead commented-out code with a TODO inside it. It serves no purpose.
- **Suggested action:** REMOVE the entire commented-out block (lines 138-141). If selective cache clearing is desired, track it in an issue, not in commented-out code.

### 6. Commented-out error handling in greetings.go

- **File:** `/Users/divkix/GitHub/Alita_Robot/alita/modules/greetings.go`
- **Lines:** 752-759
- **Comment:** Commented-out error handling for `deleteMessage` failures
- **Why stale:** The code above it (`helpers.DeleteMessageWithErrorHandling`) now handles this case. The commented-out code is the old approach that was replaced.
- **Suggested action:** REMOVE the entire commented-out block.

### 7. Commented-out error handling in notes.go

- **File:** `/Users/divkix/GitHub/Alita_Robot/alita/modules/notes.go`
- **Lines:** 958-960
- **Comment:** Commented-out `strings.Contains(err.Error(), "replied message not found")` check
- **Why stale:** The error handling below it (`if err != nil { log.Error(err); return err }`) is the current approach. The commented-out code is dead.
- **Suggested action:** REMOVE the commented-out block.

### 8. Duplicate godoc comment on cleanupLoop

- **File:** `/Users/divkix/GitHub/Alita_Robot/alita/modules/antiflood.go`
- **Lines:** 74-76
- **Comment:** Two consecutive comment lines that say the same thing:
  - Line 74: `// cleanupLoop periodically cleans up old entries from the flood cache`
  - Line 75: `// cleanupLoop periodically removes old flood control entries from memory.`
- **Why stale:** Duplicate godoc. Line 74 is the old comment, line 75 is the replacement. Both were kept.
- **Suggested action:** REMOVE line 74. Keep lines 75-77 as the single godoc block.

### 9. Stale overwriteNotesMap comment references wrong variable name

- **File:** `/Users/divkix/GitHub/Alita_Robot/alita/modules/notes.go`
- **Line:** 33
- **Comment:** `// overwriteNotesMap is a sync.Map, initialized to zero value (no make needed)`
- **Why stale:** The variable `overwriteNotesMap` does not exist. The actual variable is `notesOverwriteMap` defined as a package-level var in `helpers.go:40`. This comment is inside the `notesModule` struct literal and references a field/variable that was renamed or moved.
- **Suggested action:** REMOVE the comment entirely, since it describes a variable that lives in a different file and has a different name.

### 10. Change-tracking comment on sync.Map

- **File:** `/Users/divkix/GitHub/Alita_Robot/alita/modules/purges.go`
- **Line:** 26
- **Comment:** `delMsgs = sync.Map{} // Changed to sync.Map for concurrent access`
- **Why stale:** "Changed to" is a git-commit-style comment describing a past code change. This belongs in git history, not in source code.
- **Suggested action:** REWRITE to: `delMsgs = sync.Map{} // Concurrent-safe map for tracking messages to delete`

### 11. Typo: "statis" should be "status"

- **File:** `/Users/divkix/GitHub/Alita_Robot/alita/utils/helpers/helpers.go`
- **Line:** 592
- **Comment:** `// NOTE: extract statis helper functions`
- **Why stale:** Typo. "statis" should be "status".
- **Suggested action:** FIX to: `// NOTE: extract status helper functions`

### 12. Typo: "pasring" should be "parsing"

- **File:** `/Users/divkix/GitHub/Alita_Robot/alita/utils/extraction/extraction.go`
- **Line:** 94
- **Comment:** `// func used to trim newlines from the text, fixes the pasring issues of '\n' before and after text`
- **Why stale:** Typo. "pasring" should be "parsing". Also, the comment starts with "func used to" which is describing an anonymous function -- it should be reworded.
- **Suggested action:** REWRITE to: `// trimTextNewline trims leading/trailing newlines to fix parsing issues with '\n' before and after text`

### 13. Typo: "matchQutes" should be "matchQuotes"

- **File:** `/Users/divkix/GitHub/Alita_Robot/alita/utils/extraction/extraction.go`
- **Line:** 269
- **Comment:** `// if first character starts with '""' and matchQutes is true`
- **Why stale:** Two issues: "matchQutes" is a typo for "matchQuotes", and `'""'` (two double quotes) should be `'"'` (a single double quote character).
- **Suggested action:** REWRITE to: `// if first character is a double quote and matchQuotes is true`

### 14. Misleading channel ID digit count in comment

- **File:** `/Users/divkix/GitHub/Alita_Robot/alita/utils/chat_status/chat_status.go`
- **Line:** 44
- **Comment:** `// Channel IDs have the format -100XXXXXXXXXX (13 digits starting with -100).`
- **Why stale:** The wording "13 digits starting with -100" is confusing. `-100XXXXXXXXXX` has the format of `-100` prefix + 10 variable digits = 13 total digits (plus minus sign). The inline comment on line 46 is more accurate: `-100 followed by 10+ digits`.
- **Suggested action:** REWRITE to: `// Channel IDs have the format -100XXXXXXXXXX (-100 prefix followed by 10+ digits).`

### 15. "Temporary" comment on permanent code

- **File:** `/Users/divkix/GitHub/Alita_Robot/alita/utils/helpers/helpers.go`
- **Lines:** 955-956
- **Comment:** `// temporary variable function until we don't support notes in inline keyboard` / `// will remove non url buttons from keyboard`
- **Why stale:** This "temporary" function is actively used and there is no indication that "notes in inline keyboard" support is coming. It has been permanent code for an indeterminate period.
- **Suggested action:** REWRITE to: `// buttonUrlFixer filters out non-URL buttons from the keyboard, keeping only valid URL buttons.`

### 16. Hardcoded profiling numbers in optimized queries

- **File:** `/Users/divkix/GitHub/Alita_Robot/alita/db/optimized_queries.go`
- **Lines:** 29, 134
- **Comments:**
  - Line 29: `// Optimized for high-frequency calls (319K+ calls) by selecting only the locked column.`
  - Line 134: `// Optimized for high-frequency calls (123K+ calls) by selecting only necessary fields.`
- **Why stale:** The specific call counts (319K, 123K) are from a past profiling session. These numbers will vary with usage and are already outdated. They create a false sense of precision.
- **Suggested action:** REWRITE to remove specific numbers:
  - Line 29: `// Optimized for high-frequency lock status checks by selecting only the locked column.`
  - Line 134: `// Optimized for high-frequency calls by selecting only necessary fields.`

### 17. Dead function with aspirational comment

- **File:** `/Users/divkix/GitHub/Alita_Robot/alita/db/captcha_db.go`
- **Lines:** 303-304
- **Comment:** `// CleanupExpiredCaptchaAttempts removes all expired captcha attempts from the database.` / `// This should be called periodically to clean up old records.`
- **Why stale:** The comment says "This should be called periodically" but the function is never called anywhere in the codebase. It's dead code with an aspirational comment.
- **Suggested action:** Either REMOVE the function entirely (it's dead code) or add a periodic caller (e.g., in the cleanup goroutine or monitoring system). The comment should be updated to reflect reality.

## Bonus: Non-Comment Code Issues Found During Research

These are not comment issues but were noticed during the investigation:

| Issue | File | Line | Description |
|-------|------|------|-------------|
| Typo in variable name | `/Users/divkix/GitHub/Alita_Robot/alita/utils/chat_status/chat_status.go` | 31 | `anonChatMapExpirartion` should be `anonChatMapExpiration` |
| Duplicate utility function | `/Users/divkix/GitHub/Alita_Robot/alita/utils/helpers/channel_helpers.go` + `/Users/divkix/GitHub/Alita_Robot/alita/utils/chat_status/chat_status.go` | 5, 45 | `IsChannelID` (helpers) and `IsChannelId` (chat_status) are identical functions in different packages |

## Risks & Conflicts

1. **Removing commented-out code** -- LOW risk. The code is already dead. If functionality is needed later, git history preserves it.
2. **Rewriting godoc comments** -- LOW risk. No functional change, but should be reviewed for accuracy.
3. **Removing dead function (CleanupExpiredCaptchaAttempts)** -- MEDIUM risk. If it's intended to be wired up later, removing it loses the implementation. Better to file an issue.

## Open Questions

- [ ] Is the `CleanupExpiredCaptchaAttempts` function intentionally unwired (awaiting integration) or accidentally dead?
- [ ] Is "notes in inline keyboard" support planned? If so, the `buttonUrlFixer` "temporary" comment might be intentional.
- [ ] Should `IsChannelID` (helpers) and `IsChannelId` (chat_status) be deduplicated? This requires checking all call sites.

## File Inventory

| File | Purpose | Relevance |
|------|---------|-----------|
| `/Users/divkix/GitHub/Alita_Robot/alita/db/db.go` | GORM models and DB setup | Finding #1 (deprecated constants comment) |
| `/Users/divkix/GitHub/Alita_Robot/alita/modules/formatting.go` | Formatting module handlers | Finding #2 (vague TODO) |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/helpers/helpers.go` | Shared helper functions | Findings #3, #4, #11, #15 |
| `/Users/divkix/GitHub/Alita_Robot/alita/i18n/manager.go` | Locale manager singleton | Finding #5 (dead commented-out code) |
| `/Users/divkix/GitHub/Alita_Robot/alita/modules/greetings.go` | Welcome/goodbye handlers | Finding #6 (dead commented-out code) |
| `/Users/divkix/GitHub/Alita_Robot/alita/modules/notes.go` | Notes system handlers | Findings #7, #9 (dead code, wrong var name) |
| `/Users/divkix/GitHub/Alita_Robot/alita/modules/antiflood.go` | Flood control handlers | Finding #8 (duplicate godoc) |
| `/Users/divkix/GitHub/Alita_Robot/alita/modules/purges.go` | Message purge handlers | Finding #10 (change-tracking comment) |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/extraction/extraction.go` | User/chat extraction utils | Findings #12, #13 (typos) |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/chat_status/chat_status.go` | Permission and status checks | Finding #14 (misleading digit count) |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/optimized_queries.go` | Optimized DB query helpers | Finding #16 (hardcoded profiling numbers) |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/captcha_db.go` | Captcha DB operations | Finding #17 (dead function) |

## Raw Notes

The `godox` linter (mentioned in CLAUDE.md as having 4 pre-existing warnings) aligns with the TODOs found:
1. `formatting.go:75` -- TODO: Fix help msg here
2. `helpers.go:210` -- TODO: maybe replace by msg.GetLink()
3. `helpers.go:693` -- TODO: Fix circular dependency
4. `helpers.go:704` -- TODO: Fix circular dependency

All 4 godox warnings are accounted for in this research. The `i18n/manager.go:140` TODO is inside a commented-out block and may not be caught by godox depending on configuration.
