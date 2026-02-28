---
phase: 01-ground-truth-tooling
plan: 03
subsystem: tooling
tags: [go, inventory, docs-generator, message-watchers, makefile]

requires:
  - phase: 01-ground-truth-tooling
    provides: parseCommands() with MultiCommand support (plan 02)
provides:
  - Canonical command inventory (.planning/INVENTORY.json and INVENTORY.md)
  - parseMessageWatchers() function for message handler extraction
  - Module-to-docs mapping table (25 modules, 3 without docs)
  - make check-docs and make inventory Makefile targets
affects: [phase-2-api-reference, phase-4-operator-docs, phase-5-verification]

tech-stack:
  added: []
  patterns: [inventory generation mode via -inventory flag, multi-pattern regex extraction for message watchers]

key-files:
  created:
    - .planning/INVENTORY.json
    - .planning/INVENTORY.md
  modified:
    - scripts/generate_docs/main.go
    - scripts/generate_docs/parsers.go
    - scripts/generate_docs/parsers_test.go
    - Makefile

key-decisions:
  - "25 module files identified (excluding 7 helper files: helpers.go, moderation_input.go, callback_codec.go, callback_parse_overwrite.go, chat_permissions.go, connections_auth.go, rules_format.go)"
  - "3 modules without docs: devs, help, users. language maps to languages/ (naming mismatch resolved)"
  - "bot_updates marked as internal (non-user-facing)"
  - "Message watcher regex covers 3 patterns: literal groups, variable refs, and multi-line anonymous handlers"

patterns-established:
  - "Inventory JSON schema: module/source_file/internal/commands/callbacks/message_watchers/has_docs_directory/docs_path"
  - "make inventory for regeneration, make check-docs for drift detection"

requirements_completed: [TOOL-03, TOOL-05]

duration: 8min
completed: 2026-02-27
---

# Phase 1 Plan 03: Canonical Inventory + Module-to-Docs Mapping Summary

**Generated canonical command inventory covering 25 modules with 142 commands, 21 callbacks, 8 message watchers, and module-to-docs mapping identifying 3 undocumented modules**

## Performance

- **Duration:** 8 min
- **Started:** 2026-02-27
- **Completed:** 2026-02-27
- **Tasks:** 2
- **Files modified:** 8

## Accomplishments
- `.planning/INVENTORY.json` contains structured data for all 25 module files
- `.planning/INVENTORY.md` has summary table and module-to-docs mapping
- All 8 MultiCommand aliases present in inventory
- `parseMessageWatchers()` extracts 8 watchers using 3 regex patterns
- `make check-docs` and `make inventory` targets added to Makefile
- bot_updates correctly marked as internal

## Task Commits

Each task was committed atomically:

1. **Task 1: Add -inventory flag + parseMessageWatchers + inventory generation** - `e3abc5d` (feat)

## Files Created/Modified
- `.planning/INVENTORY.json` - Machine-consumable canonical inventory (25 modules)
- `.planning/INVENTORY.md` - Human-readable summary with module-to-docs mapping
- `scripts/generate_docs/main.go` - Added -inventory flag, generateInventory(), InventoryModule types
- `scripts/generate_docs/parsers.go` - Added MessageWatcher type and parseMessageWatchers()
- `scripts/generate_docs/parsers_test.go` - 5 new test functions for parseMessageWatchers
- `Makefile` - Added check-docs and inventory targets

## Decisions Made
- Inventory uses 25 module files (7 helper files excluded from module count)
- Module naming mismatches (mute->mutes, language->languages) resolved in docs mapping
- Message watcher parser handles 3 registration patterns for comprehensive extraction

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Message watcher regex too restrictive**
- **Found during:** Task 1 (initial inventory generation showed only 1 watcher)
- **Issue:** Original regex only matched literal handler group numbers, missing variable references and multi-line patterns
- **Fix:** Extended to 3 patterns: literal groups, variable references, multi-line anonymous handlers
- **Files modified:** scripts/generate_docs/parsers.go
- **Verification:** 8 watchers detected vs 7-8 grep matches in codebase
- **Committed in:** e3abc5d (part of task commit)

**2. [Rule 1 - Bug] gosimple lint: unnecessary fmt.Sprintf**
- **Found during:** Task 1 (pre-commit hook failure)
- **Issue:** `sb.WriteString(fmt.Sprintf("..."))` flagged as unnecessary
- **Fix:** Changed to `sb.WriteString("...")`
- **Files modified:** scripts/generate_docs/main.go
- **Verification:** golangci-lint passes
- **Committed in:** e3abc5d (part of task commit)

---

**Total deviations:** 2 auto-fixed (2 bugs)
**Impact on plan:** Both fixes necessary for correctness. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Canonical inventory ready for Phase 2 consumption
- Phase 2 can diff INVENTORY.json against existing docs to find gaps
- Module-to-docs mapping identifies 3 undocumented modules (devs, help, users)
- Phase complete, ready for verification

---
*Phase: 01-ground-truth-tooling*
*Completed: 2026-02-27*
