## 1. Core Conversion Functions

- [x] 1.1 Rewrite `convertTelegramMarkdown()` in `scripts/generate_docs/generators.go`: add `×` bullet conversion (with `/command` detection for backtick wrapping), HTML entity decoding via `html.UnescapeString()`, and `->` sub-example conversion to indented list items
- [x] 1.2 Add `formatHelpText()` function that pre-processes `_help_msg` content: convert `*Section*:` headers to `### Section`, then delegate to `convertTelegramMarkdown()` for remaining patterns
- [x] 1.3 Handle Telegram `*bold*` to Markdown `**bold**` conversion — distinguish section headers (`*text*:` at line start → `###`) from inline bold (`*text*` mid-sentence → `**text**`)

## 2. Integration

- [x] 2.1 Update `generateModuleDocs()` to call `formatHelpText()` for help text content instead of raw `convertTelegramMarkdown()`
- [x] 2.2 Keep `convertTelegramMarkdown()` for extended docs, features, permissions, examples, and notes sections

## 3. Tests

- [x] 3.1 Create `scripts/generate_docs/formatting_test.go` with unit tests for `convertTelegramMarkdown()` covering: `×` bullets, `•/·` bullets, `<b>/<i>/<code>` tags, HTML entities, `->` arrows
- [x] 3.2 Add unit tests for `formatHelpText()` covering: `*Section*:` header conversion, full antiflood help text, full filters help text with `->` examples
- [x] 3.3 Add combined/edge-case tests: nested formatting, empty input, content with no special patterns

## 4. Regenerate & Verify

- [x] 4.1 Run `make generate-docs` to regenerate all command docs
- [x] 4.2 Verify no `×` character remains in any generated command doc
- [x] 4.3 Verify no `&amp;`/`&lt;`/`&gt;` remains outside code blocks in generated docs
- [x] 4.4 Visually spot-check antiflood, filters, notes, and admin generated docs for correct formatting
- [x] 4.5 Run `make lint` and `go test ./scripts/generate_docs/...` to confirm no regressions
