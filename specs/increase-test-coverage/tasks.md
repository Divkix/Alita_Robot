# Implementation Tasks: increase-test-coverage

**Date:** 2026-02-21
**Design Source:** design.md
**Total Tasks:** 10
**Slicing Strategy:** vertical (each task = complete feature slice)

---

## TASK-001: Add unit tests for string_handling package

**Complexity:** S
**Files:**
- CREATE: `alita/utils/string_handling/string_handling_test.go`
**Dependencies:** None
**Description:**
Create a comprehensive test file for all 3 functions in the `string_handling` package. This is a Phase 1 task with zero infrastructure dependencies -- no env vars, no DB, no cache.

Functions under test (all in `alita/utils/string_handling/string_handling.go`):

1. `FindInStringSlice(slice []string, val string) bool` -- wraps `slices.Contains`
2. `FindInInt64Slice(slice []int64, val int64) bool` -- wraps `slices.Contains`
3. `IsDuplicateInStringSlice(arr []string) (string, bool)` -- map-based duplicate detection

The test file MUST use `package string_handling` (internal), NOT `package string_handling_test`.

Test cases for `TestFindInStringSlice`:
- nil slice -> false
- empty slice -> false
- slice with value present -> true
- slice with value absent -> false
- empty string as search value in slice containing empty string -> true
- empty string as search value in slice without empty string -> false

Test cases for `TestFindInInt64Slice`:
- nil slice -> false
- empty slice -> false
- slice with value present -> true
- slice with value absent -> false
- zero value (0) present -> true
- zero value (0) absent -> false
- negative value (e.g., -1001234567890 channel ID) -> true
- `math.MaxInt64` boundary -> true when present
- `math.MinInt64` boundary -> true when present

Test cases for `TestIsDuplicateInStringSlice`:
- nil slice -> ("", false) without panic
- empty slice -> ("", false)
- single element -> ("", false)
- no duplicates ["a","b","c"] -> ("", false)
- duplicate present ["a","b","a"] -> ("a", true)
- all identical ["x","x","x"] -> ("x", true)
- empty string duplicates ["",""] -> ("", true)

Follow the exact pattern from `alita/utils/callbackcodec/callbackcodec_test.go`:
- `t.Parallel()` at both top-level and subtest level
- `tc := tc` loop variable capture
- `t.Fatalf()` for assertions
- Table-driven with named struct fields

Import only `testing` and `math` from stdlib.

**Context to Read:**
- design.md, section "Component: string_handling Tests"
- `/Users/divkix/GitHub/Alita_Robot/alita/utils/string_handling/string_handling.go` -- source under test
- `/Users/divkix/GitHub/Alita_Robot/alita/utils/callbackcodec/callbackcodec_test.go` -- reference test pattern
**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && go test -v -race -count=1 ./alita/utils/string_handling/... && make lint
```

---

## TASK-002: Add unit tests for errors package

**Complexity:** M
**Files:**
- CREATE: `alita/utils/errors/errors_test.go`
**Dependencies:** None
**Description:**
Create a comprehensive test file for the custom error wrapping package. This is a Phase 1 task with zero infrastructure dependencies.

Functions under test (all in `alita/utils/errors/errors.go`):

1. `Wrap(err error, message string) error` -- wraps error with runtime caller info (file/line/func)
2. `Wrapf(err error, format string, args ...any) error` -- wraps with formatted message, delegates to `Wrap`
3. `WrappedError.Error() string` -- formats as `"<message> at <file>:<line> in <func>: <err>"`
4. `WrappedError.Unwrap() error` -- returns underlying error

The test file MUST use `package errors` (internal) to access `WrappedError` struct fields directly.

Import stdlib `errors` with an alias (e.g., `stderrors "errors"`) to avoid conflict with the package name. Also import `strings` and `testing`.

Test functions to implement:

`TestWrapNilError`: `Wrap(nil, "msg")` returns nil.

`TestWrapNonNilError`: `Wrap(fmt.Errorf("base"), "operation failed")` returns a `*WrappedError` with:
- `.Message == "operation failed"`
- `.Err` equals the base error
- `.File` is non-empty and contains at most 1 slash (path truncation: `strings.Count(we.File, "/") <= 1`)
- `.Line > 0`
- `.Function` is non-empty

`TestWrapfNilError`: `Wrapf(nil, "op %s", "save")` returns nil.

`TestWrapfFormatsMessage`: `Wrapf(err, "op %s id %d", "save", 42)` produces `.Message == "op save id 42"`.

`TestWrappedErrorFormat`: `.Error()` output contains "at", the file, ":", the line number, "in", the function name, and the original error message.

`TestUnwrapChain`: `stderrors.Is(Wrap(baseErr, "outer"), baseErr)` returns true. `stderrors.Unwrap(wrappedErr)` returns the underlying error.

`TestDoubleWrap`: `Wrap(Wrap(baseErr, "inner"), "outer")` -- `stderrors.Is` still reaches `baseErr` through the chain.

`TestWrapEmptyMessage`: `Wrap(err, "")` still produces a valid `WrappedError` with file/line populated.

`TestFilePathTruncation`: The `.File` field contains at most 2 path segments. Verify with `strings.Count(we.File, "/") <= 1`.

**Context to Read:**
- design.md, section "Component: errors Tests"
- `/Users/divkix/GitHub/Alita_Robot/alita/utils/errors/errors.go` -- source under test (62 lines)
- `/Users/divkix/GitHub/Alita_Robot/alita/utils/callbackcodec/callbackcodec_test.go` -- reference pattern
**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && go test -v -race -count=1 ./alita/utils/errors/... && make lint
```

---

## TASK-003: Add unit tests for keyword_matcher package

**Complexity:** M
**Files:**
- CREATE: `alita/utils/keyword_matcher/matcher_test.go`
**Dependencies:** None
**Description:**
Create a comprehensive test file for the Aho-Corasick keyword matcher. This is a Phase 1 task -- the package only depends on `cloudflare/ahocorasick` and `logrus` (both already in `go.mod`), no config dependency.

Functions under test (all in `alita/utils/keyword_matcher/matcher.go`):

1. `NewKeywordMatcher(patterns []string) *KeywordMatcher`
2. `FindMatches(text string) []MatchResult` -- returns `[]MatchResult{Pattern, Start, End}`
3. `HasMatch(text string) bool`
4. `GetPatterns() []string` -- returns a defensive copy

The test file MUST use `package keyword_matcher` (internal).

Imports: `testing`, `sync` (for concurrent test).

Test functions:

`TestNewKeywordMatcher`: Constructs matchers with various pattern sets. Verify patterns are stored via `GetPatterns()`.

`TestFindMatches` (table-driven):
- patterns=["hello","world"], text="hello world" -> 2 results
- patterns=["hello"], text="HELLO" -> 1 result (case-insensitive)
- patterns=["ab"], text="ababab" -> 3 results with correct Start/End positions (0-2, 2-4, 4-6)
- patterns=["hello"], text="" -> nil
- patterns=[], text="anything" -> nil
- patterns=["foo.bar"], text="foo.bar" -> 1 result (regex metacharacters matched literally)

`TestHasMatch` (table-driven):
- patterns=["hello"], text="say hello there" -> true
- patterns=["hello"], text="goodbye" -> false
- patterns=["hello"], text="" -> false
- patterns=[], text="anything" -> false

`TestGetPatterns`:
- Create matcher with ["abc","def"], call `GetPatterns()`, verify returns ["abc","def"]
- Mutate the returned slice, call `GetPatterns()` again, verify internal patterns unchanged (defensive copy)

`TestFindMatchesPositions`:
- patterns=["test"], text="test" -> Start=0, End=4
- patterns=["ab"], text="xabx" -> Start=1, End=3

`TestConcurrentAccess`:
- Create matcher with ["hello","world"]
- Launch 10 goroutines via `sync.WaitGroup`, each calling `FindMatches` and `HasMatch` 100 times
- Race detector (`-race` flag) validates thread safety
- No assertions on return values -- the test is purely for race detection

`TestNewKeywordMatcherNilPatterns`: `NewKeywordMatcher(nil)` -> `HasMatch("anything")` returns false.

`TestFindMatchesEmptyText`: `FindMatches("")` -> returns nil.

`TestSpecialCharacterPatterns`: patterns with regex metacharacters `["foo.bar", "[test]", "(abc)"]` -> matched literally in text.

**Context to Read:**
- design.md, section "Component: keyword_matcher Tests"
- `/Users/divkix/GitHub/Alita_Robot/alita/utils/keyword_matcher/matcher.go` -- source under test (177 lines)
- `/Users/divkix/GitHub/Alita_Robot/alita/utils/callbackcodec/callbackcodec_test.go` -- reference pattern
**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && go test -v -race -count=1 ./alita/utils/keyword_matcher/... && make lint
```

---

## TASK-004: Fill coverage gaps in callbackcodec tests

**Complexity:** S
**Files:**
- MODIFY: `alita/utils/callbackcodec/callbackcodec_test.go` -- append new test functions to existing file
**Dependencies:** None
**Description:**
Append new test functions to the existing `callbackcodec_test.go` to increase coverage from 79.1% to >95%. This is a Phase 1 task.

The existing file has 4 test functions: `TestEncodeDecodeRoundTrip`, `TestEncodeRejectsInvalidNamespace`, `TestEncodeRejectsOversizedPayload`, `TestDecodeRejectsMalformedPayloads`. Append AFTER these existing functions. Do NOT modify existing tests.

Functions needing additional coverage (in `alita/utils/callbackcodec/callbackcodec.go`):
1. `EncodeOrFallback(namespace string, fields map[string]string, fallback string) string`
2. `Decoded.Field(key string) (string, bool)` -- nil receiver path
3. `Encode` with empty fields, nil fields, empty key

New test functions to append:

`TestEncodeOrFallbackSuccess`: Valid encode returns encoded data (not the fallback).

`TestEncodeOrFallbackInvalidNamespace`: Empty namespace -> returns fallback string "fallback_data".

`TestEncodeOrFallbackOversized`: Oversized payload -> returns fallback.

`TestEncodeOrFallbackEmptyFallback`: Failure with fallback="" -> returns "".

`TestFieldNilReceiver`: `(*Decoded)(nil).Field("x")` -> ("", false), no panic.

`TestFieldExistingKey`: Decoded with Fields={"a":"yes"} -> `.Field("a")` returns ("yes", true).

`TestFieldMissingKey`: Decoded with Fields={"a":"yes"} -> `.Field("missing")` returns ("", false).

`TestEncodeEmptyFields`: `Encode("ns", map[string]string{})` -> succeeds, decoded payload has empty Fields map.

`TestEncodeNilFields`: `Encode("ns", nil)` -> succeeds, payload uses "_" placeholder.

`TestEncodeSkipsEmptyKey`: `Encode("ns", map[string]string{"": "val"})` -> empty key skipped, payload is "_".

`TestDecodeUnderscorePayload`: `Decode("ns|v1|_")` -> Decoded with empty Fields map.

`TestRoundTripURLSpecialChars`: Encode with field values containing `&`, `=`, `%25` -> Decode preserves values exactly.

All new tests follow existing style: `t.Parallel()`, `t.Fatalf()`. No new imports needed beyond what already exists (`errors`, `strings`, `testing`).

**Context to Read:**
- design.md, section "Component: callbackcodec Gap-Fill Tests"
- `/Users/divkix/GitHub/Alita_Robot/alita/utils/callbackcodec/callbackcodec.go` -- source under test
- `/Users/divkix/GitHub/Alita_Robot/alita/utils/callbackcodec/callbackcodec_test.go` -- existing tests to append to
**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && go test -v -race -count=1 -coverprofile=codec.out ./alita/utils/callbackcodec/... && go tool cover -func=codec.out | grep total && make lint
```

---

## TASK-005: Add unit tests for config/types typeConvertor

**Complexity:** S
**Files:**
- CREATE: `alita/config/types_test.go`
**Dependencies:** None
**Description:**
Create tests for the 5 `typeConvertor` methods. This is a Phase 2 task -- the `config` package has an `init()` that calls `log.Fatalf` when env vars are missing. Tests WILL pass in CI (which sets dummy env vars) but NOT locally without env vars.

The test file MUST use `package config` (internal) because `typeConvertor` is unexported.

Functions under test (all in `alita/config/types.go`):

1. `typeConvertor.Bool() bool` -- returns true for "yes"/"true"/"1" (case-insensitive, trimmed), false otherwise
2. `typeConvertor.Int() int` -- `strconv.Atoi`, returns 0 on failure
3. `typeConvertor.Int64() int64` -- `strconv.ParseInt(s, 10, 64)`, returns 0 on failure
4. `typeConvertor.Float64() float64` -- `strconv.ParseFloat(s, 64)`, returns 0.0 on failure
5. `typeConvertor.StringArray() []string` -- `strings.Split(s, ",")` with `TrimSpace` on each element

Imports: `testing`, `math` (for boundary values), `reflect` (for slice comparison).

`TestTypeConvertorBool` (table-driven):
- "true" -> true, "yes" -> true, "1" -> true
- "TRUE" -> true, "YES" -> true, " true " -> true (whitespace trimmed)
- "false" -> false, "no" -> false, "0" -> false
- "" -> false, "2" -> false, "random" -> false

`TestTypeConvertorInt` (table-driven):
- "42" -> 42, "-100" -> -100, "0" -> 0
- "" -> 0, "not_a_number" -> 0
- "9999999999999999999" (overflow) -> 0

`TestTypeConvertorInt64` (table-driven):
- "42" -> 42, "-100" -> -100, "0" -> 0
- "9223372036854775807" (math.MaxInt64) -> math.MaxInt64
- "" -> 0, "invalid" -> 0
- " 42 " (whitespace) -> 0 (ParseInt does NOT trim)

`TestTypeConvertorFloat64` (table-driven):
- "3.14" -> 3.14, "0" -> 0.0, "-1.5" -> -1.5
- "" -> 0.0, "invalid" -> 0.0
- "NaN" -> use `math.IsNaN()` to check
- "Inf" -> use `math.IsInf(val, 1)` to check

`TestTypeConvertorStringArray` (table-driven):
- "a,b,c" -> ["a","b","c"]
- " a , b , c " -> ["a","b","c"] (trimmed)
- "single" -> ["single"]
- "" -> [""] (strings.Split behavior: one empty element)
- ",," -> ["","",""] (consecutive commas)

Use `reflect.DeepEqual` for slice assertions.

**Context to Read:**
- design.md, section "Component: config/types Tests"
- `/Users/divkix/GitHub/Alita_Robot/alita/config/types.go` -- source under test (51 lines)
- `/Users/divkix/GitHub/Alita_Robot/alita/utils/callbackcodec/callbackcodec_test.go` -- reference pattern
**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && BOT_TOKEN=test-token OWNER_ID=1 MESSAGE_DUMP=1 DATABASE_URL="postgres://postgres:postgres@localhost:5432/alita_test?sslmode=disable" REDIS_ADDRESS=localhost:6379 go test -v -race -count=1 ./alita/config/... && make lint
```

---

## TASK-006: Add unit tests for helpers package pure functions

**Complexity:** L
**Files:**
- CREATE: `alita/utils/helpers/helpers_test.go`
**Dependencies:** None
**Description:**
Create tests for 13 pure helper functions spread across 3 source files in the helpers package. This is a Phase 2 task (requires dummy env vars because helpers imports config, db, i18n transitively).

The test file MUST use `package helpers` (internal) because `notesParser` is unexported.

Imports: `testing`, `strings`, `errors` (stdlib), `fmt`, `reflect`, `github.com/PaulSonOfLars/gotgbot/v2`, `github.com/PaulSonOfLars/gotg_md2html`, `github.com/divkix/Alita_Robot/alita/db`.

Test functions to implement:

**TestIsChannelID** (table-driven, source: `channel_helpers.go`):
- -1001234567890 -> true
- -1000000000001 -> true (boundary: first channel ID)
- -1000000000000 -> false (boundary: strictly less than)
- -123456789 -> false (regular group)
- 123456789 -> false (user)
- 0 -> false

**TestSplitMessage** (table-driven, source: `helpers.go`):
- String of 4096 runes -> single-element slice
- String of 4097 runes -> multi-element slice with each <= 4096 runes
- Empty string -> `[""]`
- Multi-byte UTF-8 (4096 emoji chars, each 4 bytes) -> single element (split by rune count, not bytes)
- Long single line (>4096 runes, no newlines) -> split into 4096-rune chunks
- String with newlines that fit -> preserves newline-based structure

**TestMentionHtml** (table-driven):
- userId=123, name="Test" -> `<a href="tg://user?id=123">Test</a>`
- name with HTML special chars "<script>" -> chars escaped via `html.EscapeString`

**TestMentionUrl** (table-driven):
- url="https://example.com", name="Link" -> `<a href="https://example.com">Link</a>`
- name with `&` -> escaped

**TestHtmlEscape** (table-driven):
- "&<>" -> "&amp;&lt;&gt;" (& escaped first to prevent double-escaping)
- "" -> ""
- "no special chars" -> "no special chars"
- Already escaped "&amp;" -> "&amp;amp;" (no double-escape prevention -- the function does NOT check for existing escapes)

**TestGetFullName** (table-driven):
- "John", "Doe" -> "John Doe"
- "John", "" -> "John"
- "", "" -> ""

**TestBuildKeyboard** (table-driven):
- empty slice -> empty 2D slice
- single button SameLine=false -> [[button]]
- two buttons, second SameLine=true -> [[button1, button2]]
- two buttons, both SameLine=false -> [[button1], [button2]]
- first button SameLine=true -> [[button]] (len(keyb)==0 check, gets own row)
Construct `db.Button{Name: "x", Url: "https://x.com", SameLine: false/true}` structs.

**TestConvertButtonV2ToDbButton** (table-driven):
- basic conversion from `[]tgmd2html.ButtonV2` to `[]db.Button`
- empty slice -> empty slice (not nil, since `make([]db.Button, 0)`)
- nil slice -> empty slice of len 0 via `make([]db.Button, 0)` (since `len(nil)` is 0)

**TestRevertButtons** (table-driven):
- single button SameLine=false -> "\n[name](buttonurl://url)"
- single button SameLine=true -> "\n[name](buttonurl://url:same)"
- empty slice -> ""

**TestChunkKeyboardSlices** (table-driven):
- 4 buttons, chunkSize=2 -> 2 chunks of 2
- 5 buttons, chunkSize=2 -> 2 chunks of 2 + 1 chunk of 1
- empty slice -> nil
- chunkSize > len(slice) -> single chunk with all elements
- NOTE: Do NOT test chunkSize=0 -- known infinite loop defect. Add a comment documenting this.

**TestReverseHTML2MD** (table-driven):
- `<b>bold</b>` -> `*bold*`
- `<i>italic</i>` -> `_italic_`
- `<a href="https://x.com">link</a>` -> `[link](https://x.com)`
- no HTML tags -> unchanged
- Verify at least bold, italic, and link conversion

**TestIsExpectedTelegramError** (table-driven, source: `telegram_helpers.go`):
- nil error -> false
- error with each of the 17 expected error strings -> true (one test case per string)
- unexpected error "unknown xyz" -> false
Construct errors with `fmt.Errorf("...string...")`.

**TestNotesParser** (table-driven):
- "hello {private} world" -> pvtOnly=true, sentBack="hello  world"
- "test {admin}" -> adminOnly=true
- "test {preview}" -> webPrev=true
- "test {noprivate}" -> grpOnly=true
- "test {protect}" -> protectedContent=true
- "test {nonotif}" -> noNotif=true
- "no tags here" -> all false, sentBack="no tags here"
- "all {private} {admin} {preview} {protect} {nonotif}" -> pvtOnly=true, adminOnly=true, webPrev=true, protectedContent=true, noNotif=true

**Context to Read:**
- design.md, section "Component: helpers Tests"
- `/Users/divkix/GitHub/Alita_Robot/alita/utils/helpers/helpers.go` -- most functions
- `/Users/divkix/GitHub/Alita_Robot/alita/utils/helpers/channel_helpers.go` -- IsChannelID
- `/Users/divkix/GitHub/Alita_Robot/alita/utils/helpers/telegram_helpers.go` -- IsExpectedTelegramError
- `/Users/divkix/GitHub/Alita_Robot/alita/utils/callbackcodec/callbackcodec_test.go` -- reference pattern
**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && BOT_TOKEN=test-token OWNER_ID=1 MESSAGE_DUMP=1 DATABASE_URL="postgres://postgres:postgres@localhost:5432/alita_test?sslmode=disable" REDIS_ADDRESS=localhost:6379 go test -v -race -count=1 ./alita/utils/helpers/... && make lint
```

---

## TASK-007: Add unit tests for chat_status pure functions

**Complexity:** S
**Files:**
- CREATE: `alita/utils/chat_status/chat_status_test.go`
**Dependencies:** None
**Description:**
Create tests for the 2 pure ID validation functions in `chat_status`. This is a Phase 2 task (imports `db` which imports `config`).

The test file MUST use `package chat_status` (internal).

Functions under test (in `alita/utils/chat_status/chat_status.go`):

1. `IsValidUserId(id int64) bool` -- returns `id > 0`
2. `IsChannelId(id int64) bool` -- returns `id < -1000000000000`

Imports: `testing`, `math`.

`TestIsValidUserId` (table-driven):
- 123456789 -> true
- 1 -> true (minimum valid)
- 0 -> false
- -1 -> false
- -1001234567890 (channel ID) -> false
- math.MaxInt64 -> true
- math.MinInt64 -> false
- 1087968824 (Group Anonymous Bot) -> true
- 777000 (Telegram) -> true

`TestIsChannelId` (table-driven):
- -1001234567890 -> true
- -1000000000001 -> true (boundary: first channel ID)
- -1000000000000 -> false (boundary: strictly less than)
- 123456789 -> false (user)
- 0 -> false
- math.MaxInt64 -> false
- math.MinInt64 -> true

**Context to Read:**
- design.md, section "Component: chat_status Tests"
- `/Users/divkix/GitHub/Alita_Robot/alita/utils/chat_status/chat_status.go` -- first 48 lines contain the functions
- `/Users/divkix/GitHub/Alita_Robot/alita/utils/callbackcodec/callbackcodec_test.go` -- reference pattern
**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && BOT_TOKEN=test-token OWNER_ID=1 MESSAGE_DUMP=1 DATABASE_URL="postgres://postgres:postgres@localhost:5432/alita_test?sslmode=disable" REDIS_ADDRESS=localhost:6379 go test -v -race -count=1 ./alita/utils/chat_status/... && make lint
```

---

## TASK-008: Add unit tests for i18n package pure functions

**Complexity:** L
**Files:**
- CREATE: `alita/i18n/i18n_test.go`
**Dependencies:** None
**Description:**
Create tests for the pure utility functions in the i18n package including loader utilities, error types, and translator utilities. This is a Phase 2 task (i18n imports `alita/utils/cache` which may transitively import config).

The test file MUST use `package i18n` (internal) because `extractLangCode`, `isYAMLFile`, `validateYAMLStructure`, `extractOrderedValues`, and `selectPluralForm` are all unexported.

Imports: `testing`, `errors` (stdlib), `strings`.

**Loader Utilities:**

`TestExtractLangCode` (table-driven, source: `loader.go:102-108`):
The function does `strings.TrimSuffix(fileName, filepath.Ext(fileName))` then trims `.yml` and `.yaml`.
- "en.yml" -> "en"
- "en.yaml" -> "en"
- "pt-BR.yml" -> "pt-BR"
- "README" (no extension) -> "README"
- "en.yml.bak" -> "en.yml" (filepath.Ext returns ".bak", trimming it leaves "en.yml", then ".yml" trim leaves "en")

`TestIsYAMLFile` (table-driven, source: `loader.go:123-126`):
The function does `strings.ToLower(filepath.Ext(fileName))` and checks `.yml` or `.yaml`.
- "en.yml" -> true
- "en.yaml" -> true
- "en.json" -> false
- "" -> false
- "en.YML" -> true (case-insensitive)
- "en.YAML" -> true

`TestValidateYAMLStructure` (table-driven, source: `loader.go:87-99`):
- Valid YAML map `[]byte("key: value\n")` -> nil error
- Invalid YAML `[]byte("{{{")` -> non-nil error
- List root `[]byte("- item1\n- item2\n")` -> non-nil error (root must be map)
- Scalar root `[]byte("hello\n")` -> non-nil error (not a map)
- Empty content `[]byte("")` -> non-nil error (nil is not a map)
- Valid nested map `[]byte("parent:\n  child: value\n")` -> nil

**Error Types:**

`TestI18nErrorFormat`:
- With Err: `NewI18nError("get", "en", "hello", "not found", fmt.Errorf("base"))` -> `.Error()` contains "i18n get failed" and "base"
- Without Err: `NewI18nError("get", "en", "hello", "not found", nil)` -> `.Error()` does NOT contain ": <nil>"

`TestI18nErrorUnwrap`:
- Non-nil Err -> `.Unwrap()` returns the underlying error
- Nil Err -> `.Unwrap()` returns nil

`TestNewI18nError`: Constructor sets all fields correctly. Verify Op, Lang, Key, Message, Err.

`TestPredefinedErrorsDistinct`: All 6 predefined errors (`ErrLocaleNotFound`, `ErrKeyNotFound`, `ErrInvalidYAML`, `ErrManagerNotInit`, `ErrRecursiveFallback`, `ErrInvalidParams`) are distinct via `errors.Is`. Each pair returns false.

`TestPredefinedErrorsChain`: `NewI18nError("op", "en", "key", "msg", ErrKeyNotFound)` -> `errors.Is(err, ErrKeyNotFound)` returns true.

**Translator Utilities:**

`TestExtractOrderedValues` (table-driven, source: `translator.go:280-324`):
- `TranslationParams{"0": "a", "1": "b", "2": "c"}` -> `[]any{"a", "b", "c"}`
- `TranslationParams{"first": "x", "second": "y"}` -> `[]any{"x", "y"}` (common key order)
- nil -> nil
- `TranslationParams{}` -> nil (empty map, no numbered keys, no common keys)
- Gap in numbered keys `{"0": "a", "2": "c"}` -> `[]any{"a"}` (breaks at missing "1")
- Mixed numbered and named `{"0": "a", "1": "b", "first": "x"}` -> `[]any{"a", "b"}` (numbered keys take priority, len > 0 so common keys skipped)

`TestSelectPluralForm` (table-driven, source: `translator.go:255-277`):
Construct a minimal `Translator` struct: `&Translator{langCode: "en", manager: &LocaleManager{defaultLang: "en"}}`.
- count=0, Zero="none" -> "none"
- count=1, One="one item" -> "one item"
- count=2, Two="two items" -> "two items"
- count=5, Other="many" -> "many"
- count=0, Zero="", Other="fallback" -> "fallback" (Zero empty, falls to Other)
- count=1, One="", Many="", Other="fallback" -> "fallback"
- All forms empty -> ""
- count=0, Zero="", One="", Many="lots", Other="" -> "lots" (fallback chain: Other empty, tries Many)

**Context to Read:**
- design.md, section "Component: i18n Tests"
- `/Users/divkix/GitHub/Alita_Robot/alita/i18n/errors.go` -- I18nError and predefined errors
- `/Users/divkix/GitHub/Alita_Robot/alita/i18n/loader.go` -- extractLangCode, isYAMLFile, validateYAMLStructure
- `/Users/divkix/GitHub/Alita_Robot/alita/i18n/translator.go` lines 255-324 -- selectPluralForm, extractOrderedValues
- `/Users/divkix/GitHub/Alita_Robot/alita/i18n/types.go` -- TranslationParams, PluralRule, Translator, LocaleManager struct definitions
- `/Users/divkix/GitHub/Alita_Robot/alita/utils/callbackcodec/callbackcodec_test.go` -- reference pattern
**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && BOT_TOKEN=test-token OWNER_ID=1 MESSAGE_DUMP=1 DATABASE_URL="postgres://postgres:postgres@localhost:5432/alita_test?sslmode=disable" REDIS_ADDRESS=localhost:6379 go test -v -race -count=1 ./alita/i18n/... && make lint
```

---

## TASK-009: Add unit tests for rules_format normalizeRulesForHTML

**Complexity:** S
**Files:**
- CREATE: `alita/modules/rules_format_test.go`
**Dependencies:** None
**Description:**
Create tests for the `normalizeRulesForHTML` function. This is a Phase 2 task (modules package imports config, db, etc.).

The test file MUST use `package modules` (internal) because `normalizeRulesForHTML` is unexported.

Function under test (in `alita/modules/rules_format.go`):

`normalizeRulesForHTML(rawRules string) string`:
1. Trims whitespace. If empty, returns "".
2. Checks `htmlTagPattern.MatchString(trimmed)` -- regex: `(?i)<\/?[a-z][^>]*>`.
3. If HTML tags detected, returns rawRules UNCHANGED (not trimmed).
4. If no HTML tags, returns `tgmd2html.MD2HTMLV2(rawRules)`.

Imports: `testing`, `strings`.

`TestNormalizeRulesForHTML` (table-driven):
- Empty string "" -> ""
- Whitespace-only "   " -> ""
- HTML tags present `"<b>Rule 1</b>"` -> returned unchanged (HTML passthrough)
- Self-closing tag `"<br/>"` -> returned unchanged
- Markdown only `"*bold* _italic_"` -> run through `tgmd2html.MD2HTMLV2` (verify output contains `<b>` or similar)
- Angle brackets NOT HTML `"1 < 2 > 0"` -> regex `<\/?[a-z][^>]*>` does NOT match (`<` is followed by space, not [a-z]), so treated as markdown
- HTML entities without tags `"&amp; stuff"` -> NOT detected as HTML, processed as markdown
- Mixed with HTML tag present `"some text <b>bold</b> more"` -> returned unchanged

For the markdown conversion cases, do NOT hardcode the exact `tgmd2html.MD2HTMLV2` output. Instead, verify:
- Output is NOT equal to input (conversion happened)
- Or use `strings.Contains` to check for expected HTML tags

For the HTML passthrough cases, verify output equals the original `rawRules` (NOT the trimmed version -- the function returns `rawRules` not `trimmed`).

**Context to Read:**
- design.md, section "Component: rules_format Tests"
- `/Users/divkix/GitHub/Alita_Robot/alita/modules/rules_format.go` -- source under test (22 lines)
- `/Users/divkix/GitHub/Alita_Robot/alita/utils/callbackcodec/callbackcodec_test.go` -- reference pattern
**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && BOT_TOKEN=test-token OWNER_ID=1 MESSAGE_DUMP=1 DATABASE_URL="postgres://postgres:postgres@localhost:5432/alita_test?sslmode=disable" REDIS_ADDRESS=localhost:6379 go test -v -race -count=1 -run TestNormalizeRulesForHTML ./alita/modules/... && make lint
```

---

## TASK-010: Full integration verification

**Complexity:** S
**Files:**
- None (verification only)
**Dependencies:** TASK-001, TASK-002, TASK-003, TASK-004, TASK-005, TASK-006, TASK-007, TASK-008, TASK-009
**Description:**
Run the full test suite, linter, and verify all acceptance criteria are met. This task produces no file changes -- it is purely verification.

Steps:

1. Run Phase 1 tests locally without env vars:
```bash
cd /Users/divkix/GitHub/Alita_Robot && go test -v -race -count=1 \
  ./alita/utils/string_handling/... \
  ./alita/utils/errors/... \
  ./alita/utils/keyword_matcher/... \
  ./alita/utils/callbackcodec/...
```
All 4 packages MUST pass with zero failures.

2. Run Phase 2 tests with dummy env vars:
```bash
cd /Users/divkix/GitHub/Alita_Robot && BOT_TOKEN=test-token OWNER_ID=1 MESSAGE_DUMP=1 \
  DATABASE_URL="postgres://postgres:postgres@localhost:5432/alita_test?sslmode=disable" \
  REDIS_ADDRESS=localhost:6379 \
  go test -v -race -count=1 \
  ./alita/config/... \
  ./alita/utils/helpers/... \
  ./alita/utils/chat_status/... \
  ./alita/i18n/... \
  ./alita/modules/...
```
Note: Some Phase 2 packages may fail due to DB connection in `init()`. The TEST FUNCTIONS themselves must pass if the process starts. If `init()` kills the process, that is a pre-existing condition, not a regression.

3. Run linter:
```bash
cd /Users/divkix/GitHub/Alita_Robot && make lint
```
No new warnings from `_test.go` files.

4. Verify callbackcodec coverage:
```bash
cd /Users/divkix/GitHub/Alita_Robot && go test -coverprofile=codec.out ./alita/utils/callbackcodec/... && go tool cover -func=codec.out | grep total
```
Coverage MUST be >95%.

5. Verify no new dependencies:
```bash
cd /Users/divkix/GitHub/Alita_Robot && git diff go.mod go.sum
```
No changes to `go.mod` or `go.sum`.

6. Verify acceptance criteria:
- [ ] US-001: string_handling -- 3 functions tested with nil/empty/present/absent/boundary cases
- [ ] US-002: errors -- Wrap/Wrapf/Error/Unwrap with nil, chain, format, truncation tests
- [ ] US-003: config/types -- 5 typeConvertor methods with truthy/falsy/boundary/invalid tests
- [ ] US-004: keyword_matcher -- 4 functions with case-insensitivity, concurrency, empty input tests
- [ ] US-005: callbackcodec -- EncodeOrFallback, nil Field, empty fields, URL special chars tested; >95% coverage
- [ ] US-006: helpers -- 13 functions tested with boundary, HTML escape order, all expected errors
- [ ] US-007: chat_status -- 2 pure functions with boundary values, system IDs
- [ ] US-008 + US-010: i18n -- loader utils, error types, predefined errors, extractOrderedValues, selectPluralForm
- [ ] US-009: rules_format -- HTML detection, markdown passthrough, edge cases

**Context to Read:**
- requirements.md, section "Acceptance Criteria" for each US
- design.md, section "Verification Commands"
**Verification:**
```bash
cd /Users/divkix/GitHub/Alita_Robot && go test -v -race -count=1 ./alita/utils/string_handling/... ./alita/utils/errors/... ./alita/utils/keyword_matcher/... ./alita/utils/callbackcodec/... && make lint
```

---

## File Manifest

| Task | Files Touched |
|------|---------------|
| TASK-001 | `alita/utils/string_handling/string_handling_test.go` |
| TASK-002 | `alita/utils/errors/errors_test.go` |
| TASK-003 | `alita/utils/keyword_matcher/matcher_test.go` |
| TASK-004 | `alita/utils/callbackcodec/callbackcodec_test.go` |
| TASK-005 | `alita/config/types_test.go` |
| TASK-006 | `alita/utils/helpers/helpers_test.go` |
| TASK-007 | `alita/utils/chat_status/chat_status_test.go` |
| TASK-008 | `alita/i18n/i18n_test.go` |
| TASK-009 | `alita/modules/rules_format_test.go` |
| TASK-010 | None (verification only) |

**Total: 9 files (8 new, 1 modified). Zero production code changes.**

## Parallelization Matrix

All tasks TASK-001 through TASK-009 touch completely different files. There is zero file overlap between any pair of tasks. Therefore, all 9 implementation tasks can run in parallel. TASK-010 depends on all of them.

| Stream | Tasks | Files |
|--------|-------|-------|
| A | TASK-001 | `string_handling_test.go` (NEW) |
| B | TASK-002 | `errors_test.go` (NEW) |
| C | TASK-003 | `matcher_test.go` (NEW) |
| D | TASK-004 | `callbackcodec_test.go` (MODIFY) |
| E | TASK-005 | `types_test.go` (NEW) |
| F | TASK-006 | `helpers_test.go` (NEW) |
| G | TASK-007 | `chat_status_test.go` (NEW) |
| H | TASK-008 | `i18n_test.go` (NEW) |
| I | TASK-009 | `rules_format_test.go` (NEW) |
| J | TASK-010 | None (depends on A-I) |

## Risk Register

| Task | Risk | Mitigation |
|------|------|------------|
| TASK-003 | Aho-Corasick `Match()` returns pattern indices, not positions. Overlapping match count depends on library internals. | Read `findMatchesWithPositions` implementation carefully. The function does its own `strings.Index` loop to find all occurrences. Verify expected positions match actual behavior by running test first. |
| TASK-004 | Modifying existing test file could break existing tests if appended code has syntax errors. | Append only. Run existing tests first to confirm baseline passes. Never modify existing functions. |
| TASK-005 | Config `init()` may crash test runner in local dev. | Phase 2 task. Document that CI env vars are required. Verification command includes env vars. |
| TASK-006 | `ChunkKeyboardSlices` with chunkSize=0 causes infinite loop. | Do NOT test chunkSize=0. Add comment documenting known defect. |
| TASK-006 | Large task (13 functions). May take longer than estimated. | Functions are simple. Most are 1-5 line pure functions. Table-driven tests follow a repetitive pattern. |
| TASK-008 | `selectPluralForm` requires constructing `Translator` struct with unexported fields. | Internal package test (`package i18n`) has access. Construct minimal struct: `&Translator{langCode: "en", manager: &LocaleManager{defaultLang: "en"}}`. |
| TASK-008 | `validateYAMLStructure` imports `gopkg.in/yaml.v3`. Test may need YAML bytes. | Already in `go.mod`. Use simple `[]byte("key: value\n")` literals. No new dependency. |
| TASK-009 | `tgmd2html.MD2HTMLV2` output format may change between library versions. | Do NOT hardcode exact output. Use `strings.Contains` or verify output != input instead of exact string match. |
| TASK-010 | Phase 2 tests may fail locally due to missing PostgreSQL. | Expected. CI has PostgreSQL. Local verification only covers Phase 1. Phase 2 verified in CI. |
| ALL | Test files introduce lint warnings (unused imports, etc.). | Each task verification includes `make lint`. Fix warnings before marking complete. |

TASKS_COMPLETE
