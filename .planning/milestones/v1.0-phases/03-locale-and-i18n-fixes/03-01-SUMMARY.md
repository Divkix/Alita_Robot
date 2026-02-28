---
phase: 03-locale-and-i18n-fixes
plan: 01
subsystem: i18n
tags: [go, yaml, locale, translation, testing]

requires:
  - phase: 01-ground-truth-and-tooling
    provides: check-translations script with path resolution fix
provides:
  - "_test.go file exclusion in check-translations key extraction"
  - "EN locale with all 10 production keys and 7 old keys removed"
affects: [03-02, 05-README-and-final-verification]

tech-stack:
  added: []
  patterns: [test fixture isolation in translation key extraction]

key-files:
  created: []
  modified:
    - scripts/check_translations/main.go
    - scripts/check_translations/main_test.go
    - locales/en.yml

key-decisions:
  - "Renamed old keys in-place rather than add+remove to keep EN locale clean and ordered"
  - "Fixed 'added Added' typo in devs_no_team_users value during rename"

patterns-established:
  - "_test.go exclusion: translation key extraction skips test files to avoid false positives"

requirements_completed: [I18N-01, I18N-03, I18N-06]

duration: 5min
completed: 2026-02-28
---

# Plan 03-01: Fix check-translations _test.go bug + EN locale cleanup Summary

**check-translations now excludes test files (8 false positives eliminated), EN locale has all 10 production keys with 7 obsolete old-named keys removed**

## Performance

- **Duration:** 5 min
- **Started:** 2026-02-28
- **Completed:** 2026-02-28
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- check-translations script excludes `_test.go` files from key extraction, eliminating 8 false-positive missing key reports
- Unit test proves `_test.go` exclusion works: `real_key` extracted from module.go, `test_only_key` excluded from module_test.go
- EN locale contains all 10 production keys matching code-referenced names (4 devs renames, 3 greetings _button->_btn renames, 3 genuinely new keys)
- 7 obsolete old key names removed from EN locale
- Fixed 'added Added' typo in devs_no_team_users value

## Task Commits

Each task was committed atomically:

1. **Task 1: Fix check-translations to exclude _test.go files and add unit test** - `e44144e` (fix)
2. **Task 2: Add 10 missing production keys to EN locale and remove 7 obsolete old keys** - `5baf725` (fix)

## Files Created/Modified
- `scripts/check_translations/main.go` - Added `strings.HasSuffix(path, "_test.go")` guard to WalkDir callback
- `scripts/check_translations/main_test.go` - Added TestExtractTranslationKeys_ExcludesTestFiles
- `locales/en.yml` - 10 new/renamed keys added, 7 old keys removed (838 total keys)

## Decisions Made
- Renamed old keys in-place rather than adding new + removing old separately, keeping the locale file ordered
- Fixed 'added Added' typo in devs_no_team_users during rename (plan specified this correction)

## Deviations from Plan

None - plan executed exactly as written

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- EN locale is clean and ready for plan 03-02 to propagate keys to ES/FR/HI
- check-translations script will accurately report real missing keys (not test fixture noise)

---
*Phase: 03-locale-and-i18n-fixes*
*Completed: 2026-02-28*
