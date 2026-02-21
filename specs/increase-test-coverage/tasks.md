# Implementation Tasks: increase-test-coverage

**Date:** 2026-02-21
**Design Source:** design.md
**Total Tasks:** 18
**Slicing Strategy:** vertical (each task = complete feature slice)

---

## TASK-001: Test error_handling package (3 pure functions)

**Complexity:** S
**Files:**
- CREATE: `alita/utils/error_handling/error_handling_test.go`
**Dependencies:** None
**Description:**
Create a test file for the `error_handling` package covering all 3 functions: `HandleErr`, `RecoverFromPanic`, and `CaptureError`. This package has zero external dependencies beyond logrus -- it is trivially testable without any env vars or infrastructure.

Test functions to implement:
- `TestHandleErr` -- table-driven with subtests: nil error (no-op), non-nil error (logs, no panic), wrapped error
- `TestRecoverFromPanic` -- subtests: recovers from panic in goroutine (use `done` channel to confirm goroutine started, then panic), no-op when no panic (just defer and return), empty funcName/modName strings
- `TestCaptureError` -- table-driven: nil error (returns immediately), non-nil error with tags, nil tags map, empty tags map
- `TestHandleErrConcurrent` -- spawn 50 goroutines calling `HandleErr` with `sync.WaitGroup`, verify no data races under `-race`

All tests must use `t.Parallel()` on both the top-level test and each subtest. Use table-driven pattern with `t.Run`. No external dependencies needed.

**Context to Read:**
- design.md, section "Component: error_handling tests" -- exact test function signatures and implementation sketch
- `/Users/divkix/GitHub/Alita_Robot/alita/utils/error_handling/error_handling.go` -- the 3 functions to test (HandleErr, RecoverFromPanic, CaptureError)

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && go test -v -race ./alita/utils/error_handling/...
```

---

## TASK-002: Test shutdown package (Manager lifecycle)

**Complexity:** S
**Files:**
- CREATE: `alita/utils/shutdown/graceful_test.go`
**Dependencies:** None
**Description:**
Create a test file for the `shutdown` package covering `NewManager`, `RegisterHandler`, and `executeHandler`. CRITICAL: Do NOT call `WaitForShutdown()` or `shutdown()` -- both call `os.Exit`.

Test functions to implement:
- `TestNewManager` -- subtests: returned manager is non-nil, handlers slice is empty and non-nil (length 0, not nil)
- `TestRegisterHandler` -- subtests: single handler registration (verify length=1), multiple sequential registrations (verify length=N), concurrent registration from 50 goroutines using `sync.WaitGroup` (verify all 50 registered, no data races)
- `TestExecuteHandler` -- subtests: handler returns nil (verify executeHandler returns nil), handler returns error (verify executeHandler returns that error), handler panics (verify executeHandler recovers, does not propagate panic, returns nil since panic recovery does not set err)

For `executeHandler`, construct a `*Manager` via `NewManager()` and call the method directly: `m.executeHandler(handler, 0)`. The method takes a `func() error` and an `int` index.

All tests must use `t.Parallel()`.

**Context to Read:**
- design.md, section "Component: shutdown tests"
- `/Users/divkix/GitHub/Alita_Robot/alita/utils/shutdown/graceful.go` -- Manager struct, NewManager, RegisterHandler, executeHandler signatures

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && go test -v -race ./alita/utils/shutdown/...
```

---

## TASK-003: Test decorators/misc package (command array helpers)

**Complexity:** S
**Files:**
- CREATE: `alita/utils/decorators/misc/handler_vars_test.go`
**Dependencies:** None
**Description:**
Create a test file for the `misc` package covering `addToArray` (unexported, accessible from same package test file) and `AddCmdToDisableable`.

Test functions to implement:
- `TestAddToArray` -- subtests: nil slice with one value (returns slice containing that value), existing slice with multiple values (all appended), empty variadic args (returns original slice unchanged), empty string value (appends empty string)
- `TestAddCmdToDisableable` -- subtests: single command (verify DisableCmds contains it), duplicate command (appears twice -- append semantics not set), concurrent 50 goroutines each adding a unique command (all 50 present, no data races under `-race`)

IMPORTANT: `DisableCmds` is a package-level `var`. Each test function must save/restore it via `t.Cleanup()` to avoid cross-test pollution:
```go
t.Cleanup(func() {
    mu.Lock()
    DisableCmds = make([]string, 0)
    mu.Unlock()
})
```

All tests must use `t.Parallel()` at the top-level function. Note: subtests that mutate `DisableCmds` should NOT be parallel with each other (shared global state).

**Context to Read:**
- design.md, section "Component: decorators/misc tests"
- `/Users/divkix/GitHub/Alita_Robot/alita/utils/decorators/misc/handler_vars.go` -- addToArray, AddCmdToDisableable, DisableCmds, mu

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && go test -v -race ./alita/utils/decorators/misc/...
```

---

## TASK-004: Test keyword_matcher cache (close 40% coverage gap)

**Complexity:** M
**Files:**
- CREATE: `alita/utils/keyword_matcher/cache_test.go`
**Dependencies:** None
**Description:**
Create a test file for the cache portion of `keyword_matcher` package covering `NewCache`, `GetOrCreateMatcher`, `CleanupExpired`, and `patternsEqual` (all unexported, accessible from same package). Do NOT test `GetGlobalCache` (it starts a background goroutine).

Test functions to implement:
- `TestNewCache` -- subtests: TTL is set correctly, matchers map is initialized and empty, lastUsed map is initialized and empty
- `TestGetOrCreateMatcher` -- subtests: new chatID creates and returns matcher, same chatID with same patterns returns cached matcher (same pointer via `==`), same chatID with different patterns creates new matcher (different pointer), empty patterns slice (creates matcher with zero patterns, does not panic), concurrent access from 10 goroutines for same chatID (no data races)
- `TestCleanupExpired` -- subtests: expired entries removed (create cache with 1ms TTL, add entry, sleep 5ms, call CleanupExpired, verify removed), unexpired entries kept (create cache with 1h TTL, add entry, call CleanupExpired, verify still present), empty cache (does not panic), zero TTL (all entries expire immediately)
- `TestPatternsEqual` -- table-driven subtests: identical sets `["a","b"]` vs `["a","b"]` -> true, different order `["b","a"]` vs `["a","b"]` -> true (set comparison), different lengths `["a"]` vs `["a","b"]` -> false, nil/nil -> true (both length 0), different content `["a","b"]` vs `["a","c"]` -> false, duplicates `["a","a"]` vs `["a"]` -> false (different lengths)

Use `time.Sleep` for TTL expiry tests. All tests must use `t.Parallel()`.

**Context to Read:**
- design.md, section "Component: keyword_matcher cache tests"
- `/Users/divkix/GitHub/Alita_Robot/alita/utils/keyword_matcher/cache.go` -- Cache struct, NewCache, GetOrCreateMatcher, CleanupExpired, patternsEqual
- `/Users/divkix/GitHub/Alita_Robot/alita/utils/keyword_matcher/matcher_test.go` -- existing test patterns for reference

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && go test -v -race ./alita/utils/keyword_matcher/...
```

---

## TASK-005: Add TestMain to alita/db package for shared initialization

**Complexity:** M
**Files:**
- CREATE: `alita/db/testmain_test.go`
- MODIFY: `alita/db/captcha_db_test.go` -- remove `DB.AutoMigrate` calls on lines 13-15 and 81-83, add `skipIfNoDb(t)` at start of each test
- MODIFY: `alita/db/locks_db_test.go` -- remove `DB.AutoMigrate` calls at the start of each test function (lines 10-12, 40-42, 78-80, 105-107), add `skipIfNoDb(t)` at start of each test
- MODIFY: `alita/db/antiflood_db_test.go` -- remove `DB.AutoMigrate` calls at the start of each test function (lines 9-11, 42-44, 75-77), add `skipIfNoDb(t)` at start of each test
**Dependencies:** None
**Description:**
Create `testmain_test.go` in `alita/db/` with a `TestMain(m *testing.M)` function and a `skipIfNoDb(t *testing.T)` helper. This is the foundational infrastructure for ALL DB tests.

`TestMain` implementation:
1. Check `DB == nil` -- if nil, print `"Skipping DB tests: PostgreSQL not available (DB == nil)"` to stdout and call `os.Exit(0)` to skip all tests gracefully
2. Call `DB.AutoMigrate(...)` for ALL GORM models: `User`, `Chat`, `WarnSettings`, `Warns`, `GreetingSettings`, `ChatFilters`, `AdminSettings`, `BlacklistSettings`, `PinSettings`, `ReportChatSettings`, `ReportUserSettings`, `DevSettings`, `ChannelSettings`, `AntifloodSettings`, `ConnectionSettings`, `ConnectionChatSettings`, `DisableSettings`, `DisableChatSettings`, `RulesSettings`, `LockSettings`, `NotesSettings`, `Notes`, `CaptchaSettings`, `CaptchaAttempts`, `StoredMessages`, `CaptchaMutedUsers`
3. If AutoMigrate returns error, print error and call `os.Exit(1)`
4. Call `os.Exit(m.Run())`

`skipIfNoDb` helper (defense-in-depth for individual test functions):
```go
func skipIfNoDb(t *testing.T) {
    t.Helper()
    if DB == nil {
        t.Skip("requires PostgreSQL connection")
    }
}
```

Then MODIFY the 3 existing test files to remove their per-test `DB.AutoMigrate()` calls. In each file, replace the `AutoMigrate` block at the start of each test function with a call to `skipIfNoDb(t)`. For example, in `captcha_db_test.go`, replace:
```go
if err := DB.AutoMigrate(&CaptchaAttempts{}); err != nil {
    t.Fatalf("failed to migrate captcha_attempts: %v", err)
}
```
with:
```go
skipIfNoDb(t)
```

Do the same for all test functions in `locks_db_test.go` and `antiflood_db_test.go`.

**Context to Read:**
- design.md, sections "Component: DB TestMain", "TestMain Contract", "skipIfNoDb Contract"
- `/Users/divkix/GitHub/Alita_Robot/alita/db/db.go` -- GORM model type definitions (all struct types that need AutoMigrate)
- `/Users/divkix/GitHub/Alita_Robot/alita/db/captcha_db_test.go` -- existing test to modify
- `/Users/divkix/GitHub/Alita_Robot/alita/db/locks_db_test.go` -- existing test to modify
- `/Users/divkix/GitHub/Alita_Robot/alita/db/antiflood_db_test.go` -- existing test to modify

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && BOT_TOKEN=test-token OWNER_ID=1 MESSAGE_DUMP=1 DATABASE_URL="postgres://postgres:postgres@localhost:5432/alita_test?sslmode=disable" go test -v -race -count=1 ./alita/db/...
```

---

## TASK-006: Test DB cache key generators and migration SQL cleaning

**Complexity:** M
**Files:**
- CREATE: `alita/db/cache_helpers_test.go`
- CREATE: `alita/db/migrations_test.go`
**Dependencies:** TASK-005
**Description:**
Create two test files for pure functions in the `db` package that do not require a live database connection. These test cache key formatting and SQL cleaning logic.

**cache_helpers_test.go:**
- `TestCacheKeyGenerators` -- table-driven test covering all 8 key generator functions. For each function, test with positive ID (12345), zero (0), and negative channel ID (-1001234567890). Verify:
  1. Each key starts with `"alita:"` prefix
  2. Each key matches expected format `"alita:{segment}:{id}"`
  3. No two functions produce the same output for the same input (segment uniqueness)

Functions to test: `chatSettingsCacheKey`, `userLanguageCacheKey`, `chatLanguageCacheKey`, `filterListCacheKey`, `blacklistCacheKey`, `warnSettingsCacheKey`, `disabledCommandsCacheKey`, `captchaSettingsCacheKey`

Expected segments: `chat_settings`, `user_lang`, `chat_lang`, `filter_list`, `blacklist`, `warn_settings`, `disabled_cmds`, `captcha_settings`

**migrations_test.go:**
- `TestCleanSupabaseSQL` -- table-driven subtests: GRANT removal (`GRANT SELECT ON users TO anon;` -> removed), policy removal, `with schema "extensions"` removal, CREATE EXTENSION normalization (adds `IF NOT EXISTS`), Supabase-only extension removal (e.g., `CREATE EXTENSION hypopg;` -> commented out), empty SQL (returns empty), clean SQL passthrough (no Supabase syntax -> unchanged), idempotency transforms (CREATE TABLE -> CREATE TABLE IF NOT EXISTS)
- `TestSplitSQLStatements` -- subtests: simple split on semicolons, dollar-quoted strings (semicolons inside `$$...$$` are not split), block comments (`/* ; */` not split), line comments (`-- ;` not split), quoted semicolons (inside single quotes), empty input
- `TestSchemaMigrationTableName` -- verify `SchemaMigration{}.TableName()` returns `"schema_migrations"`

For `cleanSupabaseSQL` and `splitSQLStatements`, construct a `MigrationRunner` with nil db: `runner := &MigrationRunner{db: nil, migrationsPath: "", cleanSQL: true}`. These methods do not use `m.db`.

All tests must call `skipIfNoDb(t)` as the first line for consistency with other `db` package tests, even though these particular functions are pure. In practice, `TestMain` already gates the package when DB is unavailable.

**Context to Read:**
- design.md, sections "Component: DB cache key generator tests" and "Component: DB cleanSupabaseSQL and splitSQLStatements tests"
- `/Users/divkix/GitHub/Alita_Robot/alita/db/cache_helpers.go` -- 8 cache key generator functions
- `/Users/divkix/GitHub/Alita_Robot/alita/db/migrations.go` -- cleanSupabaseSQL, splitSQLStatements, SchemaMigration, MigrationRunner struct

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && BOT_TOKEN=test-token OWNER_ID=1 MESSAGE_DUMP=1 DATABASE_URL="postgres://postgres:postgres@localhost:5432/alita_test?sslmode=disable" go test -v -race -run "TestCacheKey|TestCleanSupabase|TestSplitSQL|TestSchemaMigration" ./alita/db/...
```

---

## TASK-007: Test DB CRUD -- greetings (15 functions, most complex module)

**Complexity:** L
**Files:**
- CREATE: `alita/db/greetings_db_test.go`
**Dependencies:** TASK-005
**Description:**
Create integration tests for all greeting DB functions in `greetings_db.go`. This is the most complex DB module (380 LOC, 15 functions).

Pattern for every test function:
```go
func TestXxx(t *testing.T) {
    t.Parallel()
    skipIfNoDb(t)
    chatID := time.Now().UnixNano()
    if err := EnsureChatInDb(chatID, "test_greetings"); err != nil {
        t.Fatalf("EnsureChatInDb() error = %v", err)
    }
    t.Cleanup(func() {
        DB.Where("chat_id = ?", chatID).Delete(&GreetingSettings{})
        DB.Where("chat_id = ?", chatID).Delete(&Chat{})
    })
    // ... test body ...
}
```

Test functions to implement:
- `TestGetGreetingSettings_Defaults` -- verify default values: `ShouldWelcome=true`, `WelcomeText=DefaultWelcome`, `ShouldGoodbye=false`
- `TestSetWelcomeToggle_ZeroValueBoolean` -- set true, verify, set false, verify false persisted (GORM zero-value gotcha)
- `TestSetWelcomeText` -- set text/fileId/buttons/type, retrieve, verify all fields match
- `TestSetGoodbyeText` -- same pattern as welcome text
- `TestSetGoodbyeToggle_ZeroValueBoolean` -- same pattern as welcome toggle
- `TestSetShouldCleanService` -- boolean round-trip
- `TestSetShouldAutoApprove` -- boolean round-trip
- `TestSetCleanWelcomeSetting` -- boolean round-trip
- `TestSetCleanWelcomeMsgId` -- int64 round-trip
- `TestSetCleanGoodbyeSetting` -- boolean round-trip
- `TestSetCleanGoodbyeMsgId` -- int64 round-trip
- `TestGetWelcomeButtons_Empty` -- returns empty slice, not nil
- `TestGetGoodbyeButtons_Empty` -- returns empty slice, not nil
- `TestLoadGreetingsStats_EmptyDB` -- returns zeros on fresh chat ID range
- `TestGreetingSettings_ConcurrentWrites` -- 10 goroutines setting welcome/goodbye concurrently, no data corruption

**Context to Read:**
- design.md, section "Component: DB CRUD test files (16 new files)" and "DB Test Data Isolation Contract"
- requirements.md, US-010
- `/Users/divkix/GitHub/Alita_Robot/alita/db/greetings_db.go` -- all 15 functions to test
- `/Users/divkix/GitHub/Alita_Robot/alita/db/captcha_db_test.go` -- existing DB test pattern for reference

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && BOT_TOKEN=test-token OWNER_ID=1 MESSAGE_DUMP=1 DATABASE_URL="postgres://postgres:postgres@localhost:5432/alita_test?sslmode=disable" go test -v -race -run "TestGetGreeting|TestSetWelcome|TestSetGoodbye|TestSetShouldClean|TestSetShouldAutoApprove|TestSetCleanWelcome|TestSetCleanGoodbye|TestGetWelcomeButtons|TestGetGoodbyeButtons|TestLoadGreetingsStats|TestGreetingSettings" ./alita/db/...
```

---

## TASK-008: Test DB CRUD -- warns (13 functions) and notes (11 functions)

**Complexity:** L
**Files:**
- CREATE: `alita/db/warns_db_test.go`
- CREATE: `alita/db/notes_db_test.go`
**Dependencies:** TASK-005
**Description:**
Create integration tests for warn and note DB functions. Both follow the same pattern as TASK-007.

**warns_db_test.go** -- test all warn CRUD functions:
- `TestCheckWarnSettings_Defaults` -- verify default `WarnLimit=3`, `WarnMode="mute"` for new chat
- `TestWarnUser` -- warn a user, retrieve warns, verify count=1, reason matches
- `TestWarnUserReachesLimit` -- warn user N times where N=WarnLimit, verify limit-reached signal
- `TestRemoveWarn` -- add warn, remove it, verify count decremented
- `TestResetWarns` -- add multiple warns, reset all, verify zero warns for user
- `TestSetWarnLimit` -- set limit to 5, verify persisted
- `TestSetWarnMode` -- set mode to "ban", verify persisted
- `TestWarnWithEmptyReason` -- stores and retrieves empty string correctly
- `TestResetWarns_NoWarns` -- reset on user with zero warns, no error
- `TestConcurrentWarns` -- 10 goroutines warning same user simultaneously, verify correct final count
- `TestLoadWarnStats` -- verify stats after creating test data

**notes_db_test.go** -- test all note CRUD functions:
- `TestGetNotesSettings_Defaults` -- verify default `Private=false` for new chat
- `TestSaveNote` -- save note with name/text/buttons/type, retrieve by name, verify all fields
- `TestGetAllNotes` -- save 3 notes, retrieve all, verify count=3
- `TestRemoveNote` -- add note, delete by name, verify not found
- `TestToggleNotesPrivate` -- toggle private mode, verify round-trip
- `TestNoteUpsertBehavior` -- save same note name twice, verify updated not duplicated
- `TestGetAllNotes_EmptyChat` -- returns empty slice not nil
- `TestRemoveNonExistentNote` -- no error, no-op
- `TestLoadNotesStats` -- verify stats after creating test data

Each test uses `time.Now().UnixNano()` for unique IDs, `EnsureChatInDb()` for setup, and `t.Cleanup()` for teardown.

**Context to Read:**
- design.md, "DB Test Data Isolation Contract"
- requirements.md, US-011, US-012
- `/Users/divkix/GitHub/Alita_Robot/alita/db/warns_db.go` -- all warn functions
- `/Users/divkix/GitHub/Alita_Robot/alita/db/notes_db.go` -- all note functions

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && BOT_TOKEN=test-token OWNER_ID=1 MESSAGE_DUMP=1 DATABASE_URL="postgres://postgres:postgres@localhost:5432/alita_test?sslmode=disable" go test -v -race -run "TestCheckWarnSettings|TestWarnUser|TestRemoveWarn|TestResetWarns|TestSetWarnLimit|TestSetWarnMode|TestConcurrentWarns|TestLoadWarnStats|TestGetNotesSettings|TestSaveNote|TestGetAllNotes|TestRemoveNote|TestToggleNotes|TestNoteUpsert|TestRemoveNonExistent|TestLoadNotesStats" ./alita/db/...
```

---

## TASK-009: Test DB CRUD -- filters (7 functions) and blacklists (6 functions)

**Complexity:** M
**Files:**
- CREATE: `alita/db/filters_db_test.go`
- CREATE: `alita/db/blacklists_db_test.go`
**Dependencies:** TASK-005
**Description:**
Create integration tests for filter and blacklist DB functions.

**filters_db_test.go:**
- `TestSaveFilter` -- save filter with keyword/reply text/type, retrieve by keyword, verify
- `TestGetAllFilters` -- save 3 filters, retrieve all, verify count
- `TestRemoveFilter` -- add filter, delete by keyword, verify removed
- `TestFilterExists` -- add filter, verify `FilterExists` returns true; verify false for nonexistent
- `TestGetAllFilters_EmptyChat` -- returns empty slice not nil
- `TestConcurrentFilterCreation` -- concurrent creation for same chat/keyword, no corruption
- `TestFilterSpecialCharacterKeyword` -- keyword with regex metacharacters `.*+?` stores correctly
- `TestLoadFiltersStats` -- verify stats

**blacklists_db_test.go:**
- `TestAddBlacklistTrigger` -- add trigger, retrieve, verify
- `TestRemoveBlacklistTrigger` -- add then remove, verify removed
- `TestGetBlacklistSettings` -- verify default settings for new chat
- `TestSetBlacklistAction` -- set action mode, verify persisted
- `TestGetAllBlacklists` -- add multiple triggers, retrieve all, verify count
- `TestLoadBlacklistStats` -- verify stats

Each test follows the standard pattern: `skipIfNoDb`, unique IDs, `EnsureChatInDb`, `t.Cleanup`.

**Context to Read:**
- requirements.md, US-013, US-014 (blacklists section)
- `/Users/divkix/GitHub/Alita_Robot/alita/db/filters_db.go` -- all filter functions
- `/Users/divkix/GitHub/Alita_Robot/alita/db/blacklists_db.go` -- all blacklist functions

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && BOT_TOKEN=test-token OWNER_ID=1 MESSAGE_DUMP=1 DATABASE_URL="postgres://postgres:postgres@localhost:5432/alita_test?sslmode=disable" go test -v -race -run "TestSaveFilter|TestGetAllFilters|TestRemoveFilter|TestFilterExists|TestConcurrentFilter|TestFilterSpecialChar|TestLoadFiltersStats|TestAddBlacklist|TestRemoveBlacklist|TestGetBlacklist|TestSetBlacklist|TestGetAllBlacklists|TestLoadBlacklistStats" ./alita/db/...
```

---

## TASK-010: Test DB CRUD -- chats, users, channels, devs (4 core entity files)

**Complexity:** L
**Files:**
- CREATE: `alita/db/chats_db_test.go`
- CREATE: `alita/db/user_db_test.go`
- CREATE: `alita/db/channels_db_test.go`
- CREATE: `alita/db/devs_db_test.go`
**Dependencies:** TASK-005
**Description:**
Create integration tests for the 4 core entity DB files. These are foundational entities used by many other modules.

**chats_db_test.go** (6+ functions):
- `TestEnsureChatInDb` -- create chat, verify exists
- `TestUpdateChat` -- update chat title, verify
- `TestGetAllChats` -- create 2 chats, verify both returned
- `TestChatExists` -- verify true for existing, false for nonexistent
- `TestLoadChatStats` -- verify stats

**user_db_test.go** (8 functions):
- `TestEnsureUserInDb` -- create user, verify exists
- `TestUpdateUser` -- update username, verify
- `TestGetUserIdByUserName` -- create user with username, look up by username
- `TestGetUserIdByUserName_NotFound` -- returns 0 for nonexistent username
- `TestGetUserInfoById` -- verify returns username, name, found=true
- `TestGetUserInfoById_NotFound` -- returns empty strings, found=false
- `TestLoadUserStats` -- verify stats
- `TestConcurrentUserCreation` -- concurrent EnsureUserInDb for same ID

**channels_db_test.go** (6 functions):
- `TestEnsureChannelInDb` -- create channel, verify
- `TestGetChannelIdByUserName` -- create channel with username, look up
- `TestGetChannelIdByUserName_NotFound` -- returns 0
- `TestGetChannelInfoById` -- verify returns username, name, found
- `TestGetChannelInfoById_NotFound` -- returns empty, found=false
- `TestUpdateChannel` -- update channel info, verify

**devs_db_test.go** (7 functions):
- `TestAddDev` -- add dev user, verify with GetDevSettings
- `TestRemoveDev` -- add then remove, verify removed
- `TestAddSudo` -- add sudo user, verify
- `TestRemoveSudo` -- add then remove sudo, verify
- `TestGetDevSettings` -- verify default settings
- `TestDevDualBooleanFields` -- verify both `Dev`/`IsDev` and `Sudo`/`IsSudo` fields are set consistently
- `TestLoadDevStats` -- verify stats

**Context to Read:**
- requirements.md, US-014
- `/Users/divkix/GitHub/Alita_Robot/alita/db/chats_db.go`
- `/Users/divkix/GitHub/Alita_Robot/alita/db/user_db.go`
- `/Users/divkix/GitHub/Alita_Robot/alita/db/channels_db.go`
- `/Users/divkix/GitHub/Alita_Robot/alita/db/devs_db.go`

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && BOT_TOKEN=test-token OWNER_ID=1 MESSAGE_DUMP=1 DATABASE_URL="postgres://postgres:postgres@localhost:5432/alita_test?sslmode=disable" go test -v -race -run "TestEnsureChat|TestUpdateChat|TestGetAllChats|TestChatExists|TestLoadChatStats|TestEnsureUser|TestUpdateUser|TestGetUserId|TestGetUserInfo|TestLoadUserStats|TestConcurrentUser|TestEnsureChannel|TestGetChannelId|TestGetChannelInfo|TestUpdateChannel|TestAddDev|TestRemoveDev|TestAddSudo|TestRemoveSudo|TestGetDevSettings|TestDevDualBoolean|TestLoadDevStats" ./alita/db/...
```

---

## TASK-011: Test DB CRUD -- connections, disable, admin (3 settings files)

**Complexity:** M
**Files:**
- CREATE: `alita/db/connections_db_test.go`
- CREATE: `alita/db/disable_db_test.go`
- CREATE: `alita/db/admin_db_test.go`
**Dependencies:** TASK-005
**Description:**
Create integration tests for connections, disable, and admin DB files.

**connections_db_test.go** (8 functions):
- `TestConnectChat` -- connect user to chat, verify connection exists
- `TestDisconnectChat` -- connect then disconnect, verify removed
- `TestGetConnection` -- verify retrieval of active connection
- `TestReconnect` -- disconnect then reconnect, verify works
- `TestSetAllowConnect` -- toggle allow_connect boolean, verify round-trip (including false)
- `TestGetConnectedChats` -- connect user to 2 chats, verify both returned
- `TestLoadConnectionStats` -- verify stats
- `TestConcurrentConnect` -- 5 goroutines connecting same user/chat, no corruption

**disable_db_test.go** (9 functions):
- `TestDisableCommand` -- disable a command for chat, verify disabled
- `TestEnableCommand` -- disable then enable, verify enabled
- `TestIsCommandDisabled` -- verify returns true/false correctly
- `TestGetDisabledCommands` -- disable 3 commands, verify all returned
- `TestToggleDeleteEnabled_ZeroValueBoolean` -- set del_enabled true then false, verify round-trip
- `TestDisableNonExistentCommand` -- disable a command that was never enabled, verify creates record
- `TestLoadDisableStats` -- verify stats
- `TestGetDisableSettings_Defaults` -- verify defaults for new chat
- `TestConcurrentDisableEnable` -- concurrent disable/enable, no corruption

**admin_db_test.go** (3 functions):
- `TestGetAdminSettings_Defaults` -- verify default admin settings for new chat
- `TestSetAnonAdmin` -- toggle anon admin setting, verify round-trip including false
- `TestLoadAdminStats` -- verify stats (if exists)

**Context to Read:**
- requirements.md, US-014
- `/Users/divkix/GitHub/Alita_Robot/alita/db/connections_db.go`
- `/Users/divkix/GitHub/Alita_Robot/alita/db/disable_db.go`
- `/Users/divkix/GitHub/Alita_Robot/alita/db/admin_db.go`

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && BOT_TOKEN=test-token OWNER_ID=1 MESSAGE_DUMP=1 DATABASE_URL="postgres://postgres:postgres@localhost:5432/alita_test?sslmode=disable" go test -v -race -run "TestConnectChat|TestDisconnectChat|TestGetConnection|TestReconnect|TestSetAllowConnect|TestGetConnectedChats|TestLoadConnectionStats|TestConcurrentConnect|TestDisableCommand|TestEnableCommand|TestIsCommandDisabled|TestGetDisabledCommands|TestToggleDeleteEnabled|TestDisableNonExistent|TestLoadDisableStats|TestGetDisableSettings|TestConcurrentDisableEnable|TestGetAdminSettings|TestSetAnonAdmin|TestLoadAdminStats" ./alita/db/...
```

---

## TASK-012: Test DB CRUD -- lang, pin, reports, rules (4 small files)

**Complexity:** M
**Files:**
- CREATE: `alita/db/lang_db_test.go`
- CREATE: `alita/db/pin_db_test.go`
- CREATE: `alita/db/reports_db_test.go`
- CREATE: `alita/db/rules_db_test.go`
**Dependencies:** TASK-005
**Description:**
Create integration tests for the 4 remaining smaller DB files.

**lang_db_test.go** (5 functions):
- `TestGetLanguage_DefaultsToEn` -- new chat/user returns "en"
- `TestSetChatLanguage` -- set to "es", verify
- `TestSetUserLanguage` -- set to "fr", verify
- `TestGetChatLanguage` -- set and retrieve
- `TestGetUserLanguage` -- set and retrieve

**pin_db_test.go** (4 functions):
- `TestGetPinSettings_Defaults` -- verify defaults for new chat
- `TestSetCleanLinkedChannel` -- boolean round-trip
- `TestSetAntiChannelPin` -- boolean round-trip
- `TestConcurrentPinSettings` -- concurrent writes, no corruption

**reports_db_test.go** (7 functions):
- `TestGetChatReportSettings_Defaults` -- verify defaults
- `TestSetChatReportEnabled` -- boolean round-trip
- `TestGetUserReportSettings_Defaults` -- verify defaults
- `TestSetUserReportEnabled` -- boolean round-trip
- `TestGetBlockedReportsList` -- verify empty list for new user
- `TestAddBlockedReport` -- add blocked user, verify in list
- `TestRemoveBlockedReport` -- add then remove, verify removed

**rules_db_test.go** (6 functions):
- `TestGetRules_Defaults` -- verify empty/default rules for new chat
- `TestSetRules` -- set rules text, verify
- `TestClearRules` -- set then clear, verify empty
- `TestTogglePrivateRules_ZeroValueBoolean` -- boolean round-trip
- `TestGetRulesSettings_Defaults` -- verify default settings
- `TestLoadRulesStats` -- verify stats

**Context to Read:**
- requirements.md, US-014
- `/Users/divkix/GitHub/Alita_Robot/alita/db/lang_db.go`
- `/Users/divkix/GitHub/Alita_Robot/alita/db/pin_db.go`
- `/Users/divkix/GitHub/Alita_Robot/alita/db/reports_db.go`
- `/Users/divkix/GitHub/Alita_Robot/alita/db/rules_db.go`

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && BOT_TOKEN=test-token OWNER_ID=1 MESSAGE_DUMP=1 DATABASE_URL="postgres://postgres:postgres@localhost:5432/alita_test?sslmode=disable" go test -v -race -run "TestGetLanguage|TestSetChatLanguage|TestSetUserLanguage|TestGetChatLanguage|TestGetUserLanguage|TestGetPinSettings|TestSetCleanLinked|TestSetAntiChannel|TestConcurrentPin|TestGetChatReport|TestSetChatReport|TestGetUserReport|TestSetUserReport|TestGetBlockedReports|TestAddBlockedReport|TestRemoveBlockedReport|TestGetRules|TestSetRules|TestClearRules|TestTogglePrivate|TestGetRulesSettings|TestLoadRulesStats" ./alita/db/...
```

---

## TASK-013: Test extraction package pure functions (ExtractQuotes, IdFromReply)

**Complexity:** M
**Files:**
- CREATE: `alita/utils/extraction/extraction_test.go`
**Dependencies:** None
**Description:**
Create tests for the pure functions in the `extraction` package: `ExtractQuotes` and `IdFromReply`. These functions do not call the Telegram API. The package imports `db`, `i18n`, `chat_status` transitively, so it requires CI env vars to compile, but the functions themselves are pure once the package loads.

**TestExtractQuotes** -- table-driven with at least 8 subtests:
- Quoted text: `"\"hello world\" remaining"` with matchQuotes=true, matchWord=false -> inQuotes="hello world", afterWord="remaining"
- Word extraction: `"firstword rest of text"` with matchQuotes=false, matchWord=true -> inQuotes="firstword", afterWord="rest of text"
- Empty string: `""` with both flags true -> returns empty strings
- Unmatched opening quote: `"\"hello"` with matchQuotes=true -> returns empty strings (regex does not match closing quote)
- Both flags false: `"anything"` -> returns empty strings
- Special characters in quotes: `"\"hello & <world>\" rest"` with matchQuotes=true -> preserves special characters
- Multiline in quotes: `"\"hello\nworld\" rest"` with matchQuotes=true -> regex uses `(?s)` flag, matches across lines
- Word with special chars: `"hello-world_123 rest"` with matchWord=true -> inQuotes="hello-world_123", afterWord="rest"

**TestIdFromReply** -- subtests constructing `gotgbot.Message` structs directly:
- Nil ReplyToMessage: `&gotgbot.Message{ReplyToMessage: nil}` -> returns (0, "")
- Valid reply with text: `&gotgbot.Message{Text: "/cmd reason text", ReplyToMessage: &gotgbot.Message{From: &gotgbot.User{Id: 42}}}` -> returns (42, "reason text")
- Reply with no spaces in text: `&gotgbot.Message{Text: "/cmd", ReplyToMessage: &gotgbot.Message{From: &gotgbot.User{Id: 42}}}` -> returns (42, "")
- Reply from channel (SenderChat): construct message where `GetSender()` returns the channel ID

Note: `IdFromReply` calls `prevMessage.GetSender().Id()` which requires `From` field to be non-nil. Set `From: &gotgbot.User{Id: X}` in test structs.

All tests use `t.Parallel()`.

**Context to Read:**
- design.md, section "Component: extraction pure function tests"
- requirements.md, US-006
- `/Users/divkix/GitHub/Alita_Robot/alita/utils/extraction/extraction.go` -- ExtractQuotes and IdFromReply function signatures and implementation

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && BOT_TOKEN=test-token OWNER_ID=1 MESSAGE_DUMP=1 DATABASE_URL="postgres://postgres:postgres@localhost:5432/alita_test?sslmode=disable" go test -v -race -run "TestExtractQuotes|TestIdFromReply" ./alita/utils/extraction/...
```

---

## TASK-014: Test modules callback codec wrappers and expand helpers tests

**Complexity:** M
**Files:**
- CREATE: `alita/modules/callback_codec_test.go`
- MODIFY: `alita/utils/helpers/helpers_test.go` -- append new test functions (no existing code modified)
**Dependencies:** None
**Description:**
Two independent test additions for CI-dependent packages.

**callback_codec_test.go** (new file in `alita/modules/`):
Test `encodeCallbackData` and `decodeCallbackData` -- both unexported, accessible from same package test file.

- `TestEncodeCallbackData` -- subtests: valid encode (namespace="test", fields={"a":"1"}, fallback="fb" -> returns encoded string), encode error with fallback (empty namespace causes error -> returns fallback), nil fields map (verify no panic), empty fallback on error (returns "")
- `TestDecodeCallbackData` -- subtests: valid decode with no expected namespaces (returns decoded, true), valid decode with matching namespace (returns decoded, true), namespace mismatch (returns nil, false), case-insensitive match ("TEST" matches "test" via `strings.EqualFold`), invalid/malformed data (returns nil, false), empty string data (returns nil, false)

**helpers_test.go** (append new test functions -- do NOT modify existing 28 tests):
- `TestShtml` -- call `Shtml("test")`, verify it returns `*gotgbot.SendMessageOpts` with `ParseMode: "HTML"`
- `TestSmarkdown` -- call `Smarkdown("test")`, verify ParseMode is "Markdown"
- `TestGetMessageLinkFromMessageId` -- subtests: supergroup ID `-1001234567890` with messageID 42 -> correct `https://t.me/c/1234567890/42` URL, private chat positive ID -> correct URL format, messageID 0 -> valid URL
- `TestGetLangFormat` -- subtests: "en" returns English display, "es"/"fr"/"hi" return respective displays, unknown code returns fallback
- `TestExtractJoinLeftStatusChange` -- construct `gotgbot.ChatMemberUpdated` structs: join event (old=left, new=member -> identified as join), left event (old=member, new=left -> identified as left), nil update -> zero-value return
- `TestExtractAdminUpdateStatusChange` -- construct `gotgbot.ChatMemberUpdated` structs: promotion (old=member, new=admin), demotion (old=admin, new=member)

All tests use `t.Parallel()` and construct `gotgbot` structs directly (no API calls).

**Context to Read:**
- design.md, sections "Component: modules callback codec tests" and "Component: helpers expanded tests"
- requirements.md, US-007, US-019
- `/Users/divkix/GitHub/Alita_Robot/alita/modules/callback_codec.go` -- encodeCallbackData, decodeCallbackData
- `/Users/divkix/GitHub/Alita_Robot/alita/utils/helpers/helpers.go` -- Shtml, Smarkdown, GetMessageLinkFromMessageId, GetLangFormat, ExtractJoinLeftStatusChange, ExtractAdminUpdateStatusChange
- `/Users/divkix/GitHub/Alita_Robot/alita/utils/helpers/helpers_test.go` -- existing test patterns (DO NOT break existing tests)

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && BOT_TOKEN=test-token OWNER_ID=1 MESSAGE_DUMP=1 DATABASE_URL="postgres://postgres:postgres@localhost:5432/alita_test?sslmode=disable" go test -v -race -run "TestEncodeCallbackData|TestDecodeCallbackData" ./alita/modules/... && go test -v -race -run "TestShtml|TestSmarkdown|TestGetMessageLink|TestGetLangFormat|TestExtractJoinLeft|TestExtractAdminUpdate" ./alita/utils/helpers/...
```

---

## TASK-015: Expand i18n package tests

**Complexity:** M
**Files:**
- MODIFY: `alita/i18n/i18n_test.go` -- append new test functions (no existing code modified)
**Dependencies:** None
**Description:**
Add new test functions to the existing `i18n_test.go` file. The existing 10 tests must continue to pass unchanged.

New test functions to append:
- `TestTranslatorGet` -- subtests: existing key returns translated string (use a known key from `locales/en.yml`), nonexistent key returns fallback/error indicator, key with params substitutes named parameters correctly, nil params map does not panic
- `TestTranslatorGetPlural` -- subtests: count=0 selects "other" form, count=1 selects "one" form, count=2+ selects "other" form
- `TestLocaleManagerGetTranslator` -- subtests: "en" returns English translator (non-nil), nonexistent locale returns default/fallback translator or error
- `TestLocaleManagerGetAvailableLocales` -- verify at least `["en", "es", "fr", "hi"]` are available

To get a `Translator` instance, use the `i18n.MustNewTranslator("en")` or `i18n.NewTranslator("en")` function. To get the `LocaleManager`, use `i18n.GetLocaleManager()` or the singleton.

Read the `i18n_test.go` to understand the existing patterns and the `i18n.go`/`translator.go` files to understand the API surface. Use actual translation keys from `locales/en.yml` for testing.

All tests use `t.Parallel()`.

**Context to Read:**
- design.md, section "Component: i18n expanded tests"
- requirements.md, US-018
- `/Users/divkix/GitHub/Alita_Robot/alita/i18n/i18n_test.go` -- existing tests to understand patterns
- `/Users/divkix/GitHub/Alita_Robot/alita/i18n/i18n.go` -- LocaleManager API
- `/Users/divkix/GitHub/Alita_Robot/alita/i18n/translator.go` -- Translator.Get, Translator.GetPlural
- `/Users/divkix/GitHub/Alita_Robot/locales/en.yml` -- available translation keys

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && BOT_TOKEN=test-token OWNER_ID=1 MESSAGE_DUMP=1 DATABASE_URL="postgres://postgres:postgres@localhost:5432/alita_test?sslmode=disable" go test -v -race ./alita/i18n/...
```

---

## TASK-016: Test monitoring auto_remediation and background_stats pure functions

**Complexity:** M
**Files:**
- CREATE: `alita/utils/monitoring/auto_remediation_test.go`
- CREATE: `alita/utils/monitoring/background_stats_test.go`
**Dependencies:** None
**Description:**
Create tests for testable portions of the monitoring package. The package imports `config` and `db`, so it requires CI env vars.

**auto_remediation_test.go:**
Test `CanExecute`, `Name`, and `Severity` for all 4 action types. Do NOT call `Execute()` (triggers actual GC/runtime operations). Do NOT call `NewAutoRemediationManager()` (reads `config.AppConfig`). Instead, construct action structs directly.

- `TestGCActionCanExecute` -- subtests: metrics with `MemoryAllocMB` above 60% of `config.AppConfig.ResourceMaxMemoryMB` -> true, metrics below threshold AND GCPauseMs<50 -> false, metrics with GCPauseMs>50 but low memory -> true (OR condition), zero-value SystemMetrics -> false
- `TestMemoryCleanupActionCanExecute` -- subtests: `MemoryAllocMB` above `config.AppConfig.ResourceGCThresholdMB` -> true, below -> false
- `TestLogWarningActionCanExecute` -- subtests: goroutines above 80% of `config.AppConfig.ResourceMaxGoroutines` -> true, memory above 50% of max -> true, both below -> false
- `TestRestartRecommendationActionCanExecute` -- subtests: goroutines above 150% of max -> true, memory above 160% of max -> true, both below -> false
- `TestActionNames` -- verify `GCAction{}.Name()` = "garbage_collection", `MemoryCleanupAction{}.Name()` = "memory_cleanup", `LogWarningAction{}.Name()` = "log_warning", `RestartRecommendationAction{}.Name()` = "restart_recommendation"
- `TestActionSeverityOrdering` -- verify LogWarning(0) < GC(1) < MemoryCleanup(2) < RestartRecommendation(10)

**background_stats_test.go:**
Test atomic counter methods and `GetCurrentMetrics`. Do NOT call `Start()` or `Stop()` (start background goroutines).

- `TestNewBackgroundStatsCollector` -- verify intervals: systemStatsInterval=30s, databaseStatsInterval=1m, reportingInterval=5m; counters are zero
- `TestRecordMessage` -- call RecordMessage() N=100 times, verify messageCounter >= 100 via atomic read
- `TestRecordError` -- call RecordError() N=100 times, verify errorCounter >= 100
- `TestRecordResponseTime` -- call RecordResponseTime(10ms) 5 times, verify responseTimeSum and responseTimeCount
- `TestGetCurrentMetrics` -- initially returns zero-value SystemMetrics
- `TestConcurrentRecordMessage` -- 50 goroutines each calling RecordMessage() 100 times, verify total >= 5000 under `-race`
- `TestRecordResponseTimeZero` -- RecordResponseTime(0) does not cause issues

All tests use `t.Parallel()`.

**Context to Read:**
- design.md, sections "Component: monitoring auto_remediation tests" and "Component: monitoring background_stats tests"
- requirements.md, US-016, US-017
- `/Users/divkix/GitHub/Alita_Robot/alita/utils/monitoring/auto_remediation.go` -- all 4 action types, RemediationAction interface, SystemMetrics usage
- `/Users/divkix/GitHub/Alita_Robot/alita/utils/monitoring/background_stats.go` -- BackgroundStatsCollector struct, counter methods, GetCurrentMetrics

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && BOT_TOKEN=test-token OWNER_ID=1 MESSAGE_DUMP=1 DATABASE_URL="postgres://postgres:postgres@localhost:5432/alita_test?sslmode=disable" go test -v -race -run "TestGCAction|TestMemoryCleanupAction|TestLogWarningAction|TestRestartRecommendation|TestActionNames|TestActionSeverity|TestNewBackgroundStats|TestRecordMessage|TestRecordError|TestRecordResponseTime|TestGetCurrentMetrics|TestConcurrentRecord" ./alita/utils/monitoring/...
```

---

## TASK-017: Add CI coverage threshold enforcement

**Complexity:** S
**Files:**
- MODIFY: `.github/workflows/ci.yml` -- add coverage threshold check step after "Run test suite" step
**Dependencies:** None
**Description:**
Add a CI step that parses `coverage.out` and fails the build if total coverage drops below 40%.

Insert a new step after the "Run test suite" step and before the "Upload coverage reports" step in the `test` job:

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

The step must:
1. Check `coverage.out` exists (fail with clear message if missing)
2. Parse total coverage from `go tool cover -func` output
3. Compare against 40% threshold using `awk` (avoids `bc` dependency)
4. Write results to `$GITHUB_STEP_SUMMARY` for PR visibility
5. Exit 1 if below threshold, exit 0 if at or above
6. Use `if: success()` condition so it only runs if tests passed

**Context to Read:**
- design.md, section "Component: CI coverage threshold" and the YAML snippet
- requirements.md, US-021
- `/Users/divkix/GitHub/Alita_Robot/.github/workflows/ci.yml` -- current CI configuration, specifically the `test` job steps

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && grep -A 20 "Check coverage threshold" .github/workflows/ci.yml
```

---

## TASK-018: Full integration verification

**Complexity:** S
**Files:**
- None (verification only)
**Dependencies:** TASK-001, TASK-002, TASK-003, TASK-004, TASK-005, TASK-006, TASK-007, TASK-008, TASK-009, TASK-010, TASK-011, TASK-012, TASK-013, TASK-014, TASK-015, TASK-016, TASK-017
**Description:**
Run the full test suite and lint to verify all tasks are complete, all tests pass, no regressions, and coverage meets the 40% threshold.

Steps:
1. Run full test suite with race detection:
   ```bash
   BOT_TOKEN=test-token OWNER_ID=1 MESSAGE_DUMP=1 DATABASE_URL="postgres://postgres:postgres@localhost:5432/alita_test?sslmode=disable" make test
   ```
2. Run linter:
   ```bash
   make lint
   ```
3. Check coverage:
   ```bash
   go tool cover -func=coverage.out | tail -1
   ```
4. Verify per-package coverage:
   ```bash
   go tool cover -func=coverage.out | grep -E "^(github|total)"
   ```
5. Verify acceptance criteria:
   - [ ] US-001: All 8 previously-failing packages show non-zero coverage
   - [ ] US-002: `error_handling` package coverage >= 90%
   - [ ] US-003: `shutdown` package coverage >= 70%
   - [ ] US-004: `decorators/misc` package coverage >= 90%
   - [ ] US-005: `keyword_matcher` package coverage >= 85%
   - [ ] US-006: `extraction` package shows coverage for ExtractQuotes and IdFromReply
   - [ ] US-008: DB cache key generators tested
   - [ ] US-009: TestMain in alita/db/ works (tests skip when DB unavailable)
   - [ ] US-010-014: All 16 DB files have test coverage
   - [ ] US-021: CI YAML has coverage threshold step
   - [ ] NFR-003: All tests use `t.Parallel()`
   - [ ] NFR-006: Zero data races under `-race`
   - [ ] NFR-007: Overall coverage >= 40%
   - [ ] NFR-008: No new Go module dependencies (`go.mod` unchanged)

**Context to Read:**
- requirements.md, "Non-Functional Requirements" section

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && BOT_TOKEN=test-token OWNER_ID=1 MESSAGE_DUMP=1 DATABASE_URL="postgres://postgres:postgres@localhost:5432/alita_test?sslmode=disable" go test -v -race -coverprofile=coverage.out -count=1 -timeout 10m ./... && go tool cover -func=coverage.out | tail -1 && make lint
```

---

## File Manifest

| Task | Files Touched |
|------|---------------|
| TASK-001 | `alita/utils/error_handling/error_handling_test.go` |
| TASK-002 | `alita/utils/shutdown/graceful_test.go` |
| TASK-003 | `alita/utils/decorators/misc/handler_vars_test.go` |
| TASK-004 | `alita/utils/keyword_matcher/cache_test.go` |
| TASK-005 | `alita/db/testmain_test.go`, `alita/db/captcha_db_test.go`, `alita/db/locks_db_test.go`, `alita/db/antiflood_db_test.go` |
| TASK-006 | `alita/db/cache_helpers_test.go`, `alita/db/migrations_test.go` |
| TASK-007 | `alita/db/greetings_db_test.go` |
| TASK-008 | `alita/db/warns_db_test.go`, `alita/db/notes_db_test.go` |
| TASK-009 | `alita/db/filters_db_test.go`, `alita/db/blacklists_db_test.go` |
| TASK-010 | `alita/db/chats_db_test.go`, `alita/db/user_db_test.go`, `alita/db/channels_db_test.go`, `alita/db/devs_db_test.go` |
| TASK-011 | `alita/db/connections_db_test.go`, `alita/db/disable_db_test.go`, `alita/db/admin_db_test.go` |
| TASK-012 | `alita/db/lang_db_test.go`, `alita/db/pin_db_test.go`, `alita/db/reports_db_test.go`, `alita/db/rules_db_test.go` |
| TASK-013 | `alita/utils/extraction/extraction_test.go` |
| TASK-014 | `alita/modules/callback_codec_test.go`, `alita/utils/helpers/helpers_test.go` |
| TASK-015 | `alita/i18n/i18n_test.go` |
| TASK-016 | `alita/utils/monitoring/auto_remediation_test.go`, `alita/utils/monitoring/background_stats_test.go` |
| TASK-017 | `.github/workflows/ci.yml` |
| TASK-018 | None (verification only) |

---

## Dependency Graph

```
Stream A (Pure Functions -- all parallel, no deps):
  TASK-001 (error_handling)     --|
  TASK-002 (shutdown)           --|-- All independent, run in parallel
  TASK-003 (decorators/misc)    --|
  TASK-004 (keyword_matcher)    --|

Stream B (DB Tests -- TestMain first, then parallel):
  TASK-005 (TestMain + modify existing) --|
                                          |-- TASK-006 (cache keys + migrations)
                                          |-- TASK-007 (greetings)
                                          |-- TASK-008 (warns + notes)
                                          |-- TASK-009 (filters + blacklists)
                                          |-- TASK-010 (chats, users, channels, devs)
                                          |-- TASK-011 (connections, disable, admin)
                                          |-- TASK-012 (lang, pin, reports, rules)

Stream C (CI-dependent packages -- all parallel, no deps):
  TASK-013 (extraction)         --|
  TASK-014 (callback codec +    --|-- All independent, run in parallel
            helpers expansion)  --|
  TASK-015 (i18n expansion)     --|
  TASK-016 (monitoring)         --|
  TASK-017 (CI threshold)       --|

Final Gate:
  TASK-018 (verification) -- depends on ALL above
```

---

## Risk Register

| Task | Risk | Mitigation |
|------|------|------------|
| TASK-005 | `TestMain` `AutoMigrate` list falls out of sync with new GORM models added later | CI failure is immediate when a new model is used in tests but not migrated; add model to list when adding to `db.go` |
| TASK-005 | Removing `AutoMigrate` from existing tests breaks them if `TestMain` is incorrect | Run existing test suite immediately after modification to catch regressions |
| TASK-007-012 | Test data collision with `time.Now().UnixNano()` | Nanosecond resolution makes collision nearly impossible; each test uses `base + offset` pattern for multiple IDs |
| TASK-007-012 | GORM zero-value boolean gotcha: `false` not persisted via `.Updates()` struct | Design mandates explicit zero-value boolean round-trip test in every DB file with boolean fields |
| TASK-007-012 | DB functions internally call `ChatExists()` requiring chat to exist | Every test calls `EnsureChatInDb()` in setup before exercising module-specific functions |
| TASK-013 | `IdFromReply` calls `GetSender().Id()` which panics if `From` is nil | Test struct construction must always set `From: &gotgbot.User{Id: X}` |
| TASK-014 | Modifying `helpers_test.go` could break existing 28 tests | Only append new functions; never modify existing code; run full test suite after |
| TASK-015 | Modifying `i18n_test.go` could break existing 10 tests | Only append new functions; never modify existing code |
| TASK-016 | `CanExecute` reads `config.AppConfig` which may have unexpected test-token defaults | Test verifies against actual `config.AppConfig` values, not hardcoded expected values |
| TASK-017 | Coverage threshold blocks unrelated PRs | Set initial threshold at 40% (conservative); can be ratcheted up later |
| TASK-018 | Full suite takes > 5 minutes due to DB test volume | CI timeout is 20 minutes; individual DB tests are fast (sub-second each); 10-minute timeout in `make test` |

TASKS_COMPLETE
