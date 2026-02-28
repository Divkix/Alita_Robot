# Roadmap: Alita Robot — Documentation & Command Consistency Audit

## Overview

This audit brings every user-facing surface (docs site, inline help, README, locale files) into accurate alignment with the Go source code, which is the single source of truth. The work proceeds in strict dependency order: establish ground truth with working tooling first, then fix documentation surfaces, then fix locale files, then document operator behavior, then verify everything passes clean. Each phase delivers verifiable, observable improvements to what users and operators see.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [ ] **Phase 1: Ground Truth and Tooling** - Fix broken tooling, patch script blindspots, produce canonical command inventory
- [ ] **Phase 2: API Reference and Command Documentation** - Fix all command documentation to match actual registered handlers
- [ ] **Phase 3: Locale and i18n Fixes** - Clean EN locale, remove ES orphans, remediate FR and HI gaps
- [ ] **Phase 4: Operator Documentation** - Document message watcher precedence, anonymous admin flow, dev commands
- [ ] **Phase 5: README and Final Verification** - Fix README, confirm all surfaces pass verification tooling

## Phase Details

### Phase 1: Ground Truth and Tooling
**Goal**: Tooling works correctly and a canonical command inventory exists — every subsequent phase is gated on these outputs
**Depends on**: Nothing (first phase)
**Requirements**: TOOL-01, TOOL-02, TOOL-03, TOOL-04, TOOL-05
**Success Criteria** (what must be TRUE):
  1. `make check-translations` reports actual missing/orphan keys instead of "0 found keys" (the path resolution bug is fixed)
  2. The canonical command inventory lists all 22 modules with every command, alias, callback, message watcher, and disableable status — including `cmdDecorator.MultiCommand()` registrations that the old script missed
  3. `starlight-links-validator` is integrated into the Astro docs build and catches broken internal links at build time
  4. A module-to-docs mapping table exists showing which of the 22 modules have docs directories and which do not
**Plans**: TBD

Plans:
- [ ] 01-01: Fix `make check-translations` path resolution bug in `scripts/check_translations/main.go`
- [ ] 01-02: Patch `scripts/generate_docs/parsers.go` to extract `cmdDecorator.MultiCommand()` registrations
- [ ] 01-03: Produce canonical command inventory (all 22 modules, all registration patterns)
- [ ] 01-04: Install and configure `starlight-links-validator` in Astro docs build
- [ ] 01-05: Build module-to-docs mapping table

### Phase 2: API Reference and Command Documentation
**Goal**: Every registered command and alias is accurately documented in the docs site with correct permissions, disableable status, and callback codec format
**Depends on**: Phase 1
**Requirements**: DOCS-01, DOCS-02, DOCS-03, DOCS-04, DOCS-05, DOCS-06, DOCS-07
**Success Criteria** (what must be TRUE):
  1. Badge counts in `docs/src/content/docs/commands/index.mdx` match the canonical command inventory (no stale "120 commands / 21 modules" claims)
  2. `api-reference/commands.md` lists all 134 commands including previously missing entries (`start`, `help`, `donate`, `about`, dev-tier commands, and all aliases)
  3. Every alias in the docs explicitly names its primary command (e.g., `/addfilter` documented as alias of `/filter`)
  4. Docs exist for all four previously undocumented modules (`devs`, `help`, `language`, `users`)
  5. `callbacks.md` documents the versioned codec format (`namespace|v1|url-encoded-fields`) not the old dot-notation
**Plans**: TBD

Plans:
- [ ] 02-01: Fix stale command count badges in `index.mdx` using canonical inventory output
- [ ] 02-02: Add all missing commands to `api-reference/commands.md`
- [ ] 02-03: Document all command aliases with explicit primary-alias relationships
- [ ] 02-04: Verify and correct disableable column against actual `AddCmdToDisableable()` calls
- [ ] 02-05: Update `callbacks.md` with versioned codec format
- [ ] 02-06: Create docs directories and content for `devs`, `help`, `language`, `users` modules
- [ ] 02-07: Verify permission requirements in docs match actual `Require*` calls in each handler

### Phase 3: Locale and i18n Fixes
**Goal**: All four locale files (en/es/fr/hi) are internally consistent, cross-locale gaps are remediated, and `make check-translations` passes clean
**Depends on**: Phase 1
**Requirements**: I18N-01, I18N-02, I18N-03, I18N-04, I18N-05, I18N-06
**Success Criteria** (what must be TRUE):
  1. EN locale has no naming inconsistencies (e.g., `devs_getting_chatlist` is renamed to `devs_getting_chat_list` with all references updated)
  2. ES locale has no orphan keys — all 7 confirmed ES-only keys removed or added to EN
  3. FR locale gap (~5% missing keys vs EN) is fully remediated
  4. HI locale missing keys are enumerated and translated (the ~18% gap is resolved or explicitly deferred with each missing key documented)
  5. `make check-translations` passes with 0 errors, 0 orphans across all four locales
**Plans**: TBD

Plans:
- [ ] 03-01: Fix EN locale naming inconsistencies and add missing keys referenced in code
- [ ] 03-02: Remove ES locale orphan keys (7 confirmed) and align ES to EN
- [ ] 03-03: Remediate FR locale gap (~5% missing keys)
- [ ] 03-04: Enumerate and remediate HI locale gap (~18% missing keys)
- [ ] 03-05: Verify `make check-translations` passes clean across all four locales

### Phase 4: Operator Documentation
**Goal**: Admins and operators understand how message watchers interact, how anonymous admin verification works, and where developer-only commands are documented
**Depends on**: Phase 2
**Requirements**: OPER-01, OPER-02, OPER-03
**Success Criteria** (what must be TRUE):
  1. A docs page exists explaining message watcher handler group precedence — which fires first (antispam at -2, captcha at -10, antiflood, blacklists, filters, locks) and what happens when multiple match
  2. The anonymous admin flow is documented with a Mermaid sequence diagram showing the keyboard fallback process visible in the docs site
  3. A developer/owner commands section exists with explicit "owner-only, not surfaced to regular users" access notes covering all dev-tier commands
**Plans**: TBD

Plans:
- [ ] 04-01: Document message watcher handler group precedence with handler group number table
- [ ] 04-02: Document anonymous admin verification flow with Mermaid sequence diagram
- [ ] 04-03: Add developer/owner commands section with explicit access level documentation

### Phase 5: README and Final Verification
**Goal**: README accurately reflects the codebase and all surfaces pass automated verification tooling
**Depends on**: Phases 2, 3, 4
**Requirements**: VRFY-01, VRFY-02, VRFY-03, VRFY-04
**Success Criteria** (what must be TRUE):
  1. README project structure diagram has no references to nonexistent directories (the `cmd/` directory reference is removed)
  2. README command count and feature descriptions match the canonical inventory (no "120 commands / 21 modules" stale claims)
  3. Astro docs build passes clean with `starlight-links-validator` — zero broken internal links reported
  4. `make generate-docs` output matches the manually verified docs (generated and hand-authored content are consistent)
**Plans**: TBD

Plans:
- [ ] 05-01: Fix README project structure diagram (remove `cmd/` reference, correct module count)
- [ ] 05-02: Update README command count and feature descriptions
- [ ] 05-03: Confirm Astro docs build passes with `starlight-links-validator`
- [ ] 05-04: Run `make generate-docs` and verify output consistency with manual fixes

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5
Note: Phases 2 and 3 both depend on Phase 1 and can be parallelized if multiple sessions are available.

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Ground Truth and Tooling | 0/5 | Not started | - |
| 2. API Reference and Command Documentation | 0/7 | Not started | - |
| 3. Locale and i18n Fixes | 0/5 | Not started | - |
| 4. Operator Documentation | 0/3 | Not started | - |
| 5. README and Final Verification | 0/4 | Not started | - |
