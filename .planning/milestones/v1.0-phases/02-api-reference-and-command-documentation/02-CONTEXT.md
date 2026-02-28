# Phase 2: API Reference and Command Documentation - Context

**Gathered:** 2026-02-27
**Status:** Ready for planning

<domain>
## Phase Boundary

Fix all command documentation to match actual registered handlers. Corrects stale counts, adds missing commands, documents aliases explicitly, verifies permissions and disableable status, updates callback codec format, and creates docs for four undocumented modules. The Go source code is the single source of truth — docs must reflect it accurately.

</domain>

<decisions>
## Implementation Decisions

### New Module Docs (devs, help, language, users)
- Match existing module doc depth: emoji title, prose description, module aliases section, full command table with disableable column, usage examples section
- **devs module**: Prominent Starlight admonition/callout at page top: "These commands are restricted to bot owner and developer users. They are not available to group admins or regular users." (Research revealed devs module has 3 permission tiers: Owner-only, Dev/Owner, and Team-member. Admonition text updated to reflect actual code.)
- **help module**: Commands + behavior overview — explain help menu routing (module buttons, back navigation, PM redirect)
- **language module**: Full depth matching existing pattern
- **users module**: Behavior description page — research found users.go is a passive background message watcher (handler group -1) that silently tracks user/chat IDs with rate-limiting. It has NO user-facing commands and no bio/info system. Page describes automatic tracking behavior only. (Original assumption of bio/info system was incorrect; corrected based on code inspection.)

### Missing Command Descriptions
- Fill ALL "No description available" entries by reading handler code behavior
- Terse, functional tone: "Refresh cached admin list", "Set anonymous admin verification"
- Normalize all existing inconsistent descriptions to the same terse/functional style in one pass
- Docs-only changes — do NOT modify Go handler registrations to add help strings (code changes are out of scope for this audit)

### Alias Presentation
- Inline "Alias of /primary" notation in the Aliases column of command tables
- Each alias gets its own row in command tables: `/rmallbl | Alias of /remallbl | ...`
- Alias info shown in BOTH module page tables AND api-reference/commands.md
- Clarify "Module Aliases" section on module pages with note: "These are help-menu module names, not command aliases"

### API Reference Table Format
- Replace "Handler" column with "Description" column (user-facing, not internal function names)
- Add "Permission" column showing access level (User, Admin, Owner)
- Final table structure: Command | Description | Permission | Disableable | Aliases
- Corrected module/command counts in overview section only — no additional breakdown
- callbacks.md gets format specification + one encode/decode code example

### Claude's Discretion
- Exact Starlight admonition syntax/styling for devs warning
- Usage example selection for new module docs
- How to handle edge cases in description normalization (very long existing descriptions)
- callbacks.md code example language and detail level

</decisions>

<specifics>
## Specific Ideas

- Descriptions should be action-oriented: "Refresh cached admin list" not "This command refreshes the admin cache"
- Help module docs should explain the UX flow users experience (menu navigation pattern)
- The canonical inventory from Phase 1 is the source of truth for all counts, commands, and aliases

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 02-api-reference-and-command-documentation*
*Context gathered: 2026-02-27*
