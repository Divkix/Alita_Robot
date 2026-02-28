# Alita Robot: Documentation & Command Consistency Audit

## What This Is

A systematic audit and remediation of Alita Robot's user-facing documentation surfaces — now complete. The bot's 142 commands across 25 modules, Astro/Starlight documentation site, inline help system, 4 locale files, and README have all been brought into alignment with the Go source code. A sentinel-protected docs generator prevents future drift.

## Core Value

Every user-facing surface (docs site, help text, README, bot responses) accurately describes what the bot actually does, and every command behaves consistently and predictably.

## Requirements

### Validated

- ✓ 142 bot commands across 25 modules — inventoried and documented — v1.0
- ✓ Astro/Starlight documentation site with per-module command docs — existing
- ✓ Inline help system via /help with keyboard navigation — existing
- ✓ Multi-language support (en, es, fr, hi) with YAML locale files — existing
- ✓ README with project overview, installation, and commands — existing
- ✓ CLAUDE.md with architecture documentation — existing
- ✓ API reference docs (commands, callbacks, database, environment, permissions) — existing
- ✓ Self-hosting guides (database, monitoring, profiling, troubleshooting, webhooks) — existing
- ✓ Makefile with 17 development/deployment targets — existing
- ✓ GoDoc-compatible function documentation — existing
- ✓ Docs site command reference matches actual registered commands 1:1 — v1.0
- ✓ All command aliases documented — v1.0
- ✓ Disableable vs non-disableable commands accurately marked — v1.0
- ✓ Callback handlers documented with versioned codec format — v1.0
- ✓ Message watcher/background handler behavior documented — v1.0
- ✓ README bot commands section aligned with canonical inventory — v1.0
- ✓ All locale files have consistent keys (838 keys, zero orphans) — v1.0
- ✓ Command permission requirements documented — v1.0
- ✓ Anonymous admin flow documented with Mermaid diagram — v1.0
- ✓ Handler group system explained with 11-row precedence table — v1.0
- ✓ Developer/owner commands documented with 3-tier access model — v1.0
- ✓ Docs generator sentinel-protected (23 files) against overwrites — v1.0

### Active

- [ ] Per-command error message i18n audit (high effort, low-frequency impact)
- [ ] Consolidated permission matrix by command
- [ ] Filter vs blacklist interaction documentation (needs live testing)
- [ ] `setMyCommands` scope audit (requires live BotFather configuration)
- [ ] Help text in bot matches docs site descriptions
- [ ] Each command's response format/behavior verified against docs
- [ ] UX issues in command responses identified and fixed
- [ ] Bot command descriptions (BotFather) match actual behavior

### Out of Scope

- Adding new bot features or commands — audit only
- Redesigning the documentation site theme/layout — content only
- Rewriting the inline help system architecture — document as-is
- Performance optimization of commands — UX and correctness only
- Database schema changes — no structural modifications
- New locale/language additions — fix existing 4 languages only
- Generating docs from code comments — mixing generated and human-authored MDX creates maintenance problems

## Context

Shipped v1.0 audit milestone with 77 commits across 113 files (+15,648/-1,105 lines).
Tech stack: Go 1.25+ bot, Astro/Starlight docs, 4 YAML locale files.
Docs generator hardened with sentinel comment protection for 23 files (2 api-reference + 21 module pages).
All 4 locale files consistent at 838 keys with zero orphans across EN/ES/FR/HI.
Astro docs build clean with starlight-links-validator: 52 pages, all internal links valid.
Canonical command inventory: 142 commands, 25 modules, tracked in INVENTORY.json.

## Constraints

- **Tech stack**: Go 1.25+ bot, Astro/Starlight docs — no framework changes
- **Locale format**: YAML with named parameters mapped to positional formatters
- **Callback data**: 64-byte Telegram limit — cannot add verbose descriptions
- **Command limit**: BotFather limits command descriptions to 256 chars
- **Backward compatibility**: No breaking changes to existing command syntax
- **Sentinel protection**: Files with `<!-- MANUALLY MAINTAINED -->` sentinel must not be overwritten by generators

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Audit docs against code, not code against docs | Code is the source of truth; docs must reflect reality | ✓ Good — all 142 commands documented from Go source |
| Fix locale consistency across all 4 languages | Missing keys cause runtime fallback to English silently | ✓ Good — 838 keys consistent, zero orphans |
| Document handler groups and message watchers | Users confused by "invisible" actions (filter matches, flood limits) | ✓ Good — 11-row precedence table + Mermaid diagram |
| Use sentinel comment in protected files | Self-contained, no external config; generator skips marked files | ✓ Good — 23 files protected, clean round-trip verified |
| Use continue (not return nil) in generateModuleDocs sentinel | For-loop must process all modules, not exit early on first sentinel | ✓ Good — all 21 module pages processed correctly |
| Canonical YAML key is requirements_completed (underscore) | Consistent with Go/YAML conventions, matches frontmatter parser | ✓ Good — all 18 SUMMARY files use underscore |
| Fix tooling before any content work | check-translations path bug would invalidate locale verification | ✓ Good — enabled TDD for all subsequent phases |

---
*Last updated: 2026-02-28 after v1.0 milestone*
