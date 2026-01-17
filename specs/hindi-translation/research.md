---
spec: hindi-translation
phase: research
created: 2026-01-16
generated: auto
---

# Research: Hindi Translation

## Executive Summary

Adding Hindi locale support requires translating 818 YAML keys from en.yml. The i18n system dynamically loads locale files - new language detected by filename (hi.yml). No code changes needed.

## Codebase Analysis

### Existing Patterns

| File | Pattern |
|------|---------|
| `locales/en.yml` | 818 translation keys, flat YAML structure |
| `locales/es.yml` | Spanish translation following same key structure |
| `locales/config.yml` | Contains alt_names mapping, db defaults |
| `alita/i18n/loader.go` | Auto-loads `*.yml` from locales directory |
| `alita/utils/helpers/helpers.go:428-434` | Uses `language_name` and `language_flag` keys for display |

### Required Keys for Language Identity

```yaml
language_name: "Hindi"      # Display name
language_flag: "🇮🇳"         # Flag emoji
lang_sample: "हिन्दी"        # Sample text for language selector
```

### Dependencies

- No code changes needed - i18n loader auto-discovers locale files
- YAML parser supports UTF-8 (Hindi characters supported)
- Telegram supports Hindi text natively

### Constraints

- Must maintain 100% key parity with en.yml
- Multi-line strings must use YAML `|` or `>` syntax correctly
- Double quotes required for escape sequences (`\n`, `\t`)
- Max 100 chars for certain fields (filter keywords, button text)

## Feasibility Assessment

| Aspect | Assessment | Notes |
|--------|------------|-------|
| Technical Viability | High | i18n system already supports multiple locales |
| Effort Estimate | L | 818 keys to translate |
| Risk Level | Low | No code changes, isolated file |

## Recommendations

1. Copy en.yml structure exactly to maintain key parity
2. Test with actual Hindi Telegram users for natural phrasing
3. Use formal Hindi (Shuddh Hindi) for consistency
4. Preserve all HTML tags and format specifiers (%s, %d)
