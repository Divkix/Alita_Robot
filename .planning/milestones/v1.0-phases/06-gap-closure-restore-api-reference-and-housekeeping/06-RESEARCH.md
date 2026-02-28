---
phase: 06-gap-closure-restore-api-reference-and-housekeeping
researched: 2026-02-28
domain: Documentation restoration, generator hardening, metadata housekeeping
confidence: HIGH
---

# Phase 6: Gap Closure — Restore api-reference and Housekeeping - Research

**Researched:** 2026-02-28
**Domain:** Documentation restoration, generator protection, inventory/metadata sync
**Confidence:** HIGH — all findings verified against actual files in the repository

## Summary

Phase 6 closes a single integration failure that invalidated three DOCS requirements after Phase 5: `make generate-docs` unconditionally overwrites `api-reference/commands.md` and `api-reference/callbacks.md`, silently reverting Phase 2's manual rewrites. The generator produces a 4-column table (Command|Handler|Disableable|Aliases) while Phase 2 produced a 5-column table (Command|Description|Permission|Disableable|Aliases). The generator also emits old dot-notation callback documentation while Phase 2 wrote versioned codec format.

The fix has two parts: (1) restore the Phase 2 content into the two regressed files by manual rewrite — the exact content specification exists verbatim in `02-02-PLAN.md`, and (2) harden the generator so future `make generate-docs` runs do not destroy hand-authored content. The hardening strategy is a sentinel/skip mechanism: the generator detects a sentinel comment in a file and skips regeneration of that file. This is the minimal, low-risk approach that preserves generator behavior for all auto-generated files while protecting manually maintained ones.

Beyond the two broken files, there are four housekeeping tasks: update INVENTORY.json/INVENTORY.md to reflect that devs/help/users now have docs directories (Phase 2 created them but the inventory was never re-run), fix ROADMAP.md to show Phase 6 plan counts once plans are finalized, and add `requirements_completed` frontmatter arrays to the three Phase 2 SUMMARY.md files (currently empty, causing traceability gaps).

**Primary recommendation:** Rewrite both api-reference files first (direct content restore from 02-02-PLAN.md spec), then add sentinel protection to the generator, then run housekeeping metadata fixes. Do not attempt to make the generator produce the 5-column format — that requires understanding all permission guards across all modules and is out of scope for an auto-generator.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| DOCS-02 | Add all missing commands to `api-reference/commands.md` (13 missing: start, help, donate, about, stats, addsudo, adddev, remsudo, remdev, chatinfo, chatlist, leavechat, teamusers) | Full content spec exists in 02-02-PLAN.md. Current file has 129/21 modules; required is 142/25. Permission levels per module verified in plan. |
| DOCS-03 | Document all command aliases explicitly with "Alias of /primary" notation | Alias notation was in Phase 2 output; currently absent from api-reference/commands.md but present in per-module command pages. Spec in 02-02-PLAN.md covers 4 MultiCommand pairs (8 alias rows). |
| DOCS-04 | Verify and correct Disableable column against actual AddCmdToDisableable() calls | The auto-generated file has a Disableable column with accurate data from INVENTORY. The issue is the manually verified Permission context was lost. Restoring Phase 2 content satisfies DOCS-04. 17 disableable commands per canonical inventory. |
| DOCS-05 | Update callbacks.md to document versioned codec format (namespace\|v1\|url-encoded-fields) | Current file shows old `{prefix}{data}` format. Phase 2 wrote versioned format with Go encode/decode example. callbackcodec.go verified: Encode() produces `namespace\|v1\|url-encoded`, Decode() parses it. Backward compat note required. |
| DOCS-07 | Verify permission requirements listed in docs match actual Require* calls in each handler | Permission column was in Phase 2 commands.md (5-column table). Current file has no Permission column. Permission values verified in 02-02-PLAN.md against source code. Restoring Phase 2 content satisfies DOCS-07. |
</phase_requirements>

## Standard Stack

### Core
| Tool | Version | Purpose | Why Standard |
|------|---------|---------|--------------|
| Go | 1.25+ | Generator source language | Project language; generator is a Go script |
| Markdown/MDX | Starlight-compatible | Documentation format | Existing docs site format |
| `make generate-docs` | current | Runs `scripts/generate_docs/` | Existing Makefile target |
| `make inventory` | current | Re-generates INVENTORY.json and INVENTORY.md | Existing Makefile target — correct way to update inventory |

### Supporting
| Tool | Version | Purpose | When to Use |
|------|---------|---------|-------------|
| `bun run build` (Astro) | current | Verify docs build passes | Verification gate after any docs changes |
| `grep`/shell | — | Verify sentinel presence in files | Quick automated check in verification |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Sentinel comment in file | Separate "protected files" config list | Config list requires generator to read it; sentinel is self-contained in the file itself — lower coupling |
| Sentinel comment in file | Flag to generator CLI | CLI flag requires callers (Makefile) to enumerate protected files — fragile; sentinel is discoverable from the file |
| Manual content restore | Modifying generator to produce 5-column format | Generator would need permission data source it doesn't have; building that is out of scope and high-risk |
| `make inventory` re-run | Manually editing INVENTORY.json | `make inventory` is idempotent and authoritative; manual edits would drift immediately |

## Architecture Patterns

### Recommended Phase Structure

```
Phase 6 / 3 plans:
06-01: Restore api-reference/commands.md and callbacks.md (DOCS-02/03/04/05/07)
06-02: Harden generator with sentinel skip mechanism (prevent future regression)
06-03: Housekeeping — INVENTORY.json/MD update, ROADMAP/SUMMARY metadata fixes
```

### Pattern 1: Content Restore from Spec

**What:** Directly rewrite `api-reference/commands.md` and `api-reference/callbacks.md` using the complete spec from `02-02-PLAN.md`. Do not regenerate — this is a manual rewrite.

**When to use:** When a file has been auto-overwritten and the authoritative spec exists in a previous plan.

**Exact spec location:** `.planning/phases/02-api-reference-and-command-documentation/02-02-PLAN.md`

Key content to restore for `commands.md`:
- 5-column header: `| Command | Description | Permission | Disableable | Aliases |`
- Total Modules: 25 (24 user-facing + 1 internal), Total Commands: 142
- Two new module sections: `### 🔧 Devs` (9 commands) and `### ❓ Help` (4 commands)
- Permission values per module (verified against source, full list in plan)
- "Alias of /primary" notation for 4 MultiCommand pairs
- Alphabetical index with 4-column format: `| Command | Module | Description | Permission |`

Key content to restore for `callbacks.md`:
- Replace `{prefix}{data}` format description with versioned codec spec
- Format: `<namespace>|v1|<url-encoded-fields>`
- Go code example using `callbackcodec.Encode()` / `callbackcodec.Decode()` from actual implementation
- Backward compatibility note for legacy dot-notation
- Keep all callback handler entries unchanged (they are accurate)

### Pattern 2: Sentinel Skip Mechanism

**What:** Add a sentinel comment to manually-maintained files. The generator checks for this sentinel at the top of the file before writing and skips regeneration if found.

**When to use:** When a generated file requires manual post-editing that must survive future generator runs.

**Implementation in `generators.go`:**

```go
// skipIfManuallyMaintained checks if a file has the sentinel comment
// indicating it should not be overwritten by the generator.
const manualMaintenanceSentinel = "<!-- MANUALLY MAINTAINED: do not regenerate -->"

func skipIfManuallyMaintained(filePath string) bool {
    data, err := os.ReadFile(filePath)
    if err != nil {
        return false // File doesn't exist, proceed with generation
    }
    // Check first 512 bytes for sentinel (before any content)
    checkLen := 512
    if len(data) < checkLen {
        checkLen = len(data)
    }
    return strings.Contains(string(data[:checkLen]), manualMaintenanceSentinel)
}
```

**In `generateCommandReference()`:**

```go
func generateCommandReference(modules []Module, outputPath string) error {
    refFile := filepath.Join(outputPath, "api-reference", "commands.md")

    if skipIfManuallyMaintained(refFile) {
        log.Info("Skipped: api-reference/commands.md (manually maintained)")
        return nil
    }
    // ... rest of function unchanged
}
```

**In `generateCallbacksReference()`:**

```go
func generateCallbacksReference(callbacks []Callback, outputPath string) error {
    refFile := filepath.Join(outputPath, "api-reference", "callbacks.md")

    if skipIfManuallyMaintained(refFile) {
        log.Info("Skipped: api-reference/callbacks.md (manually maintained)")
        return nil
    }
    // ... rest of function unchanged
}
```

**Sentinel placement in the protected files** — add as first line after frontmatter closing `---`:

```markdown
---
title: Command Reference
description: Complete reference of all Alita Robot commands
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# 🤖 Command Reference
```

### Pattern 3: Inventory Re-generation

**What:** Run `make inventory` from project root to regenerate INVENTORY.json and INVENTORY.md with correct `has_docs_directory` values.

**When to use:** When docs directories have been created since the last inventory run.

**Current stale state:**
- INVENTORY.json: `devs: false`, `help: false`, `users: false`
- INVENTORY.md: Lists devs, help, users as "Missing docs" in "Modules Without Documentation" section

**Correct state (after re-run):**
- All three should show `has_docs_directory: true`
- Docs paths: `docs/src/content/docs/commands/devs`, `docs/src/content/docs/commands/help`, `docs/src/content/docs/commands/users`

**Verification:** `ls docs/src/content/docs/commands/devs docs/src/content/docs/commands/help docs/src/content/docs/commands/users` — all three exist with `index.md`.

Note: The Makefile `inventory` target runs: `cd scripts/generate_docs && go run . -inventory`. This writes to `.planning/INVENTORY.json` and `.planning/INVENTORY.md`.

### Pattern 4: Metadata Housekeeping

**What:** Fix four metadata items that the audit identified as tech debt.

**Items:**

1. **Phase 2 SUMMARY frontmatter** — Add `requirements_completed` arrays to all three 02-0x-SUMMARY.md files:
   - `02-01-SUMMARY.md`: `requirements_completed: [DOCS-06]`
   - `02-02-SUMMARY.md`: `requirements_completed: [DOCS-02, DOCS-03, DOCS-04, DOCS-05, DOCS-07]`
   - `02-03-SUMMARY.md`: `requirements_completed: [DOCS-01, DOCS-03]`

2. **ROADMAP.md Phase 6 plans** — Update the "Plans:" section under Phase 6 with actual plan list once finalized (currently blank "Plans:" line with no entries).

3. **ROADMAP.md progress table** — Update Phase 6 row from `0/0 Planned` to `N/N Complete` with completion date after plans execute.

4. **REQUIREMENTS.md traceability** — Already shows DOCS-02/03/04/05/07 as Pending. Update to Complete after Phase 6 executes.

Note: The audit mentioned Phase 5 progress table shows "0/3 Planned" but the actual ROADMAP.md at time of research shows Phase 5 as "3/3 Complete 2026-02-28" — this was already fixed. The Phase 5 checkbox in the phases list (`- [x] Phase 5`) is also checked. The only remaining stale entry is Phase 6 itself, which will be correct once the phase executes.

### Anti-Patterns to Avoid

- **Do not modify the generator to produce the 5-column format:** Permission data does not exist in any data source the generator reads (locale files, source AST, etc.). Building it would require parsing handler bodies for `Require*()` calls — a non-trivial static analysis problem. Out of scope.
- **Do not skip sentinel check with DryRun:** The sentinel skip must work in both normal and dry-run modes. In dry-run, log the skip but don't error.
- **Do not run `make generate-docs` before adding sentinels:** Running the generator before sentinels are in place will re-overwrite the restored files. Sentinel must be in the file before any generator runs occur.
- **Do not manually edit INVENTORY.json:** Use `make inventory` to regenerate it. The generator reads source files and docs directories; manual edits drift immediately on next run.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Inventory update | Manual JSON editing | `make inventory` | Generator reads current state from filesystem; manual edits drift on next run |
| Permission data in generator | Static analysis of handler bodies | Manual content in sentinel-protected file | Parsing Go AST for permission guards is non-trivial; Phase 2 spec already verified against source |
| File protection mechanism | External config file or .gitignore-style ignore list | Sentinel comment inside file | Sentinel is discoverable from the file itself; no external config to maintain |

**Key insight:** The generator is good at auto-discovery of commands/callbacks/handlers from source, but structurally cannot produce humanly-verified content like permission requirements, terse descriptions, or "Alias of /primary" relationships without a data source that doesn't exist. The sentinel pattern accepts this reality instead of fighting it.

## Common Pitfalls

### Pitfall 1: Restore Without Sentinel = Immediate Regression
**What goes wrong:** Files are restored correctly but `make generate-docs` is run again (e.g., in verification) and overwrites them again.
**Why it happens:** Generator has no knowledge that a file has been hand-edited.
**How to avoid:** Add sentinels to both files as part of the same plan as the restore. Never run the generator on a restored file before the sentinel is in place.
**Warning signs:** Post-restore verification grep for "142" or "namespace|v1" succeeds, but post-generator-run grep fails.

### Pitfall 2: Sentinel Placement Breaks Frontmatter
**What goes wrong:** Sentinel placed inside or before the YAML frontmatter block causes Starlight/Astro to fail parsing the file.
**Why it happens:** Astro requires frontmatter to be the very first thing in an MDX/MD file. Anything before `---` breaks it.
**How to avoid:** Always place sentinel AFTER the closing `---` of frontmatter, as an HTML comment on its own line before the first heading. `<!-- -->` comments are invisible to markdown renderers.
**Warning signs:** `bun run build` fails with frontmatter parse error.

### Pitfall 3: Generator Skip Doesn't Cover DryRun Mode
**What goes wrong:** Sentinel skip only implemented for write path, not dry-run path. Dry-run still logs "Would write" which causes confusion in CI.
**Why it happens:** Copy-paste: write guard added to write block but not to `config.DryRun` check block.
**How to avoid:** Check sentinel BEFORE both the dry-run branch and the write branch in each generator function. Early return from the entire function.
**Warning signs:** `make generate-docs --dry-run` logs "Would write: api-reference/commands.md" even after sentinel is in file.

### Pitfall 4: Stale Command Count in Restored File
**What goes wrong:** The restored file claims 142 commands but post-restore count check finds a different number.
**Why it happens:** 02-02-PLAN.md spec was written based on INVENTORY.json state at that time. If any module changes occurred between Phase 2 and Phase 6, the count may differ.
**How to avoid:** After restoring, verify count: `grep -c '^| \`/' docs/src/content/docs/api-reference/commands.md`. The expected count is based on the plan spec (142 total command rows including alias rows). Cross-check against INVENTORY.json.
**Warning signs:** Count check returns fewer than 142 or more than 142.

### Pitfall 5: INVENTORY.json Update Method
**What goes wrong:** INVENTORY.json is manually edited instead of regenerated, creating drift on next `make inventory` run.
**Why it happens:** Editing JSON directly is faster than running a Go build. But the generator reads filesystem state, so next run will correct any manual edits anyway.
**How to avoid:** Always use `make inventory` for INVENTORY updates. The target runs the Go generator with `-inventory` flag which reads docs directory state from the filesystem.
**Warning signs:** INVENTORY.json and INVENTORY.md are inconsistent with each other (manual JSON edit without corresponding MD update).

## Code Examples

Verified patterns from the actual codebase:

### callbackcodec.Encode / Decode (from callbackcodec.go)
```go
// Source: alita/utils/callbackcodec/callbackcodec.go

// Encode: namespace|v1|url-encoded-fields
data, err := callbackcodec.Encode("restrict", map[string]string{
    "a":   "ban",
    "uid": "123456789",
})
// → "restrict|v1|a=ban&uid=123456789"

// Empty payload uses "_" as placeholder
data, err := callbackcodec.Encode("helpq", map[string]string{})
// → "helpq|v1|_"

// Decode
decoded, err := callbackcodec.Decode("restrict|v1|a=ban&uid=123456789")
namespace := decoded.Namespace   // "restrict"
action, _ := decoded.Field("a")  // "ban"
uid, _ := decoded.Field("uid")   // "123456789"

// Max length: 64 bytes (Telegram callback_data limit)
// ErrDataTooLong returned if Encode exceeds limit
```

### Sentinel Skip in Generator (pattern to implement)
```go
// Source: scripts/generate_docs/generators.go (to add)

const manualMaintenanceSentinel = "<!-- MANUALLY MAINTAINED: do not regenerate -->"

func skipIfManuallyMaintained(filePath string) bool {
    data, err := os.ReadFile(filePath)
    if err != nil {
        return false
    }
    checkLen := 512
    if len(data) < checkLen {
        checkLen = len(data)
    }
    return strings.Contains(string(data[:checkLen]), manualMaintenanceSentinel)
}

// Usage in generateCommandReference:
func generateCommandReference(modules []Module, outputPath string) error {
    refFile := filepath.Join(outputPath, "api-reference", "commands.md")
    if skipIfManuallyMaintained(refFile) {
        log.Info("Skipped: api-reference/commands.md (manually maintained)")
        return nil
    }
    // ... existing generation logic
}
```

### Permission Value Mapping (from 02-02-PLAN.md, verified against source)
```
Everyone:  No permission guard
Admin:     RequireUserAdmin() or RequireUserOwner()
Owner:     user.Id != config.AppConfig.OwnerId
Dev/Owner: !memStatus.Dev && user.Id != OwnerId
Team:      FindInInt64Slice(teamint64Slice, user.Id)
User/Admin: Asymmetric — any user in PM, admin in groups (language module only)
```

### MultiCommand Pairs (from parsers.go, verified in source)
```
blacklists:   remallbl ↔ rmallbl     (primary: remallbl)
formatting:   markdownhelp ↔ formatting (primary: markdownhelp)
notes:        privnote ↔ privatenotes  (primary: privnote)
rules:        resetrules ↔ clearrules  (primary: resetrules)
```

## State of the Art

| Old Approach | Current Approach | Notes |
|--------------|------------------|-------|
| Generator unconditionally overwrites all api-reference files | Generator skips files with sentinel comment | To be implemented in Phase 6 |
| INVENTORY.json has devs/help/users as has_docs=false | INVENTORY.json shows has_docs=true for all three | After `make inventory` re-run |
| Phase 2 SUMMARY.md files have empty requirements_completed | requirements_completed arrays populated | After housekeeping task |

## Open Questions

1. **Should the sentinel be visible to docs readers?**
   - What we know: HTML comments (`<!-- -->`) in Markdown are rendered as invisible by all Markdown renderers, including Starlight/Astro.
   - What's unclear: Nothing — HTML comments are the correct format. MDX may parse them but does not render them.
   - Recommendation: Use `<!-- MANUALLY MAINTAINED: do not regenerate -->` as an HTML comment on line after frontmatter closing. Confirmed safe.

2. **Will `make inventory` update INVENTORY.md as well as INVENTORY.json?**
   - What we know: The Makefile `inventory` target runs `go run . -inventory`. The `generateInventory()` function in `main.go` writes both JSON and MD files.
   - What's unclear: Nothing — the function generates both at once.
   - Recommendation: Run `make inventory` once; verify both files updated.

3. **Is the 142 command count still accurate, or have modules changed since Phase 2?**
   - What we know: INVENTORY.md header shows 142 commands, 25 modules, generated by `make inventory`. The current INVENTORY.json count matches.
   - What's unclear: Whether any commands were added/removed between Phase 2 commit (28e578c) and now.
   - Recommendation: After restoring commands.md, verify row count matches INVENTORY.json total. If count differs, adjust. Use INVENTORY.json as ground truth.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | shell verification commands (no test framework for docs) |
| Config file | none — docs content verified via grep |
| Quick run command | `grep -c "^| \`/" docs/src/content/docs/api-reference/commands.md` |
| Full suite command | `cd docs && bun run build 2>&1 \| tail -5` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| DOCS-02 | commands.md has 142+ commands including start/help/devs commands | smoke | `grep -q "/start" docs/src/content/docs/api-reference/commands.md && grep -q "Dev/Owner" docs/src/content/docs/api-reference/commands.md && echo PASS` | ✅ (file exists, content wrong) |
| DOCS-03 | Alias rows use "Alias of /primary" notation | smoke | `grep -c "Alias of" docs/src/content/docs/api-reference/commands.md` — expect 4+ | ✅ (file exists, content wrong) |
| DOCS-04 | Disableable column accurate (17 commands) | smoke | `grep -c "✅" docs/src/content/docs/api-reference/commands.md` — expect 17+ | ✅ (present in auto-generated but no Permission context) |
| DOCS-05 | callbacks.md documents versioned codec format | smoke | `grep -q "namespace\|v1\|url-encoded-fields" docs/src/content/docs/api-reference/callbacks.md && echo PASS` | ✅ (file exists, content wrong) |
| DOCS-07 | commands.md has Permission column | smoke | `grep -q "Permission" docs/src/content/docs/api-reference/commands.md && echo PASS` | ✅ (file exists, column missing) |
| Generator hardening | make generate-docs does not overwrite sentinel-protected files | integration | `make generate-docs && grep -q "namespace\|v1" docs/src/content/docs/api-reference/callbacks.md && echo PASS` | ❌ Wave 0: sentinel mechanism doesn't exist yet |
| INVENTORY update | devs/help/users show has_docs=true in INVENTORY.json | smoke | `python3 -c "import json; d=json.load(open('.planning/INVENTORY.json')); [print(m['module'],m['has_docs_directory']) for m in d if m['module'] in ['devs','help','users']]"` | ✅ (file exists, values stale) |
| Docs build | Astro build passes after restore | integration | `cd docs && bun run build 2>&1 \| grep -E "error\|ERROR"` — expect no output | ✅ (build currently passes) |

### Sampling Rate
- **Per task commit:** Quick smoke greps for the specific requirement being addressed
- **Per wave merge:** Full `bun run build` to confirm no broken links introduced
- **Phase gate:** All smoke checks + full build green before marking Phase 6 complete

### Wave 0 Gaps
- [ ] `scripts/generate_docs/generators.go` — sentinel skip mechanism needs adding (functions `skipIfManuallyMaintained`, guards in `generateCommandReference` and `generateCallbacksReference`)
- [ ] `docs/src/content/docs/api-reference/commands.md` — sentinel comment after frontmatter, before content
- [ ] `docs/src/content/docs/api-reference/callbacks.md` — sentinel comment after frontmatter, before content

*(All target files exist — infrastructure needs code changes, not new files)*

## Sources

### Primary (HIGH confidence)
- Direct file reads of codebase — `scripts/generate_docs/generators.go` (generator functions `generateCommandReference` at line 150, `generateCallbacksReference` at line 921), `scripts/generate_docs/parsers.go` (command/callback parsers)
- Direct file reads — `alita/utils/callbackcodec/callbackcodec.go` — confirmed codec format: `namespace|v1|url-encoded-fields`, max 64 bytes, `_` placeholder for empty payloads
- Direct file reads — `docs/src/content/docs/api-reference/commands.md` — confirmed current state: 129 commands, 21 modules, 4-column table, no Permission column
- Direct file reads — `docs/src/content/docs/api-reference/callbacks.md` — confirmed current state: old `{prefix}{data}` format with `restrict.ban.123456789` example
- Direct file reads — `.planning/phases/02-api-reference-and-command-documentation/02-02-PLAN.md` — authoritative spec for restore content (permission levels, alias notation, 13 missing commands)
- Direct file reads — `.planning/INVENTORY.json` — confirmed devs/help/users have `has_docs_directory: false` (stale)
- Direct verification — `ls docs/src/content/docs/commands/devs help users` — all three directories exist with `index.md`
- Direct file reads — `.planning/v1.0-MILESTONE-AUDIT.md` — authoritative gap analysis

### Secondary (MEDIUM confidence)
- `.planning/phases/02-api-reference-and-command-documentation/02-02-SUMMARY.md` — confirms Phase 2 previously delivered 142 commands/5-column format at commit 28e578c
- `.planning/ROADMAP.md` — confirms Phase 6 goal, requirements mapping, and current progress table state

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — only Go/Markdown involved; tools already exist in repo
- Architecture patterns: HIGH — sentinel pattern is well-understood Go file check; content restore is from verified spec
- Pitfalls: HIGH — root cause (generator overwrites) confirmed by auditing actual file state; sentinel placement in frontmatter is a known Astro constraint

**Research date:** 2026-02-28
**Valid until:** 2026-03-28 (stable codebase; no external dependencies to drift)
