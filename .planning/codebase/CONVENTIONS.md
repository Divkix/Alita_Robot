# Coding Conventions

**Analysis Date:** 2026-02-23

## Naming Patterns

**Files:**
- Go source files: `snake_case.go` (e.g., `notes_db.go`, `chat_status.go`, `background_stats.go`)
- Test files: `<source_file_name>_test.go` co-located with the file under test
- DB operation files: `<domain>_db.go` (e.g., `notes_db.go`, `filters_db.go`)

**Functions:**
- Exported: `PascalCase` (e.g., `GetNotes`, `AddNote`, `RequireUserAdmin`)
- Unexported: `camelCase` (e.g., `getNotesSettings`, `getAllChatNotes`, `skipIfNoDb`)
- DB helpers: `Get*`, `Add*`, `Update*`, `Delete*` prefix convention
- DB internals: lowercase `get*` / `getAll*` wrappers called by exported `Get*`
- Handler methods: lowercase on value receiver `(m moduleStruct)` (e.g., `dkick`, `ban`, `mute`)

**Variables:**
- Local: `camelCase` (e.g., `chatID`, `noteSrc`, `lockType`)
- ID fields: `chatID`, `userId`, `chatId` — ID capitalization varies between `ID` and `Id` depending on origin (gotgbot uses `Id`; internal code uses `ID`)
- Package-level vars: `camelCase` (e.g., `notesOverwriteMap`, `bansModule`)

**Types:**
- Structs: `PascalCase` (e.g., `moduleStruct`, `WrappedError`, `AdminCache`)
- Interface-implementing types: `PascalCase` verbs (e.g., `ButtonArray`, `StringArray`)
- GORM models: `PascalCase` with `Settings` suffix for config tables (e.g., `NotesSettings`, `LockSettings`)

**Constants:**
- ALL_CAPS only for message-type constants in `alita/db/db.go` (e.g., `TEXT`, `STICKER`, `DOCUMENT`)
- Other constants use camelCase or PascalCase depending on export level

**Module-level vars:**
- Module instance named `<name>Module = moduleStruct{...}` (e.g., `bansModule`, `HelpModule`)

## Code Style

**Formatting:**
- Standard `gofmt` (enforced by golangci-lint)
- Tabs for indentation (Go standard)

**Linting:**
- Tool: `golangci-lint` with config at `.golangci.yml`
- Enabled linters: `godox` (TODO/FIXME detection), `dupl` (duplicate code, threshold 100 lines)
- Max issues per linter: unlimited (`max-issues-per-linter: 0`)
- Run: `make lint`

## Import Organization

**Groups** (single block with blank-line grouping observed in module files):

1. Standard library (e.g., `"fmt"`, `"strings"`, `"time"`)
2. Third-party packages (e.g., `"github.com/PaulSonOfLars/gotgbot/v2"`, `log "github.com/sirupsen/logrus"`)
3. Internal packages (e.g., `"github.com/divkix/Alita_Robot/alita/db"`)

**Aliasing:**
- `log` is always aliased from logrus: `log "github.com/sirupsen/logrus"`
- Standard `errors` aliased as `stderrors` when the package also defines its own errors package: `stderrors "errors"`

**Path Aliases:**
- None (uses full module paths: `github.com/divkix/Alita_Robot/alita/...`)

## Error Handling

**Patterns:**
- Always check returned errors — never use `_` to silently discard them in production code
- DB layer: log with `log.Errorf("[Database][FunctionName]: ...")` then return safe default
- Log format: `[Layer][FunctionName]: %v - %d` (error first, ID second)
- Use `errors.Is(err, gorm.ErrRecordNotFound)` for not-found checks
- Use custom `errors.Wrap(err, msg)` / `errors.Wrapf(err, format, args...)` from `alita/utils/errors/` for errors that need call-site metadata (file, line, function)
- Handler functions return `ext.EndGroups` on failure, `ext.ContinueGroups` for monitoring/watcher handlers
- Panic recovery via `error_handling.RecoverFromPanic(funcName, modName)` using `defer`
- Four-layer recovery: dispatcher → worker pool → decorator → handler

**DB errors in tests:**
- Cleanup uses `_ = DB.Where(...).Delete(&Model{}).Error` (intentional discard only in test cleanup)

## Logging

**Framework:** `logrus` (imported as `log "github.com/sirupsen/logrus"`)

**Patterns:**
- `log.Errorf("[Layer][FunctionName]: %v - %d", err, id)` — DB layer errors
- `log.Warnf("[Layer][FunctionName]: ...")` — non-fatal conditions
- `log.WithFields(log.Fields{...}).Warning(...)` — structured fields for context (e.g., Redis retry attempts)
- `log.Info(...)` — operational events (cache flush, startup)
- Module/layer prefix always in brackets: `[Database]`, `[Cache]`, `[OptimizedLockQueries]`

## Comments

**When to Comment:**
- All exported functions and types have GoDoc comments
- Unexported functions used as internal helpers also have GoDoc-style comments explaining their purpose
- Inline comments used for non-obvious logic (e.g., `// Ensure chat exists before creating notes settings`)
- Section comments using `// ---------------------------------------------------------------------------` dividers in test files to group related tests visually

**GoDoc pattern:**
```go
// GetNotes returns the notes settings for the specified chat ID.
// This is the public interface to access notes settings.
func GetNotes(chatID int64) *NotesSettings {
```

**No `TODO`/`FIXME` comments** — godox linter actively enforces this.

## Function Design

**Handler functions:**
- Value receiver on `moduleStruct` — typically unnamed `(moduleStruct)`, named `(m moduleStruct)` when body accesses struct fields
- Signature: `func (m moduleStruct) handlerName(b *gotgbot.Bot, ctx *ext.Context) error`
- Early return pattern: permission checks at top, return `ext.EndGroups` immediately on failure
- Single responsibility: one handler per command behavior

**DB functions:**
- Public wrapper calls private implementation: `GetNotes(chatID)` calls `getNotesSettings(chatID)`
- Return pointer with `nil` for not-found (never panic)
- All DB functions accept `chatID int64` as first parameter

**Size:**
- DB operation files: 50–200 lines typical; no hard limit enforced
- Module files can be large (up to 1985 lines for `captcha.go`) — no splitting convention

**Parameters:**
- Context first: `b *gotgbot.Bot, ctx *ext.Context` for handlers
- Consistent ID naming: `chatID int64` for DB functions

**Return Values:**
- Pointers returned as `nil` on not-found or error
- Bool returned for existence checks (`DoesNoteExists`)
- Named return values used occasionally (e.g., `(noteSrc *NotesSettings)`)

## Module Design

**Exports:**
- Each module file exposes one `LoadXxx(dispatcher)` function called from `alita/main.go:LoadModules()`
- Module instances are package-level unexported vars (e.g., `var bansModule = moduleStruct{...}`)
- `HelpModule` is exported as it must be accessed by other modules

**DB packages:**
- One exported public API function per operation (no interface abstraction)
- Package-level `DB *gorm.DB` var accessed directly

**Barrel Files:**
- Not used. Each package exposes symbols directly.

---

*Convention analysis: 2026-02-23*
