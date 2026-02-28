# Phase 7: Tech Debt — Module Page Sentinel Guard and Metadata Backfill - Research

**Researched:** 2026-02-28
**Domain:** Go generator code modification, Markdown/YAML frontmatter editing, git worktree hygiene
**Confidence:** HIGH

## Summary

Phase 7 closes two categories of tech debt identified in the v1.0 milestone audit:

**Category 1 — Sentinel guard gap:** `generateModuleDocs()` in `scripts/generate_docs/generators.go` does not call `skipIfManuallyMaintained()` before writing each module page. The sentinel mechanism exists and works for `commands.md` and `callbacks.md` (added in Phase 6-02), but was never extended to the 21 module pages (`commands/*/index.md`). Every `make generate-docs` run silently overwrites hand-crafted content — alias blockquotes, normalized descriptions, and extended documentation added in Phases 2 and 4. The working tree is currently dirty with 21 modified module pages from a previous generator run.

**Category 2 — SUMMARY frontmatter inconsistency:** 13 SUMMARY files across phases 01, 03, 04, 05, and 06 use `requirements-completed` (hyphen) in their YAML frontmatter. The canonical form established in Phase 2 (and verified in the 3-source requirements cross-reference) is `requirements_completed` (underscore). All 13 files already contain the correct requirement ID values — only the field name key needs renaming.

The implementation is mechanical with no architectural uncertainty. The sentinel pattern, the `skipIfManuallyMaintained()` function, and the sentinel comment string (`<!-- MANUALLY MAINTAINED: do not regenerate -->`) are already established. The fix requires adding three lines of Go (check + log + continue) inside the `generateModuleDocs()` loop, inserting the sentinel comment into 21 Markdown files after the YAML frontmatter block, and renaming `requirements-completed:` to `requirements_completed:` in 13 YAML frontmatter blocks.

**Primary recommendation:** TDD — write the failing generator test in `generators_test.go` first (RED), then make the sentinel check + module page edits (GREEN), then fix the 13 SUMMARY files as a separate commit. Two commits total, ordered by impact.

## Standard Stack

### Core

| Tool | Version | Purpose | Why Standard |
|------|---------|---------|--------------|
| Go stdlib `os`, `path/filepath`, `strings` | Go 1.25+ | File I/O and path operations in generator | Already used throughout generators.go |
| `go test ./...` | Go 1.25+ | Unit testing the sentinel skip behavior | Established in parsers_test.go |
| `sed` or Go `strings.ReplaceAll` | — | Rename hyphen->underscore in YAML frontmatter | Mechanical string replacement |

### Supporting

| Tool | Version | Purpose | When to Use |
|------|---------|---------|-------------|
| `git restore docs/src/content/docs/commands/` | git | Restore 21 dirty module pages to HEAD before adding sentinels | Must run first to restore hand-crafted content |
| `make generate-docs` | — | Round-trip verification after fixes | Final proof sentinel guards work |
| `git status --short docs/src/content/docs/commands/*/` | git | Verify working tree clean for module pages only | Post-fix validation |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `continue` in loop | `return nil` | Must use `continue` — function iterates over modules in a loop. `return nil` would exit the entire function, skipping remaining modules |
| Sentinel in YAML frontmatter | External config file | YAML does not support inline comments in all parsers; HTML comment after closing `---` is self-contained and reliable |

## Architecture Patterns

### Recommended Project Structure

No new files needed except `scripts/generate_docs/generators_test.go`.

```
scripts/generate_docs/
├── generators.go          # Add 3-line sentinel check inside generateModuleDocs() loop
├── generators_test.go     # NEW: test sentinel skip behavior (TDD RED-GREEN)
├── parsers.go             # Unchanged
└── parsers_test.go        # Unchanged
docs/src/content/docs/commands/
├── {21-module-dirs}/
│   └── index.md           # Add sentinel comment after frontmatter (all 21)
.planning/phases/
├── 01/0{1,2,3}-SUMMARY.md               # Rename requirements-completed -> requirements_completed
├── 03-*/03-0{1,2}-SUMMARY.md            # Same
├── 04-*/04-0{1,2,3}-SUMMARY.md          # Same
├── 05-*/05-0{1,2,3}-SUMMARY.md          # Same
└── 06-*/06-0{1,2}-SUMMARY.md            # Same
```

### Pattern 1: Sentinel Skip in Generator Loop

**What:** Add `skipIfManuallyMaintained(moduleFile)` call inside `generateModuleDocs()` loop, using `continue` (not `return nil`) to skip protected files while continuing to process remaining modules.

**When to use:** Any generator function that iterates over multiple output files must use `continue` in a loop, not `return nil`.

**Exact code location:** `scripts/generate_docs/generators.go`, line 33-35 (after `moduleFile` is defined):

```go
// generateModuleDocs generates individual module pages in commands/{module}/index.md
func generateModuleDocs(modules []Module, outputPath string) error {
    for _, module := range modules {
        moduleDir := filepath.Join(outputPath, "commands", module.Name)
        moduleFile := filepath.Join(moduleDir, "index.md")

        // ADD THESE THREE LINES:
        if skipIfManuallyMaintained(moduleFile) {
            log.Infof("Skipped: commands/%s/index.md (manually maintained)", module.Name)
            continue
        }

        log.Debugf("Generating module doc: %s", module.DisplayName)
        // ... rest of function unchanged
```

**Existing sentinel pattern for comparison** (from `generateCommandReference`, line 169):
```go
func generateCommandReference(modules []Module, outputPath string) error {
    refDir := filepath.Join(outputPath, "api-reference")
    refFile := filepath.Join(refDir, "commands.md")

    if skipIfManuallyMaintained(refFile) {
        log.Infof("Skipped: %s (manually maintained)", filepath.Base(refFile))
        return nil   // <-- uses return nil because this function handles ONE file
    }
```

### Pattern 2: Sentinel Comment Placement in Module Pages

**What:** The sentinel comment `<!-- MANUALLY MAINTAINED: do not regenerate -->` goes immediately after the closing `---` of the YAML frontmatter block, on its own line. `skipIfManuallyMaintained()` reads the first 512 bytes — the frontmatter + sentinel fits in ~120-160 bytes, well within the check window.

**Exact placement:**
```markdown
---
title: Admin Commands
description: Complete guide to Admin module commands and features
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# 👑 Admin Commands
```

**Verified:** The sentinel string `<!-- MANUALLY MAINTAINED: do not regenerate -->` matches `manualMaintenanceSentinel` const defined at line 15 of generators.go exactly. Do not paraphrase it.

### Pattern 3: YAML Frontmatter Field Rename

**What:** Replace `requirements-completed:` with `requirements_completed:` in the YAML frontmatter block of 13 SUMMARY files. The values (requirement IDs in brackets) do not change.

**Before:**
```yaml
requirements_completed: [TOOL-01]
```

**After:**
```yaml
requirements_completed: [TOOL-01]
```

**All 13 files and their existing values (verified correct — only key name changes):**

| File | Current Value (keep as-is) |
|------|---------------------------|
| `01/01-01-SUMMARY.md` | `[TOOL-01]` |
| `01/01-02-SUMMARY.md` | `[TOOL-02, TOOL-04]` |
| `01/01-03-SUMMARY.md` | `[TOOL-03, TOOL-05]` |
| `03-locale-and-i18n-fixes/03-01-SUMMARY.md` | `[I18N-01, I18N-03, I18N-06]` |
| `03-locale-and-i18n-fixes/03-02-SUMMARY.md` | `[I18N-02, I18N-04, I18N-05, I18N-06]` |
| `04-operator-documentation/04-01-SUMMARY.md` | `[OPER-01]` |
| `04-operator-documentation/04-02-SUMMARY.md` | `[OPER-02]` |
| `04-operator-documentation/04-03-SUMMARY.md` | `[OPER-03]` |
| `05-readme-and-final-verification/05-01-SUMMARY.md` | `[VRFY-04]` |
| `05-readme-and-final-verification/05-02-SUMMARY.md` | `[VRFY-01, VRFY-02]` |
| `05-readme-and-final-verification/05-03-SUMMARY.md` | `[VRFY-03, VRFY-04]` |
| `06-gap-closure-restore-api-reference-and-housekeeping/06-01-SUMMARY.md` | `[DOCS-02, DOCS-03, DOCS-04, DOCS-05, DOCS-07]` |
| `06-gap-closure-restore-api-reference-and-housekeeping/06-02-SUMMARY.md` | `[DOCS-02, DOCS-03, DOCS-04, DOCS-05, DOCS-07]` |

### Anti-Patterns to Avoid

- **Using `return nil` instead of `continue` in loop:** The `generateModuleDocs` function loops over modules. `return nil` exits the entire function, leaving remaining modules unprocessed. Use `continue` to skip only the current module iteration.
- **Adding sentinel inside YAML frontmatter:** YAML frontmatter uses `---` delimiters. HTML comments inside the frontmatter block may cause parser errors in Astro/Starlight. Always place the sentinel comment AFTER the closing `---`.
- **Changing the sentinel comment text:** The sentinel string in generators.go is a const (`manualMaintenanceSentinel = "<!-- MANUALLY MAINTAINED: do not regenerate -->"`). The text in module pages must match exactly — no capitalization or punctuation changes.
- **Running make generate-docs before git restore:** The 21 module pages in the working tree are currently the auto-generated versions (dirty). Must `git restore` them first to recover the hand-crafted committed versions before adding sentinels.
- **Forgetting the 21st count:** The generator produces pages for exactly 21 modules (those with `*_help_msg` keys in `locales/en.yml`). The `devs`, `help`, and `users` modules have NO help messages and are NEVER written by `generateModuleDocs()` — those 3 pages don't need sentinels.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| YAML field rename | Custom YAML parser | `sed -i` or `strings.ReplaceAll` | Single-line rename of a known key — no structural YAML manipulation needed |
| Sentinel check | Custom file header checker | Existing `skipIfManuallyMaintained()` | Already implemented, tested implicitly via generate-docs runs |
| Finding which files need sentinel | Manual enumeration | `make generate-docs --dry-run` output | Dry-run lists exactly which 21 files would be written |

**Key insight:** Every tool needed already exists. This is pure application of established patterns to the gap that was left behind.

## Common Pitfalls

### Pitfall 1: Forgetting to Restore the Dirty Working Tree First

**What goes wrong:** Adding the sentinel comment to the current dirty (auto-generated) versions of module pages, which lack the hand-crafted alias blockquotes, normalized descriptions, and caution admonitions.

**Why it happens:** The working tree currently has 21 modified module pages (auto-generated overwrites from a previous generate-docs run). If you add the sentinel to those dirty files, you're protecting the wrong content.

**How to avoid:** Run `git restore docs/src/content/docs/commands/` before adding sentinel comments. Verify with `git status --short docs/src/content/docs/commands/` — should show no M-prefixed files.

**Warning signs:** `git diff docs/src/content/docs/commands/admin/index.md` showing missing `<!--` blockquotes or missing extended content sections.

### Pitfall 2: `return nil` vs `continue` in Loop

**What goes wrong:** Using `return nil` in the sentinel guard inside `generateModuleDocs()` causes the function to exit after skipping the first protected module, leaving all subsequent modules unprocessed.

**Why it happens:** The pattern from `generateCommandReference()` uses `return nil` — copy-paste error.

**How to avoid:** Check: is this inside a `for` loop? Yes → use `continue`. No → use `return nil`.

**Warning signs:** `make generate-docs` output shows "Skipped: commands/admin/index.md" but no subsequent module pages logged.

### Pitfall 3: Sentinel Comment Byte Position

**What goes wrong:** Placing the sentinel comment deep in the file (e.g., in a comment section at line 50+). `skipIfManuallyMaintained()` only checks the first 512 bytes.

**Why it happens:** Copy-paste of sentinel to wrong location.

**How to avoid:** Place sentinel immediately after the `---` closing the frontmatter block (line 5 of any module page). The entire frontmatter + sentinel is ~120-160 bytes — well within 512.

**Warning signs:** `make generate-docs` still shows "Would write" for module pages instead of "Skipped" after sentinel is added.

### Pitfall 4: Wrong Sentinel Text

**What goes wrong:** Sentinel comment text in module page doesn't match the `manualMaintenanceSentinel` const in generators.go.

**Why it happens:** Paraphrasing or adding punctuation variants.

**How to avoid:** Copy the exact string: `<!-- MANUALLY MAINTAINED: do not regenerate -->`. Verify with `grep -c "MANUALLY MAINTAINED: do not regenerate" docs/src/content/docs/commands/admin/index.md` returning 1.

## Code Examples

Verified from current codebase:

### Existing skipIfManuallyMaintained Implementation

```go
// Source: scripts/generate_docs/generators.go lines 13-26
const manualMaintenanceSentinel = "<!-- MANUALLY MAINTAINED: do not regenerate -->"

func skipIfManuallyMaintained(filePath string) bool {
    data, err := os.ReadFile(filePath)
    if err != nil {
        return false // File doesn't exist, proceed with generation
    }
    checkLen := min(512, len(data))
    return strings.Contains(string(data[:checkLen]), manualMaintenanceSentinel)
}
```

### Required Addition to generateModuleDocs

```go
// Source: scripts/generate_docs/generators.go lines 29-36 (current)
// CHANGE NEEDED: Add sentinel check after moduleFile is defined

func generateModuleDocs(modules []Module, outputPath string) error {
    for _, module := range modules {
        moduleDir := filepath.Join(outputPath, "commands", module.Name)
        moduleFile := filepath.Join(moduleDir, "index.md")

        // ADD: sentinel guard (use continue, not return nil — this is a loop)
        if skipIfManuallyMaintained(moduleFile) {
            log.Infof("Skipped: commands/%s/index.md (manually maintained)", module.Name)
            continue
        }

        log.Debugf("Generating module doc: %s", module.DisplayName)
        // ... rest unchanged
```

### Test Pattern for Sentinel Skip

```go
// Source: To be added to scripts/generate_docs/generators_test.go
// Pattern based on parsers_test.go tmpDir usage

func TestGenerateModuleDocs_SkipsManuallyMaintainedFiles(t *testing.T) {
    tmpDir := t.TempDir()

    // Create module directory with sentinel-protected index.md
    moduleDir := filepath.Join(tmpDir, "commands", "testmodule")
    if err := os.MkdirAll(moduleDir, 0755); err != nil {
        t.Fatalf("Failed to create module dir: %v", err)
    }

    originalContent := "---\ntitle: Test\n---\n" + manualMaintenanceSentinel + "\n\n# Hand-crafted content"
    moduleFile := filepath.Join(moduleDir, "index.md")
    if err := os.WriteFile(moduleFile, []byte(originalContent), 0644); err != nil {
        t.Fatalf("Failed to write sentinel file: %v", err)
    }

    modules := []Module{{Name: "testmodule", DisplayName: "Test", HelpText: "some help"}}

    if err := generateModuleDocs(modules, tmpDir); err != nil {
        t.Fatalf("generateModuleDocs returned error: %v", err)
    }

    // Verify file was NOT overwritten
    got, err := os.ReadFile(moduleFile)
    if err != nil {
        t.Fatalf("Failed to read file: %v", err)
    }
    if string(got) != originalContent {
        t.Errorf("Sentinel-protected file was overwritten.\nExpected: %q\nGot: %q", originalContent, string(got))
    }
}

func TestGenerateModuleDocs_WritesNonSentinelFiles(t *testing.T) {
    tmpDir := t.TempDir()

    modules := []Module{{Name: "newmodule", DisplayName: "New Module", HelpText: "help text"}}

    if err := generateModuleDocs(modules, tmpDir); err != nil {
        t.Fatalf("generateModuleDocs returned error: %v", err)
    }

    moduleFile := filepath.Join(tmpDir, "commands", "newmodule", "index.md")
    if _, err := os.Stat(moduleFile); os.IsNotExist(err) {
        t.Error("Expected module file to be created but it was not")
    }
}
```

### SUMMARY Frontmatter Fix (shell one-liner for verification)

```bash
# Rename hyphen->underscore in all 13 SUMMARY files
# Run from repo root
find .planning/phases -name "*SUMMARY.md" \
  -exec grep -l "^requirements-completed:" {} \; | \
  xargs sed -i 's/^requirements-completed:/requirements_completed:/'

# Verify
grep -r "^requirements-completed:" .planning/phases/ && echo "FAIL: hyphen still present" || echo "PASS: all renamed"
grep -r "^requirements_completed:" .planning/phases/ | wc -l  # Should be 16 (13 renamed + 3 already correct in Phase 2)
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| No sentinel for module pages (all 21 unprotected) | Sentinel check in generateModuleDocs() loop | Phase 7 | `make generate-docs` no longer overwrites hand-crafted module pages |
| `requirements-completed` (hyphen) in 13 SUMMARY files | `requirements_completed` (underscore) | Phase 7 | 3-source requirements cross-reference works without manual verification footnotes |

**Deprecated/outdated:**
- `requirements-completed` YAML key: inconsistent with Phase 2 canonical form; rename to `requirements_completed`

## Open Questions

1. **Should `commands/index.md` (untracked, stale) also be addressed in Phase 7?**
   - What we know: It's untracked (not committed), generated by `generateCommandsOverview()`, and shows stale stats (21 modules/129 commands vs actual 24/142). It was flagged in the audit as tech debt.
   - What's unclear: Phase 7 success criterion 5 says "no dirty module pages" — `commands/index.md` is untracked (??), not dirty (M), and is not a "module page" (it's the overview page).
   - Recommendation: Do NOT address it in Phase 7. The stale file can be deleted (`git clean -f docs/src/content/docs/commands/index.md`) or committed with correct content in a future cleanup. It is out of scope per the success criteria wording.

2. **Should sentinel be added to `generateCommandsOverview()` for `commands/index.md`?**
   - What we know: That file has no hand-crafted content worth protecting — it's pure auto-generated content from module stats.
   - Recommendation: No. Adding a sentinel to an auto-generated file defeats the purpose. Out of scope for Phase 7.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go built-in `testing` package |
| Config file | None — standard `go test` |
| Quick run command | `cd scripts/generate_docs && go test ./...` |
| Full suite command | `cd scripts/generate_docs && go test -v -race ./...` |

### Phase Requirements to Test Map

Phase 7 has no formal requirement IDs (tech debt closure). Tests map to success criteria:

| Success Criterion | Behavior | Test Type | Automated Command |
|------------------|----------|-----------|-------------------|
| SC-1: generateModuleDocs calls skipIfManuallyMaintained | Sentinel-protected module page not overwritten | unit | `cd scripts/generate_docs && go test -run TestGenerateModuleDocs_SkipsManuallyMaintainedFiles ./...` |
| SC-1: Non-sentinel files still written | Non-protected module page IS written | unit | `cd scripts/generate_docs && go test -run TestGenerateModuleDocs_WritesNonSentinelFiles ./...` |
| SC-2/3: All 21 pages have sentinel, generate-docs skips them | `make generate-docs` produces no changes to module pages | integration/manual | `make generate-docs && git status --short docs/src/content/docs/commands/*/index.md` (expect: no output) |
| SC-4: All 13 SUMMARY files have requirements_completed | No hyphen variant remains | smoke | `grep -r "^requirements-completed:" .planning/phases/ && echo FAIL \|\| echo PASS` |
| SC-5: Working tree clean for module pages | No M-prefixed module page files | smoke | `git status --short "docs/src/content/docs/commands/*/index.md" \| grep "^.M" && echo FAIL \|\| echo PASS` |

### Sampling Rate

- **Per task commit:** `cd scripts/generate_docs && go test ./...`
- **Per wave merge:** Full suite + `make generate-docs` round-trip check
- **Phase gate:** All tests green + `make generate-docs` produces no module page changes

### Wave 0 Gaps

- [ ] `scripts/generate_docs/generators_test.go` — new file covering SC-1 (RED phase before fix)

## Sources

### Primary (HIGH confidence)

- Direct code inspection: `scripts/generate_docs/generators.go` — sentinel const, `skipIfManuallyMaintained()` function, `generateModuleDocs()` function (lines 13-165), `generateCommandReference()` sentinel guard (lines 165-175)
- Direct code inspection: `scripts/generate_docs/parsers_test.go` — test pattern for this package (tmpDir setup, `package main` scope)
- Direct file inspection: All 16 SUMMARY files in `.planning/phases/` — field name variants confirmed
- Dry-run output: `go run ./scripts/generate_docs/. --dry-run` — confirmed exactly 21 module pages generated, devs/help/users not included
- Git status: Confirmed 21 dirty module pages + 1 untracked commands/index.md in working tree

### Secondary (MEDIUM confidence)

- `.planning/v1.0-MILESTONE-AUDIT.md` — defines the two tech debt categories, identifies 13 SUMMARY files and the sentinel guard gap as distinct items with severity ratings
- `.planning/ROADMAP.md` Phase 7 success criteria — the 21 module page count is correct (not stale); devs/help/users are excluded because generateModuleDocs never writes them

### Tertiary (LOW confidence)

None — all claims are directly verifiable from the codebase.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all tools already in codebase, no new dependencies
- Architecture: HIGH — pattern already established for commands.md and callbacks.md; this is mechanical extension
- Pitfalls: HIGH — identified from direct code inspection and dry-run verification
- Test strategy: HIGH — parsers_test.go pattern is directly applicable, same package

**Research date:** 2026-02-28
**Valid until:** Stable (no external dependencies; this is pure internal Go code and YAML/Markdown editing)
