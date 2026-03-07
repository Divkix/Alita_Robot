## Context

The docs generation pipeline (`scripts/generate_docs/`) reads locale YAML files and Go source to produce Markdown for the Astro/Starlight docs site. The `convertTelegramMarkdown()` function (generators.go:719-739) performs minimal conversion: `<b>→**`, `<i>→*`, `<code>→` `` ` ``, and `•/·→-`. This misses the majority of patterns in the actual locale files.

**Formatting patterns found in `locales/en.yml`:**

| Pattern | Example | Frequency | Currently Handled |
|---------|---------|-----------|-------------------|
| `×` bullet marker | `× /flood: Get the...` | Every help_msg | No |
| `*text*` Telegram bold | `*Admin commands*:` | Every help_msg | No (stays as italic) |
| `->` sub-example | `-> /filter hello Hi` | filters, notes | No |
| `<b>`, `<code>` HTML | `<b>Anonymous Admin</b>` | Extended docs | Yes |
| `•` bullet | `• Feature item` | Extended docs | Yes |
| `&amp;`, `&lt;` entities | `Text &amp; Media` | Extended docs | No |
| Section headers ending `:` | `*Admin commands*:` | Every help_msg | No |
| YAML line wrapping | Multi-line continuation | All long strings | No (produces joined gibberish) |

## Goals / Non-Goals

**Goals:**
- All locale formatting patterns render as clean, readable Markdown
- Command listings render as structured lists with command in backtick code and description after
- Section headers (`*Admin commands*:`) render as `### Admin commands`
- Sub-examples render as indented code or lists
- HTML entities are decoded
- Unit tests cover each formatting pattern
- Regenerated docs pass visual inspection

**Non-Goals:**
- Modifying locale YAML files (the source format is fine for Telegram — the conversion layer is the problem)
- Supporting non-English locale rendering (docs site only uses `en.yml`)
- Changing the Astro/Starlight rendering pipeline
- Restructuring the overall doc generation architecture

## Decisions

### 1. Rewrite `convertTelegramMarkdown()` in-place vs. new function

**Decision**: Rewrite in-place + add `formatHelpText()` for help_msg-specific patterns.

**Rationale**: `convertTelegramMarkdown()` is used for both help_msg and extended_docs content. Help messages have unique patterns (`×` commands, `*bold*:` headers) that extended docs don't. A separate `formatHelpText()` wraps `convertTelegramMarkdown()` with additional help-specific transformations.

**Alternative considered**: Single monolithic function — rejected because help_msg and extended_docs have different formatting conventions.

### 2. `×` handling strategy

**Decision**: Convert `× /command: description` lines into `- \`/command\`: description` (Markdown list with command in backticks).

**Rationale**: The `×` character is a Telegram convention for command lists. In Markdown, these should be proper unordered list items with the command highlighted.

### 3. `*text*` Telegram bold handling

**Decision**: Convert `*text*` to `**text**` only when it appears as a section header pattern (e.g., `*Admin commands*:`). Leave standalone `*text*` as italic (Markdown default).

**Rationale**: In Telegram markdown, single `*` means bold. But blindly converting all `*text*` to `**text**` could break actual italic usage in extended docs. The help_msg pattern is consistent: `*Section Name*:` at line start → section header.

**Implementation**: Regex `^\*([^*]+)\*\s*:?\s*$` → `### $1`

### 4. `->` sub-example handling

**Decision**: Convert `-> /command args` to `  - \`/command args\`` (indented list item with code).

**Rationale**: These appear in filters/notes help as sub-examples under a parent item. Indented list items preserve the hierarchy.

### 5. YAML line wrapping

**Decision**: YAML flow scalars (quoted strings) produce joined lines with spaces. The YAML parser already handles this — the issue is that the joined text lacks Markdown line breaks. No special handling needed beyond the pattern conversions above.

**Rationale**: Go's `gopkg.in/yaml.v3` correctly joins YAML flow scalar continuation lines. The raw text from the parser is already a single continuous string per entry, which is correct for Markdown (where single newlines are ignored in paragraphs).

### 6. HTML entity decoding

**Decision**: Use `html.UnescapeString()` from Go stdlib after all other conversions.

**Rationale**: Simple, handles all standard HTML entities (`&amp;`, `&lt;`, `&gt;`, `&quot;`, numeric entities). Must run last to avoid interfering with HTML tag → Markdown conversion.

## Risks / Trade-offs

- **[False positive header detection]** → Mitigation: Only treat `*text*:` as headers when at line start with colon. Lines like `use *bold* text` stay as italic.
- **[Regenerated docs diff noise]** → Mitigation: Acceptable one-time churn. All 24 command docs will change. Review diff manually.
- **[Locale format drift]** → Mitigation: Unit tests pin expected conversions. If locale format changes, tests break early.
- **[Edge cases in `×` parsing]** → Mitigation: Some help messages use `×` without a command (just text). Pattern: only convert when followed by `/command`. Fallback: convert to plain `- ` list item.
