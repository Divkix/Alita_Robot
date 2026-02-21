# Requirements: Increase Test Coverage Across Alita Robot

**Date:** 2026-02-21
**Goal:** Raise overall test coverage from 5.3% to 60%+ by systematically adding tests to all packages, prioritizing pure functions, then DB CRUD, then infrastructure/monitoring.
**Source:** `specs/increase-test-coverage/research.md`

## Scope

### In Scope
- All Go packages under `alita/` that contain testable logic
- Pure function tests (Tier 1 and 2) for packages with zero or struct-only dependencies
- DB CRUD integration tests (Tier 3) for all 16 untested `*_db.go` files
- Cache key generator unit tests in `alita/db/cache_helpers.go`
- Monitoring and infrastructure package tests (Tier 4) where pure function extraction is feasible
- Keyword matcher `cache.go` coverage gap (currently 0% within the 60.2% package)
- Extraction package pure functions (`ExtractQuotes`, `IdFromReply`)
- Module-level pure function tests (`encodeCallbackData`, `decodeCallbackData`)
- CI pipeline enhancement: coverage threshold enforcement
- Fixing the 8 packages with existing tests that fail locally due to `config.init()` crash
- Expanding coverage on the 4 DB files that already have partial tests
- `TestMain` for the `alita/db/` package for shared initialization

### Out of Scope
- **Handler-level integration tests for `alita/modules/*` (250+ handler methods)** -- these require a `BotAPI` interface abstraction or live Telegram API, which is a separate refactor effort. Reason: ~100+ file refactor to introduce Bot interface.
- **Introducing third-party test frameworks** (testify, gomock, etc.) -- the project convention is stdlib `testing` only. Reason: maintaining zero test dependencies.
- **Creating a `BotAPI` interface wrapper around `gotgbot.Bot`** -- significant refactor, tracked separately. Reason: scope creep.
- **End-to-end tests with a real Telegram bot token** -- requires secrets management. Reason: security and infrastructure complexity.
- **Testing `alita/utils/webhook/`** -- requires Bot + HTTP integration. Reason: same Bot interface blocker.
- **Testing `alita/health/` beyond basic struct verification** -- requires live DB + Redis. Reason: infrastructure dependency.
- **Testing `alita/metrics/`** -- contains only Prometheus var declarations, no logic. Reason: nothing to test.
- **Testing `alita/utils/constants/`** -- contains only const declarations, no logic. Reason: nothing to test.
- **Testing `scripts/generate_docs/`** -- documentation generation tooling, not bot runtime. Reason: not production code.
- **Redis-dependent cache integration tests** -- CI has no Redis service; nil-guard pattern suffices. Reason: infrastructure not available.
- **Refactoring `config.init()` to lazy initialization** -- architectural change that would unblock local testing but is out of scope for a test coverage effort. Reason: separate design decision.

## User Stories

### US-001: Fix Locally-Failing Test Suites

**Priority:** P0 (must-have)

As a developer,
I want all existing test files to pass when run in CI with the established env vars,
So that the existing ~100 test cases contribute to measured coverage rather than being dead code.

**Acceptance Criteria:**
- [ ] GIVEN the 8 packages with failing tests (`config`, `db`, `i18n`, `modules`, `helpers`, `cache`, `chat_status`, `tracing`) WHEN `go test ./...` is run in CI with `BOT_TOKEN=test-token`, `OWNER_ID=1`, `MESSAGE_DUMP=1`, `DATABASE_URL` set THEN all test files in those packages pass (exit code 0)
- [ ] GIVEN any test file that requires PostgreSQL WHEN PostgreSQL is unavailable THEN the test is skipped with `t.Skip("requires PostgreSQL")` rather than failing
- [ ] GIVEN all existing test files pass WHEN coverage is measured THEN the overall coverage increases from 5.3% to at least 15% (from existing tests alone contributing)

**Edge Cases:**
- [ ] DB tests that rely on `DB.AutoMigrate()` SHALL call `t.Skip()` if `DB` is nil (no PostgreSQL connection) -> test is skipped, not failed
- [ ] Tests that import `config` transitively SHALL not crash when `DATABASE_URL` points to an unreachable host -> tests skip gracefully via `TestMain` or skip guard
- [ ] Tests SHALL not depend on Redis being available -> use nil-guard pattern (`if cache.Marshal != nil`)

**Definition of Done:**
- [ ] `make test` passes in CI with zero test failures
- [ ] No test file calls `log.Fatal` or `os.Exit` in test execution paths
- [ ] Coverage output shows non-zero percentages for all 8 previously-failing packages

---

### US-002: Test `error_handling` Package (3 functions)

**Priority:** P0 (must-have)

As a developer,
I want unit tests for `HandleErr`, `RecoverFromPanic`, and `CaptureError`,
So that the foundational error handling layer is verified to work correctly.

**Acceptance Criteria:**
- [ ] GIVEN `HandleErr` is called with a nil error WHEN executed THEN it does not panic
- [ ] GIVEN `HandleErr` is called with a non-nil error WHEN executed THEN it logs the error via logrus (no panic, no crash)
- [ ] GIVEN `RecoverFromPanic` is deferred in a goroutine that panics WHEN the panic occurs THEN the panic is recovered and does not propagate
- [ ] GIVEN `RecoverFromPanic` is deferred in a goroutine that does not panic WHEN the function completes THEN nothing happens (no-op)
- [ ] GIVEN `CaptureError` is called with nil error WHEN executed THEN it returns immediately without side effects
- [ ] GIVEN `CaptureError` is called with a non-nil error and tags map WHEN executed THEN it does not panic
- [ ] GIVEN `CaptureError` is called with an empty tags map WHEN executed THEN it does not panic

**Edge Cases:**
- [ ] `CaptureError` with nil tags map -> does not panic
- [ ] `RecoverFromPanic` with empty string funcName/modName -> does not panic
- [ ] `HandleErr` called concurrently from multiple goroutines -> no data races (verified by `-race`)

**Definition of Done:**
- [ ] Test file `alita/utils/error_handling/error_handling_test.go` exists
- [ ] All tests use table-driven pattern with `t.Parallel()`
- [ ] All 3 functions have at least 2 test cases each
- [ ] Package coverage >= 90%

---

### US-003: Test `shutdown` Package (Manager lifecycle)

**Priority:** P0 (must-have)

As a developer,
I want unit tests for the shutdown `Manager` struct,
So that handler registration, LIFO execution order, and panic recovery in handlers are verified.

**Acceptance Criteria:**
- [ ] GIVEN `NewManager()` is called WHEN the manager is returned THEN its handlers slice is empty and non-nil
- [ ] GIVEN `RegisterHandler` is called N times WHEN handlers are inspected THEN exactly N handlers are registered
- [ ] GIVEN `RegisterHandler` is called from multiple goroutines concurrently WHEN all calls complete THEN all handlers are registered without data races
- [ ] GIVEN `executeHandler` is called with a handler that returns nil WHEN executed THEN it returns nil
- [ ] GIVEN `executeHandler` is called with a handler that returns an error WHEN executed THEN it returns that error
- [ ] GIVEN `executeHandler` is called with a handler that panics WHEN executed THEN the panic is recovered and no panic propagates to the caller

**Edge Cases:**
- [ ] `RegisterHandler` with nil function pointer -> does not panic during registration (may panic during execution, which `executeHandler` recovers from)
- [ ] `executeHandler` with a panicking handler -> logged, recovered, returned error is nil (panic recovery does not return an error in the current implementation)
- [ ] Concurrent `RegisterHandler` from 50 goroutines -> all handlers registered, no data races

**Definition of Done:**
- [ ] Test file `alita/utils/shutdown/graceful_test.go` exists
- [ ] Tests do NOT call `WaitForShutdown()` or `shutdown()` (those call `os.Exit`)
- [ ] Tests cover `NewManager`, `RegisterHandler`, and `executeHandler` only
- [ ] All tests use `t.Parallel()`
- [ ] Package coverage >= 70% (excluding `WaitForShutdown` and `shutdown` which call `os.Exit`)

---

### US-004: Test `decorators/misc` Package (Command array helpers)

**Priority:** P0 (must-have)

As a developer,
I want unit tests for `addToArray` and `AddCmdToDisableable`,
So that the thread-safe command registration is verified.

**Acceptance Criteria:**
- [ ] GIVEN `addToArray` is called with a nil slice and one value WHEN executed THEN it returns a slice containing that value
- [ ] GIVEN `addToArray` is called with an existing slice and multiple values WHEN executed THEN all values are appended
- [ ] GIVEN `AddCmdToDisableable` is called with a command string WHEN executed THEN `DisableCmds` contains that command
- [ ] GIVEN `AddCmdToDisableable` is called concurrently from 50 goroutines with unique commands WHEN all calls complete THEN `DisableCmds` contains all 50 commands without data races

**Edge Cases:**
- [ ] `addToArray` with empty string value -> appends empty string
- [ ] `addToArray` with variadic empty args (no values) -> returns original slice unchanged
- [ ] `AddCmdToDisableable` with duplicate command -> command appears twice (append semantics, not set)

**Definition of Done:**
- [ ] Test file `alita/utils/decorators/misc/handler_vars_test.go` exists
- [ ] Concurrency test verifies no data races under `-race`
- [ ] Package coverage >= 90%

---

### US-005: Test `keyword_matcher/cache.go` (Cache management)

**Priority:** P0 (must-have)

As a developer,
I want unit tests for `NewCache`, `GetOrCreateMatcher`, `CleanupExpired`, and `patternsEqual`,
So that the per-chat matcher cache manages lifecycle correctly and the package coverage gap (40%) is closed.

**Acceptance Criteria:**
- [ ] GIVEN `NewCache(ttl)` is called WHEN the cache is returned THEN its matchers map is empty, lastUsed map is empty, and ttl is set to the provided value
- [ ] GIVEN `GetOrCreateMatcher(chatID, patterns)` is called for a new chatID WHEN executed THEN a new matcher is created, stored, and returned
- [ ] GIVEN `GetOrCreateMatcher(chatID, patterns)` is called for an existing chatID with the same patterns WHEN executed THEN the existing matcher is returned (same pointer)
- [ ] GIVEN `GetOrCreateMatcher(chatID, patterns)` is called for an existing chatID with different patterns WHEN executed THEN a new matcher replaces the old one
- [ ] GIVEN `CleanupExpired()` is called WHEN some matchers have exceeded TTL THEN those matchers are removed and unexpired matchers remain
- [ ] GIVEN `patternsEqual(a, b)` is called with identical sets in different order WHEN executed THEN it returns true
- [ ] GIVEN `patternsEqual(a, b)` is called with different sets WHEN executed THEN it returns false

**Edge Cases:**
- [ ] `NewCache(0)` with zero TTL -> all entries expire immediately on `CleanupExpired`
- [ ] `GetOrCreateMatcher` with empty patterns slice -> creates matcher with zero patterns, does not panic
- [ ] `GetOrCreateMatcher` called concurrently for the same chatID -> no data races, one matcher wins
- [ ] `patternsEqual(nil, nil)` -> returns true (both length 0)
- [ ] `patternsEqual([]string{"a", "a"}, []string{"a"})` -> returns false (different lengths)
- [ ] `patternsEqual([]string{"a", "b"}, []string{"a", "a"})` -> returns false ("b" not in second set)
- [ ] `CleanupExpired` with empty cache -> does not panic, no-op

**Definition of Done:**
- [ ] Test file `alita/utils/keyword_matcher/cache_test.go` exists
- [ ] All tests use `t.Parallel()`
- [ ] Package coverage >= 85% (up from 60.2%)

---

### US-006: Test `extraction` Package Pure Functions (`ExtractQuotes`, `IdFromReply`)

**Priority:** P0 (must-have)

As a developer,
I want unit tests for the pure functions `ExtractQuotes` and `IdFromReply` in the extraction package,
So that text parsing logic is verified without requiring Telegram API calls.

**Acceptance Criteria:**
- [ ] GIVEN `ExtractQuotes(sentence, true, false)` with `"\"hello world\" remaining"` WHEN executed THEN `inQuotes="hello world"` and `afterWord="remaining"`
- [ ] GIVEN `ExtractQuotes(sentence, false, true)` with `"firstword rest of text"` WHEN executed THEN `inQuotes="firstword"` and `afterWord="rest of text"`
- [ ] GIVEN `ExtractQuotes("", true, true)` with empty string WHEN executed THEN it returns empty strings without panicking
- [ ] GIVEN `IdFromReply(msg)` with a message that has `ReplyToMessage` set WHEN executed THEN it returns the sender ID from the replied message and remaining command text
- [ ] GIVEN `IdFromReply(msg)` with a message that has nil `ReplyToMessage` WHEN executed THEN it returns `(0, "")`

**Edge Cases:**
- [ ] `ExtractQuotes` with unmatched opening quote (e.g., `"\"hello`) -> returns empty strings (regex does not match)
- [ ] `ExtractQuotes` with `matchQuotes=false` and `matchWord=false` -> returns empty strings
- [ ] `ExtractQuotes` with special characters in quoted text (`"hello & <world>"`) -> preserves special characters
- [ ] `ExtractQuotes` with multiline quoted text (contains `\n`) -> regex uses `(?s)` flag, matches across lines
- [ ] `IdFromReply` with `ReplyToMessage` set but `Text` has no spaces -> returns `(senderID, "")`
- [ ] `IdFromReply` with `ReplyToMessage` containing a forwarded sender -> returns forwarded sender ID

**Definition of Done:**
- [ ] Test file `alita/utils/extraction/extraction_test.go` exists
- [ ] Tests for `ExtractQuotes` use table-driven pattern with at least 8 subtests
- [ ] Tests for `IdFromReply` construct `gotgbot.Message` structs directly (no API calls)
- [ ] Tests require CI env vars (package imports `db` transitively) -- include skip guard for local runs
- [ ] All tests use `t.Parallel()`

---

### US-007: Test `modules` Package Pure Functions (Callback codec wrappers)

**Priority:** P1 (should-have)

As a developer,
I want unit tests for `encodeCallbackData` and `decodeCallbackData` in `alita/modules/callback_codec.go`,
So that the module-level callback encoding/decoding wrappers are verified.

**Acceptance Criteria:**
- [ ] GIVEN `encodeCallbackData` is called with valid namespace and fields WHEN executed THEN it returns the encoded callback string matching `callbackcodec.Encode` output
- [ ] GIVEN `encodeCallbackData` is called and `callbackcodec.Encode` would return an error (e.g., empty namespace) WHEN executed THEN it returns the fallback string
- [ ] GIVEN `decodeCallbackData` is called with valid encoded data and no expected namespaces WHEN executed THEN it returns the decoded struct and `true`
- [ ] GIVEN `decodeCallbackData` is called with valid data but a non-matching namespace WHEN executed THEN it returns `nil` and `false`
- [ ] GIVEN `decodeCallbackData` is called with invalid/malformed data WHEN executed THEN it returns `nil` and `false`

**Edge Cases:**
- [ ] `encodeCallbackData` with nil fields map -> verify no panic
- [ ] `decodeCallbackData` with empty string data -> returns `nil, false`
- [ ] `decodeCallbackData` with case-insensitive namespace matching -> verifies `strings.EqualFold` behavior (e.g., "NS" matches "ns")
- [ ] `encodeCallbackData` with empty fallback string -> returns `""` on encode failure

**Definition of Done:**
- [ ] Tests added to a test file in `alita/modules/`
- [ ] Tests require CI env vars -- include skip guard
- [ ] At least 6 subtests covering happy path, error path, and edge cases
- [ ] All tests use `t.Parallel()`

---

### US-008: Test DB Cache Key Generators (Pure functions in `cache_helpers.go`)

**Priority:** P0 (must-have)

As a developer,
I want unit tests for all 8 cache key generator functions in `alita/db/cache_helpers.go`,
So that cache key formatting is verified and regressions in key format are caught.

**Acceptance Criteria:**
- [ ] GIVEN `chatSettingsCacheKey(12345)` is called WHEN executed THEN it returns `"alita:chat_settings:12345"`
- [ ] GIVEN each of the 8 cache key functions is called with a known ID WHEN executed THEN the returned string matches `"alita:{segment}:{id}"` format
- [ ] GIVEN any cache key function is called with `0` WHEN executed THEN it returns `"alita:{segment}:0"` (zero is a valid ID for testing)
- [ ] GIVEN any cache key function is called with a negative channel ID like `-1001234567890` WHEN executed THEN it returns `"alita:{segment}:-1001234567890"`

**Edge Cases:**
- [ ] All 8 key functions tested: `chatSettingsCacheKey`, `userLanguageCacheKey`, `chatLanguageCacheKey`, `filterListCacheKey`, `blacklistCacheKey`, `warnSettingsCacheKey`, `disabledCommandsCacheKey`, `captchaSettingsCacheKey`
- [ ] Key format consistency: all keys SHALL start with `"alita:"` prefix
- [ ] No two key functions SHALL produce the same output for the same input ID -> verify segment names are distinct

**Definition of Done:**
- [ ] Tests added to `alita/db/cache_helpers_test.go`
- [ ] Tests require CI env vars (package imports `config`) -- include skip guard
- [ ] Table-driven test covering all 8 functions with at least 3 input values each
- [ ] All tests use `t.Parallel()`

---

### US-009: Add `TestMain` to `alita/db/` for Shared Initialization

**Priority:** P1 (should-have)

As a developer,
I want a `TestMain` function in the `alita/db/` package,
So that all DB tests share a one-time `AutoMigrate` call and skip cleanly when PostgreSQL is unavailable.

**Acceptance Criteria:**
- [ ] GIVEN `TestMain` exists in `alita/db/` WHEN PostgreSQL is available THEN it runs `DB.AutoMigrate()` for all models once before any test executes
- [ ] GIVEN `TestMain` exists WHEN PostgreSQL is unavailable (`DB == nil`) THEN all DB tests are skipped with a clear message printed to stderr
- [ ] GIVEN `TestMain` handles migration WHEN individual test files no longer call `AutoMigrate()` THEN no duplicate migration calls occur

**Edge Cases:**
- [ ] `TestMain` called with `DB == nil` -> prints skip message, calls `os.Exit(0)` to skip all tests gracefully
- [ ] `DB.AutoMigrate()` fails (e.g., permission issue) -> `TestMain` calls `log.Fatalf` with clear error, test suite does not proceed with partial schema

**Definition of Done:**
- [ ] File `alita/db/testmain_test.go` (or similar) exists with `func TestMain(m *testing.M)`
- [ ] Contains `AutoMigrate` for all GORM models
- [ ] Existing DB test files (`captcha_db_test.go`, `locks_db_test.go`, etc.) no longer duplicate `AutoMigrate` calls
- [ ] All DB tests pass in CI and are skipped locally without PostgreSQL

---

### US-010: Test DB CRUD Operations -- Greetings (15 functions)

**Priority:** P1 (should-have)

As a developer,
I want integration tests for all greeting DB functions,
So that the most complex DB module (380 LOC, 15 functions) is verified.

**Acceptance Criteria:**
- [ ] GIVEN `GetGreetingSettings(chatID)` for a non-existent chat WHEN the chat does not exist in the `chats` table THEN default settings are returned with `ShouldWelcome=true`, `WelcomeText=DefaultWelcome`, `ShouldGoodbye=false`
- [ ] GIVEN `SetWelcomeText(chatID, text, fileId, buttons, type)` WHEN the greeting record exists THEN `GetGreetingSettings` returns the updated welcome text, file ID, buttons, and type
- [ ] GIVEN `SetWelcomeToggle(chatID, false)` WHEN the setting was previously true THEN `GetGreetingSettings` returns `ShouldWelcome=false` (zero-value boolean round-trip verified)
- [ ] GIVEN `SetGoodbyeText(chatID, ...)` and `SetGoodbyeToggle(chatID, true)` WHEN retrieved THEN goodbye settings reflect the updates
- [ ] GIVEN `SetShouldCleanService(chatID, true)` WHEN retrieved THEN `ShouldCleanService=true`
- [ ] GIVEN `SetShouldAutoApprove(chatID, true)` WHEN retrieved THEN `ShouldAutoApprove=true`
- [ ] GIVEN `SetCleanWelcomeSetting` and `SetCleanWelcomeMsgId` WHEN retrieved THEN `CleanWelcome` and `LastMsgId` are correct
- [ ] GIVEN `SetCleanGoodbyeSetting` and `SetCleanGoodbyeMsgId` WHEN retrieved THEN `CleanGoodbye` and `LastMsgId` are correct
- [ ] GIVEN `GetWelcomeButtons(chatID)` after setting welcome text with buttons WHEN retrieved THEN the correct buttons are returned
- [ ] GIVEN `GetGoodbyeButtons(chatID)` after setting goodbye text with buttons WHEN retrieved THEN the correct buttons are returned
- [ ] GIVEN `LoadGreetingsStats()` with multiple chats having different settings WHEN called THEN correct aggregate counts are returned

**Edge Cases:**
- [ ] Zero-value boolean toggling: `SetWelcomeToggle(chatID, false)` after `true` -> `ShouldWelcome=false`
- [ ] Empty welcome text: `SetWelcomeText(chatID, "", ...)` -> `checkGreetingSettings` returns `DefaultWelcome` for empty text
- [ ] Concurrent `SetWelcomeText` and `SetGoodbyeText` for the same chat -> no data corruption (verified by `-race`)
- [ ] `GetWelcomeButtons` for chat with no buttons set -> returns empty slice, not nil
- [ ] `LoadGreetingsStats` with empty database -> returns all zeros

**Definition of Done:**
- [ ] Test file `alita/db/greetings_db_test.go` exists
- [ ] Tests rely on `TestMain` (US-009) for schema setup, skip if `DB == nil`
- [ ] Tests use `time.Now().UnixNano()` for unique chat IDs and `t.Cleanup()` for data removal
- [ ] All 15 functions have at least one test case
- [ ] Tests use `t.Parallel()` at the top-level test function

---

### US-011: Test DB CRUD Operations -- Warns (13 functions)

**Priority:** P1 (should-have)

As a developer,
I want integration tests for all warn DB functions,
So that the warn system (critical for moderation) is verified.

**Acceptance Criteria:**
- [ ] GIVEN `checkWarnSettings(chatID)` for a new chat WHEN the chat exists THEN default settings are created with `WarnLimit=3` and `WarnMode="mute"`
- [ ] GIVEN a user is warned WHEN the warn is retrieved THEN the warn record contains the correct user ID, chat ID, reason, and warner ID
- [ ] GIVEN a user has N warns and `WarnLimit` is N WHEN another warn is added THEN the function signals that the limit is reached
- [ ] GIVEN a warn is removed WHEN the warn count is checked THEN it reflects the removal
- [ ] GIVEN `ResetWarns(chatID, userID)` WHEN all warns are retrieved THEN zero warns exist for that user in that chat
- [ ] GIVEN warn settings are updated (limit, mode) WHEN retrieved THEN the updated values are returned

**Edge Cases:**
- [ ] Warn with empty reason string -> stores and retrieves correctly
- [ ] `ResetWarns` for a user with zero warns -> no error, no-op
- [ ] Concurrent warn creation for the same user/chat -> no duplicate IDs, correct count
- [ ] Warn limit set to 0 -> should handle gracefully, not cause infinite warns
- [ ] Warn mode set to invalid string -> stores as-is (validation at handler level)

**Definition of Done:**
- [ ] Test file `alita/db/warns_db_test.go` exists
- [ ] Tests rely on `TestMain` (US-009), skip if `DB == nil`
- [ ] All CRUD functions tested with at least one happy-path and one edge case
- [ ] `t.Cleanup()` removes all test data

---

### US-012: Test DB CRUD Operations -- Notes (11 functions)

**Priority:** P1 (should-have)

As a developer,
I want integration tests for all notes DB functions,
So that the notes storage system is verified.

**Acceptance Criteria:**
- [ ] GIVEN `getNotesSettings(chatID)` for a new chat WHEN chat exists THEN default settings with `Private=false` are created
- [ ] GIVEN a note is saved with name, text, buttons, and type WHEN retrieved by name THEN all fields match
- [ ] GIVEN multiple notes are saved for a chat WHEN `GetAllNotes(chatID)` is called THEN all notes are returned
- [ ] GIVEN a note is deleted by name WHEN retrieved THEN it returns not-found
- [ ] GIVEN notes private mode is toggled WHEN retrieved THEN the toggle state is correct

**Edge Cases:**
- [ ] Note with empty name -> stores (validation at handler level)
- [ ] Note with very long text (> 4096 chars) -> stores without truncation at DB level
- [ ] Duplicate note name for the same chat -> updates existing (upsert behavior)
- [ ] `GetAllNotes` for chat with zero notes -> returns empty slice, not nil
- [ ] Delete non-existent note -> no error, no-op

**Definition of Done:**
- [ ] Test file `alita/db/notes_db_test.go` exists
- [ ] Tests rely on `TestMain` (US-009), skip if `DB == nil`
- [ ] All 11 functions have at least one test case
- [ ] `t.Cleanup()` removes all test data

---

### US-013: Test DB CRUD Operations -- Filters (7 functions)

**Priority:** P1 (should-have)

As a developer,
I want integration tests for all filter DB functions,
So that the filter/autoresponse storage is verified.

**Acceptance Criteria:**
- [ ] GIVEN a filter is saved with keyword, reply text, and type WHEN retrieved by keyword THEN all fields match
- [ ] GIVEN `GetAllFilters(chatID)` WHEN multiple filters exist THEN all are returned
- [ ] GIVEN a filter is deleted by keyword WHEN retrieved THEN it returns not-found
- [ ] GIVEN `FilterExists(chatID, keyword)` for an existing filter WHEN called THEN it returns true
- [ ] GIVEN cache invalidation occurs on filter write WHEN the same filter is read THEN fresh data is returned (not stale cache)

**Edge Cases:**
- [ ] Filter with special characters in keyword (regex metacharacters like `.*+?`) -> stores and retrieves correctly
- [ ] Filter with empty reply text -> stores as-is
- [ ] Concurrent filter creation for the same chat/keyword -> no data corruption
- [ ] `GetAllFilters` for chat with zero filters -> returns empty slice

**Definition of Done:**
- [ ] Test file `alita/db/filters_db_test.go` exists
- [ ] Tests rely on `TestMain` (US-009), skip if `DB == nil`
- [ ] All 7 functions tested
- [ ] `t.Cleanup()` removes all test data

---

### US-014: Test DB CRUD Operations -- Remaining 12 Files

**Priority:** P1 (should-have)

As a developer,
I want integration tests for the remaining 12 untested DB files,
So that all DB CRUD operations have test coverage.

**Files:** `admin_db.go` (3 functions), `blacklists_db.go` (6), `channels_db.go` (6), `chats_db.go` (6), `connections_db.go` (8), `devs_db.go` (7), `disable_db.go` (9), `lang_db.go` (5), `pin_db.go` (4), `reports_db.go` (7), `rules_db.go` (6), `user_db.go` (8).

**Acceptance Criteria:**
- [ ] GIVEN each DB file's functions WHEN tested with the existing pattern (unique IDs, `t.Cleanup`) THEN all CRUD operations work correctly for create, read, update, and delete paths
- [ ] GIVEN `admin_db.go` WHEN tested THEN admin cache list and related functions are verified
- [ ] GIVEN `blacklists_db.go` WHEN tested THEN blacklist keyword CRUD and chat blacklist mode are verified
- [ ] GIVEN `channels_db.go` WHEN tested THEN channel registration and lookup by ID/username are verified
- [ ] GIVEN `chats_db.go` WHEN tested THEN chat registration, existence check (`ChatExists`), and listing are verified
- [ ] GIVEN `connections_db.go` WHEN tested THEN user-to-chat connections (connect, disconnect, retrieval) are verified
- [ ] GIVEN `devs_db.go` WHEN tested THEN developer/sudo user management is verified
- [ ] GIVEN `disable_db.go` WHEN tested THEN command disable/enable per chat is verified including zero-value boolean round-trip
- [ ] GIVEN `lang_db.go` WHEN tested THEN language preference storage for users and chats is verified
- [ ] GIVEN `pin_db.go` WHEN tested THEN pin message tracking is verified
- [ ] GIVEN `reports_db.go` WHEN tested THEN report settings and storage are verified
- [ ] GIVEN `rules_db.go` WHEN tested THEN chat rules CRUD and private-rules toggle are verified
- [ ] GIVEN `user_db.go` WHEN tested THEN user registration, lookup by ID/username, and info retrieval are verified

**Edge Cases (applies to all 12 files):**
- [ ] Zero-value boolean fields (e.g., `false` toggles) are persisted and retrieved correctly (GORM zero-value gotcha -- must use `map[string]any` for updates)
- [ ] Non-existent record lookups return appropriate defaults or errors, not panics
- [ ] Concurrent create operations for the same entity -> exactly one succeeds or both succeed idempotently (unique constraint behavior)
- [ ] Delete of non-existent record -> no error, no-op
- [ ] Lookup by username with and without `@` prefix -> consistent behavior

**Definition of Done:**
- [ ] One test file per DB file (e.g., `admin_db_test.go`, `blacklists_db_test.go`, etc.)
- [ ] All test files rely on `TestMain` (US-009), skip if `DB == nil`
- [ ] Each function has at least one happy-path test and one error/edge-case test
- [ ] `t.Cleanup()` removes all test data
- [ ] All tests use `time.Now().UnixNano()` for unique IDs

---

### US-015: Expand Coverage on Existing DB Test Files

**Priority:** P1 (should-have)

As a developer,
I want to expand test coverage for the 4 DB files that already have partial tests (`captcha_db.go`, `locks_db.go`, `antiflood_db.go`, `update_record.go`),
So that untested functions in these files are covered.

**Acceptance Criteria:**
- [ ] GIVEN `captcha_db.go` has functions beyond the 2 tested WHEN new tests are added THEN all public functions (`CreateCaptchaAttemptPreMessage`, `GetCaptchaSettings`, `SetCaptchaEnabled`, `SetCaptchaMode`, `SetCaptchaTimeout`, `SetCaptchaMaxAttempts`, `SetCaptchaFailureAction`, `GetCaptchaAttemptByID`, etc.) have at least one test case
- [ ] GIVEN `locks_db.go` has functions beyond the 4 tested WHEN new tests are added THEN all `GetLock*`, `UpdateLock`, and `GetAllLocks` functions are tested
- [ ] GIVEN `antiflood_db.go` has functions beyond the 3 tested WHEN new tests are added THEN all `GetFlood`, `SetFlood`, `GetFloodMsgDel`, `SetFloodMsgDel` functions are tested

**Edge Cases:**
- [ ] All existing tests continue to pass after new tests are added
- [ ] New tests do not interfere with existing test data (use unique IDs via `time.Now().UnixNano()`)

**Definition of Done:**
- [ ] All public functions in `captcha_db.go`, `locks_db.go`, and `antiflood_db.go` have test coverage
- [ ] No regressions in existing tests
- [ ] Tests follow established patterns

---

### US-016: Test `monitoring/auto_remediation.go` Pure Functions

**Priority:** P2 (nice-to-have)

As a developer,
I want unit tests for the `RemediationAction` implementations' `CanExecute`, `Name`, and `Severity` methods,
So that threshold-based decision logic is verified.

**Acceptance Criteria:**
- [ ] GIVEN `GCAction.CanExecute(metrics)` with `MemoryAllocMB` above 60% of `ResourceMaxMemoryMB` WHEN called THEN it returns true
- [ ] GIVEN `GCAction.CanExecute(metrics)` with `MemoryAllocMB` below 60% threshold and `GCPauseMs` below 50 WHEN called THEN it returns false
- [ ] GIVEN `MemoryCleanupAction.CanExecute(metrics)` with `MemoryAllocMB` above `ResourceGCThresholdMB` WHEN called THEN it returns true
- [ ] GIVEN `LogWarningAction.CanExecute(metrics)` with goroutines above 80% of max WHEN called THEN it returns true
- [ ] GIVEN `RestartRecommendationAction.CanExecute(metrics)` with resources above 150% of max WHEN called THEN it returns true
- [ ] GIVEN each action's `Name()` WHEN called THEN it returns the expected string identifier
- [ ] GIVEN each action's `Severity()` WHEN called THEN LogWarning=0 < GC=1 < MemoryCleanup=2 < RestartRecommendation=10

**Edge Cases:**
- [ ] `CanExecute` with zero-value `SystemMetrics` -> all actions return false (thresholds are above zero)
- [ ] `CanExecute` with `MemoryAllocMB` exactly at threshold boundary -> verify inclusive/exclusive behavior
- [ ] `CanExecute` with negative metric values -> returns false (below threshold)

**Definition of Done:**
- [ ] Test file `alita/utils/monitoring/auto_remediation_test.go` exists
- [ ] Tests require CI env vars (package imports `config`) -- include skip guard
- [ ] All 4 actions tested for `CanExecute`, `Name`, and `Severity`
- [ ] Tests SHALL NOT call `Execute` (which triggers actual GC/runtime operations)
- [ ] All tests use `t.Parallel()`

---

### US-017: Test `monitoring/background_stats.go` Non-Infrastructure Functions

**Priority:** P2 (nice-to-have)

As a developer,
I want unit tests for the `BackgroundStatsCollector` atomic counter methods and lifecycle creation,
So that the stats recording API is verified.

**Acceptance Criteria:**
- [ ] GIVEN `NewBackgroundStatsCollector()` WHEN called THEN counter fields are zero and intervals are defaults (30s system, 1m db, 5m reporting)
- [ ] GIVEN `RecordMessage()` is called N times concurrently WHEN counter is read THEN `messageCounter >= N`
- [ ] GIVEN `RecordError()` is called N times concurrently WHEN counter is read THEN `errorCounter >= N`
- [ ] GIVEN `RecordResponseTime(duration)` is called WHEN metrics are read THEN `responseTimeSum` and `responseTimeCount` reflect the recordings
- [ ] GIVEN `GetCurrentMetrics()` is called from multiple goroutines concurrently WHEN executed THEN no data races occur

**Edge Cases:**
- [ ] `RecordResponseTime(0)` -> does not cause divide-by-zero when computing average
- [ ] `GetCurrentMetrics` before any stats collection -> returns zero-value `SystemMetrics`
- [ ] `Stop()` called twice -> second call is a no-op (`sync.Once`)

**Definition of Done:**
- [ ] Test file `alita/utils/monitoring/background_stats_test.go` exists
- [ ] Tests require CI env vars (package imports `db`) -- include skip guard
- [ ] Tests SHALL NOT call `Start()` or `Stop()` with live goroutines (test counter methods and `GetCurrentMetrics` only)
- [ ] All tests use `t.Parallel()`

---

### US-018: Expand `i18n` Package Tests

**Priority:** P1 (should-have)

As a developer,
I want the existing 10 tests in `i18n_test.go` to pass AND additional tests for `Translator` methods and `LocaleManager` methods,
So that the internationalization layer has comprehensive coverage.

**Acceptance Criteria:**
- [ ] GIVEN the existing 10 tests WHEN run with CI env vars THEN all pass
- [ ] GIVEN `Translator.Get("existing_key")` WHEN called THEN it returns the translated string
- [ ] GIVEN `Translator.Get("nonexistent_key")` WHEN called THEN it returns a fallback or error indicator
- [ ] GIVEN `Translator.Get("key_with_params", params)` WHEN called THEN named parameters are substituted
- [ ] GIVEN `LocaleManager.GetTranslator("en")` WHEN called THEN it returns the English translator
- [ ] GIVEN `LocaleManager.GetTranslator("nonexistent_locale")` WHEN called THEN it returns default translator or error
- [ ] GIVEN `LocaleManager.GetAvailableLocales()` WHEN called THEN it returns at least `["en", "es", "fr", "hi"]`

**Edge Cases:**
- [ ] `Translator.Get` with nil params map -> no panic
- [ ] `Translator.Get` with more params than placeholders -> extra params ignored
- [ ] `Translator.Get` with fewer params than placeholders -> remaining placeholders unsubstituted or error
- [ ] `selectPluralForm` with count=0, count=1, count=2 -> correct plural form selected

**Definition of Done:**
- [ ] Additional tests added to `alita/i18n/i18n_test.go`
- [ ] Tests require CI env vars -- include skip guard
- [ ] At least 5 new test cases beyond existing 10
- [ ] Package coverage >= 60%

---

### US-019: Expand `helpers` Package Tests

**Priority:** P1 (should-have)

As a developer,
I want the existing 28 tests in `helpers_test.go` to pass AND additional tests for remaining pure functions,
So that the helpers package achieves comprehensive coverage.

**Acceptance Criteria:**
- [ ] GIVEN the existing 28 tests WHEN run with CI env vars THEN all pass
- [ ] GIVEN `Shtml(text)` WHEN called THEN it returns the correct HTML parse mode opts
- [ ] GIVEN `Smarkdown(text)` WHEN called THEN it returns the correct Markdown parse mode opts
- [ ] GIVEN `GetMessageLinkFromMessageId(chatID, messageID)` with a supergroup chat ID WHEN called THEN it returns the correct `https://t.me/c/...` URL
- [ ] GIVEN `GetLangFormat(langCode)` with "en" WHEN called THEN it returns the English locale display format
- [ ] GIVEN `ExtractJoinLeftStatusChange(update)` with a join event WHEN called THEN it correctly identifies the join
- [ ] GIVEN `ExtractAdminUpdateStatusChange(update)` with a promotion event WHEN called THEN it correctly identifies the admin promotion

**Edge Cases:**
- [ ] `GetMessageLinkFromMessageId` with private chat ID -> correct URL format
- [ ] `GetMessageLinkFromMessageId` with message ID 0 -> valid URL with 0
- [ ] `ExtractJoinLeftStatusChange` with nil `ChatMemberUpdated` -> returns appropriate zero-value
- [ ] `ExtractAdminUpdateStatusChange` with demotion (admin -> member) -> identified correctly

**Definition of Done:**
- [ ] Additional tests added to `alita/utils/helpers/helpers_test.go`
- [ ] Tests require CI env vars -- include skip guard
- [ ] At least 8 new test cases
- [ ] Tests construct `gotgbot` structs directly (no API calls)
- [ ] All tests use `t.Parallel()`

---

### US-020: Test DB `migrations.go` Pure Functions

**Priority:** P2 (nice-to-have)

As a developer,
I want unit tests for the SQL cleaning functions and `SchemaMigration.TableName()` in `migrations.go`,
So that Supabase-specific SQL cleaning is verified.

**Acceptance Criteria:**
- [ ] GIVEN a SQL string containing `GRANT ... TO anon` WHEN cleaned THEN the GRANT line is removed
- [ ] GIVEN a SQL string containing `with schema "extensions"` WHEN cleaned THEN it is removed
- [ ] GIVEN a SQL string with `create extension if not exists` WHEN cleaned THEN it is uppercased to `CREATE EXTENSION IF NOT EXISTS`
- [ ] GIVEN a clean SQL string with no Supabase-specific syntax WHEN cleaned THEN it is returned unchanged
- [ ] GIVEN `SchemaMigration.TableName()` WHEN called THEN it returns `"schema_migrations"`

**Edge Cases:**
- [ ] Empty SQL string -> returned empty
- [ ] SQL with multiple GRANT lines -> all removed
- [ ] SQL with GRANT in a SQL comment (`-- GRANT ...`) -> behavior documented (current regex may remove)

**Definition of Done:**
- [ ] Test file `alita/db/migrations_test.go` exists
- [ ] Tests require CI env vars -- include skip guard
- [ ] SQL cleaning functions tested with at least 5 input variations
- [ ] `TableName()` method tested

---

### US-021: Add Coverage Threshold Enforcement to CI

**Priority:** P1 (should-have)

As a developer,
I want CI to fail if overall test coverage drops below a configured threshold,
So that coverage regressions are caught before merge.

**Acceptance Criteria:**
- [ ] GIVEN `coverage.out` is generated by `make test` WHEN a CI step parses it THEN it extracts the total coverage percentage
- [ ] GIVEN the total coverage is below the threshold (initially 40%) WHEN the CI step evaluates THEN the CI job fails with a clear message showing actual vs. required coverage
- [ ] GIVEN the total coverage is at or above the threshold WHEN the CI step evaluates THEN the CI job passes
- [ ] GIVEN the threshold is configurable WHEN a developer updates it THEN only the threshold value needs to change (single source of truth)

**Edge Cases:**
- [ ] `coverage.out` is empty or missing -> CI step fails with "no coverage data" message, not a silent pass
- [ ] Coverage is exactly at threshold (e.g., 40.0%) -> passes (inclusive comparison)
- [ ] Coverage output contains `[no test files]` for some packages -> those are excluded from the total calculation

**Definition of Done:**
- [ ] CI workflow `.github/workflows/ci.yml` contains a coverage threshold check step after test execution
- [ ] Threshold starts at 40% (achievable after P0 + P1 work)
- [ ] Coverage percentage is printed in the CI job summary
- [ ] The check uses `go tool cover -func=coverage.out` to parse total coverage

---

### US-022: Test `config/types.go` Expansion

**Priority:** P2 (nice-to-have)

As a developer,
I want the existing 5 test functions (~30 subtests) in `config/types_test.go` to pass and have additional edge cases for `LoadConfig`,
So that configuration loading edge cases are covered.

**Acceptance Criteria:**
- [ ] GIVEN the existing 5 test functions WHEN run with CI env vars THEN all pass
- [ ] GIVEN `LoadConfig()` is called with all required env vars set WHEN executed THEN it returns a valid `Config` struct with no error
- [ ] GIVEN `LoadConfig()` is called with `BOT_TOKEN` unset WHEN executed THEN it returns an error

**Edge Cases:**
- [ ] `LoadConfig` with empty string `BOT_TOKEN=""` -> verify behavior (error or accepted)
- [ ] Boolean env vars with mixed case ("TRUE", "True", "true") -> all accepted

**Definition of Done:**
- [ ] Additional tests added to `alita/config/` test files
- [ ] Tests require CI env vars
- [ ] At least 3 new test cases

---

### US-023: Verify Existing `tracing` and `cache/sanitize` Tests Pass

**Priority:** P2 (nice-to-have)

As a developer,
I want the existing 10 tracing tests and 18 cache/sanitize subtests to pass in CI,
So that these packages contribute to measured coverage.

**Acceptance Criteria:**
- [ ] GIVEN the existing `context_test.go` (6 tests) and `processor_test.go` (4 tests) WHEN run with CI env vars THEN all pass
- [ ] GIVEN the existing `sanitize_test.go` (2 functions, ~18 subtests) WHEN run with CI env vars THEN all pass
- [ ] GIVEN coverage is measured THEN both packages show non-zero coverage

**Definition of Done:**
- [ ] Existing tests pass in CI
- [ ] No new tests required (existing coverage is already solid for these packages)

---

### US-024: Test `chat_status` Package ID Validation

**Priority:** P1 (should-have)

As a developer,
I want the existing 2 test functions (~14 subtests) in `chat_status_test.go` to pass,
So that ID validation logic is verified.

**Acceptance Criteria:**
- [ ] GIVEN the existing `IsValidUserId` and `IsChannelId` tests WHEN run with CI env vars THEN all pass
- [ ] GIVEN any additional pure functions in `chat_status.go` that do not call Telegram API WHEN identified THEN tests are added

**Edge Cases:**
- [ ] Already covered by existing tests (positive IDs, zero, negative, channel IDs, boundary values, Telegram system IDs)

**Definition of Done:**
- [ ] Existing tests pass in CI
- [ ] Any newly discovered pure functions without API dependencies are tested

## Non-Functional Requirements

### NFR-001: Test Execution Time

- **Metric:** Full test suite (`make test`) SHALL complete in under 5 minutes in CI (excluding PostgreSQL startup and Go compilation)
- **Verification:** CI job summary prints duration; SHALL not exceed 300 seconds for test execution

### NFR-002: Test Conventions -- stdlib Only

- **Metric:** 100% of new test files SHALL use stdlib `testing` only. Zero new test framework dependencies.
- **Verification:** `go list -m all` shows no new test-only modules; no imports of testify, gomock, gocheck in any `_test.go` file

### NFR-003: Test Parallelism

- **Metric:** 100% of new test functions and subtests SHALL call `t.Parallel()` where safe (no shared mutable global state mutation in test body)
- **Verification:** All new test files contain `t.Parallel()` calls; `-race` flag detects no races

### NFR-004: Table-Driven Test Pattern

- **Metric:** 100% of new test functions with 3+ test cases SHALL use table-driven pattern with `t.Run` subtests
- **Verification:** Code review confirms `tests := []struct{...}` pattern

### NFR-005: Test Data Isolation

- **Metric:** 100% of DB integration tests SHALL use unique IDs (via `time.Now().UnixNano()`) and `t.Cleanup()` for data removal
- **Verification:** No DB test uses hardcoded IDs; all tests include `t.Cleanup` block

### NFR-006: Race Condition Detection

- **Metric:** `go test -race ./...` SHALL pass with zero data races for all new and existing tests
- **Verification:** CI runs `make test` which includes `-race` flag; zero race conditions reported

### NFR-007: Coverage Targets

- **Metric:** Overall coverage SHALL reach 40% after P0 + P1 work is complete, and 55% after all P0 + P1 + P2 work is complete
- **Verification:** `go tool cover -func=coverage.out | tail -1` shows total coverage >= target; CI step enforces threshold per US-021

### NFR-008: No New Module Dependencies

- **Metric:** Zero new Go module dependencies SHALL be added solely for testing purposes
- **Verification:** `go.mod` diff shows no new `require` entries added for test frameworks or test utilities

### NFR-009: Graceful Skip for Missing Infrastructure

- **Metric:** Tests requiring PostgreSQL SHALL skip (not fail) when database is unreachable. Tests requiring env vars SHALL skip (not crash) when run without them.
- **Verification:** Running `go test ./alita/db/...` locally without PostgreSQL produces skip messages, exit code 0

## Dependencies

| Dependency | Required By | Risk if Unavailable |
|-----------|------------|-------------------|
| PostgreSQL 16 in CI | US-009 through US-015, US-020 | DB tests cannot run; ~20-25% coverage gain blocked |
| `BOT_TOKEN=test-token` env var in CI | US-001, all packages importing `config` transitively | 8 packages crash; ~15% coverage blocked |
| `DATABASE_URL` env var in CI | US-009 through US-015 | DB connection fails; same as PostgreSQL unavailable |
| `GORM v1.31.1` (existing) | US-009 through US-015, US-020 | All DB tests; zero risk (already in go.mod) |
| `gotgbot/v2 v2.0.0-rc.33` (existing) | US-006, US-019 | Pure struct construction; zero risk (already in go.mod) |
| `logrus v1.9.4` (existing) | US-002 | Log verification; zero risk |
| CI PostgreSQL health check | US-009 through US-015 | Tests may start before DB ready; mitigated by existing `pg_isready` loop in CI |
| US-009 (TestMain) | US-010 through US-015 | DB tests without shared init will duplicate AutoMigrate calls; functional but wasteful |

## Assumptions

1. **CI continues to set `BOT_TOKEN=test-token`, `OWNER_ID=1`, `MESSAGE_DUMP=1`** -- if removed, all packages importing `config` crash. Impact: US-001 and all downstream blocked.

2. **`config.init()` remains `log.Fatalf` on missing BOT_TOKEN** -- if refactored to lazy init, env var requirements for testing relax. Impact: skip guards become unnecessary.

3. **DB tests only expected to pass in CI, not locally without PostgreSQL** -- if local execution is required, Docker Compose or testcontainers needed. Impact: new dependency.

4. **`gotgbot` types remain plain structs** -- if changed to interfaces, test construction patterns change. Impact: US-006, US-019.

5. **No Redis service added to CI** -- cache tests use nil-guard pattern. Impact: cache integration tests remain out of scope.

6. **Handler-level testing remains out of scope** until a `BotAPI` interface is introduced -- separate design decision. Impact: ~250 handler functions remain untested.

7. **`go test -race` produces no false positives** for existing code -- if it does, those must be fixed first. Impact: NFR-006 baseline.

## Open Questions

- [ ] **Target coverage percentage confirmation** -- This document uses 40% (P0+P1) and 55% (full). Should a different threshold be used? Blocks: US-021 threshold value.
- [ ] **Build tags vs. `t.Skip` for DB tests** -- Should `//go:build integration` tags be added so `go test ./...` does not even compile DB tests locally? Or is `t.Skip` in `TestMain` sufficient? Blocks: US-009 approach.
- [ ] **Codecov integration** -- Should coverage be tracked over time via Codecov/Coveralls? Currently `coverage.out` is an artifact only. Blocks: US-021 reporting scope.
- [ ] **TestMain vs. per-test AutoMigrate** -- Should new DB tests use `TestMain` (US-009) from the start, or write with per-test `AutoMigrate` first and consolidate later? Blocks: US-010 through US-014 implementation approach.
- [ ] **Monitoring `CanExecute` testability** -- `auto_remediation.go` reads `config.AppConfig` in `CanExecute`. Should thresholds be refactored to accept parameters (pure function), or test with CI env vars as-is? Blocks: US-016 test approach.

## Glossary

| Term | Definition |
|------|-----------|
| Pure function | A function that depends only on its input parameters and has no side effects (no DB, no API, no global state mutation) |
| Tier 1 | Packages with zero external dependencies; trivially testable with no setup |
| Tier 2 | Packages testable by constructing gotgbot structs directly (plain data, no API) |
| Tier 3 | Packages requiring live PostgreSQL for integration tests (DB CRUD) |
| Tier 4 | Packages requiring Telegram Bot API mock/interface or live API (handlers, permissions) |
| Config init crash | `log.Fatalf` in `alita/config/config.go:init()` that kills test process when `BOT_TOKEN` missing |
| Nil-guard pattern | `if cache.Marshal != nil` before cache operations, allowing tests without Redis |
| Table-driven test | Go pattern: `tests := []struct{...}` with `t.Run(tc.name, func(t *testing.T) {...})` |
| Zero-value boolean | GORM does not update `false` with `.Updates()` struct; requires `map[string]any` |
| Singleflight | `golang.org/x/sync/singleflight` for deduplicating concurrent cache loads |
| Skip guard | `t.Skip("reason")` at test start to skip when prerequisites unavailable |
| Coverage threshold | Minimum coverage enforced by CI; pipeline fails if coverage drops below value |
| TestMain | `func TestMain(m *testing.M)` -- Go's per-package test setup/teardown hook |
| LIFO | Last In, First Out -- shutdown handler execution order |

REQUIREMENTS_COMPLETE
