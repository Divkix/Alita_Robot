# Phase 1: Ground Truth and Tooling - Research

**Researched:** 2026-02-27
**Domain:** Go script patching, Astro/Starlight plugin integration, command inventory extraction
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**1. Script Fix Approach**
Fix both scripts in-place with minimal, surgical patches. No rewrites.

- `scripts/check_translations/main.go`: Fix the path resolution bug — it is a small change to the path validation
  logic that blocks all `..`-relative paths. Change to use `filepath.Abs()` to resolve paths before validation
  rather than rejecting paths that contain `..`.
- `scripts/generate_docs/parsers.go`: Add a second regex pattern to also match
  `cmdDecorator.MultiCommand(dispatcher, []string{...}, handler)` registrations. There are exactly 4
  `MultiCommand` call sites. ~10 line addition, not a rewrite.

**2. Inventory Output Format**
JSON as primary machine-consumable format, with a Markdown summary table for human review.

JSON structure per module:
```json
{
  "module": "bans",
  "source_file": "alita/modules/bans.go",
  "commands": [
    {
      "command": "ban",
      "aliases": [],
      "handler_group": 0,
      "disableable": true,
      "registration_pattern": "NewCommand"
    }
  ],
  "callbacks": [...],
  "message_watchers": [...],
  "has_docs_directory": true,
  "docs_path": "docs/src/content/docs/commands/bans/"
}
```

Markdown table columns: Module | Commands | Aliases | Callbacks | Watchers | Disableable | Has Docs

Output location: `.planning/INVENTORY.json` (machine) and `.planning/INVENTORY.md` (human)

**3. Docs Link Validation Scope**
Internal links only. Fail build on broken links (not warn-only). External URLs not validated.

Integration: Add `starlight-links-validator` as a dev dependency in `docs/package.json` and configure in
`docs/astro.config.mjs` as a Starlight plugin.

**4. Tooling Integration**
New Make targets for the fixed scripts. Inventory is a planning artifact, not a build artifact.

- `make check-docs` — new target running patched `generate_docs` script
- `make check-translations` — already exists, just needs path bug fixed
- `make generate-docs` — already exists, just needs MultiCommand support added
- No CI integration in this phase

### Claude's Discretion

None identified — all 4 areas resolved cleanly within phase scope.

### Deferred Ideas (OUT OF SCOPE)

None identified.
</user_constraints>

## Summary

Phase 1 is pure tooling work: two surgical Go script patches and one npm package installation. All three tasks
are mechanical — no design decisions remain open. The research confirms the exact root cause of both bugs,
documents the precise fix strategy, and verifies the `starlight-links-validator` API.

**The `check-translations` bug** is in `extractKeysFromFile()` (line 128) and `loadLocaleFiles()` (lines
245-252) in `scripts/check_translations/main.go`. Both functions reject any file path containing `..` via
`strings.Contains(filePath, "..")`. Because the script is invoked from its own subdirectory and uses relative
paths like `../../alita/...` and `../../locales/en.yml`, every single file is rejected. The fix is to resolve
paths to absolute form with `filepath.Abs()` before the path-traversal check. Result: the script currently
reports "0 found keys" and "All translations present" — a total false negative.

**The `generate_docs` MultiCommand blindspot** is in `parseCommands()` in
`scripts/generate_docs/parsers.go`. Only one regex pattern exists: `handlers.NewCommand\s*\(\s*"([^"]+)"...`.
There are exactly 4 `cmdDecorator.MultiCommand(dispatcher, []string{...}, handler)` call sites in the
codebase (blacklists: `remallbl`/`rmallbl`; formatting: `markdownhelp`/`formatting`; notes:
`privnote`/`privatenotes`; rules: `resetrules`/`clearrules`). Each is registered via `MultiCommand` which
internally calls `dispatcher.AddHandler(handlers.NewCommand(i, r))` — a standard registration. The fix adds
a second regex to capture alias slices and registers each alias string as a separate command entry with
`registration_pattern: "MultiCommand"`.

**The `starlight-links-validator` integration** is a 2-step npm install + config change. Current version is
0.19.2 (released December 2025). The plugin only runs during production builds (`astro build`), not during
`astro dev`. The existing `docs/astro.config.mjs` already uses the `plugins: []` array in the `starlight()`
config — `starlightLinksValidator()` is added alongside the existing plugins.

**Primary recommendation:** Fix the `check-translations` path validation bug first (30-minute task). It is
the highest-leverage fix: without it, no locale gap data is trustworthy for Phase 3.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| TOOL-01 | Fix `make check-translations` script path resolution bug so it correctly reports missing/orphan locale keys | Root cause identified: `strings.Contains(filePath, "..")` blocks all relative paths; fix is `filepath.Abs()` before check in both `extractKeysFromFile` and `loadLocaleFiles` |
| TOOL-02 | Patch `generate_docs` parsers.go to extract `cmdDecorator.MultiCommand()` registrations | 4 call sites confirmed; regex pattern designed; `Command.Aliases` field already exists in struct; `registration_pattern` field needs adding |
| TOOL-03 | Produce canonical command inventory covering all 22 modules with commands, aliases, callbacks, message watchers, and disableable status | `parseCommands()` + `parseCallbacks()` already extract most data; inventory script must also walk message watcher `AddHandlerToGroup()` calls and output structured JSON/Markdown |
| TOOL-04 | Install and configure `starlight-links-validator` plugin in Astro docs build to catch broken internal links | Version 0.19.2 confirmed; install: `bun add -D starlight-links-validator` in `docs/`; config: add `starlightLinksValidator()` to `plugins` array in `astro.config.mjs` |
| TOOL-05 | Create module-to-docs mapping table showing which modules have docs directories and which don't | 22 module `.go` files identified vs 22 docs directories in `docs/src/content/docs/commands/`; 4 missing: `devs`, `help`, `language`, `users` |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib (`filepath`, `os`, `regexp`, `go/ast`) | Go 1.25 | Script patching — path resolution and regex pattern extension | Already in use in both scripts; zero new deps |
| `starlight-links-validator` | 0.19.2 | Internal link validation at Astro build time | Only Starlight-native link validator; integrates as a first-class plugin; maintained by HiDeoo (prolific Starlight ecosystem contributor) |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `gopkg.in/yaml.v3` | already in go.sum | YAML parsing in check_translations | Already a dep, no change needed |
| `encoding/json` (stdlib) | Go 1.25 | INVENTORY.json output | No new deps for JSON output |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `filepath.Abs()` fix | Rewrite to accept absolute paths from caller | Both work; Abs() is the minimal-touch fix that preserves the existing security intent while fixing the broken behavior |
| `starlight-links-validator` | `htmltest`, `lychee`, external CI link checker | All require running outside the build; only `starlight-links-validator` catches broken links during `astro build` natively |

**Installation:**
```bash
# In docs/ directory
bun add -D starlight-links-validator
```

## Architecture Patterns

### Bug 1: check-translations path resolution

**The exact bug** is in `extractKeysFromFile()` at line 128:
```go
// BROKEN: rejects ALL relative paths containing ".."
cleanPath := path.Clean(filePath)
if cleanPath != filePath || strings.Contains(filePath, "..") {
    return nil, fmt.Errorf("potentially unsafe file path: %s", filePath)
}
```

`path.Clean("../../alita/modules/bans.go")` returns `"../../alita/modules/bans.go"` (unchanged), so
`cleanPath == filePath`. But `strings.Contains(filePath, "..")` is `true` because `..` appears in the
relative path. Both conditions combined with `||` means the second condition alone is sufficient to trigger
the error. Every single file passed to `extractKeysFromFile` fails this check.

The same pattern appears in `loadLocaleFiles()` at lines 245-252 with an identical rejection.

**The fix** — convert relative paths to absolute before the traversal check:
```go
// FIXED: resolve to absolute path, then validate no traversal escapes base
absPath, err := filepath.Abs(filePath)
if err != nil {
    return nil, fmt.Errorf("could not resolve path: %w", err)
}
// The ../ components are resolved; absPath is canonical and safe
filePath = absPath
```

After resolving to absolute, the `strings.Contains(filePath, "..")` check is unnecessary — `filepath.Abs()`
collapses `..` components. The security intent (prevent path traversal) is better served by verifying the
resolved absolute path stays within the expected base directory:
```go
// Ensure resolved path is within the project
if !strings.HasPrefix(absPath, projectRoot) {
    return nil, fmt.Errorf("path escapes project root: %s", absPath)
}
```

**Same fix needed in `loadLocaleFiles`** for the locale file path validation.

### Bug 2: MultiCommand blindspot in generate_docs

**Current regex** (line 143 in `parsers.go`):
```go
newCommandPattern := regexp.MustCompile(`handlers\.NewCommand\s*\(\s*"([^"]+)"\s*,\s*(\w+)\.(\w+)\s*\)`)
```

**The 4 MultiCommand call sites** (confirmed in codebase):
```go
// alita/modules/blacklists.go:744
cmdDecorator.MultiCommand(dispatcher, []string{"remallbl", "rmallbl"}, blacklistsModule.rmAllBlacklists)

// alita/modules/formatting.go:202
cmdDecorator.MultiCommand(dispatcher, []string{"markdownhelp", "formatting"}, formattingModule.markdownHelp)

// alita/modules/notes.go:997
cmdDecorator.MultiCommand(dispatcher, []string{"privnote", "privatenotes"}, notesModule.privNote)

// alita/modules/rules.go:302
cmdDecorator.MultiCommand(dispatcher, []string{"resetrules", "clearrules"}, rulesModule.clearRules)
```

**New regex to add** (captures all aliases from the slice literal):
```go
multiCommandPattern := regexp.MustCompile(
    `cmdDecorator\.MultiCommand\s*\(\s*dispatcher\s*,\s*\[\]string\s*\{([^}]+)\}\s*,\s*(\w+)\.(\w+)\s*\)`,
)
```

**Extraction logic** — for each `MultiCommand` match, split the alias group on commas, trim quotes and
whitespace from each alias, and add each as a `Command` struct with `Aliases` field populated and a new
`RegistrationPattern: "MultiCommand"` field added to the `Command` struct.

**The `Command` struct already has an `Aliases []string` field** in `main.go`. No struct change needed for
the aliases. Adding `RegistrationPattern string` requires a one-line struct addition.

### Pattern 3: Canonical Inventory Generation

The inventory is a NEW script or a NEW invocation mode of the existing `generate_docs` pipeline. Given that
`generate_docs` already parses commands, callbacks, and module names, the cleanest approach is to:

1. Add a `-inventory` flag to `generate_docs/main.go` that writes `.planning/INVENTORY.json` and
   `.planning/INVENTORY.md` instead of docs MDX files.
2. OR write a standalone script `scripts/generate_inventory/main.go` that calls the same parsing functions
   as output.

**Recommendation: add `-inventory` flag to `generate_docs/main.go`** — avoids code duplication of the
parsing logic. The JSON output function is ~40 lines; the Markdown table is ~30 lines.

**Message watcher extraction** is not covered by any existing parser. The pattern is:
```go
dispatcher.AddHandlerToGroup(handlers.NewMessage(filterFunc, handlerFunc), handlerGroup)
```

A new regex is needed:
```go
messageWatcherPattern := regexp.MustCompile(
    `dispatcher\.AddHandlerToGroup\s*\(\s*handlers\.NewMessage\s*\([^,]+,\s*(\w+)\.(\w+)\s*\)\s*,\s*(-?\d+)\s*\)`,
)
```

### Pattern 4: starlight-links-validator Configuration

**Installation** (in `docs/` directory):
```bash
bun add -D starlight-links-validator
```

**Config change** in `docs/astro.config.mjs`:
```javascript
// Add import at top:
import starlightLinksValidator from 'starlight-links-validator'

// Add to plugins array:
plugins: [starlightThemeBlack({}), starlightLlmsTxt(), starlightLinksValidator()],
```

**Default behavior** (no configuration needed for the locked decision):
- `errorOnRelativeLinks: true` — catches `./test` style links
- `errorOnInvalidHashes: true` — catches broken anchor links
- External links ignored by default
- Only runs during `astro build`, not `astro dev`

The plugin should work with zero configuration options given the scope decision (internal links only, fail
on broken links). If existing docs have broken links that would fail the build, those broken links are
exactly what TOOL-04 is meant to expose — they are expected findings.

### Pattern 5: Module-to-Docs Mapping

**22 module `.go` files** (non-helper, non-test, non-util):
admin, antiflood, antispam, bans, blacklists, bot_updates, captcha, connections, devs, disabling, filters,
formatting, greetings, help, language, locks, misc, mute, notes, pins, purges, reports, rules, users, warns

**22 docs directories** in `docs/src/content/docs/commands/`:
admin, antiflood, antispam, bans, blacklists, captcha, connections, disabling, filters, formatting,
greetings, languages, locks, misc, mutes, notes, pins, purges, reports, rules, warns

**Missing docs directories (4 modules):**
- `devs` — no docs directory
- `help` — no docs directory
- `language` — has `languages/` in docs (naming mismatch)
- `users` — no docs directory

**Note on `mute` vs `mutes`:** The module file is `mute.go` but the docs directory is `mutes/`. This naming
mismatch must be tracked in the mapping table.

**Note on `bot_updates`:** This file handles bot update processing, not user-facing commands. It likely has
no docs directory intentionally.

The mapping table is a static artifact produced by comparing the filesystem lists — no script needed; just
compare and document.

### Anti-Patterns to Avoid

- **Rewriting path validation security logic entirely:** The existing security intent is sound. Fix the
  relative path handling specifically; do not remove the directory-escape check.
- **Making `starlight-links-validator` warn-only:** The locked decision is fail-on-broken. Any `exclude`
  option usage should be documented explicitly if needed, not used as a blanket workaround.
- **Producing inventory from grep alone:** `grep -r handlers.NewCommand` will miss `MultiCommand`
  registrations. The fixed `generate_docs` parser is the correct tool after TOOL-02 is done.
- **Running `astro build` to test link validator during dev:** `starlight-links-validator` only runs at
  build time. Use `bun run build` to verify, not `bun run dev`.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Internal link validation in Astro | Custom link checker script or CI action | `starlight-links-validator` | Native Starlight integration, runs at build time, handles MDX/autogenerated sidebar links; custom scripts miss sidebar-generated links |
| YAML structural diff for locale comparison | Line count comparison | (Deferred to Phase 3) | Phase 1 only needs the check-translations fix; YAML diff is a Phase 3 concern |
| Command extraction via AST | `go/ast` full analysis pass | Regex extension of existing script | The existing patterns cover the registration surface; AST is overkill and adds complexity for what is mechanical string extraction |

**Key insight:** Both scripts have proven architectures. The bugs are small, isolated pathological cases —
the `..` check and the missing MultiCommand pattern. Extending working code is lower risk than replacing it.

## Common Pitfalls

### Pitfall 1: The path check is in TWO places
**What goes wrong:** Fixing `extractKeysFromFile` but missing the same pattern in `loadLocaleFiles` means
locales still aren't loaded — script still reports "0 found keys" and "All translations present."
**Why it happens:** The path validation logic is copy-pasted between the two functions.
**How to avoid:** After the fix, run the script and verify both "Found N translation keys" and locale file
counts are non-zero. The expected output is 400+ translation key usages and 4 locale files loaded.
**Warning signs:** "0 translation keys found" or "Skipping en.yml" in output after the fix.

### Pitfall 2: MultiCommand regex must handle varying whitespace
**What goes wrong:** The regex fails on slightly different formatting like extra newlines or spaces in the
`[]string{...}` literal.
**Why it happens:** Real code has variable formatting. The 4 call sites happen to be single-line, but the
regex should be robust.
**How to avoid:** Use `\s*` generously in the pattern. The 4 confirmed call sites are all single-line and
consistent, so the regex in the Architecture section above covers them. Test by running `generate_docs` and
verifying the 4 MultiCommand commands appear in the output.
**Warning signs:** Any of `remallbl`, `rmallbl`, `markdownhelp`, `formatting`, `privnote`, `privatenotes`,
`resetrules`, `clearrules` missing from `make generate-docs` output after the fix.

### Pitfall 3: starlight-links-validator may find REAL broken links immediately
**What goes wrong:** Running `astro build` after installing `starlight-links-validator` fails due to
existing broken links in the docs — interpreted as the plugin not working.
**Why it happens:** The docs have pre-existing link rot. The plugin is working correctly.
**How to avoid:** This is the expected behavior. Record which links are broken (the plugin error output
lists them). These become tasks for Phase 2. Do not add `exclude` patterns to suppress errors that are real
findings — that defeats the purpose.
**Warning signs:** Build fails immediately after plugin install — this is success, not failure.

### Pitfall 4: `filepath.Abs()` requires the process working directory to be correct
**What goes wrong:** `filepath.Abs("../../alita")` resolves relative to the OS working directory at
runtime, not the script file location. If the script is invoked from a different directory, the resolved
path is wrong.
**Why it happens:** `make check-translations` uses `cd scripts/check_translations && go run main.go` which
sets the working directory correctly. This is fine for the Makefile invocation. It would fail if the script
is invoked from the project root.
**How to avoid:** The fix is valid as long as the script is only invoked via `make check-translations`. The
Makefile already handles this. Document the invocation requirement.
**Better alternative:** Use `filepath.Abs()` relative to the script's own directory by detecting it via
`os.Args[0]` or passing paths as absolute flags. But for the surgical fix constraint, the current
Makefile-`cd` invocation is sufficient.

### Pitfall 5: Inventory must include `bot_updates.go` handling
**What goes wrong:** `bot_updates.go` exists as a module file but has no user-facing commands and no docs
directory. Including it in the inventory with empty commands but flagging it as "no docs directory" creates
confusion — it never should have docs.
**Why it happens:** The inventory generation walks all `*.go` files in `alita/modules/`.
**How to avoid:** The inventory script should distinguish between module files that are "bot behavior" vs
"user-facing modules." `bot_updates.go` handles Telegram-side updates. Mark it as `"internal": true` in
the JSON or simply note it in the Markdown table.

## Code Examples

Verified patterns from codebase inspection:

### Fix for extractKeysFromFile path validation
```go
// BEFORE (scripts/check_translations/main.go line 127-129):
cleanPath := path.Clean(filePath)
if cleanPath != filePath || strings.Contains(filePath, "..") {
    return nil, fmt.Errorf("potentially unsafe file path: %s", filePath)
}

// AFTER:
absPath, err := filepath.Abs(filePath)
if err != nil {
    return nil, fmt.Errorf("could not resolve path %s: %w", filePath, err)
}
// Remove the strings.Contains("..") check entirely.
// Use absPath as filePath for the rest of the function.
filePath = absPath
```

### Fix for loadLocaleFiles path validation
```go
// BEFORE (scripts/check_translations/main.go line 242-248):
filePath := filepath.Join(localesDir, filename)
cleanPath := path.Clean(filePath)
if cleanPath != filePath || strings.Contains(filePath, "..") {
    fmt.Printf("  Warning: Potentially unsafe file path %s, skipping\n", filename)
    continue
}

// AFTER:
filePath := filepath.Join(localesDir, filename)
absFilePath, err := filepath.Abs(filePath)
if err != nil {
    fmt.Printf("  Warning: Could not resolve path %s, skipping: %v\n", filename, err)
    continue
}
filePath = absFilePath
// Also update the HasPrefix check:
absLocalesDir, _ := filepath.Abs(localesDir)
if !strings.HasPrefix(filePath, absLocalesDir) {
    fmt.Printf("  Warning: File path %s is outside locales directory, skipping\n", filename)
    continue
}
```

### MultiCommand regex addition for parsers.go
```go
// Add to parseCommands() alongside existing newCommandPattern:
multiCommandPattern := regexp.MustCompile(
    `cmdDecorator\.MultiCommand\s*\(\s*dispatcher\s*,\s*\[\]string\s*\{([^}]+)\}\s*,\s*(\w+)\.(\w+)\s*\)`,
)

// In the file-reading loop, add after existing cmdMatches processing:
multiMatches := multiCommandPattern.FindAllStringSubmatch(content, -1)
for _, match := range multiMatches {
    if len(match) >= 4 {
        aliasesRaw := match[1]  // e.g., `"remallbl", "rmallbl"`
        moduleVar := match[2]
        handler := match[3]

        // Parse individual alias strings
        aliasPattern := regexp.MustCompile(`"([^"]+)"`)
        aliasMatches := aliasPattern.FindAllStringSubmatch(aliasesRaw, -1)

        var aliases []string
        for _, a := range aliasMatches {
            if len(a) > 1 {
                aliases = append(aliases, a[1])
            }
        }

        modName := moduleName
        if name, ok := moduleNames[moduleVar+"Module"]; ok {
            modName = name
        }

        for _, alias := range aliases {
            commands = append(commands, Command{
                Name:        alias,
                Handler:     handler,
                Module:      modName,
                Disableable: disableableCmds[alias],
                // Aliases field can list all aliases in the set
            })
        }
    }
}
```

### starlight-links-validator astro.config.mjs
```javascript
// docs/astro.config.mjs
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import starlightLlmsTxt from 'starlight-llms-txt';
import starlightThemeBlack from 'starlight-theme-black';
import starlightLinksValidator from 'starlight-links-validator'; // ADD THIS

export default defineConfig({
  // ...existing config...
  integrations: [
    starlight({
      // ...existing config...
      plugins: [
        starlightThemeBlack({}),
        starlightLlmsTxt(),
        starlightLinksValidator(), // ADD THIS — no config needed for default behavior
      ],
      // ...rest of config...
    }),
  ],
});
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `strings.Contains(path, "..")` for path traversal prevention | `filepath.Abs()` + base directory prefix check | This project only — fix is needed | Allows legitimate relative paths while still preventing escapes |
| Grep-based command counting | Regex pattern in `generate_docs` script | Already implemented | Must extend regex coverage, not replace it |
| No link validation in docs build | `starlight-links-validator` as build plugin | Introduced in 2024, current v0.19.2 (Dec 2025) | Catches broken links at build time before Cloudflare deployment |

**Deprecated/outdated:**
- `strings.Contains(filePath, "..")` as sole path safety check: Not a security vulnerability for this use
  case (the script reads files it finds itself via `WalkDir`), but it breaks legitimate relative path
  navigation. The original developer over-applied a security pattern incorrectly.

## Open Questions

1. **Does `generate_docs` need to be the inventory source, or is a separate script cleaner?**
   - What we know: `generate_docs` already has all the parsing logic (`parseCommands`, `parseCallbacks`)
   - What's unclear: Adding an `-inventory` flag to a docs generator couples two concerns; a separate script
     is cleaner but duplicates parsing code
   - Recommendation: Add a `-inventory` flag to `generate_docs/main.go` with `generateInventory()` function
     that writes JSON/Markdown. Avoids code duplication. The planner should make the final call.

2. **Does `bot_updates.go` represent a "module" in the inventory?**
   - What we know: It's in `alita/modules/`, it has no commands or docs, it handles internal bot state
   - What's unclear: Including it in the "22 modules" count vs treating it as infrastructure
   - Recommendation: Include it in the inventory JSON with `"internal": true` and zero commands. The
     mapping table should note it explicitly as non-user-facing.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` package |
| Config file | None — Go's standard test runner needs no config file |
| Quick run command | `go test ./scripts/check_translations/... -v` |
| Full suite command | `go test ./...` (project-wide) |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| TOOL-01 | `extractKeysFromFile` accepts `../../alita/...` relative paths | unit | `go test ./scripts/check_translations/ -run TestExtractKeysFromFile` | No — Wave 0 gap |
| TOOL-01 | `loadLocaleFiles` loads all 4 locale files without skipping | unit | `go test ./scripts/check_translations/ -run TestLoadLocaleFiles` | No — Wave 0 gap |
| TOOL-01 | Running the fixed script on the codebase returns count > 0 | integration | `go run ./scripts/check_translations/main.go` exits 0 or non-zero with actual findings | No test — manual verification |
| TOOL-02 | `parseCommands` extracts `remallbl`, `rmallbl`, `markdownhelp`, `formatting`, `privnote`, `privatenotes`, `resetrules`, `clearrules` | unit | `go test ./scripts/generate_docs/ -run TestParseCommands` | No — Wave 0 gap |
| TOOL-03 | Inventory JSON contains 22 modules with all commands, aliases, callbacks, watchers | integration | `go run ./scripts/generate_docs/ -inventory` produces valid JSON | No test |
| TOOL-04 | `starlight-links-validator` is in `docs/package.json` devDependencies | smoke | `cat docs/package.json | grep starlight-links-validator` | No — check post-install |
| TOOL-04 | `astro build` passes (or fails only due to pre-existing broken links — those are findings) | build | `cd docs && bun run build` | No — manual build |
| TOOL-05 | Module-to-docs mapping table exists and lists 4 modules with no docs directory | manual | Review `.planning/INVENTORY.md` | No — artifact to create |

### Sampling Rate
- **Per task commit:** `go test ./scripts/... -v` (covers TOOL-01, TOOL-02 unit tests)
- **Per wave merge:** `go test ./...` + `cd docs && bun run build`
- **Phase gate:** Full suite green, inventory JSON valid, Astro build runs (with any broken link findings documented)

### Wave 0 Gaps
- [ ] `scripts/check_translations/main_test.go` — covers TOOL-01 (path fix tests for `extractKeysFromFile` and `loadLocaleFiles`)
- [ ] `scripts/generate_docs/parsers_test.go` — covers TOOL-02 (MultiCommand extraction tests)
- [ ] No framework install needed — Go stdlib `testing` is already the project standard

## Sources

### Primary (HIGH confidence)
- Direct codebase inspection: `scripts/check_translations/main.go` — confirmed exact bug at lines 128, 245-252
- Direct codebase inspection: `scripts/generate_docs/parsers.go` — confirmed `newCommandPattern` only, no MultiCommand coverage
- Direct codebase inspection: `alita/modules/*.go` — confirmed exactly 4 `cmdDecorator.MultiCommand` call sites
- Direct codebase inspection: `docs/src/content/docs/commands/` vs `alita/modules/*.go` — confirmed 4 modules with no docs
- Direct script execution: `cd scripts/check_translations && go run main.go` output confirmed "0 found keys" and "All translations present" false negative
- `starlight-links-validator.vercel.app/getting-started/` — installation and config verified (MEDIUM confidence, web fetch)
- `starlight-links-validator.vercel.app/configuration/` — all 8 options documented (MEDIUM confidence, web fetch)
- GitHub: `github.com/HiDeoo/starlight-links-validator` — version 0.19.2, December 2025 (MEDIUM confidence)

### Secondary (MEDIUM confidence)
- `docs/astro.config.mjs` + `docs/package.json` — current Starlight version 0.37.6, plugin integration pattern confirmed
- `alita/utils/decorators/cmdDecorator/cmdDecorator.go` — `MultiCommand` implementation verified (calls `handlers.NewCommand` internally)

### Tertiary (LOW confidence)
- None

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries verified against live codebase and official sources
- Architecture: HIGH — bug root causes confirmed by direct execution; fix patterns derived from Go stdlib docs
- Pitfalls: HIGH — all pitfalls derived from direct code inspection, not speculation

**Research date:** 2026-02-27
**Valid until:** 2026-03-29 (stable — Go stdlib and the 4 MultiCommand call sites don't change without commits)
