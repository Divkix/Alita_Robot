# Requirements: Alita Robot Documentation & Command Consistency Audit

**Defined:** 2026-02-27
**Core Value:** Every user-facing surface accurately describes what the bot actually does, and every command behaves consistently and predictably.

## v1 Requirements

Requirements for this audit. Each maps to roadmap phases.

### Tooling

- [ ] **TOOL-01**: Fix `make check-translations` script path resolution bug so it correctly reports missing/orphan locale keys
- [ ] **TOOL-02**: Patch `generate_docs` parsers.go to extract `cmdDecorator.MultiCommand()` registrations in addition to `handlers.NewCommand()`
- [ ] **TOOL-03**: Produce canonical command inventory covering all 22 modules with commands, aliases, callbacks, message watchers, and disableable status
- [ ] **TOOL-04**: Install and configure `starlight-links-validator` plugin in Astro docs build to catch broken internal links
- [ ] **TOOL-05**: Create module-to-docs mapping table showing which modules have docs directories and which don't

### Command Documentation

- [ ] **DOCS-01**: Fix stale command count badges in `docs/src/content/docs/commands/index.mdx` to match actual registered handler counts per module
- [ ] **DOCS-02**: Add all missing commands to `api-reference/commands.md` (14+ missing: `start`, `help`, `donate`, `about`, `formatting`/`markdownhelp`, `remallbl`/`rmallbl`, `privnote`/`privatenotes`, dev-tier commands)
- [ ] **DOCS-03**: Document all command aliases explicitly — each alias points to its primary command with explanation
- [ ] **DOCS-04**: Verify and correct disableable column in docs against actual `AddCmdToDisableable()` calls in code
- [ ] **DOCS-05**: Update `callbacks.md` to document versioned codec format (`namespace|v1|url-encoded-fields`) instead of old dot-notation
- [ ] **DOCS-06**: Create docs directory and content for undocumented modules (`devs`, `help`)
- [ ] **DOCS-07**: Verify permission requirements listed in docs match actual `Require*` calls in each handler

### Locale Consistency

- [ ] **I18N-01**: Fix EN locale key naming inconsistencies (e.g., `devs_getting_chatlist` vs `devs_getting_chat_list`)
- [ ] **I18N-02**: Remove orphan keys from ES locale that don't exist in EN (7 confirmed orphans)
- [ ] **I18N-03**: Add missing locale keys to EN that exist in code but not in any locale file
- [ ] **I18N-04**: Remediate FR locale gap (~5% missing keys compared to EN)
- [ ] **I18N-05**: Enumerate and remediate HI locale gap (~18% missing keys compared to EN)
- [ ] **I18N-06**: Verify `make check-translations` passes clean after all locale fixes

### Operator Documentation

- [ ] **OPER-01**: Document message watcher handler group precedence (which fires first: antispam at -2, captcha at -10, antiflood, blacklists, filters, locks)
- [ ] **OPER-02**: Document anonymous admin verification flow with sequence diagram
- [ ] **OPER-03**: Add developer/owner commands section with explicit "owner-only" access note

### README and Verification

- [ ] **VRFY-01**: Fix README project structure diagram (remove references to nonexistent `cmd/` directory)
- [ ] **VRFY-02**: Update README command count and feature descriptions to match actual codebase
- [ ] **VRFY-03**: Confirm Astro docs build passes clean with `starlight-links-validator`
- [ ] **VRFY-04**: Run regenerated docs through `make generate-docs` and verify output matches manual fixes

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Advanced Documentation

- **ADV-01**: Per-command error message i18n audit (high effort, low-frequency impact)
- **ADV-02**: Consolidated permission matrix by command (useful reference, not blocking)
- **ADV-03**: Filter vs blacklist interaction documentation (needs live testing to confirm behavior first)
- **ADV-04**: `setMyCommands` scope audit (requires checking live bot BotFather configuration)
- **ADV-05**: `language.go` and `users.go` module docs (determine if they should have separate docs pages)

## Out of Scope

| Feature | Reason |
|---------|--------|
| Adding new bot features or commands | Audit only — code is source of truth |
| Redesigning docs site theme/layout | Content accuracy only, not design |
| Rewriting inline help system architecture | Document as-is, flag for future |
| Adding new locale languages beyond en/es/fr/hi | Fix existing languages first |
| Changing command syntax for consistency | Breaking changes out of scope |
| Generating docs from code comments | Mixing generated and human-authored MDX creates maintenance problems |
| Performance optimization of commands | UX and correctness only |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| TOOL-01 | Phase 1 | Pending |
| TOOL-02 | Phase 1 | Pending |
| TOOL-03 | Phase 1 | Pending |
| TOOL-04 | Phase 1 | Pending |
| TOOL-05 | Phase 1 | Pending |
| DOCS-01 | Phase 2 | Pending |
| DOCS-02 | Phase 2 | Pending |
| DOCS-03 | Phase 2 | Pending |
| DOCS-04 | Phase 2 | Pending |
| DOCS-05 | Phase 2 | Pending |
| DOCS-06 | Phase 2 | Pending |
| DOCS-07 | Phase 2 | Pending |
| I18N-01 | Phase 3 | Pending |
| I18N-02 | Phase 3 | Pending |
| I18N-03 | Phase 3 | Pending |
| I18N-04 | Phase 3 | Pending |
| I18N-05 | Phase 3 | Pending |
| I18N-06 | Phase 3 | Pending |
| OPER-01 | Phase 4 | Pending |
| OPER-02 | Phase 4 | Pending |
| OPER-03 | Phase 4 | Pending |
| VRFY-01 | Phase 5 | Pending |
| VRFY-02 | Phase 5 | Pending |
| VRFY-03 | Phase 5 | Pending |
| VRFY-04 | Phase 5 | Pending |

**Coverage:**
- v1 requirements: 25 total
- Mapped to phases: 25
- Unmapped: 0 ✓

---
*Requirements defined: 2026-02-27*
*Last updated: 2026-02-27 after initial definition*
