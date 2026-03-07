## ADDED Requirements

### Requirement: Cross-bullet conversion
The `convertTelegramMarkdown()` function SHALL convert `×` bullet markers to Markdown list items (`- `).

#### Scenario: Command bullet line
- **WHEN** input contains `× /flood: Get the current antiflood settings.`
- **THEN** output SHALL be `- \`/flood\`: Get the current antiflood settings.`

#### Scenario: Non-command bullet line
- **WHEN** input contains `× Some plain text item`
- **THEN** output SHALL be `- Some plain text item`

#### Scenario: Multiple consecutive bullets
- **WHEN** input contains three consecutive `×` lines
- **THEN** output SHALL produce three consecutive `- ` list items with no blank lines between them

### Requirement: Telegram bold to Markdown bold
The system SHALL convert Telegram-style `*text*` to Markdown bold `**text**`.

#### Scenario: Section header pattern
- **WHEN** input contains `*Admin commands*:` at line start
- **THEN** output SHALL be `### Admin commands`

#### Scenario: Inline bold in help text
- **WHEN** input contains `*User Commands*:` followed by command lines
- **THEN** output SHALL render `### User Commands` as a Markdown heading

#### Scenario: Bold within sentence
- **WHEN** input contains `use *bold* formatting in text`
- **THEN** output SHALL be `use **bold** formatting in text`

### Requirement: Sub-example arrow conversion
The system SHALL convert `->` prefixed lines to indented list items.

#### Scenario: Filter example
- **WHEN** input contains `-> /filter hello Hello there!`
- **THEN** output SHALL be `  - \`/filter hello Hello there!\``

#### Scenario: Multiple sub-examples
- **WHEN** input contains consecutive `->` lines under a parent item
- **THEN** output SHALL produce consecutive indented `  - ` items

### Requirement: HTML entity decoding
The system SHALL decode HTML entities to their character equivalents.

#### Scenario: Ampersand entity
- **WHEN** input contains `Text &amp; Media`
- **THEN** output SHALL be `Text & Media`

#### Scenario: Less-than entity
- **WHEN** input contains `Set to &lt;number&gt;`
- **THEN** output SHALL be `Set to <number>`

#### Scenario: Entity decoding runs after HTML tag conversion
- **WHEN** input contains `<b>Text &amp; Media</b>`
- **THEN** output SHALL be `**Text & Media**` (HTML tags converted first, then entities decoded)

### Requirement: HTML tag to Markdown conversion
The system SHALL convert Telegram HTML tags to Markdown equivalents.

#### Scenario: Bold tags
- **WHEN** input contains `<b>Important</b>`
- **THEN** output SHALL be `**Important**`

#### Scenario: Italic tags
- **WHEN** input contains `<i>note</i>`
- **THEN** output SHALL be `*note*`

#### Scenario: Code tags
- **WHEN** input contains `<code>/command</code>`
- **THEN** output SHALL be `` `/command` ``

### Requirement: Bullet point conversion
The system SHALL convert `•` and `·` bullet characters to Markdown list items.

#### Scenario: Bullet dot
- **WHEN** input contains `• Feature item`
- **THEN** output SHALL be `- Feature item`

#### Scenario: Middle dot
- **WHEN** input contains `· Another item`
- **THEN** output SHALL be `- Another item`

### Requirement: Help text formatting function
A dedicated `formatHelpText()` function SHALL process `_help_msg` content with help-specific patterns before calling general conversion.

#### Scenario: Full antiflood help text
- **WHEN** `formatHelpText()` receives the `antiflood_help_msg` locale value
- **THEN** output SHALL contain `### Admin commands` as a heading, command lines as `- \`/command\`: description` list items, and proper paragraph spacing

#### Scenario: Full filters help text
- **WHEN** `formatHelpText()` receives the `filters_help_msg` locale value
- **THEN** output SHALL contain `Commands:` as a heading, command lines as proper list items, and `->` examples as indented sub-items

### Requirement: Unit test coverage
All conversion functions SHALL have unit tests covering each formatting pattern.

#### Scenario: Test file exists
- **WHEN** the test suite is run
- **THEN** `scripts/generate_docs/generators_test.go` (or `formatting_test.go`) SHALL exist and pass

#### Scenario: Pattern coverage
- **WHEN** tests run
- **THEN** there SHALL be test cases for: `×` bullets, `*bold*` headers, `->` arrows, HTML entities, `<b>/<i>/<code>` tags, `•/·` bullets, and combined patterns

### Requirement: Regenerated docs correctness
After running `make generate-docs`, all command doc pages SHALL render with proper Markdown formatting.

#### Scenario: No raw `×` in output
- **WHEN** `make generate-docs` completes
- **THEN** no file in `docs/src/content/docs/commands/` SHALL contain the `×` character

#### Scenario: No unescaped HTML entities
- **WHEN** `make generate-docs` completes
- **THEN** no file in `docs/src/content/docs/commands/` SHALL contain `&amp;`, `&lt;`, or `&gt;` outside of code blocks
