# Technical Design: Increase Test Coverage Across Alita Robot

**Date:** 2026-02-21
**Requirements Source:** requirements.md
**Codebase Conventions:** Table-driven tests with `t.Parallel()`, stdlib `testing` only (no testify/gomock), `time.Now().UnixNano()` for unique DB IDs, `t.Cleanup()` for teardown, DB tests use live PostgreSQL via CI services, nil-guard pattern for cache (`if cache.Marshal != nil`)

## Design Overview

This design covers raising test coverage from 5.3% to 40%+ by systematically adding tests to all packages with testable logic. The work splits into three independent streams that can execute in parallel: (1) pure-function unit tests for packages with zero or struct-only dependencies, (2) DB CRUD integration tests for all 16 untested `*_db.go` files plus expansion of 4 partially-tested files, and (3) CI pipeline enhancement with coverage threshold enforcement.

The design follows existing patterns exactly. Every new test file mirrors the structure of `callbackcodec_test.go` (pure functions) or `captcha_db_test.go` (DB integration). No new dependencies are introduced. No architectural changes to production code. DB tests rely on the CI-provided PostgreSQL service and skip gracefully when it is unavailable. The `config.init()` / `db.init()` fatal crash on missing env vars is handled by the CI environment setting `BOT_TOKEN=test-token` -- no production code changes.

The key infrastructure addition is a `TestMain` in `alita/db/` that calls `DB.AutoMigrate()` once for all GORM models and skips the entire package when PostgreSQL is unavailable. This replaces per-test `AutoMigrate` calls and is the only shared setup required.

## Component Architecture

### Component: error_handling tests

**Responsibility:** Verify `HandleErr`, `RecoverFromPanic`, and `CaptureError` do not panic and behave correctly for nil/non-nil inputs.
**Location:** `/Users/divkix/GitHub/Alita_Robot/alita/utils/error_handling/error_handling_test.go` (NEW)
**Pattern:** Same as `callbackcodec_test.go` -- pure function table-driven tests.

**Public Interface (test functions):**
```go
func TestHandleErr(t *testing.T)           // nil error, non-nil error, concurrent calls
func TestRecoverFromPanic(t *testing.T)     // panic recovery, no-panic no-op, empty strings
func TestCaptureError(t *testing.T)         // nil error, non-nil error, nil tags, empty tags
```

**Dependencies:** None (package has no external imports beyond logrus/debug)

**Error Handling:**
- All functions are void/no return -- tests verify no-panic behavior via `t.Run` subtests
- `RecoverFromPanic` tested by triggering panic inside goroutine with deferred call

---

### Component: shutdown tests

**Responsibility:** Verify `NewManager`, `RegisterHandler`, and `executeHandler` lifecycle, LIFO order, and panic recovery.
**Location:** `/Users/divkix/GitHub/Alita_Robot/alita/utils/shutdown/graceful_test.go` (NEW)
**Pattern:** Same as `callbackcodec_test.go` -- pure function tests with concurrency verification.

**Public Interface (test functions):**
```go
func TestNewManager(t *testing.T)           // non-nil, empty handlers
func TestRegisterHandler(t *testing.T)      // single, multiple, concurrent registration
func TestExecuteHandler(t *testing.T)       // nil return, error return, panicking handler
```

**Dependencies:**
- `error_handling` (transitively, via import in graceful.go)

**Error Handling:**
- Tests must NOT call `WaitForShutdown()` or `shutdown()` (they call `os.Exit`)
- `executeHandler` is tested directly by constructing a `*Manager` and calling the method

---

### Component: decorators/misc tests

**Responsibility:** Verify `addToArray` and `AddCmdToDisableable` thread-safe append semantics.
**Location:** `/Users/divkix/GitHub/Alita_Robot/alita/utils/decorators/misc/handler_vars_test.go` (NEW)
**Pattern:** Pure function tests with concurrency stress test.

**Public Interface (test functions):**
```go
func TestAddToArray(t *testing.T)           // nil slice, existing slice, empty args, empty string
func TestAddCmdToDisableable(t *testing.T)  // single cmd, concurrent 50 goroutines, duplicates
```

**Dependencies:** None

**Error Handling:**
- `addToArray` is unexported -- tests are in same package, can call directly
- `DisableCmds` is package-level var -- tests must reset it in `t.Cleanup()` to avoid cross-test pollution

---

### Component: keyword_matcher cache tests

**Responsibility:** Close the 40% coverage gap by testing `NewCache`, `GetOrCreateMatcher`, `CleanupExpired`, `patternsEqual`.
**Location:** `/Users/divkix/GitHub/Alita_Robot/alita/utils/keyword_matcher/cache_test.go` (NEW)
**Pattern:** Same as `matcher_test.go` -- pure function tests with concurrency.

**Public Interface (test functions):**
```go
func TestNewCache(t *testing.T)             // ttl set, maps initialized
func TestGetOrCreateMatcher(t *testing.T)   // new chat, same patterns (cache hit), different patterns (replacement), empty patterns, concurrent
func TestCleanupExpired(t *testing.T)       // expired removed, unexpired kept, empty cache, zero TTL
func TestPatternsEqual(t *testing.T)        // same set, different order, different length, nil/nil, duplicates in one side
```

**Dependencies:** None (same package, unexported functions accessible)

**Error Handling:**
- Tests use short TTLs (1ms) and `time.Sleep` to force expiration
- `GetGlobalCache` is NOT tested (starts background goroutine)

---

### Component: extraction pure function tests

**Responsibility:** Verify `ExtractQuotes` and `IdFromReply` without Telegram API calls.
**Location:** `/Users/divkix/GitHub/Alita_Robot/alita/utils/extraction/extraction_test.go` (NEW)
**Pattern:** Table-driven tests constructing `gotgbot.Message` structs directly.

**Public Interface (test functions):**
```go
func TestExtractQuotes(t *testing.T)  // quoted text, word extraction, empty string, unmatched quote, special chars, multiline
func TestIdFromReply(t *testing.T)    // nil ReplyToMessage, valid reply with text, reply with no spaces
```

**Dependencies:**
- `gotgbot/v2` (struct construction only, no API calls)
- Package imports `db`, `i18n`, `chat_status` transitively -- requires CI env vars

**Error Handling:**
- `IdFromReply` calls `prevMessage.GetSender().Id()` which requires `ReplyToMessage.From` to be non-nil -- tests must set `From` field
- Package-level skip guard not needed (CI has env vars), but the functions themselves are pure once the package loads

---

### Component: DB cache key generator tests

**Responsibility:** Verify all 8 cache key generators produce correct format strings.
**Location:** `/Users/divkix/GitHub/Alita_Robot/alita/db/cache_helpers_test.go` (NEW)
**Pattern:** Table-driven tests, pure function verification.

**Public Interface (test functions):**
```go
func TestCacheKeyGenerators(t *testing.T)  // all 8 functions, positive/zero/negative IDs, prefix verification, uniqueness
```

**Dependencies:**
- Package imports `config` -- requires CI env vars

**Error Handling:**
- Functions are unexported -- test file is in `db` package, can access directly
- No DB connection needed for key generators (pure `fmt.Sprintf`)

---

### Component: DB `cleanSupabaseSQL` and `splitSQLStatements` tests

**Responsibility:** Verify SQL cleaning removes GRANT, policy, and extension statements; verify idempotency transformations; verify SQL statement splitting.
**Location:** `/Users/divkix/GitHub/Alita_Robot/alita/db/migrations_test.go` (NEW)
**Pattern:** Table-driven tests with SQL string inputs.

**Public Interface (test functions):**
```go
func TestCleanSupabaseSQL(t *testing.T)      // GRANT removal, extension handling, schema removal, idempotency transforms, empty SQL, clean passthrough
func TestSplitSQLStatements(t *testing.T)    // simple split, dollar quotes, block comments, line comments, quoted semicolons
func TestSchemaMigrationTableName(t *testing.T)  // returns "schema_migrations"
```

**Dependencies:**
- `MigrationRunner` requires `config.AppConfig.MigrationsPath` for construction via `NewMigrationRunner`
- Tests construct `MigrationRunner` with test-specific fields to avoid config dependency: `&MigrationRunner{db: nil, migrationsPath: "", cleanSQL: true}`

**Error Handling:**
- Methods `cleanSupabaseSQL` and `splitSQLStatements` are on `*MigrationRunner` receiver but do not use `m.db` -- safe to call with nil db
- `SchemaMigration.TableName()` is a plain method, no dependencies

---

### Component: modules callback codec tests

**Responsibility:** Verify `encodeCallbackData` and `decodeCallbackData` module-level wrappers.
**Location:** `/Users/divkix/GitHub/Alita_Robot/alita/modules/callback_codec_test.go` (NEW)
**Pattern:** Same as `callback_parse_overwrite_test.go` -- pure function tests.

**Public Interface (test functions):**
```go
func TestEncodeCallbackData(t *testing.T)  // valid encode, encode error with fallback, nil fields, empty fallback
func TestDecodeCallbackData(t *testing.T)  // valid decode no namespace filter, namespace match, namespace mismatch (case insensitive), invalid data, empty string
```

**Dependencies:**
- Package imports `db`, `config` transitively -- requires CI env vars

**Error Handling:**
- Functions are unexported -- test file in `modules` package

---

### Component: DB TestMain

**Responsibility:** One-time `AutoMigrate` for all GORM models before any DB test runs; skip all tests when PostgreSQL is unavailable.
**Location:** `/Users/divkix/GitHub/Alita_Robot/alita/db/testmain_test.go` (NEW)
**Pattern:** No existing pattern (first `TestMain` in codebase).

**Public Interface:**
```go
func TestMain(m *testing.M)  // AutoMigrate all models, run tests, exit
```

**Dependencies:**
- `DB` global variable (set by `db.init()`)
- All GORM model types from `db.go`

**Error Handling:**
- `DB == nil` -> print skip message to stderr, `os.Exit(0)` (skips all tests gracefully)
- `DB.AutoMigrate()` fails -> print error, `os.Exit(1)` (cannot proceed with partial schema)
- Must AutoMigrate ALL models used by any test file in the package

---

### Component: DB CRUD test files (16 new files)

**Responsibility:** Integration tests for all DB CRUD operations.
**Location:** One test file per DB source file (see File-by-File Change Plan below).
**Pattern:** Same as `captcha_db_test.go` -- unique IDs via `time.Now().UnixNano()`, `t.Cleanup()` for data removal.

**Shared test helper pattern for all DB test files:**
```go
func skipIfNoDb(t *testing.T) {
    t.Helper()
    if DB == nil {
        t.Skip("requires PostgreSQL connection")
    }
}
```

This helper is defined in `testmain_test.go` and used at the top of every DB test function. When `TestMain` skips (DB == nil), individual tests never run, but the helper provides defense-in-depth.

**Dependencies:**
- Live PostgreSQL (CI provides via services)
- `TestMain` (US-009) for shared migration

**Error Handling:**
- Every test must use `time.Now().UnixNano()` for unique IDs to prevent cross-test interference
- Every test must use `t.Cleanup()` to delete test data
- Tests that set boolean fields to `false` must verify the round-trip (GORM zero-value gotcha)
- DB functions that call `ChatExists()` internally require the chat to exist first -- tests call `EnsureChatInDb()` in setup

---

### Component: helpers expanded tests

**Responsibility:** Add tests for `Shtml`, `Smarkdown`, `GetMessageLinkFromMessageId`, `GetLangFormat`, `ExtractJoinLeftStatusChange`, `ExtractAdminUpdateStatusChange`.
**Location:** `/Users/divkix/GitHub/Alita_Robot/alita/utils/helpers/helpers_test.go` (MODIFY -- add test functions)
**Pattern:** Same as existing 28 tests in the file.

**New test functions:**
```go
func TestShtml(t *testing.T)                              // returns HTML parse mode opts
func TestSmarkdown(t *testing.T)                           // returns Markdown parse mode opts
func TestGetMessageLinkFromMessageId(t *testing.T)         // supergroup ID, private chat ID, zero messageID
func TestGetLangFormat(t *testing.T)                       // "en", "es", "fr", "hi", unknown
func TestExtractJoinLeftStatusChange(t *testing.T)         // join event, left event, nil update
func TestExtractAdminUpdateStatusChange(t *testing.T)      // promotion, demotion, nil update
```

**Dependencies:**
- `gotgbot/v2` types (struct construction)
- Package imports `config`, `db` -- requires CI env vars

---

### Component: i18n expanded tests

**Responsibility:** Add tests for `Translator.Get`, `Translator.GetPlural`, `LocaleManager` methods.
**Location:** `/Users/divkix/GitHub/Alita_Robot/alita/i18n/i18n_test.go` (MODIFY -- add test functions)
**Pattern:** Same as existing 10 tests in the file.

**New test functions:**
```go
func TestTranslatorGet(t *testing.T)              // existing key, nonexistent key, key with params, nil params
func TestTranslatorGetPlural(t *testing.T)         // count=0, count=1, count=2+
func TestLocaleManagerGetTranslator(t *testing.T)  // "en", nonexistent locale
func TestLocaleManagerGetAvailableLocales(t *testing.T)  // at least en, es, fr, hi
```

**Dependencies:**
- Package imports `config` transitively -- requires CI env vars

---

### Component: monitoring auto_remediation tests

**Responsibility:** Verify `CanExecute`, `Name`, and `Severity` for all 4 action types.
**Location:** `/Users/divkix/GitHub/Alita_Robot/alita/utils/monitoring/auto_remediation_test.go` (NEW)
**Pattern:** Table-driven tests with constructed `SystemMetrics` structs.

**Public Interface (test functions):**
```go
func TestGCActionCanExecute(t *testing.T)                     // above/below threshold, boundary
func TestMemoryCleanupActionCanExecute(t *testing.T)           // above/below GC threshold
func TestLogWarningActionCanExecute(t *testing.T)              // high goroutines, high memory, below threshold
func TestRestartRecommendationActionCanExecute(t *testing.T)  // above 150% threshold
func TestActionNames(t *testing.T)                             // all 4 names
func TestActionSeverityOrdering(t *testing.T)                  // LogWarning=0 < GC=1 < MemoryCleanup=2 < Restart=10
```

**Dependencies:**
- `config.AppConfig` (read by `CanExecute` methods) -- requires CI env vars

**Error Handling:**
- Tests must NOT call `Execute()` (triggers actual GC/runtime operations)
- Zero-value `SystemMetrics` -> all actions return false (verified)

---

### Component: monitoring background_stats tests

**Responsibility:** Verify atomic counter methods and `GetCurrentMetrics` concurrency.
**Location:** `/Users/divkix/GitHub/Alita_Robot/alita/utils/monitoring/background_stats_test.go` (NEW)
**Pattern:** Table-driven tests with concurrency.

**Public Interface (test functions):**
```go
func TestNewBackgroundStatsCollector(t *testing.T)   // zero counters, default intervals
func TestRecordMessage(t *testing.T)                  // increment, concurrent increments
func TestRecordError(t *testing.T)                    // increment, concurrent increments
func TestRecordResponseTime(t *testing.T)             // single, zero duration, concurrent
func TestGetCurrentMetrics(t *testing.T)              // initial zero, concurrent reads
```

**Dependencies:**
- Package imports `db` -- requires CI env vars

**Error Handling:**
- Tests must NOT call `Start()` or `Stop()` (start background goroutines)
- Tests only call atomic counter methods and `GetCurrentMetrics`

---

### Component: CI coverage threshold

**Responsibility:** Fail CI if coverage drops below 40%.
**Location:** `/Users/divkix/GitHub/Alita_Robot/.github/workflows/ci.yml` (MODIFY)
**Pattern:** Shell script step after test execution.

---

## Data Models

No new data models are introduced. All tests use existing GORM models defined in `/Users/divkix/GitHub/Alita_Robot/alita/db/db.go`.

**Models requiring AutoMigrate in TestMain:**
```go
DB.AutoMigrate(
    &User{},
    &Chat{},
    &WarnSettings{},
    &Warns{},
    &GreetingSettings{},
    &ChatFilters{},
    &AdminSettings{},
    &BlacklistSettings{},
    &PinSettings{},
    &ReportChatSettings{},
    &ReportUserSettings{},
    &DevSettings{},
    &ChannelSettings{},
    &AntifloodSettings{},
    &ConnectionSettings{},
    &ConnectionChatSettings{},
    &DisableSettings{},
    &DisableChatSettings{},
    &RulesSettings{},
    &LockSettings{},
    &NotesSettings{},
    &Notes{},
    &CaptchaSettings{},
    &CaptchaAttempts{},
    &StoredMessages{},
    &CaptchaMutedUsers{},
)
```

No database migrations are needed. Tests use `AutoMigrate` which is a GORM DDL operation, not a migration file.

**Cache Strategy:** Not applicable -- tests do not introduce new cache keys.

## Data Flow

### Flow 1: Pure function test execution
1. `go test` compiles the test package
2. Package `init()` runs (for packages importing `config`: requires env vars)
3. Test function executes with `t.Parallel()`
4. Each subtest constructs input, calls function, asserts output
5. No external I/O

**Error paths:**
- Package init fails (missing env vars) -> test binary crashes (only happens locally, not CI)

### Flow 2: DB integration test execution
1. `go test` compiles the `db` package
2. `db.init()` runs, connects to PostgreSQL (requires `DATABASE_URL`)
3. `TestMain` runs, calls `AutoMigrate` for all models
4. Individual test functions run with `t.Parallel()`
5. Each test: creates unique test data -> exercises CRUD functions -> asserts results -> `t.Cleanup` removes data
6. `TestMain` calls `os.Exit(m.Run())` after all tests complete

**Error paths:**
- `db.init()` fails (no PostgreSQL) -> `DB == nil`, `TestMain` prints skip, exits 0
- `AutoMigrate` fails -> `TestMain` prints error, exits 1 (cannot proceed with partial schema)
- Individual CRUD operation fails -> test reports via `t.Fatalf`/`t.Errorf`

### Flow 3: Coverage threshold enforcement
1. `make test` generates `coverage.out`
2. CI step parses `coverage.out` with `go tool cover -func`
3. Extracts total percentage
4. Compares against threshold (40%)
5. Fails job if below threshold

**Error paths:**
- `coverage.out` missing -> `go tool cover` fails with non-zero exit -> CI fails
- Coverage below threshold -> explicit failure with message

## API Contracts

### TestMain Contract (alita/db/testmain_test.go)

**Input:** `*testing.M` (Go testing framework provides)

**Output (success):** `os.Exit(m.Run())` -- runs all tests, exits with their aggregate status

**Output (DB unavailable):**
```
fmt.Println("Skipping DB tests: PostgreSQL not available (DB == nil)")
os.Exit(0)
```

**Output (migration failure):**
```
fmt.Printf("TestMain: AutoMigrate failed: %v\n", err)
os.Exit(1)
```

### skipIfNoDb Contract

**Input:** `*testing.T`

**Output:** Either `t.Skip(...)` (test skipped) or returns (test proceeds)

```go
func skipIfNoDb(t *testing.T) {
    t.Helper()
    if DB == nil {
        t.Skip("requires PostgreSQL connection")
    }
}
```

### DB Test Data Isolation Contract

Every DB test function must follow this contract:
```go
func TestXxx(t *testing.T) {
    t.Parallel()
    skipIfNoDb(t)

    base := time.Now().UnixNano()
    chatID := base
    userID := base + 1

    // Setup: ensure chat exists (many DB functions call ChatExists internally)
    err := EnsureChatInDb(chatID, "test_chat")
    if err != nil {
        t.Fatalf("EnsureChatInDb() error = %v", err)
    }

    t.Cleanup(func() {
        // Delete ALL test data created by this test
        DB.Where("chat_id = ?", chatID).Delete(&ModelType{})
        DB.Where("chat_id = ?", chatID).Delete(&Chat{})
    })

    // ... test body ...
}
```

## Testing Strategy

### Unit Tests (Stream A: Pure Functions)

| Component | Test File | Test Functions | Verification |
|-----------|-----------|----------------|-------------|
| error_handling | `error_handling_test.go` | `TestHandleErr` (3 subtests), `TestRecoverFromPanic` (3 subtests), `TestCaptureError` (4 subtests) | No panics, correct nil-handling |
| shutdown | `graceful_test.go` | `TestNewManager` (2 subtests), `TestRegisterHandler` (3 subtests), `TestExecuteHandler` (3 subtests) | LIFO order, panic recovery, concurrent registration |
| decorators/misc | `handler_vars_test.go` | `TestAddToArray` (4 subtests), `TestAddCmdToDisableable` (3 subtests) | Thread-safe append, no data races |
| keyword_matcher cache | `cache_test.go` | `TestNewCache` (2 subtests), `TestGetOrCreateMatcher` (5 subtests), `TestCleanupExpired` (4 subtests), `TestPatternsEqual` (6 subtests) | Cache lifecycle, TTL expiry, set comparison |
| extraction | `extraction_test.go` | `TestExtractQuotes` (8 subtests), `TestIdFromReply` (4 subtests) | Regex parsing, struct-based reply extraction |
| modules callback codec | `callback_codec_test.go` | `TestEncodeCallbackData` (4 subtests), `TestDecodeCallbackData` (5 subtests) | Encode/decode round-trip, fallback, namespace filtering |
| DB cache keys | `cache_helpers_test.go` | `TestCacheKeyGenerators` (all 8 functions, 3+ inputs each) | Format `alita:{segment}:{id}`, uniqueness |
| DB migrations | `migrations_test.go` | `TestCleanSupabaseSQL` (6 subtests), `TestSplitSQLStatements` (5 subtests), `TestSchemaMigrationTableName` (1 subtest) | SQL cleaning, statement splitting |

### Integration Tests (Stream B: DB CRUD)

| Component | Test File | Functions Tested | Verification |
|-----------|-----------|-----------------|-------------|
| TestMain | `testmain_test.go` | AutoMigrate all models | Schema ready, skip on no DB |
| greetings | `greetings_db_test.go` | 15 functions | Default settings, welcome/goodbye toggle, clean service, auto approve, buttons, stats |
| warns | `warns_db_test.go` | 13 functions | Default settings, warn/unwarn, reset, limit/mode, concurrent warns |
| notes | `notes_db_test.go` | 11 functions | Settings, add/get/remove note, note list, exists check, private toggle, stats |
| filters | `filters_db_test.go` | 7 functions | Add/remove filter, exists, count, list with cache, stats |
| admin | `admin_db_test.go` | 3 functions | Default settings, anon admin toggle |
| blacklists | `blacklists_db_test.go` | 6 functions | Add/remove trigger, action setting, cached retrieval, stats |
| channels | `channels_db_test.go` | 6 functions | Registration, lookup by ID/username |
| chats | `chats_db_test.go` | 6+ functions | EnsureChatInDb, UpdateChat, GetAllChats, ChatExists, LoadChatStats |
| connections | `connections_db_test.go` | 8 functions | Connect/disconnect, reconnect, allow_connect toggle, stats |
| devs | `devs_db_test.go` | 7 functions | Dev/sudo management, dual boolean fields |
| disable | `disable_db_test.go` | 9 functions | Disable/enable cmd, cached retrieval, IsCommandDisabled, ToggleDel, stats |
| lang | `lang_db_test.go` | 5 functions | User/chat language get/set |
| pin | `pin_db_test.go` | 4 functions | Pin settings, clean linked, anti-channel pin |
| reports | `reports_db_test.go` | 7 functions | Chat/user report settings, blocked list |
| rules | `rules_db_test.go` | 6 functions | Rules CRUD, private toggle |
| user | `user_db_test.go` | 8 functions | EnsureUserInDb, UpdateUser, GetUserIdByUserName, stats |
| captcha (expand) | `captcha_db_test.go` | remaining functions | Settings getter/setter coverage |
| locks (expand) | `locks_db_test.go` | remaining functions | GetAllLocks, additional lock types |
| antiflood (expand) | `antiflood_db_test.go` | remaining functions | GetFlood, additional settings |

### Edge Case Tests

Every DB test file includes these mandatory edge cases:
1. **Zero-value boolean round-trip:** Set `true`, verify, set `false`, verify `false` is persisted
2. **Concurrent writes:** At least one test per file spawns N goroutines writing to same entity
3. **Non-existent record lookup:** Verify graceful handling (default struct, not panic)
4. **Delete non-existent:** Verify no error, no-op
5. **Empty/zero inputs:** Empty strings, zero IDs

### Verification Commands

```bash
# Run all tests locally (requires env vars + PostgreSQL)
BOT_TOKEN=test-token OWNER_ID=1 MESSAGE_DUMP=1 DATABASE_URL="postgres://postgres:postgres@localhost:5432/alita_test?sslmode=disable" make test

# Run only pure-function tests locally (no env vars needed)
go test ./alita/utils/error_handling/... ./alita/utils/shutdown/... ./alita/utils/decorators/misc/... ./alita/utils/keyword_matcher/...

# Run DB tests only (requires PostgreSQL)
BOT_TOKEN=test-token OWNER_ID=1 MESSAGE_DUMP=1 DATABASE_URL="..." go test -v -race ./alita/db/...

# Check coverage
go tool cover -func=coverage.out | tail -1

# Check per-package coverage
go tool cover -func=coverage.out | grep -E "^(github|total)"

# Verify no data races
go test -race ./...
```

## Parallelization Analysis

### Independent Streams

**Stream A: Pure Function Tests (no shared files, no DB dependency)**
- `alita/utils/error_handling/error_handling_test.go` (NEW)
- `alita/utils/shutdown/graceful_test.go` (NEW)
- `alita/utils/decorators/misc/handler_vars_test.go` (NEW)
- `alita/utils/keyword_matcher/cache_test.go` (NEW)

These four files are in four separate packages with zero file overlap. They can be written simultaneously by four agents.

**Stream B: DB Infrastructure + CRUD Tests (shared TestMain, separate test files)**
- `alita/db/testmain_test.go` (NEW) -- must complete first within this stream
- `alita/db/cache_helpers_test.go` (NEW)
- `alita/db/migrations_test.go` (NEW)
- `alita/db/greetings_db_test.go` (NEW)
- `alita/db/warns_db_test.go` (NEW)
- `alita/db/notes_db_test.go` (NEW)
- `alita/db/filters_db_test.go` (NEW)
- `alita/db/admin_db_test.go` (NEW)
- `alita/db/blacklists_db_test.go` (NEW)
- `alita/db/channels_db_test.go` (NEW)
- `alita/db/chats_db_test.go` (NEW)
- `alita/db/connections_db_test.go` (NEW)
- `alita/db/devs_db_test.go` (NEW)
- `alita/db/disable_db_test.go` (NEW)
- `alita/db/lang_db_test.go` (NEW)
- `alita/db/pin_db_test.go` (NEW)
- `alita/db/reports_db_test.go` (NEW)
- `alita/db/rules_db_test.go` (NEW)
- `alita/db/user_db_test.go` (NEW)
- `alita/db/captcha_db_test.go` (MODIFY -- expand)
- `alita/db/locks_db_test.go` (MODIFY -- expand)
- `alita/db/antiflood_db_test.go` (MODIFY -- expand)

All DB test files are in the same package (`db`) but each file tests a different source file. They do NOT share any source file modifications. The `testmain_test.go` must be written first, then all other DB test files can be written in parallel.

**Stream C: CI-Dependent Package Tests + CI Enhancement**
- `alita/utils/extraction/extraction_test.go` (NEW)
- `alita/modules/callback_codec_test.go` (NEW)
- `alita/utils/helpers/helpers_test.go` (MODIFY -- add functions)
- `alita/i18n/i18n_test.go` (MODIFY -- add functions)
- `alita/utils/monitoring/auto_remediation_test.go` (NEW)
- `alita/utils/monitoring/background_stats_test.go` (NEW)
- `.github/workflows/ci.yml` (MODIFY -- add coverage threshold step)

These files are in separate packages and do not share source file modifications with Stream A or B. They require CI env vars but no DB.

### Sequential Dependencies

1. `testmain_test.go` MUST complete before any other `alita/db/*_test.go` file is written, because it defines the `TestMain` and `skipIfNoDb` helper that all DB tests use.
2. `captcha_db_test.go`, `locks_db_test.go`, `antiflood_db_test.go` are MODIFY operations -- they must remove existing `DB.AutoMigrate()` calls after `testmain_test.go` is in place.

### Shared Resources (Serialization Points)

- `/Users/divkix/GitHub/Alita_Robot/alita/db/captcha_db_test.go` -- modified by Stream B (remove AutoMigrate, add tests)
- `/Users/divkix/GitHub/Alita_Robot/alita/db/locks_db_test.go` -- modified by Stream B (remove AutoMigrate, add tests)
- `/Users/divkix/GitHub/Alita_Robot/alita/db/antiflood_db_test.go` -- modified by Stream B (remove AutoMigrate, add tests)
- `/Users/divkix/GitHub/Alita_Robot/alita/utils/helpers/helpers_test.go` -- modified by Stream C (add test functions)
- `/Users/divkix/GitHub/Alita_Robot/alita/i18n/i18n_test.go` -- modified by Stream C (add test functions)
- `/Users/divkix/GitHub/Alita_Robot/.github/workflows/ci.yml` -- modified by Stream C (add coverage step)

No file is modified by more than one stream.

## File-by-File Change Plan

### NEW Files (27 files)

| # | File Path | Package | US | Stream |
|---|-----------|---------|-----|--------|
| 1 | `/Users/divkix/GitHub/Alita_Robot/alita/utils/error_handling/error_handling_test.go` | error_handling | US-002 | A |
| 2 | `/Users/divkix/GitHub/Alita_Robot/alita/utils/shutdown/graceful_test.go` | shutdown | US-003 | A |
| 3 | `/Users/divkix/GitHub/Alita_Robot/alita/utils/decorators/misc/handler_vars_test.go` | misc | US-004 | A |
| 4 | `/Users/divkix/GitHub/Alita_Robot/alita/utils/keyword_matcher/cache_test.go` | keyword_matcher | US-005 | A |
| 5 | `/Users/divkix/GitHub/Alita_Robot/alita/db/testmain_test.go` | db | US-009 | B |
| 6 | `/Users/divkix/GitHub/Alita_Robot/alita/db/cache_helpers_test.go` | db | US-008 | B |
| 7 | `/Users/divkix/GitHub/Alita_Robot/alita/db/migrations_test.go` | db | US-020 | B |
| 8 | `/Users/divkix/GitHub/Alita_Robot/alita/db/greetings_db_test.go` | db | US-010 | B |
| 9 | `/Users/divkix/GitHub/Alita_Robot/alita/db/warns_db_test.go` | db | US-011 | B |
| 10 | `/Users/divkix/GitHub/Alita_Robot/alita/db/notes_db_test.go` | db | US-012 | B |
| 11 | `/Users/divkix/GitHub/Alita_Robot/alita/db/filters_db_test.go` | db | US-013 | B |
| 12 | `/Users/divkix/GitHub/Alita_Robot/alita/db/admin_db_test.go` | db | US-014 | B |
| 13 | `/Users/divkix/GitHub/Alita_Robot/alita/db/blacklists_db_test.go` | db | US-014 | B |
| 14 | `/Users/divkix/GitHub/Alita_Robot/alita/db/channels_db_test.go` | db | US-014 | B |
| 15 | `/Users/divkix/GitHub/Alita_Robot/alita/db/chats_db_test.go` | db | US-014 | B |
| 16 | `/Users/divkix/GitHub/Alita_Robot/alita/db/connections_db_test.go` | db | US-014 | B |
| 17 | `/Users/divkix/GitHub/Alita_Robot/alita/db/devs_db_test.go` | db | US-014 | B |
| 18 | `/Users/divkix/GitHub/Alita_Robot/alita/db/disable_db_test.go` | db | US-014 | B |
| 19 | `/Users/divkix/GitHub/Alita_Robot/alita/db/lang_db_test.go` | db | US-014 | B |
| 20 | `/Users/divkix/GitHub/Alita_Robot/alita/db/pin_db_test.go` | db | US-014 | B |
| 21 | `/Users/divkix/GitHub/Alita_Robot/alita/db/reports_db_test.go` | db | US-014 | B |
| 22 | `/Users/divkix/GitHub/Alita_Robot/alita/db/rules_db_test.go` | db | US-014 | B |
| 23 | `/Users/divkix/GitHub/Alita_Robot/alita/db/user_db_test.go` | db | US-014 | B |
| 24 | `/Users/divkix/GitHub/Alita_Robot/alita/utils/extraction/extraction_test.go` | extraction | US-006 | C |
| 25 | `/Users/divkix/GitHub/Alita_Robot/alita/modules/callback_codec_test.go` | modules | US-007 | C |
| 26 | `/Users/divkix/GitHub/Alita_Robot/alita/utils/monitoring/auto_remediation_test.go` | monitoring | US-016 | C |
| 27 | `/Users/divkix/GitHub/Alita_Robot/alita/utils/monitoring/background_stats_test.go` | monitoring | US-017 | C |

### MODIFY Files (6 files)

| # | File Path | Change | US | Stream |
|---|-----------|--------|-----|--------|
| 28 | `/Users/divkix/GitHub/Alita_Robot/alita/db/captcha_db_test.go` | Remove `DB.AutoMigrate` calls (lines 13-15, 81-83), add tests for remaining functions (`CreateCaptchaAttemptPreMessage`, `GetCaptchaSettings`, `GetCaptchaAttemptByID`, etc.) | US-015 | B |
| 29 | `/Users/divkix/GitHub/Alita_Robot/alita/db/locks_db_test.go` | Remove `DB.AutoMigrate` calls, add tests for `GetAllLocks`, additional lock types | US-015 | B |
| 30 | `/Users/divkix/GitHub/Alita_Robot/alita/db/antiflood_db_test.go` | Remove `DB.AutoMigrate` calls, add tests for `GetFlood` and remaining settings functions | US-015 | B |
| 31 | `/Users/divkix/GitHub/Alita_Robot/alita/utils/helpers/helpers_test.go` | Add 6+ new test functions: `TestShtml`, `TestSmarkdown`, `TestGetMessageLinkFromMessageId`, `TestGetLangFormat`, `TestExtractJoinLeftStatusChange`, `TestExtractAdminUpdateStatusChange` (no existing code modified) | US-019 | C |
| 32 | `/Users/divkix/GitHub/Alita_Robot/alita/i18n/i18n_test.go` | Add 4+ new test functions: `TestTranslatorGet`, `TestTranslatorGetPlural`, `TestLocaleManagerGetTranslator`, `TestLocaleManagerGetAvailableLocales` (no existing code modified) | US-018 | C |
| 33 | `/Users/divkix/GitHub/Alita_Robot/.github/workflows/ci.yml` | Add coverage threshold check step after "Run test suite" step (see CI Coverage Threshold section below) | US-021 | C |

### NO CHANGE Files (production code)

No production source files are modified. All changes are test files and CI configuration only.

## Detailed Test Function Specifications

### `/Users/divkix/GitHub/Alita_Robot/alita/utils/error_handling/error_handling_test.go`

```go
package error_handling

import (
    "errors"
    "fmt"
    "sync"
    "testing"
)

func TestHandleErr(t *testing.T) {
    t.Parallel()
    tests := []struct {
        name string
        err  error
    }{
        {"nil error", nil},
        {"non-nil error", errors.New("test error")},
        {"wrapped error", fmt.Errorf("wrapped: %w", errors.New("inner"))},
    }
    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            t.Parallel()
            HandleErr(tc.err) // must not panic
        })
    }
}

func TestRecoverFromPanic(t *testing.T) {
    t.Parallel()

    t.Run("recovers from panic", func(t *testing.T) {
        t.Parallel()
        done := make(chan bool, 1)
        go func() {
            defer RecoverFromPanic("testFunc", "testMod")
            done <- true
            panic("test panic")
        }()
        <-done // goroutine started
    })

    t.Run("no-op when no panic", func(t *testing.T) {
        t.Parallel()
        defer RecoverFromPanic("testFunc", "testMod")
        // no panic -- should be a no-op
    })

    t.Run("empty funcName and modName", func(t *testing.T) {
        t.Parallel()
        done := make(chan bool, 1)
        go func() {
            defer RecoverFromPanic("", "")
            done <- true
            panic("empty names panic")
        }()
        <-done
    })
}

func TestCaptureError(t *testing.T) {
    t.Parallel()
    tests := []struct {
        name string
        err  error
        tags map[string]string
    }{
        {"nil error", nil, map[string]string{"key": "val"}},
        {"non-nil error with tags", errors.New("test"), map[string]string{"module": "test"}},
        {"nil tags", errors.New("test"), nil},
        {"empty tags", errors.New("test"), map[string]string{}},
    }
    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            t.Parallel()
            CaptureError(tc.err, tc.tags) // must not panic
        })
    }
}

func TestHandleErrConcurrent(t *testing.T) {
    t.Parallel()
    var wg sync.WaitGroup
    for i := 0; i < 50; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            HandleErr(errors.New("concurrent error"))
        }()
    }
    wg.Wait()
}
```

### `/Users/divkix/GitHub/Alita_Robot/alita/db/testmain_test.go`

```go
package db

import (
    "fmt"
    "os"
    "testing"
)

// skipIfNoDb skips the current test if PostgreSQL is not available.
func skipIfNoDb(t *testing.T) {
    t.Helper()
    if DB == nil {
        t.Skip("requires PostgreSQL connection")
    }
}

func TestMain(m *testing.M) {
    if DB == nil {
        fmt.Println("Skipping DB tests: PostgreSQL not available (DB == nil)")
        os.Exit(0)
    }

    err := DB.AutoMigrate(
        &User{},
        &Chat{},
        &WarnSettings{},
        &Warns{},
        &GreetingSettings{},
        &ChatFilters{},
        &AdminSettings{},
        &BlacklistSettings{},
        &PinSettings{},
        &ReportChatSettings{},
        &ReportUserSettings{},
        &DevSettings{},
        &ChannelSettings{},
        &AntifloodSettings{},
        &ConnectionSettings{},
        &ConnectionChatSettings{},
        &DisableSettings{},
        &DisableChatSettings{},
        &RulesSettings{},
        &LockSettings{},
        &NotesSettings{},
        &Notes{},
        &CaptchaSettings{},
        &CaptchaAttempts{},
        &StoredMessages{},
        &CaptchaMutedUsers{},
    )
    if err != nil {
        fmt.Printf("TestMain: AutoMigrate failed: %v\n", err)
        os.Exit(1)
    }

    os.Exit(m.Run())
}
```

### `/Users/divkix/GitHub/Alita_Robot/alita/db/greetings_db_test.go` (representative DB test)

```go
package db

import (
    "testing"
    "time"
)

func TestGetGreetingSettings_Defaults(t *testing.T) {
    t.Parallel()
    skipIfNoDb(t)

    chatID := time.Now().UnixNano()
    if err := EnsureChatInDb(chatID, "test_greetings_chat"); err != nil {
        t.Fatalf("EnsureChatInDb() error = %v", err)
    }
    t.Cleanup(func() {
        DB.Where("chat_id = ?", chatID).Delete(&GreetingSettings{})
        DB.Where("chat_id = ?", chatID).Delete(&Chat{})
    })

    settings := GetGreetingSettings(chatID)
    if settings == nil {
        t.Fatal("expected non-nil settings")
    }
    if !settings.WelcomeSettings.ShouldWelcome {
        t.Errorf("expected ShouldWelcome=true by default")
    }
    if settings.WelcomeSettings.WelcomeText != DefaultWelcome {
        t.Errorf("expected WelcomeText=%q, got %q", DefaultWelcome, settings.WelcomeSettings.WelcomeText)
    }
    if settings.GoodbyeSettings.ShouldGoodbye {
        t.Errorf("expected ShouldGoodbye=false by default")
    }
}

func TestSetWelcomeToggle_ZeroValueBoolean(t *testing.T) {
    t.Parallel()
    skipIfNoDb(t)

    chatID := time.Now().UnixNano()
    if err := EnsureChatInDb(chatID, "test_welcome_toggle"); err != nil {
        t.Fatalf("EnsureChatInDb() error = %v", err)
    }
    t.Cleanup(func() {
        DB.Where("chat_id = ?", chatID).Delete(&GreetingSettings{})
        DB.Where("chat_id = ?", chatID).Delete(&Chat{})
    })

    // Trigger creation of greeting settings
    _ = GetGreetingSettings(chatID)

    // Set to false (zero-value boolean)
    SetWelcomeToggle(chatID, false)
    settings := GetGreetingSettings(chatID)
    if settings.WelcomeSettings.ShouldWelcome {
        t.Errorf("expected ShouldWelcome=false after SetWelcomeToggle(false)")
    }

    // Set back to true
    SetWelcomeToggle(chatID, true)
    settings = GetGreetingSettings(chatID)
    if !settings.WelcomeSettings.ShouldWelcome {
        t.Errorf("expected ShouldWelcome=true after SetWelcomeToggle(true)")
    }
}

// Additional test function signatures (each follows the same pattern):
func TestSetWelcomeText(t *testing.T) { /* text, fileId, buttons, type round-trip */ }
func TestSetGoodbyeText(t *testing.T) { /* goodbye text round-trip */ }
func TestSetGoodbyeToggle_ZeroValueBoolean(t *testing.T) { /* same pattern as welcome */ }
func TestSetShouldCleanService(t *testing.T) { /* boolean round-trip */ }
func TestSetShouldAutoApprove(t *testing.T) { /* boolean round-trip */ }
func TestSetCleanWelcomeSetting(t *testing.T) { /* boolean round-trip */ }
func TestSetCleanWelcomeMsgId(t *testing.T) { /* int64 round-trip */ }
func TestSetCleanGoodbyeSetting(t *testing.T) { /* boolean round-trip */ }
func TestSetCleanGoodbyeMsgId(t *testing.T) { /* int64 round-trip */ }
func TestGetWelcomeButtons_Empty(t *testing.T) { /* returns empty slice, not nil */ }
func TestGetGoodbyeButtons_Empty(t *testing.T) { /* returns empty slice, not nil */ }
func TestLoadGreetingsStats_EmptyDB(t *testing.T) { /* all zeros on fresh DB */ }
func TestGreetingSettings_NonExistentChat(t *testing.T) { /* returns defaults without ChatExists */ }
```

### `/Users/divkix/GitHub/Alita_Robot/.github/workflows/ci.yml` modification

Insert after the "Run test suite" step and before the "Upload coverage reports" step:

```yaml
      - name: Check coverage threshold
        if: success()
        run: |
          if [ ! -f coverage.out ]; then
            echo "ERROR: coverage.out not found"
            exit 1
          fi

          THRESHOLD=40
          COVERAGE=$(go tool cover -func=coverage.out | grep '^total:' | awk '{print $3}' | tr -d '%')

          echo "## Coverage Report" >> $GITHUB_STEP_SUMMARY
          echo "" >> $GITHUB_STEP_SUMMARY
          echo "**Total Coverage:** ${COVERAGE}%" >> $GITHUB_STEP_SUMMARY
          echo "**Minimum Threshold:** ${THRESHOLD}%" >> $GITHUB_STEP_SUMMARY
          echo "" >> $GITHUB_STEP_SUMMARY

          # Use awk for float comparison (bc may not be available)
          BELOW=$(awk "BEGIN {print ($COVERAGE < $THRESHOLD) ? 1 : 0}")
          if [ "$BELOW" -eq 1 ]; then
            echo "FAIL: Coverage ${COVERAGE}% is below threshold ${THRESHOLD}%" >> $GITHUB_STEP_SUMMARY
            echo "Coverage ${COVERAGE}% is below threshold ${THRESHOLD}%"
            exit 1
          else
            echo "PASS: Coverage ${COVERAGE}% meets threshold ${THRESHOLD}%" >> $GITHUB_STEP_SUMMARY
          fi
```

## Design Decisions

### Decision: TestMain with os.Exit(0) for DB unavailable, not build tags

- **Context:** DB tests need PostgreSQL. How to handle local runs without it?
- **Options considered:** (a) `//go:build integration` tags, (b) `TestMain` with skip, (c) per-test `t.Skip`
- **Chosen:** (b) `TestMain` with `os.Exit(0)` because it aligns with how existing tests work -- no build tags anywhere in the codebase, and `TestMain` provides one-time setup. `os.Exit(0)` means Go reports "ok" for the package rather than "FAIL".
- **Trade-offs:** `os.Exit(0)` does not clearly indicate skipping in output; mitigated by printing a message to stdout. Build tags would be cleaner but would require changing `make test` to pass `-tags=integration`.

### Decision: skipIfNoDb helper in testmain_test.go, not a separate helper package

- **Context:** DB tests need a skip guard. Where to define it?
- **Options considered:** (a) separate `testhelpers` package, (b) in `testmain_test.go`, (c) inline in each test
- **Chosen:** (b) `testmain_test.go` because `_test.go` files in the same package can access each other's unexported symbols, and it avoids creating a new package for a single function.
- **Trade-offs:** If the helper were in a separate package, other packages could use it. But only the `db` package needs it -- other packages handle missing infra via the CI env var mechanism.

### Decision: Do not refactor config.init() to lazy initialization

- **Context:** `config.init()` calls `log.Fatalf` when `BOT_TOKEN` is missing, which kills 8 test packages locally.
- **Options considered:** (a) refactor to lazy init, (b) accept CI-only execution for those packages
- **Chosen:** (b) accept CI-only because refactoring `config.init()` is out of scope (requirements explicitly say so), and CI already works.
- **Trade-offs:** Developers cannot run these 8 packages' tests locally without setting env vars. This is documented and consistent with existing behavior.

### Decision: Remove AutoMigrate from existing DB test files after TestMain is in place

- **Context:** `captcha_db_test.go`, `locks_db_test.go`, `antiflood_db_test.go` call `DB.AutoMigrate` at the top of individual test functions. After `TestMain` runs `AutoMigrate` for all models, these become redundant.
- **Options considered:** (a) leave them (harmless but redundant), (b) remove them
- **Chosen:** (b) remove them to avoid confusion and to establish the convention that `TestMain` handles migration.
- **Trade-offs:** If `TestMain` is ever removed, these tests would need their `AutoMigrate` back. Extremely unlikely.

### Decision: 40% initial coverage threshold, not 60%

- **Context:** US-021 requires a CI threshold. What value?
- **Options considered:** 30%, 40%, 50%, 60%
- **Chosen:** 40% because it is achievable after P0 + P1 work (currently 5.3%, existing tests alone would bring it to ~15%, pure functions add ~10%, DB CRUD adds ~20%). Setting it at 60% risks blocking unrelated PRs before P2 work completes.
- **Trade-offs:** 40% is low by industry standards but is a safe starting point that can be ratcheted up.

### Decision: No new test framework dependencies

- **Context:** Several assertion patterns would be cleaner with testify.
- **Options considered:** (a) add testify, (b) add gomock, (c) stay with stdlib
- **Chosen:** (c) stay with stdlib `testing` per existing codebase convention and explicit requirement.
- **Trade-offs:** More verbose assertion code. Consistency over convenience.

### Decision: EnsureChatInDb as setup prerequisite for DB tests

- **Context:** Many DB functions (greetings, warns, notes, filters, etc.) call `ChatExists()` internally and return defaults without creating records if the chat does not exist. Tests need the chat to exist.
- **Options considered:** (a) call `EnsureChatInDb()` in each test, (b) create a shared fixture chat
- **Chosen:** (a) each test calls `EnsureChatInDb()` with its unique chat ID because shared fixtures risk cross-test pollution with `t.Parallel()`.
- **Trade-offs:** Slightly more verbose test setup. But total isolation guarantees no flaky tests.

## Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| `TestMain` `AutoMigrate` list falls out of sync with new GORM models | MEDIUM | New model not migrated, DB tests fail | CI failure is immediate; add model to list when adding to db.go |
| CI PostgreSQL service timing -- tests start before DB ready | LOW | Tests fail with connection error | Existing `pg_isready` loop in CI already handles this; `TestMain` also catches via `DB == nil` |
| Test data collision with `time.Now().UnixNano()` | LOW | Two parallel tests get same ID, false failure | Nanosecond resolution makes collision nearly impossible; each test also uses `base + offset` pattern |
| GORM zero-value boolean gotcha in new tests | MEDIUM | Test passes but doesn't verify the edge case | Design mandates explicit zero-value boolean round-trip test in every DB file with boolean fields |
| `config.AppConfig` fields have unexpected test-token defaults affecting monitoring tests | LOW | `CanExecute` thresholds based on default config values produce unexpected results | Test verifies against actual `config.AppConfig` values, not hardcoded expected values |
| Coverage threshold blocks unrelated PRs | LOW | Developer unable to merge because coverage dropped | Set initial threshold at 40% (conservative); only need to not regress |
| Existing test modifications introduce regressions | LOW | Removing AutoMigrate from existing tests breaks them | TestMain covers all models; run full suite before merging |
| `db.init()` log.Fatalf when PostgreSQL unreachable in CI | LOW | All DB tests fail if Postgres service not ready | CI has wait loop with 30 retries; `db.init()` has 5 retries with exponential backoff |
| Flaky concurrent DB tests | MEDIUM | Intermittent failures in CI | Each test uses unique IDs, `t.Cleanup` removes data, no shared mutable state between tests |

DESIGN_COMPLETE
