# Requirements: Increase Test Coverage to 45%+

**Date:** 2026-02-21
**Goal:** Raise Go test coverage from 12.5% to >= 45% (CI threshold: 40%, target: 45% for margin)
**Source:** research.md

## Scope

### In Scope
- Adding unit tests for pure functions across all packages (no external dependencies)
- Adding unit tests for GORM custom types (Scan/Value/TableName methods) in `alita/db/db.go`
- Adding unit tests for `ValidateConfig()` and `setDefaults()` in `alita/config/config.go`
- Adding integration tests for DB CRUD operations following existing TestMain/skipIfNoDb pattern
- Adding unit tests for i18n translator and manager functions
- Adding unit tests for unexported helper functions in `alita/modules/`
- Adding unit tests for `alita/utils/extraction/`, `alita/utils/helpers/`, `alita/utils/monitoring/`
- Adding unit tests for migration SQL processing functions in `alita/db/migrations.go`
- Adding unit tests for `alita/utils/chat_status/` pure helper functions
- Ensuring all new tests follow existing patterns (table-driven, `t.Parallel()`, subtests)
- Ensuring all new tests pass in CI (with env vars set, PostgreSQL available, no Redis)

### Out of Scope
- Refactoring `config.init()` to remove `log.Fatalf` -- separate PR, architectural change
- Adding Redis to CI services -- infrastructure change, not test code
- Mocking the Telegram Bot API (`gotgbot.Bot`) for handler-level integration tests -- requires significant infrastructure (mock HTTP server or interface abstraction) that is a project in itself
- Testing HTTP server endpoints in `alita/utils/httpserver/` -- requires full server lifecycle management
- Testing `alita/utils/media/` sender functions -- requires Telegram API mocking
- Increasing coverage beyond 50% -- diminishing returns, handler-level tests require API mocking
- Performance optimization of existing tests -- focus is on coverage quantity, not test speed
- Adding tests for `alita/utils/webhook/` (18 LOC, trivial, negligible coverage impact)
- Adding tests for `alita/utils/debug_bot/` (20 LOC, trivial)
- Adding tests for `alita/utils/decorators/cmdDecorator/` (15 LOC, trivial)

## User Stories

### US-001: GORM Custom Type Unit Tests

**Priority:** P0 (must-have)

As a developer,
I want unit tests for all GORM custom types (ButtonArray, StringArray, Int64Array) Scan/Value methods,
So that serialization and deserialization correctness is verified without requiring a database.

**Acceptance Criteria:**
- [ ] GIVEN a nil input WHEN `ButtonArray.Scan(nil)` is called THEN the result is an empty `ButtonArray{}` with no error
- [ ] GIVEN valid JSON bytes WHEN `ButtonArray.Scan([]byte)` is called THEN the result contains the deserialized buttons with no error
- [ ] GIVEN a non-byte input WHEN `ButtonArray.Scan("string")` is called THEN an error is returned with message containing "type assertion"
- [ ] GIVEN malformed JSON WHEN `ButtonArray.Scan([]byte("{invalid"))` is called THEN a JSON unmarshal error is returned
- [ ] GIVEN an empty ButtonArray WHEN `ButtonArray.Value()` is called THEN `"[]"` is returned with no error
- [ ] GIVEN a populated ButtonArray WHEN `ButtonArray.Value()` is called THEN valid JSON bytes are returned with no error
- [ ] GIVEN identical test cases WHEN applied to StringArray Scan/Value THEN identical behavior patterns hold
- [ ] GIVEN identical test cases WHEN applied to Int64Array Scan/Value THEN identical behavior patterns hold

**Edge Cases:**
- [ ] `Scan` with empty byte slice `[]byte("")` -> JSON unmarshal error (empty string is not valid JSON)
- [ ] `Scan` with `[]byte("null")` -> empty array (JSON null unmarshals to nil slice, but pointer receiver sets empty)
- [ ] `Value` with single-element arrays -> valid JSON with one element
- [ ] `Value` with ButtonArray containing empty-string fields -> valid JSON with empty strings preserved
- [ ] `Scan` with deeply nested JSON (Button with special characters in Name/Url) -> correctly deserialized
- [ ] Int64Array with max int64 values -> correctly serialized and deserialized
- [ ] Int64Array with negative values -> correctly serialized and deserialized
- [ ] StringArray with unicode characters -> correctly serialized and deserialized

**Definition of Done:**
- [ ] Test file `alita/db/gorm_types_test.go` exists with table-driven tests
- [ ] All 6 Scan/Value methods have at least 4 test cases each (nil, valid, invalid type, malformed)
- [ ] Tests pass with `go test ./alita/db/... -run TestButtonArray -run TestStringArray -run TestInt64Array`
- [ ] Tests do NOT require PostgreSQL (no `skipIfNoDb` guard needed)
- [ ] Tests use `t.Parallel()` at both top level and subtest level

---

### US-002: TableName Method Unit Tests

**Priority:** P0 (must-have)

As a developer,
I want unit tests for every `TableName()` method on GORM model structs,
So that table name mappings are verified and regressions are caught if a table name changes.

**Acceptance Criteria:**
- [ ] GIVEN each GORM model struct WHEN its `TableName()` method is called THEN the expected table name string is returned
- [ ] GIVEN the full list of models (User, Chat, WarnSettings, Warns, GreetingSettings, ChatFilters, AdminSettings, BlacklistSettings, PinSettings, ReportChatSettings, ReportUserSettings, DevSettings, ChannelSettings, AntifloodSettings, ConnectionSettings, ConnectionChatSettings, DisableSettings, DisableChatSettings, RulesSettings, LockSettings, NotesSettings, Notes, CaptchaSettings, CaptchaAttempts, StoredMessages, CaptchaMutedUsers, SchemaMigration) WHEN each is tested THEN all return their documented table name

**Edge Cases:**
- [ ] Ensure zero-value struct (no fields set) still returns correct table name -> TableName is independent of struct field values

**Definition of Done:**
- [ ] All 27+ TableName() methods have a corresponding test case
- [ ] Tests are in a single table-driven test function `TestTableNames`
- [ ] Tests pass without PostgreSQL connection
- [ ] Tests use `t.Parallel()`

---

### US-003: BlacklistSettingsSlice Method Tests

**Priority:** P0 (must-have)

As a developer,
I want unit tests for `BlacklistSettingsSlice.Triggers()`, `.Action()`, and `.Reason()`,
So that the slice helper methods return correct defaults and aggregations.

**Acceptance Criteria:**
- [ ] GIVEN an empty BlacklistSettingsSlice WHEN `Triggers()` is called THEN an empty/nil slice is returned
- [ ] GIVEN a slice with 3 entries WHEN `Triggers()` is called THEN a slice of 3 Word strings is returned in order
- [ ] GIVEN an empty BlacklistSettingsSlice WHEN `Action()` is called THEN `"warn"` is returned (default)
- [ ] GIVEN a slice with entries WHEN `Action()` is called THEN the first entry's Action is returned
- [ ] GIVEN an empty BlacklistSettingsSlice WHEN `Reason()` is called THEN the default format string `"Blacklisted word: '%s'"` is returned
- [ ] GIVEN a slice with a non-empty Reason WHEN `Reason()` is called THEN the first entry's Reason is returned
- [ ] GIVEN a slice with an empty-string Reason WHEN `Reason()` is called THEN the default format string is returned

**Edge Cases:**
- [ ] Slice with one entry that has empty Action string -> returns empty string (not default, because len > 0)
- [ ] Slice with nil pointer entries -> panic expected; document this behavior or test that it does panic
- [ ] Slice with mixed Action values -> only first entry's Action matters

**Definition of Done:**
- [ ] Tests cover all 3 methods with at least 3 test cases each
- [ ] Tests pass without PostgreSQL
- [ ] Tests verify the exact default string values

---

### US-004: getSpanAttributes and NotesSettings Helper Tests

**Priority:** P1 (should-have)

As a developer,
I want unit tests for `getSpanAttributes()` and `NotesSettings.PrivateNotesEnabled()`,
So that tracing attributes and boolean accessor correctness is verified.

**Acceptance Criteria:**
- [ ] GIVEN a nil model WHEN `getSpanAttributes(nil)` is called THEN an empty attribute slice is returned
- [ ] GIVEN a User struct WHEN `getSpanAttributes(user)` is called THEN a slice with one attribute `db.model` containing `*db.User` is returned
- [ ] GIVEN a NotesSettings with Private=true WHEN `PrivateNotesEnabled()` is called THEN true is returned
- [ ] GIVEN a NotesSettings with Private=false WHEN `PrivateNotesEnabled()` is called THEN false is returned

**Edge Cases:**
- [ ] `getSpanAttributes` with a non-struct type (e.g., string) -> attribute contains `string` type name
- [ ] `getSpanAttributes` with a pointer vs non-pointer model -> attribute reflects the pointer type

**Definition of Done:**
- [ ] Tests exist and pass without external dependencies
- [ ] Tests use `t.Parallel()`

---

### US-005: Config ValidateConfig Unit Tests

**Priority:** P0 (must-have)

As a developer,
I want unit tests for `ValidateConfig()` covering all validation branches,
So that configuration validation logic is verified without triggering `init()` side effects.

**Acceptance Criteria:**
- [ ] GIVEN a Config with empty BotToken WHEN `ValidateConfig` is called THEN an error containing "BOT_TOKEN is required" is returned
- [ ] GIVEN a Config with OwnerId=0 WHEN `ValidateConfig` is called THEN an error containing "OWNER_ID" is returned
- [ ] GIVEN a Config with empty DatabaseURL WHEN `ValidateConfig` is called THEN an error containing "DATABASE_URL" is returned
- [ ] GIVEN a Config with empty RedisAddress WHEN `ValidateConfig` is called THEN an error containing "REDIS_ADDRESS" is returned
- [ ] GIVEN a Config with UseWebhooks=true and empty WebhookDomain WHEN `ValidateConfig` is called THEN an error containing "WEBHOOK_DOMAIN" is returned
- [ ] GIVEN a Config with UseWebhooks=true and empty WebhookSecret WHEN `ValidateConfig` is called THEN an error containing "WEBHOOK_SECRET" is returned
- [ ] GIVEN a Config with HTTPPort=0 WHEN `ValidateConfig` is called THEN an error containing "HTTP_PORT" is returned
- [ ] GIVEN a Config with HTTPPort=70000 WHEN `ValidateConfig` is called THEN an error containing "HTTP_PORT" is returned
- [ ] GIVEN a Config with ChatValidationWorkers=0 WHEN `ValidateConfig` is called THEN an error is returned
- [ ] GIVEN a Config with ChatValidationWorkers=101 WHEN `ValidateConfig` is called THEN an error is returned
- [ ] GIVEN a fully valid Config WHEN `ValidateConfig` is called THEN nil is returned
- [ ] GIVEN a Config with all worker pool values at boundary minimums (1) WHEN `ValidateConfig` is called THEN nil is returned
- [ ] GIVEN a Config with all worker pool values at boundary maximums WHEN `ValidateConfig` is called THEN nil is returned
- [ ] GIVEN a Config with DB pool values at boundary (DBMaxIdleConns=100, DBMaxOpenConns=1000) WHEN `ValidateConfig` is called THEN nil is returned
- [ ] GIVEN a Config with DB pool values exceeding max (DBMaxIdleConns=101) WHEN `ValidateConfig` is called THEN an error is returned

**Edge Cases:**
- [ ] Config with MessageDump=0 but all other required fields valid -> error on MessageDump
- [ ] Config with negative OwnerId -> passes validation (only checks == 0), document this gap
- [ ] Config with DispatcherMaxRoutines=0 -> passes (0 means "use default", validation allows it)
- [ ] Config with DBMaxIdleConns=0 -> passes (0 means "not set, use default")
- [ ] Config with UseWebhooks=false and empty WebhookDomain -> passes (webhook validation skipped)
- [ ] Config with OperationTimeoutSeconds=301 -> error
- [ ] Config with MaxConcurrentOperations=-1 -> error

**Definition of Done:**
- [ ] Tests exist in `alita/config/config_test.go`
- [ ] NOTE: This file is in the `config` package which has `init()` that calls `log.Fatalf`. Tests rely on CI environment variables being set (`BOT_TOKEN`, `OWNER_ID`, etc.) to survive the init(). Tests SHALL be guarded with a skip check if required env vars are absent.
- [ ] All 17 validation branches in ValidateConfig are covered
- [ ] Tests use table-driven pattern with `t.Parallel()`
- [ ] Each validation error message is checked via `strings.Contains` or `errors.Is`

---

### US-006: Config setDefaults Unit Tests

**Priority:** P0 (must-have)

As a developer,
I want unit tests for `Config.setDefaults()` covering all default value assignments,
So that default configuration behavior is verified and documented via tests.

**Acceptance Criteria:**
- [ ] GIVEN a zero-value Config WHEN `setDefaults()` is called THEN ApiServer is set to `"https://api.telegram.org"`
- [ ] GIVEN a zero-value Config WHEN `setDefaults()` is called THEN WorkingMode is set to `"worker"`
- [ ] GIVEN a zero-value Config WHEN `setDefaults()` is called THEN RedisAddress is set to `"localhost:6379"`
- [ ] GIVEN a zero-value Config WHEN `setDefaults()` is called THEN HTTPPort is set to 8080
- [ ] GIVEN a Config with WebhookPort=9090 and HTTPPort=0 WHEN `setDefaults()` is called THEN HTTPPort is set to 9090 (backward compat)
- [ ] GIVEN a Config with HTTPPort=3000 WHEN `setDefaults()` is called THEN HTTPPort remains 3000 (not overwritten)
- [ ] GIVEN a zero-value Config WHEN `setDefaults()` is called THEN all worker pool configs are populated with values > 0
- [ ] GIVEN a zero-value Config WHEN `setDefaults()` is called THEN DB pool defaults are set (DBMaxIdleConns=50, DBMaxOpenConns=200, etc.)
- [ ] GIVEN a zero-value Config WHEN `setDefaults()` is called THEN MigrationsPath is set to `"migrations"`
- [ ] GIVEN a Config with Debug=false WHEN `setDefaults()` is called THEN EnablePerformanceMonitoring and EnableBackgroundStats are true
- [ ] GIVEN a Config with Debug=true WHEN `setDefaults()` is called THEN EnablePerformanceMonitoring and EnableBackgroundStats remain their original values

**Edge Cases:**
- [ ] Config with pre-set values -> pre-set values are NOT overwritten (only zero values get defaults)
- [ ] Config with RedisDB=0 -> set to 1 (default), but RedisDB=5 stays 5
- [ ] ClearCacheOnStartup is always set to true regardless of prior value -> verify this unconditional set

**Definition of Done:**
- [ ] Tests exist in `alita/config/config_test.go` alongside US-005 tests
- [ ] Same CI env var guard applies as US-005
- [ ] At least 15 default values verified
- [ ] Tests verify both "sets default when zero" and "preserves non-zero"

---

### US-007: Config getRedisAddress and getRedisPassword Tests

**Priority:** P1 (should-have)

As a developer,
I want unit tests for `getRedisAddress()` and `getRedisPassword()`,
So that Redis connection string parsing from both direct env vars and Heroku-style REDIS_URL is verified.

**Acceptance Criteria:**
- [ ] GIVEN REDIS_ADDRESS env var is set WHEN `getRedisAddress()` is called THEN the REDIS_ADDRESS value is returned
- [ ] GIVEN REDIS_ADDRESS is empty and REDIS_URL is set to a valid Redis URL WHEN `getRedisAddress()` is called THEN the host:port from REDIS_URL is returned
- [ ] GIVEN both REDIS_ADDRESS and REDIS_URL are empty WHEN `getRedisAddress()` is called THEN empty string is returned
- [ ] GIVEN REDIS_PASSWORD env var is set WHEN `getRedisPassword()` is called THEN the password is returned
- [ ] GIVEN REDIS_PASSWORD is empty and REDIS_URL contains a password WHEN `getRedisPassword()` is called THEN the password from REDIS_URL is returned
- [ ] GIVEN both REDIS_PASSWORD and REDIS_URL are empty WHEN `getRedisPassword()` is called THEN empty string is returned

**Edge Cases:**
- [ ] REDIS_URL with invalid URL format -> empty string returned (parse error handled)
- [ ] REDIS_URL with no password in userinfo -> empty string returned
- [ ] REDIS_URL with username but no password -> empty string returned
- [ ] REDIS_ADDRESS takes priority over REDIS_URL even when both are set

**Definition of Done:**
- [ ] Tests use `t.Setenv()` to manipulate environment variables safely
- [ ] Tests restore env vars after each test case (t.Setenv handles this automatically)
- [ ] Tests NOT marked `t.Parallel()` due to env var mutation (or use subtests that each set their own env)
- [ ] Same CI env var guard as US-005/006

---

### US-008: DB Integration Tests for Optimized Queries

**Priority:** P0 (must-have)

As a developer,
I want integration tests for `OptimizedLockQueries`, `OptimizedDisableQueries`, `OptimizedGreetingQueries`, and `OptimizedBlacklistQueries`,
So that the optimized query paths are verified against a real PostgreSQL database.

**Acceptance Criteria:**
- [ ] GIVEN a nil DB WHEN `NewOptimizedLockQueries()` is called THEN a non-nil struct with nil db field is returned
- [ ] GIVEN a nil db field WHEN `GetLockStatus()` is called THEN an error containing "database not initialized" is returned
- [ ] GIVEN no lock record exists WHEN `GetLockStatus(chatID, "sticker")` is called THEN `(false, nil)` is returned
- [ ] GIVEN a lock record with locked=true WHEN `GetLockStatus(chatID, "sticker")` is called THEN `(true, nil)` is returned
- [ ] GIVEN no locks exist WHEN `GetChatLocksOptimized(chatID)` is called THEN an empty map is returned
- [ ] GIVEN 3 locks exist WHEN `GetChatLocksOptimized(chatID)` is called THEN a map with 3 entries is returned
- [ ] GIVEN equivalent tests for `OptimizedDisableQueries` WHEN tested THEN disabled command checks work correctly
- [ ] GIVEN equivalent tests for `OptimizedGreetingQueries` WHEN tested THEN greeting status checks work correctly
- [ ] GIVEN equivalent tests for `OptimizedBlacklistQueries` WHEN tested THEN blacklist trigger checks work correctly

**Edge Cases:**
- [ ] Query for non-existent chat ID -> default/empty results, no error
- [ ] Query after inserting and then deleting the record -> returns default/not-found
- [ ] Concurrent reads on the same chat ID -> no race conditions (use `-race` flag)
- [ ] Chat ID at boundary (max int64) -> query executes without overflow

**Definition of Done:**
- [ ] Test file `alita/db/optimized_queries_test.go` exists
- [ ] All tests call `skipIfNoDb(t)` at the start
- [ ] All tests use unique chat IDs via `time.Now().UnixNano()` to avoid collisions
- [ ] All tests use `t.Cleanup()` to remove test data
- [ ] Tests pass in CI with PostgreSQL service
- [ ] Tests are skipped gracefully when PostgreSQL is unavailable

---

### US-009: DB Integration Tests for Captcha Operations

**Priority:** P0 (must-have)

As a developer,
I want integration tests for CRUD operations in `captcha_db.go`,
So that captcha settings, attempts, stored messages, and muted users operations are verified.

**Acceptance Criteria:**
- [ ] GIVEN no captcha settings exist for a chat WHEN `GetCaptchaSettings(chatID)` is called THEN default settings are returned (Enabled=false, CaptchaMode="math", Timeout=2, FailureAction="kick", MaxAttempts=3)
- [ ] GIVEN captcha settings are created WHEN `GetCaptchaSettings(chatID)` is called THEN the created settings are returned
- [ ] GIVEN captcha settings exist WHEN settings are updated THEN subsequent Get returns updated values
- [ ] GIVEN a captcha attempt is created WHEN `GetCaptchaAttempt(userID, chatID)` is called THEN the attempt is returned
- [ ] GIVEN a captcha attempt exists WHEN `IncrementCaptchaAttempts(userID, chatID)` is called THEN the attempt count increases by 1
- [ ] GIVEN a captcha attempt exists WHEN `DeleteCaptchaAttempt(userID, chatID)` is called THEN subsequent Get returns not-found
- [ ] GIVEN stored messages exist WHEN `GetStoredMessages(attemptID)` is called THEN all stored messages for that attempt are returned
- [ ] GIVEN a muted user record exists WHEN `GetCaptchaMutedUsers()` is called THEN the record is included in results
- [ ] GIVEN a muted user with expired UnmuteAt WHEN `CleanupExpiredCaptchaMutes()` is called THEN the record is removed

**Edge Cases:**
- [ ] Create captcha attempt with expiry in the past -> still created, cleanup handles it
- [ ] Multiple stored messages for same attempt -> all returned in order
- [ ] Delete non-existent captcha attempt -> no error (idempotent) or ErrRecordNotFound depending on implementation
- [ ] Get captcha settings for chat ID 0 -> returns default settings

**Definition of Done:**
- [ ] Test file `alita/db/captcha_db_test.go` has comprehensive CRUD tests (some tests already exist; expand them)
- [ ] All tests use `skipIfNoDb(t)` and unique IDs
- [ ] Tests cover the full lifecycle: create -> read -> update -> read -> delete -> read
- [ ] Tests pass in CI

---

### US-010: DB Integration Tests for Greetings Operations

**Priority:** P0 (must-have)

As a developer,
I want integration tests for CRUD operations in `greetings_db.go`,
So that greeting settings for welcome/goodbye messages are verified.

**Acceptance Criteria:**
- [ ] GIVEN no greeting settings exist for a chat WHEN `GetGreetingSettings(chatID)` is called THEN default settings are returned with DefaultWelcome and DefaultGoodbye text
- [ ] GIVEN greeting settings are created WHEN `GetGreetingSettings(chatID)` is called THEN the created settings are returned
- [ ] GIVEN greeting settings exist WHEN welcome text is updated THEN subsequent Get returns updated welcome text
- [ ] GIVEN greeting settings exist WHEN welcome is disabled THEN `ShouldWelcome` returns false
- [ ] GIVEN greeting settings exist WHEN clean service is toggled THEN the toggle is persisted
- [ ] GIVEN greeting settings exist WHEN goodbye text is updated THEN subsequent Get returns updated goodbye text

**Edge Cases:**
- [ ] Update greeting with empty welcome text -> persisted as empty string
- [ ] Update greeting with ButtonArray containing buttons -> buttons serialized and deserialized correctly
- [ ] Get greeting for non-existent chat -> returns defaults, not error
- [ ] Update welcome and goodbye independently -> one update does not affect the other

**Definition of Done:**
- [ ] Test file `alita/db/greetings_db_test.go` has comprehensive tests (some may exist; expand them)
- [ ] All tests use `skipIfNoDb(t)` and unique IDs
- [ ] Tests verify default values match `DefaultWelcome` and `DefaultGoodbye` constants
- [ ] Tests pass in CI

---

### US-011: i18n Translator and Manager Tests

**Priority:** P0 (must-have)

As a developer,
I want comprehensive unit tests for `Translator.GetString()`, `GetPlural()`, `interpolateParams()`, and `LocaleManager` methods,
So that translation lookup, fallback, interpolation, and pluralization logic is verified.

**Acceptance Criteria:**
- [ ] GIVEN a Translator with nil manager WHEN `GetString(key)` is called THEN an error wrapping `ErrManagerNotInit` is returned
- [ ] GIVEN a valid Translator WHEN `GetString("existing_key")` is called THEN the translated string is returned with no error
- [ ] GIVEN a valid Translator for "es" WHEN `GetString("nonexistent_key")` is called THEN it falls back to the default language ("en") value
- [ ] GIVEN a valid Translator for the default language WHEN `GetString("nonexistent_key")` is called THEN an error wrapping `ErrKeyNotFound` is returned
- [ ] GIVEN a string with `{user}` placeholder WHEN `GetString(key, params{"user": "Alice"})` is called THEN `{user}` is replaced with "Alice"
- [ ] GIVEN a Translator with nil manager WHEN `GetPlural(key, count)` is called THEN an error wrapping `ErrManagerNotInit` is returned
- [ ] GIVEN a LocaleManager WHEN `IsLanguageSupported("en")` is called THEN true is returned
- [ ] GIVEN a LocaleManager WHEN `IsLanguageSupported("zz")` is called THEN false is returned
- [ ] GIVEN a LocaleManager WHEN `GetDefaultLanguage()` is called THEN `"en"` is returned
- [ ] GIVEN a LocaleManager WHEN `GetStats()` is called THEN stats contain the number of loaded languages

**Edge Cases:**
- [ ] GetString with empty key -> returns error (key not found)
- [ ] GetString with params but string has no placeholders -> returns string unchanged
- [ ] GetString with params containing extra unused keys -> returns string with only matched placeholders replaced
- [ ] GetString with params containing key that maps to non-string value (int) -> value is converted to string via fmt.Sprintf
- [ ] interpolateParams with `%s` legacy format and params -> positional replacement works
- [ ] GetPlural with count=0, count=1, count=2, count=100 -> correct plural form selected
- [ ] Recursive fallback detection (translator falls back to itself) -> `ErrRecursiveFallback` returned
- [ ] GetStringSlice with nil manager -> `ErrManagerNotInit` returned

**Definition of Done:**
- [ ] Tests in `alita/i18n/i18n_test.go` are expanded (file already exists with test helpers)
- [ ] Use existing `newTestTranslator` helper pattern for test setup
- [ ] At least 15 new test cases added
- [ ] Tests pass without Redis (cacheClient is nil in tests, and code handles nil cacheClient)
- [ ] Tests use `t.Parallel()`

---

### US-012: Module Helper Function Tests

**Priority:** P0 (must-have)

As a developer,
I want unit tests for pure unexported functions in `alita/modules/`,
So that the largest package in the codebase (51% of code) has measurably higher coverage.

**Acceptance Criteria:**
- [ ] GIVEN no arguments WHEN `defaultUnmutePermissions()` is called THEN a `ChatPermissions` struct is returned with `CanSendMessages=true`, `CanChangeInfo=false`, `CanPinMessages=false`, `CanManageTopics=false`
- [ ] GIVEN a nil ChatFullInfo pointer WHEN `resolveUnmutePermissions(nil)` is called THEN `defaultUnmutePermissions()` result is returned
- [ ] GIVEN a ChatFullInfo with nil Permissions WHEN `resolveUnmutePermissions(chatInfo)` is called THEN `defaultUnmutePermissions()` result is returned
- [ ] GIVEN a ChatFullInfo with non-nil Permissions WHEN `resolveUnmutePermissions(chatInfo)` is called THEN the chatInfo.Permissions is returned
- [ ] GIVEN a moduleEnabled with Init() called WHEN `Store("admin", true)` then `Load("admin")` are called THEN `("admin", true)` is returned
- [ ] GIVEN a moduleEnabled with Init() called WHEN `Load("nonexistent")` is called THEN `("nonexistent", false)` is returned
- [ ] GIVEN a moduleEnabled with 3 modules stored (2 enabled, 1 disabled) WHEN `LoadModules()` is called THEN a slice of 2 enabled module names is returned
- [ ] GIVEN a moduleEnabled with Init() called and no modules stored WHEN `LoadModules()` is called THEN an empty slice is returned

**Edge Cases:**
- [ ] resolveUnmutePermissions with ChatFullInfo that has Permissions with all-false fields -> returns all-false permissions
- [ ] moduleEnabled Store with empty string key -> stores and loads with empty key (map allows it)
- [ ] moduleEnabled Store same module twice with different values -> second value overwrites first
- [ ] LoadModules result order is NOT guaranteed (map iteration) -> test should use `slices.Contains` not index comparison
- [ ] listModules returns sorted output -> test the sort guarantees after populating modules

**Definition of Done:**
- [ ] New test file `alita/modules/chat_permissions_test.go` for permission helpers
- [ ] New test file `alita/modules/help_test.go` or expanded existing test for moduleEnabled
- [ ] Tests are in `package modules` (internal tests, accessing unexported functions)
- [ ] Tests use `t.Parallel()`
- [ ] At least 12 test cases across the module helper functions

---

### US-013: Extraction Function Tests

**Priority:** P1 (should-have)

As a developer,
I want additional unit tests for functions in `alita/utils/extraction/extraction.go`,
So that user extraction, time parsing, and mention ID extraction are verified with more edge cases.

**Acceptance Criteria:**
- [ ] GIVEN a message with a reply to another user WHEN `ExtractUser()` is called THEN the replied-to user's ID is extracted
- [ ] GIVEN a message with a @username mention WHEN `ExtractUser()` is called THEN the mentioned user's details are extracted
- [ ] GIVEN a message with a numeric user ID argument WHEN `ExtractUser()` is called THEN the user ID is parsed correctly
- [ ] GIVEN a time string "2h30m" WHEN `ExtractTime()` is called THEN the duration is correctly parsed
- [ ] GIVEN a message with text_mention entity WHEN `ExtractIDFromMention()` is called THEN the user ID from the entity is returned

**Edge Cases:**
- [ ] ExtractUser with no reply, no arguments, no mentions -> returns appropriate zero value or error
- [ ] ExtractUser with channel message (negative ID) -> handles correctly per IsChannelId rules
- [ ] ExtractTime with invalid format "abc" -> returns error
- [ ] ExtractTime with zero duration "0s" -> returns zero duration
- [ ] ExtractTime with negative duration -> handles per implementation
- [ ] ExtractIDFromMention with both Entities and CaptionEntities -> both are checked (per CLAUDE.md rules)
- [ ] Message with nil Entities slice -> no panic

**Definition of Done:**
- [ ] Tests in `alita/utils/extraction/extraction_test.go` are expanded
- [ ] Tests construct `gotgbot.Message` structs directly (no API calls needed)
- [ ] At least 10 new test cases
- [ ] Tests pass in CI with env vars set

---

### US-014: Monitoring Pure Function Tests

**Priority:** P1 (should-have)

As a developer,
I want unit tests for pure functions in `alita/utils/monitoring/`,
So that system stats collection, remediation action metadata, and threshold calculations are verified.

**Acceptance Criteria:**
- [ ] GIVEN the Go runtime is running WHEN `CollectSystemStats()` is called THEN a non-nil stats struct is returned with `NumGoroutine > 0` and `MemAllocMB >= 0`
- [ ] GIVEN each remediation action type WHEN `Name()` is called THEN a non-empty string is returned
- [ ] GIVEN each remediation action type WHEN `Severity()` is called THEN a valid severity level is returned
- [ ] GIVEN a memory usage below threshold WHEN `CanExecute()` is called on GCTriggerAction THEN false is returned
- [ ] GIVEN a memory usage above threshold WHEN `CanExecute()` is called on GCTriggerAction THEN true is returned

**Edge Cases:**
- [ ] CollectSystemStats on a system with very few goroutines -> values are still positive
- [ ] Remediation action Execute with nil dependencies -> no panic (defensive)

**Definition of Done:**
- [ ] Tests in `alita/utils/monitoring/` test files are expanded
- [ ] Tests for pure stat collection do not require config package (or handle init gracefully)
- [ ] At least 8 new test cases

---

### US-015: Migration SQL Processing Tests

**Priority:** P1 (should-have)

As a developer,
I want unit tests for `cleanSupabaseSQL()`, `splitSQLStatements()`, and `getMigrationFiles()` in `alita/db/migrations.go`,
So that SQL cleaning and splitting logic is verified without requiring a database.

**Acceptance Criteria:**
- [ ] GIVEN SQL with Supabase GRANT statements WHEN `cleanSupabaseSQL()` is called THEN GRANT statements targeting anon/authenticated/service_role are removed
- [ ] GIVEN SQL with CREATE TABLE WHEN `cleanSupabaseSQL()` is called THEN it is transformed to CREATE TABLE IF NOT EXISTS
- [ ] GIVEN SQL with CREATE INDEX WHEN `cleanSupabaseSQL()` is called THEN it is transformed to CREATE INDEX IF NOT EXISTS
- [ ] GIVEN SQL with CREATE UNIQUE INDEX WHEN `cleanSupabaseSQL()` is called THEN it is transformed to CREATE UNIQUE INDEX IF NOT EXISTS
- [ ] GIVEN SQL with CREATE TYPE ... AS ENUM WHEN `cleanSupabaseSQL()` is called THEN it is wrapped in a DO block with exception handling
- [ ] GIVEN SQL with two statements separated by `;` WHEN `splitSQLStatements()` is called THEN a slice of 2 strings is returned
- [ ] GIVEN SQL with a semicolon inside a single-quoted string WHEN `splitSQLStatements()` is called THEN the string is not split on the internal semicolon
- [ ] GIVEN SQL with dollar-quoted strings WHEN `splitSQLStatements()` is called THEN dollar-quoted content is preserved as one statement
- [ ] GIVEN SQL with `--` line comments WHEN `splitSQLStatements()` is called THEN comments do not interfere with splitting
- [ ] GIVEN SQL with `/* */` block comments WHEN `splitSQLStatements()` is called THEN block comments do not interfere with splitting

**Edge Cases:**
- [ ] Empty SQL string -> empty results
- [ ] SQL with only comments and whitespace -> empty results
- [ ] SQL with nested quotes (escaped single quotes `''`) -> handled correctly
- [ ] getMigrationFiles with non-existent path -> returns error
- [ ] getMigrationFiles with empty directory -> returns empty slice
- [ ] cleanSupabaseSQL with ALTER TABLE ADD CONSTRAINT -> wrapped in idempotent DO block
- [ ] cleanSupabaseSQL with Supabase-only extensions (hypopg, pg_graphql, etc.) -> extension statements removed

**Definition of Done:**
- [ ] Tests in `alita/db/migrations_test.go` are expanded (file exists)
- [ ] Tests for cleanSupabaseSQL and splitSQLStatements are pure unit tests (no DB needed)
- [ ] Tests for getMigrationFiles use temporary directories created with `t.TempDir()`
- [ ] At least 15 test cases covering the SQL processing functions
- [ ] Tests pass without PostgreSQL for the pure functions

---

### US-016: Helpers Package Additional Tests

**Priority:** P1 (should-have)

As a developer,
I want additional unit tests for pure functions in `alita/utils/helpers/helpers.go`,
So that message splitting, HTML reversal, and formatting helpers have higher coverage.

**Acceptance Criteria:**
- [ ] GIVEN a message shorter than MaxMessageLength (4096 chars) WHEN `SplitMessage()` is called THEN a single-element slice is returned
- [ ] GIVEN a message longer than MaxMessageLength WHEN `SplitMessage()` is called THEN a multi-element slice is returned with each element <= MaxMessageLength
- [ ] GIVEN a message with newlines WHEN `SplitMessage()` is called THEN splitting happens on newline boundaries
- [ ] GIVEN HTML with `<a href="...">text</a>` WHEN `ReverseHTML2MD()` is called THEN it is converted to `[text](url)` markdown format
- [ ] GIVEN HTML with `<b>text</b>` WHEN `ReverseHTML2MD()` is called THEN it is converted to `*text*` or `**text**` markdown
- [ ] GIVEN `Shtml()` is called THEN the returned opts have ParseMode="HTML", disabled link preview, and AllowSendingWithoutReply=true
- [ ] GIVEN `Smarkdown()` is called THEN the returned opts have ParseMode="Markdown"

**Edge Cases:**
- [ ] SplitMessage with empty string -> single-element slice with empty string (or empty slice)
- [ ] SplitMessage with exactly MaxMessageLength chars -> single-element slice
- [ ] SplitMessage with MaxMessageLength+1 chars and no newlines -> forced split at boundary
- [ ] SplitMessage with unicode characters (multi-byte) -> splits on rune count, not byte count
- [ ] ReverseHTML2MD with nested tags -> handles innermost tags
- [ ] ChunkKeyboardSlices with empty input -> returns empty result
- [ ] ChunkKeyboardSlices with items not divisible by chunk size -> last chunk is smaller

**Definition of Done:**
- [ ] Tests in `alita/utils/helpers/helpers_test.go` are expanded
- [ ] Tests pass in CI with env vars set (helpers imports config)
- [ ] At least 12 new test cases
- [ ] Tests use `t.Parallel()` where safe

---

### US-017: Chat Status Helper Tests

**Priority:** P2 (nice-to-have)

As a developer,
I want additional unit tests for functions in `alita/utils/chat_status/`,
So that ID validation, permission checking helpers, and chat type guards have higher coverage.

**Acceptance Criteria:**
- [ ] GIVEN a positive int64 WHEN `IsValidUserId(123)` is called THEN true is returned (already tested, verify edge cases)
- [ ] GIVEN a large negative int64 (<-1000000000000) WHEN `IsChannelId(id)` is called THEN true is returned
- [ ] GIVEN a small negative int64 (>-1000000000000) WHEN `IsChannelId(id)` is called THEN false is returned
- [ ] GIVEN 0 WHEN `IsValidUserId(0)` is called THEN false is returned

**Edge Cases:**
- [ ] IsValidUserId with int64 max value -> true
- [ ] IsChannelId with exactly -1000000000000 -> test boundary
- [ ] IsValidUserId with negative non-channel ID -> false

**Definition of Done:**
- [ ] Tests in `alita/utils/chat_status/chat_status_test.go` are expanded
- [ ] At least 6 new test cases for boundary conditions
- [ ] Tests pass in CI

---

### US-018: DB Integration Tests for Remaining CRUD Operations

**Priority:** P0 (must-have)

As a developer,
I want comprehensive integration tests for DB operations that currently have low or no coverage: `disable_db.go`, `notes_db.go`, `rules_db.go`, `admin_db.go`, `connections_db.go`,
So that the core CRUD paths for these modules are verified against a real database.

**Acceptance Criteria:**
- [ ] GIVEN no disable settings for a chat WHEN `GetDisabledCommands(chatID)` is called THEN an empty result is returned
- [ ] GIVEN a command is disabled WHEN `GetDisabledCommands(chatID)` is called THEN the disabled command is in the result
- [ ] GIVEN no notes for a chat WHEN `GetAllNotes(chatID)` is called THEN an empty slice is returned
- [ ] GIVEN a note is saved WHEN `GetNote(chatID, noteName)` is called THEN the note is returned with correct content
- [ ] GIVEN no rules for a chat WHEN `GetRules(chatID)` is called THEN default/empty rules are returned
- [ ] GIVEN rules are set WHEN `GetRules(chatID)` is called THEN the rules text is returned
- [ ] GIVEN admin settings are created WHEN `GetAdminSettings(chatID)` is called THEN the settings are returned
- [ ] GIVEN a connection is created WHEN `GetConnection(userID)` is called THEN the connected chat ID is returned
- [ ] GIVEN a connection exists WHEN `DisconnectId(userID)` is called THEN subsequent GetConnection returns no active connection

**Edge Cases:**
- [ ] Disable the same command twice -> idempotent, no duplicate entries
- [ ] Save a note with the same name twice -> update/overwrite, not duplicate
- [ ] GetNote with case-insensitive name matching -> verify behavior
- [ ] Delete a note that does not exist -> no error or appropriate error
- [ ] Connection for user with no prior connections -> default/not-found
- [ ] Admin settings with AnonAdmin toggle -> boolean false is persisted (UPSERT pattern)

**Definition of Done:**
- [ ] Existing test files (`disable_db_test.go`, `notes_db_test.go`, `rules_db_test.go`, `admin_db_test.go`, `connections_db_test.go`) are expanded
- [ ] All tests use `skipIfNoDb(t)` and unique IDs
- [ ] Full CRUD lifecycle tested for each module
- [ ] Tests pass in CI
- [ ] At least 20 new test cases across these files

## Non-Functional Requirements

### NFR-001: CI Coverage Threshold

- **Metric:** `go tool cover -func=coverage.out | grep '^total:'` SHALL report >= 40.0% (hard floor) and SHOULD report >= 45.0% (target)
- **Verification:** CI pipeline `Check coverage threshold` step passes. Manual verification: run `make test` locally with appropriate env vars, then check `go tool cover -func=coverage.out`.

### NFR-002: Test Execution Time

- **Metric:** Total test suite execution time SHALL remain under 10 minutes (current CI timeout)
- **Verification:** CI `Run test suite` step completes within the 20-minute job timeout. The `make test` command has a 10-minute timeout built in. No single test SHALL take more than 60 seconds.

### NFR-003: Test Isolation

- **Metric:** Every test SHALL be independently runnable. No test SHALL depend on the execution order or side effects of another test.
- **Verification:** Run tests with `-count=1 -shuffle=on` and verify all pass. Run any single test in isolation and verify it passes.

### NFR-004: Race Condition Freedom

- **Metric:** Zero data races detected by the Go race detector
- **Verification:** `go test -race ./...` completes with no race warnings. This is already enforced by the `-race` flag in `make test`.

### NFR-005: No Redis Dependency for Non-Cache Tests

- **Metric:** All unit tests and non-cache integration tests SHALL pass when Redis is unavailable (CI has no Redis service)
- **Verification:** Tests pass in CI where `REDIS_ADDRESS` is set to `localhost:6379` but no Redis server is running. Cache-dependent code paths return nil/zero values gracefully.

### NFR-006: Test Naming Convention

- **Metric:** All test function names SHALL follow the pattern `Test[FunctionName]` or `Test[TypeName]_[MethodName]`. All subtests SHALL have descriptive names.
- **Verification:** `go test -v ./...` output shows clear, descriptive test names. No unnamed subtests.

### NFR-007: No Test Pollution of Production Data

- **Metric:** All DB integration tests SHALL use unique IDs generated via `time.Now().UnixNano()` and SHALL clean up via `t.Cleanup()`.
- **Verification:** Code review confirms all DB test functions use the `skipIfNoDb` + unique ID + `t.Cleanup` pattern from `testmain_test.go`.

## Dependencies

| Dependency | Required By | Risk if Unavailable |
|-----------|------------|-------------------|
| PostgreSQL 16 (CI service) | US-008, US-009, US-010, US-018 | DB integration tests are skipped; ~1,000+ statements uncovered; likely miss 45% target |
| CI environment variables (BOT_TOKEN, OWNER_ID, etc.) | US-005, US-006, US-007, US-011, US-012, US-013, US-016 | Tests in packages that import `alita/config` fail with `log.Fatalf`; ~2,000+ statements uncovered |
| gotgbot/v2 structs | US-012, US-013 | Cannot construct test message/chat structs; moderate impact on modules coverage |
| Embedded locale YAML files (go:embed) | US-011 | i18n tests cannot load translations; ~200 statements uncovered |
| `alita/db/testmain_test.go` TestMain harness | US-008, US-009, US-010, US-018 | DB tests cannot run AutoMigrate; all DB integration tests fail |
| go test `-coverpkg=./...` flag | All | Without cross-package coverage measurement, reported total coverage is lower than actual |

## Assumptions

1. **CI environment variables remain as documented in ci.yml** -- if CI stops setting `BOT_TOKEN=test-token` and other env vars, all tests in packages importing `alita/config` will crash with `log.Fatalf`. Impact: ~70% of new tests would fail.
2. **PostgreSQL 16 service continues to be available in CI** -- if removed, all DB integration tests (~1,000 statements) are skipped. Impact: coverage drops by ~10-12 percentage points.
3. **The `-coverpkg=./...` flag continues to be used** -- this flag measures cross-package coverage (a test in package A exercising code in package B counts for B). Without it, per-package percentages are different. Impact: reported total coverage may appear lower without the flag.
4. **No Redis service is added to CI** -- tests are designed to gracefully handle nil cache. If Redis were added, additional cache-layer tests could contribute ~2-3% more coverage. Impact: minor upside missed.
5. **gotgbot/v2 struct constructors remain public** -- tests construct `gotgbot.Message`, `gotgbot.ChatFullInfo`, etc. directly. If these become private, extraction and module tests break. Impact: US-012, US-013 would need rewriting.
6. **The 45% target assumes ~11,288 total statements** -- the actual statement count depends on compiler version and build tags. If the count is higher (e.g., 13,000), more tests would be needed. Impact: may need to add 10-15% more test cases.
7. **Existing tests continue to pass** -- if existing tests break (e.g., due to upstream dependency changes), the total coverage could decrease. Impact: rework existing tests first before adding new ones.

## Open Questions

- [ ] What is the exact statement count from `go tool cover -func=coverage.out`? The 12.5% is from CI, but the denominator matters for calculating how many new statements we need. -- blocks accurate estimation for all stories
- [ ] Should `config.init()` be refactored to not use `log.Fatalf`? This would unblock local testing for all packages that transitively import config. Currently a separate PR is recommended. -- blocks nothing (CI has env vars), but would improve developer experience for US-005, US-006, US-007
- [ ] Is there an existing gotgbot mock or testing utility? The gotgbot library does not ship one. Creating a minimal mock HTTP server for `Bot.GetChat()` etc. would unlock handler-level testing (~51% of code). -- blocks future coverage beyond 50%, does not block current scope
- [ ] Can we add Redis to CI to enable cache integration tests? -- blocks cache-layer testing, does not block current scope (tests handle nil cache)
- [ ] Are there any planned schema changes that would break DB integration tests? -- blocks US-008, US-009, US-010, US-018 if migrations change model shapes

## Glossary

| Term | Definition |
|------|-----------|
| Coverage | Percentage of Go source statements executed during `go test` runs, as measured by `go tool cover -func` |
| Statement | A single executable line in compiled Go code; the unit of coverage measurement |
| `-coverpkg=./...` | Go test flag that measures coverage across ALL packages in the module, not just the package under test |
| `skipIfNoDb` | Test helper function in `alita/db/testmain_test.go` that skips a test when PostgreSQL is unavailable |
| TestMain | Go testing entry point that runs before any tests in a package; used in `alita/db/` to set up AutoMigrate |
| Pure function | A function with no side effects and no external dependencies (DB, cache, API); always safe to unit test |
| Integration test | A test that requires external infrastructure (PostgreSQL, Redis, HTTP server) to execute |
| Table-driven test | Go testing pattern where test cases are defined as a slice of structs and iterated with `t.Run()` |
| GORM custom type | A Go type that implements `database/sql.Scanner` and `database/sql/driver.Valuer` for custom DB serialization |
| Stampede protection | Mechanism (via `singleflight`) that prevents multiple goroutines from executing the same cache-miss query simultaneously |
| Trophy testing | Testing strategy that prioritizes integration tests (the "trophy" shape) over pure unit tests, focusing on the layer that provides the most confidence |
| UPSERT | Database operation that inserts a row if it does not exist, or updates it if it does (INSERT ... ON CONFLICT DO UPDATE) |

REQUIREMENTS_COMPLETE
