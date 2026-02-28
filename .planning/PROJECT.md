# Alita Robot: Documentation & Command Consistency Audit

## What This Is

A systematic audit and remediation of Alita Robot's user-facing documentation and bot commands. The bot has 120+ commands across 21 modules, an Astro/Starlight documentation site, inline help system, and README — all of which have drifted from the actual implementation. This project brings everything into sync, fixes command UX issues, and ensures documentation accurately reflects what the bot does.

## Core Value

Every user-facing surface (docs site, help text, README, bot responses) accurately describes what the bot actually does, and every command behaves consistently and predictably.

## Requirements

### Validated

<!-- Existing capabilities inferred from codebase -->

- ✓ 120+ bot commands across 21 modules — existing
- ✓ Astro/Starlight documentation site with per-module command docs — existing
- ✓ Inline help system via /help with keyboard navigation — existing
- ✓ Multi-language support (en, es, fr, hi) with YAML locale files — existing
- ✓ README with project overview, installation, and commands — existing
- ✓ CLAUDE.md with architecture documentation — existing
- ✓ API reference docs (commands, callbacks, database, environment, permissions) — existing
- ✓ Self-hosting guides (database, monitoring, profiling, troubleshooting, webhooks) — existing
- ✓ Makefile with 17 development/deployment targets — existing
- ✓ GoDoc-compatible function documentation — existing

### Active

- [ ] Docs site command reference matches actual registered commands 1:1
- [ ] All command aliases documented (e.g., `extra`/`extras` for misc)
- [ ] Disableable vs non-disableable commands accurately marked with explanation of criteria
- [ ] Callback handlers documented and mapped to triggering commands
- [ ] Message watcher/background handler behavior documented
- [ ] README bot commands section expanded or linked to full reference
- [ ] Help text in bot matches docs site descriptions
- [ ] All locale files have consistent keys (no missing translations)
- [ ] Command permission requirements clearly documented
- [ ] Anonymous admin flow documented for end users
- [ ] Handler group system explained in architecture docs
- [ ] Module load order rationale documented
- [ ] Each command's response format/behavior verified against docs
- [ ] UX issues in command responses identified and fixed (error messages, formatting, edge cases)
- [ ] Bot command descriptions (BotFather) match actual behavior

### Out of Scope

- Adding new bot features or commands — audit only
- Redesigning the documentation site theme/layout — content only
- Rewriting the inline help system architecture — document as-is
- Performance optimization of commands — UX and correctness only
- Database schema changes — no structural modifications
- New locale/language additions — fix existing 4 languages only

## Context

- Bot has grown organically with 21 modules, each adding commands independently
- Documentation site was built after many commands existed, leading to coverage gaps
- Callback handlers (30+) are largely undocumented for end users
- Message watchers (filters, blacklists, antiflood, locks) run silently — users don't understand precedence
- README lists ~20 commands at high level while bot actually has 120+
- Some commands have undocumented aliases (e.g., `/addfilter` = `/filter`, `/addnote` = `/save`)
- Help system has `helpableKb` and `AltHelpOptions` patterns that are powerful but undocumented
- Anonymous admin verification via inline keyboard is confusing to new users
- Translation keys may be missing across es/fr/hi locales
- Docs site deployed to Cloudflare Workers, bot commands docs are per-module in `docs/src/content/docs/commands/`

## Constraints

- **Tech stack**: Go 1.25+ bot, Astro/Starlight docs — no framework changes
- **Locale format**: YAML with named parameters mapped to positional formatters
- **Callback data**: 64-byte Telegram limit — cannot add verbose descriptions
- **Command limit**: BotFather limits command descriptions to 256 chars
- **Backward compatibility**: No breaking changes to existing command syntax

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Audit docs against code, not code against docs | Code is the source of truth; docs must reflect reality | — Pending |
| Fix locale consistency across all 4 languages | Missing keys cause runtime fallback to English silently | — Pending |
| Document handler groups and message watchers | Users confused by "invisible" actions (filter matches, flood limits) | — Pending |

---
*Last updated: 2026-02-27 after initialization*
