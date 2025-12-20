# Large-Scale Code Quality Refactoring Plan

## Executive Summary

This plan addresses three major architectural issues:
1. **Eliminate Global State** - Remove 40+ global config variables and global DB
2. **Break Up God Files** - Split helpers.go (1684 lines), chat_status.go (1112 lines)
3. **Reduce Code Duplication** - Consolidate EnumFuncMaps, permission checks, greeting logic

## Phase 1: Eliminate Global Config Variables

### Problem
`config.go` has both a `Config` struct AND 40+ global variables that duplicate it. The init() function copies values from struct to globals. This creates:
- Two sources of truth
- Untestable code (globals can't be mocked)
- Race condition risks

### Solution
Remove all global variables except `AppConfig`. Update all consumers to use `config.AppConfig.FieldName`.

**Files to modify:**
- `alita/config/config.go` - Remove global vars (lines 108-193), keep only `AppConfig`
- All files that reference `config.BotToken`, `config.DatabaseURL`, etc.

**Estimated Impact:** ~50 files to update

---

## Phase 2: Database Connection Injection

### Problem
`db/db.go` has `var DB *gorm.DB` as a global. Every DB operation uses this directly.

### Solution
Create a `Database` struct wrapper that holds the connection and can be injected.

```go
// db/database.go
type Database struct {
    conn *gorm.DB
}

func NewDatabase(dsn string) (*Database, error) { ... }
func (d *Database) GetConnection() *gorm.DB { return d.conn }
```

For backward compatibility, keep a `GetDB()` function initially.

---

## Phase 3: Consolidate EnumFuncMaps Using Generics

### Problem
`helpers.go` has three nearly identical maps:
- `NotesEnumFuncMap` (lines 929-1214) - 285 lines
- `GreetingsEnumFuncMap` (lines 1219-1329) - 110 lines
- `FiltersEnumFuncMap` (lines 1333-1611) - 279 lines

All handle the same 8 media types (TEXT, STICKER, DOCUMENT, PHOTO, AUDIO, VOICE, VIDEO, VideoNote) with 95% identical code.

### Solution
Create a generic media sender:

```go
// utils/media/sender.go
type MediaContent struct {
    Text     string
    FileID   string
    MsgType  int
    Buttons  []db.Button
    Options  MediaOptions
}

type MediaOptions struct {
    NoFormat    bool
    NoNotif     bool
    WebPreview  bool
    IsProtected bool
    ReplyMsgId  int64
}

func SendMedia(b *gotgbot.Bot, chatID int64, content MediaContent, opts MediaOptions) (*gotgbot.Message, error)
```

**Lines saved:** ~500 lines

---

## Phase 4: Consolidate Permission Checking

### Problem
`chat_status.go` has 11 nearly identical permission functions:
- `CanUserChangeInfo()` (250-320)
- `CanUserRestrict()` (325-394)
- `CanUserPromote()` (451-520)
- `CanUserPin()` (564-619)
- `CanUserDelete()` (747-811)
- Plus 6 more `CanBot*` variants

Each has ~70 lines of identical structure:
1. Nil chat fallback
2. Anonymous admin check
3. Cache lookup
4. Permission check
5. Error messaging

### Solution
Create a generic permission checker:

```go
// utils/permissions/checker.go
type Permission int

const (
    PermChangeInfo Permission = iota
    PermRestrict
    PermPromote
    PermPin
    PermDelete
    // ...
)

func CheckUserPermission(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userID int64, perm Permission, justCheck bool) bool

func CheckBotPermission(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, perm Permission, justCheck bool) bool
```

**Lines saved:** ~400 lines

---

## Phase 5: Unify Welcome/Goodbye Logic

### Problem
`greetings.go` has `welcome()` and `goodbye()` functions that are 120+ lines each and nearly identical.

### Solution
Create a unified greeting handler:

```go
type GreetingType int

const (
    Welcome GreetingType = iota
    Goodbye
)

func (m moduleStruct) handleGreeting(bot *gotgbot.Bot, ctx *ext.Context, greetType GreetingType) error
```

Similarly unify:
- `setWelcome()` / `setGoodbye()`
- `resetWelcome()` / `resetGoodbye()`
- `cleanWelcome()` / `cleanGoodbye()`

**Lines saved:** ~300 lines

---

## Phase 6: Split God Files

### helpers.go (1684 lines) → Multiple packages:

```
utils/
├── keyboard/        # BuildKeyboard, ChunkKeyboardSlices, ConvertButtonV2ToDbButton
├── formatting/      # FormattingReplacer, SplitMessage, ReverseHTML2MD
├── media/           # Consolidated SendMedia (from EnumFuncMaps)
├── status/          # ExtractJoinLeftStatusChange, ExtractAdminUpdateStatusChange
└── message/         # GetNoteAndFilterType, GetWelcomeType, preFixes
```

### chat_status.go (1112 lines) → Split by responsibility:

```
utils/
├── permissions/     # All Can* functions (consolidated)
├── admin/           # IsUserAdmin, IsBotAdmin, admin cache helpers
└── chat/            # GetChat, CheckDisabledCmd
```

---

## Implementation Order

1. **Phase 1**: Config globals (foundation - needed for other changes)
2. **Phase 3**: EnumFuncMaps consolidation (standalone, high impact)
3. **Phase 4**: Permission checking consolidation (standalone, high impact)
4. **Phase 5**: Greeting logic unification (depends on Phase 3)
5. **Phase 6**: File splitting (cleanup, can be done incrementally)
6. **Phase 2**: DB injection (optional, lower priority)

---

## Commit Strategy

Make atomic commits after each sub-task:
1. `refactor(config): remove global config variables, use AppConfig consistently`
2. `refactor(helpers): consolidate EnumFuncMaps using generic media sender`
3. `refactor(chat_status): consolidate permission checking functions`
4. `refactor(greetings): unify welcome/goodbye handlers`
5. `refactor(helpers): split into focused packages`

---

## Risk Mitigation

1. **Run `make lint`** after each change to catch issues early
2. **Keep backward compatibility** where possible during transition
3. **Test with `make run`** periodically to verify functionality
4. **Commit frequently** to allow easy rollback

---

## Estimated Impact

| Metric | Before | After | Improvement |
|--------|--------|-------|-------------|
| Lines in helpers.go | 1684 | ~800 | -52% |
| Lines in chat_status.go | 1112 | ~400 | -64% |
| Lines in greetings.go | 1255 | ~700 | -44% |
| Global config variables | 40+ | 1 | -97% |
| Duplicate code lines | ~2000 | ~200 | -90% |
