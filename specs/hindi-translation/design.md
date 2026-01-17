---
spec: hindi-translation
phase: design
created: 2026-01-16
generated: auto
---

# Design: Hindi Translation

## Overview

Create `locales/hi.yml` following the existing locale file pattern. No code changes - the i18n loader auto-discovers locale files.

## Architecture

```
locales/
├── config.yml       # Shared config, db defaults
├── en.yml          # English (source)
├── es.yml          # Spanish (existing)
└── hi.yml          # Hindi (NEW)
```

## Components

### Component: hi.yml

**Purpose**: Hindi translation file with all 818 keys

**Structure**:
```yaml
# Language identity (required for language selector)
language_name: "हिन्दी"
language_flag: "🇮🇳"
lang_sample: "भारतीय हिन्दी"

# Module keys follow...
admin_adminlist: "<b>%s</b> में व्यवस्थापक:"
...
```

## Data Flow

1. Bot startup → i18n loader scans `locales/*.yml`
2. User runs `/lang` → Shows all available languages including Hindi
3. User/Admin selects Hindi → Language code "hi" stored in DB
4. Bot fetches translations → Uses hi.yml keys
5. Response sent → Hindi text to Telegram

## Technical Decisions

| Decision | Options | Choice | Rationale |
|----------|---------|--------|-----------|
| File encoding | UTF-8, UTF-16 | UTF-8 | YAML standard, Go native support |
| Hindi script | Devanagari, Romanized | Devanagari | Native script, better UX |
| Formal register | Formal, Informal | Formal | Professional bot context |
| Honorifics | Use, Skip | Skip for general | Consistent with English source |

## File Structure

| File | Action | Purpose |
|------|--------|---------|
| `locales/hi.yml` | Create | Hindi translation file |

## Translation Guidelines

### Format Preservation

```yaml
# CORRECT - format specifiers preserved
admin_demote_success_demote: "%s को सफलतापूर्वक पदावनत किया!"

# CORRECT - HTML tags preserved
admin_adminlist: "<b>%s</b> में व्यवस्थापक:"

# CORRECT - multi-line with escape sequences
rules_for_chat: "Rules for <b>%s</b>:\n\n%s"
```

### Common Translation Patterns

| English | Hindi |
|---------|-------|
| Successfully | सफलतापूर्वक |
| Admin | व्यवस्थापक |
| User | उपयोगकर्ता |
| Chat | चैट |
| Group | समूह |
| Enabled | सक्षम |
| Disabled | अक्षम |
| Ban | प्रतिबंध |
| Kick | निकालना |
| Mute | म्यूट |
| Warning | चेतावनी |

## Error Handling

| Error | Handling | User Impact |
|-------|----------|-------------|
| Missing key in hi.yml | i18n falls back to en.yml | Partial Hindi, some English |
| Invalid YAML syntax | Bot fails to load locale | Bot uses default English |
| Malformed format specifier | Runtime error on translation | Broken message display |

## Existing Patterns to Follow

From `locales/es.yml`:
- Flat key structure (no nesting)
- Double quotes for strings with escape sequences
- Preserve exact key names from en.yml
- Language identity keys at specific positions

From `alita/i18n/loader.go:101-103`:
- Language code extracted from filename (hi.yml → "hi")
- File must be valid YAML
- UTF-8 encoding required

## Validation Checklist

Before completion:
- [ ] All 818 keys present
- [ ] YAML syntax valid (`yq` or `yamllint`)
- [ ] Format specifiers match count with en.yml
- [ ] HTML tags balanced and preserved
- [ ] No untranslated English text (except technical terms)
