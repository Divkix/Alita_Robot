---
spec: french-translation
phase: research
created: 2026-01-16
generated: auto
---

# Research: french-translation

## Executive Summary

Adding French (fr.yml) locale is straightforward - translate ~2191 lines from en.yml following existing patterns. Bot already supports multiple languages (en, es, hi). French adds to this set by creating fr.yml and updating ENABLED_LOCALES env var.

## Codebase Analysis

### Existing Patterns

| File | Pattern |
|------|---------|
| `locales/en.yml` | Base English locale, 2191 lines, source of truth |
| `locales/es.yml` | Spanish translation example |
| `locales/hi.yml` | Hindi translation with proper escape sequences |
| `locales/config.yml` | Default strings and module alt_names |
| `alita/i18n/i18n.go` | LocaleManager loads YAML files, creates translators |
| `alita/utils/helpers/helpers.go` | `GetLangFormat()` reads `language_name` + `language_flag` |
| `alita/config/config.go` | `ValidLangCodes` from `ENABLED_LOCALES` env var |

### i18n System Details

1. **Locale file structure**: Flat YAML key-value pairs
2. **Required metadata keys**: `lang_sample`, `language_flag`, `language_name`
3. **Parameter system**:
   - Named params in code: `{"s": value, "first": name}`
   - Printf formatters in YAML: `%s`, `%d`
4. **Escape sequences**: Must use double quotes for `\n`, `\t`
5. **Multi-line strings**: Use `|` for block scalars, double quotes for inline

### Dependencies

| Dependency | Usage |
|------------|-------|
| `gopkg.in/yaml.v3` | YAML parsing |
| `alita/i18n` | Translation loading/retrieval |
| `ENABLED_LOCALES` env var | Must include "fr" to enable |

### Constraints

- No code changes required - just add fr.yml and configure env
- Bot dynamically loads locales from `locales/` directory
- Locale code must match filename: `fr.yml` -> code `fr`
- All keys from en.yml should be present for completeness

## Feasibility Assessment

| Aspect | Assessment | Notes |
|--------|------------|-------|
| Technical Viability | High | Drop-in locale file, no code changes |
| Effort Estimate | M | 2191 lines to translate |
| Risk Level | Low | Existing pattern well-established |

## Key Translation Considerations

1. **Escape sequences**: Use double quotes for strings with `\n`:
   ```yaml
   # Wrong (literal \n)
   key: 'Line one\nLine two'

   # Correct (actual newline)
   key: "Line one\nLine two"
   ```

2. **Printf formatters**: Match order and type:
   ```yaml
   # %s = string, %d = integer
   banned_for: "Banni %s pour %s"  # user, duration
   ```

3. **HTML tags**: Preserve exactly:
   ```yaml
   success: "Promu <b>%s</b> avec succès !"
   ```

4. **Block scalars**: For help messages:
   ```yaml
   help_msg: |
     *Commandes Admin:*
     × /ban - Bannir un utilisateur
   ```

5. **Markdown in help**: Preserve formatting:
   ```yaml
   help_msg: "*Gras* et `code`"
   ```

## French Language Specifics

| English | French | Notes |
|---------|--------|-------|
| User | Utilisateur | Common term |
| Admin | Admin/Administrateur | Both acceptable |
| Ban | Bannir/Exclure | Context dependent |
| Mute | Rendre muet | Literal translation |
| Kick | Expulser | Standard term |
| Warn | Avertir | Standard term |
| Chat/Group | Discussion/Groupe | Context dependent |

## Recommendations

1. Use en.yml as source - most complete and up-to-date
2. Preserve all key names exactly
3. Use formal "vous" form for politeness
4. Set `language_flag: "🇫🇷"` and `language_name: "Français"`
5. Test with `ENABLED_LOCALES=en,fr` first
