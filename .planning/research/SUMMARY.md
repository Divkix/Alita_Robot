# Project Research Summary

**Project:** Alita Robot — Documentation and Command Consistency Audit
**Domain:** Go Telegram bot — documentation audit, command coverage, i18n verification
**Researched:** 2026-02-27
**Confidence:** HIGH

## Executive Summary

This is a documentation and consistency audit for an existing, production Go Telegram bot — not a greenfield project. The codebase has 134 registered command handlers across 22 modules, 23 callback handlers, 12 message watchers, and 4 locale files (en/es/fr/hi). The fundamental problem is a divergence between three surfaces: the Go source (authoritative source of truth), the Astro/Starlight docs site (partially stale), and the locale YAML files (structurally inconsistent). The docs claim 120 commands across 21 modules; the actual count is 134 across 22 modules. The Spanish locale has 7 keys that exist only in es.yml, meaning English users get empty string responses for those features. The Hindi locale is 403 lines shorter than English. The `make check-translations` tooling has a known path resolution bug that makes it report "0 found keys" and "All translations present" — a false clean bill of health.

The recommended audit approach is strictly "code is source of truth, docs adapt to code." No code changes except one-line trivial bug fixes; everything else goes into a behavior-issues backlog. The execution must start with establishing ground truth: a canonical command inventory extracted from Go source covering both `handlers.NewCommand()` and `cmdDecorator.MultiCommand()` registration patterns. Without this inventory, doc fixes will miss aliases and double-count primaries. Tooling gaps must be fixed before the audit proper begins — specifically the `check-translations` script path bug and the `starlight-links-validator` plugin for catching broken internal docs links.

The highest-risk item in this audit is the i18n work. The `hi.yml` locale gap (~18% missing) requires bilingual review or translation-service involvement and cannot be fixed programmatically. The locale tooling is also broken, so the gap baseline cannot even be established until the script is repaired. Everything else — command count fixes, missing command documentation, alias documentation, callback codec format updates — is LOW complexity, HIGH value work that can proceed in parallel once the command inventory is established.

## Key Findings

### Recommended Stack

The project already has two custom scripts that handle most of the audit automation: `scripts/generate_docs/` (regex-based command extraction, generates Astro MDX) and `scripts/check_translations/` (AST-based locale key extraction). Neither should be replaced — both should be extended. The `generate_docs` script has a specific blindspot: it only matches `handlers.NewCommand(...)` via regex and misses all `cmdDecorator.MultiCommand(...)` alias registrations. Adding a second regex pattern is a 10-line fix, not a rewrite.

**Core technologies:**
- `go/ast` + `regexp` (stdlib): Command and locale key extraction — already integrated, zero deps, covers 138 of 138 registrations if MultiCommand blindspot is patched
- `gopkg.in/yaml.v3`: YAML parsing in audit scripts and bot locale loading — already a project dependency
- `starlight-links-validator`: Internal link validation at Astro build time — catches broken cross-references before Cloudflare deployment
- `dyff` (homebrew): Structural YAML diff for cross-locale comparison — semantically better than line-count comparison; reports by key path
- `ast-grep` (ad-hoc): One-off structural code search during investigation — faster than writing a Go analysis pass for audit queries
- `make` targets: Orchestration — add `check-docs` and `check-locale-sync` alongside existing targets
- Astro/Starlight (existing): Docs site — do not change; `make docs-dev` for preview

**What NOT to use:** `go-i18n` (wrong call pattern for this project), `i18n-linter` (hardcoded for `i18n.T()` not `tr.GetString()`), full `golang.org/x/tools/go/analysis` framework (overkill for literal string extraction), any cloud-based link checkers (unnecessary when `starlight-links-validator` covers it at build time).

### Expected Features

The audit has a clear MVP boundary. The P1 work eliminates active user confusion caused by factually wrong documentation. The P2 work reduces operator/admin confusion. P3 is quality improvement that can be deferred.

**Must have (table stakes — P1):**
- Command count accuracy: fix stale "120 commands / 21 modules" claim (actual: 134 commands, 22 modules)
- Missing commands in API reference: `start`, `help`, `donate`, `about`, `formatting`/`markdownhelp`, `remallbl`/`rmallbl`, `privnote`/`privatenotes`, dev-tier commands (`addsudo`, `adddev`, `remsudo`, `remdev`, `teamusers`, `chatinfo`, `chatlist`, `leavechat`, `stats`)
- Alias documentation: every command that is an alias must say so explicitly and point to the primary
- Disableable column accuracy: verify `AddCmdToDisableable` calls in code against the checkmark column in docs
- Callback codec format: `callbacks.md` documents old dot-notation; actual format is `namespace|v1|url-encoded-fields`
- Fix `check-translations` script path bug before any i18n work proceeds

**Should have (v1.x — reduces operator confusion):**
- Hi locale gap remediation (~18% missing, requires bilingual review — highest-effort item)
- Fr locale gap remediation (~5% missing)
- Message watcher precedence documentation (which handler fires first: antiflood, antispam, blacklists, filters)
- Anonymous admin verification flow diagram (Mermaid, supported by Starlight)
- Dev/owner commands section with explicit "owner-only, not surfaced to regular users" note

**Defer (v2+):**
- Per-command error message i18n audit (high effort, low-frequency impact)
- Consolidated permission matrix by command (useful reference, not blocking)
- Filter vs blacklist interaction documentation (needs testing to confirm behavior first)
- `setMyCommands` scope audit (requires checking live bot configuration)

**Explicit anti-features (do not do these during audit):**
- Rewriting the inline help system — document as-is, flag for future improvement
- Adding new locale languages — out of scope, stick to en/es/fr/hi
- Changing command syntax for consistency — breaking changes, out of scope
- Generating docs from code comments — mixing generated and human-authored MDX creates maintenance problems

### Architecture Approach

The audit has three independent surfaces that must be brought into sync, all deriving from the Go source as single source of truth. The surfaces do NOT need to be kept textually identical — they have different formatting requirements (Telegram HTML vs Astro MDX). They must be factually consistent: same commands, same permissions, same behavior described. The fix propagation rule is unidirectional: code → docs/locales. Never reverse.

**Major components:**
1. **Command Inventory** (foundation): Extract all `NewCommand()` + `MultiCommand()` registrations from `alita/modules/*.go` into a structured per-module list. Every other component's audit is gated on this. No editing any surface until this inventory exists.
2. **Docs Site Audit** (`docs/src/content/docs/commands/`): 21 module directories mapped against 22 module Go files. Four modules (`devs.go`, `help.go`, `language.go`, `users.go`) have no docs directory. Per-module audit unit: commands in code vs docs, aliases, permissions, disableable flags, callback handler coverage.
3. **i18n Locale Audit** (`locales/en.yml` as canonical): Fix tooling first, then establish gap baseline, then fix EN naming bugs (e.g., `devs_getting_chatlist` vs `devs_getting_chat_list`), then propagate to other locales. ES has 7 orphan keys not in EN — remove them. EN locale gap = bug for all users, not just non-English users.
4. **README Update**: Lowest priority. Fix wrong directory structure (`cmd/` does not exist), update command count, trim the 35-command list or point to the docs site instead.

**Key architectural constraint:** The `make check-translations` script is broken (path resolution failure returns 0 findings). All i18n audit work is blocked on fixing this first.

### Critical Pitfalls

1. **Command count drift in static documentation** — Badge components in `index.mdx` hardcode counts that are already wrong (blacklists claims 8, actual is 7; notes claims 10, actual is 8). Never eyeball-fix these counts; use script output. Prevention: establish command inventory script before touching any Badge text.

2. **Alias blindspot in naive grep** — `cmdDecorator.MultiCommand()` registrations do not appear in a grep for `handlers.NewCommand`. Any audit that only scans for `NewCommand` will miss aliased commands (`remallbl`/`rmallbl`, `formatting`/`markdownhelp`, etc.) and produce an incomplete inventory. Prevention: patch the `generate_docs` script to cover both patterns, then use its output as the canonical command list.

3. **Broken check-translations script giving false confidence** — `make check-translations` reports "0 found keys" and "All translations present" due to a path resolution bug. This is a silent failure that masks real gaps. Prevention: fix the script path issue before any i18n work; supplement with `dyff` or a Python YAML key diff until the script is repaired.

4. **Docs directory presence does not mean documentation completeness** — The Astro sidebar autogenerates from any directory with an `index.mdx`. Four modules have no docs directory at all; others have docs that cover only some registered commands. Prevention: build a mapping table (module file → docs path → commands documented) before any content editing.

5. **Scope creep from "document behavior" to "fix behavior"** — The audit will surface behavior issues in Go handlers. Fixing them is out of scope (except trivial one-liners). Prevention: maintain a separate "behavior issues found" log; review it at each phase boundary; do not commit Go handler file changes without explicit scope approval.

6. **Conflating inline help format with docs site format** — Locale YAML uses Markdown; the bot renders HTML via `tgmd2html.MD2HTMLV2()`. Docs site uses Astro MDX. Copy-pasting between surfaces creates formatting bugs. Prevention: treat each surface as an independent artifact; verify factual consistency, not textual identity.

## Implications for Roadmap

Based on the dependency graph discovered in research, the audit must proceed in strict phases. The first phase is entirely tooling and inventory — no documentation writing until ground truth exists. Subsequent phases can overlap across modules but not across surfaces (don't mix i18n fixes with docs fixes in the same commit).

### Phase 1: Ground Truth and Tooling
**Rationale:** Every other phase depends on the command inventory and working tooling. Starting anywhere else produces wrong results that require rework. The `check-translations` bug must be fixed before any i18n work. The MultiCommand blindspot must be patched before any docs work.
**Delivers:** Canonical command inventory (all 22 modules, all registration patterns), working `check-translations` output, `dyff`-based locale gap baseline, `starlight-links-validator` integrated into Astro build, module-to-docs mapping table.
**Addresses:** Pitfall 1 (command count drift), Pitfall 2 (alias blindspot), Pitfall 3 (broken tooling).
**No research-phase needed:** Patterns are well-understood; work is mechanical code extension of existing scripts.

### Phase 2: API Reference and Command Documentation
**Rationale:** This is the highest-ROI phase — LOW complexity, HIGH user value. Can proceed immediately after Phase 1 inventory exists. Module docs are independent MDX files; parallel work across modules is safe.
**Delivers:** Accurate `commands.md` with all 134 commands, correct alias documentation, corrected Badge counts in `index.mdx`, new docs sections for `devs.go`/`help.go`/`language.go`/`users.go` modules, updated `callbacks.md` with versioned codec format.
**Addresses:** FEATURES.md P1 items — command count accuracy, missing commands, alias documentation, disableable column, callback codec format.
**Research flag:** NONE — the command inventory from Phase 1 is the complete spec. No domain research needed.

### Phase 3: Locale and i18n Fixes
**Rationale:** Can overlap with Phase 2 since locale audit is independent of docs audit. But EN locale key naming bugs must be fixed before propagating to other locales. The ES orphan keys must be removed before adding any new keys.
**Delivers:** EN locale cleaned (naming bugs fixed, missing keys added for `devs_getting_chat_list` class bugs), ES orphan keys removed, FR locale gap remediated (~5% missing), HI locale gap enumerated and translated (requires external contribution or translation service).
**Addresses:** FEATURES.md P1 (fix check-translations tooling), P2 (hi/fr locale gaps).
**Research flag:** HI locale translation requires bilingual Hindi reviewer — this may block Phase 3 completion. Flag as potentially needing external contributor.

### Phase 4: Operator Documentation
**Rationale:** Admin/operator confusion (watcher precedence, anonymous admin flow, connection scope) is real but less blocking than factual inaccuracies. Builds on Phase 2's module docs infrastructure.
**Delivers:** Message watcher precedence page (handler groups -2 through +10 explained), anonymous admin Mermaid sequence diagram in `admin/index.mdx`, connection module scope clarification, filter vs blacklist interaction documented (after behavior confirmed by testing).
**Addresses:** FEATURES.md P2 items — watcher precedence, anonymous admin flow, connection scope clarity.
**Research flag:** Filter vs blacklist interaction needs live bot testing before documentation can be accurate. Flag for validation before committing docs on this topic.

### Phase 5: README and Final Verification
**Rationale:** README is lowest priority and last to fix. Final verification confirms the audit is complete across all surfaces and BotFather command list matches the documented command set.
**Delivers:** Corrected README (directory structure, command count, feature list), verified BotFather `/setcommands` matches docs, confirmed `make check-translations` passes clean, confirmed Astro build passes with `starlight-links-validator`.
**Addresses:** "Looks Done But Isn't" checklist from PITFALLS.md.
**Research flag:** NONE — mechanical verification work.

### Phase Ordering Rationale

- Phase 1 must be first because every subsequent phase is gated on its outputs (inventory, working tools).
- Phases 2 and 3 can be parallelized across contributors but not within a single contributor session (context switching between code-reading and YAML editing is wasteful).
- Phase 4 follows Phase 2 because operator docs build on the module docs structure established in Phase 2.
- Phase 5 is always last because it verifies the cumulative output of all previous phases.
- The hi locale translation work in Phase 3 is the only item with an external dependency (bilingual contributor). Start it early in Phase 3 to avoid blocking final delivery.

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 3 (HI locale):** Hindi translation requires bilingual review — cannot be done by code analysis alone. If no bilingual contributor is available, consider translation service + native review pass.
- **Phase 4 (filter vs blacklist interaction):** Behavior must be confirmed by testing before documenting. Write a test case in a dev group; observe which watcher fires. Document the observed result, not what the code suggests.

Phases with standard patterns (skip research-phase):
- **Phase 1:** Script extension patterns are straightforward Go regexp additions to existing code.
- **Phase 2:** Doc writing follows the established pattern of existing well-documented modules (e.g., `bans`, `notes`).
- **Phase 5:** Mechanical verification — no research needed.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All recommendations verified against live codebase. Existing scripts confirmed working (minus the path bug). Third-party tools (`dyff`, `starlight-links-validator`, `ast-grep`) verified via GitHub/npm. |
| Features | HIGH | Command counts derived from direct codebase grep. Locale line counts from direct measurement. P1/P2/P3 prioritization based on user impact, not speculation. |
| Architecture | HIGH | Entirely derived from live codebase inspection — no external sources required. Module-to-docs mapping, locale key structure, and data flow all verified directly. |
| Pitfalls | HIGH | All 6 critical pitfalls identified from direct codebase evidence: the broken script is confirmed, the MultiCommand blindspot is confirmed, the ES orphan keys are confirmed, the missing module docs directories are confirmed. |

**Overall confidence:** HIGH

### Gaps to Address

- **Hi locale translation gap baseline:** The exact list of missing keys will only be known after fixing `check-translations` (Phase 1). The 403-line deficit is confirmed but whether it maps to missing keys vs structural differences requires tooling to verify. Handle by: fix tooling first, run diff, enumerate exact missing keys before assigning translation work.
- **BotFather command list current state:** The audit scope includes verifying the `/setcommands` list matches registered commands. The `setMyCommands` call location in the codebase needs to be identified and its contents compared to the command inventory. Handle during Phase 5 verification.
- **Filter vs blacklist interaction behavior:** Undocumented and requires live testing to confirm which handler group fires first and what action wins. Handle in Phase 4 by writing a concrete test case and observing behavior before writing docs.
- **`language.go` and `users.go` module scope:** These modules have no docs directories. The exact commands they register and their intended audience (user-facing vs admin-only vs dev-only) affects what docs to write. Handle at Phase 2 start by pulling their handler registrations from the command inventory.

## Sources

### Primary (HIGH confidence)
- `alita/modules/*.go` — command/callback/watcher registrations (direct codebase inspection)
- `scripts/generate_docs/parsers.go` — confirmed MultiCommand blindspot
- `scripts/check_translations/main.go` — confirmed path resolution bug
- `locales/*.yml` — direct line count and key structure comparison
- `docs/src/content/docs/commands/` — module coverage gaps confirmed
- `.planning/codebase/ARCHITECTURE.md`, `.planning/codebase/STRUCTURE.md` — system architecture
- `.planning/PROJECT.md` — audit scope and constraints
- `pkg.go.dev/go/ast` — stdlib AST package (official docs)
- `pkg.go.dev/golang.org/x/tools/go/analysis` — official analysis framework docs

### Secondary (MEDIUM confidence)
- [HiDeoo/starlight-links-validator GitHub](https://github.com/HiDeoo/starlight-links-validator) — Dec 2025 release, Starlight v0.37.x compatibility
- [homeport/dyff GitHub](https://github.com/homeport/dyff) — YAML structural diff capabilities
- [ast-grep Go catalog](https://ast-grep.github.io/catalog/go/) — Go pattern syntax, function-context workaround
- [Telegram Bot Commands API](https://core.telegram.org/api/bots/commands) — command limits, setMyCommands behavior
- [BotFather command description best practices](https://telegramhpc.com/news/63/) — 22-char mobile truncation limit

### Tertiary (LOW confidence)
- [Telegram Bot UX best practices](https://medium.com/@bsideeffect/10-best-ux-practices-for-telegram-bots-79ffed24b6de) — single source, general guidance only
- [grammY i18n plugin fallback pattern](https://grammy.dev/plugins/i18n) — different ecosystem (JS), extrapolated fallback logic

---
*Research completed: 2026-02-27*
*Ready for roadmap: yes*
