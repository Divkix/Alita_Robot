# Research: Increase Test Coverage Across Alita Robot

**Date:** 2026-02-21
**Goal:** Audit the entire codebase for test coverage, identify untested packages, understand testability constraints, and produce a priority-ranked plan for coverage increase.
**Confidence:** HIGH -- every claim below was verified by reading actual source files and running `go test -cover`.

## Executive Summary

Total test coverage is **5.3%** (from `go tool cover -func`). The project has 29 Go packages; only 4 pass tests with measurable coverage (`callbackcodec` 95.3%, `errors` 94.7%, `string_handling` 100%, `keyword_matcher` 60.2%). Seven packages have test files but **FAIL** because importing `config` or `db` triggers `log.Fatal` in `init()` when `BOT_TOKEN` is missing. Fifteen packages have **zero test files**. The critical blocker is `alita/config/config.go:init()` which calls `log.Fatalf` during package init if env vars are missing -- this cascades through `db`, `cache`, `helpers`, `chat_status`, `i18n`, `tracing`, and `modules`. The existing test infrastructure uses only `testing` stdlib (no testify, no mocks, no `TestMain`). DB tests require a live PostgreSQL connection (CI provides one via services).

## Current Coverage Status

### Packages That PASS Tests

| Package | Coverage | Test File(s) | Notes |
|---------|----------|--------------|-------|
| `alita/utils/callbackcodec` | 95.3% | `callbackcodec_test.go` | Pure logic, no external deps |
| `alita/utils/errors` | 94.7% | `errors_test.go` | Pure logic, no external deps |
| `alita/utils/string_handling` | 100.0% | `string_handling_test.go` | Pure logic, no external deps |
| `alita/utils/keyword_matcher` | 60.2% | `matcher_test.go` | Pure logic; `cache.go` untested (40% gap) |

### Packages With Tests That FAIL (config/db init crash)

| Package | Test File(s) | Failure Reason |
|---------|--------------|----------------|
| `alita/config` | `types_test.go` | `config.go:init()` calls `log.Fatalf` when `BOT_TOKEN` missing |
| `alita/db` | `captcha_db_test.go`, `locks_db_test.go`, `antiflood_db_test.go`, `update_record_test.go` | `db.go:init()` imports config -> crash; also requires live PostgreSQL |
| `alita/i18n` | `i18n_test.go` | Imports config indirectly (tests pure functions but package init fails) |
| `alita/modules` | `moderation_input_test.go`, `misc_translate_parser_test.go`, `callback_parse_overwrite_test.go`, `rules_format_test.go` | Module files import db/config -> crash |
| `alita/utils/helpers` | `helpers_test.go` | `helpers.go` imports config + db |
| `alita/utils/cache` | `sanitize_test.go` | `cache.go` imports config |
| `alita/utils/chat_status` | `chat_status_test.go` | `chat_status.go` imports db -> config |
| `alita/utils/tracing` | `context_test.go`, `processor_test.go` | `tracing.go` imports config |

### Packages With ZERO Test Files

| Package | Source Files | LOC | Functions |
|---------|-------------|-----|-----------|
| `alita` (main.go loader) | 1 | ~200 | Module loader |
| `alita/health` | 1 | ~80 | 3 (checkDatabase, checkRedis, Handler) |
| `alita/metrics` | 1 | ~80 | 0 (only var declarations) |
| `alita/utils/async` | 1 | ~60 | 3 |
| `alita/utils/constants` | 1 | ~30 | 0 (only const declarations) |
| `alita/utils/debug_bot` | 1 | ~30 | 1 |
| `alita/utils/decorators/cmdDecorator` | 1 | ~15 | 1 (MultiCommand) |
| `alita/utils/decorators/misc` | 1 | ~25 | 2 (addToArray, AddCmdToDisableable) |
| `alita/utils/error_handling` | 1 | ~42 | 3 (HandleErr, RecoverFromPanic, CaptureError) |
| `alita/utils/extraction` | 1 | ~350 | 8 |
| `alita/utils/httpserver` | 1 | ~100 | ~3 |
| `alita/utils/media` | 1 | ~300 | 12 |
| `alita/utils/monitoring` | 3 | ~700 | ~40 |
| `alita/utils/shutdown` | 1 | ~100 | 5 |
| `alita/utils/webhook` | 1 | ~50 | ~2 |
| `scripts/generate_docs` | 3 | ~900 | ~20 |

## CI/CD Coverage Configuration

**Location:** `/Users/divkix/GitHub/Alita_Robot/.github/workflows/ci.yml`

### Test Job Configuration
- **Command:** `make test` which runs `go test -v -race -coverprofile=coverage.out -count=1 -timeout 10m ./...`
- **Coverage output:** `coverage.out` is generated and uploaded as an artifact
- **Coverage reporting:** Artifact upload only (`actions/upload-artifact@v6`); **NO Codecov, Coveralls, or threshold enforcement**
- **Services:** PostgreSQL 16 (for DB tests) -- no Redis service (cache tests skip)
- **Environment:** `BOT_TOKEN=test-token`, `OWNER_ID=1`, `MESSAGE_DUMP=1`, `DATABASE_URL` set

### Key Finding
CI sets `BOT_TOKEN=test-token` which means the `config.init()` crash does NOT occur in CI. Tests that fail locally (config init crash) likely **pass in CI** because the env vars are set. This is confirmed by the CI job structure -- it expects `make test` to succeed as a gate for the pipeline.

## Existing Test Patterns and Conventions

### Pattern 1: Table-Driven Tests with Subtests
- **Location:** All test files
- **How it works:** `tests := []struct{...}` with `t.Run(tc.name, func(t *testing.T) { t.Parallel(); ... })`
- **Relevant:** This is the standard pattern; all new tests must follow it

### Pattern 2: `t.Parallel()` on Every Test and Subtest
- **Location:** All test files
- **How it works:** Every test function and subtest calls `t.Parallel()`
- **Relevant:** Must be continued for all new tests

### Pattern 3: No TestMain
- **Location:** Nowhere
- **How it works:** No `TestMain()` exists. DB tests use `DB.AutoMigrate()` inline
- **Relevant:** A `TestMain()` could be added to DB package for one-time setup

### Pattern 4: No External Test Dependencies
- **Location:** All test files
- **How it works:** Only stdlib `testing` package. No testify, no gomock, no similar
- **Relevant:** New tests should maintain this convention unless explicitly decided otherwise

### Pattern 5: DB Tests Use Live PostgreSQL
- **Location:** `alita/db/*_test.go`
- **How it works:** Tests call `DB.AutoMigrate(&Model{})` directly and use `time.Now().UnixNano()` for unique IDs. Cleanup via `t.Cleanup()`
- **Relevant:** DB tests only run when PostgreSQL is available (CI has it, local may not)

### Pattern 6: Pure Function Isolation
- **Location:** `moderation_input_test.go`, `misc_translate_parser_test.go`, `rules_format_test.go`
- **How it works:** Tests target unexported pure functions (`buildModerationMatchText`, `parseTranslateResponse`, `normalizeRulesForHTML`) that do not touch external deps
- **Relevant:** Best strategy for modules -- test pure logic functions, not Telegram handler methods

### Pattern 7: Concurrency Tests
- **Location:** `captcha_db_test.go`, `locks_db_test.go`, `keyword_matcher_test.go`
- **How it works:** Spawn N goroutines, use `sync.WaitGroup`, verify exactly-once semantics
- **Relevant:** Important for DB operations and cache

## Testability Assessment

### Tier 1: Trivially Testable (Pure Functions, No Dependencies)

These packages have zero external dependencies and can be tested without any setup:

| Package | Functions to Test | Difficulty |
|---------|-------------------|------------|
| `alita/utils/error_handling` | `HandleErr`, `RecoverFromPanic`, `CaptureError` | TRIVIAL |
| `alita/utils/decorators/misc` | `addToArray`, `AddCmdToDisableable` | TRIVIAL |
| `alita/utils/constants` | Constants only -- no testable logic | SKIP |
| `alita/utils/shutdown` | `NewManager`, `RegisterHandler`, `executeHandler` (panic recovery) | EASY |
| `alita/utils/monitoring` (remediation actions) | `GCAction.CanExecute`, `MemoryCleanupAction.CanExecute`, `LogWarningAction.CanExecute` | EASY |

### Tier 2: Testable with Struct Mocking (gotgbot types)

These functions accept gotgbot structs as parameters. Since gotgbot types are plain structs (not interfaces), tests can construct them directly:

| Package | Functions to Test | Approach |
|---------|-------------------|----------|
| `alita/utils/helpers` (more) | `Shtml`, `Smarkdown`, `GetMessageLinkFromMessageId`, `InlineKeyboardMarkupToTgmd2htmlButtonV2`, `MakeLanguageKeyboard`, `GetLangFormat`, `ExtractJoinLeftStatusChange`, `ExtractAdminUpdateStatusChange`, `setRawText` | Construct gotgbot structs in tests |
| `alita/utils/helpers` (telegram) | `DeleteMessageWithErrorHandling`, `SendMessageWithErrorHandling` | Would need Bot mock/interface |
| `alita/modules` (pure funcs) | `buildModerationMatchText` (already tested), `parseTranslateResponse` (already tested), `normalizeRulesForHTML` (already tested), `parseNoteOverwriteCallbackData` (already tested) | Already done |
| `alita/modules` (more pure funcs) | `encodeCallbackData` in `callback_codec.go` | Construct maps |

### Tier 3: Testable with DB Setup (Requires PostgreSQL)

| Package | Functions to Test | Approach |
|---------|-------------------|----------|
| `alita/db/*_db.go` (16 untested files) | CRUD operations, cache invalidation | Same pattern as existing `captcha_db_test.go` |
| `alita/db/cache_helpers.go` | `getFromCacheOrLoad`, cache key generators | DB + optional Redis |
| `alita/db/optimized_queries.go` | `GetLockStatus`, `GetChatLocksOptimized` | DB required |
| `alita/db/migrations.go` | `RunMigrations`, SQL cleaning functions | DB required |

### Tier 4: Hard to Test (Telegram Bot API, Infrastructure)

| Package | Functions | Why Hard |
|---------|-----------|----------|
| `alita/modules/*` (handler methods) | 250+ handler functions | Require `gotgbot.Bot`, `ext.Context` with real API behavior |
| `alita/utils/chat_status` | 25+ permission check functions | Call Telegram API (`GetChatMember`) |
| `alita/utils/extraction` | `ExtractChat`, `ExtractUser`, `ExtractTime` | Call Telegram API |
| `alita/utils/media` | `Send`, `SendNote`, `SendFilter`, `SendGreeting` | Call Telegram API |
| `alita/utils/cache/cache.go` | `InitCache`, `TracedGet/Set/Delete` | Requires live Redis |
| `alita/health` | `checkDatabase`, `checkRedis`, `Handler` | Requires live DB + Redis |
| `alita/utils/httpserver` | HTTP server setup | Integration test territory |
| `alita/utils/webhook` | Webhook setup | Requires Bot + HTTP |

## Critical Blocker: Config init() Fatal

**File:** `/Users/divkix/GitHub/Alita_Robot/alita/config/config.go:536-554`

```go
func init() {
    cfg, err := LoadConfig()
    if err != nil {
        log.Fatalf("[Config] Failed to load configuration: %v", err)
    }
    AppConfig = cfg
}
```

This `log.Fatalf` kills the test process when `BOT_TOKEN` is not set. The dependency chain:

```
config.init() -> log.Fatalf (if BOT_TOKEN missing)
  |
  +-- db.init() imports config -> also crashes
  |     |
  |     +-- chat_status imports db
  |     +-- helpers imports db + config
  |     +-- extraction imports db
  |     +-- modules import db
  |     +-- monitoring imports db
  |     +-- media imports db
  |     +-- health imports config + db
  |
  +-- cache imports config
  +-- tracing imports config
  +-- async imports config
```

**Impact:** Any package that imports `config`, `db`, or `cache` (directly or transitively) will crash locally without env vars. In CI, `BOT_TOKEN=test-token` is set, so this is not a CI problem.

**DB init() also crashes independently** (`/Users/divkix/GitHub/Alita_Robot/alita/db/db.go:659-700`): it tries to connect to PostgreSQL via `config.AppConfig.DatabaseURL`. If the database is unreachable, it retries 5 times then crashes.

## Existing Test Infrastructure

### What Exists
- **Stdlib `testing` only** -- no third-party test frameworks
- **No `TestMain()`** in any package
- **No mock interfaces** -- only 1 interface in entire codebase (`RemediationAction` in monitoring)
- **No test fixtures or test data files**
- **No test build tags** (e.g., `//go:build integration`)
- **CI PostgreSQL service** for DB integration tests
- **No Redis service in CI** -- cache tests run with `cache.Marshal == nil` checks

### What is Missing
- `TestMain()` for DB package setup/teardown
- Interface abstractions for Telegram Bot API (would enable mocking)
- Build tags to separate unit tests from integration tests
- Coverage threshold enforcement in CI
- Codecov or similar coverage tracking service

## Risks & Conflicts

1. **Config init() fatal crash -- SEVERITY: HIGH** -- The `log.Fatalf` in `config.init()` kills any test process that transitively imports config without env vars. This blocks 8 packages from running tests locally. Fix options: (a) guard `init()` with a test sentinel, (b) use a `TestMain` that sets env vars, (c) refactor to lazy initialization. Option (c) is a significant refactor. Option (b) is the least invasive -- CI already does this.

2. **DB init() PostgreSQL connection requirement -- SEVERITY: HIGH** -- `db.init()` tries to connect to PostgreSQL with retry. Tests in the `db` package and everything importing it require a running PostgreSQL instance. CI has this via services. Local dev requires manual setup or Docker.

3. **No interface abstraction for gotgbot.Bot -- SEVERITY: MEDIUM** -- The `gotgbot.Bot` type is a concrete struct with HTTP-calling methods. All handler functions, permission checks, and extraction functions take `*gotgbot.Bot` as a parameter. Without an interface wrapper, these functions cannot be unit tested with mocks. Creating a `BotAPI` interface would be a significant refactor touching 100+ files.

4. **Global state in packages -- SEVERITY: MEDIUM** -- Several packages use package-level `var` for singletons: `config.AppConfig`, `db.DB`, `cache.Marshal`, `cache.Manager`. Tests that modify these affect other tests. No `t.Cleanup` resets global state.

5. **Module handler methods use value receivers on unnamed struct -- SEVERITY: LOW** -- Handlers like `(moduleStruct) echomsg(...)` cannot be tested without constructing the full module struct context. However, the pattern of extracting pure logic into testable functions (as done with `parseTranslateResponse`, `buildModerationMatchText`) is the right approach.

## Priority Ranking

### Priority 1: Fix Locally-Failing Tests (High Impact, Low Effort)

The 8 packages with existing test files that crash locally need to pass. The fix is environmental (set env vars for local test runs) or structural (ensure `init()` does not crash during tests).

**Packages:** `config`, `db`, `i18n`, `modules`, `helpers`, `cache`, `chat_status`, `tracing`
**Estimated coverage gain:** ~15-20% (these tests already exist and test real logic)

### Priority 2: Test Pure Utility Functions (High Value, Low Effort)

| Package | Target Functions | Estimated Effort |
|---------|-----------------|------------------|
| `alita/utils/error_handling` | All 3 functions | 30 min |
| `alita/utils/decorators/misc` | `addToArray`, `AddCmdToDisableable` | 15 min |
| `alita/utils/shutdown` | `NewManager`, `RegisterHandler`, `executeHandler` | 45 min |
| `alita/utils/keyword_matcher` (cache.go) | `NewCache`, `GetOrCreateMatcher`, `CleanupExpired`, `patternsEqual` | 45 min |
| `alita/utils/helpers` (more pure funcs) | `Shtml`, `Smarkdown`, `GetMessageLinkFromMessageId`, `GetLangFormat`, `InlineKeyboardMarkupToTgmd2htmlButtonV2`, `ExtractJoinLeftStatusChange`, `ExtractAdminUpdateStatusChange`, `setRawText` | 2 hrs |
| `alita/i18n` (more funcs) | `Translator.Get`, `Translator.GetPlural`, `LocaleManager` methods | 2 hrs |
| `alita/modules` (more pure funcs) | `encodeCallbackData` from callback_codec.go, more formatting funcs | 1 hr |

**Estimated coverage gain:** ~10-15%

### Priority 3: DB CRUD Tests (High Value, Medium Effort)

All 16 untested `*_db.go` files follow the same pattern. Tests require PostgreSQL.

| File | Functions | Priority |
|------|-----------|----------|
| `greetings_db.go` | 15 CRUD functions | HIGH (most complex module) |
| `warns_db.go` | 13 CRUD functions | HIGH (critical for moderation) |
| `notes_db.go` | 11 CRUD functions | HIGH (heavily used) |
| `filters_db.go` | 7 CRUD functions | HIGH (heavily used) |
| `disable_db.go` | 9 CRUD functions | MEDIUM |
| `connections_db.go` | 8 CRUD functions | MEDIUM |
| `user_db.go` | 8 CRUD functions | MEDIUM |
| `devs_db.go` | 7 CRUD functions | MEDIUM |
| `blacklists_db.go` | 6 CRUD functions | MEDIUM |
| `channels_db.go` | 6 CRUD functions | MEDIUM |
| `chats_db.go` | 6 CRUD functions | MEDIUM |
| `rules_db.go` | 6 CRUD functions | LOW |
| `lang_db.go` | 5 CRUD functions | LOW |
| `reports_db.go` | 7 CRUD functions | LOW |
| `pin_db.go` | 4 CRUD functions | LOW |
| `admin_db.go` | 3 CRUD functions | LOW |

**Estimated coverage gain:** ~20-25%

### Priority 4: Monitoring/Infrastructure Tests (Medium Value, Medium Effort)

| Package | Approach |
|---------|----------|
| `alita/utils/monitoring` | Test `CanExecute` methods on remediation actions, `NewManager`, metric collection |
| `alita/metrics` | Verify Prometheus metric registration (no logic to test) |
| `alita/utils/async` | Test `AsyncProcessor` lifecycle |

**Estimated coverage gain:** ~5%

### Priority 5: Integration-Level Tests (Lower Priority)

Handler methods in `alita/modules/*` (250+ functions across 32 files, 17,590 LOC) would require either:
- A `BotAPI` interface + mock implementation (major refactor)
- Integration tests with a real Telegram Bot Token (CI secrets)
- Extracting more pure logic from handlers into testable functions

This is the largest surface area but the hardest to test.

## External Dependencies and Mocking Strategy

| Dependency | Version | Used For | Mocking Feasibility |
|-----------|---------|----------|---------------------|
| `gotgbot/v2` | v2.0.0-rc.33 | Telegram Bot API | Concrete struct, no interface. Would need wrapper interface. gotgbot types (Message, Chat, User) ARE constructable as plain structs |
| `gorm.io/gorm` | v1.31.1 | PostgreSQL ORM | Use real DB in tests (existing pattern). Alternative: sqlmock |
| `go-redis/v9` | v9.17.3 | Redis cache | Use miniredis for tests, or skip cache (existing pattern uses nil checks) |
| `gocache/lib/v4` | v4.2.3 | Cache marshaling | Wraps Redis; test via miniredis or nil-guard pattern |
| `opentelemetry` | v1.40.0 | Tracing | NoopTracerProvider for tests (already works) |
| `logrus` | v1.9.4 | Logging | Can capture with test hooks; `error_handling` tests can verify log output |
| `ahocorasick` | latest | Keyword matching | Already tested via KeywordMatcher wrapper |
| `gotg_md2html` | latest | Markdown to HTML | Already tested indirectly in helpers_test.go |

### Recommended Mocking Strategy

1. **No new test framework dependencies.** Keep using stdlib `testing`. The existing pattern is clean and sufficient.
2. **For DB tests:** Continue using live PostgreSQL (CI has it). Add `TestMain()` to `alita/db/` for one-time setup.
3. **For cache tests:** Use nil-guard pattern (`if cache.Marshal != nil`) already established.
4. **For gotgbot types:** Construct structs directly (`&gotgbot.Message{Text: "...", Chat: gotgbot.Chat{...}}`). These are plain data structs.
5. **For Telegram API calls:** Extract pure logic into separate functions (existing pattern: `parseTranslateResponse`, `buildModerationMatchText`). Test the pure functions. Leave handler orchestration for integration tests.
6. **For tracing:** Use `otel.SetTracerProvider(trace.NewNoopTracerProvider())` which is already implicitly used when no OTLP endpoint is configured.

## Open Questions

- [ ] Should we add Codecov or similar coverage tracking to CI? Currently coverage.out is uploaded as artifact but not tracked over time.
- [ ] Is there a target coverage percentage? (Industry standard for Go projects: 60-80%)
- [ ] Should we add a `//go:build integration` tag to DB tests so `go test ./...` passes without PostgreSQL?
- [ ] Should we invest in a `BotAPI` interface to enable mocking Telegram API calls, or continue the "extract pure logic" approach?

## File Inventory

### Files Critical for Testing (Source)

| File | Purpose | LOC | Relevance |
|------|---------|-----|-----------|
| `/Users/divkix/GitHub/Alita_Robot/alita/config/config.go` | Config loading + fatal init() | ~600 | ROOT CAUSE of test failures; init() must not crash |
| `/Users/divkix/GitHub/Alita_Robot/alita/config/types.go` | Type converters (pure functions) | 50 | Already tested, 100% testable |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/db.go` | GORM models + DB init + generic CRUD | 902 | Core infrastructure; init() connects to PG |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/cache_helpers.go` | Cache key generators + singleflight loader | 170 | Key generators are pure, loader needs cache |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/optimized_queries.go` | Optimized SELECT queries | 523 | Needs DB connection |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/captcha_db.go` | Captcha CRUD + atomic operations | 518 | Partially tested (2 tests) |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/greetings_db.go` | Greeting settings CRUD | 380 | 0 tests, 15 functions |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/warns_db.go` | Warn system CRUD | 265 | 0 tests, 13 functions |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/helpers/helpers.go` | 30+ helper functions | ~1000 | Partially tested; many pure functions remain |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/helpers/channel_helpers.go` | `IsChannelID` (1 pure function) | 7 | Already tested |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/helpers/telegram_helpers.go` | Message send/delete wrappers | ~50 | Needs Bot mock or interface |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/chat_status/chat_status.go` | Permission checks | ~1050 | 2 pure functions tested; 25+ need Bot mock |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/extraction/extraction.go` | User/chat extraction | ~350 | All functions need Bot/Context |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/error_handling/error_handling.go` | Error handlers + panic recovery | 42 | 0 tests, fully testable |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/shutdown/graceful.go` | Shutdown manager | ~100 | 0 tests, testable (no external deps beyond error_handling) |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/keyword_matcher/cache.go` | Per-chat matcher cache | 125 | 0 tests, accounts for 40% gap in package |
| `/Users/divkix/GitHub/Alita_Robot/alita/i18n/i18n.go` | Locale manager | ~350 | Test file exists but fails due to config |
| `/Users/divkix/GitHub/Alita_Robot/alita/i18n/translator.go` | Translation logic | ~200 | Pure logic, testable |
| `/Users/divkix/GitHub/Alita_Robot/alita/i18n/loader.go` | YAML file loading | ~150 | Pure logic, partially tested in i18n_test.go |
| `/Users/divkix/GitHub/Alita_Robot/alita/i18n/errors.go` | i18n error types | ~50 | Tested in i18n_test.go |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/decorators/misc/handler_vars.go` | Command array helpers | 25 | 0 tests, trivially testable |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/monitoring/auto_remediation.go` | Auto-remediation actions | ~330 | `CanExecute` methods are pure functions testable without infra |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/monitoring/background_stats.go` | System metrics collector | ~390 | Lifecycle methods testable, stat collection needs runtime |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/monitoring/activity_monitor.go` | Activity tracking | ~160 | Needs DB |
| `/Users/divkix/GitHub/Alita_Robot/alita/modules/captcha.go` | Captcha module handlers | 1982 | Largest module, 23 functions, 0 handler tests |
| `/Users/divkix/GitHub/Alita_Robot/alita/modules/bans.go` | Ban module handlers | 1480 | 13 functions, 0 handler tests |
| `/Users/divkix/GitHub/Alita_Robot/alita/modules/greetings.go` | Greeting module handlers | 1194 | 22 functions, 0 handler tests |
| `/Users/divkix/GitHub/Alita_Robot/alita/modules/notes.go` | Notes module handlers | 1000 | 12 functions, 0 handler tests |

### Files Critical for Testing (Existing Test Files)

| File | Package | Tests | Status |
|------|---------|-------|--------|
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/callbackcodec/callbackcodec_test.go` | callbackcodec | 14 | PASS |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/errors/errors_test.go` | errors | 8 | PASS |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/string_handling/string_handling_test.go` | string_handling | 3 | PASS |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/keyword_matcher/matcher_test.go` | keyword_matcher | 8 | PASS |
| `/Users/divkix/GitHub/Alita_Robot/alita/config/types_test.go` | config | 5 | FAIL (init) |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/captcha_db_test.go` | db | 2 | FAIL (init) |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/locks_db_test.go` | db | 4 | FAIL (init) |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/antiflood_db_test.go` | db | 3 | FAIL (init) |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/update_record_test.go` | db | 4 | FAIL (init) |
| `/Users/divkix/GitHub/Alita_Robot/alita/i18n/i18n_test.go` | i18n | 10 | FAIL (init) |
| `/Users/divkix/GitHub/Alita_Robot/alita/modules/moderation_input_test.go` | modules | 2 | FAIL (init) |
| `/Users/divkix/GitHub/Alita_Robot/alita/modules/misc_translate_parser_test.go` | modules | 4 | FAIL (init) |
| `/Users/divkix/GitHub/Alita_Robot/alita/modules/callback_parse_overwrite_test.go` | modules | 5 | FAIL (init) |
| `/Users/divkix/GitHub/Alita_Robot/alita/modules/rules_format_test.go` | modules | 7 | FAIL (init) |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/helpers/helpers_test.go` | helpers | 28 | FAIL (init) |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/cache/sanitize_test.go` | cache | 2 | FAIL (init) |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/chat_status/chat_status_test.go` | chat_status | 2 | FAIL (init) |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/tracing/context_test.go` | tracing | 6 | FAIL (init) |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/tracing/processor_test.go` | tracing | 4 | FAIL (init) |

## Raw Notes

### Config init() dependency chain (verified by reading imports)
```
config.init() log.Fatalf
  -> db.init() imports config + connects PG
    -> chat_status imports db
    -> helpers imports config + db
    -> extraction imports db + i18n + chat_status
    -> all 32 module files import db
    -> monitoring imports db
    -> media imports db
    -> health imports config + db + cache
  -> cache imports config
  -> tracing imports config
  -> async imports config
  -> httpserver imports db + config (indirect)
```

### Lines of code breakdown
- Total Go source (non-test): ~25,000 LOC
- Total Go test: ~2,500 LOC (~10% of source)
- Largest untested files: `captcha.go` (1982 LOC), `bans.go` (1480 LOC), `greetings.go` (1194 LOC), `notes.go` (1000 LOC)
- DB layer: 6,185 LOC total (only ~540 LOC tested)
- Module layer: 17,590 LOC total (only ~400 LOC tested via pure function extraction)

### Functions already tested (complete list by test file)

**callbackcodec_test.go (14 tests):** `Encode`, `Decode`, `EncodeOrFallback`, `Decoded.Field` -- round-trip, invalid namespace, oversized payload, malformed decode, nil receiver, empty fields, URL special chars

**errors_test.go (8 tests):** `Wrap`, `Wrapf` -- nil error, non-nil, formatted message, Error() format, Unwrap chain, double wrap, empty message, file path truncation

**string_handling_test.go (3 test functions, ~18 subtests):** `FindInStringSlice`, `FindInInt64Slice`, `IsDuplicateInStringSlice` -- nil/empty slices, boundary values, channel IDs, MaxInt64/MinInt64

**keyword_matcher_test.go (8 tests):** `NewKeywordMatcher`, `FindMatches`, `HasMatch`, `GetPatterns` -- empty/nil patterns, case insensitivity, overlapping matches, concurrent access, special characters, position verification, defensive copy

**helpers_test.go (28 tests):** `IsChannelID`, `SplitMessage` (5 variants), `MentionHtml`, `MentionUrl` (2 variants), `HtmlEscape` (4 variants), `GetFullName` (2 variants), `BuildKeyboard` (4 variants), `ConvertButtonV2ToDbButton` (2 variants), `RevertButtons` (3 variants), `ChunkKeyboardSlices` (4 variants), `ReverseHTML2MD` (4 variants), `IsExpectedTelegramError` (3 variants), `notesParser` (7 variants)

**chat_status_test.go (2 test functions, ~14 subtests):** `IsValidUserId`, `IsChannelId` -- positive IDs, zero, negative, channel IDs, boundary values, Telegram system IDs

**sanitize_test.go (2 test functions, ~18 subtests):** `SanitizeCacheKey`, `CacheKeySegmentCount` -- 3-segment, 4-segment, 6-segment, empty, no colons, trailing/leading colons

**tracing context_test.go (6 tests):** `ExtractContext` -- valid context, nil ext.Context, nil Data, missing key, wrong type, empty Data, cancelled context

**tracing processor_test.go (4 tests):** `injectTraceContext` -- injects context, preserves existing, initializes nil Data, skips webhook context

**config types_test.go (5 test functions, ~30 subtests):** `typeConvertor.Bool`, `.Int`, `.Int64`, `.Float64`, `.StringArray` -- true/false variants, overflow, NaN/Inf, whitespace, consecutive commas

**i18n_test.go (10 test functions):** `extractLangCode`, `isYAMLFile`, `validateYAMLStructure`, `I18nError.Error()`, `.Unwrap()`, `NewI18nError`, predefined errors distinct, errors chain, `extractOrderedValues`, `selectPluralForm`

**modules moderation_input_test.go (2 tests):** `buildModerationMatchText` -- nil message, combined text+caption+entities with dedup

**modules misc_translate_parser_test.go (4 tests):** `parseTranslateResponse` -- valid payload, malformed JSON, empty payload, unexpected shape

**modules callback_parse_overwrite_test.go (5 tests):** `parseNoteOverwriteCallbackData`, `parseFilterOverwriteCallbackData` -- tokenized, legacy, cancel

**modules rules_format_test.go (7 subtests):** `normalizeRulesForHTML` -- empty, whitespace, HTML passthrough, markdown conversion, angle brackets, HTML entities

**db captcha_db_test.go (2 tests):** `DeleteCaptchaAttemptByIDAtomic` concurrent single claim, captcha settings cache invalidation

**db locks_db_test.go (4 tests):** `UpdateLock` creates record, zero-value boolean, idempotent, concurrent creation

**db antiflood_db_test.go (3 tests):** `SetFloodMsgDel` zero-value boolean, `SetFlood` zero-value limit, creates record

**db update_record_test.go (4 tests):** `UpdateRecord` returns error on no match, `UpdateRecordWithZeroValues` returns error on no match, updates zero values, succeeds when rows affected

### DB files with zero tests (16 files, ~2800 LOC, ~140 functions)

| File | Functions | LOC |
|------|-----------|-----|
| `/Users/divkix/GitHub/Alita_Robot/alita/db/admin_db.go` | 3 | 65 |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/blacklists_db.go` | 6 | 106 |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/channels_db.go` | 6 | 148 |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/chats_db.go` | 6 | 187 |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/connections_db.go` | 8 | 119 |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/devs_db.go` | 7 | 230 |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/disable_db.go` | 9 | 137 |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/filters_db.go` | 7 | 145 |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/greetings_db.go` | 15 | 380 |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/lang_db.go` | 5 | 152 |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/notes_db.go` | 11 | 200 |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/pin_db.go` | 4 | 68 |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/reports_db.go` | 7 | 131 |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/rules_db.go` | 6 | 86 |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/user_db.go` | 8 | 200 |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/warns_db.go` | 13 | 265 |

RESEARCH_COMPLETE
