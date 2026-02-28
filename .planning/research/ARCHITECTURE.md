# Architecture Research

**Domain:** Documentation and command consistency audit for a multi-module Go Telegram bot
**Researched:** 2026-02-27
**Confidence:** HIGH — derived entirely from live codebase inspection, no external sources required

## Standard Architecture

### System Overview

The audit problem maps onto three independent surfaces that need to be brought into sync:

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        SOURCE OF TRUTH                                   │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │  alita/modules/*.go  — 22 module files, 138 registered handlers  │   │
│  │  (134 command handlers + 23 callbacks + 12 message watchers)     │   │
│  └──────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────┘
           │                         │                          │
           ▼                         ▼                          ▼
┌─────────────────┐       ┌─────────────────┐       ┌─────────────────────┐
│  DOCS SITE      │       │  INLINE HELP    │       │  LOCALE FILES       │
│  docs/src/      │       │  locales/*.yml  │       │  en/es/fr/hi.yml    │
│  content/docs/  │       │  (help strings  │       │  835 keys each,     │
│  commands/      │       │  in bot)        │       │  4 language files   │
│  21 dirs +      │       │                 │       │                     │
│  index.mdx      │       │                 │       │                     │
└─────────────────┘       └─────────────────┘       └─────────────────────┘
           │                         │                          │
           └─────────────────────────┴──────────────────────────┘
                                     │
                                     ▼
                         ┌─────────────────────┐
                         │  README.md          │
                         │  (high-level,       │
                         │  ~35 commands,      │
                         │  inaccurate struct) │
                         └─────────────────────┘
```

### Audit Work Component Boundaries

Each component can be audited largely independently. Dependencies exist only in the "fix" direction (findings from the code audit gate fixes to docs/locales), not between docs/locale audits themselves.

| Component | What It Contains | Audit Independence | Fix Dependency |
|-----------|-----------------|-------------------|----------------|
| **Command Inventory** | 134 commands + aliases across 22 modules | Fully independent | None — this IS the source of truth |
| **Docs Site Audit** | 21 module docs, command tables, permission rows | Independent | Depends on Command Inventory |
| **i18n Locale Audit** | 835 keys × 4 languages, key naming consistency | Independent | Can proceed in parallel with Docs Audit |
| **README Audit** | ~35 listed commands, wrong directory structure | Independent | Depends on Command Inventory |
| **Callback Documentation** | 23 callback handlers, currently undocumented | Independent | Depends on Command Inventory |
| **Message Watcher Documentation** | 12 background handlers (filters, locks, antiflood, antispam) | Independent | Depends on Command Inventory |

## Recommended Project Structure

The audit work maps to the existing codebase structure — no new directories needed:

```
Alita_Robot/
├── alita/modules/          # Source of truth for all audits
│   ├── *.go                # 22 module files — command registry
│   └── helpers.go          # moduleStruct definition
├── locales/                # i18n audit target
│   ├── en.yml              # Reference locale (835 keys)
│   ├── es.yml              # 842 keys (7 EXTRA keys vs EN = bug)
│   ├── fr.yml              # 835 keys (parity with EN)
│   └── hi.yml              # 835 keys (parity with EN)
├── docs/src/content/docs/  # Docs site audit target
│   └── commands/           # 21 module dirs + index.mdx
│       ├── index.mdx       # Overview (badges show command counts)
│       └── */index.mdx     # Per-module command reference
└── README.md               # High-level audit target
```

### Structure Rationale

- **`alita/modules/*.go`**: Start every audit cycle here. Register the ground truth before touching any documentation surface.
- **`locales/en.yml`**: English is the canonical locale. All other locales must match its key structure exactly.
- **`docs/src/content/docs/commands/`**: Each module has exactly one `index.mdx`. One-to-one correspondence with module files in code (with two naming exceptions: `mutes/` vs `mute.go`, `languages/` vs `language.go`).

## Architectural Patterns

### Pattern 1: Command Inventory First

**What:** Before touching any documentation surface, extract the complete canonical command list from Go source via `dispatcher.AddHandler(handlers.NewCommand(...))` and `cmdDecorator.MultiCommand(...)` calls.

**When to use:** Start of every audit pass. Gate all doc/locale fixes on this inventory.

**Trade-offs:** Adds one explicit step before any writing begins, but prevents documenting phantom commands or missing real ones.

**Example inventory structure:**
```
Module: warns
  Commands: /warn /swarn /dwarn /resetwarns /resetwarn /rmwarn /unwarn /warns /setwarnlimit /setwarnmode /resetallwarns /warnings (12)
  Disableable: warns
  Callbacks: rmAllChatWarns, rmWarn (2)
  Message watchers: none
```

### Pattern 2: Per-Module Audit Unit

**What:** Treat each module as an atomic audit unit covering: (a) commands in code vs docs, (b) aliases in code vs docs, (c) permission requirements in code vs docs, (d) disableable status in code vs docs, (e) locale keys referenced vs locale keys present.

**When to use:** Each of the 21 documented modules (plus the 2 undocumented: `devs`, `help`).

**Trade-offs:** Slightly more setup per module, but isolates regressions and allows parallel work on different modules.

**Example:**
```
Module audit checklist (bans):
  [x] code commands: ban sban dban tban unban kick dkick kickme restrict unrestrict (10)
  [x] docs commands: ban sban dban tban unban kick dkick kickme restrict unrestrict (10) ✓
  [ ] code callbacks: banb unbanb — NOT documented
  [ ] code message watchers: none ✓
  [ ] disableable in code: none ✓ (docs say none) ✓
  [ ] locale keys for ban operations: check en/es/fr/hi ✓
```

### Pattern 3: Locale Key Diff as Separate Pass

**What:** Run a standalone locale key comparison pass using the EN file as reference, independent of the docs audit. Find keys that are: (a) in code but not in any locale, (b) in EN but not in other locales, (c) in other locales but not in EN (orphans).

**When to use:** After command inventory; can run fully in parallel with docs audit.

**Trade-offs:** Tools like `make check-translations` exist but are currently broken (path safety bug returning 0 found keys). Manual Python diff or fixed tooling needed.

**Current known findings:**
- ES has 5 extra keys (`devs_getting_chat_list`, `devs_chat_list_caption`, `devs_no_team_users`, `devs_no_users`, `misc_translate_need_text`) that are actively used in Go code but MISSING from EN, FR, and HI locales. This is an active bug where English users get empty strings for these bot responses.
- The Go code calls `devs_getting_chat_list` but EN only has `devs_getting_chatlist` (underscore difference). ES was updated; EN/FR/HI were not.

## Data Flow

### Audit Data Flow

```
Go source (alita/modules/*.go)
    │
    ▼ grep/parse
Command Inventory (per module: commands, aliases, callbacks, watchers, disableable)
    │
    ├──► Docs Site Comparison
    │        │
    │        ▼
    │    Findings: missing commands, wrong permissions, undocumented aliases,
    │              missing callback docs, wrong disableable flags
    │        │
    │        ▼
    │    Fix: update docs/src/content/docs/commands/*/index.mdx
    │
    ├──► Locale Key Comparison
    │        │
    │        ▼
    │    Findings: keys in code missing from locales, orphan keys in non-EN locales,
    │              key naming mismatches across locales
    │        │
    │        ▼
    │    Fix: update locales/en.yml locales/fr.yml locales/hi.yml
    │
    └──► README Comparison
             │
             ▼
         Findings: commands not listed, wrong directory structure,
                   inaccurate feature descriptions
             │
             ▼
         Fix: update README.md bot commands section + project structure diagram
```

### Fix Propagation Rules

1. Code is always right. Docs/locales/README adapt to code, never the reverse.
2. Locale fixes in non-EN files require parallel fix in EN if EN is the source of the naming bug (as with `devs_getting_chatlist` vs `devs_getting_chat_list`).
3. Docs fixes in `index.mdx` for a module require updating `docs/src/content/docs/commands/index.mdx` badge counts if the command count changes.
4. No code changes in this project. Audit and docs only.

### State Management for Audit Findings

Track findings in a structured format per module to prevent revisiting:

```
Module state: [not_started | inventoried | docs_audited | locale_audited | fixed | verified]
```

## Scaling Considerations

| Scale | Architecture Adjustment |
|-------|------------------------|
| 1 module at a time | No tooling needed — manual grep + read |
| 5-10 modules | Script to extract command inventories from all modules at once |
| All 22 modules | Automated diff script: code commands vs docs tables + locale key cross-reference |

### Scaling Priorities

1. **First bottleneck:** Extracting the command inventory manually is error-prone. A grep script extracting all `NewCommand(` and `MultiCommand(` calls into a structured list eliminates this before the audit begins.
2. **Second bottleneck:** Locale key comparison across 4 files × 835 keys. Fix the `make check-translations` tool or use the Python diff approach (confirmed working above).

## Anti-Patterns

### Anti-Pattern 1: Fixing Code to Match Docs

**What people do:** Find a docs description that doesn't match code behavior, then "fix" the code to match.

**Why it's wrong:** Code is the source of truth. Docs describe behavior, not mandate it. Changing code introduces regressions.

**Do this instead:** Update docs to accurately describe what code does.

### Anti-Pattern 2: Auditing Docs Without Module-Level Command Inventory

**What people do:** Read `index.mdx` files directly and edit them against intuition.

**Why it's wrong:** Without the extracted command inventory, you'll miss aliases registered via `MultiCommand`, miss commands registered in one place but documented in another, and miss the 23 callback handlers entirely.

**Do this instead:** Extract the command inventory for each module from Go source before opening any `.mdx` file.

### Anti-Pattern 3: Treating Locale Key Absence as "Not a Problem"

**What people do:** See a locale key missing from EN/FR/HI and assume it's a translation in progress.

**Why it's wrong:** Missing locale keys in the English file cause silent empty-string responses for ALL users, not just non-English users. The ES locale discovered that `devs_getting_chat_list` is used in code but absent from EN — English users see blank text.

**Do this instead:** EN locale key absence = bug. Fix EN first; other locales follow.

### Anti-Pattern 4: Updating Index Badge Counts Manually

**What people do:** Increment/decrement badge counts in `docs/src/content/docs/commands/index.mdx` by hand.

**Why it's wrong:** Manual counts drift. The index currently shows `<Badge text="8 commands" />` for Blacklists, which matches `6 NewCommand + 1 MultiCommand = 7 unique commands` in code (with `blacklist` being an alias for `addblacklist`). Manual counting will get this wrong.

**Do this instead:** Drive badge counts from the command inventory script output.

## Integration Points

### External Services

| Service | Integration Pattern | Notes |
|---------|---------------------|-------|
| Astro/Starlight docs site | MDX files in `docs/src/content/docs/commands/` | Built with `make docs-dev`; deployed to Cloudflare Workers |
| BotFather command descriptions | Set via Telegram API at bot startup | Limited to 256 chars per description; not currently audited in scope |
| Telegram inline help system | Locale strings rendered in bot responses | Tied to locale audit — fixing locale keys fixes inline help |

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| Command inventory → Docs audit | Manual transfer of structured list | Script automation recommended |
| Command inventory → Locale audit | `grep` for `GetString("key")` calls to find locale key usage | Bidirectional: code→locale and locale→code |
| Docs audit → Index badge update | Manual update to `index.mdx` badge text | Derive from inventory count |
| Module code → Help inline text | Module's `helpText` string in `Load*()` function registered with `HelpModule` | Separate surface from docs site — also needs audit |

## Suggested Audit Order

Dependencies between areas inform this order:

1. **Command Inventory (all 22 modules)** — No dependencies. Produces the master list that everything else references. Do this entirely before any editing.

2. **i18n Locale Audit** — No dependency on docs audit. Can proceed immediately after inventory in parallel. Fix EN first for any key naming bugs, then propagate to other locales. The `make check-translations` tool is currently broken; use manual Python key diff.

3. **Docs Site Audit (per module)** — Depends on Command Inventory. Can be done module-by-module in any order. Suggested order: modules with most commands first (warns=12, rules=11, notes=10+alias, greetings=10, bans=10) to maximize impact per unit of effort.

4. **Callback Documentation** — Can be done within each module's docs audit pass. The 23 callback handlers are currently undocumented across all 15 modules that have them.

5. **Message Watcher Documentation** — Clarify what triggers each watcher, in what handler group, and what action results. This is the "invisible behavior" that confuses users most. Document in each module's docs page.

6. **README Update** — Lowest priority. Last to fix. The README's project structure diagram references `cmd/` which does not exist. The command list covers ~35 of 134 commands. Update after all module docs are correct.

## Build Order Implications

- **No code compilation required** for any audit or doc fix. All surfaces are `.mdx`, `.yml`, or `.md` files.
- **Locale fixes** should be verified by running `make check-translations` after the tool is fixed, or via the Python diff script.
- **Docs fixes** can be previewed with `make docs-dev` (Astro dev server via bun).
- **Fixes are safe to ship incrementally** — each module's docs are independent MDX files.

## Sources

- Live codebase inspection: `alita/modules/*.go` — command/callback/watcher registrations
- `docs/src/content/docs/commands/` — existing docs structure and content
- `locales/*.yml` — locale key inventory (Python yaml diff, 2026-02-27)
- `.planning/codebase/ARCHITECTURE.md` — system architecture analysis
- `.planning/codebase/STRUCTURE.md` — directory layout
- `.planning/PROJECT.md` — audit scope and constraints

---
*Architecture research for: Alita Robot documentation and command consistency audit*
*Researched: 2026-02-27*
