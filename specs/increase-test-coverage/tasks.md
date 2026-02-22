# Implementation Tasks: Increase Test Coverage to 45%+

**Date:** 2026-02-21
**Design Source:** design.md
**Total Tasks:** 17
**Slicing Strategy:** vertical (each task = complete feature slice)

---

## TASK-001: GORM Custom Type Scan/Value Unit Tests

**Complexity:** M
**Files:**
- CREATE: `alita/db/gorm_types_test.go`
**Dependencies:** None
**Description:**

Create `alita/db/gorm_types_test.go` in `package db` containing table-driven unit tests for all six Scan/Value methods on `ButtonArray`, `StringArray`, and `Int64Array`. These are pure functions that do NOT require a database connection.

**Test functions to implement:**

1. `TestButtonArray_Scan(t *testing.T)` -- table-driven with these cases:
   - `nil` input -> result is `ButtonArray{}`, no error
   - `[]byte("[{\"name\":\"btn\",\"url\":\"http://x\"}]")` -> 1-element array, no error
   - `"string"` (not `[]byte`) -> error containing "type assertion"
   - `[]byte("{invalid")` -> JSON unmarshal error
   - `[]byte("null")` -> `ButtonArray(nil)` or empty, no error
   - `[]byte("")` -> JSON unmarshal error
   - `[]byte` with special chars in Name/Url fields (e.g., `"name":"a&b<c>","url":"http://x?a=1&b=2"`) -> correctly deserialized, no error

2. `TestButtonArray_Value(t *testing.T)` -- table-driven:
   - `ButtonArray{}` (empty) -> result is `"[]"`, no error
   - `ButtonArray{{Name:"a",Url:"b"}}` -> valid JSON bytes, no error. Assert via `json.Valid` and re-unmarshal
   - `ButtonArray{{Name:"",Url:""}}` -> valid JSON with empty strings, no error

3. `TestStringArray_Scan(t *testing.T)` -- same pattern as ButtonArray_Scan:
   - `nil` -> `StringArray{}`, no error
   - `[]byte("[\"hello\",\"world\"]")` -> 2-element array, no error
   - `"string"` -> error "type assertion"
   - `[]byte("{invalid")` -> unmarshal error
   - `[]byte` with unicode: `[]byte("[\"cafe\\u0301\"]")` -> correctly deserialized

4. `TestStringArray_Value(t *testing.T)`:
   - `StringArray{}` -> `"[]"`, no error
   - `StringArray{"a","b"}` -> valid JSON, no error

5. `TestInt64Array_Scan(t *testing.T)`:
   - `nil` -> `Int64Array{}`, no error
   - `[]byte("[1,2,3]")` -> `Int64Array{1,2,3}`, no error
   - `"string"` -> error
   - `[]byte("[9223372036854775807]")` -> `Int64Array{math.MaxInt64}`, no error
   - `[]byte("[-9223372036854775808]")` -> `Int64Array{math.MinInt64}`, no error
   - `[]byte("[0,-1,1]")` -> `Int64Array{0,-1,1}`, no error

6. `TestInt64Array_Value(t *testing.T)`:
   - `Int64Array{}` -> `"[]"`, no error
   - `Int64Array{math.MaxInt64}` -> valid JSON, no error

**Pattern:** Use `t.Parallel()` at top level and in each subtest. Use `t.Run(tc.name, ...)`. Import `encoding/json` for validation, `math` for int64 boundaries. Follow existing style from `alita/config/types_test.go`.

**Context to Read:**
- design.md, section "Component: GORM Custom Type Unit Tests"
- `alita/db/db.go` lines 49-131 -- the Scan/Value method implementations

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && go test -v -race -count=1 ./alita/db/... -run "TestButtonArray|TestStringArray|TestInt64Array"
```

---

## TASK-002: TableName Method Unit Tests

**Complexity:** S
**Files:**
- MODIFY: `alita/db/gorm_types_test.go` -- append TestTableNames function
**Dependencies:** TASK-001
**Description:**

Append to `alita/db/gorm_types_test.go` a single table-driven test function `TestTableNames` that verifies all 27 `TableName()` methods return the correct table name string. No database needed.

**Implementation:**

```go
func TestTableNames(t *testing.T) {
    t.Parallel()

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

    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            t.Parallel()
            if got := tc.model.TableName(); got != tc.wantTable {
                t.Fatalf("%s.TableName() = %q, want %q", tc.name, got, tc.wantTable)
            }
        })
    }
}
```

Note: `SchemaMigration.TableName()` is already tested in `migrations_test.go`, but including it here for completeness is harmless.

**Context to Read:**
- `alita/db/db.go` -- all TableName() method definitions
- `alita/db/gorm_types_test.go` -- file created by TASK-001

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && go test -v -race -count=1 ./alita/db/... -run "TestTableNames"
```

---

## TASK-003: BlacklistSettingsSlice, getSpanAttributes, and NotesSettings Tests

**Complexity:** S
**Files:**
- MODIFY: `alita/db/gorm_types_test.go` -- append BlacklistSettingsSlice, getSpanAttributes, PrivateNotesEnabled tests
**Dependencies:** TASK-001
**Description:**

Append to `alita/db/gorm_types_test.go` these test functions:

1. `TestBlacklistSettingsSlice_Triggers(t *testing.T)`:
   - Empty slice -> `nil` (or `[]string(nil)`)
   - 3 entries with Words "a","b","c" -> `[]string{"a","b","c"}`

2. `TestBlacklistSettingsSlice_Action(t *testing.T)`:
   - Empty slice -> `"warn"` (default)
   - Slice with first `Action="ban"` -> `"ban"`
   - Slice with first `Action=""` -> `""` (not default -- len > 0 triggers first element return)

3. `TestBlacklistSettingsSlice_Reason(t *testing.T)`:
   - Empty slice -> `"Blacklisted word: '%s'"` (default)
   - Slice with first `Reason="spam"` -> `"spam"`
   - Slice with first `Reason=""` -> `"Blacklisted word: '%s'"` (empty reason triggers default)

4. `TestGetSpanAttributes(t *testing.T)`:
   - `nil` input -> empty `[]attribute.KeyValue{}`
   - `&User{}` -> 1 attribute with key `"db.model"`, value containing `"*db.User"`
   - `"hello"` (string) -> 1 attribute with key `"db.model"`, value `"string"`
   Import `go.opentelemetry.io/otel/attribute` for assertions.

5. `TestNotesSettings_PrivateNotesEnabled(t *testing.T)`:
   - `NotesSettings{Private: true}` -> `true`
   - `NotesSettings{Private: false}` -> `false`

All tests use `t.Parallel()` at top and subtest levels.

**Context to Read:**
- `alita/db/db.go` lines 299-328 -- BlacklistSettingsSlice methods
- `alita/db/db.go` lines 744-749 -- getSpanAttributes
- `alita/db/db.go` lines 546-548 -- PrivateNotesEnabled

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && go test -v -race -count=1 ./alita/db/... -run "TestBlacklistSettingsSlice|TestGetSpanAttributes|TestNotesSettings_Private"
```

---

## TASK-004: Config ValidateConfig Unit Tests

**Complexity:** M
**Files:**
- CREATE: `alita/config/config_test.go`
**Dependencies:** None
**Description:**

Create `alita/config/config_test.go` in `package config` with `TestValidateConfig`. This file is in the `config` package which has an `init()` that calls `log.Fatalf` without env vars. CI sets `BOT_TOKEN=test-token` etc. Add a skip guard at the top of each test:

```go
func skipIfNoConfig(t *testing.T) {
    t.Helper()
    if os.Getenv("BOT_TOKEN") == "" {
        t.Skip("skipping: BOT_TOKEN not set (config.init() would fatalf)")
    }
}
```

**Helper function:**
```go
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

**Test cases (table-driven in `TestValidateConfig`):**

| # | Override | Expected Error Substring |
|---|---------|-------------------------|
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
| 23 | All workers at min (1), valid config | `nil` |
| 24 | `UseWebhooks=false, WebhookDomain=""` | `nil` |
| 25 | `DispatcherMaxRoutines=0` | `nil` (0 means default) |
| 26 | `DBMaxIdleConns=0` | `nil` (0 means default) |
| 27 | `MaxConcurrentOperations=-1` | `"MAX_CONCURRENT_OPERATIONS"` |

For each case where error is expected, assert `err != nil` and `strings.Contains(err.Error(), expectedSubstring)`. For nil cases, assert `err == nil`.

Each subtest modifies a copy of the config (not the original). Use `t.Parallel()` at top level and subtest level.

**Context to Read:**
- design.md, section "Component: Config ValidateConfig and setDefaults Tests"
- `alita/config/config.go` lines 159-239 -- ValidateConfig implementation

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && go test -v -race -count=1 ./alita/config/... -run "TestValidateConfig"
```

---

## TASK-005: Config setDefaults Unit Tests

**Complexity:** M
**Files:**
- MODIFY: `alita/config/config_test.go` -- append TestSetDefaults
**Dependencies:** TASK-004
**Description:**

Append `TestSetDefaults` to `alita/config/config_test.go`. This tests that `Config.setDefaults()` correctly populates zero-value fields with expected defaults and preserves non-zero fields.

**Test cases (table-driven subtests):**

Group 1 - "zero config gets defaults":
Create a `Config{}` (zero value), call `cfg.setDefaults()`, then assert:
- `cfg.ApiServer == "https://api.telegram.org"`
- `cfg.WorkingMode == "worker"`
- `cfg.RedisAddress == "localhost:6379"`
- `cfg.RedisDB == 1`
- `cfg.HTTPPort == 8080`
- `cfg.ChatValidationWorkers == 10`
- `cfg.DatabaseWorkers == 5`
- `cfg.BulkOperationWorkers == 4`
- `cfg.CacheWorkers == 3`
- `cfg.StatsCollectionWorkers == 2`
- `cfg.DBMaxIdleConns == 50`
- `cfg.DBMaxOpenConns == 200`
- `cfg.DBConnMaxLifetimeMin == 240`
- `cfg.DBConnMaxIdleTimeMin == 60`
- `cfg.MigrationsPath == "migrations"`
- `cfg.ClearCacheOnStartup == true`
- `cfg.MaxConcurrentOperations == 50`
- `cfg.OperationTimeoutSeconds == 30`
- `cfg.DispatcherMaxRoutines == 200`
- `cfg.ResourceMaxGoroutines == 1000`
- `cfg.ResourceMaxMemoryMB == 500`
- `cfg.ResourceGCThresholdMB == 400`

Group 2 - "pre-set values preserved":
- Set `ApiServer="custom"`, call `setDefaults()` -> `ApiServer == "custom"`
- Set `RedisDB=5`, call `setDefaults()` -> `RedisDB == 5`
- Set `HTTPPort=3000`, call `setDefaults()` -> `HTTPPort == 3000`

Group 3 - "backward compat WebhookPort":
- Set `WebhookPort=9090, HTTPPort=0`, call `setDefaults()` -> `HTTPPort == 9090`

Group 4 - "ClearCacheOnStartup unconditional":
- Set `ClearCacheOnStartup=false`, call `setDefaults()` -> `ClearCacheOnStartup == true`

Group 5 - "Debug=false enables monitoring":
- `Debug=false` zero config -> `EnablePerformanceMonitoring == true`, `EnableBackgroundStats == true`

Use `skipIfNoConfig(t)` from TASK-004. Use `t.Parallel()`.

**Context to Read:**
- `alita/config/config.go` lines 372-531 -- setDefaults implementation
- `alita/config/config_test.go` -- file created in TASK-004

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && go test -v -race -count=1 ./alita/config/... -run "TestSetDefaults"
```

---

## TASK-006: Config getRedisAddress and getRedisPassword Tests

**Complexity:** S
**Files:**
- MODIFY: `alita/config/config_test.go` -- append TestGetRedisAddress and TestGetRedisPassword
**Dependencies:** TASK-004
**Description:**

Append `TestGetRedisAddress` and `TestGetRedisPassword` to `alita/config/config_test.go`. These test environment-variable-based Redis configuration parsing.

**Critical: These tests use `t.Setenv()` which is incompatible with `t.Parallel()`.** The top-level test MUST NOT call `t.Parallel()`. Each subtest uses `t.Setenv()` to set env vars (auto-restored after subtest).

**TestGetRedisAddress subtests:**

| # | Env Setup | Expected Result |
|---|-----------|----------------|
| 1 | `REDIS_ADDRESS=myhost:1234` | `"myhost:1234"` |
| 2 | `REDIS_ADDRESS=""`, `REDIS_URL=redis://user:pass@host:6380` | `"host:6380"` |
| 3 | Both empty | `""` |
| 4 | `REDIS_ADDRESS=x`, `REDIS_URL=redis://host:9999` | `"x"` (REDIS_ADDRESS takes priority) |
| 5 | `REDIS_URL=not-a-valid-url-%%%` | `""` (parse error) |

**TestGetRedisPassword subtests:**

| # | Env Setup | Expected Result |
|---|-----------|----------------|
| 1 | `REDIS_PASSWORD=secret` | `"secret"` |
| 2 | `REDIS_PASSWORD=""`, `REDIS_URL=redis://user:pass123@host:6380` | `"pass123"` |
| 3 | Both empty | `""` |
| 4 | `REDIS_URL=redis://host:6380` (no userinfo) | `""` |
| 5 | `REDIS_URL=redis://user@host:6380` (user, no pass) | `""` |

Each subtest within the top-level test calls `t.Setenv()` for the relevant env vars. The `REDIS_ADDRESS` and `REDIS_URL` vars must be cleared (set to `""`) at the start of each subtest to avoid leaking between subtests.

Use `skipIfNoConfig(t)` guard.

**Context to Read:**
- `alita/config/config.go` lines 18-61 -- getRedisAddress and getRedisPassword implementations
- `alita/config/config_test.go` -- file modified in TASK-004/005

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && go test -v -race -count=1 ./alita/config/... -run "TestGetRedis"
```

---

## TASK-007: i18n Translator and Manager Expanded Tests

**Complexity:** M
**Files:**
- MODIFY: `alita/i18n/i18n_test.go` -- append new test functions
**Dependencies:** None
**Description:**

Append new test functions to `alita/i18n/i18n_test.go` using the existing `newTestTranslator` helper. The i18n package does NOT import `alita/config`, so these tests run without env vars.

**New test functions to add:**

1. `TestTranslator_GetString_NilManager(t *testing.T)`:
   - Create `tr := &Translator{manager: nil, langCode: "en"}`
   - Call `tr.GetString("any_key")`
   - Assert error wraps `ErrManagerNotInit` via `errors.Is(err, ErrManagerNotInit)`

2. `TestTranslator_GetString_FallbackToDefault(t *testing.T)`:
   - Create a manager with "en" data: `greeting: "Hello"` and "es" data: `other_key: "Hola"`
   - Create an "es" translator that has manager with both locales
   - Call `esTranslator.GetString("greeting")` -- should fall back to "en" and return "Hello"
   - Implementation: build a LocaleManager with both `viperCache["en"]` and `viperCache["es"]`, create a Translator with `langCode: "es"`, and verify fallback

3. `TestTranslator_GetString_NamedParams(t *testing.T)`:
   - YAML: `greet: "Hello, {user}!"`
   - Call `tr.GetString("greet", TranslationParams{"user": "Alice"})`
   - Assert result contains "Alice"

4. `TestTranslator_GetString_UnusedParams(t *testing.T)`:
   - YAML: `static: "No placeholders here"`
   - Call with `TranslationParams{"extra": "val"}`
   - Assert result is `"No placeholders here"` unchanged

5. `TestTranslator_GetString_EmptyKey(t *testing.T)`:
   - Call `tr.GetString("")`
   - Assert error wraps `ErrKeyNotFound`

6. `TestTranslator_GetPlural_NilManager(t *testing.T)`:
   - Create `tr := &Translator{manager: nil}`
   - Call `tr.GetPlural("items", 1)`
   - Assert `ErrManagerNotInit`

7. `TestTranslator_GetStringSlice_NilManager(t *testing.T)`:
   - Create `tr := &Translator{manager: nil}`
   - Call `tr.GetStringSlice("items")`
   - Assert `ErrManagerNotInit`

8. `TestLocaleManager_IsLanguageSupported(t *testing.T)`:
   - Build LocaleManager with localeData for "en", "es"
   - `IsLanguageSupported("en")` -> true
   - `IsLanguageSupported("zz")` -> false

9. `TestLocaleManager_GetDefaultLanguage(t *testing.T)`:
   - Build LocaleManager with `defaultLang: "en"`
   - `GetDefaultLanguage()` -> `"en"`

10. `TestLocaleManager_GetStats(t *testing.T)`:
    - Build LocaleManager with 4 locales
    - Call `GetStats()`, assert it has `total_languages: 4` (or check the map key)

All tests use `t.Parallel()`. Use `newTestTranslator` where applicable, construct `LocaleManager` directly for manager method tests.

**Context to Read:**
- design.md, section "Component: i18n Translator and Manager Tests"
- `alita/i18n/i18n_test.go` -- existing tests and `newTestTranslator` helper (lines 372-389)
- `alita/i18n/translator.go` -- GetString, GetPlural, interpolateParams implementations
- `alita/i18n/manager.go` -- IsLanguageSupported, GetDefaultLanguage, GetStats implementations

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && go test -v -race -count=1 ./alita/i18n/... -run "TestTranslator_GetString_NilManager|TestTranslator_GetString_Fallback|TestTranslator_GetString_Named|TestTranslator_GetString_Unused|TestTranslator_GetString_Empty|TestTranslator_GetPlural_NilManager|TestTranslator_GetStringSlice|TestLocaleManager_Is|TestLocaleManager_GetDefault|TestLocaleManager_GetStats"
```

---

## TASK-008: Module chat_permissions Tests

**Complexity:** S
**Files:**
- CREATE: `alita/modules/chat_permissions_test.go`
**Dependencies:** None
**Description:**

Create `alita/modules/chat_permissions_test.go` in `package modules` (internal test -- accesses unexported functions). This file tests `defaultUnmutePermissions()` and `resolveUnmutePermissions()`.

**Test functions:**

1. `TestDefaultUnmutePermissions(t *testing.T)`:
   - Call `defaultUnmutePermissions()`, assert each field:
     - `CanSendMessages == true`
     - `CanSendPhotos == true`
     - `CanSendVideos == true`
     - `CanSendAudios == true`
     - `CanSendDocuments == true`
     - `CanSendVideoNotes == true`
     - `CanSendVoiceNotes == true`
     - `CanAddWebPagePreviews == true`
     - `CanChangeInfo == false`
     - `CanInviteUsers == true`
     - `CanPinMessages == false`
     - `CanManageTopics == false`
     - `CanSendPolls == true`
     - `CanSendOtherMessages == true`

2. `TestResolveUnmutePermissions(t *testing.T)` -- table-driven:
   - `nil` input -> returns same as `defaultUnmutePermissions()`
   - `&gotgbot.ChatFullInfo{Permissions: nil}` -> returns same as defaults
   - `&gotgbot.ChatFullInfo{Permissions: &gotgbot.ChatPermissions{CanSendMessages: false}}` -> returns the provided permissions (all false)
   - `&gotgbot.ChatFullInfo{Permissions: &gotgbot.ChatPermissions{CanSendMessages: true, CanPinMessages: true}}` -> returns the provided permissions exactly

Import `github.com/PaulSonOfLars/gotgbot/v2`. Use `t.Parallel()`.

**Context to Read:**
- `alita/modules/chat_permissions.go` -- the 32-line source file with both functions

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && go test -v -race -count=1 ./alita/modules/... -run "TestDefaultUnmutePermissions|TestResolveUnmutePermissions"
```

---

## TASK-009: Module helpers (moduleEnabled) Tests

**Complexity:** S
**Files:**
- CREATE: `alita/modules/helpers_test.go`
**Dependencies:** None
**Description:**

Create `alita/modules/helpers_test.go` in `package modules` (internal test). Tests for the `moduleEnabled` struct methods and the `listModules()` function.

**Test functions:**

1. `TestModuleEnabled_StoreAndLoad(t *testing.T)`:
   - Create `var me moduleEnabled; me.Init()`
   - `me.Store("admin", true)` -> `me.Load("admin")` returns `("admin", true)`
   - `me.Load("nonexistent")` returns `("nonexistent", false)`
   - `me.Store("admin", false)` -> `me.Load("admin")` returns `("admin", false)` (overwrite)
   - `me.Store("", true)` -> `me.Load("")` returns `("", true)` (empty key allowed)

2. `TestModuleEnabled_LoadModules(t *testing.T)`:
   - Create `var me moduleEnabled; me.Init()`
   - No stores -> `me.LoadModules()` returns empty slice `len == 0`
   - Store "a" true, "b" true, "c" false -> `me.LoadModules()` returns 2-element slice containing "a" and "b" (order not guaranteed, use `slices.Contains` to check)

3. `TestListModules(t *testing.T)`:
   - This function calls `HelpModule.AbleMap.LoadModules()` and sorts. Since `HelpModule` is a package-level var, initialize it for testing:
     ```go
     HelpModule.AbleMap.Init()
     HelpModule.AbleMap.Store("admin", true)
     HelpModule.AbleMap.Store("filters", true)
     HelpModule.AbleMap.Store("help", true)
     ```
   - Call `listModules()`, assert result is sorted: `[]string{"admin", "filters", "help"}`
   - WARNING: This modifies package-level state. Do NOT use `t.Parallel()` for this test. Clean up in `t.Cleanup()` by re-initializing `HelpModule.AbleMap.Init()`.

Use `t.Parallel()` for tests 1 and 2 (they use local variables). Test 3 must be sequential.

**Context to Read:**
- `alita/modules/help.go` lines 156-186 -- moduleEnabled type definition and methods
- `alita/modules/helpers.go` lines 96-100 -- listModules function

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && go test -v -race -count=1 ./alita/modules/... -run "TestModuleEnabled|TestListModules"
```

---

## TASK-010: Migration SQL Processing Expanded Tests

**Complexity:** M
**Files:**
- MODIFY: `alita/db/migrations_test.go` -- append new test functions
**Dependencies:** None
**Description:**

Append to `alita/db/migrations_test.go` additional test cases for `cleanSupabaseSQL`, `splitSQLStatements`, and a new `TestGetMigrationFiles` function. All are pure function tests that do NOT need a database.

**New test functions:**

1. `TestCleanSupabaseSQL_AdditionalCases(t *testing.T)` -- table-driven, using `newTestRunner()`:
   - `CREATE INDEX idx_foo ON bar(col)` -> output contains `CREATE INDEX IF NOT EXISTS`
   - `CREATE UNIQUE INDEX idx_foo ON bar(col)` -> output contains `CREATE UNIQUE INDEX IF NOT EXISTS`
   - `CREATE INDEX IF NOT EXISTS idx_foo ON bar(col)` -> unchanged (no double `IF NOT EXISTS`)
   - `CREATE TYPE mood AS ENUM ('happy','sad')` -> output contains `DO $$` block wrapper
   - `ALTER TABLE foo ADD CONSTRAINT uk_bar UNIQUE(col)` -> output contains `DO $$` block wrapper
   - Multiple GRANT lines mixed with a CREATE TABLE -> GRANTs removed, CREATE TABLE preserved
   - Empty string -> empty output

2. `TestSplitSQLStatements_AdditionalCases(t *testing.T)` -- table-driven, using `newTestRunner()`:
   - Nested quotes `SELECT 'it''s'` -> 1 statement (escaped quote not a split point)
   - Only comments `-- comment\n/* block */` -> 0 statements
   - Empty string -> 0 statements
   - Mixed dollar-quoted and regular: `SELECT 1; DO $$ BEGIN NULL; END $$; SELECT 2;` -> 3 statements

3. `TestGetMigrationFiles(t *testing.T)`:
   - Use `t.TempDir()` to create temporary directories
   - Empty directory -> empty slice, no error
   - Directory with 3 `.sql` files (create them via `os.WriteFile`) -> 3 entries sorted by name
   - Non-existent path -> error returned
   - Directory with mixed `.sql` and `.txt` files -> only `.sql` files returned
   - NOTE: `getMigrationFiles` is unexported, so this is an internal package test (already `package db`)
   - Create a `MigrationRunner` with the temp dir as `migrationsPath`, call the relevant method

**Context to Read:**
- `alita/db/migrations_test.go` -- existing test structure and `newTestRunner()` helper
- `alita/db/migrations.go` -- cleanSupabaseSQL, splitSQLStatements, getMigrationFiles implementations

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && go test -v -race -count=1 ./alita/db/... -run "TestCleanSupabaseSQL_Additional|TestSplitSQLStatements_Additional|TestGetMigrationFiles"
```

---

## TASK-011: DB Integration Tests for Optimized Queries

**Complexity:** L
**Files:**
- CREATE: `alita/db/optimized_queries_test.go`
**Dependencies:** None
**Description:**

Create `alita/db/optimized_queries_test.go` in `package db` with integration tests for the optimized query types. All tests call `skipIfNoDb(t)`, use unique IDs via `time.Now().UnixNano()`, and clean up via `t.Cleanup()`.

**Test functions:**

1. `TestOptimizedLockQueries_NilDB(t *testing.T)`:
   - Construct `q := &OptimizedLockQueries{db: nil}`
   - `q.GetLockStatus(1, "sticker")` -> `false, error containing "not initialized"`
   - `q.GetChatLocksOptimized(1)` -> `nil, error containing "not initialized"`

2. `TestOptimizedLockQueries_GetLockStatus(t *testing.T)`:
   - `skipIfNoDb(t)`
   - `chatID := time.Now().UnixNano()`
   - Create `OptimizedLockQueries{db: DB}`
   - No lock record -> `GetLockStatus(chatID, "sticker")` returns `false, nil`
   - Insert `LockSettings{ChatId: chatID, LockType: "sticker", Locked: true}` via `DB.Create`
   - `GetLockStatus(chatID, "sticker")` returns `true, nil`
   - Cleanup: delete lock record

3. `TestOptimizedLockQueries_GetChatLocksOptimized(t *testing.T)`:
   - `skipIfNoDb(t)`
   - Insert 3 lock records with different LockTypes
   - `GetChatLocksOptimized(chatID)` returns map with 3 entries
   - No locks for different chatID -> empty map

4. `TestOptimizedUserQueries_NilDB(t *testing.T)`:
   - Construct with nil db
   - `GetUserBasicInfo(1)` -> error containing "not initialized"

5. `TestOptimizedUserQueries_GetUserBasicInfo(t *testing.T)`:
   - `skipIfNoDb(t)`
   - Insert a user record
   - `GetUserBasicInfo(userID)` -> returns user with correct fields
   - Non-existent userID -> returns `gorm.ErrRecordNotFound`

6. `TestOptimizedChatQueries_NilDB(t *testing.T)`:
   - Similar nil DB test

7. `TestGetOptimizedQueries_Singleton(t *testing.T)`:
   - `skipIfNoDb(t)`
   - Call `GetOptimizedQueries()` twice, assert both return the same pointer (singleton pattern)
   - NOTE: The singleton uses `sync.Once`. Since the package-level singleton may already be initialized, reset it if possible, or just verify non-nil return.

8. `TestCacheKeyFormats(t *testing.T)` (if cache key functions are exported or accessible):
   - Test that cache key generation produces expected formats
   - If the functions are unexported methods on the cached wrapper, construct the wrapper and test

**Context to Read:**
- design.md, section "Component: DB Integration Tests for Optimized Queries"
- `alita/db/optimized_queries.go` -- all query types and their methods
- `alita/db/testmain_test.go` -- the TestMain and skipIfNoDb pattern

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && go test -v -race -count=1 ./alita/db/... -run "TestOptimized|TestGetOptimized|TestCacheKey"
```

---

## TASK-012: DB Integration Tests for Captcha CRUD Expansion

**Complexity:** M
**Files:**
- MODIFY: `alita/db/captcha_db_test.go` -- append new test functions
**Dependencies:** None
**Description:**

Append new integration tests to `alita/db/captcha_db_test.go` to cover more CRUD lifecycle paths. All tests call `skipIfNoDb(t)`, use unique IDs, and clean up.

**New test functions:**

1. `TestCaptchaAttempt_Lifecycle(t *testing.T)`:
   - `skipIfNoDb(t)`
   - Create: `CreateCaptchaAttemptPreMessage(userID, chatID, "42", 2)` -> non-nil, no error
   - Read: `GetCaptchaAttemptByID(attempt.ID)` -> matches created
   - Verify answer field matches "42"
   - Cleanup: delete by ID

2. `TestCaptchaAttempt_IncrementAttempts(t *testing.T)`:
   - Create attempt -> initial `Attempts == 0`
   - Call `IncrementCaptchaAttempts(userID, chatID)`
   - Re-read -> `Attempts == 1`
   - Increment again -> `Attempts == 2`

3. `TestGetCaptchaSettings_NonExistentChat(t *testing.T)`:
   - `GetCaptchaSettings(999999999)` -> returns non-nil default settings
   - Assert `Enabled == false`, `CaptchaMode == "math"`, `Timeout == 2`, `FailureAction == "kick"`, `MaxAttempts == 3`

4. `TestStoredMessages_CRUD(t *testing.T)`:
   - Create captcha attempt first
   - Store 3 messages via `StoreMessage` or equivalent
   - Retrieve via `GetStoredMessages(attemptID)` -> 3 messages returned
   - Cleanup

5. `TestCaptchaMutedUsers_CleanupExpired(t *testing.T)`:
   - Insert muted user with `UnmuteAt` in the past
   - Call `CleanupExpiredCaptchaMutes()`
   - Verify record is gone

**Context to Read:**
- `alita/db/captcha_db.go` -- CRUD function signatures
- `alita/db/captcha_db_test.go` -- existing test pattern

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && go test -v -race -count=1 ./alita/db/... -run "TestCaptchaAttempt_Lifecycle|TestCaptchaAttempt_Increment|TestGetCaptchaSettings_NonExistent|TestStoredMessages_CRUD|TestCaptchaMutedUsers_Cleanup"
```

---

## TASK-013: DB Integration Tests for Greetings Expansion

**Complexity:** S
**Files:**
- MODIFY: `alita/db/greetings_db_test.go` -- append new test functions
**Dependencies:** None
**Description:**

Append new integration tests to `alita/db/greetings_db_test.go`.

**New test functions:**

1. `TestGetGreetingSettings_NonExistentChat(t *testing.T)`:
   - Use a chatID that has no records
   - `GetGreetingSettings(chatID)` -> returns non-nil with default welcome text `DefaultWelcome` and default goodbye text `DefaultGoodbye`
   - Setup chat first via `EnsureChatInDb`, cleanup both Chat and GreetingSettings

2. `TestSetWelcomeText_EmptyText(t *testing.T)`:
   - Create chat, ensure greeting exists
   - Call the welcome text setter with empty string `""`
   - Re-read -> `WelcomeText == ""`

3. `TestWelcomeAndGoodbye_Independent(t *testing.T)`:
   - Create chat, ensure greeting exists
   - Set custom welcome text
   - Set custom goodbye text
   - Re-read -> both welcome and goodbye reflect their independent updates
   - Modify welcome again -> goodbye unchanged

4. `TestResetWelcomeText(t *testing.T)`:
   - Set custom welcome, verify it persisted
   - Set back to `DefaultWelcome`, verify restoration

**Context to Read:**
- `alita/db/greetings_db.go` -- CRUD function signatures, default constants
- `alita/db/greetings_db_test.go` -- existing tests and patterns
- `alita/db/db.go` lines 37-40 -- DefaultWelcome and DefaultGoodbye constants

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && go test -v -race -count=1 ./alita/db/... -run "TestGetGreetingSettings_NonExistent|TestSetWelcomeText_Empty|TestWelcomeAndGoodbye_Independent|TestResetWelcomeText"
```

---

## TASK-014: DB Integration Tests for Remaining CRUD (disable, notes, rules, admin, connections)

**Complexity:** L
**Files:**
- MODIFY: `alita/db/disable_db_test.go` -- append 2 new tests
- MODIFY: `alita/db/notes_db_test.go` -- append 2 new tests
- MODIFY: `alita/db/rules_db_test.go` -- append 2 new tests
- MODIFY: `alita/db/admin_db_test.go` -- append 1 new test
- MODIFY: `alita/db/connections_db_test.go` -- append 2 new tests
**Dependencies:** None
**Description:**

Expand existing DB test files with additional CRUD lifecycle tests. Each file gets 1-2 new test functions.

**disable_db_test.go:**

1. `TestDisableSameCommandTwice(t *testing.T)`:
   - `skipIfNoDb(t)`, unique chatID
   - `DisableCMD(chatID, "start")` twice
   - `GetChatDisabledCMDs(chatID)` -> "start" appears exactly once, not duplicated

2. `TestEnableCommand(t *testing.T)`:
   - Disable "start", verify disabled
   - `EnableCMD(chatID, "start")`
   - `GetChatDisabledCMDs(chatID)` -> "start" no longer in list

**notes_db_test.go:**

1. `TestAddNoteWithAdminOnly(t *testing.T)`:
   - Save note with `adminOnly=true`
   - `GetNotesList(chatID, true)` (admin) -> includes the note
   - `GetNotesList(chatID, false)` (non-admin) -> excludes the note

2. `TestSaveNoteTwice_Overwrites(t *testing.T)`:
   - Save note "test" with content "v1"
   - Save note "test" again with content "v2"
   - `GetNote(chatID, "test")` -> content is "v2", not duplicate

**rules_db_test.go:**

1. `TestSetRules_EmptyString(t *testing.T)`:
   - `SetChatRules(chatID, "")` -> persists empty string
   - `GetChatRulesInfo(chatID).Rules == ""`

2. `TestClearRules(t *testing.T)`:
   - Set rules to "Some rules"
   - Set rules to ""
   - Read back -> empty

**admin_db_test.go:**

1. `TestSetAnonAdmin_Toggle(t *testing.T)`:
   - Get default (false), set to true, verify true
   - Set back to false, verify false (tests boolean zero-value persistence with UPSERT)

**connections_db_test.go:**

1. `TestConnectionForNewUser(t *testing.T)`:
   - `Connection(newUserID)` for a user with no prior connections -> returns non-nil, `Connected == false`

2. `TestDisconnectId(t *testing.T)`:
   - Connect user to chat, verify connected
   - `DisconnectId(userID)`, verify no longer connected

All tests follow `skipIfNoDb(t)`, unique IDs, `t.Cleanup()`, `t.Parallel()`.

**Context to Read:**
- design.md, section "Component: DB Integration Tests for Remaining CRUD"
- Each `*_db.go` file for function signatures
- Each existing `*_db_test.go` file for patterns

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && go test -v -race -count=1 ./alita/db/... -run "TestDisableSameCommand|TestEnableCommand|TestAddNoteWithAdmin|TestSaveNoteTwice|TestSetRules_Empty|TestClearRules|TestSetAnonAdmin_Toggle|TestConnectionForNew|TestDisconnectId"
```

---

## TASK-015: Extraction Function Additional Tests

**Complexity:** S
**Files:**
- MODIFY: `alita/utils/extraction/extraction_test.go` -- append new test functions
**Dependencies:** None
**Description:**

Append additional edge case tests to `alita/utils/extraction/extraction_test.go`.

**New test functions:**

1. `TestExtractQuotes_AdditionalEdgeCases(t *testing.T)` -- table-driven:
   - Single word no spaces: `"hello"` with matchWord=true -> inQuotes="hello", after=""
   - Quoted with trailing spaces: `'"hello"   '` with matchQuotes=true -> inQuotes="hello", after="" (trimmed)
   - Multiple quotes: `'"first" "second"'` with matchQuotes=true -> inQuotes="first", after='"second"'

2. `TestIdFromReply_NilReply(t *testing.T)`:
   - Construct `gotgbot.Message{ReplyToMessage: nil}`
   - Call `IdFromReply` or equivalent extraction function
   - Assert no panic, returns 0 or appropriate default

3. `TestExtractQuotes_UnicodeContent(t *testing.T)`:
   - Input with unicode: `'"cafe\u0301"'` with matchQuotes=true
   - Assert correctly extracted without corruption

All tests use `t.Parallel()`.

**Context to Read:**
- `alita/utils/extraction/extraction_test.go` -- existing tests
- `alita/utils/extraction/extraction.go` -- function signatures

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && go test -v -race -count=1 ./alita/utils/extraction/... -run "TestExtractQuotes_Additional|TestIdFromReply_Nil|TestExtractQuotes_Unicode"
```

---

## TASK-016: Monitoring Pure Function Expanded Tests

**Complexity:** S
**Files:**
- MODIFY: `alita/utils/monitoring/auto_remediation_test.go` -- append tests
- MODIFY: `alita/utils/monitoring/background_stats_test.go` -- append tests
**Dependencies:** None
**Description:**

Expand monitoring tests with additional pure function tests.

**auto_remediation_test.go -- append:**

1. `TestAggressiveGCAction_CanExecute(t *testing.T)`:
   - Create `AggressiveGCAction{}`
   - Metrics with `MemoryAllocMB` below 150% of `ResourceMaxMemoryMB` -> `CanExecute() == false`
   - Metrics with `MemoryAllocMB` above 150% -> `CanExecute() == true`

2. `TestWarningAction_CanExecute(t *testing.T)`:
   - Create `WarningAction{}`
   - Metrics with `MemoryAllocMB` below 80% of `ResourceMaxMemoryMB` -> `CanExecute() == false`
   - Metrics above 80% -> `CanExecute() == true`

3. `TestGCAction_NameAndSeverity(t *testing.T)`:
   - `GCAction{}.Name()` -> non-empty string
   - `GCAction{}.Severity()` -> valid string (not empty)
   - `AggressiveGCAction{}.Name()` -> non-empty
   - `WarningAction{}.Name()` -> non-empty

**background_stats_test.go -- append:**

1. `TestCollectSystemStats(t *testing.T)`:
   - Call `CollectSystemStats()` (if exported) or the relevant method on the collector
   - Assert `NumGoroutines > 0` and `MemAllocMB >= 0`

2. `TestRecordMessageAndError(t *testing.T)`:
   - Create collector via `NewBackgroundStatsCollector()`
   - Call `RecordMessage()` 3 times, `RecordError()` 2 times
   - Verify counters reflect the calls (access internal counters or use exported methods)

All tests use `t.Parallel()`. These tests import `alita/config` (the monitoring package does), so they pass in CI with env vars.

**Context to Read:**
- `alita/utils/monitoring/auto_remediation.go` -- action types and CanExecute methods
- `alita/utils/monitoring/background_stats.go` -- collector and stats methods
- Existing test files for patterns

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && go test -v -race -count=1 ./alita/utils/monitoring/... -run "TestAggressiveGCAction|TestWarningAction|TestGCAction_Name|TestCollectSystemStats|TestRecordMessage"
```

---

## TASK-017: Full Integration Verification

**Complexity:** S
**Files:** None (verification only)
**Dependencies:** TASK-001, TASK-002, TASK-003, TASK-004, TASK-005, TASK-006, TASK-007, TASK-008, TASK-009, TASK-010, TASK-011, TASK-012, TASK-013, TASK-014, TASK-015, TASK-016
**Description:**

Run the full test suite and verify coverage meets the 45% target. This task does NOT create or modify any files. It runs verification commands and reports results.

**Steps:**

1. Run linter to ensure no new issues:
   ```bash
   cd /Users/divkix/GitHub/Alita_Robot && make lint
   ```

2. Run full test suite with coverage:
   ```bash
   cd /Users/divkix/GitHub/Alita_Robot && make test
   ```

3. Check coverage threshold:
   ```bash
   cd /Users/divkix/GitHub/Alita_Robot && go tool cover -func=coverage.out | grep '^total:'
   ```
   Assert: total coverage >= 40.0% (hard floor), target >= 45.0%.

4. Verify test isolation (shuffle order):
   ```bash
   cd /Users/divkix/GitHub/Alita_Robot && go test -v -race -count=1 -shuffle=on -timeout 10m ./...
   ```

5. Check for race conditions with multiple runs:
   ```bash
   cd /Users/divkix/GitHub/Alita_Robot && go test -race -count=3 -timeout 10m ./alita/db/... ./alita/config/... ./alita/i18n/... ./alita/modules/...
   ```

If coverage is below 40%, identify which tasks failed or which test files are not contributing expected coverage and report the gap.

**Context to Read:**
- `Makefile` -- test command definition
- `.github/workflows/ci.yml` -- CI threshold check logic

**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && make test && go tool cover -func=coverage.out | grep '^total:'
```

---

## File Manifest

| Task | Files Touched |
|------|---------------|
| TASK-001 | `alita/db/gorm_types_test.go` (CREATE) |
| TASK-002 | `alita/db/gorm_types_test.go` (MODIFY) |
| TASK-003 | `alita/db/gorm_types_test.go` (MODIFY) |
| TASK-004 | `alita/config/config_test.go` (CREATE) |
| TASK-005 | `alita/config/config_test.go` (MODIFY) |
| TASK-006 | `alita/config/config_test.go` (MODIFY) |
| TASK-007 | `alita/i18n/i18n_test.go` (MODIFY) |
| TASK-008 | `alita/modules/chat_permissions_test.go` (CREATE) |
| TASK-009 | `alita/modules/helpers_test.go` (CREATE) |
| TASK-010 | `alita/db/migrations_test.go` (MODIFY) |
| TASK-011 | `alita/db/optimized_queries_test.go` (CREATE) |
| TASK-012 | `alita/db/captcha_db_test.go` (MODIFY) |
| TASK-013 | `alita/db/greetings_db_test.go` (MODIFY) |
| TASK-014 | `alita/db/disable_db_test.go` (MODIFY), `alita/db/notes_db_test.go` (MODIFY), `alita/db/rules_db_test.go` (MODIFY), `alita/db/admin_db_test.go` (MODIFY), `alita/db/connections_db_test.go` (MODIFY) |
| TASK-015 | `alita/utils/extraction/extraction_test.go` (MODIFY) |
| TASK-016 | `alita/utils/monitoring/auto_remediation_test.go` (MODIFY), `alita/utils/monitoring/background_stats_test.go` (MODIFY) |
| TASK-017 | None (verification only) |

## Parallelism Analysis

**Fully independent tasks (can run in parallel with each other):**
- TASK-001 (creates `alita/db/gorm_types_test.go`)
- TASK-004 (creates `alita/config/config_test.go`)
- TASK-007 (modifies `alita/i18n/i18n_test.go`)
- TASK-008 (creates `alita/modules/chat_permissions_test.go`)
- TASK-009 (creates `alita/modules/helpers_test.go`)
- TASK-010 (modifies `alita/db/migrations_test.go`)
- TASK-011 (creates `alita/db/optimized_queries_test.go`)
- TASK-012 (modifies `alita/db/captcha_db_test.go`)
- TASK-013 (modifies `alita/db/greetings_db_test.go`)
- TASK-014 (modifies `disable_db_test.go`, `notes_db_test.go`, `rules_db_test.go`, `admin_db_test.go`, `connections_db_test.go`)
- TASK-015 (modifies `alita/utils/extraction/extraction_test.go`)
- TASK-016 (modifies `alita/utils/monitoring/*_test.go`)

**Sequential dependencies:**
- TASK-002 depends on TASK-001 (appends to same file)
- TASK-003 depends on TASK-001 (appends to same file)
- TASK-005 depends on TASK-004 (appends to same file)
- TASK-006 depends on TASK-004 (appends to same file)
- TASK-017 depends on ALL other tasks

**Maximum parallelism: 12 tasks can run simultaneously** (TASK-001, TASK-004, TASK-007, TASK-008, TASK-009, TASK-010, TASK-011, TASK-012, TASK-013, TASK-014, TASK-015, TASK-016).

## Risk Register

| Task | Risk | Mitigation |
|------|------|------------|
| TASK-004, TASK-005, TASK-006 | Config `init()` crashes without env vars (`BOT_TOKEN` etc.) -- tests cannot run locally | Tests include `skipIfNoConfig(t)` guard. CI sets all required env vars. Document in test file header. |
| TASK-011, TASK-012, TASK-013, TASK-014 | PostgreSQL unavailable in CI -> DB integration tests skipped, coverage drops ~10% | CI has PostgreSQL 16 service configured. `skipIfNoDb(t)` ensures graceful skip. If PostgreSQL fails, CI job fails before coverage check. |
| TASK-011, TASK-012, TASK-013, TASK-014 | DB test isolation failure (ID collisions between parallel tests) | Strict use of `time.Now().UnixNano()` for unique IDs. Each test cleans up via `t.Cleanup()`. Tests use separate chatIDs. |
| TASK-009 | `TestListModules` modifies package-level `HelpModule` state | Test must NOT use `t.Parallel()`. Uses `t.Cleanup()` to re-initialize `HelpModule.AbleMap`. |
| TASK-006 | `t.Setenv()` incompatible with `t.Parallel()` | Top-level Redis test functions do NOT call `t.Parallel()`. Subtests are sequential. Negligible performance impact (~10 cases). |
| TASK-017 | Total statement count higher than estimated (~11,288) -> coverage % lower than 45% target | If coverage is 40-45%, it still passes CI threshold. If below 40%, identify gaps and add more test cases to highest-impact areas (modules helpers, more DB integration tests). |
| ALL | Existing tests break due to unrelated regressions | Run `make test` before starting any work to confirm green baseline. Each task verifies independently. |
| TASK-002, TASK-003 | Appending to file created by TASK-001 could cause merge conflicts | TASK-002 and TASK-003 depend on TASK-001, so they run sequentially after TASK-001 completes. TASK-002 and TASK-003 can run in parallel with each other since they append different functions. |

TASKS_COMPLETE
