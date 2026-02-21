# Requirements: Increase Test Coverage on Critical Codebase Paths

**Date:** 2026-02-21
**Goal:** Increase unit test coverage on the most important pure-function paths of the Alita_Robot Go codebase, targeting packages that can be tested without infrastructure dependencies first, then packages testable with dummy environment variables.
**Source:** research.md

## Current State

- ~35,000 lines of Go code, ~1,184 lines of tests (3.4% ratio)
- 22 of ~30 packages have zero tests
- Only 1 package (`callbackcodec` at 79.1%) passes tests locally without env vars
- 44 pure functions identified as testable without infrastructure
- Config `init()` in `alita/config/config.go` calls `log.Fatalf` when env vars are missing, blocking any package that imports `config` transitively

## Scope

### In Scope

- Phase 1: Unit tests for pure-function packages with zero infrastructure dependencies (5 packages, ~20 functions)
- Phase 2: Unit tests for pure functions in packages that require dummy env vars to import (4 packages, ~24 functions)
- Gap-filling on existing `callbackcodec` tests (increase from 79.1%)
- All tests SHALL follow the existing convention: stdlib `testing`, table-driven subtests, `t.Parallel()`, `t.Fatalf()`
- All tests SHALL pass both locally (Phase 1) and in CI (Phase 1 + Phase 2)
- All tests SHALL pass `make lint` and race detection (`-race` flag)

### Out of Scope

- **Phase 3 (DB integration tests):** Tests requiring PostgreSQL. These are deferred because they require CI infrastructure and represent a separate body of work with different patterns (GORM setup, `t.Cleanup`, `time.Now().UnixNano()` IDs). Covered by existing tests in `captcha_db_test.go`, `locks_db_test.go`, `antiflood_db_test.go`, `update_record_test.go`.
- **Config `init()` refactoring:** Changing `log.Fatalf` to lazy initialization would unblock ~70% of the codebase for local testing but is an architectural change outside the scope of "add tests." Documented as a dependency for future work.
- **Mock interfaces for `gotgbot.Bot`:** The library does not provide interfaces. Adding them is a significant refactor. Handler-level tests that require bot interaction are out of scope.
- **Test frameworks or assertion libraries:** No testify, gomock, or other test dependencies SHALL be added. All tests use stdlib `testing` only.
- **HTTP server tests:** `alita/utils/httpserver` testing with `httptest.NewServer` is deferred.
- **Monitoring/background goroutine tests:** `alita/utils/monitoring` requires mocked clocks and is deferred.
- **Coverage percentage targets:** No specific percentage target is mandated. The goal is to cover the 44 identified pure functions.

## User Stories

### US-001: Test `alita/utils/string_handling` Package

**Priority:** P0 (must-have)

As a developer,
I want unit tests for all 3 functions in `string_handling`,
So that regressions in slice search logic are caught before they reach production.

**Target file:** `/Users/divkix/GitHub/Alita_Robot/alita/utils/string_handling/string_handling_test.go`

**Functions under test:**
1. `FindInStringSlice(slice []string, val string) bool`
2. `FindInInt64Slice(slice []int64, val int64) bool`
3. `IsDuplicateInStringSlice(arr []string) (string, bool)`

**Acceptance Criteria:**
- [ ] GIVEN a string slice containing "hello" WHEN `FindInStringSlice` is called with "hello" THEN it returns `true`
- [ ] GIVEN an empty string slice WHEN `FindInStringSlice` is called with any value THEN it returns `false`
- [ ] GIVEN a string slice WHEN `FindInStringSlice` is called with a value not in the slice THEN it returns `false`
- [ ] GIVEN an int64 slice containing 42 WHEN `FindInInt64Slice` is called with 42 THEN it returns `true`
- [ ] GIVEN an empty int64 slice WHEN `FindInInt64Slice` is called with any value THEN it returns `false`
- [ ] GIVEN a slice with negative int64 values (e.g., channel IDs like -1001234567890) WHEN `FindInInt64Slice` is called with that value THEN it returns `true`
- [ ] GIVEN an int64 slice WHEN `FindInInt64Slice` is called with 0 THEN it correctly reports presence/absence
- [ ] GIVEN a slice `["a", "b", "a"]` WHEN `IsDuplicateInStringSlice` is called THEN it returns `("a", true)`
- [ ] GIVEN a slice `["a", "b", "c"]` WHEN `IsDuplicateInStringSlice` is called THEN it returns `("", false)`
- [ ] GIVEN an empty slice WHEN `IsDuplicateInStringSlice` is called THEN it returns `("", false)`
- [ ] GIVEN a nil slice WHEN `IsDuplicateInStringSlice` is called THEN it returns `("", false)` without panic

**Edge Cases:**
- [ ] `FindInStringSlice` with nil slice -> returns `false`, no panic
- [ ] `FindInStringSlice` with empty string as search value -> correct match behavior
- [ ] `FindInInt64Slice` with `math.MaxInt64` and `math.MinInt64` -> correct boundary behavior
- [ ] `IsDuplicateInStringSlice` with single-element slice -> `("", false)`
- [ ] `IsDuplicateInStringSlice` with all identical elements -> returns first duplicate found
- [ ] `IsDuplicateInStringSlice` with empty strings as elements -> detects `""` duplicates

**Definition of Done:**
- [ ] Test file compiles and passes with `go test ./alita/utils/string_handling/...`
- [ ] All tests use `t.Parallel()` at both the top-level and subtest level
- [ ] Table-driven subtests with descriptive names
- [ ] `make lint` passes with no new warnings from the test file
- [ ] Race detector passes (`-race` flag, already in `make test`)

---

### US-002: Test `alita/utils/errors` Package

**Priority:** P0 (must-have)

As a developer,
I want unit tests for the custom error wrapping in `alita/utils/errors`,
So that error chains with file/line metadata work correctly across the codebase.

**Target file:** `/Users/divkix/GitHub/Alita_Robot/alita/utils/errors/errors_test.go`

**Functions under test:**
1. `Wrap(err error, message string) error`
2. `Wrapf(err error, format string, args ...any) error`
3. `WrappedError.Error() string`
4. `WrappedError.Unwrap() error`

**Acceptance Criteria:**
- [ ] GIVEN a nil error WHEN `Wrap` is called THEN it returns nil
- [ ] GIVEN a non-nil error and message "failed" WHEN `Wrap` is called THEN the returned error's `.Error()` string contains "failed"
- [ ] GIVEN a wrapped error WHEN `.Error()` is called THEN the output contains file path, line number, function name, message, and original error
- [ ] GIVEN a wrapped error WHEN `errors.Unwrap()` is called THEN it returns the original error
- [ ] GIVEN a wrapped error WHEN `errors.Is(wrappedErr, originalErr)` is called THEN it returns true (chain traversal)
- [ ] GIVEN a nil error WHEN `Wrapf` is called with format args THEN it returns nil
- [ ] GIVEN a non-nil error WHEN `Wrapf` is called with "op %s failed for id %d", "save", 42 THEN `.Error()` contains "op save failed for id 42"

**Edge Cases:**
- [ ] `Wrap` with empty message string -> still produces valid error output with file/line info
- [ ] `Wrapf` with zero format args -> behaves like `Wrap`
- [ ] Double-wrapping: `Wrap(Wrap(err, "inner"), "outer")` -> both messages accessible, `errors.Is` still works on innermost error
- [ ] `WrappedError.Error()` output format is deterministic (contains "at" keyword, colon separators)
- [ ] File path truncation: the `File` field contains at most 2 path segments (verified by checking output does not contain full absolute path)

**Definition of Done:**
- [ ] Test file compiles and passes with `go test ./alita/utils/errors/...`
- [ ] All tests use `t.Parallel()`
- [ ] Tests verify error chain behavior using `errors.Is` and `errors.Unwrap` from stdlib
- [ ] `make lint` passes

---

### US-003: Test `alita/config` `typeConvertor` Methods

**Priority:** P0 (must-have)

As a developer,
I want unit tests for all 5 `typeConvertor` methods in `alita/config/types.go`,
So that environment variable parsing does not silently produce wrong values.

**Target file:** `/Users/divkix/GitHub/Alita_Robot/alita/config/types_test.go`

**CRITICAL NOTE:** This test file MUST be placed in `alita/config/` but SHALL NOT trigger the `init()` function in `config.go`. Since `types.go` is in the `config` package, the test file will be in the same package and `init()` WILL run on import. Therefore, either:
- (a) The test file uses `package config` and the CI env vars handle it, OR
- (b) The test file uses `package config_test` (external test package) which still triggers init on import.

**Resolution:** Since `init()` runs regardless when testing the `config` package, this test SHALL require the same dummy env vars as CI: `BOT_TOKEN=test-token OWNER_ID=1 MESSAGE_DUMP=1 DATABASE_URL=... REDIS_ADDRESS=...`. This makes it a Phase 2 test that passes in CI but not locally without env vars.

**ALTERNATIVE:** If the `typeConvertor` type and its methods can be tested WITHOUT importing the `config` package (e.g., by duplicating the struct in a `_test.go` file), that approach SHOULD be used to keep it Phase 1. However, since `typeConvertor` is unexported, the test MUST be in the `config` package.

**Functions under test:**
1. `typeConvertor.Bool() bool`
2. `typeConvertor.Int() int`
3. `typeConvertor.Int64() int64`
4. `typeConvertor.Float64() float64`
5. `typeConvertor.StringArray() []string`

**Acceptance Criteria:**
- [ ] GIVEN `typeConvertor{str: "true"}` WHEN `Bool()` is called THEN it returns `true`
- [ ] GIVEN `typeConvertor{str: "yes"}` WHEN `Bool()` is called THEN it returns `true`
- [ ] GIVEN `typeConvertor{str: "1"}` WHEN `Bool()` is called THEN it returns `true`
- [ ] GIVEN `typeConvertor{str: "TRUE"}` WHEN `Bool()` is called THEN it returns `true` (case-insensitive)
- [ ] GIVEN `typeConvertor{str: "YES"}` WHEN `Bool()` is called THEN it returns `true` (case-insensitive)
- [ ] GIVEN `typeConvertor{str: "false"}` WHEN `Bool()` is called THEN it returns `false`
- [ ] GIVEN `typeConvertor{str: "no"}` WHEN `Bool()` is called THEN it returns `false`
- [ ] GIVEN `typeConvertor{str: "0"}` WHEN `Bool()` is called THEN it returns `false`
- [ ] GIVEN `typeConvertor{str: ""}` WHEN `Bool()` is called THEN it returns `false`
- [ ] GIVEN `typeConvertor{str: " true "}` WHEN `Bool()` is called THEN it returns `true` (whitespace trimmed)
- [ ] GIVEN `typeConvertor{str: "42"}` WHEN `Int()` is called THEN it returns `42`
- [ ] GIVEN `typeConvertor{str: ""}` WHEN `Int()` is called THEN it returns `0`
- [ ] GIVEN `typeConvertor{str: "not_a_number"}` WHEN `Int()` is called THEN it returns `0`
- [ ] GIVEN `typeConvertor{str: "-100"}` WHEN `Int()` is called THEN it returns `-100`
- [ ] GIVEN `typeConvertor{str: "9223372036854775807"}` WHEN `Int64()` is called THEN it returns `math.MaxInt64`
- [ ] GIVEN `typeConvertor{str: ""}` WHEN `Int64()` is called THEN it returns `0`
- [ ] GIVEN `typeConvertor{str: "3.14"}` WHEN `Float64()` is called THEN it returns `3.14`
- [ ] GIVEN `typeConvertor{str: ""}` WHEN `Float64()` is called THEN it returns `0.0`
- [ ] GIVEN `typeConvertor{str: "a,b,c"}` WHEN `StringArray()` is called THEN it returns `["a", "b", "c"]`
- [ ] GIVEN `typeConvertor{str: " a , b , c "}` WHEN `StringArray()` is called THEN it returns `["a", "b", "c"]` (trimmed)
- [ ] GIVEN `typeConvertor{str: "single"}` WHEN `StringArray()` is called THEN it returns `["single"]`
- [ ] GIVEN `typeConvertor{str: ""}` WHEN `StringArray()` is called THEN it returns `[""]` (one empty element -- this is the actual behavior of `strings.Split("", ",")`)

**Edge Cases:**
- [ ] `Bool()` with "2" -> `false` (only "1" is truthy)
- [ ] `Int()` with "9999999999999999999" (overflow) -> returns 0 (strconv.Atoi fails)
- [ ] `Int64()` with leading/trailing whitespace -> returns 0 (strconv.ParseInt does not trim)
- [ ] `Float64()` with "NaN" -> returns `NaN` (strconv.ParseFloat parses it)
- [ ] `Float64()` with "Inf" -> returns `+Inf`
- [ ] `StringArray()` with ",," (consecutive commas) -> returns `["", "", ""]`

**Definition of Done:**
- [ ] Test file compiles and passes in CI with dummy env vars set
- [ ] All tests use `t.Parallel()` and table-driven subtests
- [ ] `make lint` passes

---

### US-004: Test `alita/utils/keyword_matcher` Package

**Priority:** P0 (must-have)

As a developer,
I want unit tests for the Aho-Corasick keyword matcher,
So that blacklist and filter pattern matching is reliable under all input conditions.

**Target file:** `/Users/divkix/GitHub/Alita_Robot/alita/utils/keyword_matcher/matcher_test.go`

**Functions under test:**
1. `NewKeywordMatcher(patterns []string) *KeywordMatcher`
2. `KeywordMatcher.FindMatches(text string) []MatchResult`
3. `KeywordMatcher.HasMatch(text string) bool`
4. `KeywordMatcher.GetPatterns() []string`

**Acceptance Criteria:**
- [ ] GIVEN patterns `["hello", "world"]` WHEN `FindMatches("hello world")` is called THEN it returns 2 results, one for "hello" and one for "world"
- [ ] GIVEN patterns `["hello"]` WHEN `FindMatches("HELLO")` is called THEN it returns 1 result (case-insensitive)
- [ ] GIVEN patterns `["hello"]` WHEN `HasMatch("say hello there")` is called THEN it returns `true`
- [ ] GIVEN patterns `["hello"]` WHEN `HasMatch("goodbye")` is called THEN it returns `false`
- [ ] GIVEN patterns `["abc"]` WHEN `GetPatterns()` is called THEN it returns `["abc"]` (copy, not reference)
- [ ] GIVEN empty patterns `[]` WHEN `FindMatches("anything")` is called THEN it returns nil
- [ ] GIVEN empty patterns `[]` WHEN `HasMatch("anything")` is called THEN it returns `false`
- [ ] GIVEN patterns `["ab"]` WHEN `FindMatches("ababab")` is called THEN it returns all 3 overlapping occurrences with correct Start/End positions
- [ ] GIVEN patterns returned by `GetPatterns()` WHEN the returned slice is mutated THEN the internal patterns remain unchanged (defensive copy)

**Edge Cases:**
- [ ] `NewKeywordMatcher(nil)` -> creates matcher with no patterns, `HasMatch` returns `false`
- [ ] `FindMatches("")` -> returns nil (empty text)
- [ ] `FindMatches` with text containing only whitespace -> no false positives unless whitespace is a pattern
- [ ] `HasMatch` with Unicode text (e.g., "hello") and ASCII pattern "hello" -> correct behavior
- [ ] Patterns with special regex characters (e.g., `"foo.bar"`) -> matched literally, not as regex
- [ ] Single-character patterns -> matched correctly
- [ ] Pattern that is the entire text -> matched with correct Start=0 and End=len(text)
- [ ] Concurrent `FindMatches` and `HasMatch` calls from multiple goroutines -> no race condition (verify via `-race` flag)

**Definition of Done:**
- [ ] Test file compiles and passes with `go test ./alita/utils/keyword_matcher/...`
- [ ] All tests use `t.Parallel()`
- [ ] Concurrent access test uses `sync.WaitGroup` with at least 10 goroutines
- [ ] Race detector passes
- [ ] `make lint` passes

---

### US-005: Fill Coverage Gaps in `alita/utils/callbackcodec`

**Priority:** P0 (must-have)

As a developer,
I want to increase `callbackcodec` coverage from 79.1% to >95%,
So that the callback data encoding used across all module handlers is bulletproof.

**Target file:** `/Users/divkix/GitHub/Alita_Robot/alita/utils/callbackcodec/callbackcodec_test.go` (append to existing)

**Functions needing additional coverage:**
1. `EncodeOrFallback(namespace string, fields map[string]string, fallback string) string`
2. `Decoded.Field(key string) (string, bool)` -- nil receiver path
3. `Encode` -- empty fields map, fields with empty key (skipped), underscore placeholder

**Acceptance Criteria:**
- [ ] GIVEN valid namespace and fields WHEN `EncodeOrFallback` is called THEN it returns the encoded string (same as `Encode`)
- [ ] GIVEN an invalid namespace (empty string) WHEN `EncodeOrFallback` is called with fallback "fallback_data" THEN it returns "fallback_data"
- [ ] GIVEN an oversized payload WHEN `EncodeOrFallback` is called with fallback "fb" THEN it returns "fb"
- [ ] GIVEN a nil `*Decoded` receiver WHEN `Field("any")` is called THEN it returns `("", false)` without panic
- [ ] GIVEN a valid `Decoded` with fields `{"a": "yes"}` WHEN `Field("a")` is called THEN it returns `("yes", true)`
- [ ] GIVEN a valid `Decoded` with fields `{"a": "yes"}` WHEN `Field("missing")` is called THEN it returns `("", false)`
- [ ] GIVEN `Encode("ns", map[string]string{})` (empty fields map) WHEN called THEN it produces a valid encoded string with `_` as the payload placeholder
- [ ] GIVEN `Encode("ns", map[string]string{"": "val"})` (empty key) WHEN called THEN the empty key is skipped, payload is `_`
- [ ] GIVEN `Decode` of a string with `_` payload WHEN called THEN `Fields` map is empty (not containing `_`)
- [ ] GIVEN `Encode("ns", nil)` (nil fields map) WHEN called THEN it produces a valid encoded string with `_` placeholder

**Edge Cases:**
- [ ] `EncodeOrFallback` with empty fallback string -> returns "" on failure
- [ ] `Field` with empty string key -> returns `("", false)` (key not in map)
- [ ] Round-trip: `Encode` then `Decode` with URL-special characters in values (e.g., `&`, `=`, `%`) -> values preserved correctly
- [ ] `Encode` with namespace at maximum allowed length (filling up to 64 bytes) -> succeeds or fails with `ErrDataTooLong` deterministically

**Definition of Done:**
- [ ] Tests pass with `go test ./alita/utils/callbackcodec/... -cover` showing >95% coverage
- [ ] New tests appended to existing test file, maintaining existing style
- [ ] All new tests use `t.Parallel()`
- [ ] `make lint` passes

---

### US-006: Test `alita/utils/helpers` Pure Functions

**Priority:** P1 (should-have)

As a developer,
I want unit tests for the pure helper functions in `alita/utils/helpers`,
So that keyboard building, HTML escaping, message splitting, and error classification logic is verified.

**Target file:** `/Users/divkix/GitHub/Alita_Robot/alita/utils/helpers/helpers_test.go`

**CRITICAL NOTE:** This package imports `alita/config`, `alita/db`, and `alita/i18n`. The `config` import triggers `init()` which requires env vars. These tests SHALL only pass in CI with dummy env vars set. They are Phase 2 tests.

**Functions under test:**
1. `IsChannelID(chatID int64) bool` (in `channel_helpers.go`)
2. `SplitMessage(msg string) []string`
3. `MentionHtml(userId int64, name string) string`
4. `MentionUrl(url, name string) string`
5. `HtmlEscape(s string) string`
6. `GetFullName(FirstName, LastName string) string`
7. `BuildKeyboard(buttons []db.Button) [][]gotgbot.InlineKeyboardButton`
8. `ConvertButtonV2ToDbButton(buttons []tgmd2html.ButtonV2) []db.Button`
9. `RevertButtons(buttons []db.Button) string`
10. `ChunkKeyboardSlices(slice []gotgbot.InlineKeyboardButton, chunkSize int) [][]gotgbot.InlineKeyboardButton`
11. `ReverseHTML2MD(text string) string`
12. `IsExpectedTelegramError(err error) bool` (in `telegram_helpers.go`)
13. `notesParser(sent string) (pvtOnly, grpOnly, adminOnly, webPrev, protectedContent, noNotif bool, sentBack string)`

**Acceptance Criteria:**
- [ ] GIVEN chatID = -1001234567890 WHEN `IsChannelID` is called THEN it returns `true`
- [ ] GIVEN chatID = -123456789 (regular group) WHEN `IsChannelID` is called THEN it returns `false`
- [ ] GIVEN chatID = 123456789 (user) WHEN `IsChannelID` is called THEN it returns `false`
- [ ] GIVEN chatID = 0 WHEN `IsChannelID` is called THEN it returns `false`
- [ ] GIVEN chatID = -1000000000000 (boundary) WHEN `IsChannelID` is called THEN it returns `false` (strictly less than)
- [ ] GIVEN chatID = -1000000000001 WHEN `IsChannelID` is called THEN it returns `true`
- [ ] GIVEN a message of 4096 runes WHEN `SplitMessage` is called THEN it returns a single-element slice
- [ ] GIVEN a message of 4097 runes WHEN `SplitMessage` is called THEN it returns a multi-element slice
- [ ] GIVEN a message with multi-byte UTF-8 characters (e.g., emoji) WHEN `SplitMessage` is called THEN it splits by rune count, not byte count
- [ ] GIVEN an empty string WHEN `SplitMessage` is called THEN it returns `[""]`
- [ ] GIVEN a single line longer than 4096 runes WHEN `SplitMessage` is called THEN it splits the line into 4096-rune chunks
- [ ] GIVEN userId=123, name="Test" WHEN `MentionHtml` is called THEN it returns `<a href="tg://user?id=123">Test</a>`
- [ ] GIVEN name with HTML special chars (e.g., `<script>`) WHEN `MentionHtml` is called THEN the name is HTML-escaped
- [ ] GIVEN `s = "<b>bold&amp;</b>"` WHEN `HtmlEscape` is called THEN `&` is escaped first, then `<` and `>` (order matters to avoid double-escaping)
- [ ] GIVEN FirstName="John", LastName="Doe" WHEN `GetFullName` is called THEN it returns "John Doe"
- [ ] GIVEN FirstName="John", LastName="" WHEN `GetFullName` is called THEN it returns "John"
- [ ] GIVEN an empty buttons slice WHEN `BuildKeyboard` is called THEN it returns an empty 2D slice
- [ ] GIVEN buttons where second button has SameLine=true WHEN `BuildKeyboard` is called THEN both buttons appear in the same row
- [ ] GIVEN buttons where all have SameLine=false WHEN `BuildKeyboard` is called THEN each button gets its own row
- [ ] GIVEN a non-nil error containing "bot was kicked from the" WHEN `IsExpectedTelegramError` is called THEN it returns `true`
- [ ] GIVEN a nil error WHEN `IsExpectedTelegramError` is called THEN it returns `false`
- [ ] GIVEN an error "unknown error xyz" WHEN `IsExpectedTelegramError` is called THEN it returns `false`
- [ ] GIVEN text `"hello {private} world {admin}"` WHEN `notesParser` is called THEN pvtOnly=true, adminOnly=true, sentBack has tags removed
- [ ] GIVEN text with no tags WHEN `notesParser` is called THEN all booleans are false and sentBack equals input

**Edge Cases:**
- [ ] `SplitMessage` with only newlines -> splits correctly without empty last element duplication
- [ ] `BuildKeyboard` with first button having SameLine=true -> button gets its own row (no previous row to append to, len(keyb)==0 check)
- [ ] `ChunkKeyboardSlices` with chunkSize=0 -> SHALL NOT infinite loop (verify behavior or document panic)
- [ ] `ChunkKeyboardSlices` with chunkSize > len(slice) -> returns single chunk with all elements
- [ ] `ChunkKeyboardSlices` with empty slice -> returns nil
- [ ] `ReverseHTML2MD` with nested HTML tags -> handles correctly or degrades gracefully
- [ ] `ReverseHTML2MD` with no HTML tags -> returns input unchanged
- [ ] `RevertButtons` with empty slice -> returns ""
- [ ] `ConvertButtonV2ToDbButton` with nil slice -> returns empty slice (not nil, since `make([]db.Button, 0)`)
- [ ] `IsExpectedTelegramError` tests SHALL cover ALL 13 error strings in the `expectedErrors` slice
- [ ] `notesParser` with `{protect}` tag -> protectedContent=true
- [ ] `notesParser` with `{nonotif}` tag -> noNotif=true
- [ ] `notesParser` with `{noprivate}` tag -> grpOnly=true
- [ ] `notesParser` with `{preview}` tag -> webPrev=true
- [ ] `HtmlEscape` with empty string -> returns ""
- [ ] `MentionUrl` with URL containing quotes -> no injection (quotes not in URL context)

**Definition of Done:**
- [ ] Test file compiles and passes in CI with dummy env vars
- [ ] All tests use `t.Parallel()` and table-driven subtests
- [ ] `make lint` passes
- [ ] `notesParser` tests verify all 6 tag types independently and in combination

---

### US-007: Test `alita/utils/chat_status` Pure Functions

**Priority:** P1 (should-have)

As a developer,
I want unit tests for the 2 pure ID validation functions in `chat_status`,
So that the permission system correctly distinguishes users from channels.

**Target file:** `/Users/divkix/GitHub/Alita_Robot/alita/utils/chat_status/chat_status_test.go`

**CRITICAL NOTE:** Phase 2 -- requires dummy env vars because `chat_status` imports `db` which imports `config`.

**Functions under test:**
1. `IsValidUserId(id int64) bool`
2. `IsChannelId(id int64) bool`

**Acceptance Criteria:**
- [ ] GIVEN id = 123456789 WHEN `IsValidUserId` is called THEN it returns `true`
- [ ] GIVEN id = 0 WHEN `IsValidUserId` is called THEN it returns `false`
- [ ] GIVEN id = -1 WHEN `IsValidUserId` is called THEN it returns `false`
- [ ] GIVEN id = -1001234567890 WHEN `IsValidUserId` is called THEN it returns `false`
- [ ] GIVEN id = 1 WHEN `IsValidUserId` is called THEN it returns `true` (minimum valid user ID)
- [ ] GIVEN id = -1001234567890 WHEN `IsChannelId` is called THEN it returns `true`
- [ ] GIVEN id = -1000000000001 WHEN `IsChannelId` is called THEN it returns `true` (boundary)
- [ ] GIVEN id = -1000000000000 WHEN `IsChannelId` is called THEN it returns `false` (boundary, strictly less than)
- [ ] GIVEN id = 123456789 WHEN `IsChannelId` is called THEN it returns `false`
- [ ] GIVEN id = 0 WHEN `IsChannelId` is called THEN it returns `false`

**Edge Cases:**
- [ ] `math.MaxInt64` -> `IsValidUserId` returns true, `IsChannelId` returns false
- [ ] `math.MinInt64` -> `IsValidUserId` returns false, `IsChannelId` returns true
- [ ] The known Telegram system IDs (1087968824 = Group Anonymous Bot, 777000 = Telegram) -> `IsValidUserId` returns true for both (they are positive)

**Definition of Done:**
- [ ] Test file compiles and passes in CI with dummy env vars
- [ ] All tests use `t.Parallel()`
- [ ] `make lint` passes

---

### US-008: Test `alita/i18n` Pure Functions

**Priority:** P1 (should-have)

As a developer,
I want unit tests for the pure utility functions in the i18n package,
So that locale loading, YAML validation, parameter interpolation, and error formatting are verified.

**Target file:** `/Users/divkix/GitHub/Alita_Robot/alita/i18n/i18n_test.go`

**CRITICAL NOTE:** Phase 2 -- the `i18n` package imports `alita/utils/cache` which may import `config` transitively. Verify import chain. If cache import is only used for the `cacheClient` field (set at runtime, not init), the package MAY be testable without env vars. The pure functions (`extractLangCode`, `isYAMLFile`, `validateYAMLStructure`, `I18nError.Error()`) do NOT use cache or config.

**Functions under test:**
1. `extractLangCode(fileName string) string`
2. `isYAMLFile(fileName string) bool`
3. `validateYAMLStructure(content []byte) error`
4. `I18nError.Error() string`
5. `I18nError.Unwrap() error`
6. `NewI18nError(op, lang, key, message string, err error) *I18nError`
7. `extractOrderedValues(params TranslationParams) []any`
8. `selectPluralForm(rule PluralRule, count int) string` (method on `Translator`, but logic is pure)

**Acceptance Criteria:**
- [ ] GIVEN fileName = "en.yml" WHEN `extractLangCode` is called THEN it returns "en"
- [ ] GIVEN fileName = "en.yaml" WHEN `extractLangCode` is called THEN it returns "en"
- [ ] GIVEN fileName = "pt-BR.yml" WHEN `extractLangCode` is called THEN it returns "pt-BR"
- [ ] GIVEN fileName = "en.yml" WHEN `isYAMLFile` is called THEN it returns `true`
- [ ] GIVEN fileName = "en.yaml" WHEN `isYAMLFile` is called THEN it returns `true`
- [ ] GIVEN fileName = "en.json" WHEN `isYAMLFile` is called THEN it returns `false`
- [ ] GIVEN fileName = "en.YML" (uppercase) WHEN `isYAMLFile` is called THEN it returns `true` (case-insensitive via `strings.ToLower`)
- [ ] GIVEN fileName = "" WHEN `isYAMLFile` is called THEN it returns `false`
- [ ] GIVEN valid YAML `key: value` WHEN `validateYAMLStructure` is called THEN it returns nil
- [ ] GIVEN invalid YAML `{{{` WHEN `validateYAMLStructure` is called THEN it returns an error
- [ ] GIVEN YAML that parses to a list (not a map) WHEN `validateYAMLStructure` is called THEN it returns an error (root must be map)
- [ ] GIVEN empty YAML `""` WHEN `validateYAMLStructure` is called THEN it returns an error (nil is not a map)
- [ ] GIVEN I18nError with op="get", lang="en", key="hello", message="not found", err=nil WHEN `.Error()` is called THEN output format is `"i18n get failed for lang=en key=hello: not found"`
- [ ] GIVEN I18nError with a non-nil Err WHEN `.Error()` is called THEN output includes the underlying error
- [ ] GIVEN I18nError with a non-nil Err WHEN `.Unwrap()` is called THEN it returns the underlying error
- [ ] GIVEN I18nError with nil Err WHEN `.Unwrap()` is called THEN it returns nil
- [ ] GIVEN params `{"0": "a", "1": "b", "2": "c"}` WHEN `extractOrderedValues` is called THEN it returns `["a", "b", "c"]`
- [ ] GIVEN params `{"first": "x", "second": "y"}` WHEN `extractOrderedValues` is called THEN it returns `["x", "y"]` (common key order)
- [ ] GIVEN nil params WHEN `extractOrderedValues` is called THEN it returns nil
- [ ] GIVEN empty params `{}` WHEN `extractOrderedValues` is called THEN it returns nil (or empty slice)

**Edge Cases:**
- [ ] `extractLangCode` with double extension "en.yml.bak" -> behavior depends on `filepath.Ext` (returns ".bak", then trims ".yml" from "en.yml")
- [ ] `extractLangCode` with no extension "README" -> returns "README"
- [ ] `validateYAMLStructure` with scalar YAML value (e.g., just `"hello"`) -> returns error (not a map)
- [ ] `extractOrderedValues` with mixed numbered and named keys -> numbered keys take priority (numbered loop runs first)
- [ ] `extractOrderedValues` with gap in numbered keys (e.g., `{"0": "a", "2": "c"}`) -> returns only `["a"]` (breaks at missing "1")
- [ ] `selectPluralForm` with count=0 and Zero="" -> falls through to Other
- [ ] `selectPluralForm` with all forms empty -> returns ""
- [ ] `selectPluralForm` with count=1 and One set -> returns One
- [ ] `selectPluralForm` with count=5 and only Other set -> returns Other

**Definition of Done:**
- [ ] Test file compiles and passes (locally if no config import chain, or in CI with dummy env vars)
- [ ] `selectPluralForm` tested via constructing a minimal `Translator` struct (or by testing the logic directly if refactored into a standalone function)
- [ ] All tests use `t.Parallel()`
- [ ] `make lint` passes

---

### US-009: Test `alita/modules/rules_format.go` `normalizeRulesForHTML`

**Priority:** P1 (should-have)

As a developer,
I want unit tests for the rules HTML normalization function,
So that legacy markdown rules display correctly when rendered in HTML mode.

**Target file:** `/Users/divkix/GitHub/Alita_Robot/alita/modules/rules_format_test.go`

**CRITICAL NOTE:** Phase 2 -- `alita/modules` package imports `config`, `db`, and other infra-dependent packages. Tests require CI env vars.

**Functions under test:**
1. `normalizeRulesForHTML(rawRules string) string`

**Acceptance Criteria:**
- [ ] GIVEN an empty string WHEN `normalizeRulesForHTML` is called THEN it returns `""`
- [ ] GIVEN a whitespace-only string WHEN `normalizeRulesForHTML` is called THEN it returns `""`
- [ ] GIVEN a string containing HTML tags (e.g., `"<b>Rule 1</b>"`) WHEN `normalizeRulesForHTML` is called THEN it returns the input unchanged (HTML passthrough)
- [ ] GIVEN a string with markdown formatting (e.g., `"*bold* _italic_"`) and no HTML tags WHEN `normalizeRulesForHTML` is called THEN it returns the result of `tgmd2html.MD2HTMLV2(rawRules)`
- [ ] GIVEN a string with mixed content but containing at least one HTML tag WHEN `normalizeRulesForHTML` is called THEN it returns the input unchanged (HTML tag detection takes precedence)

**Edge Cases:**
- [ ] Input with self-closing HTML tags (e.g., `<br/>`) -> detected as HTML, returned unchanged
- [ ] Input with HTML entities but no tags (e.g., `&amp;`) -> NOT detected as HTML, processed as markdown
- [ ] Input with angle brackets that are not HTML tags (e.g., `1 < 2 > 0`) -> regex `(?i)<\/?[a-z][^>]*>` does NOT match this, so it goes through markdown conversion
- [ ] Very long input (>4096 runes) -> still processes correctly (no truncation in this function)

**Definition of Done:**
- [ ] Test file compiles and passes in CI with dummy env vars
- [ ] All tests use `t.Parallel()`
- [ ] `make lint` passes

---

### US-010: Test `alita/i18n/errors.go` Predefined Error Variables

**Priority:** P2 (nice-to-have)

As a developer,
I want to verify that the predefined i18n error variables are distinct and usable with `errors.Is`,
So that error handling throughout the i18n layer is correct.

**Target file:** Same as US-008 (`/Users/divkix/GitHub/Alita_Robot/alita/i18n/i18n_test.go`)

**Acceptance Criteria:**
- [ ] GIVEN `ErrLocaleNotFound` WHEN compared to `ErrKeyNotFound` via `errors.Is` THEN they are not equal
- [ ] GIVEN `NewI18nError("op", "en", "key", "msg", ErrKeyNotFound)` WHEN `errors.Is(err, ErrKeyNotFound)` is called on the result THEN it returns `true` (unwrap chain)
- [ ] GIVEN all 6 predefined errors WHEN `.Error()` is called on each THEN each returns a distinct non-empty string

**Definition of Done:**
- [ ] Tests pass alongside US-008 tests
- [ ] All tests use `t.Parallel()`

---

## Non-Functional Requirements

### NFR-001: Test Execution Time

- **Metric:** All Phase 1 tests SHALL complete in under 5 seconds on a single core. All Phase 1 + Phase 2 tests SHALL complete in under 30 seconds in CI.
- **Verification:** `time go test ./alita/utils/string_handling/... ./alita/utils/errors/... ./alita/utils/keyword_matcher/... ./alita/utils/callbackcodec/...` reports under 5 seconds.

### NFR-002: Race Condition Safety

- **Metric:** All tests SHALL pass with the `-race` flag (already enabled in `make test` via `-race`).
- **Verification:** `go test -race ./...` reports no data races in any new test file.

### NFR-003: Test Isolation

- **Metric:** Each test case SHALL be independent. No test SHALL depend on execution order or shared mutable state.
- **Verification:** Tests pass when run individually via `go test -run TestXxx` and when run in any order.

### NFR-004: Code Quality

- **Metric:** New test files SHALL introduce zero new `make lint` warnings. Test code SHALL follow existing patterns in `callbackcodec_test.go`.
- **Verification:** `make lint` output has no new warnings referencing `_test.go` files.

### NFR-005: CI Compatibility

- **Metric:** `make test` SHALL pass in the CI environment (GitHub Actions with PostgreSQL 16, dummy env vars as documented in `.github/workflows/ci.yml`).
- **Verification:** CI pipeline green after merging.

### NFR-006: No New Dependencies

- **Metric:** No new entries SHALL be added to `go.mod` for testing purposes. All tests use stdlib `testing` only.
- **Verification:** `git diff go.mod` shows no changes after adding test files.

## Dependencies

| Dependency | Required By | Risk if Unavailable |
|-----------|------------|-------------------|
| Dummy env vars (`BOT_TOKEN`, `OWNER_ID`, `MESSAGE_DUMP`, `DATABASE_URL`, `REDIS_ADDRESS`) | US-003, US-006, US-007, US-008, US-009 (Phase 2) | Phase 2 tests cannot run. Phase 1 tests are unaffected. |
| PostgreSQL 16 service | None (out of scope) | No impact on Phase 1 or Phase 2 tests. |
| `cloudflare/ahocorasick` library | US-004 | Already in `go.mod`. If removed, `keyword_matcher` package would not compile. |
| `gotg_md2html` library | US-006 (ReverseHTML2MD), US-009 (normalizeRulesForHTML) | Already in `go.mod`. If removed, affected functions would not compile. |
| `gotgbot/v2` types | US-006 (BuildKeyboard, ChunkKeyboardSlices) | Already in `go.mod`. Test constructs `gotgbot.InlineKeyboardButton` structs directly. |
| Existing `callbackcodec_test.go` patterns | US-005, all stories | If file is deleted, US-005 additions have no base. Low risk. |

## Assumptions

1. **`typeConvertor` is unexported** -- The `typeConvertor` struct is lowercase (unexported). Tests MUST be in the `config` package to access it. If this assumption is wrong and it is exported, tests could use an external test package. Impact: changes Phase classification of US-003.

2. **`config/init()` runs on package import** -- There is no way to skip `init()` when importing the `config` package. If this changes (e.g., someone refactors to lazy init), Phase 2 tests become Phase 1 tests. Impact: reduces CI dependency for Phase 2.

3. **`notesParser` is unexported** -- The function is lowercase. Tests MUST be in the `helpers` package. If exported, tests could use an external package. Impact: test file must be `package helpers`.

4. **`selectPluralForm` is a method on `Translator`** -- Testing requires constructing a `Translator` struct. Since `Translator` fields are unexported, tests MUST be in the `i18n` package. Impact: test file must use `package i18n`.

5. **`extractOrderedValues` is a package-level function** -- It is unexported but accessible from within the `i18n` package. Impact: test must be in `package i18n`.

6. **CI runs `make test` which includes `-race` and `-count=1`** -- All tests are run fresh (no caching) with race detection. Impact: flaky tests or race conditions will be caught in CI.

7. **No Redis in CI** -- The CI config does NOT include a Redis service container. Tests that require Redis will fail or must be skipped. Impact: cache-dependent tests are out of scope.

## Open Questions

- [ ] **Should `config/init()` be refactored for testability?** -- Currently blocks ~70% of the codebase from local testing. A lazy-init pattern would unblock all Phase 2 tests for local execution. Blocks: future test expansion beyond Phase 2.
- [ ] **Should DB integration tests use build tags?** -- Gating with `//go:build integration` would allow `go test ./...` to pass locally. Blocks: Phase 3 work (out of scope).
- [ ] **Is there a target test-to-code ratio?** -- Currently 3.4%. After this work, expected to reach approximately 6-8%. Blocks: nothing, but informs future prioritization.
- [ ] **Should `ChunkKeyboardSlices` with chunkSize=0 panic or be guarded?** -- The current implementation would infinite-loop. Blocks: US-006 edge case decision.
- [ ] **Should `notesParser` use precompiled regexes instead of `regexp.MatchString` on every call?** -- Performance concern flagged during research. Does not block tests but is a refactor opportunity. Blocks: nothing.

## Glossary

| Term | Definition |
|------|-----------|
| Phase 1 | Tests that run locally without any environment variables or external services (zero-infra). Packages: `string_handling`, `errors`, `keyword_matcher`, `callbackcodec`. |
| Phase 2 | Tests that require dummy environment variables (as set in CI) because the package transitively imports `config` which calls `log.Fatalf` in `init()`. Packages: `config/types`, `helpers`, `chat_status`, `i18n`, `modules/rules_format`. |
| Phase 3 | Tests that require a running PostgreSQL instance. Deferred, out of scope. |
| Pure function | A function with no side effects: no DB calls, no API calls, no cache access, no file I/O. Its output depends only on its inputs. |
| Table-driven test | Go testing pattern where test cases are defined as a slice of structs, each iterated via `t.Run` subtests. |
| `typeConvertor` | Unexported struct in `alita/config/types.go` that converts string environment variable values to typed Go values. |
| Dummy env vars | The set of environment variables (`BOT_TOKEN=test-token`, `OWNER_ID=1`, `MESSAGE_DUMP=1`, `DATABASE_URL=...`, `REDIS_ADDRESS=...`) required by `config/init()` to avoid `log.Fatalf`. |
| `init()` blocker | The `init()` function in `alita/config/config.go` that calls `log.Fatalf` when required env vars are missing. Root cause of most test failures in the codebase. |

## Implementation Priority Summary

| Priority | Story | Phase | Est. Effort | Functions Covered |
|----------|-------|-------|-------------|-------------------|
| P0 | US-001: string_handling | 1 | 30 min | 3 |
| P0 | US-002: errors | 1 | 30 min | 4 |
| P0 | US-003: config/types | 2 | 45 min | 5 |
| P0 | US-004: keyword_matcher | 1 | 60 min | 4 |
| P0 | US-005: callbackcodec gaps | 1 | 30 min | 3 |
| P1 | US-006: helpers | 2 | 90 min | 13 |
| P1 | US-007: chat_status | 2 | 20 min | 2 |
| P1 | US-008: i18n | 2 | 60 min | 8 |
| P1 | US-009: rules_format | 2 | 20 min | 1 |
| P2 | US-010: i18n errors | 2 | 15 min | 6 predefined vars |
| | | **Total** | **~6.5 hours** | **44+ functions** |

REQUIREMENTS_COMPLETE
