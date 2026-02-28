# Pitfalls Research

**Domain:** Documentation audit and command consistency for a Go Telegram bot (120+ commands, 21 modules)
**Researched:** 2026-02-27
**Confidence:** HIGH (based on direct codebase analysis + external research)

---

## Critical Pitfalls

### Pitfall 1: Command Count Drift in Static Documentation

**What goes wrong:**
The docs index (`docs/src/content/docs/commands/index.mdx`) hardcodes command counts in Badge components (e.g., `<Badge text="8 commands" />`). These numbers are already wrong. Direct codebase analysis shows: blacklists docs claim 8 commands, actual registered handlers are 6 primary + 2 aliases. Notes docs claim 10, actual is 8. Rules docs claim 11, actual is 9. Every time a command is added, renamed, or aliased without updating the docs, this count diverges further.

**Why it happens:**
Counts were set manually at doc creation time. Go handler registration (`dispatcher.AddHandler`) happens in code; the docs site has no automated linkage to it. Aliases registered via `cmdDecorator.MultiCommand` are invisible to anyone auditing the docs file manually.

**How to avoid:**
Establish a canonical definition of "what counts as a command" before auditing — primary commands only, or primaries + aliases. Write a script that tallies `dispatcher.AddHandler(handlers.NewCommand(...)` calls and `cmdDecorator.MultiCommand(...)` calls per module file, then diff against what docs claim. Run this before and after every doc edit during the audit. Do not fix the Badge counts by eyeballing — use the script output.

**Warning signs:**
- Docs say a module has X commands, code has X ± 2 or more
- A module file is named `mute.go` but the docs directory is `mutes/` — naming drift signals the docs were written independently of the code
- Badge text is a round number (5, 10) — round numbers in counts are almost always estimates

**Phase to address:**
Discovery phase, before writing a single line of documentation. Establish the script first, run it, and use its output as ground truth for all subsequent work.

---

### Pitfall 2: Auditing Aliases as Primary Commands (or Missing Them Entirely)

**What goes wrong:**
This codebase has two alias patterns that are easy to miss:
1. `dispatcher.AddHandler(handlers.NewCommand("addblacklist", ...))` AND `dispatcher.AddHandler(handlers.NewCommand("blacklist", ...))` — registered as separate handlers pointing to the same function
2. `cmdDecorator.MultiCommand(dispatcher, []string{"remallbl", "rmallbl"}, ...)` — registered through a decorator

Both patterns produce working commands. If the audit only scans for `NewCommand` calls, the MultiCommand aliases are missed. If the audit treats every `NewCommand` call as a distinct command, aliases are double-counted. The PROJECT.md itself lists `/addfilter` = `/filter` and `/addnote` = `/save` as examples of undocumented aliases — these exist beyond what the codebase check surfaces.

**Why it happens:**
The module authors chose different alias patterns for different modules. There is no single canonical way to enumerate all valid command strings from the codebase.

**How to avoid:**
The audit script must cover both registration patterns. Extract: (1) all strings from `handlers.NewCommand("STRING", ...)` and (2) all string slices from `cmdDecorator.MultiCommand(dispatcher, []string{...}, ...)`. Group by handler function to identify which commands are aliases of each other. The grouping is what determines which should be marked as primary and which as aliases in docs.

**Warning signs:**
- Two `NewCommand` registrations in the same module point to the same handler function — those are aliases
- `cmdDecorator.MultiCommand` calls are present in any module — those commands will not appear in a naive grep for `NewCommand`

**Phase to address:**
Discovery phase. Build the alias map before writing documentation for any module.

---

### Pitfall 3: Silent Locale Fallback Masking Missing Translations

**What goes wrong:**
The `check-translations` Makefile target reports "0 translation keys in codebase" and "All translations present" — this is a false negative caused by the script failing to resolve relative paths (`../../alita/...`). The script works from `scripts/check_translations/` and outputs warnings like "potentially unsafe file path" for every Go source file, meaning it scans nothing. The Spanish locale (`es.yml`) has 7 extra keys vs English (`en.yml`) — `devs_chat_list_caption`, `devs_getting_chat_list`, `devs_no_team_users`, `devs_no_users`, `misc_translate_need_text`, `misc_translate_no_text`, `misc_translate_provide_text` — keys that exist in `es.yml` but not in `en.yml`. These are orphan keys that will never be rendered. Hindi (`hi.yml`) has 403 fewer lines than English, which warrants deeper inspection beyond top-level key counts.

**Why it happens:**
The translation checker runs from a subdirectory and uses relative paths that violate its own safety checks. The false "all clear" gives false confidence. Locale files drift when features are added or removed — keys are added to the reference locale and not propagated, or removed from the reference but left in others.

**How to avoid:**
Fix the check-translations script path issue before relying on it for anything. Supplement with a Python/Go script that loads all four YAML files, recursively flattens all keys (including nested), and reports (a) keys in `en.yml` missing from other locales, (b) keys in other locales missing from `en.yml`, and (c) keys where the value is identical to English (likely untranslated). Verify `hi.yml`'s 403-line deficit is accounted for by structure differences, not missing content.

**Warning signs:**
- `make check-translations` says "Found 0 translation keys in codebase" — that is always wrong if the bot has locale string lookups
- `es.yml` has more keys than `en.yml` — reverse drift, indicates stale keys from removed features
- One locale file is significantly shorter than others in line count (hi.yml is 1804 lines vs en.yml's 2207)

**Phase to address:**
Before any i18n work. Fix the tooling first, then audit. Do not add new locale keys until the gap baseline is established.

---

### Pitfall 4: Treating "Docs Directory Exists" as "Module is Documented"

**What goes wrong:**
The audit scope includes 21 modules. Four Go module files (`devs.go`, `help.go`, `language.go`, `users.go`) have no corresponding docs directory under `docs/src/content/docs/commands/`. The docs index lists 21 modules, but some entries link to directories that document modules with different names than their Go file (e.g., `mute.go` → `mutes/` directory). Simply checking that a directory exists does not verify that the content inside covers all commands registered by that module.

**Why it happens:**
The docs site uses auto-generated sidebar (`autogenerate: { directory: 'commands' }` in `astro.config.mjs`), which means any directory with an index file appears. Modules that grew after the initial doc site creation have no docs. The naming mismatch (singular vs plural: `mute` vs `mutes`, `language` vs `languages`) shows the docs were written with different naming conventions than the code.

**How to avoid:**
For each module file in `alita/modules/`, verify a docs directory exists AND that the docs directory covers the commands registered by that module. Create a mapping table: module file → docs path → commands documented. Flag any module where this mapping is incomplete or missing. The audit checklist must include `devs.go`, `help.go`, `language.go`, and `users.go` as explicit targets for new documentation.

**Warning signs:**
- Module Go file name does not match docs directory name (singular vs plural, abbreviation vs full name)
- The autogenerate sidebar will show entries in alphabetical order but won't signal missing modules
- "21 modules" claim in the docs index should be verified by counting actual directory entries

**Phase to address:**
Discovery phase. Establish the full module → docs mapping before any content work begins.

---

### Pitfall 5: Conflating Inline Bot Help Text with Docs Site Content

**What goes wrong:**
The bot has two distinct help surfaces: (1) the inline `/help` system using locale YAML strings rendered in Telegram, and (2) the Astro/Starlight docs site. These have different formatting requirements, length limits, and update paths. Locale strings use Markdown but the bot sends HTML (requiring `tgmd2html.MD2HTMLV2()` conversion per the CLAUDE.md rules). The docs site uses MDX with Starlight components. An audit that tries to keep these "in sync" by copy-pasting between them will create formatting bugs — escape sequences that work in one context break in the other.

**Why it happens:**
Both surfaces describe the same commands, so the natural instinct is to keep them identical. But they are not the same format. Telegram HTML has a restricted tag set. MDX supports code blocks, tabs, asides, badges. Trying to write one source and derive the other mechanically does not work without a transformation layer.

**How to avoid:**
Treat inline help text and docs site content as separate artifacts that should describe the same facts but in different formats appropriate to their medium. The audit should verify factual consistency (same commands listed, same permission requirements, same behavior described) but not textual identity. Update each surface in its own format. Do not copy-paste raw locale YAML into MDX or vice versa.

**Warning signs:**
- A locale YAML string contains HTML tags (these render as literal text in the bot's HTML parse mode if not converted)
- A docs page contains escape sequences like `\n` literally (YAML single-quote preservation issue per CLAUDE.md)
- The same text appears verbatim in both a locale file and a docs file — copy-paste from one to the other

**Phase to address:**
Content update phases. Establish format rules for each surface at the start of the content work.

---

### Pitfall 6: Incomplete Scope Definition Leading to Rework

**What goes wrong:**
The PROJECT.md scope excludes "adding new bot features or commands" and "redesigning the documentation site theme/layout." But the scope includes "UX issues in command responses identified and fixed (error messages, formatting, edge cases)" — which can easily expand into fixing bot behavior, not just documentation. The 38 callback handlers that are "largely undocumented" represent behavior that may need code changes to document accurately (e.g., documenting what a callback does may reveal it behaves incorrectly). If the scope boundary is not enforced, the audit becomes a feature project.

**Why it happens:**
Documentation audits reveal gaps. Gaps feel like they need to be filled. Fixing a wrong error message requires touching Go code, not docs. The boundary between "document behavior" and "fix behavior" is fuzzy during execution.

**How to avoid:**
Create a log of "behavior issues found" that is explicitly separate from "documentation issues fixed." Behavior issues get filed as future work. During the audit, only fix behavior if it is a trivial one-line change and fixing it is faster than documenting the wrong behavior. Everything else goes in the backlog. Review this log at the end of each phase to prevent scope creep accumulation.

**Warning signs:**
- A PR modifies a `.go` handler file and a docs `.mdx` file in the same commit for non-trivial Go changes
- The audit backlog has 0 items — it is not possible to audit 120+ commands without finding behavior issues
- Time estimate for a "documentation only" task exceeds 2 hours — that usually means behavior investigation crept in

**Phase to address:**
All phases. Enforce the boundary from phase 1 onward, not after scope has already expanded.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Manually hardcoding command counts in Badge components | Fast to write | Becomes wrong with every code change, requires manual updates | Never — automate or use prose descriptions |
| Copy-pasting command descriptions from locale YAML into docs MDX | Avoids writing twice | Format drift, HTML vs Markdown escaping bugs | Never for automation; OK for initial draft with explicit re-formatting |
| Skipping docs for "internal" modules (devs, help) | Faster audit | Users who find the commands via autocomplete have no reference | Only if the module is genuinely admin-only and not surfaced to users |
| Treating `make check-translations` output as authoritative | Fast CI check | Script has a known bug (path resolution failure) — gives false confidence | Never until the script is fixed |
| Documenting aliases as separate commands with separate descriptions | Easier to write | Users see duplicate entries; search returns multiple results | Never — use a canonical primary with an "also known as" note |

---

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| BotFather command list | Setting command descriptions that exceed 256 characters | Keep descriptions under 32 characters for the menu display (BotFather shows truncated text); put full description in docs |
| Telegram callback data | Documenting raw callback data strings for end users | Document the triggering command and the expected UI flow; the 64-byte callback data limit is an internal constraint not relevant to users |
| Astro/Starlight autogenerate sidebar | Assuming all modules appear because sidebar uses `autogenerate` | Autogenerate only picks up directories with valid index files — modules without docs directories are silently absent from the nav |
| YAML locale format | Using single quotes for strings that contain `\n` — they are preserved literally | Use double quotes for any locale string with escape sequences; verify rendered output in bot |
| Go i18n named parameters | Assuming `{"user": value}` maps to `%s` positionally in the right order | The mapping is positional and order-dependent — test every locale string that has named parameters with actual renders |

---

## Performance Traps

These are not performance concerns for this audit project. The audit is a documentation task, not a runtime task. The one operational concern relevant to this project:

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Astro build time increases with large MDX files | `make docs-dev` slow to start | Keep individual docs pages focused and small; don't aggregate all module docs into one file | Not a real concern at this scale |

---

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Documenting bot token or webhook secret configuration in public docs | Credential exposure if users copy example configs | Use placeholder values (`YOUR_BOT_TOKEN`) in all docs and sample.env; never document real values |
| Documenting that `ENABLE_PPROF=true` is a valid production setting | Attackers gain access to memory profiling endpoints | The docs must explicitly warn that pprof is dangerous in production; the existing CLAUDE.md says "dangerous in production" — replicate this warning in user-facing docs |

---

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Documenting both `/addfilter` and `/filter` as separate commands without explaining they are aliases | Users think these are different features; confusion when behavior is identical | Document primary command clearly, note aliases in a dedicated "Aliases" field; do not create separate description blocks |
| Not documenting the anonymous admin flow | Users with anonymous admin roles get a confusing inline keyboard they do not understand | Every command that requires user admin status must explain that anonymous admins need to use the verification keyboard |
| Documenting message watcher behavior (filters, blacklists, antiflood, locks) without explaining precedence | Users cannot predict which watcher fires first when a message matches multiple rules | Document handler group order explicitly; negative groups fire before group 0, which fires before positive groups |
| Omitting the "reply to a user" vs "provide username" distinction in command docs | Users try `/ban` with no argument and get an error they cannot interpret | Every moderation command must document all valid invocation forms: reply, username, ID |
| Help text that lists only the primary command but the bot autocomplete shows all aliases | Users see more options in the autocomplete than in the help text — confusing | Either list all aliases in help text or suppress aliases from BotFather command list |

---

## "Looks Done But Isn't" Checklist

- [ ] **Module docs page created:** Verify it is actually linked from the commands index — autogenerate sidebar will show it, but the `index.mdx` CardGrid must have an explicit entry or users won't discover it from the overview
- [ ] **Command count badge updated:** Verify the count in `index.mdx` matches the script output, not what you remember writing
- [ ] **Locale keys added to all 4 locales:** Adding a key to `en.yml` only causes silent English fallback in es/fr/hi — `make check-translations` will not catch this until the script is fixed
- [ ] **Command aliases documented:** Verify that `cmdDecorator.MultiCommand` aliases appear in the docs, not just `NewCommand` primaries
- [ ] **Permission level verified in code:** Docs claim a permission level (Admin Only, Everyone) — verify by checking the actual `RequireUserAdmin()` / `RequireBotAdmin()` calls in the handler, not by inference from what the command seems to do
- [ ] **Callback handlers mapped:** A command that produces an inline keyboard response has callback handlers that do work — if only the command is documented but not the keyboard behavior, the docs are incomplete
- [ ] **BotFather command list updated:** Docs site update does not automatically update the `/setcommands` list in Telegram — these are separate artifacts that must both be updated

---

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Hardcoded command counts are wrong in index.mdx | LOW | Run count script, update Badge text values — mechanical find-and-replace |
| `make check-translations` script broken | MEDIUM | Fix path resolution in `scripts/check_translations/main.go` to use absolute paths or accept a base path argument; re-run to get actual missing key report |
| Locale file has orphan keys (es.yml extra 7 keys) | LOW | Remove keys from es.yml that do not exist in en.yml; verify no Go code references them first |
| Module has no docs directory (devs, help, language, users) | MEDIUM | Create the directory, write an index.md following the existing module doc pattern, add a CardGrid entry to index.mdx |
| Alias commands not documented | LOW | Add an "Aliases" section or table to the module docs page; update BotFather list if aliases are user-visible |
| Inline help text and docs site content have diverged | MEDIUM | Establish the factual baseline from code, update each surface separately in its own format — do not try to derive one from the other |

---

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Command count drift in static docs | Discovery (first) | Script output matches all Badge text values in index.mdx |
| Alias commands missed or double-counted | Discovery (first) | Complete alias map generated and reviewed before any doc writing |
| Silent locale fallback / broken check script | Discovery (first) | `make check-translations` runs without path errors and reports real findings |
| Docs directory missing for modules | Discovery (first) | Module-to-docs mapping table is complete with no gaps |
| Conflating inline help with docs site format | Content update phases | No raw YAML locale strings appear verbatim in MDX files |
| Scope creep into behavior fixes | All phases | Behavior issue backlog is non-empty; no Go handler files modified without explicit scope approval |
| BotFather list not updated after docs | Final verification phase | `/setcommands` output verified to match command list in docs after all changes |

---

## Sources

- Direct codebase analysis: `/Users/divkix/GitHub/Alita_Robot/alita/modules/` — handler registrations, alias patterns
- Direct codebase analysis: `/Users/divkix/GitHub/Alita_Robot/locales/` — YAML key comparison across en/es/fr/hi
- Direct codebase analysis: `/Users/divkix/GitHub/Alita_Robot/docs/src/content/docs/commands/` — module coverage gaps
- Direct codebase analysis: `scripts/check_translations/main.go` — identifies path resolution bug
- [Taming Version Drift: Keeping Product Docs Aligned](https://www.cinfinitysolutions.com/limitless-blog/version-drift-doc-chaos) — documentation drift root causes
- [Localization Key Not Found: Complete Guide](https://copyprogramming.com/howto/easy-localization-localization-key-not-found) — silent fallback behavior
- [Telegram Bot Commands API](https://core.telegram.org/api/bots/commands) — BotFather limits and setMyCommands behavior
- [GitHub - nicksnyder/go-i18n](https://github.com/nicksnyder/go-i18n) — Go i18n patterns and pitfalls
- [10 Technical Documentation Best Practices 2025](https://deepdocs.dev/technical-documentation-best-practices/) — documentation audit patterns
- [Writing AI coding agent context files](https://packmind.com/evaluate-context-ai-coding-agent/) — documentation drift in live codebases

---
*Pitfalls research for: Alita Robot documentation audit and command consistency*
*Researched: 2026-02-27*
