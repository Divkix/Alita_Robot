# Phase 3: Locale and i18n Fixes - Research

**Researched:** 2026-02-28
**Domain:** YAML locale files (en/es/fr/hi), check-translations Go script, i18n key naming consistency
**Confidence:** HIGH — derived entirely from live codebase inspection and `make check-translations` output

---

## Summary

Phase 3's goal is to make `make check-translations` pass clean with 0 errors across all four
locales. The original phase description contained inaccurate gap estimates (~5% FR gap, ~18% HI
gap) based on pre-Phase-1 analysis. After Phase 1 fixed the tooling, the actual state is sharply
different and much more tractable.

The real problem has three parts. First, `check-translations` is incorrectly scanning `_test.go`
files and picking up test fixture keys (e.g., `nonexistent_key_xyz`, `fallback_key`, `greet`,
`greeting`, `some_key`, `static`, `templ`, `""`) that intentionally do not exist in locale files.
This is a script bug, not a locale gap. Second, EN locale is missing 10 production keys that
production Go code actually calls. Some are genuinely new (never existed anywhere); others are
victims of a naming inconsistency where the code was updated to use more consistent key names but
EN/FR/HI locales were not updated (while ES was partially updated). Third, ES has 7 orphan keys:
4 are the correctly-named devs keys that need to be propagated to EN/FR/HI, and 3 are unused
`misc_translate_*` keys with no production code references.

**Primary recommendation:** Fix `check-translations` to exclude `_test.go` files first. Then add
the 10 missing production keys to EN as the canonical reference. Then propagate to FR/HI using
existing translations for the rename cases. Then clean up ES orphans.

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| I18N-01 | Fix EN locale key naming inconsistencies (e.g., `devs_getting_chatlist` vs `devs_getting_chat_list`) | EN has old names; code uses new names; ES already has both. Strategy: add new keys to EN/FR/HI, optionally remove old keys. |
| I18N-02 | Remove orphan keys from ES locale that don't exist in EN (7 confirmed orphans) | 4 devs_ orphans are the correct code-facing names (must add to EN first, then remove ES duplicate old names). 3 misc_translate_ orphans have zero code references and must be deleted. |
| I18N-03 | Add missing locale keys to EN that exist in code but not in any locale file | 10 keys total: bans_cannot_identify_user, captcha_internal_error, devs_chat_list_caption, devs_getting_chat_list, devs_no_team_users, devs_no_users, greetings_join_request_approve_btn, greetings_join_request_ban_btn, greetings_join_request_decline_btn, reports_cannot_report_channel |
| I18N-04 | Remediate FR locale gap (~5% missing keys compared to EN) | FR has 0 missing keys vs EN. The apparent gap was the script not working before Phase 1. FR does have 5 YAML block-scalar keys that appear empty in raw grep but have real values when YAML-parsed. No remediation needed for FR key completeness. |
| I18N-05 | Enumerate and remediate HI locale gap (~18% missing keys compared to EN) | HI has 0 missing keys vs EN. The apparent gap was the same pre-Phase-1 false reporting. No missing keys to enumerate or translate. |
| I18N-06 | Verify `make check-translations` passes clean after all locale fixes | Requires: (a) fix script to exclude _test.go; (b) add 10 missing keys to all 4 locales; (c) remove 7 ES orphans. Then `make check-translations` must exit 0. |
</phase_requirements>

---

## Definitive Current State (Ground Truth)

### Key Counts (Python YAML parse, accurate)

| Locale | Keys | Notes |
|--------|------|-------|
| en.yml | 835 | Reference. Missing 10 production keys. |
| es.yml | 842 | 7 orphan keys vs EN. |
| fr.yml | 835 | Parity with EN. Zero missing keys. |
| hi.yml | 835 | Parity with EN. Zero missing keys. |

### What `make check-translations` Reports Right Now

68 total missing translations across all 4 locales. Breakdown of the 18 per locale (EN/FR/HI)
and 14 for ES:

**Test fixture keys (8) — false positives from scanning `_test.go`:**
- `""` (empty string) — `i18n_test.go:676`
- `fallback_key` — `i18n_test.go:631`
- `greet` — `i18n_test.go:646`
- `greeting` — `i18n_test.go:443`
- `nonexistent_key_xyz` — `i18n_test.go:416`
- `some_key` — `i18n_test.go:612,702`
- `static` — `i18n_test.go:661`
- `templ` — `i18n_test.go:429`

**Real production keys missing from ALL 4 locales (10):**
- `bans_cannot_identify_user` — `bans.go:72` (nil `msg.ReplyToMessage.From` guard)
- `captcha_internal_error` — `captcha.go:713` (DB write failure for captcha settings)
- `devs_chat_list_caption` — `devs.go:143` (EN has `devs_chatlist_caption` — naming inconsistency)
- `devs_getting_chat_list` — `devs.go:92` (EN has `devs_getting_chatlist` — naming inconsistency)
- `devs_no_team_users` — `devs.go:450` (EN has `devs_no_team_members` — naming inconsistency)
- `devs_no_users` — `devs.go:467` (EN has `devs_no_users_in_category` — naming inconsistency)
- `greetings_join_request_approve_btn` — `greetings.go:985` (all locales have `_button` suffix)
- `greetings_join_request_ban_btn` — `greetings.go:987` (all locales have `_button` suffix)
- `greetings_join_request_decline_btn` — `greetings.go:986` (all locales have `_button` suffix)
- `reports_cannot_report_channel` — `reports.go:56,316,336` (nil `From` on channel reply)

**ES-only orphans (7):**
- `devs_chat_list_caption` — correct code-facing name; ES already has translated value
- `devs_getting_chat_list` — correct code-facing name; ES already has translated value
- `devs_no_team_users` — correct code-facing name; ES already has translated value
- `devs_no_users` — correct code-facing name; ES already has translated value
- `misc_translate_need_text` — no production code reference; ES-only dead key
- `misc_translate_no_text` — no production code reference; ES-only dead key
- `misc_translate_provide_text` — no production code reference; ES-only dead key

---

## Architecture Patterns

### check-translations Script Location and Behavior

```
scripts/check_translations/
├── main.go         # Go program: extracts keys from Go source, checks against locale YAML files
├── main_test.go    # Tests for path resolution behavior
├── go.mod          # Module: github.com/divkix/Alita_Robot/scripts/check_translations, go 1.21
├── go.sum          # Dependency lock
└── check_translations  # Pre-compiled binary (not used by make target)
```

`make check-translations` runs `cd scripts/check_translations && go run main.go` which:
1. Walks `../../alita/` for all `.go` files (DOES NOT exclude `_test.go` files — BUG)
2. Extracts `tr.GetString("key")` and `tr.GetStringSlice("key")` calls via regex + AST parsing
3. Loads `../../locales/*.yml` (excludes `config.yml`)
4. For each locale, reports keys used in code that are absent from the locale YAML
5. Exits 1 if any missing translations found

**The `_test.go` bug:** The walk in `extractTranslationKeys()` at line 91-113 in `main.go` does:
```go
if d.IsDir() || !strings.HasSuffix(path, ".go") {
    return nil
}
```
Fix: add `|| strings.HasSuffix(path, "_test.go")` to the condition.

### i18n System Fallback Behavior (Relevant to Gap Analysis)

The translator falls back to EN when a key is missing in another locale:
```go
// From alita/i18n/translator.go
if t.langCode != t.manager.defaultLang {
    defaultTranslator, err := t.manager.GetTranslator(t.manager.defaultLang)
    return defaultTranslator.GetString(key, params...)
}
return "", ErrKeyNotFound
```

This means: missing keys in FR/HI/ES silently fall back to EN. If EN is also missing the key,
all locales return an empty string. The 10 missing production keys in EN cause actual silent
failures — users see empty strings from the bot.

### YAML Block Scalar Syntax (Important for Editing)

FR/HI locales extensively use YAML block scalars (folded `>` and literal `|`). Keys that appear
"empty" in raw grep (`key:\n  value on next line`) are NOT empty — they are multi-line strings.
Never assess emptiness from raw line parsing; always use YAML parsing.

```yaml
# This looks empty in raw grep but has a value:
bans_kick_cannot_kick_admin:
  Pourquoi expulserais-je un admin ? Ça semble être une idée plutôt
  stupide.
```

FR has 96 block scalars vs EN's 42. This explains FR having fewer raw lines (2099 vs 2207) while
having the same key count.

### Old Key Names vs Code Key Names (Naming Inconsistency)

Four devs keys have a code-locale naming drift. The code was updated to more consistent names but
EN/FR/HI locales were not updated. ES was partially updated (has BOTH old and new names):

| Code uses (new name) | EN/FR/HI have (old name) | ES has |
|---------------------|--------------------------|--------|
| `devs_getting_chat_list` | `devs_getting_chatlist` | BOTH |
| `devs_chat_list_caption` | `devs_chatlist_caption` | BOTH |
| `devs_no_team_users` | `devs_no_team_members` | BOTH |
| `devs_no_users` | `devs_no_users_in_category` | BOTH |

Neither the old key names NOR the greetings `_button` suffixed keys are referenced in any
production `.go` file (only in locale YAML). So both old devs keys and `_button` greetings keys
can be removed after adding the correctly-named variants.

Three greetings button keys also have suffix inconsistency:

| Code uses | All locales have |
|-----------|-----------------|
| `greetings_join_request_approve_btn` | `greetings_join_request_approve_button` |
| `greetings_join_request_decline_btn` | `greetings_join_request_decline_button` |
| `greetings_join_request_ban_btn` | `greetings_join_request_ban_button` |

---

## Standard Stack

This phase requires no new libraries. All work is editing YAML locale files and modifying one Go
script.

| Tool | Version | Purpose |
|------|---------|---------|
| YAML (locales/*.yml) | YAML 1.1 | Locale storage format; gopkg.in/yaml.v3 parser |
| Go (check_translations) | 1.21 (go.mod) | check-translations script |
| `make check-translations` | — | Validation gate; must exit 0 on completion |
| `make test` | — | Runs `go test ./...`; must remain green throughout |

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Finding key naming drift | Custom diff script | Python `yaml.safe_load()` + set comparison | grep-based key extraction misses block scalars |
| Verifying fix completeness | Manual locale scan | `make check-translations` (after fixing it) | It's the authoritative gate |
| Translating missing HI/FR content | Machine translation API | Copy EN values as fallback for non-translated content | i18n system already falls back to EN; adding EN text as placeholder is acceptable and correct behavior |

---

## Common Pitfalls

### Pitfall 1: Editing Locale YAML Without a Parser (Block Scalar Corruption)

**What goes wrong:** Using `sed` or naive string replacement to add/remove keys in YAML files
that contain block scalars corrupts indentation or splits multi-line values.

**Why it happens:** Block scalar syntax depends on consistent indentation. A line-level edit
that adds a key after a block scalar may break the scalar's value.

**How to avoid:** Use a YAML-aware approach for verification: `python3 -c "import yaml; yaml.safe_load(open('locales/en.yml'))"` after every edit to confirm the file parses cleanly. Edit YAML files directly but always validate parse after.

**Warning signs:** `yaml.safe_load()` throws a `ScannerError`. The check-translations script
reporting "Could not parse X" warnings.

### Pitfall 2: Removing Old Keys Before Confirming No Code Uses Them

**What goes wrong:** Deleting `devs_getting_chatlist` from locales before confirming no code path
uses it. If any module still calls `tr.GetString("devs_getting_chatlist")`, removing the key
causes silent empty strings.

**How to avoid:** Before removing any old key, grep the entire `alita/` directory:
```bash
grep -r "devs_getting_chatlist" /path/to/alita/ --include="*.go" | grep -v "_test.go"
```
Confirmed: `devs_getting_chatlist`, `devs_chatlist_caption`, `devs_no_team_members`,
`devs_no_users_in_category`, `greetings_join_request_approve_button`,
`greetings_join_request_decline_button`, `greetings_join_request_ban_button` — none appear in
production code. Safe to remove after adding correctly-named replacements.

### Pitfall 3: Adding New Keys With Wrong String Formatting Tokens

**What goes wrong:** A new key that renders a username uses `{user}` YAML notation but the Go
code passes `%s` as a legacy printf format.

**Why it happens:** The i18n system supports both `{key}` named interpolation and `%s`/`%d`
legacy placeholders. Mixing them produces malformed output.

**How to avoid:** Check existing similar keys for the same function to determine which style is
used. For new keys without parameters, no interpolation concerns.

**Key-specific format analysis:**
- `bans_cannot_identify_user` — no params (nil user message, no context to format)
- `captcha_internal_error` — no params (internal error message)
- `devs_chat_list_caption`, `devs_getting_chat_list` — no params (status messages)
- `devs_no_team_users` — no params
- `devs_no_users` — no params
- `greetings_join_request_*_btn` — no params (button labels)
- `reports_cannot_report_channel` — no params

### Pitfall 4: check-translations Scanning Docs `dist/` Directory

**What goes wrong:** The script could pick up `tr.GetString(...)` calls in generated HTML/JS
files under `docs/dist/` if that directory is ever under `../../alita/`. It currently isn't (the
`docs/` dir is a sibling of `alita/`, not inside it), but worth noting.

**How to avoid:** The script's root dir is hardcoded as `../../alita` — this is correct and
safe. No action needed.

### Pitfall 5: Treating FR/HI Gap as Real After Phase 1

**What goes wrong:** Planning large translation work for FR and HI based on the original phase
description's ~5%/~18% gap claims.

**Why it's wrong:** Those estimates predate Phase 1. After Phase 1 fixed path resolution in
check-translations, FR and HI are confirmed at 835 keys each — exact parity with EN. No
large translation effort is needed.

**Impact on plans:** Plans 03-03 (FR remediation) and 03-04 (HI remediation) from the roadmap
can be collapsed into the propagation step of 03-01 (add new keys to all 4 locales at once
rather than separate plans per language).

---

## Code Examples

### Verified: Adding a New Key to YAML Locale File

```yaml
# EN locale — add after nearby contextual key
# Correct style for single-line string without parameters:
bans_cannot_identify_user: "Cannot identify user from the replied message."

# Correct style for key matching EN's existing captcha error pattern:
captcha_internal_error: "An internal error occurred. Please try again."

# Correct style for short action labels (no params):
greetings_join_request_approve_btn: "✅ Approve"
greetings_join_request_decline_btn: "❌ Decline"
greetings_join_request_ban_btn: "✅ Ban"
```

### Verified: Adding New devs Keys (Must Match Code Names Exactly)

```yaml
# Code calls tr.GetString("devs_getting_chat_list") — underscore between "chat" and "list"
devs_getting_chat_list: Getting list of chats I'm in...
devs_chat_list_caption: Here is the list of chats in my Database!
devs_no_team_users: No users are added in Team!
devs_no_users: "No Users"
```

### Verified: Fixing check-translations to Exclude _test.go Files

In `scripts/check_translations/main.go`, function `extractTranslationKeys`, the walk callback:

```go
// CURRENT (buggy):
if d.IsDir() || !strings.HasSuffix(path, ".go") {
    return nil
}

// FIXED:
if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
    return nil
}
```

After this fix, the 8 test fixture keys disappear from the report, leaving only the 10 real
production keys per locale (before adding them to locale files).

### Verified: Removing ES Orphan Keys

ES orphan keys to remove from `locales/es.yml`:
- Remove `devs_getting_chatlist`, `devs_chatlist_caption`, `devs_no_team_members`,
  `devs_no_users_in_category` (the OLD names — code no longer uses these)
- Remove `misc_translate_need_text`, `misc_translate_no_text`, `misc_translate_provide_text`
  (unused in code; never were in EN)

ES orphan keys to KEEP (but they become non-orphans once added to EN):
- `devs_getting_chat_list`, `devs_chat_list_caption`, `devs_no_team_users`, `devs_no_users`
  (these are the CORRECT code-facing names; after adding to EN, they're no longer orphans)

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib `testing`) |
| Config file | None required — script is standalone |
| Quick run command | `cd scripts/check_translations && go test ./...` |
| Full suite command | `make check-translations` (exits 0 = passing) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| I18N-01 | EN locale naming inconsistencies resolved | smoke | `make check-translations` exits 0 | N/A (validation tool) |
| I18N-02 | ES orphan keys removed | smoke | `make check-translations` exits 0 | N/A (validation tool) |
| I18N-03 | Missing EN production keys added | smoke | `make check-translations` exits 0 | N/A (validation tool) |
| I18N-04 | FR locale no-op (no gap exists) | smoke | `make check-translations` exits 0 | N/A (validation tool) |
| I18N-05 | HI locale no-op (no gap exists) | smoke | `make check-translations` exits 0 | N/A (validation tool) |
| I18N-06 | All locales pass clean | integration | `make check-translations` exits 0 | N/A (validation tool) |

Additional test to add for I18N-01 (test file exclusion fix):

| What | Test Type | Location | File Exists? |
|------|-----------|----------|-------------|
| check-translations excludes `_test.go` files | unit | `scripts/check_translations/main_test.go` | YES — add new test case |

### Sampling Rate

- **Per task commit:** `make check-translations` (fast, ~2-3s)
- **Per wave merge:** `make check-translations && make test` (full suite)
- **Phase gate:** `make check-translations` exits 0 AND `make test` green

### Wave 0 Gaps

- [ ] `scripts/check_translations/main_test.go` — add `TestExtractKeysFromFile_ExcludesTestFiles` test case to cover the `_test.go` exclusion fix. Framework already present; file exists.

---

## Execution Order for Plans

The plan structure from the roadmap can be significantly collapsed given the actual state:

| Original Plan | Revised Scope | Reason |
|--------------|---------------|--------|
| 03-01: Fix EN naming inconsistencies | Fix check-translations script + add 10 keys to EN | Script fix must happen first; EN is canonical |
| 03-02: Remove ES orphans | Remove 7 ES orphan keys (4 old devs + 3 misc_translate) | Only 7 keys; straightforward |
| 03-03: Remediate FR gap | Propagate new EN keys to FR | FR has zero missing keys; only needs new 10 keys added |
| 03-04: Remediate HI gap | Propagate new EN keys to HI | HI has zero missing keys; only needs new 10 keys added |
| 03-05: Verify clean pass | Run make check-translations and confirm exits 0 | Final gate |

The former FR/HI "large translation effort" plans collapse to simple key-propagation steps.
Plans 03-03 and 03-04 are trivial and could be merged into 03-01 if desired.

---

## Open Questions

1. **Should old keys (devs_getting_chatlist etc.) be removed from EN/FR/HI or kept as dead keys?**
   - What we know: They are not referenced in any production code.
   - What's unclear: Whether any external tooling, docs, or integrations reference them.
   - Recommendation: Remove them — they cause confusion and check-translations doesn't care either way. But this is optional; keeping them is safe and doesn't affect the success criterion.

2. **What value should `reports_cannot_report_channel` contain?**
   - What we know: It fires when `msg.ReplyToMessage.From == nil` — i.e., the replied-to message is from a channel, not a user.
   - Recommendation: `"You can't report a channel post."` — matches the tone of nearby `reports_cannot_report_self` and `reports_special_account`.

3. **Should 03-03 and 03-04 be merged into 03-01?**
   - What we know: FR and HI only need the same 10 new keys added; they have no other gaps.
   - Recommendation: Planner's call. Keeping them separate is cleaner for plan tracking but adds minimal work. Merging reduces plan overhead.

---

## Sources

### Primary (HIGH confidence)

- Direct codebase inspection — `locales/*.yml` (python3 yaml.safe_load comparison), `alita/modules/devs.go`, `alita/modules/greetings.go`, `alita/modules/bans.go`, `alita/modules/captcha.go`, `alita/modules/reports.go`
- `scripts/check_translations/main.go` — script source, confirmed walk behavior
- `make check-translations` output — confirmed 68 missing translations, all categories
- `alita/i18n/translator.go` — confirmed EN fallback behavior

### Secondary (MEDIUM confidence)

None required — all findings are direct codebase evidence.

---

## Metadata

**Confidence breakdown:**
- Actual current state (key counts, missing keys): HIGH — confirmed by Python yaml.safe_load + make check-translations output
- Fix strategy (add keys, remove orphans): HIGH — code is source of truth; direction is unambiguous
- Old key removal safety: HIGH — confirmed no production code references via grep
- FR/HI gap claim debunking: HIGH — Python yaml.safe_load shows 835 keys each, exact EN parity

**Research date:** 2026-02-28
**Valid until:** Until any new keys are added to codebase (stable for 30+ days if no dev activity)
