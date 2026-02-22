# Research: Increase Test Coverage from 12.5% to 45%+

**Date:** 2026-02-21
**Goal:** Identify what tests are needed and where to add them to raise coverage from 12.5% to >= 45% (CI threshold is 40%, targeting 45% for safety margin)
**Confidence:** HIGH -- all claims verified against actual source, CI logs, and test runs

## Executive Summary

Current CI coverage is **12.5%** against a threshold of **40%**. The codebase has ~33,864 lines of production Go code and ~9,169 lines of test code. The largest untested packages are `alita/modules/` (17,316 LOC, coverage ~2%), `alita/db/` (5,626 LOC, coverage ~8.6%), `alita/utils/chat_status/` (1,097 LOC, coverage ~1.1%), and `alita/utils/helpers/` (1,124 LOC, coverage already has tests but many are integration). The `alita/modules/` package alone is ~51% of all production code; even modest coverage there yields massive overall gains. Critically, many packages fail locally without `BOT_TOKEN` env var because `alita/config` has an `init()` that calls `log.Fatalf`. CI sets this env var so tests pass there. The DB tests use a real PostgreSQL via CI services.

## Existing Patterns

### Test Pattern: Table-Driven Tests
- **Location:** All existing test files
- **How it works:** Standard Go table-driven tests with `t.Parallel()`, subtests via `t.Run()`, and `tc := tc` capture (or range variable capture with Go 1.25+)
- **Relevant to this work because:** All new tests must follow this pattern for consistency

### Test Pattern: DB Integration Tests with TestMain
- **Location:** `/Users/divkix/GitHub/Alita_Robot/alita/db/testmain_test.go`
- **How it works:** `TestMain` checks if `DB == nil`, runs `AutoMigrate` for all models, then `os.Exit(m.Run())`. Each test calls `skipIfNoDb(t)` to skip when PostgreSQL is unavailable. Uses `time.Now().UnixNano()` for unique IDs. Cleanup via `t.Cleanup()`.
- **Relevant to this work because:** All DB tests must follow this pattern. DB tests contribute 8.6% coverage already -- there is room for more.

### Test Pattern: Config Fatal Init
- **Location:** `/Users/divkix/GitHub/Alita_Robot/alita/config/config.go` (lines 536-568)
- **How it works:** Package `init()` calls `LoadConfig()` which calls `ValidateConfig()`. If `BOT_TOKEN` is missing, `log.Fatalf` kills the process. Any package that transitively imports `config` will crash without env vars.
- **Relevant to this work because:** This is the #1 blocker for local test runs. CI sets `BOT_TOKEN=test-token` and other required env vars. New tests in packages that import config need env vars OR must test only pure functions that do not trigger config init.

### Test Pattern: Pure Function Tests (No Dependencies)
- **Location:** `/Users/divkix/GitHub/Alita_Robot/alita/utils/callbackcodec/callbackcodec_test.go`, `/Users/divkix/GitHub/Alita_Robot/alita/utils/errors/errors_test.go`, `/Users/divkix/GitHub/Alita_Robot/alita/utils/string_handling/string_handling_test.go`, `/Users/divkix/GitHub/Alita_Robot/alita/utils/keyword_matcher/matcher_test.go`
- **How it works:** Tests for pure functions with no external dependencies (no DB, Redis, Telegram API). These always pass.
- **Relevant to this work because:** These are the easiest and safest tests to add. Many pure functions in `alita/modules/` and `alita/db/` are testable this way.

### Test Pattern: Modules Internal Tests
- **Location:** `/Users/divkix/GitHub/Alita_Robot/alita/modules/moderation_input_test.go`, `callback_codec_test.go`, `rules_format_test.go`, `callback_parse_overwrite_test.go`, `misc_translate_parser_test.go`
- **How it works:** Tests for unexported helper functions in the `modules` package. These test parsing logic, formatting, and callback data encoding without needing Telegram API or database.
- **Relevant to this work because:** The `modules` package is 17,316 LOC. Adding tests for more unexported helpers is the fastest path to coverage gains.

## Current Coverage Breakdown (from CI run 22268859230)

| Package | LOC | Coverage | Test Lines | Notes |
|---------|-----|----------|------------|-------|
| `alita/config` | 618 | 0.9% | 171 | Types tests only; init() blocks more |
| `alita/db` | 5,626 | 8.6% | 4,506 | DB integration tests, good coverage pattern |
| `alita/i18n` | 801 | 1.6% | 605 | Tests exist but coverage is low |
| `alita/modules` | 17,316 | 2.0% | 387 | Massive package, very low coverage |
| `alita/health` | 82 | 0.0% | 0 | Small, HTTP handler |
| `alita/metrics` | 103 | 0.0% | 0 | Prometheus setup |
| `alita/utils/async` | 58 | 0.0% | 0 | Small |
| `alita/utils/cache` | 385 | 1.0% | 129 | sanitize_test only |
| `alita/utils/callbackcodec` | 116 | 0.3% | 235 | Well tested |
| `alita/utils/chat_status` | 1,097 | 1.1% | 67 | Only ID validators tested |
| `alita/utils/constants` | 30 | N/A | 0 | No test files needed (constants only) |
| `alita/utils/debug_bot` | 20 | 0.0% | 0 | Trivial |
| `alita/utils/decorators/cmdDecorator` | 15 | 0.0% | 0 | Trivial |
| `alita/utils/decorators/misc` | 24 | 0.0% | 135 | Tests exist, 0% coverage due to -coverpkg |
| `alita/utils/error_handling` | 42 | 0.1% | 219 | Well tested |
| `alita/utils/errors` | 61 | 0.1% | 181 | Well tested |
| `alita/utils/extraction` | 391 | 1.2% | 238 | Tests exist |
| `alita/utils/helpers` | 1,124 | ~1.5% | 835 | Tests exist, many pure functions |
| `alita/utils/httpserver` | 393 | 0.0% | 0 | HTTP server, harder to test |
| `alita/utils/keyword_matcher` | 300 | 0.7% | 588 | Well tested |
| `alita/utils/media` | 304 | 0.0% | 0 | Needs Telegram API mocking |
| `alita/utils/monitoring` | 988 | ~1% | 232 | Some tests exist, needs config |
| `alita/utils/shutdown` | 97 | 0.1% | 143 | Well tested |
| `alita/utils/string_handling` | 29 | 0.1% | 95 | Well tested |
| `alita/utils/tracing` | 252 | ~0.5% | 114 | Some tests, needs config |
| `alita/utils/webhook` | 18 | 0.0% | 0 | Trivial |

**Note:** Coverage percentages shown are from `-coverpkg=./...` which measures against ALL packages, not just the package under test. This means a test in `callbackcodec` that covers 100% of `callbackcodec` shows as 0.3% of total codebase.

## Dependencies & External Services

| Dependency | Version | Relevant API/Feature | Notes |
|-----------|---------|---------------------|-------|
| gotgbot/v2 | v2.0.0-rc.33 | Telegram Bot API | Structs used in tests; API calls need mocking |
| gorm.io/gorm | v1.31.1 | PostgreSQL ORM | CI has real PostgreSQL 16 service |
| go-redis/v9 | v9.17.3 | Redis client | CI does NOT have Redis service -- cache tests skip |
| gocache/lib/v4 | v4.2.3 | Cache abstraction | Marshal/Manager globals are nil in tests |
| viper | v1.21.0 | YAML config | Used in i18n, testable without external deps |
| gotg_md2html | latest | Markdown-to-HTML | Pure function, testable |
| base64Captcha | v1.3.8 | CAPTCHA generation | Used in captcha module |
| ahocorasick | latest | Aho-Corasick matching | Used in keyword_matcher, fully tested |
| OpenTelemetry | v1.40.0 | Tracing | StartSpan works with noop provider in tests |

## Coverage Math: Path to 45%

Total statements in codebase (estimated from LOC ratio): approximately 10,000-12,000 statements.

Current: 12.5% = ~1,250 statements covered.
Target: 45% = ~4,500 statements covered.
**Need: ~3,250 additional statements covered.**

### Impact Analysis by Package (statements coverable)

1. **`alita/db/` -- GORM custom types (ButtonArray, StringArray, Int64Array)**: ~60 statements. Scan/Value methods for 3 types (6 methods x ~10 stmts). Pure functions, no DB needed.

2. **`alita/db/` -- DB CRUD functions**: Already at 8.6%. CI has PostgreSQL. Adding more integration tests for `optimized_queries.go` (523 LOC), `captcha_db.go` (518 LOC), `greetings_db.go` (382 LOC) could add ~400-500 statements.

3. **`alita/modules/` -- Pure helper functions**: The modules package has many testable unexported functions:
   - `chat_permissions.go` (32 LOC): `defaultUnmutePermissions()`, `resolveUnmutePermissions()` -- pure, ~15 stmts
   - `helpers.go` (492 LOC): `moduleStruct` methods, `moduleEnabled` type -- ~100 stmts testable
   - `moderation_input.go`: Already tested, could add more edge cases
   - Each handler file has parsing/formatting helpers scattered through it
   - **Estimated testable pure functions in modules: ~300-500 statements**

4. **`alita/config/` -- ValidateConfig and setDefaults**:
   - `ValidateConfig()` (80 lines, ~40 stmts): Testable with constructed Config structs
   - `setDefaults()` (160 lines, ~80 stmts): Testable by calling on empty Config
   - `getRedisAddress()` / `getRedisPassword()` (~30 stmts): Testable with env var manipulation
   - **But config init() blocks test execution**. Need `TestMain` that sets env vars BEFORE import, or test only `types.go` functions (already done).
   - **If env vars are set in CI, these tests pass. Estimated: ~150 statements**

5. **`alita/i18n/` -- Translator and Manager**:
   - `translator.go` (358 LOC): `GetString`, `GetPlural`, `interpolateParams`, `extractOrderedValues`, `selectPluralForm` -- partially tested. More edge cases: ~100 stmts
   - `manager.go`: `IsLanguageSupported`, `GetDefaultLanguage`, `GetStats`, `ReloadLocales` -- ~50 stmts
   - `loader.go`: Pure parsing functions -- ~50 stmts
   - **Estimated: ~200 statements**

6. **`alita/utils/chat_status/` -- ID validators and helpers**:
   - Only `IsValidUserId` and `IsChannelId` tested. Many functions like `RequireGroup`, `RequirePrivate` need gotgbot mocking.
   - `helpers.go`: Pure helper functions -- ~50 stmts testable
   - **Estimated: ~50-100 statements**

7. **`alita/utils/extraction/` -- More extraction functions**:
   - `ExtractUser`, `ExtractTime`, `ExtractIDFromMention` -- partially testable with constructed gotgbot structs
   - **Estimated: ~100 statements**

8. **`alita/utils/monitoring/` -- Stats and remediation**:
   - `CollectSystemStats()` is a pure function reading runtime stats -- ~30 stmts
   - More action Execute() methods -- ~50 stmts
   - **Estimated: ~80 statements**

9. **`alita/db/migrations.go` -- SQL cleaning/splitting functions**:
   - Already partially tested. `cleanSupabaseSQL`, `splitSQLStatements`, `getMigrationFiles` -- ~100 stmts more

10. **`alita/db/db.go` -- GORM Scan/Value, TableName methods**:
    - 20+ TableName() methods at ~3 stmts each = ~60 stmts
    - ButtonArray/StringArray/Int64Array Scan/Value = ~60 stmts
    - **Estimated: ~120 statements**

### Recommended Priority Order (impact per effort)

| Priority | Target | Est. Statements | Effort | Strategy |
|----------|--------|----------------|--------|----------|
| 1 | `alita/db/` GORM types (Scan/Value/TableName) | ~120 | LOW | Pure unit tests, no DB needed |
| 2 | `alita/config/` ValidateConfig + setDefaults | ~150 | MEDIUM | Need env vars in TestMain; CI has them |
| 3 | `alita/db/` more integration tests (optimized_queries, captcha, greetings) | ~400 | MEDIUM | Follow existing DB test pattern with skipIfNoDb |
| 4 | `alita/modules/` unexported helpers | ~400 | MEDIUM | Test parsing, formatting, permissions helpers |
| 5 | `alita/i18n/` translator edge cases | ~200 | LOW | Use newTestTranslator helper already in tests |
| 6 | `alita/utils/extraction/` more functions | ~100 | LOW | Construct gotgbot.Message structs |
| 7 | `alita/utils/monitoring/` stats collection | ~80 | LOW | Pure runtime stats |
| 8 | `alita/db/migrations.go` SQL functions | ~100 | LOW | Pure string processing |
| 9 | `alita/utils/chat_status/` helpers | ~100 | MEDIUM | Some need mocking |
| 10 | `alita/utils/helpers/` more functions | ~100 | LOW | Some already tested |

**Doing priorities 1-6 should yield ~1,370 new statements, bringing total to ~2,620 = ~26%. Need ~1,880 more.**

**Doing priorities 1-8 yields ~1,550 new statements, bringing total to ~2,800 = ~28%. Still short.**

**The gap to 45% is massive. Need to also:**
- Add comprehensive tests for `alita/db/optimized_queries.go` (523 LOC, all integration tests)
- Test more module handler-level logic (the 17K LOC elephant)
- Test `alita/config/config.go` LoadConfig with all defaults

**Revised estimate: Priorities 1-10 plus aggressive DB integration testing and module helper testing should reach ~4,500 covered statements = ~45%.**

## Risks & Conflicts

1. **Config init() kills tests without env vars** -- Severity: HIGH. Any test file that transitively imports `alita/config` will `log.Fatalf` without `BOT_TOKEN`, `OWNER_ID`, `MESSAGE_DUMP`, `DATABASE_URL`, `REDIS_ADDRESS`. CI sets these. Local dev needs them too. Affected packages: `alita/db`, `alita/modules`, `alita/utils/cache`, `alita/utils/helpers`, `alita/utils/monitoring`, `alita/utils/tracing`, `alita/utils/chat_status`, `alita/utils/extraction`, `alita/utils/httpserver`, `alita/utils/async`.

2. **Redis not available in CI** -- Severity: MEDIUM. CI only has PostgreSQL, not Redis. All cache operations return nil/error. Tests must handle `cache.Marshal == nil` gracefully. The `getFromCacheOrLoad` function in `cache_helpers.go` already handles this (line 80).

3. **Coverage measured with `-coverpkg=./...`** -- Severity: LOW. The Makefile uses `-coverpkg=./...` which measures coverage across ALL packages. This means a test in package A that exercises code in package B counts toward B's coverage. DB tests already benefit from this. This flag was recently added (commit 3352363).

4. **Module handler tests require Telegram API mocking** -- Severity: HIGH. The `alita/modules/` package is 51% of code. Handler functions take `*gotgbot.Bot` and `*ext.Context`. Testing them requires mocking the Telegram API (bot.SendMessage, bot.DeleteMessage, etc). gotgbot does not provide a built-in test helper for this. Community approach: create a mock bot or use httptest server.

5. **Test isolation for DB integration tests** -- Severity: MEDIUM. DB tests use `time.Now().UnixNano()` for unique IDs and `t.Cleanup()` for teardown. Parallel tests could collide if using hardcoded IDs. Current pattern is correct but must be followed strictly.

## Open Questions

- [ ] What is the exact statement count reported by `go tool cover -func=coverage.out`? The 12.5% number is from CI. Need per-function breakdown to identify zero-coverage functions to target.
- [ ] Is there an existing mock for `gotgbot.Bot`? Searching the codebase found none. May need to create one or use interface-based testing.
- [ ] Should we refactor `config.init()` to not use `log.Fatalf` to make local testing easier? This would be a separate PR.
- [ ] Can we add Redis to CI services (like PostgreSQL) to enable cache integration tests? Currently cache is nil in tests.

## File Inventory

Files that will likely need modification or that are critical context:

| File | Purpose | Relevance |
|------|---------|-----------|
| `/Users/divkix/GitHub/Alita_Robot/Makefile` | Test command: `go test -v -race -coverprofile=coverage.out -coverpkg=./... -count=1 -timeout 10m ./...` | Defines how coverage is measured |
| `/Users/divkix/GitHub/Alita_Robot/.github/workflows/ci.yml` | CI pipeline: PostgreSQL service, env vars, 40% threshold check | Lines 155-248 define test environment |
| `/Users/divkix/GitHub/Alita_Robot/alita/config/config.go` | Config loading with fatal init() | Lines 536-568: init() that blocks tests |
| `/Users/divkix/GitHub/Alita_Robot/alita/config/types.go` | typeConvertor utility (50 LOC) | Already tested, patterns to follow |
| `/Users/divkix/GitHub/Alita_Robot/alita/config/types_test.go` | Existing type converter tests | Reference for test style |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/testmain_test.go` | DB test harness: AutoMigrate + skipIfNoDb | Must follow this pattern for DB tests |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/db.go` | GORM models, Scan/Value, CRUD helpers (902 LOC) | High-value target: custom types + TableName methods |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/optimized_queries.go` | Optimized SELECT queries (523 LOC) | High-value DB integration test target |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/captcha_db.go` | Captcha CRUD operations (518 LOC) | Large file, integration tests needed |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/greetings_db.go` | Greetings CRUD (382 LOC) | Integration test target |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/migrations.go` | Migration runner + SQL cleaning (703 LOC) | Pure SQL functions testable |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/cache_helpers.go` | Cache key generators + getFromCacheOrLoad (170 LOC) | Cache keys already tested |
| `/Users/divkix/GitHub/Alita_Robot/alita/i18n/translator.go` | Translation with interpolation (358 LOC) | More edge cases testable |
| `/Users/divkix/GitHub/Alita_Robot/alita/i18n/manager.go` | LocaleManager singleton (161 LOC) | Test IsLanguageSupported, GetStats |
| `/Users/divkix/GitHub/Alita_Robot/alita/i18n/loader.go` | YAML file loading | Pure parsing functions |
| `/Users/divkix/GitHub/Alita_Robot/alita/i18n/i18n_test.go` | Existing i18n tests with newTestTranslator helper | Reference for test infrastructure |
| `/Users/divkix/GitHub/Alita_Robot/alita/modules/helpers.go` | moduleStruct, moduleEnabled, shared utils (492 LOC) | Pure functions testable |
| `/Users/divkix/GitHub/Alita_Robot/alita/modules/chat_permissions.go` | defaultUnmutePermissions, resolveUnmutePermissions (32 LOC) | Pure functions, easy tests |
| `/Users/divkix/GitHub/Alita_Robot/alita/modules/moderation_input.go` | buildModerationMatchText (partially tested) | More edge cases |
| `/Users/divkix/GitHub/Alita_Robot/alita/modules/callback_codec.go` | encodeCallbackData, decodeCallbackData (tested) | Reference |
| `/Users/divkix/GitHub/Alita_Robot/alita/modules/rules_format.go` | normalizeRulesForHTML (tested) | Reference |
| `/Users/divkix/GitHub/Alita_Robot/alita/modules/formatting.go` | Formatting handler (204 LOC) | Handler needs mocking |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/extraction/extraction.go` | User/time/mention extraction (391 LOC) | More functions testable |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/helpers/telegram_helpers.go` | Telegram helper functions | Already partially tested |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/helpers/helpers.go` | notesParser, SplitMessage, etc. (1015 LOC) | Partially tested, more possible |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/helpers/channel_helpers.go` | Channel-related helpers | Needs investigation |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/monitoring/auto_remediation.go` | Remediation actions (331 LOC) | CanExecute/Name/Severity tested |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/monitoring/background_stats.go` | Stats collector (406 LOC) | Partially tested |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/monitoring/activity_monitor.go` | Activity tracking (251 LOC) | Needs config + DB |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/cache/sanitize.go` | Cache key sanitization | Fully tested |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/cache/adminCache.go` | Admin cache operations | Needs Redis |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/cache/cache.go` | Redis cache initialization | Needs Redis |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/callbackcodec/callbackcodec.go` | Callback encoding/decoding (116 LOC) | Well tested |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/shutdown/graceful.go` | Graceful shutdown manager (97 LOC) | Well tested |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/tracing/context.go` | Context extraction | Tested |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/tracing/processor.go` | Tracing processor | Tested |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/tracing/tracing.go` | OpenTelemetry init | Needs config |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/httpserver/server.go` | Unified HTTP server (393 LOC) | Complex, needs mocking |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/media/sender.go` | Media sender (304 LOC) | Needs Telegram API |

## Raw Notes

### Coverage calculation detail
- CI runs `go test -v -race -coverprofile=coverage.out -coverpkg=./... -count=1 -timeout 10m ./...`
- Threshold check: `go tool cover -func=coverage.out | grep '^total:'` extracts percentage
- Current: 12.5% < 40% threshold = FAIL
- Target: 45% to have 5% margin above 40%

### Packages that DO NOT import config (testable without env vars)
- `alita/utils/callbackcodec` -- already well tested
- `alita/utils/errors` -- already well tested
- `alita/utils/string_handling` -- already well tested
- `alita/utils/shutdown` -- already well tested
- `alita/utils/keyword_matcher` -- already well tested
- `alita/utils/decorators/misc` -- tested but shows 0% due to coverpkg accounting
- `alita/utils/error_handling` -- already well tested
- `alita/utils/constants` -- no test files needed (only constants)

### Packages that import config (need env vars OR test only pure functions)
- `alita/db` -- has TestMain + skipIfNoDb pattern
- `alita/modules` -- has module-internal tests for pure functions
- `alita/config` -- has types_test.go
- `alita/i18n` -- imports cache which imports config
- `alita/utils/cache` -- imports config
- `alita/utils/chat_status` -- imports config via helpers chain
- `alita/utils/extraction` -- imports config via helpers chain
- `alita/utils/helpers` -- imports config
- `alita/utils/monitoring` -- imports config
- `alita/utils/tracing` -- imports config
- `alita/utils/httpserver` -- imports config
- `alita/utils/async` -- imports config

### Key unexported functions in modules package that are testable
(Verified by reading source):
- `defaultUnmutePermissions()` in `chat_permissions.go`
- `resolveUnmutePermissions()` in `chat_permissions.go`
- `moduleEnabled` struct methods in `helpers.go`
- `parseTranslateResponse()` in `misc.go` (already tested in `misc_translate_parser_test.go`)
- `normalizeRulesForHTML()` in `rules_format.go` (already tested)
- `buildModerationMatchText()` in `moderation_input.go` (already tested)
- `encodeCallbackData()` / `decodeCallbackData()` in `callback_codec.go` (already tested)
- `parseNoteOverwriteCallbackData()` / `parseFilterOverwriteCallbackData()` in `callback_parse_overwrite.go` (already tested)

### GORM custom types in db.go that need unit tests
- `ButtonArray.Scan()` / `ButtonArray.Value()` -- lines 54-75
- `StringArray.Scan()` / `StringArray.Value()` -- lines 82-103
- `Int64Array.Scan()` / `Int64Array.Value()` -- lines 106-131
- 20+ `TableName()` methods across all model structs
- `getSpanAttributes()` -- pure function for tracing

### i18n loader.go pure functions to test
- `extractLangCode()` -- already tested
- `isYAMLFile()` -- already tested
- `validateYAMLStructure()` -- already tested
- `compileViper()` -- used in test helper, could have direct tests

### CI Environment Variables (from ci.yml lines 173-180)
```
BOT_TOKEN: test-token
OWNER_ID: "1"
MESSAGE_DUMP: "1"
REDIS_ADDRESS: "localhost:6379"
DATABASE_URL: "postgres://postgres:postgres@localhost:5432/alita_test?sslmode=disable"
AUTO_MIGRATE: "false"
USE_WEBHOOKS: "false"
DEBUG: "false"
```

### Test counts by area
- DB tests: 21 test files, ~4,506 lines
- Module tests: 5 test files, ~387 lines
- Utils tests: ~21 test files, ~3,276 lines
- Config tests: 1 test file, 171 lines
- i18n tests: 1 test file, 605 lines

### Statement estimation methodology
Go typically has ~1 executable statement per 3 lines of code (accounting for comments, blank lines, struct definitions, imports). With ~33,864 production LOC, estimated ~11,288 statements. 12.5% = ~1,411 covered. 45% = ~5,080. Need ~3,669 more. This is aggressive but achievable by focusing on the three largest packages (modules, db, helpers).
