---
spec: hindi-translation
phase: tasks
total_tasks: 10
created: 2026-01-16
generated: auto
---

# Tasks: Hindi Translation

## Phase 1: Make It Work (POC)

Focus: Create minimal working Hindi locale with core keys.

- [x] 1.1 Create hi.yml with language identity keys
  - **Do**: Create `/Users/divkix/GitHub/Alita_Robot/locales/hi.yml` with language_name, language_flag, lang_sample keys
  - **Files**: `locales/hi.yml`
  - **Done when**: File exists with 3 identity keys
  - **Verify**: `cat locales/hi.yml | head -5`
  - **Commit**: `feat(i18n): add Hindi locale identity keys`
  - _Requirements: FR-2_
  - _Design: Component hi.yml_

- [x] 1.2 Translate admin module keys (~30 keys)
  - **Do**: Translate all keys starting with `admin_` from en.yml
  - **Files**: `locales/hi.yml`
  - **Done when**: All admin_* keys translated
  - **Verify**: `grep -c "^admin_" locales/hi.yml` matches en.yml count
  - **Commit**: `feat(i18n): add Hindi admin module translations`
  - _Requirements: FR-1_

- [x] 1.3 Translate common/utility keys (~100 keys)
  - **Do**: Translate common_, utils_, helpers_, button_, format_ keys
  - **Files**: `locales/hi.yml`
  - **Done when**: All utility keys translated
  - **Verify**: `grep -c "^common_\|^utils_\|^helpers_\|^button_\|^format_" locales/hi.yml`
  - **Commit**: `feat(i18n): add Hindi common utility translations`
  - _Requirements: FR-1_

- [ ] 1.4 POC Checkpoint - Test language selector
  - **Do**: Run bot locally, test `/lang` shows Hindi option
  - **Done when**: Hindi appears in language keyboard with flag
  - **Verify**: Start bot with `make run`, use `/lang` command
  - **Commit**: `feat(i18n): complete Hindi POC with core keys`
  - _Requirements: AC-1.5_

## Phase 2: Complete Translation

- [ ] 2.1 Translate all module help messages
  - **Do**: Translate all *_help_msg keys (major feature docs)
  - **Files**: `locales/hi.yml`
  - **Done when**: All help_msg keys translated (~25 keys)
  - **Verify**: `grep -c "_help_msg:" locales/hi.yml`
  - **Commit**: `feat(i18n): add Hindi module help translations`
  - _Requirements: FR-1_

- [ ] 2.2 Translate remaining module keys
  - **Do**: Translate antiflood_, bans_, blacklists_, captcha_, connections_, disabling_, filters_, greetings_, locks_, misc_, mutes_, notes_, pins_, purges_, reports_, rules_, warns_ keys
  - **Files**: `locales/hi.yml`
  - **Done when**: All module keys translated (~650 remaining keys)
  - **Verify**: `wc -l locales/hi.yml` approximately matches en.yml
  - **Commit**: `feat(i18n): complete Hindi module translations`
  - _Requirements: FR-1_

- [ ] 2.3 Translate extended documentation keys
  - **Do**: Translate all *_extended_docs, *_notes_docs, *_permissions_docs keys
  - **Files**: `locales/hi.yml`
  - **Done when**: All documentation keys translated
  - **Verify**: `grep -c "_docs:" locales/hi.yml`
  - **Commit**: `feat(i18n): add Hindi extended documentation`
  - _Requirements: FR-1_

## Phase 3: Validation

- [ ] 3.1 Validate YAML syntax
  - **Do**: Run YAML linter on hi.yml
  - **Files**: `locales/hi.yml`
  - **Done when**: No YAML syntax errors
  - **Verify**: `yq eval '.' locales/hi.yml > /dev/null && echo "Valid YAML"`
  - **Commit**: `fix(i18n): correct Hindi locale YAML syntax` (if needed)
  - _Requirements: NFR-3_

- [ ] 3.2 Validate key parity with en.yml
  - **Do**: Compare key counts and names between en.yml and hi.yml
  - **Files**: `locales/hi.yml`, `locales/en.yml`
  - **Done when**: Key count matches (818 keys)
  - **Verify**: `diff <(grep "^[a-z_]*:" locales/en.yml | sort) <(grep "^[a-z_]*:" locales/hi.yml | sort)`
  - **Commit**: `fix(i18n): add missing Hindi translation keys` (if needed)
  - _Requirements: FR-5, AC-1.1_

## Phase 4: Quality Gates

- [ ] 4.1 Local quality check
  - **Do**: Run bot locally, test Hindi in private and group contexts
  - **Verify**: `/lang` to select Hindi, test various commands
  - **Done when**: Commands respond in Hindi without format errors
  - **Commit**: `docs(i18n): verify Hindi translation quality`
  - _Requirements: AC-1.2, AC-2.3_

- [ ] 4.2 Create PR and verify CI
  - **Do**: Push branch, create PR with gh CLI
  - **Verify**: `gh pr checks --watch` all green
  - **Done when**: PR ready for review
  - _Requirements: All_

## Notes

- **POC shortcuts taken**: Start with admin + common keys for quick feedback
- **Production TODOs**: Full 818 key translation in Phase 2
- **Translation quality**: Consider native Hindi speaker review before merge
