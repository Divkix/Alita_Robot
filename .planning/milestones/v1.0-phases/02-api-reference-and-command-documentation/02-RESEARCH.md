# Phase 2: API Reference and Command Documentation - Research

**Researched:** 2026-02-27
**Domain:** Markdown documentation authoring — Starlight MDX/MD docs, Go source reading, command inventory diffing
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- Match existing module doc depth: emoji title, prose description, module aliases section, full command table with disableable column, usage examples section
- **devs module**: Prominent Starlight admonition/callout at page top: "These commands are restricted to bot owner. They are not available to group admins or regular users."
- **help module**: Commands + behavior overview — explain help menu routing (module buttons, back navigation, PM redirect)
- **language module**: Full depth matching existing pattern
- **users module**: Commands + behavior notes — describe bio/info system behavior (who sets it, who sees it, any limits)
- Fill ALL "No description available" entries by reading handler code behavior
- Terse, functional tone: "Refresh cached admin list", "Set anonymous admin verification"
- Normalize all existing inconsistent descriptions to the same terse/functional style in one pass
- Docs-only changes — do NOT modify Go handler registrations to add help strings (code changes are out of scope)
- Inline "Alias of /primary" notation in the Aliases column of command tables
- Each alias gets its own row in command tables: `/rmallbl | Alias of /remallbl | ...`
- Alias info shown in BOTH module page tables AND api-reference/commands.md
- Clarify "Module Aliases" section on module pages with note: "These are help-menu module names, not command aliases"
- Replace "Handler" column with "Description" column in api-reference/commands.md (user-facing, not internal function names)
- Add "Permission" column showing access level (User, Admin, Owner)
- Final table structure: Command | Description | Permission | Disableable | Aliases
- Corrected module/command counts in overview section only — no additional breakdown
- callbacks.md gets format specification + one encode/decode code example

### Claude's Discretion

- Exact Starlight admonition syntax/styling for devs warning
- Usage example selection for new module docs
- How to handle edge cases in description normalization (very long existing descriptions)
- callbacks.md code example language and detail level

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope

</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| DOCS-01 | Fix stale command count badges in `docs/src/content/docs/commands/index.mdx` | Inventory shows exact counts: devs (9), help (4), users (0) are missing from index.mdx. All existing badge counts (admin:8, bans:10, etc.) are already correct. The header "21 modules / 120+" is stale — correct is 24 user-facing modules, 142 commands |
| DOCS-02 | Add all missing commands to `api-reference/commands.md` | Inventory vs docs diff confirms 13 missing commands: `start`, `help`, `donate`, `about` (help module) + `stats`, `adddev`, `addsudo`, `remdev`, `remsudo`, `chatinfo`, `chatlist`, `leavechat`, `teamusers` (devs module) |
| DOCS-03 | Document all command aliases explicitly | Alias inventory is complete: `remallbl/rmallbl` (blacklists), `markdownhelp/formatting` (formatting), `privnote/privatenotes` (notes), `resetrules/clearrules` (rules). Existing docs show aliases inconsistently — need inline "Alias of /primary" notation |
| DOCS-04 | Verify and correct disableable column in docs against `AddCmdToDisableable()` | Inventory is authoritative: 17 disableable commands across 11 modules. All existing badges in commands.md appear correct per inventory cross-check |
| DOCS-05 | Update `callbacks.md` codec format | Current docs show old dot-notation. Actual format from `alita/utils/callbackcodec/callbackcodec.go`: `<namespace>|v1|<url-encoded-fields>` with empty payload represented as `_` |
| DOCS-06 | Create docs for undocumented modules: devs, help, language (already has docs), users | Inventory confirms: devs (no docs), help (no docs), users (no docs). language maps to `languages/` dir — already has `index.md` but needs content improvements per locked decisions |
| DOCS-07 | Verify permission requirements listed in docs match actual `Require*` calls in handlers | Handler code read confirms: devs commands require `OwnerId` check (owner-only) or `memStatus.Dev` check. help commands have no permission guards. language requires admin in groups, any user in private. users module has no commands (passive watcher only) |

</phase_requirements>

## Summary

Phase 2 is a pure documentation editing phase. No Go code changes are needed. The canonical command inventory from Phase 1 provides the complete ground truth — the work is exactly the diff between that inventory and the current docs files.

The gaps are well-defined: 13 commands missing from `api-reference/commands.md` (the entire `devs` and `help` modules); 3 new module doc pages to create (`devs/index.md`, `help/index.md`, `users/index.md`); 1 module (`languages/index.md`) that needs depth improvements; the callbacks.md needs the versioned codec format documented; and `index.mdx` needs updated module/command counts and cards for the 3 missing modules.

The existing docs style is consistent and discoverable from codebase inspection: Starlight `.md` files, emoji headings, prose description, "Module Aliases" section (for help-menu names), "Available Commands" table (Command | Description | Disableable), "Usage Examples" section, "Required Permissions" section. The table format change requested (adding Permission column, replacing Handler with Description) applies only to `api-reference/commands.md`, not to module pages.

**Primary recommendation:** Work module-page-first: create the 3 new module pages (devs, help, users), then update the api-reference files, then fix index.mdx counts/cards. All work is additive editing with zero risk of breaking existing functionality.

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Starlight (Astro) | Already installed | Docs framework | Project uses it; no change needed |
| Markdown (.md) | CommonMark | Module page format | All existing module pages use .md |
| MDX (.mdx) | Already installed | Commands index page | index.mdx uses JSX components |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `@astrojs/starlight/components` | Already installed | Badge, Card, LinkCard | Used in index.mdx for module cards |
| Starlight admonitions (`:::type`) | Built-in | Callout boxes | devs page warning; already used in troubleshooting.md |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `.md` module pages | `.mdx` with JSX | Unnecessary for content-only pages; all existing module pages use `.md` |

**Installation:** None required — all dependencies already installed.

## Architecture Patterns

### Recommended Project Structure

```
docs/src/content/docs/
├── commands/
│   ├── index.mdx           # Fix: module counts, add cards for devs/help
│   ├── devs/index.md       # Create new
│   ├── help/index.md       # Create new
│   ├── users/index.md      # Create new
│   └── languages/index.md  # Update: full depth, better descriptions
└── api-reference/
    ├── commands.md          # Fix: add 13 missing commands, new columns, aliases
    └── callbacks.md         # Fix: versioned codec format + code example
```

### Pattern 1: Module Page Structure

**What:** All module pages follow this exact template order
**When to use:** All new module pages AND when editing existing ones to normalize descriptions

```markdown
---
title: [Module] Commands
description: Complete guide to [Module] module commands and features
---

# [emoji] [Module] Commands

[Prose description — 1-3 paragraphs explaining what the module does and how it works]

## Module Aliases

This module can be accessed using the following aliases:

- `alias1`
- `alias2`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/cmd` | Terse action description | ❌ |

## Usage Examples

### Basic Usage

```
/cmd [args]
```

## Required Permissions

[Permission statement]
```

### Pattern 2: Starlight Admonition for devs Warning

**What:** Callout box at the top of the devs module page, above the prose description
**When to use:** devs module page only (owner-restricted commands)

```markdown
:::caution
These commands are restricted to bot owner. They are not available to group admins or regular users.
:::
```

Confirmed syntax from `docs/src/content/docs/self-hosting/troubleshooting.md` — `:::caution` without brackets is the standard bare form. With brackets adds a custom title: `:::caution[Custom Title]`.

### Pattern 3: api-reference/commands.md Table Format

**What:** New column structure replacing "Handler" with "Description" and adding "Permission"
**When to use:** api-reference/commands.md only — NOT on module pages

```markdown
| Command | Description | Permission | Disableable | Aliases |
|---------|-------------|------------|-------------|---------|
| `/start` | Show welcome message with navigation menu | Everyone | ❌ | — |
| `/addsudo` | Grant elevated permissions to a user | Owner | ❌ | — |
| `/remallbl` | Remove all blacklisted words from chat | Admin | ❌ | remallbl, rmallbl |
```

Permission levels:
- **Everyone** — no guard, any user can execute
- **Admin** — `RequireUserAdmin()` or `RequireUserOwner()` in handler
- **Owner** — `user.Id != config.AppConfig.OwnerId` check in handler
- **Dev/Owner** — devs module pattern: `user.Id != config.AppConfig.OwnerId && !memStatus.Dev`

### Pattern 4: Alias Rows in Module Page Tables

**What:** Each alias gets its own row, with "Alias of /primary" in the Description column
**When to use:** Any command registered via `MultiCommand` (8 total: remallbl/rmallbl, markdownhelp/formatting, privnote/privatenotes, resetrules/clearrules)

```markdown
| `/rmallbl` | Alias of `/remallbl` | ❌ |
```

### Pattern 5: callbacks.md Versioned Format

**What:** Document the actual codec format with a complete encode+decode code example
**When to use:** Replacing the old dot-notation description in callbacks.md

```markdown
## Callback Data Format

Callbacks use a versioned codec with URL-encoded fields:

```
<namespace>|v1|<url-encoded-fields>
```

For example: `restrict|v1|a=ban&uid=123456789` routes to the `restrict` namespace handler.

When the payload has no fields, `_` is used as placeholder: `helpq|v1|m=Help`.

**Maximum length**: 64 bytes (Telegram's `callback_data` limit).
```

Go code example (from `alita/utils/callbackcodec/callbackcodec.go`):

```go
// Encode
data, err := callbackcodec.Encode("restrict", map[string]string{"a": "ban", "uid": "123456789"})
// → "restrict|v1|a=ban&uid=123456789"

// Decode
decoded, err := callbackcodec.Decode(data)
namespace := decoded.Namespace  // "restrict"
action, _ := decoded.Field("a") // "ban"
```

### Anti-Patterns to Avoid

- **Creating new command tables in module pages with Handler column**: The Handler column is api-reference/commands.md only — module pages never had it
- **Adding "Module Aliases" confusion**: The "Module Aliases" section on module pages lists help-menu names (e.g., `admins`, `promote`, `demote`), NOT command aliases. Add a clarifying note to each section: "These are help-menu module names, not command aliases."
- **Modifying Go source**: This entire phase is docs-only. No handler registrations, no help text in Go files
- **Using `.mdx` for new module pages**: Existing pattern is `.md` for all module pages. Only `index.mdx` uses MDX

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Counting commands per module | Manual counting | INVENTORY.json already computed | Risk of counting errors; inventory is authoritative |
| Determining permission levels | Re-reading all handlers | Pattern from this research (Owner check pattern, Admin check, none) | Fully documented from handler inspection |
| Alias relationships | Re-parsing Go source | INVENTORY.json `aliases` field and `registration_pattern: MultiCommand` | Already extracted in Phase 1 |

**Key insight:** The INVENTORY.json is the authoritative source for all counts, disableable flags, and alias relationships. Never count manually — always verify against the inventory.

## Common Pitfalls

### Pitfall 1: Confusing "Module Aliases" with Command Aliases

**What goes wrong:** The "Module Aliases" section in existing docs lists names like `admins`, `promote`, `demote` — these are help-menu shortcut names registered with `HelpModule.AbleMap`, NOT command aliases. Command aliases are registered via `MultiCommand()`.

**Why it happens:** Both use the word "alias" but mean completely different things.

**How to avoid:** In each new module page's "Module Aliases" section, add the note: "These are help-menu module names, not command aliases." For actual command aliases (MultiCommand pairs), put them in the command table rows with "Alias of /primary" notation.

**Warning signs:** If you see `admins` listed as an alias but `/admins` is not a command, it's a help-menu name.

### Pitfall 2: Wrong Module/Command Count in index.mdx

**What goes wrong:** Current header says "21 modules / 120+" — both numbers are wrong.

**Why it happens:** devs, help, users modules were never added to the index; they're the 3 undocumented modules.

**How to avoid:**
- Correct module count: 24 user-facing modules (25 total minus bot_updates internal)
- Correct command count: 142 commands total
- The header `**Total Modules**: 21 | **Total Commands**: 120+` needs updating to `**Total Modules**: 24 | **Total Commands**: 142`
- Also add 3 new Card entries for devs, help, and potentially a note about users (passive module — no commands for users to run)

### Pitfall 3: devs Module — Two Access Tiers Within Same Module

**What goes wrong:** Treating all devs commands as "Owner only". Four commands (chatinfo, chatlist, leavechat, stats) require **Dev OR Owner** access; five commands (addsudo, adddev, remsudo, remdev, teamusers) require **Owner only**.

**Why it happens:** Both tiers are in the same module but have different guard patterns.

**How to avoid:** In the devs module page and api-reference/commands.md Permission column:
- `addsudo`, `adddev`, `remsudo`, `remdev`: Permission = "Owner"  (guard: `user.Id != config.AppConfig.OwnerId`)
- `chatinfo`, `chatlist`, `leavechat`, `stats`: Permission = "Dev/Owner" (guard: `user.Id != config.AppConfig.OwnerId && !memStatus.Dev`)
- `teamusers`: Permission = "Team" (guard: `FindInInt64Slice(teamint64Slice, user.Id)` — any team member including sudo)

### Pitfall 4: users Module Has No Commands

**What goes wrong:** Creating a command table for users module with a message like "No commands" or leaving the table empty.

**Why it happens:** users.go has only a passive message watcher (`logUsers`) with handler group -1. It logs every message to update the user/chat database. There are zero `/commands` registered.

**How to avoid:** The users module page should be a behavior-description page with no command table. Describe: automatic silent tracking of all message senders and chat membership; rate-limited database updates; no user-visible commands; powers other modules that need user/chat data (e.g., /info in misc uses this data).

### Pitfall 5: help Module Callbacks vs Commands Conflation

**What goes wrong:** Treating `helpq`, `about`, `configuration` as commands — they are callback handlers, not slash commands.

**Why it happens:** help.go registers both commands (`/start`, `/help`, `/donate`, `/about`) and callbacks (`helpq`, `about`, `configuration`).

**How to avoid:** The help module page documents 4 commands only. The 3 callbacks are documented in callbacks.md (already correct in current docs). The page should explain the UX flow: `/start` in PM → inline keyboard → module buttons → `/help <module>` deep-link via PM redirect.

### Pitfall 6: callbacks.md Old Format vs New Format

**What goes wrong:** Keeping or mixing the old `{prefix}{data}` dot-notation explanation alongside the new versioned format.

**Why it happens:** The old format (e.g., `restrict.ban.123456789`) was how callbacks worked before the codec was introduced. The codec has backward-compatible fallback for old-format data in handlers, but all new callbacks use `namespace|v1|url-encoded`.

**How to avoid:** Replace the entire "Callback Data Format" section with the versioned codec specification. Add a brief backward-compatibility note: "Legacy dot-notation (`prefix.field1.field2`) is accepted by handlers for backward compatibility but is deprecated. All new callbacks use the versioned format."

### Pitfall 7: Language Module — User Permission Asymmetry

**What goes wrong:** Documenting `/lang` as "Everyone" when it's asymmetric.

**Why it happens:** In private chats, any user can change their own language. In group chats, only admins can change the group language. The handler has an explicit admin check for group context.

**How to avoid:** Permission column entry = "User (self) / Admin (group)". Description = "Change bot language for yourself (private) or the group (requires admin in groups)."

## Code Examples

Verified patterns from source inspection:

### Callback Codec Encode/Decode (from alita/utils/callbackcodec/callbackcodec.go)

```go
// Encode: namespace|v1|url-query-encoded-fields
data, err := callbackcodec.Encode("helpq", map[string]string{"m": "Help"})
// → "helpq|v1|m=Help"

// Empty payload uses "_" placeholder
data, err := callbackcodec.Encode("helpq", map[string]string{})
// → "helpq|v1|_"

// Decode
decoded, err := callbackcodec.Decode("helpq|v1|m=Help")
module, _ := decoded.Field("m") // "Help"

// Max length: 64 bytes (Telegram limit enforced at encode time)
```

### devs Module Permission Guard Patterns (from alita/modules/devs.go)

```go
// Dev OR Owner tier (chatinfo, chatlist, leavechat, stats):
if user.Id != config.AppConfig.OwnerId && !memStatus.Dev {
    return ext.ContinueGroups
}

// Owner-only tier (addsudo, adddev, remsudo, remdev):
if user.Id != config.AppConfig.OwnerId {
    return ext.ContinueGroups
}

// Team member tier (teamusers) — any team member including sudo:
if !string_handling.FindInInt64Slice(teamint64Slice, user.Id) {
    return ext.EndGroups
}
```

### Language Module Asymmetric Permission (from alita/modules/language.go)

```go
// In group chats only:
if !chat_status.RequireUserAdmin(b, ctx, chat, user.Id, false) {
    return ext.EndGroups
}
// In private chats: no admin check required
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Callback format: `prefix.field1.field2` | `namespace\|v1\|url-encoded-fields` | Phase 1 era | callbacks.md must document new format; old format is backward-compatible in handlers |
| Manual docs authoring with stale counts | INVENTORY.json as authoritative source | Phase 1 completion | All counts must come from inventory, not memory |

**Deprecated/outdated:**
- Old dot-notation callback format: `restrict.ban.123456789` — replaced by versioned codec; still accepted by handlers via fallback
- "Handler" column in api-reference/commands.md — replace with "Description" column

## Open Questions

1. **Should `users` module get a card in index.mdx?**
   - What we know: users module has no commands — it's a passive background tracker
   - What's unclear: Is it user-visible enough to warrant a card in the commands overview? Including it increases module count accuracy (currently 21 shown, should be 24) but may confuse users who click and find no commands
   - Recommendation: Include a card in index.mdx with "No commands — automatic" badge to accurately reflect the 24-module count. Without it, the module count would still be wrong (24 vs 21 or 23).

2. **help module — what card category does it belong to in index.mdx?**
   - What we know: help module provides /start, /help, /donate, /about
   - What's unclear: Does it go under "Bot Management" (with Connections, Disabling, Languages, Pins) or a new "General" section?
   - Recommendation: Add to "Bot Management" section alongside Languages — it fits the "meta-bot-operation" category.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Astro build (Starlight with starlight-links-validator) |
| Config file | `docs/astro.config.mjs` |
| Quick run command | `cd docs && bun run build` |
| Full suite command | `cd docs && bun run build` (build validates links) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| DOCS-01 | Badge counts in index.mdx match inventory | Manual verify | `python3 -c "import json; d=json.load(open('.planning/INVENTORY.json')); [print(m['module'],len(m.get('commands') or [])) for m in d]"` | ✅ |
| DOCS-02 | All 13 missing commands appear in commands.md | Smoke | `grep -c "start\|/help\|donate\|about\|adddev\|addsudo\|chatinfo\|chatlist\|leavechat\|remdev\|remsudo\|stats\|teamusers" docs/src/content/docs/api-reference/commands.md` | ✅ |
| DOCS-03 | Alias notation present in commands.md | Smoke | `grep -c "Alias of" docs/src/content/docs/api-reference/commands.md` | ✅ |
| DOCS-04 | Disableable column matches inventory | Manual verify | Cross-check `✅` counts in commands.md against inventory disableable list | ✅ |
| DOCS-05 | versioned codec format in callbacks.md | Smoke | `grep "namespace|v1" docs/src/content/docs/api-reference/callbacks.md` | ✅ |
| DOCS-06 | New module doc directories exist | File check | `test -f docs/src/content/docs/commands/devs/index.md && test -f docs/src/content/docs/commands/help/index.md && test -f docs/src/content/docs/commands/users/index.md` | ❌ Wave 0 |
| DOCS-07 | Permission column in commands.md matches handler code | Manual verify | Read devs.go and help.go permission guards vs commands.md entries | ✅ |

### Sampling Rate

- **Per task commit:** `cd /Users/divkix/GitHub/Alita_Robot/docs && bun run build 2>&1 | tail -5`
- **Per wave merge:** `cd /Users/divkix/GitHub/Alita_Robot/docs && bun run build`
- **Phase gate:** Full build green with zero link validation errors before `/gsd:verify-work`

### Wave 0 Gaps

- [ ] `docs/src/content/docs/commands/devs/index.md` — covers DOCS-06 (devs module)
- [ ] `docs/src/content/docs/commands/help/index.md` — covers DOCS-06 (help module)
- [ ] `docs/src/content/docs/commands/users/index.md` — covers DOCS-06 (users module)

These files must be created in the first task before validation commands can pass.

## Sources

### Primary (HIGH confidence)

- `.planning/INVENTORY.json` — canonical command inventory from Phase 1; all counts derived from this
- `alita/modules/devs.go` — permission guard patterns verified by direct read
- `alita/modules/help.go` — command registration and callback registration verified by direct read
- `alita/modules/users.go` — confirmed no commands, passive watcher only
- `alita/modules/language.go` — asymmetric permission pattern verified by direct read
- `alita/utils/callbackcodec/callbackcodec.go` — versioned codec format `namespace|v1|url-encoded-fields` verified by direct read
- `docs/src/content/docs/api-reference/commands.md` — current state verified; 13 commands confirmed missing
- `docs/src/content/docs/commands/index.mdx` — current badges verified; header "21 modules / 120+" confirmed stale

### Secondary (MEDIUM confidence)

- Starlight admonition syntax: `:::caution`, `:::caution[Title]`, `:::tip[Title]` — verified from multiple existing files in `docs/src/content/docs/self-hosting/` and `docs/src/content/docs/commands/index.mdx`

### Tertiary (LOW confidence)

None

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — Starlight already installed, syntax verified from existing files
- Architecture: HIGH — module page pattern extracted from 5+ existing modules; no guesswork
- Pitfalls: HIGH — all derived from direct source code inspection, not training data
- Validation: HIGH — Astro build command verified from CLAUDE.md (`make docs-dev` uses bun)

**Research date:** 2026-02-27
**Valid until:** Until INVENTORY.json is regenerated (if Go source changes) — 90 days otherwise
