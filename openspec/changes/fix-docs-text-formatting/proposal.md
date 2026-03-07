## Why

The docs site renders module help text as raw dumps from locale YAML files. Telegram-specific formatting (`×` bullets, `*bold*` syntax, `->` sub-items, HTML entities like `&amp;`) passes through with minimal conversion, producing unstructured walls of text instead of properly formatted Markdown. The `convertTelegramMarkdown()` function in `scripts/generate_docs/generators.go` only handles `<b>/<i>/<code>` tags and `•/·` bullets — it misses the majority of formatting patterns actually used in locale files.

## What Changes

- **Rewrite `convertTelegramMarkdown()`** to handle all Telegram/locale formatting patterns:
  - `×` bullet markers → proper Markdown list items (`- `)
  - `->` sub-example markers → indented list items or code blocks
  - `*text*` Telegram bold → `**text**` Markdown bold
  - HTML entities (`&amp;`, `&lt;`, `&gt;`) → decoded characters
  - Section headers (lines ending in `:` like `*Admin commands*:`) → `### Header`
  - Multi-line command descriptions that wrap across YAML lines → joined single lines
  - Preserve intentional paragraph breaks (double newlines)
- **Add a `formatHelpText()` function** specifically for `_help_msg` content that structures command listings into proper Markdown (command in backticks, description after)
- **Regenerate all module docs** with the improved formatter
- **Add unit tests** for the conversion functions covering all formatting patterns

## Capabilities

### New Capabilities
- `docs-text-formatting`: Proper conversion of Telegram-flavored locale text (help messages, extended docs) into well-structured Markdown for the Astro docs site

### Modified Capabilities

## Impact

- **Code**: `scripts/generate_docs/generators.go` (rewrite `convertTelegramMarkdown`, add `formatHelpText`)
- **Generated output**: All 24 files in `docs/src/content/docs/commands/*/index.md` will be regenerated
- **No runtime impact**: This only affects the docs generation tooling, not the bot itself
- **No dependency changes**
