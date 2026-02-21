# Technical Design: Increase Test Coverage on Critical Codebase Paths

**Date:** 2026-02-21
**Requirements Source:** requirements.md
**Codebase Conventions:** Standard library `testing` only, table-driven subtests with `t.Parallel()`, `t.Fatalf()` assertions, named test case structs. Reference pattern: `alita/utils/callbackcodec/callbackcodec_test.go`.

## Design Overview

This design covers adding unit tests to 44+ pure functions across 10 packages in the Alita_Robot Go codebase. The current test-to-code ratio is 3.4% (1,184 lines of tests for 35,000 lines of code), and only 1 package out of ~30 passes tests locally.

The work is split into two phases based on a hard infrastructure constraint: the `config` package's `init()` function calls `log.Fatalf` when `BOT_TOKEN` and other env vars are missing, which kills the test runner. Any package that transitively imports `config` (through `db`, `cache`, `i18n`, etc.) cannot be tested without dummy env vars. Phase 1 covers 4 packages with zero transitive dependency on `config`. Phase 2 covers 6 packages that require the CI dummy env vars (`BOT_TOKEN=test-token`, `OWNER_ID=1`, `MESSAGE_DUMP=1`, `DATABASE_URL=postgres://...`, `REDIS_ADDRESS=localhost:6379`).

Every test file follows the exact pattern established in `/Users/divkix/GitHub/Alita_Robot/alita/utils/callbackcodec/callbackcodec_test.go`: package-internal tests (`package X`, not `package X_test`), table-driven subtests, `t.Parallel()` at both top-level and subtest level, `t.Fatalf()` for assertions, and no external test dependencies. No new entries in `go.mod`.

## Component Architecture

### Component: string_handling Tests

**Responsibility:** Verify `FindInStringSlice`, `FindInInt64Slice`, `IsDuplicateInStringSlice` handle all input combinations correctly.
**Location:** `/Users/divkix/GitHub/Alita_Robot/alita/utils/string_handling/string_handling_test.go` (NEW)
**Pattern:** Table-driven subtests per function, same as `callbackcodec_test.go`.
**Phase:** 1 (zero deps)
**Traces to:** US-001

**Public Interface:**
```go
package string_handling

func TestFindInStringSlice(t *testing.T)       // table-driven: nil/empty/present/absent/empty-string-value
func TestFindInInt64Slice(t *testing.T)         // table-driven: nil/empty/present/absent/zero/negative/boundary
func TestIsDuplicateInStringSlice(t *testing.T) // table-driven: nil/empty/no-dup/dup/all-same/empty-string-dup
```

**Dependencies:** None beyond stdlib.

**Error Handling:**
- All functions are pure with no error returns. Tests use `t.Fatalf` for assertion failures.

---

### Component: errors Tests

**Responsibility:** Verify `Wrap`, `Wrapf`, `WrappedError.Error()`, `WrappedError.Unwrap()` produce correct error chains with file/line metadata.
**Location:** `/Users/divkix/GitHub/Alita_Robot/alita/utils/errors/errors_test.go` (NEW)
**Pattern:** Table-driven subtests plus individual tests for chain behavior.
**Phase:** 1 (stdlib only)
**Traces to:** US-002

**Public Interface:**
```go
package errors

func TestWrapNilError(t *testing.T)        // Wrap(nil, "msg") -> nil
func TestWrapNonNilError(t *testing.T)     // Wrap(err, "msg") -> WrappedError with file/line/func
func TestWrapfNilError(t *testing.T)       // Wrapf(nil, "%s", "x") -> nil
func TestWrapfFormatsMessage(t *testing.T) // Wrapf(err, "op %s id %d", "save", 42) -> formatted
func TestWrappedErrorFormat(t *testing.T)  // .Error() contains "at", file:line, "in", function, original
func TestUnwrapChain(t *testing.T)         // errors.Is and errors.Unwrap work through chain
func TestDoubleWrap(t *testing.T)          // Wrap(Wrap(err, "inner"), "outer") -> both accessible
func TestWrapEmptyMessage(t *testing.T)    // Wrap(err, "") -> still valid with file/line
func TestFilePathTruncation(t *testing.T)  // File field has at most 2 path segments
```

**Dependencies:** `errors` (stdlib), `strings` (stdlib) for output assertions.

**Error Handling:**
- `Wrap(nil, ...)` returns nil -- tested explicitly.
- `runtime.Caller` failure path is untestable (requires corrupted stack) -- accepted gap.

---

### Component: keyword_matcher Tests

**Responsibility:** Verify Aho-Corasick matcher handles case-insensitive matching, empty inputs, overlapping patterns, and concurrent access.
**Location:** `/Users/divkix/GitHub/Alita_Robot/alita/utils/keyword_matcher/matcher_test.go` (NEW)
**Pattern:** Table-driven subtests plus concurrent stress test with `sync.WaitGroup`.
**Phase:** 1 (only depends on `ahocorasick` and `logrus`, no config)
**Traces to:** US-004

**Public Interface:**
```go
package keyword_matcher

func TestNewKeywordMatcher(t *testing.T)         // constructs matcher with various pattern sets
func TestFindMatches(t *testing.T)               // table-driven: basic/case-insensitive/overlapping/empty
func TestHasMatch(t *testing.T)                  // table-driven: present/absent/empty-text/empty-patterns
func TestGetPatterns(t *testing.T)               // returns copy, mutation of copy does not affect internal
func TestFindMatchesPositions(t *testing.T)      // verifies Start/End positions are correct
func TestConcurrentAccess(t *testing.T)          // 10 goroutines calling FindMatches/HasMatch simultaneously
func TestNewKeywordMatcherNilPatterns(t *testing.T) // nil input -> HasMatch returns false
func TestFindMatchesEmptyText(t *testing.T)      // empty string -> nil
func TestSpecialCharacterPatterns(t *testing.T)  // regex metacharacters matched literally
```

**Dependencies:**
- `sync` (stdlib) -- for concurrent access test
- `cloudflare/ahocorasick` -- already in `go.mod`, used transitively

**Error Handling:**
- No error returns from public API. Tests verify nil returns for empty inputs and correct behavior under concurrency via `-race` flag.

---

### Component: callbackcodec Gap-Fill Tests

**Responsibility:** Increase coverage from 79.1% to >95% by testing `EncodeOrFallback`, nil `Decoded.Field`, empty fields, and URL-special character round-trips.
**Location:** `/Users/divkix/GitHub/Alita_Robot/alita/utils/callbackcodec/callbackcodec_test.go` (MODIFY -- append)
**Pattern:** Same as existing tests in this file.
**Phase:** 1 (zero deps)
**Traces to:** US-005

**Public Interface:**
```go
// Appended to existing file:

func TestEncodeOrFallbackSuccess(t *testing.T)         // valid encode returns encoded data
func TestEncodeOrFallbackInvalidNamespace(t *testing.T) // returns fallback on invalid namespace
func TestEncodeOrFallbackOversized(t *testing.T)        // returns fallback on oversized payload
func TestEncodeOrFallbackEmptyFallback(t *testing.T)    // returns "" on failure with empty fallback
func TestFieldNilReceiver(t *testing.T)                 // (*Decoded)(nil).Field("x") -> ("", false)
func TestFieldExistingKey(t *testing.T)                 // d.Field("a") -> ("yes", true)
func TestFieldMissingKey(t *testing.T)                  // d.Field("missing") -> ("", false)
func TestEncodeEmptyFields(t *testing.T)                // Encode("ns", map[]{}) -> valid with "_" payload
func TestEncodeNilFields(t *testing.T)                  // Encode("ns", nil) -> valid with "_" payload
func TestEncodeSkipsEmptyKey(t *testing.T)              // Encode("ns", {"": "val"}) -> key skipped
func TestDecodeUnderscorePayload(t *testing.T)          // Decode("ns|v1|_") -> empty Fields map
func TestRoundTripURLSpecialChars(t *testing.T)         // values with &, =, % survive round-trip
```

**Dependencies:** Same as existing: `errors`, `strings`, `testing`.

**Error Handling:**
- `EncodeOrFallback` swallows errors and returns fallback -- tested explicitly.

---

### Component: config/types Tests

**Responsibility:** Verify `typeConvertor` methods handle all string-to-type conversion edge cases.
**Location:** `/Users/divkix/GitHub/Alita_Robot/alita/config/types_test.go` (NEW)
**Pattern:** Table-driven subtests. Internal package test (`package config`) since `typeConvertor` is unexported.
**Phase:** 2 (config `init()` runs on import -- requires dummy env vars)
**Traces to:** US-003

**Public Interface:**
```go
package config

func TestTypeConvertorBool(t *testing.T)        // table-driven: true/yes/1/TRUE/YES/false/no/0/empty/whitespace
func TestTypeConvertorInt(t *testing.T)          // table-driven: valid/empty/invalid/negative/overflow
func TestTypeConvertorInt64(t *testing.T)        // table-driven: valid/empty/invalid/MaxInt64/whitespace
func TestTypeConvertorFloat64(t *testing.T)      // table-driven: valid/empty/invalid/NaN/Inf
func TestTypeConvertorStringArray(t *testing.T)  // table-driven: csv/trimmed/single/empty/consecutive-commas
```

**Dependencies:** `math` (stdlib) for boundary values.

**Error Handling:**
- All methods silently return zero values on parse failure. Tests verify these zero-value returns explicitly.

---

### Component: helpers Tests

**Responsibility:** Verify 13 pure helper functions for keyboard building, HTML escaping, message splitting, error classification, and note option parsing.
**Location:** `/Users/divkix/GitHub/Alita_Robot/alita/utils/helpers/helpers_test.go` (NEW)
**Pattern:** Table-driven subtests. Internal package test (`package helpers`) since `notesParser` is unexported.
**Phase:** 2 (imports `config`, `db`, `i18n` transitively)
**Traces to:** US-006

**Public Interface:**
```go
package helpers

func TestIsChannelID(t *testing.T)           // table-driven: channel/group/user/zero/boundary
func TestSplitMessage(t *testing.T)          // table-driven: under-limit/at-limit/over-limit/utf8/empty/long-single-line
func TestMentionHtml(t *testing.T)           // table-driven: basic/html-special-chars-in-name
func TestMentionUrl(t *testing.T)            // table-driven: basic/html-escape-in-name
func TestHtmlEscape(t *testing.T)            // table-driven: all-entities/empty/no-special-chars/double-escape-prevention
func TestGetFullName(t *testing.T)           // table-driven: both-names/first-only/empty-strings
func TestBuildKeyboard(t *testing.T)         // table-driven: empty/single/sameline/first-sameline
func TestConvertButtonV2ToDbButton(t *testing.T) // table-driven: basic/empty/nil
func TestRevertButtons(t *testing.T)         // table-driven: basic/sameline/empty
func TestChunkKeyboardSlices(t *testing.T)   // table-driven: exact-chunk/remainder/empty/larger-chunk
func TestReverseHTML2MD(t *testing.T)        // table-driven: bold/italic/link/no-html/nested
func TestIsExpectedTelegramError(t *testing.T) // table-driven: nil/all-17-expected-errors/unexpected-error
func TestNotesParser(t *testing.T)           // table-driven: each-tag-alone/all-tags/no-tags/combinations
```

**Dependencies:**
- `github.com/PaulSonOfLars/gotgbot/v2` -- for `gotgbot.InlineKeyboardButton` struct construction
- `github.com/PaulSonOfLars/gotg_md2html` -- for `tgmd2html.ButtonV2` struct construction
- `github.com/divkix/Alita_Robot/alita/db` -- for `db.Button` struct construction

**Error Handling:**
- `notesParser` uses `regexp.MatchString` which can error on invalid regex -- but the regexes are hardcoded string literals, so errors are impossible in practice. Tests do not need to exercise this path.
- `ChunkKeyboardSlices` with `chunkSize=0` would infinite loop. The test SHALL NOT call this case (it is a known defect to document, not a test target). The test documents this edge case with a comment.

---

### Component: chat_status Tests

**Responsibility:** Verify `IsValidUserId` and `IsChannelId` correctly distinguish users, channels, groups, and boundary IDs.
**Location:** `/Users/divkix/GitHub/Alita_Robot/alita/utils/chat_status/chat_status_test.go` (NEW)
**Pattern:** Table-driven subtests.
**Phase:** 2 (imports `db` -> `config`)
**Traces to:** US-007

**Public Interface:**
```go
package chat_status

func TestIsValidUserId(t *testing.T) // table-driven: positive/zero/negative/channel-id/MaxInt64/MinInt64/system-ids
func TestIsChannelId(t *testing.T)   // table-driven: channel/group/user/zero/boundary/-1000000000000/-1000000000001
```

**Dependencies:** `math` (stdlib) for boundary values.

**Error Handling:** None -- pure boolean functions.

---

### Component: i18n Tests

**Responsibility:** Verify locale loading utilities, YAML validation, error formatting, parameter interpolation, and plural form selection.
**Location:** `/Users/divkix/GitHub/Alita_Robot/alita/i18n/i18n_test.go` (NEW)
**Pattern:** Table-driven subtests. Internal package test (`package i18n`) since `extractLangCode`, `isYAMLFile`, `validateYAMLStructure`, `extractOrderedValues`, `selectPluralForm` are all unexported.
**Phase:** 2 (imports `alita/utils/cache` -> `config`)
**Traces to:** US-008, US-010

**Public Interface:**
```go
package i18n

// Loader utilities (US-008)
func TestExtractLangCode(t *testing.T)       // table-driven: en.yml/en.yaml/pt-BR.yml/no-ext/double-ext
func TestIsYAMLFile(t *testing.T)            // table-driven: .yml/.yaml/.json/empty/uppercase
func TestValidateYAMLStructure(t *testing.T) // table-driven: valid-map/invalid-yaml/list-root/scalar-root/empty

// Error types (US-008 + US-010)
func TestI18nErrorFormat(t *testing.T)         // with-err/without-err output format
func TestI18nErrorUnwrap(t *testing.T)         // non-nil/nil underlying error
func TestNewI18nError(t *testing.T)            // constructor sets all fields
func TestPredefinedErrorsDistinct(t *testing.T) // all 6 sentinel errors are distinct
func TestPredefinedErrorsChain(t *testing.T)    // errors.Is works through I18nError -> sentinel

// Translator utilities (US-008)
func TestExtractOrderedValues(t *testing.T)  // table-driven: numbered/common-keys/nil/empty/gap-in-numbered/mixed
func TestSelectPluralForm(t *testing.T)      // table-driven: count=0+zero/count=1+one/count=2+two/fallback-other/all-empty
```

**Dependencies:**
- `errors` (stdlib) -- for `errors.Is` chain testing
- `gopkg.in/yaml.v3` -- already in `go.mod`, used transitively by `validateYAMLStructure`
- `github.com/spf13/viper` -- already in `go.mod`, needed to construct `Translator` for `selectPluralForm`

**Error Handling:**
- `validateYAMLStructure` returns `*I18nError` wrapping YAML parse errors -- tested with invalid input.
- `selectPluralForm` requires constructing a `Translator` with a minimal `LocaleManager` (just needs `defaultLang` field set). No `init()` involvement.

**Constructing Translator for selectPluralForm:**
```go
func makeTestTranslator() *Translator {
    return &Translator{
        langCode: "en",
        manager:  &LocaleManager{defaultLang: "en"},
    }
}
```
This avoids calling `GetManager()` (singleton) or `Initialize()` (requires embedded FS).

---

### Component: rules_format Tests

**Responsibility:** Verify `normalizeRulesForHTML` detects HTML tags and passes through, or converts markdown to HTML when no tags are present.
**Location:** `/Users/divkix/GitHub/Alita_Robot/alita/modules/rules_format_test.go` (NEW)
**Pattern:** Table-driven subtests. Internal package test (`package modules`) since `normalizeRulesForHTML` is unexported.
**Phase:** 2 (modules package imports config, db, etc.)
**Traces to:** US-009

**Public Interface:**
```go
package modules

func TestNormalizeRulesForHTML(t *testing.T) // table-driven: empty/whitespace/html-tags/markdown/self-closing/angle-brackets-not-tags/html-entities
```

**Dependencies:**
- `github.com/PaulSonOfLars/gotg_md2html` -- used transitively to verify markdown conversion output

**Error Handling:** None -- pure string function.

## Data Models

No new data models. All tests operate on existing types:
- `string_handling`: primitive slices
- `errors`: `WrappedError` struct (existing)
- `keyword_matcher`: `KeywordMatcher`, `MatchResult` structs (existing)
- `callbackcodec`: `Decoded` struct (existing)
- `config`: `typeConvertor` struct (existing, unexported)
- `helpers`: `db.Button`, `gotgbot.InlineKeyboardButton`, `tgmd2html.ButtonV2` (existing)
- `chat_status`: primitive `int64`
- `i18n`: `I18nError`, `PluralRule`, `TranslationParams`, `Translator`, `LocaleManager` (existing)
- `modules`: primitive `string`

No database migrations required.

## Data Flow

### Flow: Test Execution (Phase 1)

1. Developer runs `go test ./alita/utils/string_handling/... ./alita/utils/errors/... ./alita/utils/keyword_matcher/... ./alita/utils/callbackcodec/...`
2. Go test runner compiles each package.
3. No `init()` functions trigger that require env vars.
4. Table-driven subtests execute in parallel via `t.Parallel()`.
5. Assertions use `t.Fatalf()` for failures.
6. Race detector validates concurrent access (keyword_matcher).
7. Exit code 0 on success.

### Flow: Test Execution (Phase 2)

1. CI sets env vars: `BOT_TOKEN=test-token`, `OWNER_ID=1`, `MESSAGE_DUMP=1`, `DATABASE_URL=postgres://postgres:postgres@localhost:5432/alita_test?sslmode=disable`, `REDIS_ADDRESS=localhost:6379`.
2. `make test` runs `go test -v -race -coverprofile=coverage.out -count=1 -timeout 10m ./...`
3. Config `init()` runs, reads env vars, does NOT call `log.Fatalf`.
4. DB `init()` runs, attempts PostgreSQL connection (may fail -- tests in `config/types`, `helpers`, `chat_status`, `i18n`, `modules` do NOT use DB at runtime so this is acceptable; the package just needs to compile and init).
5. Pure function tests execute and pass.

**Error paths:**
- Step 1 missing (no env vars): Phase 2 tests fail immediately with `log.Fatalf`. Phase 1 tests unaffected.
- Step 4 DB connection fails: Tests still pass because the tested functions are pure and never touch DB at runtime.

## API Contracts

No new APIs. All components are test files that consume existing function signatures. The test function signatures follow Go convention: `func TestXxx(t *testing.T)`.

## Testing Strategy

### Test File Template

Every new test file follows this exact template, derived from the existing `callbackcodec_test.go`:

```go
package <package_name>

import (
    "testing"
    // additional stdlib imports as needed
)

func TestFunctionName(t *testing.T) {
    t.Parallel()

    tests := []struct {
        name string
        // input fields
        // expected output fields
    }{
        {name: "descriptive case name", /* fields */},
        {name: "another case", /* fields */},
    }

    for _, tc := range tests {
        tc := tc // capture range variable (required for Go < 1.22, kept for consistency)
        t.Run(tc.name, func(t *testing.T) {
            t.Parallel()

            // Act
            got := FunctionUnderTest(tc.input)

            // Assert
            if got != tc.expected {
                t.Fatalf("FunctionUnderTest(%v) = %v, want %v", tc.input, got, tc.expected)
            }
        })
    }
}
```

**Key conventions from the existing pattern:**
- `t.Parallel()` at BOTH top-level and subtest level.
- `tc := tc` loop variable capture (Go 1.22+ does not strictly need this, but the codebase uses Go 1.25+ and the existing test does it -- keep for consistency).
- `t.Fatalf()` for all assertions. Never `t.Errorf()` unless the test should continue after failure.
- Test case struct field names are lowercase (e.g., `name`, `input`, `expected`).
- No helper assertion functions. Direct comparisons in each subtest.
- For error checks: `errors.Is(err, expectedErr)` from stdlib, not string matching.
- For slice/struct comparisons: `reflect.DeepEqual` from stdlib, or field-by-field comparison.

### Test Struct Patterns by Return Type

**Boolean return:**
```go
tests := []struct {
    name     string
    input    <type>
    expected bool
}{...}
```

**String return:**
```go
tests := []struct {
    name     string
    input    <type>
    expected string
}{...}
```

**Multi-return (string, bool):**
```go
tests := []struct {
    name         string
    input        []string
    expectedStr  string
    expectedBool bool
}{...}
```

**Error return:**
```go
tests := []struct {
    name        string
    input       <type>
    wantErr     error  // nil means expect no error
    wantContain string // substring check on Error() output
}{...}
```

### Unit Tests

| Component | Test | Verification |
|-----------|------|-------------|
| `string_handling` | FindInStringSlice nil/empty/present/absent | Returns correct bool |
| `string_handling` | FindInInt64Slice boundary values | Handles math.MaxInt64, math.MinInt64, 0 |
| `string_handling` | IsDuplicateInStringSlice edge cases | nil slice no panic, empty string duplicate detection |
| `errors` | Wrap nil returns nil | `if got != nil { t.Fatalf }` |
| `errors` | Wrap produces WrappedError | Type assertion + field checks |
| `errors` | errors.Is chain traversal | `errors.Is(Wrap(Wrap(base, "a"), "b"), base) == true` |
| `errors` | File path truncation | `strings.Count(we.File, "/") <= 1` |
| `keyword_matcher` | Case-insensitive matching | `FindMatches("HELLO")` with pattern "hello" |
| `keyword_matcher` | Overlapping matches | `FindMatches("ababab")` with pattern "ab" returns 3 |
| `keyword_matcher` | GetPatterns defensive copy | Mutate returned slice, verify internal unchanged |
| `keyword_matcher` | Concurrent access | 10 goroutines, no race |
| `callbackcodec` | EncodeOrFallback success/failure paths | Returns encoded or fallback |
| `callbackcodec` | nil Decoded.Field | Returns ("", false), no panic |
| `callbackcodec` | URL-special characters round-trip | `&`, `=`, `%` values preserved |
| `config/types` | Bool truthy values | "true", "yes", "1", "TRUE", "YES", " true " |
| `config/types` | Bool falsy values | "false", "no", "0", "", "2", "random" |
| `config/types` | Int/Int64 edge cases | "", "not_a_number", overflow, math.MaxInt64 |
| `config/types` | Float64 special values | "NaN", "Inf", "" |
| `config/types` | StringArray whitespace trimming | " a , b , c " -> ["a", "b", "c"] |
| `helpers` | IsChannelID boundary | -1000000000000 -> false, -1000000000001 -> true |
| `helpers` | SplitMessage UTF-8 | 4096 emoji chars -> single element; 4097 -> split |
| `helpers` | HtmlEscape order | `&` escaped first to prevent double-escaping |
| `helpers` | BuildKeyboard SameLine first | First button with SameLine=true gets own row |
| `helpers` | IsExpectedTelegramError all variants | All 17 expected error strings return true |
| `helpers` | notesParser all tags | Each of 6 tags individually and in combination |
| `chat_status` | IsValidUserId/IsChannelId boundaries | 0, -1, -1000000000000, -1000000000001, math.MaxInt64, math.MinInt64 |
| `i18n` | extractLangCode extensions | .yml, .yaml, double-ext, no-ext |
| `i18n` | isYAMLFile case-insensitive | .YML, .YAML -> true |
| `i18n` | validateYAMLStructure non-map root | List root -> error; scalar root -> error |
| `i18n` | I18nError format with/without underlying error | Output matches expected format string |
| `i18n` | Predefined errors distinct | All 6 are not equal via errors.Is |
| `i18n` | extractOrderedValues numbered keys with gap | {"0": "a", "2": "c"} -> ["a"] (breaks at missing "1") |
| `i18n` | selectPluralForm fallback chain | count=0 with Zero="" falls through to Other |
| `modules` | normalizeRulesForHTML HTML passthrough | `<b>Rule</b>` -> returned unchanged |
| `modules` | normalizeRulesForHTML markdown conversion | `*bold*` -> converted via tgmd2html |
| `modules` | normalizeRulesForHTML angle brackets not HTML | `1 < 2 > 0` -> treated as markdown |

### Integration Tests

None. All tests in this design are pure-function unit tests with no cross-component interactions, no DB calls, no API calls, and no cache access.

### Verification Commands

```bash
# Phase 1: Run locally without any env vars
go test -v -race -count=1 ./alita/utils/string_handling/...
go test -v -race -count=1 ./alita/utils/errors/...
go test -v -race -count=1 ./alita/utils/keyword_matcher/...
go test -v -race -count=1 ./alita/utils/callbackcodec/...

# Phase 1: Coverage check
go test -coverprofile=coverage_phase1.out ./alita/utils/string_handling/... ./alita/utils/errors/... ./alita/utils/keyword_matcher/... ./alita/utils/callbackcodec/...
go tool cover -func=coverage_phase1.out

# Phase 2: Run with dummy env vars (CI or locally with env set)
BOT_TOKEN=test-token OWNER_ID=1 MESSAGE_DUMP=1 \
DATABASE_URL="postgres://postgres:postgres@localhost:5432/alita_test?sslmode=disable" \
REDIS_ADDRESS=localhost:6379 \
go test -v -race -count=1 ./alita/config/... ./alita/utils/helpers/... ./alita/utils/chat_status/... ./alita/i18n/... ./alita/modules/...

# Full test suite (same as make test)
make test

# Lint check
make lint

# Specific coverage for callbackcodec (target >95%)
go test -coverprofile=codec.out ./alita/utils/callbackcodec/...
go tool cover -func=codec.out | grep total
```

## Parallelization Analysis

### Independent Streams

These streams have NO shared file modifications and can be implemented simultaneously by different agents:

- **Stream A (string_handling + errors):** Creates 2 new test files in separate packages.
  - `/Users/divkix/GitHub/Alita_Robot/alita/utils/string_handling/string_handling_test.go` (NEW)
  - `/Users/divkix/GitHub/Alita_Robot/alita/utils/errors/errors_test.go` (NEW)

- **Stream B (keyword_matcher):** Creates 1 new test file in its own package.
  - `/Users/divkix/GitHub/Alita_Robot/alita/utils/keyword_matcher/matcher_test.go` (NEW)

- **Stream C (callbackcodec gap-fill):** Modifies 1 existing test file.
  - `/Users/divkix/GitHub/Alita_Robot/alita/utils/callbackcodec/callbackcodec_test.go` (MODIFY)

- **Stream D (config/types):** Creates 1 new test file.
  - `/Users/divkix/GitHub/Alita_Robot/alita/config/types_test.go` (NEW)

- **Stream E (helpers + chat_status):** Creates 2 new test files in separate packages.
  - `/Users/divkix/GitHub/Alita_Robot/alita/utils/helpers/helpers_test.go` (NEW)
  - `/Users/divkix/GitHub/Alita_Robot/alita/utils/chat_status/chat_status_test.go` (NEW)

- **Stream F (i18n + rules_format):** Creates 2 new test files in separate packages.
  - `/Users/divkix/GitHub/Alita_Robot/alita/i18n/i18n_test.go` (NEW)
  - `/Users/divkix/GitHub/Alita_Robot/alita/modules/rules_format_test.go` (NEW)

### Sequential Dependencies

- **Phase 2 cannot be verified locally before Phase 1 is complete.** Phase 1 tests serve as the validation that the test patterns work. Phase 2 tests reuse the same patterns but require CI env vars. However, both phases can be WRITTEN in parallel -- the dependency is only on verification.
- **No code-level sequential dependencies exist between any streams.** Every stream creates/modifies files in different directories with no shared state.

### Shared Resources (Serialization Points)

- **Stream C** modifies an existing file (`callbackcodec_test.go`). Only one agent should touch this file.
- All other streams create NEW files, so there are no conflicts.

## File Inventory (All Files Modified or Created)

| File | Action | Phase | Stream |
|------|--------|-------|--------|
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/string_handling/string_handling_test.go` | CREATE | 1 | A |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/errors/errors_test.go` | CREATE | 1 | A |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/keyword_matcher/matcher_test.go` | CREATE | 1 | B |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/callbackcodec/callbackcodec_test.go` | MODIFY | 1 | C |
| `/Users/divkix/GitHub/Alita_Robot/alita/config/types_test.go` | CREATE | 2 | D |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/helpers/helpers_test.go` | CREATE | 2 | E |
| `/Users/divkix/GitHub/Alita_Robot/alita/utils/chat_status/chat_status_test.go` | CREATE | 2 | E |
| `/Users/divkix/GitHub/Alita_Robot/alita/i18n/i18n_test.go` | CREATE | 2 | F |
| `/Users/divkix/GitHub/Alita_Robot/alita/modules/rules_format_test.go` | CREATE | 2 | F |

**Total: 9 files (8 new, 1 modified). Zero production code changes.**

## Design Decisions

### Decision: Package-internal tests (`package X`) instead of external tests (`package X_test`)

- **Context:** Several functions are unexported: `typeConvertor` (config), `notesParser` (helpers), `extractLangCode`/`isYAMLFile`/`validateYAMLStructure`/`extractOrderedValues`/`selectPluralForm` (i18n), `normalizeRulesForHTML` (modules).
- **Options considered:** (a) Internal tests in same package, (b) External tests with exported wrappers, (c) Export the functions.
- **Chosen:** (a) Internal tests, because the existing test (`callbackcodec_test.go`) uses `package callbackcodec` (internal), and exporting functions or adding wrappers just for testing violates YAGNI. Consistency with existing pattern wins.
- **Trade-offs:** Test code has access to all package internals, which could lead to fragile tests coupled to implementation details. Mitigated by only testing public-contract behavior (inputs/outputs), not internal state.

### Decision: No test helpers or shared test utilities package

- **Context:** Multiple test files share similar table-driven patterns.
- **Options considered:** (a) Create `alita/utils/testhelpers/` with assertion helpers, (b) Duplicate test patterns in each file.
- **Chosen:** (b) Duplicate patterns, because the existing codebase has zero shared test utilities, adding one would be a convention change, and the duplication is trivially simple (`t.Fatalf` calls). YAGNI.
- **Trade-offs:** Minor code duplication across test files. Acceptable for the volume of tests being added.

### Decision: Skip `ChunkKeyboardSlices(slice, 0)` test case

- **Context:** `chunkSize=0` causes an infinite loop in the current implementation.
- **Options considered:** (a) Add a guard in production code and test it, (b) Document as known defect and skip, (c) Test and accept the infinite loop.
- **Chosen:** (b) Document as known defect in test file with a comment. Fixing production code is out of scope for "add tests" and introducing a code change muddies the PR.
- **Trade-offs:** Known defect remains. Mitigated by documenting in test file.

### Decision: Construct minimal `Translator` struct directly for `selectPluralForm` tests

- **Context:** `selectPluralForm` is a method on `*Translator`. The `Translator` struct has unexported fields. Creating one via `GetManager().GetTranslator()` requires the full i18n singleton lifecycle (embedded FS, locale loading).
- **Options considered:** (a) Use `GetManager().Initialize()` with test embedded FS, (b) Construct `Translator` directly using struct literal in internal package test, (c) Extract `selectPluralForm` into a standalone function.
- **Chosen:** (b) Direct struct construction. Since the test is in `package i18n`, it can access unexported fields. `selectPluralForm` only uses `self` to exist as a method -- it does not reference `t.langCode`, `t.manager`, or `t.viper`. A minimal `Translator{langCode: "en", manager: &LocaleManager{defaultLang: "en"}}` suffices.
- **Trade-offs:** Tightly coupled to internal struct layout. Acceptable because the fields are stable and the test is in the same package.

### Decision: `interpolateParams` tested indirectly through `extractOrderedValues` and `selectPluralForm`

- **Context:** `interpolateParams` is a method on `*Translator` that requires a `viper` instance for the regex replacement logic and a `manager` for logging. Testing it directly requires a fully initialized `Translator`.
- **Options considered:** (a) Test `interpolateParams` directly with a full `Translator`, (b) Test only the helper functions it calls (`extractOrderedValues`), (c) Create a test-only initialization path.
- **Chosen:** (b) Test `extractOrderedValues` directly (it is a package-level function) and `selectPluralForm` directly (minimal struct needed). `interpolateParams` gets indirect coverage through the tested helpers. Full `interpolateParams` testing would require an initialized viper instance with locale data, which is Phase 3 complexity.
- **Trade-offs:** `interpolateParams` itself has lower direct coverage. The regex replacement logic (`paramRegex.ReplaceAllStringFunc`) is not directly tested. Acceptable because the helper functions contain the complex logic.

## Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Phase 2 tests fail in CI due to DB connection error during `init()` | MEDIUM | Phase 2 tests cannot run even with env vars. The `db` package `init()` tries to connect to PostgreSQL. If CI PostgreSQL is not ready or connection fails, the process may crash. | CI already provides PostgreSQL 16 service container. Existing DB tests pass in CI. The connection failure would affect existing tests too, so it is a pre-existing condition, not a new risk. |
| `config/init()` behavior changes break Phase 2 tests | LOW | Tests may need env var updates. | Env vars are documented in both `sample.env` and CI config. If `init()` is refactored to lazy-init, Phase 2 tests become Phase 1 (improvement, not breakage). |
| Aho-Corasick `Match()` return value semantics change | LOW | `keyword_matcher` tests produce wrong expected values. | Pin to current library version. Tests verify behavior, not implementation. |
| `tgmd2html.MD2HTMLV2` output format changes | MEDIUM | `normalizeRulesForHTML` test expected values become stale. | Use the actual function output as expected value (compute in test setup), not hardcoded strings. Alternatively, test only the branching logic (HTML detected vs. not) rather than exact output strings. |
| Test files introduce lint warnings | LOW | `make lint` fails. | Run `make lint` as part of verification for each test file before committing. |
| Singleton `i18n.GetManager()` interference between test functions | LOW | Tests in `i18n` package share singleton state. | The singleton is initialized via `sync.Once`. Tests that construct `Translator` directly bypass the singleton. Tests that test `extractLangCode`, `isYAMLFile`, `validateYAMLStructure` do not touch the singleton at all. |

DESIGN_COMPLETE
