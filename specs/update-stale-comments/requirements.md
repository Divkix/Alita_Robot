# Requirements: Update Stale Comments

**Date:** 2026-02-20
**Goal:** Eliminate all stale, misleading, incorrect, and dead comments across the Alita_Robot codebase to reduce developer confusion and resolve all `godox` lint warnings.
**Source:** research.md (17 findings across 11 files, verified against source)

## Scope

### In Scope

- Removing all 4 `godox` lint warnings (TODO/FIXME comments in `formatting.go`, `helpers.go`)
- Rewriting misleading or factually incorrect comments (references to nonexistent functions, wrong variable names, incorrect descriptions)
- Removing dead commented-out code blocks (`i18n/manager.go`, `greetings.go`, `notes.go`)
- Fixing typos in comments (`statis`, `pasring`, `matchQutes`)
- Removing duplicate godoc comments (`antiflood.go`)
- Replacing change-tracking and "temporary" comments with descriptive ones
- Removing hardcoded profiling numbers from comments (`optimized_queries.go`)
- Updating the dead `CleanupExpiredCaptchaAttempts` function comment to reflect its unwired status
- Fixing the typo in the unexported variable `anonChatMapExpirartion` (bonus finding, same file as Finding #14)

### Out of Scope

- **Functional code changes** -- This work touches only comments and one unexported variable rename. No logic changes.
- **Deduplicating `IsChannelID`/`IsChannelId`** -- The two functions live in different packages (`helpers` vs `chat_status`), are both exported, and `IsChannelId` in `chat_status` has zero external callers. Deduplication requires an import-graph analysis and touches 20+ call sites. Tracked separately.
- **Wiring up `CleanupExpiredCaptchaAttempts`** -- Whether to integrate this into the monitoring cleanup loop is a functional decision, not a comment cleanup. Tracked separately as a feature issue.
- **Adding "notes in inline keyboard" support** -- The `buttonUrlFixer` function's "temporary" label references a feature that does not exist and has no planned timeline. Out of scope.
- **Refactoring the circular dependency between `helpers` and `extraction`** -- The inline extraction in `helpers.go:693-704` is the permanent solution. Refactoring the package structure is out of scope.
- **Documentation site updates** -- Changes to comments do not require docs site regeneration since generated docs reference module-level descriptions, not inline comments.

## User Stories

### US-001: Remove godox lint warnings

**Priority:** P0 (must-have)

As a developer,
I want all `godox` (TODO/FIXME) lint warnings eliminated,
So that `make lint` output is clean of godox issues and the team can enforce a zero-TODO policy going forward.

**Acceptance Criteria:**

- [ ] GIVEN the codebase after changes WHEN `make lint` is run THEN the output contains `godox: 0` or no godox section at all
- [ ] GIVEN `alita/modules/formatting.go:75` WHEN the TODO comment is removed THEN it is replaced with a factual comment describing the current behavior (e.g., `// Uses localized largeOptionsText for the formatting help message.`) or removed entirely with no blank lines left behind
- [ ] GIVEN `alita/utils/helpers/helpers.go:210` WHEN the TODO is replaced THEN the new comment is a `// NOTE:` explaining why `msg.GetLink()` is insufficient (only works for supergroups/channels, does not handle private groups or non-supergroups)
- [ ] GIVEN `alita/utils/helpers/helpers.go:693` and `:704` WHEN both TODO comments are replaced THEN each is rewritten as `// Uses inline extraction to avoid circular dependency with the extraction package.`
- [ ] GIVEN the above changes WHEN the commented-out line `// help.HELPABLE[ModName],` at `formatting.go:73` is present THEN it is also removed (dead code, not a standalone TODO but associated with the same block)

**Edge Cases:**

- [ ] If any TODO replacement introduces new godox-triggering keywords (TODO, BUG, FIXME, HACK, XXX) -> the lint check SHALL fail, catching the regression

**Definition of Done:**

- [ ] `make lint` produces zero `godox` warnings
- [ ] All 4 original TODO locations contain accurate, non-TODO replacement comments or no comment at all
- [ ] `make test` passes (no functional changes, but verify no accidental breakage)

---

### US-002: Remove dead commented-out code

**Priority:** P0 (must-have)

As a developer,
I want all commented-out code blocks removed from source files,
So that the codebase contains only active code and comments that describe behavior, not ghost code preserved "just in case."

**Acceptance Criteria:**

- [ ] GIVEN `alita/i18n/manager.go:136-141` WHEN the commented-out cache clearing block is removed THEN lines 136-141 are replaced by nothing (or a single blank line for readability), and the surrounding code (`lm.localeData = make(...)` and `return lm.loadLocaleFiles()`) remains intact
- [ ] GIVEN `alita/modules/greetings.go:752-759` WHEN the commented-out error handling block is removed THEN the closing brace of the `if greetPrefs.GoodbyeSettings.CleanGoodbye` block follows directly after `db.SetCleanGoodbyeMsgId(chat.Id, sent.MessageId)`
- [ ] GIVEN `alita/modules/notes.go:958-960` WHEN the commented-out `strings.Contains` block is removed THEN the `if err != nil` check on the line immediately following remains unchanged

**Edge Cases:**

- [ ] If any commented-out block is the only documentation of an error pattern -> the replacement SHALL include a brief inline comment noting the error was handled by the preceding function call (applies to greetings.go where `DeleteMessageWithErrorHandling` now covers that case)
- [ ] If removal creates consecutive blank lines (3+) -> normalize to at most 1 blank line between statements

**Definition of Done:**

- [ ] Zero commented-out code blocks remain in the 3 identified files
- [ ] `go build ./...` succeeds
- [ ] `make test` passes
- [ ] `make lint` produces no new warnings related to these files

---

### US-003: Fix misleading and incorrect comments

**Priority:** P0 (must-have)

As a developer,
I want all comments that reference nonexistent functions, wrong variable names, or incorrect descriptions corrected,
So that reading a comment does not send me on a wild goose chase looking for things that do not exist.

**Acceptance Criteria:**

- [ ] GIVEN `alita/db/db.go:36` WHEN the comment is rewritten THEN it reads `// Default greeting messages used when no custom greetings are configured.` and does NOT mention `GetDefaultWelcome` or `GetDefaultGoodbye`
- [ ] GIVEN `alita/modules/notes.go:33` WHEN the stale comment `// overwriteNotesMap is a sync.Map...` is removed THEN the `notesModule` struct literal has no misleading comment about a variable that lives in a different file
- [ ] GIVEN `alita/utils/chat_status/chat_status.go:44` WHEN the godoc comment is rewritten THEN it reads `// IsChannelId checks if an ID represents a Telegram channel.` followed by `// Channel IDs have the format -100XXXXXXXXXX (-100 prefix followed by 10+ digits).` matching the accuracy of the inline comment on line 46

**Edge Cases:**

- [ ] If the `DefaultWelcome`/`DefaultGoodbye` constants are referenced by godoc tooling or external docs -> the comment rewrite SHALL not break any `go doc` output for the package (verified by running `go doc alita/db DefaultWelcome`)
- [ ] If `notesModule` struct literal becomes an empty `{}` after comment removal -> that is acceptable; Go allows it

**Definition of Done:**

- [ ] All 3 misleading comments are corrected
- [ ] `go doc` output for affected packages produces accurate descriptions
- [ ] `go vet ./...` passes
- [ ] No comment in the codebase references `GetDefaultWelcome`, `GetDefaultGoodbye`, or `overwriteNotesMap`

---

### US-004: Fix typos in comments

**Priority:** P0 (must-have)

As a developer,
I want all typos in code comments corrected,
So that comments are professional and searchable (grep for "status" should find status-related comments).

**Acceptance Criteria:**

- [ ] GIVEN `alita/utils/helpers/helpers.go:592` WHEN `statis` is corrected THEN the comment reads `// NOTE: extract status helper functions`
- [ ] GIVEN `alita/utils/extraction/extraction.go:94` WHEN the comment is rewritten THEN it reads `// trimTextNewline trims leading/trailing newlines to fix parsing issues with '\n' before and after text` (fixing "pasring" -> "parsing" and improving the wording)
- [ ] GIVEN `alita/utils/extraction/extraction.go:269` WHEN the comment is rewritten THEN it reads `// if first character is a double quote and matchQuotes is true` (fixing "matchQutes" -> "matchQuotes" and `'""'` -> accurate description)

**Edge Cases:**

- [ ] If any typo fix changes the semantics of the comment (not just spelling) -> the new comment SHALL be reviewed against the actual code behavior to ensure accuracy

**Definition of Done:**

- [ ] All 3 typos are fixed
- [ ] `grep -rn "pasring\|statis\|matchQutes" alita/` returns zero results
- [ ] No new typos introduced (verified by review)

---

### US-005: Remove duplicate and change-tracking comments

**Priority:** P1 (should-have)

As a developer,
I want duplicate godoc comments and git-history-style change-tracking comments cleaned up,
So that each function has exactly one godoc block and comments describe current state, not past changes.

**Acceptance Criteria:**

- [ ] GIVEN `alita/modules/antiflood.go:74-76` WHEN the duplicate godoc is resolved THEN only ONE godoc comment block remains for `cleanupLoop`, starting with `// cleanupLoop periodically removes old flood control entries from memory.` followed by the detail lines (lines 76-77)
- [ ] GIVEN `alita/modules/purges.go:26` WHEN the change-tracking comment is rewritten THEN it reads `delMsgs = sync.Map{} // Concurrent-safe map for tracking messages to delete`

**Edge Cases:**

- [ ] If `go doc` output for `antiflood.cleanupLoop` currently shows both lines -> after the fix it SHALL show only the retained line
- [ ] Removing the duplicate line SHALL NOT create a gap between the godoc block and the function signature

**Definition of Done:**

- [ ] `antiflood.go` has exactly one godoc comment block for `cleanupLoop`
- [ ] `purges.go` inline comment describes purpose, not history
- [ ] `go doc ./alita/modules/` output is clean for affected functions

---

### US-006: Replace "temporary" and hardcoded-number comments

**Priority:** P1 (should-have)

As a developer,
I want comments that claim code is "temporary" or cite specific profiling numbers updated to reflect reality,
So that I do not waste time investigating whether "temporary" code should be removed or whether profiling numbers are still valid.

**Acceptance Criteria:**

- [ ] GIVEN `alita/utils/helpers/helpers.go:955-956` WHEN the "temporary" comment is rewritten THEN it reads `// buttonUrlFixer filters out non-URL buttons from the keyboard, keeping only valid URL buttons.`
- [ ] GIVEN `alita/db/optimized_queries.go:29` WHEN the profiling number is removed THEN the comment reads `// GetLockStatus retrieves only the lock status for a specific lock type.` followed by `// Optimized for high-frequency lock status checks by selecting only the locked column.`
- [ ] GIVEN `alita/db/optimized_queries.go:134` WHEN the profiling number is removed THEN the comment reads `// GetChatBasicInfo retrieves only essential chat information with minimal column selection.` followed by `// Optimized for high-frequency calls by selecting only necessary fields.`

**Edge Cases:**

- [ ] The profiling numbers SHALL NOT appear anywhere in the codebase after changes (grep for `319K` and `123K` returns zero results in `.go` files)
- [ ] The word "temporary" SHALL NOT appear in any comment in `helpers.go` after changes

**Definition of Done:**

- [ ] All 3 comments rewritten with accurate, timeless language
- [ ] `grep -rn "temporary\|319K\|123K" alita/` returns zero results in `.go` files
- [ ] `make lint` passes

---

### US-007: Update dead function comment (CleanupExpiredCaptchaAttempts)

**Priority:** P1 (should-have)

As a developer,
I want the `CleanupExpiredCaptchaAttempts` function's comment to accurately reflect that it is currently unused,
So that the next developer who encounters it knows to either wire it up or remove it, rather than assuming it is called somewhere.

**Acceptance Criteria:**

- [ ] GIVEN `alita/db/captcha_db.go:303-304` WHEN the comment is updated THEN it reads:
  ```
  // CleanupExpiredCaptchaAttempts removes all expired captcha attempts from the database.
  // NOTE: This function is currently not called from anywhere. Wire it into a periodic
  // cleanup goroutine (e.g., monitoring or antiflood cleanup loop) or remove if unneeded.
  ```
- [ ] GIVEN the function is kept WHEN `go vet ./...` is run THEN no "unused" warnings appear (it is exported, so vet will not flag it)

**Edge Cases:**

- [ ] If the function is removed instead of annotated -> all references to it (including this requirements doc) become moot. This is an acceptable alternative outcome, but the decision SHALL be documented in the commit message
- [ ] If the function is wired up (called periodically) -> the "NOTE: not called" comment SHALL be removed as part of the wiring-up work, not left stale

**Definition of Done:**

- [ ] The comment on `CleanupExpiredCaptchaAttempts` accurately describes its current wiring status
- [ ] `go build ./...` succeeds
- [ ] A follow-up issue is filed to decide: wire up or delete

---

### US-008: Fix variable name typo (anonChatMapExpirartion)

**Priority:** P1 (should-have)

As a developer,
I want the unexported variable `anonChatMapExpirartion` renamed to `anonChatMapExpiration`,
So that the variable name is correctly spelled and grep-discoverable.

**Acceptance Criteria:**

- [ ] GIVEN `alita/utils/chat_status/chat_status.go:31` WHEN the variable is renamed THEN all references to `anonChatMapExpirartion` in the file are updated to `anonChatMapExpiration`
- [ ] GIVEN the variable is unexported (lowercase) WHEN all references are in the same file THEN no other files need updating

**Edge Cases:**

- [ ] If the variable is referenced via reflection or string-based lookup -> renaming would break it. Verified: it is used only as a direct Go identifier (line 31 declaration, line 1021 usage). No reflection risk.
- [ ] If any tests reference the old name -> they SHALL be updated. Verified: no test files reference this variable.

**Definition of Done:**

- [ ] `grep -rn "anonChatMapExpirartion" alita/` returns zero results
- [ ] `grep -rn "anonChatMapExpiration" alita/utils/chat_status/chat_status.go` returns exactly 2 results (declaration + usage)
- [ ] `go build ./...` succeeds
- [ ] `make test` passes

---

## Non-Functional Requirements

### NFR-001: Zero new lint warnings

- **Metric:** `make lint` output after all changes SHALL produce equal or fewer total warnings than before. Specifically: `godox: 0` (down from 4), `dupl` count unchanged at 63.
- **Verification:** Run `make lint` before and after, diff the warning counts.

### NFR-002: No functional behavior change

- **Metric:** The bot's runtime behavior SHALL be identical before and after this change. No handler logic, database queries, cache operations, or API calls are modified.
- **Verification:** `make test` passes. Manual smoke test (optional): start bot, verify `/ping` and `/help` respond.

### NFR-003: Clean git history

- **Metric:** All changes SHALL be in a single commit (or logically grouped atomic commits per category: dead-code-removal, typo-fixes, comment-rewrites, variable-rename) using conventional commit format.
- **Verification:** `git log --oneline` shows commit(s) with `fix(comments):` or `refactor(comments):` prefix.

### NFR-004: Compilation integrity

- **Metric:** `go build ./...` and `go vet ./...` SHALL pass with zero errors.
- **Verification:** Run both commands after all changes.

## Dependencies

| Dependency | Required By | Risk if Unavailable |
|-----------|------------|-------------------|
| `golangci-lint` (with `godox` enabled) | US-001 (verification) | Cannot verify godox warning elimination. MEDIUM risk -- fallback is manual grep for TODO/FIXME. |
| `go build` toolchain (Go 1.25+) | All stories (verification) | Cannot verify compilation. HIGH risk -- blocking. |
| Git (clean working tree) | NFR-003 | Cannot create clean commits. LOW risk -- stash existing work. |

## Assumptions

1. **The 4 `godox` warnings in `make lint` are the only TODO/FIXME comments** -- if false, additional stories are needed. Impact: minor scope increase.
2. **`CleanupExpiredCaptchaAttempts` is accidentally dead code, not an intentional stub** -- if false, the follow-up issue (US-007) should be prioritized as a feature request to wire it up. Impact: scope increase for a separate story.
3. **No external tooling parses these specific comment strings** -- if false (e.g., a doc generator relies on the exact text "deprecated constants"), the rewrite could break downstream tools. Impact: requires investigation before rewriting. Likelihood: very low.
4. **The `anonChatMapExpirartion` variable is only referenced within its declaring file** -- verified true as of 2026-02-20. Impact: if false, additional files need updating.

## Open Questions

- [ ] **Is `CleanupExpiredCaptchaAttempts` wanted?** -- Blocks final disposition in US-007. If the answer is "wire it up," a separate implementation story is needed. If "delete it," US-007 changes from comment-update to function-removal.
- [ ] **Should the `IsChannelId` function in `chat_status` be removed?** -- It has zero callers (verified). Removing it would clean up the duplicate, but it is out of scope for this comment-cleanup work. Does not block any story here. Filed as separate work.

## Glossary

| Term | Definition |
|------|-----------|
| godox | A golangci-lint linter that flags TODO, FIXME, BUG, HACK, and XXX comments in source code |
| Dead commented-out code | Source code that has been commented out (prefixed with `//`) and is no longer executed. Distinct from documentation comments. |
| Change-tracking comment | A comment that describes a past code change (e.g., "Changed to sync.Map") rather than describing current behavior. This information belongs in git commit history. |
| godoc | Go's documentation generation tool. The first comment block before a function declaration becomes its documentation. Duplicate blocks cause confusion. |
| Unexported variable | A Go variable starting with a lowercase letter, visible only within its declaring package. Renaming it has no cross-package impact. |
| singleflight | A concurrency pattern (from `golang.org/x/sync/singleflight`) that deduplicates concurrent calls for the same key. Referenced in `optimized_queries.go` comments. |

REQUIREMENTS_COMPLETE
