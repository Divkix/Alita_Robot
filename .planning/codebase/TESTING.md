# Testing Patterns

**Analysis Date:** 2026-02-23

## Test Framework

**Runner:**
- Go's built-in `testing` package — no external test runner
- Go 1.25+

**Assertion Library:**
- None. Standard `t.Fatalf`, `t.Fatal`, `t.Errorf`, `t.Error` only. No testify or gomock.

**Run Commands:**
```bash
make test              # Run all tests with race detector, coverage, 10m timeout
go test ./...          # Run all tests without coverage flags
go test -run TestName ./alita/db/  # Run specific test in a package
```

**Full test command from Makefile:**
```bash
go test -v -race -coverprofile=coverage.out -coverpkg=$(go list ./... | grep -v 'scripts/' | paste -sd, -) -count=1 -timeout 10m ./...
```

## Test File Organization

**Location:**
- Co-located with source in the same package directory
- Same package name as source (`package db`, `package modules`, `package callbackcodec`)
- Exception: tests accessing unexported symbols use same package (no `_test` suffix)

**Naming:**
- `<source_file>_test.go` (e.g., `notes_db.go` → `notes_db_test.go`)
- Shared DB setup file: `alita/db/testmain_test.go` — defines `TestMain` and `skipIfNoDb`
- Shared config setup: `alita/config/config_test.go` defines `skipIfNoConfig` and `validBaseConfig()`

**Structure:**
```
alita/db/
├── notes_db.go
├── notes_db_test.go          # Co-located tests for notes DB operations
├── testmain_test.go          # TestMain + skipIfNoDb() shared helper
├── cache_helpers_test.go
└── optimized_queries_test.go

alita/utils/callbackcodec/
├── callbackcodec.go
└── callbackcodec_test.go

alita/modules/
├── helpers.go
├── helpers_test.go
├── callback_codec_test.go
└── ...
```

## Test Structure

**Suite Organization:**
```go
func TestFunctionName_Scenario(t *testing.T) {
    t.Parallel()
    // setup
    // act
    // assert with t.Fatalf
}

// Table-driven tests for multiple inputs:
func TestFunctionName(t *testing.T) {
    t.Parallel()

    tests := []struct {
        name      string
        input     string
        wantValue string
        wantErr   bool
    }{
        {name: "scenario description", input: "x", wantValue: "y"},
    }

    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            t.Parallel()
            // test body using tc.*
        })
    }
}

// Sub-tests for grouping related scenarios:
func TestFunctionName(t *testing.T) {
    t.Parallel()

    t.Run("first scenario", func(t *testing.T) {
        t.Parallel()
        // ...
    })

    t.Run("second scenario", func(t *testing.T) {
        t.Parallel()
        // ...
    })
}
```

**Patterns:**
- `t.Parallel()` called at the top of every test function and every subtest, unless the test modifies package-level state (e.g., `TestListModules` explicitly does NOT call `t.Parallel()`)
- Setup done inline, no shared `setUp`/`tearDown` methods
- Cleanup via `t.Cleanup(func() { ... })` — always used for DB tests to delete created records
- Skip conditions checked at the top via helper functions: `skipIfNoDb(t)`, `skipIfNoConfig(t)`

## Mocking

**Framework:** None. No mock library used.

**Patterns:**
- No interface mocking: DB layer and external services are tested directly against real infrastructure (real PostgreSQL via `DATABASE_URL` env var, real Redis)
- Nil-check guards tested directly: `&OptimizedLockQueries{db: nil}` passed to test error paths without a DB
- External Telegram API calls: not tested (no mock bot). Handler logic that requires `*gotgbot.Bot` is not unit-tested at the handler level
- Struct construction used directly: `&Decoded{Namespace: "test", Fields: map[string]string{"a": "yes"}}`

**What IS tested without external deps:**
- Pure logic functions: codec, string manipulation, keyword matching, error wrapping, chat status helpers, config validation
- Struct methods: `ButtonArray.Scan/Value`, `moduleEnabled.Store/Load`, `WrappedError.Error()`
- Concurrency safety: tested with `sync.WaitGroup` and multiple goroutines (e.g., `TestConcurrentAccess`, `TestConcurrentRecordMessage`)

## Fixtures and Factories

**Test Data:**
```go
// DB tests: use time.Now().UnixNano() for unique chat/user IDs to avoid parallel test collisions
chatID := time.Now().UnixNano()

// Config tests: use factory function returning valid struct
func validBaseConfig() *Config {
    return &Config{
        BotToken:    "test-token",
        OwnerId:     1,
        DatabaseURL: "postgres://localhost/test",
        // ...
    }
}

// Table-driven: inline struct literals in tests slice
tests := []struct {
    name    string
    data    string
    err     error
}{
    {name: "missing separators", data: "notes.overwrite", err: ErrInvalidFormat},
}
```

**Cleanup Pattern:**
```go
t.Cleanup(func() {
    _ = DB.Where("chat_id = ?", chatID).Delete(&Notes{}).Error
    _ = DB.Where("chat_id = ?", chatID).Delete(&NotesSettings{}).Error
    _ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
})
```

**Location:**
- No shared fixtures directory. All test data is inline per test.
- `alita/db/testmain_test.go` provides the shared `skipIfNoDb` helper and `TestMain` for DB package

## Coverage

**Requirements:** No enforced minimum. Coverage profile generated on every `make test` run.

**View Coverage:**
```bash
make test                          # Generates coverage.out
go tool cover -html=coverage.out   # Open HTML coverage report
```

**Coverage scope:**
- `make test` uses `-coverpkg` set to all packages excluding `scripts/` — measures cross-package coverage

## Test Types

**Unit Tests:**
- Scope: pure functions, struct methods, error types, codec encode/decode, string parsing
- Location: co-located with source in same package
- No external dependencies required
- Examples: `alita/utils/callbackcodec/`, `alita/utils/errors/`, `alita/utils/keyword_matcher/`, `alita/utils/string_handling/`

**Integration Tests (DB):**
- Scope: GORM model operations against real PostgreSQL
- Location: `alita/db/*_test.go`
- Require: `DATABASE_URL` env var pointing to running PostgreSQL
- Skip when DB unavailable via `skipIfNoDb(t)` which calls `t.Skip()`
- `TestMain` in `alita/db/testmain_test.go` runs `AutoMigrate` for all models before tests
- Tests are parallel-safe using unique IDs (`time.Now().UnixNano()`)

**E2E Tests:**
- Not used. No end-to-end Telegram bot testing framework present.

## Common Patterns

**Conditional skip for infrastructure:**
```go
func skipIfNoDb(t *testing.T) {
    t.Helper()
    if DB == nil {
        t.Skip("requires PostgreSQL connection")
    }
}

func skipIfNoConfig(t *testing.T) {
    t.Helper()
    if os.Getenv("BOT_TOKEN") == "" {
        t.Skip("skipping: BOT_TOKEN not set (config.init() would fatalf)")
    }
}
```

**Async/Concurrent Testing:**
```go
func TestConcurrentAccess(t *testing.T) {
    t.Parallel()

    km := NewKeywordMatcher([]string{"hello", "world"})
    const goroutines = 10
    const callsEach = 100

    var wg sync.WaitGroup
    wg.Add(goroutines)

    for range goroutines {
        go func() {
            defer wg.Done()
            for range callsEach {
                _ = km.FindMatches("hello world")
            }
        }()
    }
    wg.Wait()
}
```

**Error Testing:**
```go
// errors.Is for sentinel errors:
if !errors.Is(err, ErrInvalidNamespace) {
    t.Fatalf("expected ErrInvalidNamespace, got %v", err)
}

// Type assertion for custom error types:
we, ok := result.(*WrappedError)
if !ok {
    t.Fatalf("expected *WrappedError, got %T", result)
}

// Nil error expected:
if err != nil {
    t.Fatalf("unexpected error: %v", err)
}
```

**Panic Recovery Testing:**
```go
// Test that a function recovers from panics internally:
var panicCaught bool
func() {
    defer func() {
        if r := recover(); r != nil {
            panicCaught = true
        }
    }()
    _ = m.executeHandler(func() error {
        panic("test panic")
    }, 2)
}()
if panicCaught {
    t.Fatal("panic escaped executeHandler — recovery is not working")
}
```

**Table-driven test loop:**
```go
for _, tc := range tests {
    t.Run(tc.name, func(t *testing.T) {
        t.Parallel()
        // tc.* fields used directly
        // No extra setup
    })
}
```

**Channel timeout test:**
```go
select {
case metrics := <-c.systemStatsChan:
    // assertions on metrics
case <-time.After(1 * time.Second):
    t.Fatal("timeout: expected value not received within 1s")
}
```

**State isolation for package-level vars:**
```go
// TestListModules modifies package-level HelpModule state -- do NOT use t.Parallel().
func TestListModules(t *testing.T) {
    t.Cleanup(func() {
        HelpModule.AbleMap.Init()
    })
    HelpModule.AbleMap.Init()
    // test body
}
```

## Test Section Dividers (Style)

Test files use ASCII divider comments to group related tests visually:
```go
// ---------------------------------------------------------------------------
// FunctionName
// ---------------------------------------------------------------------------

func TestFunctionName_Case1(t *testing.T) { ... }
func TestFunctionName_Case2(t *testing.T) { ... }
```

This pattern is used in: `alita/utils/helpers/helpers_test.go`, `alita/utils/monitoring/background_stats_test.go`

---

*Testing analysis: 2026-02-23*
