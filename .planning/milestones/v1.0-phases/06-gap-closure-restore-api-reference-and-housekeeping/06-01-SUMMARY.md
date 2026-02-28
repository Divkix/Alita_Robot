---
phase: 06-gap-closure-restore-api-reference-and-housekeeping
plan: 01
subsystem: docs
tags: [markdown, starlight, api-reference, commands, callbacks, sentinel]

requires:
  - phase: 02-api-reference-and-command-documentation
    provides: Original 5-column command format specification and callback codec documentation
provides:
  - Restored commands.md with 142 commands in 5-column format
  - Restored callbacks.md with versioned codec documentation
  - Sentinel comments protecting both files from generator overwrites
affects: [06-02-generator-hardening]

tech-stack:
  added: []
  patterns: [sentinel-comment-protection]

key-files:
  created: []
  modified:
    - docs/src/content/docs/api-reference/commands.md
    - docs/src/content/docs/api-reference/callbacks.md

key-decisions:
  - "Combined both tasks into single commit since both files are part of the same logical restoration"

patterns-established:
  - "Sentinel comment pattern: <!-- MANUALLY MAINTAINED: do not regenerate --> after frontmatter to protect hand-edited docs"

requirements_completed: [DOCS-02, DOCS-03, DOCS-04, DOCS-05, DOCS-07]

duration: 8min
completed: 2026-02-28
---

# Plan 06-01: Restore api-reference files with sentinels

**Restored commands.md (142 commands, 5-column format with Permission column) and callbacks.md (versioned codec format) with sentinel protection against future generator overwrites**

## Performance

- **Duration:** 8 min
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- Rewrote commands.md with 5-column table format (Command|Description|Permission|Disableable|Aliases) for all 142 commands across 25 modules
- Added 13 previously missing commands: start, help, donate, about, stats, addsudo, adddev, remsudo, remdev, chatinfo, chatlist, leavechat, teamusers
- Added all 8 MultiCommand alias rows with "Alias of /primary" notation
- Rewrote callbacks.md with versioned codec format (namespace|v1|url-encoded-fields) including encode/decode Go example
- Added sentinel comments to both files to prevent regeneration

## Task Commits

1. **Task 1+2: Restore commands.md and callbacks.md** - `333c0ab` (docs)

## Files Created/Modified

- `docs/src/content/docs/api-reference/commands.md` - Complete command reference with 142 commands, Permission column, alias notation, sentinel
- `docs/src/content/docs/api-reference/callbacks.md` - Callback reference with versioned codec format, backward compatibility note, sentinel

## Decisions Made

- Combined both tasks (commands.md and callbacks.md restoration) into a single commit since they are logically related and both part of the same regression fix

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Both files now contain sentinel comments. Plan 06-02 (generator hardening) can now add the sentinel skip mechanism to generators.go to prevent future overwrites.
- Docs build passes with no broken links.

---
*Phase: 06-gap-closure-restore-api-reference-and-housekeeping*
*Completed: 2026-02-28*
