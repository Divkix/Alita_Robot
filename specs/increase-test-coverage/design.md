# Technical Design: Increase Test Coverage to 45%+

**Date:** 2026-02-21
**Requirements Source:** requirements.md
**Codebase Conventions:** Table-driven tests with `t.Parallel()`, `t.Run()` subtests; DB tests use `skipIfNoDb(t)` + `time.Now().UnixNano()` unique IDs + `t.Cleanup()`; internal package tests for unexported functions; no mocking of Telegram API.

## Design Overview

The codebase sits at 12.5% test coverage against a CI threshold of 40%. The target is 45% for safety margin. The coverage is measured with `-coverpkg=./...` which counts cross-package coverage -- a test in `alita/db/` that exercises `alita/config/` code contributes to `alita/config/`'s coverage.

The strategy is to maximize statement coverage with minimum new test infrastructure. No new mocking frameworks, no new test utilities beyond what exists. The work falls into three categories: (1) pure unit tests for types, helpers, and SQL processing that need no external dependencies; (2) DB integration tests that follow the existing `TestMain`/`skipIfNoDb` pattern; (3) config and i18n tests that run in CI where env vars are set. The `alita/modules/` package at 17K LOC and ~2% coverage is the elephant; we target its unexported pure functions since handler-level testing requires Telegram API mocking (out of scope).

Every new test file follows existing conventions exactly: table-driven, `t.Parallel()` at both top and subtest level, descriptive subtest names, `t.Fatalf` for assertion failures. No new test helpers or abstractions are introduced.

## Component Architecture

### Component: GORM Custom Type Unit Tests (US-001, US-002, US-003, US-004)

**Responsibility:** Verify serialization/deserialization correctness for `ButtonArray`, `StringArray`, `Int64Array` Scan/Value methods, all `TableName()` methods, `BlacklistSettingsSlice` methods, `getSpanAttributes()`, and `NotesSettings.PrivateNotesEnabled()`.
**Location:** `/Users/divkix/GitHub/Alita_Robot/alita/db/gorm_types_test.go` (NEW)
**Pattern:** Pure unit tests, same as `alita/config/types_test.go` -- no DB, no external deps.

**Public Interface:**
```go
// Test functions in package db

func TestButtonArray_Scan(t *testing.T)     // US-001
func TestButtonArray_Value(t *testing.T)    // US-001
func TestStringArray_Scan(t *testing.T)     // US-001
func TestStringArray_Value(t *testing.T)    // US-001
func TestInt64Array_Scan(t *testing.T)      // US-001
func TestInt64Array_Value(t *testing.T)     // US-001
func TestTableNames(t *testing.T)           // US-002
func TestBlacklistSettingsSlice_Triggers(t *testing.T) // US-003
func TestBlacklistSettingsSlice_Action(t *testing.T)   // US-003
func TestBlacklistSettingsSlice_Reason(t *testing.T)   // US-003
func TestGetSpanAttributes(t *testing.T)    // US-004
func TestNotesSettings_PrivateNotesEnabled(t *testing.T) // US-004
```

**Dependencies:**
- `encoding/json` -- for constructing valid/invalid JSON test data
- `database/sql/driver` -- for verifying `Value()` return types
- `go.opentelemetry.io/otel/attribute` -- for `getSpanAttributes` return type checks

**Error Handling:**
- `Scan(nil)` -> empty array, no error
- `Scan(non-[]byte)` -> error containing "type assertion"
- `Scan(malformed JSON)` -> JSON unmarshal error
- `Value()` on empty -> `"[]"`, no error

**Test Cases (ButtonArray.Scan -- representative of all three types):**

| # | Input | Expected Result | Expected Error |
|---|-------|----------------|---------------|
| 1 | `nil` | `ButtonArray{}` | `nil` |
| 2 | `[]byte("[{\"name\":\"btn\",\"url\":\"http://x\"}]")` | 1-element array | `nil` |
| 3 | `"string"` (not []byte) | unchanged | contains "type assertion" |
| 4 | `[]byte("{invalid")` | unchanged | JSON unmarshal error |
| 5 | `[]byte("null")` | `ButtonArray(nil)` or `ButtonArray{}` | `nil` |
| 6 | `[]byte("")` | unchanged | JSON unmarshal error |
| 7 | `[]byte` with special chars in Name/Url | correctly deserialized | `nil` |

**Test Cases (ButtonArray.Value -- representative):**

| # | Input | Expected Result | Expected Error |
|---|-------|----------------|---------------|
| 1 | `ButtonArray{}` (empty) | `"[]"` | `nil` |
| 2 | `ButtonArray{{Name:"a",Url:"b"}}` | valid JSON bytes | `nil` |
| 3 | `ButtonArray{{Name:"",Url:""}}` | valid JSON with empty strings | `nil` |

**Test Cases (Int64Array-specific edge cases):**

| # | Input | Expected | Notes |
|---|-------|----------|-------|
| 1 | `[]byte("[9223372036854775807]")` | `Int64Array{math.MaxInt64}` | max int64 |
| 2 | `[]byte("[-9223372036854775808]")` | `Int64Array{math.MinInt64}` | min int64 |
| 3 | `[]byte("[0,-1,1]")` | `Int64Array{0,-1,1}` | mixed signs |

**Test Cases (TableNames -- US-002):**

Single table-driven test with struct slice:
```go
tests := []struct {
    name      string
    model     interface{ TableName() string }
    wantTable string
}{
    {"User", User{}, "users"},
    {"Chat", Chat{}, "chats"},
    {"WarnSettings", WarnSettings{}, "warns_settings"},
    {"Warns", Warns{}, "warns_users"},
    {"GreetingSettings", GreetingSettings{}, "greetings"},
    {"ChatFilters", ChatFilters{}, "filters"},
    {"AdminSettings", AdminSettings{}, "admin"},
    {"BlacklistSettings", BlacklistSettings{}, "blacklists"},
    {"PinSettings", PinSettings{}, "pins"},
    {"ReportChatSettings", ReportChatSettings{}, "report_chat_settings"},
    {"ReportUserSettings", ReportUserSettings{}, "report_user_settings"},
    {"DevSettings", DevSettings{}, "devs"},
    {"ChannelSettings", ChannelSettings{}, "channels"},
    {"AntifloodSettings", AntifloodSettings{}, "antiflood_settings"},
    {"ConnectionSettings", ConnectionSettings{}, "connection"},
    {"ConnectionChatSettings", ConnectionChatSettings{}, "connection_settings"},
    {"DisableSettings", DisableSettings{}, "disable"},
    {"DisableChatSettings", DisableChatSettings{}, "disable_chat_settings"},
    {"RulesSettings", RulesSettings{}, "rules"},
    {"LockSettings", LockSettings{}, "locks"},
    {"NotesSettings", NotesSettings{}, "notes_settings"},
    {"Notes", Notes{}, "notes"},
    {"CaptchaSettings", CaptchaSettings{}, "captcha_settings"},
    {"CaptchaAttempts", CaptchaAttempts{}, "captcha_attempts"},
    {"StoredMessages", StoredMessages{}, "stored_messages"},
    {"CaptchaMutedUsers", CaptchaMutedUsers{}, "captcha_muted_users"},
    {"SchemaMigration", SchemaMigration{}, "schema_migrations"},
}
```

**Test Cases (BlacklistSettingsSlice -- US-003):**

| Method | Input | Expected |
|--------|-------|----------|
| `Triggers()` | empty slice | `nil` or `[]string(nil)` |
| `Triggers()` | 3 entries with Words "a","b","c" | `[]string{"a","b","c"}` |
| `Action()` | empty slice | `"warn"` |
| `Action()` | slice with first Action="ban" | `"ban"` |
| `Action()` | slice with first Action="" | `""` (not default, because len > 0) |
| `Reason()` | empty slice | `"Blacklisted word: '%s'"` |
| `Reason()` | slice with first Reason="spam" | `"spam"` |
| `Reason()` | slice with first Reason="" | `"Blacklisted word: '%s'"` |

**Test Cases (getSpanAttributes -- US-004):**

| Input | Expected |
|-------|----------|
| `nil` | empty `[]attribute.KeyValue{}` |
| `&User{}` | 1 attribute with key "db.model", value "*db.User" |
| `"hello"` (string, not pointer) | 1 attribute with key "db.model", value "string" |

**Test Cases (PrivateNotesEnabled -- US-004):**

| Input | Expected |
|-------|----------|
| `&NotesSettings{Private: true}` | `true` |
| `&NotesSettings{Private: false}` | `false` |

**Estimated statements covered:** ~120

---

### Component: Config ValidateConfig and setDefaults Tests (US-005, US-006, US-007)

**Responsibility:** Verify all validation branches in `ValidateConfig()`, all default assignments in `setDefaults()`, and Redis address/password parsing.
**Location:** `/Users/divkix/GitHub/Alita_Robot/alita/config/config_test.go` (NEW)
**Pattern:** Same as `alita/config/types_test.go`. Tests are in `package config` which triggers `init()` -- CI has required env vars set. Guard with skip check for local dev.

**Public Interface:**
```go
func TestValidateConfig(t *testing.T)      // US-005
func TestSetDefaults(t *testing.T)         // US-006
func TestGetRedisAddress(t *testing.T)     // US-007
func TestGetRedisPassword(t *testing.T)    // US-007
```

**Dependencies:**
- `os` -- for `t.Setenv()` in Redis tests
- `strings` -- for error message checking

**Error Handling:**
- Each validation branch returns `fmt.Errorf` with a specific substring that tests check via `strings.Contains`

**Test Cases (ValidateConfig -- US-005):**

```go
// validBaseConfig returns a Config that passes all validation.
// Each test case overrides one field to trigger a specific error.
func validBaseConfig() *Config {
    return &Config{
        BotToken:               "test-token",
        OwnerId:                1,
        MessageDump:            1,
        DatabaseURL:            "postgres://localhost/test",
        RedisAddress:           "localhost:6379",
        HTTPPort:               8080,
        ChatValidationWorkers:  10,
        DatabaseWorkers:        5,
        MessagePipelineWorkers: 4,
        BulkOperationWorkers:   4,
        CacheWorkers:           3,
        StatsCollectionWorkers: 2,
        MaxConcurrentOperations: 50,
        OperationTimeoutSeconds: 30,
    }
}
```

| # | Field Override | Expected Error Substring |
|---|---------------|-------------------------|
| 1 | `BotToken=""` | `"BOT_TOKEN is required"` |
| 2 | `OwnerId=0` | `"OWNER_ID"` |
| 3 | `MessageDump=0` | `"MESSAGE_DUMP"` |
| 4 | `DatabaseURL=""` | `"DATABASE_URL"` |
| 5 | `RedisAddress=""` | `"REDIS_ADDRESS"` |
| 6 | `UseWebhooks=true, WebhookDomain=""` | `"WEBHOOK_DOMAIN"` |
| 7 | `UseWebhooks=true, WebhookDomain="x", WebhookSecret=""` | `"WEBHOOK_SECRET"` |
| 8 | `HTTPPort=0` | `"HTTP_PORT"` |
| 9 | `HTTPPort=70000` | `"HTTP_PORT"` |
| 10 | `ChatValidationWorkers=0` | `"CHAT_VALIDATION_WORKERS"` |
| 11 | `ChatValidationWorkers=101` | `"CHAT_VALIDATION_WORKERS"` |
| 12 | `DatabaseWorkers=0` | `"DATABASE_WORKERS"` |
| 13 | `MessagePipelineWorkers=0` | `"MESSAGE_PIPELINE_WORKERS"` |
| 14 | `BulkOperationWorkers=0` | `"BULK_OPERATION_WORKERS"` |
| 15 | `CacheWorkers=0` | `"CACHE_WORKERS"` |
| 16 | `StatsCollectionWorkers=0` | `"STATS_COLLECTION_WORKERS"` |
| 17 | `MaxConcurrentOperations=0` | `"MAX_CONCURRENT_OPERATIONS"` |
| 18 | `OperationTimeoutSeconds=0` | `"OPERATION_TIMEOUT_SECONDS"` |
| 19 | `OperationTimeoutSeconds=301` | `"OPERATION_TIMEOUT_SECONDS"` |
| 20 | `DBMaxIdleConns=101` | `"DB_MAX_IDLE_CONNS"` |
| 21 | `DBMaxOpenConns=1001` | `"DB_MAX_OPEN_CONNS"` |
| 22 | All valid (no override) | `nil` |
| 23 | Boundary: all workers at min (1) | `nil` |
| 24 | Boundary: all workers at max | `nil` |
| 25 | `UseWebhooks=false, WebhookDomain=""` | `nil` (webhook validation skipped) |
| 26 | `DispatcherMaxRoutines=0` | `nil` (0 means use default) |
| 27 | `DBMaxIdleConns=0` | `nil` (0 means use default) |
| 28 | `MaxConcurrentOperations=-1` | `"MAX_CONCURRENT_OPERATIONS"` |

**Test Cases (setDefaults -- US-006):**

| # | Scenario | Field | Expected Value |
|---|----------|-------|---------------|
| 1 | Zero Config | `ApiServer` | `"https://api.telegram.org"` |
| 2 | Zero Config | `WorkingMode` | `"worker"` |
| 3 | Zero Config | `RedisAddress` | `"localhost:6379"` |
| 4 | Zero Config | `RedisDB` | `1` |
| 5 | Zero Config | `HTTPPort` | `8080` |
| 6 | `WebhookPort=9090, HTTPPort=0` | `HTTPPort` | `9090` |
| 7 | `HTTPPort=3000` | `HTTPPort` | `3000` (preserved) |
| 8 | Zero Config | `ChatValidationWorkers` | `10` |
| 9 | Zero Config | `DatabaseWorkers` | `5` |
| 10 | Zero Config | `DBMaxIdleConns` | `50` |
| 11 | Zero Config | `DBMaxOpenConns` | `200` |
| 12 | Zero Config | `DBConnMaxLifetimeMin` | `240` |
| 13 | Zero Config | `DBConnMaxIdleTimeMin` | `60` |
| 14 | Zero Config | `MigrationsPath` | `"migrations"` |
| 15 | Zero Config | `ClearCacheOnStartup` | `true` |
| 16 | `Debug=false` zero Config | `EnablePerformanceMonitoring` | `true` |
| 17 | `Debug=false` zero Config | `EnableBackgroundStats` | `true` |
| 18 | Pre-set `ApiServer="custom"` | `ApiServer` | `"custom"` (preserved) |
| 19 | Pre-set `RedisDB=5` | `RedisDB` | `5` (preserved) |
| 20 | `ClearCacheOnStartup=false` before call | `ClearCacheOnStartup` | `true` (unconditional) |
| 21 | Zero Config | `MaxConcurrentOperations` | `50` |
| 22 | Zero Config | `OperationTimeoutSeconds` | `30` |
| 23 | Zero Config | `DispatcherMaxRoutines` | `200` |
| 24 | Zero Config | `ResourceMaxGoroutines` | `1000` |
| 25 | Zero Config | `ResourceMaxMemoryMB` | `500` |
| 26 | Zero Config | `ResourceGCThresholdMB` | `400` |

**Test Cases (getRedisAddress / getRedisPassword -- US-007):**

These tests use `t.Setenv()` and must NOT be `t.Parallel()`.

| # | Env Vars Set | Function | Expected |
|---|-------------|----------|----------|
| 1 | `REDIS_ADDRESS=myhost:1234` | `getRedisAddress()` | `"myhost:1234"` |
| 2 | `REDIS_ADDRESS=""`, `REDIS_URL=redis://user:pass@host:6380` | `getRedisAddress()` | `"host:6380"` |
| 3 | Both empty | `getRedisAddress()` | `""` |
| 4 | `REDIS_ADDRESS=x`, `REDIS_URL=redis://host:9999` | `getRedisAddress()` | `"x"` (priority) |
| 5 | `REDIS_URL=not-a-url` | `getRedisAddress()` | `""` (parse error) |
| 6 | `REDIS_PASSWORD=secret` | `getRedisPassword()` | `"secret"` |
| 7 | `REDIS_PASSWORD=""`, `REDIS_URL=redis://user:pass123@host:6380` | `getRedisPassword()` | `"pass123"` |
| 8 | Both empty | `getRedisPassword()` | `""` |
| 9 | `REDIS_URL=redis://host:6380` (no userinfo) | `getRedisPassword()` | `""` |
| 10 | `REDIS_URL=redis://user@host:6380` (user, no pass) | `getRedisPassword()` | `""` |

**Estimated statements covered:** ~150

---

### Component: DB Integration Tests for Optimized Queries (US-008)

**Responsibility:** Verify optimized query paths against real PostgreSQL.
**Location:** `/Users/divkix/GitHub/Alita_Robot/alita/db/optimized_queries_test.go` (NEW)
**Pattern:** Same as `captcha_db_test.go` -- `skipIfNoDb(t)`, unique IDs, `t.Cleanup()`.

**Public Interface:**
```go
func TestNewOptimizedLockQueries(t *testing.T)
func TestOptimizedLockQueries_GetLockStatus(t *testing.T)
func TestOptimizedLockQueries_GetChatLocksOptimized(t *testing.T)
func TestOptimizedLockQueries_NilDB(t *testing.T)
func TestNewOptimizedUserQueries(t *testing.T)
func TestOptimizedUserQueries_GetUserBasicInfo(t *testing.T)
func TestNewOptimizedChatQueries(t *testing.T)
func TestOptimizedChatQueries_GetChatBasicInfo(t *testing.T)
func TestOptimizedAntifloodQueries_GetAntifloodSettings(t *testing.T)
func TestOptimizedFilterQueries_GetChatFiltersOptimized(t *testing.T)
func TestOptimizedChannelQueries_GetChannelSettings(t *testing.T)
func TestNewCachedOptimizedQueries(t *testing.T)
func TestGetOptimizedQueries(t *testing.T)
func TestCacheKeyFormats(t *testing.T)
```

**Dependencies:**
- `alita/db` (internal -- same package)
- PostgreSQL via `DB` global

**Error Handling:**
- Nil DB field -> error containing "database not initialized"
- Non-existent chat ID -> default/empty result, no error
- Record not found -> `false`/`nil`/empty map, no error

**Test Cases:**

| Test | Setup | Action | Expected |
|------|-------|--------|----------|
| Lock nil DB | construct `OptimizedLockQueries{db: nil}` | `GetLockStatus(1, "sticker")` | `false, error "not initialized"` |
| Lock no record | valid queries, no lock rows | `GetLockStatus(chatID, "sticker")` | `false, nil` |
| Lock exists true | insert `LockSettings{ChatId: X, LockType: "sticker", Locked: true}` | `GetLockStatus(X, "sticker")` | `true, nil` |
| Lock exists false | insert with `Locked: false` | `GetLockStatus(X, "sticker")` | `false, nil` |
| GetChatLocks empty | no locks | `GetChatLocksOptimized(chatID)` | `map[string]bool{}`, nil |
| GetChatLocks 3 entries | insert 3 locks | `GetChatLocksOptimized(chatID)` | map with 3 entries |
| User nil DB | `OptimizedUserQueries{db: nil}` | `GetUserBasicInfo(1)` | `nil, error` |
| User not found | no user row | `GetUserBasicInfo(99)` | `&User{}, gorm.ErrRecordNotFound` |
| User found | insert user | `GetUserBasicInfo(userID)` | correct user |
| Chat nil DB | `OptimizedChatQueries{db: nil}` | `GetChatBasicInfo(1)` | `nil, error` |
| Antiflood defaults | no antiflood row | `GetAntifloodSettings(chatID)` | `Limit=0, Action="mute"` |
| Antiflood existing | insert antiflood row | `GetAntifloodSettings(chatID)` | matches inserted |
| Filter empty | no filters | `GetChatFiltersOptimized(chatID)` | empty slice |
| Filter 2 entries | insert 2 filters | `GetChatFiltersOptimized(chatID)` | 2-element slice |
| Channel nil DB | `OptimizedChannelQueries{db: nil}` | `GetChannelSettings(1)` | `nil, error` |
| Cache key format | N/A | `lockCacheKey(123, "sticker")` | `"alita:lock:123:sticker"` |
| Cache key format | N/A | `userCacheKey(456)` | `"alita:user:456"` |
| Cache key format | N/A | `chatCacheKey(789)` | `"alita:chat:789"` |
| GetOptimizedQueries | valid DB | call twice | same instance (singleton) |

**Estimated statements covered:** ~200

---

### Component: DB Integration Tests for Captcha (US-009, expanding existing)

**Responsibility:** Expand captcha CRUD tests to cover full lifecycle.
**Location:** `/Users/divkix/GitHub/Alita_Robot/alita/db/captcha_db_test.go` (MODIFY -- append new tests)
**Pattern:** Existing pattern in same file.

**New Test Functions:**
```go
func TestCaptchaAttempt_CreateAndGet(t *testing.T)
func TestCaptchaAttempt_IncrementAttempts(t *testing.T)
func TestCaptchaAttempt_DeleteNonExistent(t *testing.T)
func TestStoredMessages_CRUD(t *testing.T)
func TestCaptchaMutedUsers_CreateAndCleanup(t *testing.T)
func TestGetCaptchaSettings_ForNonExistentChat(t *testing.T)
```

**Test Cases:**

| Test | Action | Expected |
|------|--------|----------|
| Create attempt | `CreateCaptchaAttemptPreMessage(uid, cid, "42", 2)` | non-nil attempt, no error |
| Get attempt | `GetCaptchaAttemptByID(id)` after create | matches created |
| Increment | `IncrementCaptchaAttempts` after create | Attempts field increases by 1 |
| Delete non-existent | `DeleteCaptchaAttemptByIDAtomic(999999, uid, cid)` | `false, nil` |
| Stored messages | Create attempt, store 3 messages, retrieve | 3 messages returned |
| Muted user cleanup | Create muted user with past `UnmuteAt`, run cleanup | record removed |
| Non-existent chat | `GetCaptchaSettings(999)` | default settings (Enabled=false, Mode="math", etc.) |

**Estimated statements covered:** ~100

---

### Component: DB Integration Tests for Greetings (US-010, expanding existing)

**Responsibility:** Expand greeting tests to cover edge cases.
**Location:** `/Users/divkix/GitHub/Alita_Robot/alita/db/greetings_db_test.go` (MODIFY -- append)
**Pattern:** Existing pattern in same file.

**New Test Functions:**
```go
func TestGetGreetingSettings_NonExistentChat(t *testing.T)
func TestSetWelcomeText_EmptyText(t *testing.T)
func TestWelcomeAndGoodbye_Independent(t *testing.T)
func TestResetWelcomeText(t *testing.T)
```

**Test Cases:**

| Test | Action | Expected |
|------|--------|----------|
| Non-existent chat | `GetGreetingSettings(nonExistentChatID)` | default settings returned, not nil |
| Empty welcome text | `SetWelcomeText(chatID, "", "", nil, TEXT)` | persisted as empty string |
| Independent updates | Set welcome, then set goodbye | welcome unaffected by goodbye update |
| Reset welcome | Set custom then set back to `DefaultWelcome` | original default restored |

**Estimated statements covered:** ~60

---

### Component: DB Integration Tests for Remaining CRUD (US-018, expanding existing)

**Responsibility:** Expand tests for `disable_db.go`, `notes_db.go`, `rules_db.go`, `admin_db.go`, `connections_db.go`.
**Location:** Existing test files (MODIFY -- append to each)
**Pattern:** Existing patterns in respective files.

**Files Modified:**
- `/Users/divkix/GitHub/Alita_Robot/alita/db/disable_db_test.go` -- add `TestDisableSameCommandTwice`, `TestGetChatDisabledCMDsCached`
- `/Users/divkix/GitHub/Alita_Robot/alita/db/notes_db_test.go` -- add `TestAddNoteWithAdminOnly`, `TestGetNotesList_AdminVsNonAdmin`, `TestAddNoteWithAllFlags`
- `/Users/divkix/GitHub/Alita_Robot/alita/db/rules_db_test.go` -- add `TestClearRules`, `TestSetRules_EmptyString`
- `/Users/divkix/GitHub/Alita_Robot/alita/db/admin_db_test.go` -- add `TestGetAdminSettings_CreatesThenUpdates`
- `/Users/divkix/GitHub/Alita_Robot/alita/db/connections_db_test.go` -- add `TestConnectionForNewUser`, `TestGetChatConnectionSetting_Defaults`

**New Test Cases (across all files, ~20 total):**

| File | Test | Scenario |
|------|------|----------|
| disable | `TestDisableSameCommandTwice` | Disable cmd twice -> no duplicate, still 1 entry |
| disable | `TestGetChatDisabledCMDsCached` | Call cached version, verify result matches uncached |
| notes | `TestAddNoteWithAdminOnly` | Add with `adminOnly=true`, verify non-admin list excludes it |
| notes | `TestGetNotesList_AdminVsNonAdmin` | 3 notes (1 admin-only), admin sees 3, non-admin sees 2 |
| notes | `TestAddNoteWithAllFlags` | All boolean flags true, verify round-trip |
| rules | `TestClearRules` | Set rules then clear to empty, verify empty returned |
| rules | `TestSetRules_EmptyString` | `SetChatRules(chatID, "")`, verify empty persisted |
| admin | `TestGetAdminSettings_CreatesThenUpdates` | Get creates, then Set updates same record |
| connections | `TestConnectionForNewUser` | New user -> default Connected=false |
| connections | `TestGetChatConnectionSetting_Defaults` | New chat -> AllowConnect default |

**Estimated statements covered:** ~150

---

### Component: i18n Translator and Manager Tests (US-011)

**Responsibility:** Expand tests for `GetString()`, `GetPlural()`, `interpolateParams()`, `GetStringSlice()`, and manager methods.
**Location:** `/Users/divkix/GitHub/Alita_Robot/alita/i18n/i18n_test.go` (MODIFY -- append)
**Pattern:** Uses existing `newTestTranslator` helper.

**New Test Functions:**
```go
func TestTranslator_GetString_NilManager(t *testing.T)
func TestTranslator_GetString_FallbackToDefault(t *testing.T)
func TestTranslator_GetString_ParamInterpolation(t *testing.T)
func TestTranslator_GetString_NamedParams(t *testing.T)
func TestTranslator_GetString_UnusedParams(t *testing.T)
func TestTranslator_GetString_EmptyKey(t *testing.T)
func TestTranslator_GetPlural_NilManager(t *testing.T)
func TestTranslator_GetPlural_AllCounts(t *testing.T)
func TestTranslator_GetStringSlice_NilManager(t *testing.T)
func TestLocaleManager_IsLanguageSupported(t *testing.T)
func TestLocaleManager_GetDefaultLanguage(t *testing.T)
func TestLocaleManager_GetStats(t *testing.T)
func TestInterpolateParams_NamedPlaceholders(t *testing.T)
func TestInterpolateParams_LegacyFormat(t *testing.T)
func TestInterpolateParams_NoPlaceholders(t *testing.T)
```

**Test Cases:**

| Test | Setup | Expected |
|------|-------|----------|
| Nil manager | `Translator{manager: nil}` | `ErrManagerNotInit` |
| Fallback | "es" translator missing key, "en" has it | returns "en" value |
| Named params `{user}` | YAML: `"Hello, {user}!"`, params: `{"user": "Alice"}` | `"Hello, Alice!"` |
| Unused params | YAML: `"Static text"`, params: `{"extra": "val"}` | `"Static text"` unchanged |
| Empty key | valid translator, key="" | `ErrKeyNotFound` |
| GetPlural nil manager | `Translator{manager: nil}` | `ErrManagerNotInit` |
| GetPlural count=0 with Zero | YAML with `zero: "none"` | `"none"` |
| GetPlural count=100 | YAML with `other: "many"` | `"many"` |
| GetStringSlice nil manager | `Translator{manager: nil}` | `ErrManagerNotInit` |
| IsLanguageSupported "en" | manager with en locale data | `true` |
| IsLanguageSupported "zz" | manager without zz | `false` |
| GetDefaultLanguage | manager with defaultLang="en" | `"en"` |
| GetStats | manager with 4 languages | `total_languages: 4` |
| Legacy `%s` format | YAML: `"Hello, %s!"`, params: `{"0": "Bob"}` | `"Hello, Bob!"` |
| No placeholders + params | YAML: `"Plain"`, params: `{"0": "x"}` | `"Plain"` |

**Estimated statements covered:** ~200

---

### Component: Module Helper Function Tests (US-012)

**Responsibility:** Test pure unexported functions in `alita/modules/`.
**Location:**
- `/Users/divkix/GitHub/Alita_Robot/alita/modules/chat_permissions_test.go` (NEW)
- `/Users/divkix/GitHub/Alita_Robot/alita/modules/helpers_test.go` (NEW)
**Pattern:** Internal tests (`package modules`), same as `moderation_input_test.go`.

**Public Interface:**
```go
// chat_permissions_test.go
func TestDefaultUnmutePermissions(t *testing.T)
func TestResolveUnmutePermissions(t *testing.T)

// helpers_test.go
func TestModuleEnabled_StoreAndLoad(t *testing.T)
func TestModuleEnabled_LoadModules(t *testing.T)
func TestListModules(t *testing.T)
```

**Dependencies:**
- `github.com/PaulSonOfLars/gotgbot/v2` -- for `ChatPermissions`, `ChatFullInfo`

**Test Cases (defaultUnmutePermissions):**

| Field | Expected |
|-------|----------|
| `CanSendMessages` | `true` |
| `CanSendPhotos` | `true` |
| `CanSendVideos` | `true` |
| `CanSendAudios` | `true` |
| `CanSendDocuments` | `true` |
| `CanSendVideoNotes` | `true` |
| `CanSendVoiceNotes` | `true` |
| `CanAddWebPagePreviews` | `true` |
| `CanChangeInfo` | `false` |
| `CanInviteUsers` | `true` |
| `CanPinMessages` | `false` |
| `CanManageTopics` | `false` |
| `CanSendPolls` | `true` |
| `CanSendOtherMessages` | `true` |

**Test Cases (resolveUnmutePermissions):**

| Input | Expected |
|-------|----------|
| `nil` | `defaultUnmutePermissions()` result |
| `&ChatFullInfo{Permissions: nil}` | `defaultUnmutePermissions()` result |
| `&ChatFullInfo{Permissions: &ChatPermissions{CanSendMessages: false}}` | the provided permissions (all false) |
| `&ChatFullInfo{Permissions: &ChatPermissions{CanSendMessages: true, CanPinMessages: true}}` | the provided permissions |

**Test Cases (moduleEnabled):**

| Action | Expected |
|--------|----------|
| `Init()` then `Store("admin", true)` then `Load("admin")` | `true` |
| `Init()` then `Load("nonexistent")` | `false` |
| `Store("a", true)`, `Store("b", true)`, `Store("c", false)` then `LoadModules()` | `["a", "b"]` (order not guaranteed, use Contains) |
| `Init()` with no stores, `LoadModules()` | empty slice |
| `Store("x", true)`, `Store("x", false)` | `Load("x")` returns `false` |
| `Store("", true)` | `Load("")` returns `true` (map allows empty key) |

**Estimated statements covered:** ~50

---

### Component: Extraction Function Tests (US-013)

**Responsibility:** Add edge case tests for extraction functions.
**Location:** `/Users/divkix/GitHub/Alita_Robot/alita/utils/extraction/extraction_test.go` (MODIFY -- append)
**Pattern:** Existing pattern in same file.

**New Test Functions:**
```go
func TestExtractTime(t *testing.T)
func TestIdFromReply_AdditionalEdgeCases(t *testing.T)
func TestExtractQuotes_AdditionalEdgeCases(t *testing.T)
```

**Test Cases (ExtractTime):**

| Input | Expected Duration | Expected Unit | Error? |
|-------|------------------|---------------|--------|
| `"2h"` | 2*time.Hour | "h" | no |
| `"30m"` | 30*time.Minute | "m" | no |
| `"0s"` | 0 | "s" | no |
| `"abc"` | 0 | "" | yes |
| `""` | 0 | "" | yes |
| `"1d"` | 24*time.Hour | "d" | no |

**Estimated statements covered:** ~50

---

### Component: Monitoring Pure Function Tests (US-014)

**Responsibility:** Test pure functions in monitoring package.
**Location:** `/Users/divkix/GitHub/Alita_Robot/alita/utils/monitoring/auto_remediation_test.go` (MODIFY -- append), `/Users/divkix/GitHub/Alita_Robot/alita/utils/monitoring/background_stats_test.go` (MODIFY -- append)
**Pattern:** Existing pattern in same files.

**New Test Functions:**
```go
// auto_remediation_test.go
func TestRemediationAction_NameAndSeverity(t *testing.T)
func TestAggressiveGCAction_CanExecute(t *testing.T)
func TestWarningAction_CanExecute(t *testing.T)

// background_stats_test.go
func TestCollectSystemStats(t *testing.T)
func TestRecordError(t *testing.T)
func TestGetStats(t *testing.T)
```

**Test Cases:**

| Test | Action | Expected |
|------|--------|----------|
| GCAction Name | `GCAction{}.Name()` | non-empty string |
| GCAction Severity | `GCAction{}.Severity()` | valid severity |
| AggressiveGC CanExecute low mem | metrics with low mem | `false` |
| AggressiveGC CanExecute high mem | metrics above 150% threshold | `true` |
| WarningAction CanExecute below | metrics below 80% | `false` |
| WarningAction CanExecute above | metrics above 80% | `true` |
| CollectSystemStats | call it | `NumGoroutines > 0`, `MemAllocMB >= 0` |
| RecordError | call then check counter | counter incremented |
| GetStats | after RecordMessage/RecordError | reflects recorded values |

**Estimated statements covered:** ~80

---

### Component: Migration SQL Processing Tests (US-015)

**Responsibility:** Expand existing migration tests with more edge cases.
**Location:** `/Users/divkix/GitHub/Alita_Robot/alita/db/migrations_test.go` (MODIFY -- append)
**Pattern:** Existing `newTestRunner()` helper.

**New Test Functions:**
```go
func TestCleanSupabaseSQL_AdditionalCases(t *testing.T)
func TestSplitSQLStatements_AdditionalCases(t *testing.T)
func TestGetMigrationFiles(t *testing.T)
```

**Test Cases (additional cleanSupabaseSQL):**

| Input | Expected |
|-------|----------|
| `CREATE INDEX idx_foo ON bar(col)` | `CREATE INDEX IF NOT EXISTS` |
| `CREATE UNIQUE INDEX idx_foo ON bar(col)` | `CREATE UNIQUE INDEX IF NOT EXISTS` |
| `CREATE TYPE mood AS ENUM ('happy','sad')` | wrapped in DO block |
| `ALTER TABLE foo ADD CONSTRAINT uk_bar UNIQUE(col)` | wrapped in idempotent DO block |
| Multiple GRANT lines mixed with DML | GRANTs removed, DML preserved |
| Already-idempotent `CREATE INDEX IF NOT EXISTS` | unchanged (no double `IF NOT EXISTS`) |

**Test Cases (additional splitSQLStatements):**

| Input | Expected Count |
|-------|---------------|
| Nested quotes `'it''s'` | correctly handles escaped quotes |
| Only comments `-- comment\n/* block */` | 0 |
| Mixed dollar-quoted and regular | correct count |

**Test Cases (getMigrationFiles):**

| Scenario | Expected |
|----------|----------|
| `t.TempDir()` with 0 SQL files | empty slice, no error |
| `t.TempDir()` with 3 SQL files | 3 entries sorted by name |
| Non-existent path | error returned |
| `t.TempDir()` with non-.sql files mixed in | only .sql files returned |

**Estimated statements covered:** ~100

---

### Component: Helpers Package Tests (US-016)

**Responsibility:** Test pure functions in helpers package.
**Location:** `/Users/divkix/GitHub/Alita_Robot/alita/utils/helpers/helpers_test.go` (MODIFY -- append)
**Pattern:** Existing pattern in same file.

**New Test Functions:**
```go
func TestSplitMessage_EdgeCases(t *testing.T)
func TestReverseHTML2MD(t *testing.T)
func TestShtml(t *testing.T)
func TestSmarkdown(t *testing.T)
func TestChunkKeyboardSlices(t *testing.T)
```

**Test Cases:**

| Test | Input | Expected |
|------|-------|----------|
| SplitMessage empty | `""` | single-element slice with `""` |
| SplitMessage exactly MaxMessageLength+1 no newlines | forced split | 2 parts |
| SplitMessage with unicode | multi-byte chars | correct rune-based splitting |
| ReverseHTML2MD bold | `"<b>text</b>"` | markdown bold |
| ReverseHTML2MD link | `"<a href=\"url\">text</a>"` | `"[text](url)"` |
| Shtml | call | `ParseMode="HTML"`, `IsDisabled=true`, `AllowSendingWithoutReply=true` |
| Smarkdown | call | `ParseMode="Markdown"` |
| ChunkKeyboardSlices empty | `[]` | empty result |
| ChunkKeyboardSlices 5 items chunk 3 | 5 buttons | 2 rows: [3, 2] |
| ChunkKeyboardSlices 6 items chunk 3 | 6 buttons | 2 rows: [3, 3] |

**Estimated statements covered:** ~100

---

### Component: Chat Status Helper Tests (US-017)

**Responsibility:** Test ID validation boundary cases.
**Location:** `/Users/divkix/GitHub/Alita_Robot/alita/utils/chat_status/chat_status_test.go` (MODIFY -- append)
**Pattern:** Existing pattern in same file.

**New Test Functions:**
```go
func TestIsValidUserId_Boundaries(t *testing.T)
func TestIsChannelId_Boundaries(t *testing.T)
```

**Test Cases:**

| Function | Input | Expected |
|----------|-------|----------|
| `IsValidUserId` | `math.MaxInt64` | `true` |
| `IsValidUserId` | `0` | `false` |
| `IsValidUserId` | `-1` | `false` |
| `IsValidUserId` | `1` | `true` |
| `IsChannelId` | `-1000000000000` | boundary -- test actual behavior |
| `IsChannelId` | `-1000000000001` | `true` |
| `IsChannelId` | `-999999999999` | `false` |
| `IsChannelId` | `0` | `false` |
| `IsChannelId` | `123` | `false` |

**Estimated statements covered:** ~20

---

## Data Models

No new data models. All tests operate on existing GORM models defined in `/Users/divkix/GitHub/Alita_Robot/alita/db/db.go`.

## Data Flow

### Flow: Pure Unit Test Execution

1. Test runner imports `package db` (or `package config`, etc.)
2. For `alita/db`: `init()` tries DB connection, may fail -> `TestMain` checks `DB == nil` -> exits 0 if nil
3. For `alita/config`: `init()` calls `LoadConfig()` which requires env vars -> CI sets them, local dev may not have them
4. Test function constructs input data (structs, byte slices, strings)
5. Test function calls target function
6. Test function asserts output matches expected

**Error paths:**
- Config `init()` fails locally without env vars -> test binary exits before any tests run. Not a problem in CI.
- DB `init()` fails without PostgreSQL -> `TestMain` exits 0, all DB tests skipped.

### Flow: DB Integration Test Execution

1. `TestMain` in `alita/db` checks `DB != nil`
2. `TestMain` runs `AutoMigrate` for all models
3. Each test calls `skipIfNoDb(t)`
4. Test generates unique ID via `time.Now().UnixNano()`
5. Test registers `t.Cleanup()` to delete created rows
6. Test performs CRUD operations
7. Test asserts results

**Error paths:**
- PostgreSQL unavailable -> all tests skipped gracefully
- `AutoMigrate` fails -> `TestMain` calls `os.Exit(1)`
- CRUD operation fails -> test fails with `t.Fatalf`

## API Contracts

No API changes. All new code is test code. No production code modifications.

## Testing Strategy

### Unit Tests

| Component | File | Test Count | Requires DB? | Requires Env Vars? |
|-----------|------|-----------|-------------|-------------------|
| GORM Types (Scan/Value) | `alita/db/gorm_types_test.go` | ~24 | No | No (but DB init runs) |
| TableNames | `alita/db/gorm_types_test.go` | 27 | No | No |
| BlacklistSettingsSlice | `alita/db/gorm_types_test.go` | ~9 | No | No |
| getSpanAttributes | `alita/db/gorm_types_test.go` | ~3 | No | No |
| NotesSettings accessor | `alita/db/gorm_types_test.go` | ~2 | No | No |
| ValidateConfig | `alita/config/config_test.go` | ~28 | No | Yes (CI) |
| setDefaults | `alita/config/config_test.go` | ~26 | No | Yes (CI) |
| getRedisAddress | `alita/config/config_test.go` | ~5 | No | Yes (CI) |
| getRedisPassword | `alita/config/config_test.go` | ~5 | No | Yes (CI) |
| Translator methods | `alita/i18n/i18n_test.go` | ~15 | No | No |
| Module permissions | `alita/modules/chat_permissions_test.go` | ~6 | No | Yes (CI) |
| Module helpers | `alita/modules/helpers_test.go` | ~8 | No | Yes (CI) |
| Extraction | `alita/utils/extraction/extraction_test.go` | ~6 | No | Yes (CI) |
| Monitoring | `alita/utils/monitoring/*_test.go` | ~9 | No | Yes (CI) |
| Migrations SQL | `alita/db/migrations_test.go` | ~12 | No | No |
| Helpers | `alita/utils/helpers/helpers_test.go` | ~10 | No | Yes (CI) |
| Chat Status | `alita/utils/chat_status/chat_status_test.go` | ~9 | No | Yes (CI) |

### Integration Tests

| Component | File | Test Count | Requires DB? |
|-----------|------|-----------|-------------|
| Optimized Queries | `alita/db/optimized_queries_test.go` | ~18 | Yes |
| Captcha CRUD | `alita/db/captcha_db_test.go` | ~6 | Yes |
| Greetings CRUD | `alita/db/greetings_db_test.go` | ~4 | Yes |
| Disable CRUD | `alita/db/disable_db_test.go` | ~2 | Yes |
| Notes CRUD | `alita/db/notes_db_test.go` | ~3 | Yes |
| Rules CRUD | `alita/db/rules_db_test.go` | ~2 | Yes |
| Admin CRUD | `alita/db/admin_db_test.go` | ~1 | Yes |
| Connections CRUD | `alita/db/connections_db_test.go` | ~2 | Yes |

### Verification Commands

```bash
# Run all tests (requires env vars for config-dependent packages)
make test

# Run only the new GORM type tests (no DB needed)
go test -v -race ./alita/db/... -run "TestButtonArray|TestStringArray|TestInt64Array|TestTableNames|TestBlacklistSettingsSlice|TestGetSpanAttributes|TestNotesSettings_Private"

# Run only config tests (needs BOT_TOKEN etc.)
go test -v -race ./alita/config/... -run "TestValidateConfig|TestSetDefaults|TestGetRedis"

# Run only i18n tests
go test -v -race ./alita/i18n/...

# Run only modules pure function tests
go test -v -race ./alita/modules/... -run "TestDefaultUnmutePermissions|TestResolveUnmutePermissions|TestModuleEnabled|TestListModules"

# Run only optimized query tests (needs PostgreSQL)
go test -v -race ./alita/db/... -run "TestOptimized|TestCacheKeyFormats|TestGetOptimizedQueries|TestNewCached"

# Run only migration tests (no DB needed for pure functions)
go test -v -race ./alita/db/... -run "TestCleanSupabaseSQL|TestSplitSQLStatements|TestGetMigrationFiles"

# Check coverage after all tests
go tool cover -func=coverage.out | grep '^total:'

# Verify test isolation (shuffle order)
go test -v -race -count=1 -shuffle=on -coverprofile=coverage.out -coverpkg=./... ./...

# Check for race conditions
go test -v -race -count=3 ./alita/db/... ./alita/config/... ./alita/i18n/... ./alita/modules/...
```

## Parallelization Analysis

### Independent Streams

- **Stream A (Pure Unit Tests -- No DB, No Config):**
  - `alita/db/gorm_types_test.go` (NEW) -- US-001, US-002, US-003, US-004
  - `alita/db/migrations_test.go` (MODIFY) -- US-015
  - `alita/i18n/i18n_test.go` (MODIFY) -- US-011
  - No shared files between these three.

- **Stream B (Config Tests -- Needs Env Vars):**
  - `alita/config/config_test.go` (NEW) -- US-005, US-006, US-007
  - Standalone file, no conflicts with other streams.

- **Stream C (Module Tests -- Needs Env Vars):**
  - `alita/modules/chat_permissions_test.go` (NEW) -- US-012
  - `alita/modules/helpers_test.go` (NEW) -- US-012
  - No shared files with other streams.

- **Stream D (Utils Tests -- Needs Env Vars):**
  - `alita/utils/extraction/extraction_test.go` (MODIFY) -- US-013
  - `alita/utils/monitoring/auto_remediation_test.go` (MODIFY) -- US-014
  - `alita/utils/monitoring/background_stats_test.go` (MODIFY) -- US-014
  - `alita/utils/helpers/helpers_test.go` (MODIFY) -- US-016
  - `alita/utils/chat_status/chat_status_test.go` (MODIFY) -- US-017
  - Each file is independent, can all be built in parallel.

- **Stream E (DB Integration Tests -- Needs PostgreSQL):**
  - `alita/db/optimized_queries_test.go` (NEW) -- US-008
  - `alita/db/captcha_db_test.go` (MODIFY) -- US-009
  - `alita/db/greetings_db_test.go` (MODIFY) -- US-010
  - `alita/db/disable_db_test.go` (MODIFY) -- US-018
  - `alita/db/notes_db_test.go` (MODIFY) -- US-018
  - `alita/db/rules_db_test.go` (MODIFY) -- US-018
  - `alita/db/admin_db_test.go` (MODIFY) -- US-018
  - `alita/db/connections_db_test.go` (MODIFY) -- US-018
  - All in same package (`alita/db`), share `testmain_test.go`. Can be worked on in parallel since they modify different files, but all compile together.

### Sequential Dependencies

- None between streams. All streams can execute simultaneously.
- Within Stream E, all files are in the same package but modify different files, so they can be coded in parallel as long as they all compile together.

### Shared Resources (Serialization Points)

- `alita/db/testmain_test.go` is shared by all DB test files but is NOT modified by any task.
- Each existing test file being modified (MODIFY) is only touched by ONE user story, so no serialization needed.

## Files Summary

### New Files (4)
| File | User Stories | Stream |
|------|-------------|--------|
| `/Users/divkix/GitHub/Alita_Robot/alita/db/gorm_types_test.go` | US-001, US-002, US-003, US-004 | A |
| `/Users/divkix/GitHub/Alita_Robot/alita/config/config_test.go` | US-005, US-006, US-007 | B |
| `/Users/divkix/GitHub/Alita_Robot/alita/modules/chat_permissions_test.go` | US-012 | C |
| `/Users/divkix/GitHub/Alita_Robot/alita/modules/helpers_test.go` | US-012 | C |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/optimized_queries_test.go` | US-008 | E |

### Modified Files (11)
| File | User Stories | Stream |
|------|-------------|--------|
| `/Users/divkix/GitHub/Alita_Robot/alita/i18n/i18n_test.go` | US-011 | A |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/migrations_test.go` | US-015 | A |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/captcha_db_test.go` | US-009 | E |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/greetings_db_test.go` | US-010 | E |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/disable_db_test.go` | US-018 | E |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/notes_db_test.go` | US-018 | E |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/rules_db_test.go` | US-018 | E |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/admin_db_test.go` | US-018 | E |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/connections_db_test.go` | US-018 | E |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/extraction/extraction_test.go` | US-013 | D |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/monitoring/auto_remediation_test.go` | US-014 | D |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/monitoring/background_stats_test.go` | US-014 | D |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/helpers/helpers_test.go` | US-016 | D |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/chat_status/chat_status_test.go` | US-017 | D |

### Unchanged Files (read-only context)
| File | Why Referenced |
|------|---------------|
| `/Users/divkix/GitHub/Alita_Robot/alita/db/db.go` | Source under test for US-001-004 |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/testmain_test.go` | Shared DB test infrastructure |
| `/Users/divkix/GitHub/Alita_Robot/alita/config/config.go` | Source under test for US-005-007 |
| `/Users/divkix/GitHub/Alita_Robot/alita/i18n/translator.go` | Source under test for US-011 |
| `/Users/divkix/GitHub/Alita_Robot/alita/i18n/manager.go` | Source under test for US-011 |
| `/Users/divkix/GitHub/Alita_Robot/alita/modules/chat_permissions.go` | Source under test for US-012 |
| `/Users/divkix/GitHub/Alita_Robot/alita/modules/helpers.go` | Source under test for US-012 |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/optimized_queries.go` | Source under test for US-008 |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/captcha_db.go` | Source under test for US-009 |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/greetings_db.go` | Source under test for US-010 |
| `/Users/divkix/GitHub/Alita_Robot/alita/db/migrations.go` | Source under test for US-015 |
| `/Users/divkix/GitHub/Alita_Robot/Makefile` | Test command reference |
| `/Users/divkix/GitHub/Alita_Robot/.github/workflows/ci.yml` | CI env vars and threshold |

## Design Decisions

### Decision: No production code changes
- **Context:** The temptation exists to refactor `config.init()` to remove `log.Fatalf` for better testability.
- **Options considered:** (A) Refactor init, (B) Accept CI-only testing.
- **Chosen:** (B) because the requirements explicitly state refactoring `config.init()` is out of scope (separate PR, architectural change). CI has the env vars. Local devs who want to run tests set them.
- **Trade-offs:** Local test runs require env vars. Acceptable.

### Decision: Single file for all GORM type tests
- **Context:** Could put each type's tests in separate files or one combined file.
- **Options considered:** (A) One file per type, (B) One combined file.
- **Chosen:** (B) `gorm_types_test.go` because all types follow identical patterns (Scan/Value), and the codebase groups all types in a single `db.go`. Keeps it scannable.
- **Trade-offs:** File may be longer (~300 lines). Acceptable for a test file.

### Decision: Internal tests for modules package
- **Context:** Module unexported functions need testing. Could use `package modules_test` (external) or `package modules` (internal).
- **Options considered:** (A) External tests requiring exported wrappers, (B) Internal tests.
- **Chosen:** (B) Internal tests (`package modules`) to directly test unexported functions without creating exported wrappers. This follows the existing pattern in `moderation_input_test.go`, `callback_codec_test.go`, `rules_format_test.go`.
- **Trade-offs:** Tests have access to all internals, which could lead to brittle tests. Mitigated by only testing stable, well-defined helper functions.

### Decision: No test helpers or mocking frameworks
- **Context:** Could introduce testify, gomock, or custom helpers.
- **Options considered:** (A) Add testify for assertions, (B) Use stdlib only.
- **Chosen:** (B) because the entire codebase uses stdlib `testing` with `t.Fatalf`. Introducing a dependency for tests alone violates the consistency rule.
- **Trade-offs:** Slightly more verbose assertion code. Consistent with codebase.

### Decision: Use t.Setenv for Redis tests, not t.Parallel
- **Context:** `getRedisAddress()` and `getRedisPassword()` read env vars. `t.Setenv()` is incompatible with `t.Parallel()` in Go.
- **Options considered:** (A) Use t.Setenv without parallel, (B) Use os.Setenv with manual cleanup.
- **Chosen:** (A) because `t.Setenv()` automatically restores env vars and is the idiomatic Go approach. The test count is small (~10) so sequential execution is fast.
- **Trade-offs:** These specific subtests run sequentially. Negligible performance impact.

## Coverage Estimation

| Component | Est. New Statements Covered |
|-----------|---------------------------|
| GORM Types (Scan/Value/TableName) | ~120 |
| Config (ValidateConfig + setDefaults) | ~150 |
| Config (Redis helpers) | ~30 |
| Optimized Queries (integration) | ~200 |
| Captcha (integration expansion) | ~100 |
| Greetings (integration expansion) | ~60 |
| i18n (translator/manager) | ~200 |
| Module helpers (permissions, moduleEnabled) | ~50 |
| Extraction (edge cases) | ~50 |
| Monitoring (stats, remediation) | ~80 |
| Migrations (SQL processing) | ~100 |
| Helpers (SplitMessage, HTML, chunks) | ~100 |
| Chat Status (boundaries) | ~20 |
| Remaining DB CRUD (disable, notes, rules, admin, connections) | ~150 |
| **Total new statements** | **~1,410** |

**Current coverage:** ~1,410 statements out of ~11,288 (12.5%)
**After this work:** ~1,410 + ~1,410 = ~2,820 direct + cross-package gains

**Cross-package coverage boost:** The `-coverpkg=./...` flag means every DB integration test that calls functions in `alita/config`, `alita/i18n`, `alita/utils/tracing`, etc. counts toward those packages' coverage. DB integration tests exercising `GetRecord`, `CreateRecord`, `GetRecordWithContext` etc. will significantly boost `alita/db/db.go` coverage beyond the direct test count. Similarly, config `init()` running during test setup covers `LoadConfig()`, `setDefaults()`, and `ValidateConfig()` for every package that imports config.

**Estimated total with cross-package:** ~4,500-5,200 statements = **40-46%**

The 45% target is achievable with this plan. The primary risk is if the total statement count is higher than estimated (>12,000), which would require additional test cases.

## Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Total statement count higher than ~11,288 | MEDIUM | Coverage % lower than estimated, could miss 45% | Add more test cases in highest-impact areas (modules helpers, DB operations) |
| Config `init()` crash in local dev without env vars | HIGH (local) / LOW (CI) | Developers cannot run tests locally | Document required env vars; tests pass in CI which is what matters for the threshold |
| PostgreSQL unavailable in CI | LOW | DB integration tests skipped, coverage drops ~10-12% | CI has PostgreSQL 16 service configured; if it fails, CI job fails before coverage check |
| Redis not in CI | N/A (known) | Cache-dependent code paths not tested | Tests handle `cache.Marshal == nil` gracefully; this is by design |
| DB test isolation failure (ID collisions) | LOW | Flaky tests | Strict use of `time.Now().UnixNano()` for unique IDs and `t.Cleanup()` for teardown |
| Test execution time exceeds 10-minute timeout | LOW | CI job timeout | New tests are fast (pure unit tests <1s, DB tests <30s each); total should remain under 5 minutes |
| Existing tests break | LOW | Reduces baseline coverage | Run `make test` before adding new tests to confirm green baseline |

DESIGN_COMPLETE
