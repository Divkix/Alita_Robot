# Phase 1 Context: Ground Truth and Tooling

**Phase Goal:** Fix broken tooling, patch script blindspots, produce canonical command inventory
**Created:** 2026-02-27
**Mode:** Auto — decisions derived from research findings

## Decisions

### 1. Script Fix Approach

**Decision:** Fix both scripts in-place with minimal, surgical patches. No rewrites.

**check-translations fix:**
- The path resolution bug in `scripts/check_translations/main.go` causes it to report "0 found keys" and falsely claim "All translations present"
- Fix the path resolution so it correctly walks the `locales/` directory and `alita/` source tree
- This is a one-line to few-line fix — diagnose the exact path issue, patch it, verify output changes from 0 to actual counts

**parsers.go fix:**
- `scripts/generate_docs/parsers.go` only matches `handlers.NewCommand(...)` via regex
- Add a second regex pattern to also match `cmdDecorator.MultiCommand(dispatcher, []string{...}, handler)` registrations
- There are exactly 4 `MultiCommand` call sites in the codebase — the regex must capture all alias strings from the slice literal
- ~10 line addition, not a rewrite. No AST parsing needed — regex is sufficient for the registration patterns used

**Rationale:** Research confirmed both fixes are mechanical. The existing scripts work correctly for their covered patterns; they just have blindspots. Extending coverage is lower-risk than replacing.

### 2. Inventory Output Format

**Decision:** JSON as primary machine-consumable format, with a Markdown summary table for human review.

**JSON structure (per module):**
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

**Markdown table:** Summary view with columns: Module | Commands | Aliases | Callbacks | Watchers | Disableable | Has Docs

**Output location:** `.planning/INVENTORY.json` (machine) and `.planning/INVENTORY.md` (human)

**Rationale:** Phase 2 needs structured data to diff against docs. JSON is grep-able and script-consumable. The Markdown table is for human spot-checking during review.

### 3. Docs Link Validation Scope

**Decision:** Internal links only. Fail build on broken links (not warn-only).

**What to validate:**
- Internal cross-references between docs pages (the primary source of link rot)
- Anchor links within pages
- Sidebar-generated links matching actual file paths

**What NOT to validate:**
- External URLs (GitHub, Telegram API docs, etc.) — these are stable and checking them adds network dependency to builds
- Image/asset links (Starlight handles these at build time already)

**Integration:**
- Add `starlight-links-validator` as a dev dependency in `docs/package.json`
- Configure in `docs/astro.config.mjs` as a Starlight plugin
- Build fails if any internal link is broken — this catches issues before Cloudflare deployment

**Rationale:** Research identified broken internal links as the primary link-rot risk. External URL checking adds build fragility (network timeouts, rate limits) for minimal benefit.

### 4. Tooling Integration

**Decision:** New Make targets for the fixed scripts. Inventory is a planning artifact, not a build artifact.

**New Make targets:**
- `make check-docs` — runs the patched `generate_docs` script and diffs output against existing docs (smoke test for drift)
- Existing `make check-translations` — already exists, just needs the path bug fixed
- Existing `make generate-docs` — already exists, just needs MultiCommand support added

**Inventory location:** `.planning/INVENTORY.json` and `.planning/INVENTORY.md` — these are audit artifacts, not checked into the docs build pipeline. They're consumed by humans and by Phase 2 planning, not by CI.

**No CI integration in this phase:** Adding GitHub Actions workflows is scope creep. The Make targets are sufficient for local development verification. CI integration can be a future enhancement.

**Rationale:** The audit is about establishing ground truth, not building a CI pipeline. Make targets are the project's existing orchestration pattern. Adding CI workflows would be a new capability, not an audit fix.

## Deferred Ideas

None identified — all 4 areas resolved cleanly within phase scope.

## Constraints Carried Forward

- `make check-translations` must pass clean BEFORE any Phase 3 i18n work begins
- The canonical inventory must exist BEFORE any Phase 2 docs work begins
- `starlight-links-validator` must be integrated BEFORE Phase 5 final verification

---
*Context established: 2026-02-27*
*Ready for: planning*
