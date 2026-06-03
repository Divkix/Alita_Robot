# Splitting helpers.go — Design Document

**Date:** 2026-06-02
**Status:** Draft
**Author:** Architecture Review

## Context

`alita/modules/helpers.go` is a 507-line file that serves as the shared everything file for all 25+ bot modules. It contains 6 distinct concerns mixed together:

1. **Module infrastructure** (`moduleStruct`, `moduleEnabled`)
2. **Temporary state storage** (`overwriteBase`, `overwriteFilter`, `overwriteNote`, `notesOverwriteMap`)
3. **Anti-spam types** (`spamKey`, `antiSpamInfo`, `antiSpamLevel`)
4. **Help system** (`listModules`, `initHelpButtons`, `getModuleHelpAndKb`, `sendHelpkb`, etc.)
5. **Deep-link router** (`startHelpPrefixHandler` — handles 6 different deep-link types)
6. **Global state** (`markup` help keyboard, `notesOverwriteMap` sync.Map)

This is the file every module touches. When a developer wants to understand the help system, they open a 500-line file that also contains antispam data structures and note overwrite logic. The file violates the principle of locality — change, bugs, and knowledge are scattered across unrelated concepts.

## Goal

Split `helpers.go` into focused, domain-coherent files within the `modules` package. Refactor the deep-link router and anonymous admin router to use a registry pattern, eliminating the manual if-else/switch routing.

## Approach

### 3 Phases

1. **Phase 1 — Pure file moves** (no behavior changes): Split `helpers.go` into 4-5 files
2. **Phase 2 — Registry pattern**: Extract deep-link router into a registry where each module registers its own handler
3. **Phase 3 — Reuse pattern**: Apply the same registry pattern to `bot_updates.go` anonymous admin handlers

### Why stay in the `modules` package?

- `moduleStruct` is a receiver tag used by all 25+ modules. Moving it to a sub-package would require touching every module file for zero behavioral gain.
- The `modules/` directory is flat — no precedent for sub-packages. Creating `modules/core/` would break from existing organization.
- Import cycle risk: `help.go` already defines `moduleEnabled` and `DefaultHelpRegistry()` which return `moduleStruct`.
- Tests are already in-package. No test structure changes needed.
- The goal is **locality**, not isolation. Splitting into 5 files of ~100 lines each fixes the cognitive load problem.

## Design

### Phase 1: Pure File Moves

**Delete:** `helpers.go`

**Create 4 files:**

#### `core.go` — Module Infrastructure
```go
type moduleStruct struct {
    moduleName        string
    handlerGroup      int
    permHandlerGroup  int
    restrHandlerGroup int
    defaultRulesBtn   string
    AbleMap           moduleEnabled
    AltHelpOptions    map[string][]string
    helpableKb        map[string][][]gotgbot.InlineKeyboardButton
}

// moduleEnabled (migrated from help.go)
type moduleEnabled struct {
    modules map[string]bool
    // ... methods
}

// DefaultHelpRegistry (migrated from help.go)
func DefaultHelpRegistry() *moduleStruct { ... }
```

#### `help_system.go` — Help Menu & Navigation
```go
var markup gotgbot.InlineKeyboardMarkup

func listModules() []string
func listModulesFrom(registry *moduleStruct) []string
func initHelpButtons()
func initHelpButtonsFrom(registry *moduleStruct) gotgbot.InlineKeyboardMarkup
func getModuleHelpAndKb(module, lang string, registry *moduleStruct) (string, gotgbot.InlineKeyboardMarkup)
func sendHelpkb(b *gotgbot.Bot, ctx *ext.Context, module string, registry *moduleStruct) (*gotgbot.Message, error)
func getAltNamesOfModule(moduleName string) []string
func getModuleNameFromAltName(altName string, registry *moduleStruct) string
func getHelpTextAndMarkup(ctx *ext.Context, module string, registry *moduleStruct) (string, gotgbot.InlineKeyboardMarkup, string)
```

#### `overwrite.go` — Temporary State Storage
```go
var notesOverwriteMap sync.Map

type overwriteBase struct {
    ChatID   int64
    ItemName string
    Text     string
    FileID   string
    Buttons  []db.Button
    DataType int
}

type overwriteFilter struct {
    overwriteBase
}

type overwriteNote struct {
    overwriteBase
    pvtOnly     bool
    grpOnly     bool
    adminOnly   bool
    webPrev     bool
    isProtected bool
    noNotif     bool
}
```

#### `antispam.go` — Merge into existing file
```go
type spamKey struct {
    chatId int64
    userId int64
}

type antiSpamInfo struct {
    Levels []antiSpamLevel
}

type antiSpamLevel struct {
    Count    int
    Limit    int
    CurrTime time.Time
    Expiry   time.Duration
    Spammed  bool
}
```

### Phase 2: Deep Link Router Registry

#### The Problem

`startHelpPrefixHandler` is a 246-line if-else chain that imports `db/connections`, `db/rules`, `db/notes`, `utils/chat_status`, `utils/media`, `utils/keyboard`. This is not a helper — it's a router that knows about 5 different domains.

#### The Interface

```go
// deeplink_router.go
type DeepLinkHandler func(b *gotgbot.Bot, ctx *ext.Context, user *gotgbot.User, arg string) error

var deepLinkRegistry = make(map[string]DeepLinkHandler)

func RegisterDeepLinkHandler(prefix string, handler DeepLinkHandler)
func HandleDeepLink(b *gotgbot.Bot, ctx *ext.Context, user *gotgbot.User, arg string) error
```

#### Registration Points

| Module | File | Prefix | Handler |
|--------|------|--------|---------|
| Help | `help.go` | `help_` | `helpDeepLinkHandler` |
| Connections | `connections.go` | `connect_` | `connectDeepLinkHandler` |
| Rules | `rules.go` | `rules_` | `rulesDeepLinkHandler` |
| Notes | `notes.go` | `notes_` | `notesListDeepLinkHandler` |
| Notes | `notes.go` | `note_` | `noteDeepLinkHandler` |
| Help | `help.go` | `about` | `aboutDeepLinkHandler` |

#### Call Site Changes

**`help.go` line 435:**
```go
// Before
err := startHelpPrefixHandler(b, ctx, user, args[1])

// After
err := HandleDeepLink(b, ctx, user, args[1])
```

**Each module's `init()`:**
```go
// In help.go init()
RegisterDeepLinkHandler("help_", helpDeepLinkHandler)

// In connections.go init()
RegisterDeepLinkHandler("connect_", connectDeepLinkHandler)

// etc.
```

**Tests:** `helpers_test.go` tests become `deeplink_router_test.go` — testing the registry, prefix matching, and each handler separately.

### Phase 3: Anonymous Admin Router Registry

#### The Problem

`bot_updates.go` (lines 215-290) has a massive switch statement:
```go
switch command {
case "promote":
    return adminModule.promote(c)
case "ban":
    return bansModule.ban(b, ctx)
// ... 20+ cases
}
```

This is the same structural problem as `startHelpPrefixHandler` — manual routing with global module variables.

#### The Interface

```go
// anonymous_admin_router.go
type AnonymousAdminHandler func(b *gotgbot.Bot, ctx *ext.Context) error

var anonAdminRegistry = make(map[string]AnonymousAdminHandler)

func RegisterAnonymousAdminHandler(command string, handler AnonymousAdminHandler)
func HandleAnonymousAdmin(b *gotgbot.Bot, ctx *ext.Context, command string) error
```

#### Registration Points

| Module | Commands | Handler |
|--------|----------|---------|
| Admin | `promote`, `demote`, `settitle` | `adminModule.promote`, `adminModule.demote`, etc. |
| Bans | `ban`, `dban`, `sban`, `tban`, `unban`, `restrict`, `unrestrict` | `bansModule.ban`, `bansModule.dBan`, etc. |
| Mutes | `mute`, `smute`, `dmute`, `tmute`, `unmute` | `mutesModule.mute`, etc. |
| Pins | `pin`, `unpin`, `permapin`, `unpinall` | `pinsModule.pin`, etc. |
| Purges | `purge`, `del` | `purgesModule.purge`, `purgesModule.delCmd` |
| Warns | `warn`, `swarn`, `dwarn` | `warnsModule.warnUser`, etc. |

#### Call Site Changes

**`bot_updates.go` lines 215-290:**
```go
// Before
func verifyAnonymousAdmin(b *gotgbot.Bot, ctx *ext.Context, command string) error {
    switch command {
    case "promote":
        return adminModule.promote(c)
    // ... 20+ cases
    }
}

// After
func verifyAnonymousAdmin(b *gotgbot.Bot, ctx *ext.Context, command string) error {
    return HandleAnonymousAdmin(b, ctx, command)
}
```

**Each module's `init()`:**
```go
// In bans.go init()
RegisterAnonymousAdminHandler("ban", bansModule.ban)
RegisterAnonymousAdminHandler("dban", bansModule.dBan)
// etc.
```

## Benefits

### Locality
- Change to the help system no longer requires opening a file that also contains antispam data structures.
- A bug in admin caching lives in one file.
- Permission composition logic lives in one place.

### Leverage
- The `permission` seam provides a small interface ("can this user do X?") with deep implementations.
- New modules register their deep link handler and anonymous admin handler instead of editing central routers.
- One unified pattern for command routing across the codebase.

### Testability
- Each extracted file can be tested in isolation.
- The deep link router can be tested with mock handlers.
- The anonymous admin router can be tested with mock handlers.
- `moduleEnabled` can be tested without loading the entire modules package.

### Deletion Test
- Deleting `helpers.go` concentrates complexity into focused files.
- Deleting the deep link router doesn't break call sites — they just use `HandleDeepLink`.
- Deleting the anonymous admin router doesn't break call sites — they just use `HandleAnonymousAdmin`.

## Migration Plan

### Phase 1 (No behavior changes)
1. Create `core.go`, `help_system.go`, `overwrite.go`
2. Move `antispam` types into existing `antispam.go`
3. Migrate `moduleEnabled` and `DefaultHelpRegistry` from `help.go` to `core.go`
4. Delete `helpers.go`
5. Run tests: `go test ./alita/modules/...`
6. Run lint: `make lint`

### Phase 2 (Registry pattern)
1. Create `deeplink_router.go` with `RegisterDeepLinkHandler` and `HandleDeepLink`
2. Extract each deep link handler from `startHelpPrefixHandler` into its module file
3. Add registration calls in each module's `init()`
4. Update `help.go` to call `HandleDeepLink`
5. Migrate tests from `helpers_test.go` to `deeplink_router_test.go`
6. Run tests: `go test ./alita/modules/...`
7. Run lint: `make lint`

### Phase 3 (Anonymous admin router)
1. Create `anonymous_admin_router.go` with `RegisterAnonymousAdminHandler` and `HandleAnonymousAdmin`
2. Add registration calls in each moderation module's `init()`
3. Replace switch statement in `bot_updates.go` with `HandleAnonymousAdmin` call
4. Run tests: `go test ./alita/modules/...`
5. Run lint: `make lint`

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| Breaking module initialization order | Phase 1 is pure file moves — no init() changes. Registry registrations happen in existing init() functions. |
| Import cycle when moving `moduleEnabled` | `moduleEnabled` stays in `modules` package. `help.go` already imports `modules` (it's in the same package). |
| Test failures from file moves | Tests use same package. No import changes needed. Run `go test` after each phase. |
| Merge conflicts with concurrent work | Each phase is a small, focused PR. Phase 1 is the largest but only touches one file. |

## Success Criteria

- `helpers.go` is deleted
- No module file has >200 lines of unrelated concerns
- Adding a new deep link type requires 0 changes to `deeplink_router.go`
- Adding a new anonymous admin command requires 0 changes to `anonymous_admin_router.go`
- All existing tests pass
- Test coverage for `modules` package does not decrease

## Dependencies

- No new dependencies
- No external library changes
- No database schema changes

## Out of Scope

- Converting `moduleStruct` to an interface (this is a separate candidate from the architecture review)
- Moving to sub-packages (deliberately scoped out — see "Why stay in the modules package?")
- Changing any module logic or behavior
- Adding new features

## Related Work

- **Candidate B:** `chat_status.go` god object — can be tackled after this refactor
- **Candidate D:** `bot_updates.go` anonymous admin router — directly addressed by Phase 3
- **Candidate C:** Duplicate command pipelines — `moderation.go` vs `command_pipeline.go` should be unified after this refactor provides the registry pattern template

---

*This document follows the architecture review vocabulary: Module, Interface, Depth, Seam, Adapter, Leverage, Locality.*
