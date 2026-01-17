---
spec: french-translation
phase: tasks
total_tasks: 12
created: 2026-01-16
generated: auto
---

# Tasks: french-translation

## Phase 1: Make It Work (POC)

Focus: Create fr.yml with core translations to validate language selection works.

- [x] 1.1 Create fr.yml with metadata and admin module
  - **Do**:
    1. Create `locales/fr.yml`
    2. Add language metadata: `lang_sample`, `language_flag`, `language_name`
    3. Translate admin module strings (~50 keys)
  - **Files**: `locales/fr.yml`
  - **Done when**: File created with all admin_* keys translated
  - **Verify**: `yq eval '.' locales/fr.yml` parses without error
  - **Commit**: `feat(i18n): add French locale with admin module translations`
  - _Requirements: FR-1, FR-2, FR-3, FR-4_
  - _Design: fr.yml Locale File_

- [x] 1.2 Add common strings and language module
  - **Do**:
    1. Translate all `common_*` keys
    2. Translate all `language_*` keys
    3. Translate all `button_*` keys
  - **Files**: `locales/fr.yml`
  - **Done when**: Common strings, language strings, and buttons translated
  - **Verify**: Keys present with `grep -c "^language_\|^common_\|^button_" locales/fr.yml`
  - **Commit**: `feat(i18n): add common and language strings to French locale`
  - _Requirements: FR-8_

- [x] 1.3 Add bans, mutes, warns module translations
  - **Do**:
    1. Translate all `bans_*` keys
    2. Translate all `mutes_*` keys
    3. Translate all `warns_*` keys
  - **Files**: `locales/fr.yml`
  - **Done when**: All moderation module strings translated
  - **Verify**: Key count matches en.yml for these prefixes
  - **Commit**: `feat(i18n): add moderation module strings to French locale`
  - _Requirements: FR-6, FR-7_

- [x] 1.4 Add remaining core modules
  - **Do**:
    1. Translate `blacklists_*`, `captcha_*`, `greetings_*` keys
    2. Translate `filters_*`, `notes_*`, `rules_*` keys
    3. Translate `connections_*`, `disabling_*`, `locks_*` keys
  - **Files**: `locales/fr.yml`
  - **Done when**: All core module strings translated
  - **Verify**: Compare key count with en.yml
  - **Commit**: `feat(i18n): add core module strings to French locale`
  - _Requirements: FR-5, FR-6_

- [x] 1.5 Add utility and system strings
  - **Do**:
    1. Translate `utils_*`, `misc_*`, `help_*` keys
    2. Translate `chat_status_*`, `extraction_*` keys
    3. Translate `pins_*`, `purges_*`, `reports_*` keys
  - **Files**: `locales/fr.yml`
  - **Done when**: All utility strings translated
  - **Verify**: Compare total line count with en.yml
  - **Commit**: `feat(i18n): add utility strings to French locale`
  - _Requirements: FR-6_

- [ ] 1.6 Add extended documentation strings
  - **Do**:
    1. Translate all `*_extended_docs` keys
    2. Translate all `*_notes_docs` keys
    3. Translate all `*_help_msg` keys
  - **Files**: `locales/fr.yml`
  - **Done when**: All documentation strings translated
  - **Verify**: `grep -c "_docs\|_help_msg" locales/fr.yml` matches en.yml
  - **Commit**: `feat(i18n): add extended documentation to French locale`
  - _Requirements: FR-5_

- [ ] 1.7 POC Checkpoint - Test French locale
  - **Do**:
    1. Validate YAML syntax: `yq eval '.' locales/fr.yml`
    2. Compare key count: `wc -l locales/fr.yml locales/en.yml`
    3. Check for missing keys (if tooling available)
  - **Done when**: YAML valid, key count within 5% of en.yml
  - **Verify**: `yq eval 'keys | length' locales/fr.yml` returns similar count to en.yml
  - **Commit**: `feat(i18n): complete French locale translation`

## Phase 2: Refactoring

After POC validated, fix any quality issues.

- [ ] 2.1 Fix escape sequence formatting
  - **Do**:
    1. Find all strings containing `\n` with single quotes
    2. Convert to double quotes
    3. Verify newlines render correctly
  - **Files**: `locales/fr.yml`
  - **Done when**: All escape sequences use double quotes
  - **Verify**: `grep "'\|\\\\n" locales/fr.yml` returns minimal matches
  - **Commit**: `fix(i18n): correct escape sequence quoting in French locale`
  - _Design: Translation Format Patterns_

- [ ] 2.2 Verify printf formatter consistency
  - **Do**:
    1. Extract all keys with %s, %d from en.yml
    2. Compare formatter order in fr.yml
    3. Fix any mismatched formatters
  - **Files**: `locales/fr.yml`
  - **Done when**: All formatters match en.yml pattern
  - **Verify**: Manual review of key samples
  - **Commit**: `fix(i18n): align French locale printf formatters`
  - _Requirements: FR-10_

## Phase 3: Testing

- [ ] 3.1 Validate YAML structure
  - **Do**:
    1. Parse fr.yml with yq or YAML validator
    2. Check for duplicate keys
    3. Verify UTF-8 encoding
  - **Files**: `locales/fr.yml`
  - **Done when**: YAML parses without warnings
  - **Verify**: `yq eval '.' locales/fr.yml > /dev/null && echo "Valid"`
  - **Commit**: `test(i18n): validate French locale YAML structure`

- [ ] 3.2 Compare key coverage
  - **Do**:
    1. Extract all keys from en.yml
    2. Extract all keys from fr.yml
    3. Identify any missing keys
    4. Add missing translations
  - **Files**: `locales/fr.yml`
  - **Done when**: All en.yml keys present in fr.yml
  - **Verify**: Key diff shows no missing keys
  - **Commit**: `fix(i18n): add missing keys to French locale`
  - _Requirements: FR-1_

## Phase 4: Quality Gates

- [ ] 4.1 Local quality check
  - **Do**:
    1. Run `make lint` (if applicable)
    2. Verify YAML is valid
    3. Check line count matches en.yml approximately
  - **Verify**: All checks pass
  - **Done when**: No errors from validation
  - **Commit**: `chore(i18n): finalize French locale` (if needed)

- [ ] 4.2 Create PR
  - **Do**:
    1. Create feature branch if not exists
    2. Push changes
    3. Create PR with description of French locale addition
  - **Verify**: `gh pr checks --watch` all green
  - **Done when**: PR ready for review

## Notes

- **POC shortcuts taken**: None - full translation required
- **Production TODOs**: Update sample.env documentation for ENABLED_LOCALES
- **Translation sources**: Professional French translations, not machine translation
