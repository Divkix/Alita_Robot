# Feature Research

**Domain:** Telegram group management bot — documentation and command UX audit
**Researched:** 2026-02-27
**Confidence:** HIGH (based on direct codebase analysis + medium-confidence web research)

---

## Context: What We're Actually Auditing

This is not a greenfield project. The audit covers an existing system with:
- 134 registered command handlers across 21 modules (not 120 as claimed in docs — that number is stale)
- 21 callback handlers registered via `callbackquery.Prefix`
- 4 locale files (en/es/fr/hi) with a measurable hi locale gap (~18% missing vs en)
- An Astro/Starlight docs site with per-module command pages
- An inline `/help` system with `helpableKb` and `AltHelpOptions` patterns

Key finding: the `api-reference/commands.md` already documents most commands well. The gaps are specific and discoverable.

---

## Feature Landscape

### Table Stakes (Users Suffer Without These)

These are the audit checks that, if skipped, leave users confused or misled.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Command count accuracy in docs | Docs claim "120 commands / 21 modules" — actual count from grep is 134 handlers. A 12% undercount makes the reference untrustworthy | LOW | One-pass grep to count exactly; update header in `commands.md` |
| Missing commands in API reference | `start`, `help`, `donate`, `about`, `markdownhelp`, `formatting`, `stats`, `addsudo`, `adddev`, `remsudo`, `remdev`, `teamusers`, `chatinfo`, `chatlist`, `leavechat`, `remallbl`, `rmallbl`, `privnote`, `privatenotes` are registered in code but absent from `api-reference/commands.md` | LOW | Grep confirms these are absent; they need rows added to the reference |
| Alias documentation completeness | `remallbl`/`rmallbl` (blacklists), `privnote`/`privatenotes` (notes), `formatting`/`markdownhelp`, `rulesbtn`/`rulesbutton` / `clearrulesbtn`/`clearrulesbutton`/`resetrulesbtn`/`resetrulesbutton` — alias pairs are inconsistently marked as "Alias for X" vs listed as independent commands | LOW | `notes/index.mdx` gets this right; other modules don't |
| Hi locale gap remediation | `hi.yml` is 1804 lines vs `en.yml` 2207 lines — approximately 403 missing lines (~18%). Silent fallback to English means Hindi users see mixed-language UX | HIGH | Translation work, not code. Requires bilingual review. This is the hardest table-stakes item |
| Fr locale gap remediation | `fr.yml` is 2099 lines vs 2207 — approximately 108 missing lines (~5%) | MEDIUM | Smaller gap, same process |
| Disableable command accuracy | The `api-reference/commands.md` marks `saved`, `privatenotes`, `privnote` as `❌` (not disableable) but `saved` is not registered with `AddCmdToDisableable`. The cross-reference between `AddCmdToDisableable` calls in code vs docs is unverified | MEDIUM | Requires grepping all `AddCmdToDisableable` calls and comparing to docs |
| Dev commands documented or explicitly excluded | `stats`, `addsudo`, `adddev`, `remsudo`, `remdev`, `teamusers`, `chatinfo`, `chatlist`, `leavechat` exist but no docs mention them — this leaves operators confused about what commands exist for bot owners | LOW | Either document them in a "Dev/Owner Commands" section or explicitly note they're owner-only and unlisted |
| BotFather command list vs registered commands | BotFather's `/setcommands` list must match registered commands. Keep descriptions under 22 chars for mobile. Lead with verbs | LOW | Verify `setMyCommands` call exists in codebase and its content matches docs |
| Callback handler documentation completeness | `callbacks.md` lists 21 callbacks; the current count from code matches. But format documentation is wrong: it documents old dot-notation format, not the versioned codec format (`namespace|v1|url-encoded-fields`) from `alita/utils/callbackcodec/` | MEDIUM | Update callback format section in `callbacks.md` to reflect actual versioned codec |

### Differentiators (Would Make Docs Exceptional)

These aren't expected but would meaningfully raise the quality bar.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Message watcher / background handler precedence map | Locks, antiflood, antispam, blacklists, and filters all run as message watchers. Users don't understand why their message was deleted and by which handler. A single page documenting handler groups (-2 antispam, -1 anonymous admin, 0 commands, 4-10 watchers) and their firing order gives operators the mental model they need | MEDIUM | Architecture docs exist but don't explain user-visible behavior from watcher ordering |
| Permission requirement matrix by command | A single table: command → required bot permissions → required user permissions. Currently scattered across per-module docs. A consolidated matrix in `api-reference/permissions.md` would be the definitive reference | MEDIUM | `permissions.md` exists but covers general concepts; a command-level matrix doesn't exist |
| Connection module scope clarity | The connections module lets admins manage notes/filters/rules from PM. This is powerful and undocumented in terms of *which operations are connection-scoped*. Documenting the exact scope of connected operations prevents confusing support questions | LOW | Single section addition to `connections/index.mdx` |
| Anonymous admin verification flow diagram | `admin/index.mdx` describes the 5-step flow in text. A sequence diagram (Mermaid, which Starlight supports) would make it immediately understandable | LOW | Docs infrastructure supports Mermaid; content work only |
| Filter precedence vs blacklist precedence | Both can match the same trigger word. Which fires first? Which action takes precedence? This is undocumented and causes real confusion in groups using both | LOW | Investigate handler group ordering in code; document the result |
| `#notename` shorthand behavior in locked chats | Notes can be triggered by `#name`. If `messages` lock is active, does the trigger still fire? Document the interaction | LOW | Test case needed; document result |
| Inline help system architecture explanation | `helpableKb` and `AltHelpOptions` in `HelpModule` are used by 5 modules (notes, formatting, language, greetings, filters) to inject module-specific inline keyboards into help responses. This is undocumented for contributors. Documenting it prevents copy-paste bugs when adding new modules | LOW | CLAUDE.md mentions it; the docs site doesn't |
| Per-command error message audit | 134 commands each have error paths. Many use `tr.GetString()` for localized errors; some use hardcoded English strings. Identifying which commands have hardcoded English errors in their error paths is high-value for i18n completeness | HIGH | Requires grepping all error paths; complex but catches real bugs |
| `setMyCommands` scope documentation | Telegram Bot API 6.9+ supports `setMyCommands` with scope (private, group, group_admin, all_chat_admins). If the bot sets different command lists by scope, this is invisible to users and operators. Document what scopes are registered | LOW | Verify in codebase and document |

### Anti-Features (Things to Explicitly NOT Do in This Audit)

| Anti-Feature | Why Requested | Why Problematic | Alternative |
|--------------|---------------|-----------------|-------------|
| Rewriting the inline help system | The current text-based `helpableKb` system is functional but uses Markdown-converted-to-HTML. It's tempting to "fix" the architecture during an audit | Audit scope says "document as-is"; architectural changes break backward compatibility and introduce regression risk outside the audit's purpose | Document the existing system accurately; flag for future improvement |
| Adding new locale languages | Hi locale gap is painful, but someone might suggest adding pt-BR or de during the audit | New locales are green-field translations, not audit work. The audit fixes existing locales only | Stick to en/es/fr/hi; note other language contributions welcome |
| Changing command syntax for consistency | Some commands are `setflood`/`delflood` while others are `setwarnlimit`/`resetwarn`. Normalizing naming is appealing | Breaking changes. Existing users have these commands memorized or in scripts. Out of scope per PROJECT.md | Document existing names accurately; note inconsistencies as future improvement opportunities |
| Generating docs from code comments | It's tempting to add GoDoc comments to all handlers and auto-generate docs | The docs site is human-authored Astro/Starlight MDX, not generated from code. Mixing generated and human content creates maintenance complexity. The audit is about correctness of existing docs, not pipeline redesign | Audit existing docs against code manually |
| Auditing performance of commands | Purge command has concurrent deletion logic; antiflood has rate limiting. It's tempting to audit these during a UX audit | Out of scope per PROJECT.md; performance changes risk regression | Note performance concerns separately; don't conflate with UX audit |
| Per-user behavioral differences documentation | The bot behaves differently for bot owner, sudo users, dev users, regular admins, regular users. Fully documenting this matrix is endless | The permission system is already documented in `api-reference/permissions.md`. The dev/sudo tier is intentionally undocumented publicly | Document the 3-tier public model (owner, admin, user); keep dev-tier implicit |

---

## Feature Dependencies

```
[i18n audit: identify missing keys]
    └──requires──> [hi locale gap fix] (can't fix what you don't enumerate)
    └──requires──> [fr locale gap fix]

[command count accuracy]
    └──requires──> [authoritative grep of all handlers] (establish ground truth first)
                       └──enables──> [missing commands in API reference]
                       └──enables──> [alias documentation completeness]
                       └──enables──> [disableable command accuracy]

[callback documentation update]
    └──requires──> [understanding versioned codec format] (not just legacy dot-notation)

[permission matrix] ──enhances──> [per-module command docs]

[message watcher precedence] ──enhances──> [antiflood docs] ──enhances──> [blacklists docs]

[filter vs blacklist precedence] ──depends on──> [message watcher precedence]
```

### Dependency Notes

- **Command count requires authoritative grep first:** The foundation of the audit is establishing ground truth. Do this first via a single `grep -rn 'handlers.NewCommand\|MultiCommand'` across all module files. Everything else depends on this count.
- **i18n audit requires key enumeration:** Before fixing hi/fr locales, enumerate all keys in `en.yml` that are absent from target locales. The project has a `make check-translations` target that runs `scripts/check_translations` — use this output as the canonical gap list.
- **Callback format depends on codec understanding:** The `callbacks.md` documents old dot-notation. The versioned codec in `alita/utils/callbackcodec/` uses `namespace|v1|url-encoded-fields`. Updating callback docs requires understanding the actual codec format first.
- **Permission matrix enhances but doesn't block:** Command docs can be accurate without a consolidated permission matrix. Build the matrix after individual module docs are verified.

---

## MVP Definition

This is an audit, not a new product. "MVP" here means the minimum audit scope that eliminates user-visible confusion.

### Launch With (v1) — Eliminate Active User Confusion

- [ ] Command count accuracy — fix the stale "120 commands" claim with the actual number
- [ ] Missing commands in API reference — add `start`, `help`, `donate`, `about`, `formatting`/`markdownhelp`, dev commands with "owner-only" note, `remallbl`/`rmallbl`, `privnote`/`privatenotes`
- [ ] Disableable command accuracy — verify `AddCmdToDisableable` calls against docs column by column
- [ ] Alias documentation — ensure every command that's an alias of another says so explicitly
- [ ] Callback codec format documentation — update `callbacks.md` to reflect versioned codec format

### Add After Validation (v1.x) — Reduce Operator Confusion

- [ ] Hi locale gap — enumerate and translate missing keys (requires bilingual contribution or translation service)
- [ ] Fr locale gap — smaller gap, same process
- [ ] Message watcher precedence page — explain which handler fires first and why
- [ ] Anonymous admin flow diagram — Mermaid sequence in `admin/index.mdx`

### Future Consideration (v2+) — Nice to Have

- [ ] Per-command error message i18n audit — high-effort, low-frequency impact
- [ ] Consolidated permission matrix — useful reference but not blocking
- [ ] Filter vs blacklist interaction docs — needs testing to confirm actual behavior
- [ ] `setMyCommands` scope audit — requires checking live bot configuration

---

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| Command count accuracy | HIGH | LOW | P1 |
| Missing commands in API reference | HIGH | LOW | P1 |
| Disableable column accuracy | HIGH | LOW | P1 |
| Alias documentation completeness | MEDIUM | LOW | P1 |
| Callback codec format update | MEDIUM | LOW | P1 |
| Hi locale gap fix | HIGH | HIGH | P1 (but blocked on translation) |
| Fr locale gap fix | MEDIUM | MEDIUM | P2 |
| Message watcher precedence docs | HIGH | LOW | P2 |
| Anonymous admin flow diagram | MEDIUM | LOW | P2 |
| Dev commands documentation | MEDIUM | LOW | P2 |
| Consolidated permission matrix | MEDIUM | MEDIUM | P3 |
| Per-command error message audit | LOW | HIGH | P3 |
| Filter vs blacklist interaction | MEDIUM | LOW | P3 |

**Priority key:**
- P1: Directly addresses the audit's core value (accuracy of user-facing surfaces)
- P2: Reduces operator/admin confusion significantly
- P3: Quality improvement, defer if time-constrained

---

## Competitor Feature Analysis

Context: Comparable Telegram group management bots (Rose, Combot, Group Help) for benchmarking what documentation quality looks like in this space.

| Feature | Rose Bot | Combot | Alita (current) |
|---------|----------|--------|-----------------|
| Command reference accuracy | Docs match code (commands list updated per release) | Web dashboard is source of truth | API reference partially stale (missing ~14 commands) |
| Alias documentation | Commands listed with aliases inline | N/A (web dashboard UI, no CLI aliases) | Inconsistent — some modules list aliases, others don't |
| i18n coverage | Multi-language with community translations tracked publicly | English-primary | 4 languages; hi has ~18% gap; no public gap tracking |
| Callback documentation | Not publicly documented | N/A | `callbacks.md` exists but uses wrong format description |
| Message watcher precedence | Not documented | Documented via "filter priority" setting in dashboard | Not documented |
| Inline help | Module-level `/help ModuleName` with descriptions | Web dashboard | `/help` with `helpableKb` keyboard navigation; descriptions accurate |

**Takeaway:** Alita's inline help system and per-module docs site are more sophisticated than most competitors. The gap is accuracy and completeness, not structure. Fix what's wrong, don't redesign what's working.

---

## Sources

- Direct codebase analysis: `alita/modules/*.go` handler registrations (HIGH confidence)
- Direct docs analysis: `docs/src/content/docs/` MDX/MD files (HIGH confidence)
- Locale line count comparison: `locales/*.yml` (HIGH confidence — line count is a proxy; exact key diff requires `make check-translations`)
- [Telegram Bot Commands official docs](https://core.telegram.org/api/bots/commands) — command character limits, setMyCommands API (HIGH confidence)
- [BotFather command description best practices via WebSearch](https://telegramhpc.com/news/63/) — 22 char mobile truncation limit, verb-first descriptions (MEDIUM confidence)
- [grammY i18n plugin fallback pattern](https://grammy.dev/plugins/i18n) — multi-step locale fallback strategy (MEDIUM confidence, different ecosystem but same Telegram API)
- [Telegram Bot UX best practices via WebSearch](https://medium.com/@bsideeffect/10-best-ux-practices-for-telegram-bots-79ffed24b6de) — error message quality, command discoverability (LOW confidence — single source)

---

*Feature research for: Telegram bot documentation and command UX audit*
*Researched: 2026-02-27*
