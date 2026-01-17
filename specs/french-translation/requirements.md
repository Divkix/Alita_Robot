---
spec: french-translation
phase: requirements
created: 2026-01-16
generated: auto
---

# Requirements: french-translation

## Summary

Add French language support to Alita Robot by creating fr.yml locale file with complete translations of all 2191 English strings.

## User Stories

### US-1: French-speaking user uses bot in PM

As a French-speaking user, I want to change the bot language to French so that I can understand all bot messages in my native language.

**Acceptance Criteria**:
- AC-1.1: `/lang` command shows French as an option with 🇫🇷 flag
- AC-1.2: Selecting French changes all bot responses to French
- AC-1.3: Language preference persists across sessions

### US-2: French-speaking admin manages group

As a French-speaking group admin, I want to set the group language to French so that all members see French bot messages.

**Acceptance Criteria**:
- AC-2.1: Admin can change group language via `/lang` command
- AC-2.2: All module help messages display in French
- AC-2.3: Error messages and confirmations show in French

### US-3: Bot displays complete French translations

As a user, I want all bot features to work correctly in French so that no English text appears unexpectedly.

**Acceptance Criteria**:
- AC-3.1: All 2191+ translation keys present in fr.yml
- AC-3.2: No missing keys causing empty responses
- AC-3.3: Printf formatters match English version

## Functional Requirements

| ID | Requirement | Priority | Source |
|----|-------------|----------|--------|
| FR-1 | Create fr.yml with all translation keys from en.yml | Must | US-3 |
| FR-2 | Set language_name to "Français" | Must | US-1 |
| FR-3 | Set language_flag to "🇫🇷" | Must | US-1 |
| FR-4 | Set lang_sample to "Français standard" | Must | US-1 |
| FR-5 | Translate all help messages (*_help_msg keys) | Must | US-2 |
| FR-6 | Translate all error messages | Must | US-3 |
| FR-7 | Translate all confirmation messages | Must | US-2 |
| FR-8 | Translate all button texts | Must | US-1 |
| FR-9 | Preserve HTML tags in translations | Must | US-3 |
| FR-10 | Preserve Printf formatters (%s, %d) | Must | US-3 |
| FR-11 | Use double quotes for escape sequences | Must | US-3 |
| FR-12 | Add "fr" to ENABLED_LOCALES documentation | Should | US-1 |

## Non-Functional Requirements

| ID | Requirement | Category |
|----|-------------|----------|
| NFR-1 | Translation quality - natural French phrasing | Quality |
| NFR-2 | Consistent terminology across all strings | Quality |
| NFR-3 | Use formal "vous" form throughout | Quality |
| NFR-4 | File encoding UTF-8 | Technical |
| NFR-5 | YAML syntax valid and parseable | Technical |

## Translation Categories

| Category | Key Count (approx) | Priority |
|----------|-------------------|----------|
| Admin module | ~50 | High |
| Bans/Mutes/Warns | ~80 | High |
| Common strings | ~60 | High |
| Help messages | ~30 | High |
| Captcha module | ~60 | Medium |
| Greetings module | ~50 | Medium |
| Other modules | ~200 | Medium |
| Extended docs | ~400 | Low |

## Out of Scope

- Crowdin integration setup
- Automated translation validation
- Adding other languages beyond French
- Modifying i18n system code
- RTL language support

## Dependencies

- en.yml as source file for translation
- ENABLED_LOCALES env var must include "fr"
