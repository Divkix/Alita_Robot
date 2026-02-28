---
phase: 03-locale-and-i18n-fixes
plan: 02
subsystem: i18n
tags: [yaml, locale, translation, es, fr, hi]

requires:
  - phase: 03-locale-and-i18n-fixes
    plan: 01
    provides: "Clean EN locale with all 10 production keys and _test.go exclusion in check-translations"
provides:
  - "ES/FR/HI locales aligned with EN at 838 keys each"
  - "ES orphan keys (misc_translate_*) removed"
  - "make check-translations passes clean with zero errors"
affects: [05-README-and-final-verification]

tech-stack:
  added: []
  patterns: []

key-files:
  created: []
  modified:
    - locales/es.yml
    - locales/fr.yml
    - locales/hi.yml

key-decisions:
  - "Used Spanish translations from plan for genuinely new keys rather than EN fallback"
  - "FR and HI genuinely new keys translated to native language (not EN placeholders)"

patterns-established:
  - "All four locales must have identical key sets at 838 keys"

requirements_completed: [I18N-02, I18N-04, I18N-05, I18N-06]

duration: 5min
completed: 2026-02-28
---

# Plan 03-02: Propagate keys to ES/FR/HI + remove orphans Summary

**All four locales aligned at 838 keys, ES orphans removed, `make check-translations` passes clean with zero errors**

## Performance

- **Duration:** 5 min
- **Started:** 2026-02-28
- **Completed:** 2026-02-28
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- ES locale cleaned: 10 old/orphan keys removed, 6 new keys added (842 -> 838)
- FR locale updated: 4 devs + 3 greetings keys renamed, 3 new keys added (835 -> 838)
- HI locale updated: identical treatment to FR (835 -> 838)
- `make check-translations` exits 0 with zero missing translations across all four locales
- All YAML files parse cleanly

## Task Commits

Each task was committed atomically:

1. **Task 1: Propagate new keys to ES/FR/HI and remove orphan/old keys** - `e016c9a` (fix)
2. **Task 2: Run make check-translations and verify clean exit** - verification only, no commit needed

## Files Created/Modified
- `locales/es.yml` - 6 keys added, 10 keys removed (842 -> 838)
- `locales/fr.yml` - 10 keys added/renamed, 7 old keys removed (835 -> 838)
- `locales/hi.yml` - 10 keys added/renamed, 7 old keys removed (835 -> 838)

## Decisions Made
- Used native-language translations for genuinely new keys (ES, FR, HI) rather than EN placeholders
- ES orphan keys (misc_translate_*) removed since no code references them

## Deviations from Plan

None - plan executed exactly as written

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All four locale files are internally consistent and cross-locale aligned
- Phase 3 success criteria #5 met: `make check-translations` passes with 0 errors
- Ready for Phase 4 (Operator Documentation) and Phase 5 (README and Final Verification)

---
*Phase: 03-locale-and-i18n-fixes*
*Completed: 2026-02-28*
