# Research: Increase Test Coverage on Most Important Parts of the Codebase

**Date:** 2026-02-21
**Goal:** Analyze current test coverage, identify highest-value areas for new tests, and document existing patterns to guide implementation.
**Confidence:** HIGH -- all claims verified by reading source files directly.

## Executive Summary

The codebase has ~35,000 lines of Go code with only ~1,184 lines of tests across 11 test files. The only package that actually passes tests is `callbackcodec` (79.1% coverage). Three packages (`alita/db`, `alita/modules`, `alita/utils/cache`) have tests that FAIL because they import the `config` package, which has an `init()` function that calls `log.Fatal` when `BOT_TOKEN` is missing. The CI pipeline works around this by setting dummy env vars (`BOT_TOKEN=test-token`) and providing a real PostgreSQL instance. The codebase has many pure functions with zero external dependencies that are trivially testable without any infrastructure, and several packages with ~0% coverage that contain critical business logic.

## Critical Blocker: Config `init()` Kills Test Runner

The single biggest obstacle to testing is `/Users/divkix/GitHub/Alita_Robot/alita/config/config.go` lines 536-568. The `init()` function calls `LoadConfig()` which calls `ValidateConfig()` which requires `BOT_TOKEN`, `OWNER_ID`, `MESSAGE_DUMP`, `DATABASE_URL`, and `REDIS_ADDRESS`. If any are missing, `log.Fatalf` kills the process.

**Impact:** ANY package that imports `config` directly or transitively (which is most of the codebase) cannot be tested locally without setting those env vars. The CI pipeline handles this by setting dummy values and running a real PostgreSQL service.

**Packages affected:**
- `alita/db` -- imports `config` in `init()` to get `DatabaseURL`
- `alita/modules` -- imports `db` which imports `config`
- `alita/utils/cache` -- imports `config` transitively
- `alita/utils/tracing` -- imports `config` transitively
- `alita/utils/helpers` -- imports `config`, `db`, `i18n`
- `alita/utils/chat_status` -- imports `db`, `cache`, `i18n`
- `alita/utils/extraction` -- imports `db`, `i18n`, `chat_status`

**Packages NOT affected (testable without env vars):**
- `alita/utils/callbackcodec` -- zero external deps (ALREADY TESTED, 79.1%)
- `alita/utils/string_handling` -- pure functions, zero deps
- `alita/utils/errors` -- pure functions, only stdlib deps
- `alita/utils/keyword_matcher` -- depends only on `ahocorasick` and `logrus`
- `alita/i18n` (errors.go, loader.go utilities) -- some pure functions
- `alita/config/types.go` -- `typeConvertor` is pure, no `init()` dependency

## Existing Test Patterns

### Pattern 1: Standard Library Table-Driven Tests
- **Location:** `/Users/divkix/GitHub/Alita_Robot/alita/utils/callbackcodec/callbackcodec_test.go`, `/Users/divkix/GitHub/Alita_Robot/alita/utils/cache/sanitize_test.go`
- **How it works:** Uses `testing.T` with table-driven subtests, `t.Parallel()`, `t.Fatalf()`, named test cases in structs
- **Relevant to this work because:** This is the established convention. No testify or other test frameworks are used. All tests use standard library `testing` only.

### Pattern 2: Integration Tests with Real DB
- **Location:** `/Users/divkix/GitHub/Alita_Robot/alita/db/captcha_db_test.go`, `/Users/divkix/GitHub/Alita_Robot/alita/db/locks_db_test.go`, `/Users/divkix/GitHub/Alita_Robot/alita/db/antiflood_db_test.go`, `/Users/divkix/GitHub/Alita_Robot/alita/db/update_record_test.go`
- **How it works:** Uses real `DB` global variable (GORM), calls `DB.AutoMigrate()` per test, uses `time.Now().UnixNano()` for unique IDs, `t.Cleanup()` for teardown, tests concurrency with `sync.WaitGroup`
- **Relevant to this work because:** DB tests require the CI environment (PostgreSQL + env vars). They test cache invalidation, zero-value boolean persistence, concurrent write safety, and atomic operations.

### Pattern 3: Pure Function Unit Tests
- **Location:** `/Users/divkix/GitHub/Alita_Robot/alita/modules/moderation_input_test.go`, `/Users/divkix/GitHub/Alita_Robot/alita/modules/misc_translate_parser_test.go`
- **How it works:** Tests package-internal functions (`buildModerationMatchText`, `parseTranslateResponse`) by constructing gotgbot structs directly, no mocking needed
- **Relevant to this work because:** Shows how to test internal module logic without Telegram API or DB access. The functions are exported within the package.

### Pattern 4: Tracing Context Tests
- **Location:** `/Users/divkix/GitHub/Alita_Robot/alita/utils/tracing/context_test.go`, `/Users/divkix/GitHub/Alita_Robot/alita/utils/tracing/processor_test.go`
- **How it works:** Tests context injection/extraction with `ext.Context` structs, covers nil cases, wrong types, cancelled contexts. Uses extracted helper (`injectTraceContext`) to test processor logic without full dispatcher.
- **Relevant to this work because:** Shows the pattern for testing code that normally integrates with the Telegram dispatcher -- extract the logic into a testable function.

## Dependencies & External Services

| Dependency | Version | Relevant API/Feature | Notes |
|-----------|---------|---------------------|-------|
| Go | 1.25.0 | Standard `testing` package | No test framework dependencies in go.mod |
| gotgbot/v2 | rc.33 | `gotgbot.Message`, `ext.Context` structs | Used to construct test inputs for handler tests |
| gorm.io/gorm | 1.31.1 | DB layer | Tests use real DB via global `DB` variable |
| cloudflare/ahocorasick | - | Pattern matching | Used by keyword_matcher, testable in isolation |
| gotg_md2html | - | Markdown to HTML | Used by helpers, rules_format |
| gocache | v4.2.3 | Redis caching | Tests check cache invalidation behavior |

**No test-only dependencies exist.** No testify, no gomock, no httptest wrappers. Everything uses standard library `testing`.

## Package-by-Package Coverage Analysis

### Tier 1: ZERO Tests, Trivially Testable (Pure Functions, No Infra)

| Package | File(s) | Functions to Test | LOC | Priority |
|---------|---------|-------------------|-----|----------|
| `alita/utils/string_handling` | `string_handling.go` | `FindInStringSlice`, `FindInInt64Slice`, `IsDuplicateInStringSlice` | 30 | HIGH |
| `alita/utils/errors` | `errors.go` | `Wrap`, `Wrapf`, `WrappedError.Error()`, `WrappedError.Unwrap()` | 62 | HIGH |
| `alita/config` (types only) | `types.go` | `typeConvertor.Bool()`, `.Int()`, `.Int64()`, `.Float64()`, `.StringArray()` | 50 | HIGH |
| `alita/utils/keyword_matcher` | `matcher.go` | `NewKeywordMatcher`, `FindMatches`, `HasMatch`, `GetPatterns` | 177 | HIGH |
| `alita/config` (validation) | `config.go` | `ValidateConfig()` (does not trigger `init()` if called directly on a struct) | 80 | MEDIUM |

### Tier 2: ZERO Tests, Testable With Env Vars (CI Only or Dummy Env)

| Package | File(s) | Functions to Test | LOC | Priority |
|---------|---------|-------------------|-----|----------|
| `alita/i18n` | All files | `extractLangCode`, `isYAMLFile`, `validateYAMLStructure`, `I18nError.Error()`, `selectPluralForm`, `interpolateParams`, `extractOrderedValues` | ~325 | HIGH |
| `alita/utils/helpers` | `channel_helpers.go`, `telegram_helpers.go`, `helpers.go` | `IsChannelID`, `IsExpectedTelegramError`, `SplitMessage`, `MentionHtml`, `MentionUrl`, `HtmlEscape`, `GetFullName`, `BuildKeyboard`, `ConvertButtonV2ToDbButton`, `RevertButtons`, `ChunkKeyboardSlices`, `ReverseHTML2MD`, `notesParser` | ~500 | HIGH |
| `alita/utils/chat_status` | `chat_status.go` | `IsValidUserId`, `IsChannelId` (pure functions) | 10 | HIGH |
| `alita/utils/shutdown` | `graceful.go` | `NewManager`, `RegisterHandler`, `executeHandler` (testable without signals) | 98 | MEDIUM |

### Tier 3: Existing Tests But Low Coverage or Failing

| Package | Current State | Gap |
|---------|---------------|-----|
| `alita/db` | 4 test files, FAILS without env vars | Tests exist but cannot run locally. Need CI env. Missing tests for many `*_db.go` files (15+ files with zero tests) |
| `alita/modules` | 3 test files, FAILS without env vars | Only `moderation_input`, `misc_translate_parser`, `callback_parse_overwrite` tested. 30+ module files have zero tests |
| `alita/utils/cache` | 1 test file (sanitize), FAILS without env vars | `sanitize_test.go` passes locally but package test fails due to config import from other files in package |
| `alita/utils/tracing` | 2 test files, FAILS without env vars | `context_test.go` and `processor_test.go` are well-written but fail because `tracing` package imports `config` |
| `alita/utils/callbackcodec` | 1 test file, 79.1% coverage, PASSES | Missing tests for `EncodeOrFallback`, `Decoded.Field` with nil receiver, empty fields edge cases |

### Tier 4: Hard to Test (Heavy Telegram API / Infrastructure Dependencies)

| Package | Why Hard | Recommendation |
|---------|----------|----------------|
| `alita/modules/*.go` (handlers) | Require `gotgbot.Bot`, `ext.Context`, Telegram API | Extract pure logic into testable functions (like `buildModerationMatchText` pattern) |
| `alita/utils/chat_status` (most functions) | Require `gotgbot.Bot` for API calls, cache for admin lookups | Test pure functions (`IsValidUserId`, `IsChannelId`) first, mock API for rest |
| `alita/utils/monitoring` | Background goroutines, system stats | Would need time-based testing with mocked clocks |
| `alita/utils/httpserver` | Full HTTP server | Use `httptest.NewServer` from stdlib |
| `main.go` | Full application bootstrap | Not worth unit testing |

## Risks & Conflicts

1. **Config init() blocks local testing** -- Any new test file in a package that transitively imports `config` will fail without env vars. Severity: HIGH. Affected: most packages. Workaround: Set dummy env vars in test runner, or restructure config to use lazy initialization.

2. **DB tests require real PostgreSQL** -- The `alita/db` package uses `init()` to connect to PostgreSQL via GORM. Tests in this package require a running PostgreSQL instance. Severity: MEDIUM. Workaround: CI already provides this. Consider `TestMain` with build tags for integration tests.

3. **No mocking infrastructure** -- No mock interfaces exist for `gotgbot.Bot`, cache, or DB. The codebase uses concrete types everywhere. Adding interfaces for testability is a significant refactor. Severity: MEDIUM for handler tests. Workaround: Focus on pure function tests first.

4. **Singleton i18n manager** -- `alita/i18n/manager.go` uses `sync.Once` for singleton. Test isolation requires understanding that the manager can only be initialized once per test binary. Severity: LOW.

5. **Package-level `init()` functions** -- `alita/db/db.go` and `alita/config/config.go` both have `init()` that run on import, making the packages hard to test in isolation. Severity: HIGH for any test that imports these packages transitively.

## Open Questions

- [ ] Should the config `init()` be refactored to support lazy initialization for testability? This would be a significant change but would unblock testing for ~70% of the codebase.
- [ ] Are there plans to add mock interfaces for `gotgbot.Bot`? The gotgbot library does not provide interfaces out of the box.
- [ ] Should DB integration tests be gated behind a build tag (e.g., `//go:build integration`) to allow `go test ./...` to pass without PostgreSQL?
- [ ] Is there a target coverage percentage the project aims for?

## Highest-Value Test Targets (Prioritized Implementation Plan)

### Phase 1: Pure Functions, Zero Infrastructure (Immediate, ~2 hours)

These can run anywhere without any env vars or services:

1. **`alita/utils/string_handling`** -- 3 simple functions, trivial table-driven tests
2. **`alita/utils/errors`** -- `Wrap`/`Wrapf` with nil errors, error chaining, `Unwrap()`
3. **`alita/config/types.go`** -- `typeConvertor` Bool/Int/Int64/Float64/StringArray with edge cases (empty string, invalid input, whitespace)
4. **`alita/utils/keyword_matcher`** -- `NewKeywordMatcher`, `FindMatches` (case insensitivity, empty patterns, overlapping matches, concurrent access)
5. **`alita/utils/callbackcodec`** -- increase from 79.1% by testing `EncodeOrFallback`, `Field` on nil `Decoded`

### Phase 2: Pure Functions in Mixed Packages (Requires Dummy Env Vars, ~3 hours)

These are pure functions that happen to live in packages importing `config`:

6. **`alita/utils/helpers`** -- `IsChannelID`, `SplitMessage`, `MentionHtml`, `HtmlEscape`, `GetFullName`, `BuildKeyboard`, `ChunkKeyboardSlices`, `ReverseHTML2MD`, `IsExpectedTelegramError`, `notesParser`
7. **`alita/utils/chat_status`** -- `IsValidUserId`, `IsChannelId` (2 pure functions)
8. **`alita/i18n`** -- `extractLangCode`, `isYAMLFile`, `validateYAMLStructure`, `I18nError.Error()`, `interpolateParams`, `selectPluralForm`, `extractOrderedValues`
9. **`alita/config`** -- `ValidateConfig()` with various invalid/valid configs
10. **`alita/modules/rules_format.go`** -- `normalizeRulesForHTML` with HTML and markdown inputs

### Phase 3: DB Integration Tests (Requires PostgreSQL, CI-focused, ~4 hours)

11. **`alita/db/user_db.go`** -- user CRUD, username lookups
12. **`alita/db/chats_db.go`** -- chat CRUD, user membership
13. **`alita/db/filters_db.go`** -- filter CRUD, keyword matching
14. **`alita/db/notes_db.go`** -- note CRUD, settings
15. **`alita/db/warns_db.go`** -- warn CRUD, warn settings
16. **`alita/db/blacklists_db.go`** -- blacklist CRUD
17. **`alita/db/connections_db.go`** -- connection CRUD
18. **`alita/db/cache_helpers.go`** -- `getFromCacheOrLoad` with mock cache

### Phase 4: Shutdown & Utility Tests (~1 hour)

19. **`alita/utils/shutdown`** -- `RegisterHandler`, `executeHandler` (panic recovery), LIFO order
20. **`alita/db/optimized_queries.go`** -- optimized query functions

## File Inventory

Files that will likely need modification or that are critical context:

| File | Purpose | Relevance |
|------|---------|-----------|
| `/Users/divkix/GitHub/Alita_Robot/alita/config/config.go` | Config loading with `init()` | ROOT CAUSE of test failures; `init()` at line 536 calls `log.Fatal` |
| `/Users/divkix/GitHub/Alita_Robot/alita/config/types.go` | `typeConvertor` pure functions | Tier 1 test target, zero deps |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/string_handling/string_handling.go` | Slice search utilities | Tier 1 test target, zero deps |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/errors/errors.go` | Error wrapping with file/line info | Tier 1 test target, stdlib only |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/keyword_matcher/matcher.go` | Aho-Corasick pattern matching | Tier 1 test target, one external dep |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/callbackcodec/callbackcodec.go` | Callback data encoding/decoding | Existing tests at 79.1%, gap fillable |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/callbackcodec/callbackcodec_test.go` | Existing callbackcodec tests | Reference for test style |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/helpers/helpers.go` | Helper functions (keyboards, formatting, HTML) | Many pure testable functions |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/helpers/channel_helpers.go` | `IsChannelID` | 1 pure function, 7 lines |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/helpers/telegram_helpers.go` | `IsExpectedTelegramError`, message deletion/send helpers | Pure error classification function testable |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/chat_status/chat_status.go` | Permission system | `IsValidUserId`, `IsChannelId` are pure; rest needs bot mock |
| `/Users/divkix/GitHub/Alita_Robot/alita/i18n/loader.go` | Locale file loading | `extractLangCode`, `isYAMLFile`, `validateYAMLStructure` are pure |
| `/Users/divkix/GitHub/Alita_Robot/alita/i18n/translator.go` | Translation retrieval | `interpolateParams`, `selectPluralForm`, `extractOrderedValues` testable |
| `/Users/divkix/GitHub/Alita_Robot/alita/i18n/errors.go` | i18n error types | `I18nError.Error()`, `Unwrap()` testable |
| `/Users/divkix/GitHub/Alita_Robot/alita/modules/moderation_input.go` | Moderation text builder | Already tested, good reference |
| `/Users/divkix/GitHub/Alita_Robot/alita/modules/rules_format.go` | Rules HTML normalization | `normalizeRulesForHTML` is pure |
| `/Users/divkix/GitHub/Alita_Robot/alita/modules/callback_codec.go` | Module-level callback helpers | `encodeCallbackData`, `decodeCallbackData` |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/shutdown/graceful.go` | Graceful shutdown manager | `RegisterHandler`, `executeHandler` testable |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/db.go` | GORM models, DB connection, CRUD helpers | Has `init()` that connects to PostgreSQL |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/cache_helpers.go` | Cache key generation, `getFromCacheOrLoad` | Cache key functions are pure |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/captcha_db_test.go` | Existing captcha DB tests | Reference for DB test patterns |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/locks_db_test.go` | Existing locks DB tests | Reference for concurrent test patterns |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/antiflood_db_test.go` | Existing antiflood DB tests | Reference for zero-value boolean tests |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/update_record_test.go` | Existing update record tests | Reference for GORM error handling tests |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/cache/sanitize_test.go` | Existing sanitize tests | Reference for table-driven test style |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/tracing/context_test.go` | Existing context extraction tests | Reference for nil/edge case coverage |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/tracing/processor_test.go` | Existing tracing processor tests | Reference for extracted-function test pattern |
| `/Users/divkix/GitHub/Alita_Robot/Makefile` | Build commands | `make test` runs `go test -v -race -coverprofile=coverage.out -count=1 -timeout 10m ./...` |
| `/Users/divkix/GitHub/Alita_Robot/.github/workflows/ci.yml` | CI pipeline | Tests run with dummy env vars + real PostgreSQL service |

## Raw Notes

### Test Command Details
- `make test` = `go test -v -race -coverprofile=coverage.out -count=1 -timeout 10m ./...`
- CI sets: `BOT_TOKEN=test-token`, `OWNER_ID=1`, `MESSAGE_DUMP=1`, `REDIS_ADDRESS=localhost:6379`, `DATABASE_URL=postgres://postgres:postgres@localhost:5432/alita_test?sslmode=disable`
- CI provides PostgreSQL 16 service container but NO Redis service (tests that need Redis will skip/fail gracefully)

### Code Statistics
- Total Go code: ~35,016 lines
- Total test code: ~1,184 lines
- Test-to-code ratio: 3.4% (extremely low)
- Existing test files: 11
- Packages with zero tests: 22 out of ~30 packages
- Only 1 package passes tests locally: `callbackcodec`

### Functions Confirmed as Pure (Verified by Reading Source)

These functions have ZERO side effects, no DB calls, no API calls, no cache access:

1. `string_handling.FindInStringSlice(slice, val)` -- wraps `slices.Contains`
2. `string_handling.FindInInt64Slice(slice, val)` -- wraps `slices.Contains`
3. `string_handling.IsDuplicateInStringSlice(arr)` -- map-based duplicate detection
4. `errors.Wrap(err, message)` -- wraps error with runtime caller info
5. `errors.Wrapf(err, format, args...)` -- wraps with formatted message
6. `config.typeConvertor.Bool()` -- string to bool conversion
7. `config.typeConvertor.Int()` -- string to int conversion
8. `config.typeConvertor.Int64()` -- string to int64 conversion
9. `config.typeConvertor.Float64()` -- string to float64 conversion
10. `config.typeConvertor.StringArray()` -- comma-separated string to slice
11. `keyword_matcher.NewKeywordMatcher(patterns)` -- builds Aho-Corasick matcher
12. `keyword_matcher.FindMatches(text)` -- returns match positions
13. `keyword_matcher.HasMatch(text)` -- boolean match check
14. `keyword_matcher.GetPatterns()` -- returns copy of patterns
15. `helpers.IsChannelID(chatID)` -- `chatID < -1000000000000`
16. `chat_status.IsValidUserId(id)` -- `id > 0`
17. `chat_status.IsChannelId(id)` -- `id < -1000000000000`
18. `helpers.SplitMessage(msg)` -- splits by rune count at 4096
19. `helpers.MentionHtml(userId, name)` -- HTML link generation
20. `helpers.MentionUrl(url, name)` -- HTML link generation
21. `helpers.HtmlEscape(s)` -- HTML entity escaping
22. `helpers.GetFullName(first, last)` -- string concatenation
23. `helpers.BuildKeyboard(buttons)` -- converts DB buttons to TG keyboard
24. `helpers.ChunkKeyboardSlices(slice, chunkSize)` -- slice chunking
25. `helpers.ReverseHTML2MD(text)` -- HTML to markdown conversion
26. `helpers.IsExpectedTelegramError(err)` -- error string classification
27. `helpers.notesParser(sent)` -- regex-based option extraction
28. `i18n.extractLangCode(fileName)` -- strips file extension
29. `i18n.isYAMLFile(fileName)` -- checks extension
30. `i18n.validateYAMLStructure(content)` -- YAML validation
31. `i18n.I18nError.Error()` -- error formatting
32. `i18n.extractOrderedValues(params)` -- extracts ordered values from map
33. `i18n.selectPluralForm(rule, count)` -- plural form selection
34. `config.ValidateConfig(cfg)` -- struct validation (no side effects)
35. `modules.normalizeRulesForHTML(rawRules)` -- HTML detection + MD conversion
36. `db.ButtonArray.Scan/Value` -- JSON serialization
37. `db.StringArray.Scan/Value` -- JSON serialization
38. `db.Int64Array.Scan/Value` -- JSON serialization
39. `db.BlacklistSettingsSlice.Triggers()` -- extracts words from slice
40. `db.BlacklistSettingsSlice.Action()` -- returns first action
41. `db.BlacklistSettingsSlice.Reason()` -- returns first reason or default
42. `shutdown.NewManager()` -- creates struct
43. `shutdown.RegisterHandler(handler)` -- appends to slice
44. `shutdown.executeHandler(handler, index)` -- runs with panic recovery

### DB Files with Zero Tests (15 files)
- `/Users/divkix/GitHub/Alita_Robot/alita/db/admin_db.go`
- `/Users/divkix/GitHub/Alita_Robot/alita/db/blacklists_db.go`
- `/Users/divkix/GitHub/Alita_Robot/alita/db/channels_db.go`
- `/Users/divkix/GitHub/Alita_Robot/alita/db/chats_db.go`
- `/Users/divkix/GitHub/Alita_Robot/alita/db/connections_db.go`
- `/Users/divkix/GitHub/Alita_Robot/alita/db/devs_db.go`
- `/Users/divkix/GitHub/Alita_Robot/alita/db/disable_db.go`
- `/Users/divkix/GitHub/Alita_Robot/alita/db/filters_db.go`
- `/Users/divkix/GitHub/Alita_Robot/alita/db/greetings_db.go`
- `/Users/divkix/GitHub/Alita_Robot/alita/db/lang_db.go`
- `/Users/divkix/GitHub/Alita_Robot/alita/db/notes_db.go`
- `/Users/divkix/GitHub/Alita_Robot/alita/db/pin_db.go`
- `/Users/divkix/GitHub/Alita_Robot/alita/db/reports_db.go`
- `/Users/divkix/GitHub/Alita_Robot/alita/db/rules_db.go`
- `/Users/divkix/GitHub/Alita_Robot/alita/db/user_db.go`
- `/Users/divkix/GitHub/Alita_Robot/alita/db/warns_db.go`
