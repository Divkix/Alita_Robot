## 1. Core Conversion Functions

- [ ] 1.1 Rewrite `convertTelegramMarkdown()` in `scripts/generate_docs/generators.go`: add `×` bullet conversion (with `/command` detection for backtick wrapping), HTML entity decoding via `html.UnescapeString()`, and `->` sub-example conversion to indented list items
- [ ] 1.2 Add `formatHelpText()` function that pre-processes `_help_msg` content: convert `*Section*:` headers to `### Section`, then delegate to `convertTelegramMarkdown()` for remaining patterns
- [ ] 1.3 Handle Telegram `*bold*` to Markdown `**bold**` conversion — distinguish section headers (`*text*:` at line start → `###`) from inline bold (`*text*` mid-sentence → `**text**`)

## 2. Integration

- [ ] 2.1 Update `generateModuleDocs()` to call `formatHelpText()` for help text content instead of raw `convertTelegramMarkdown()`
- [ ] 2.2 Keep `convertTelegramMarkdown()` for extended docs, features, permissions, examples, and notes sections

## 3. Tests

- [ ] 3.1 Create `scripts/generate_docs/formatting_test.go` with unit tests for `convertTelegramMarkdown()` covering: `×` bullets, `•/·` bullets, `<b>/<i>/<code>` tags, HTML entities, `->` arrows
- [ ] 3.2 Add unit tests for `formatHelpText()` covering: `*Section*:` header conversion, full antiflood help text, full filters help text with `->` examples
- [ ] 3.3 Add combined/edge-case tests: nested formatting, empty input, content with no special patterns

## 4. Regenerate & Verify

- [ ] 4.1 Run `make generate-docs` to regenerate all command docs
- [ ] 4.2 Verify no `×` character remains in any generated command doc
- [ ] 4.3 Verify no `&amp;`/`&lt;`/`&gt;` remains outside code blocks in generated docs
- [ ] 4.4 Visually spot-check antiflood, filters, notes, and admin generated docs for correct formatting
- [ ] 4.5 Run `make lint` and `go test ./scripts/generate_docs/...` to confirm no regressions
