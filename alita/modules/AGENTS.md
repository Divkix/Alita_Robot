# alita/modules - Module Development Guidelines

**Scope:** Telegram bot command handlers, watcher modules, feature implementations

## WHERE TO LOOK

| Task | File | Notes |
|------|------|-------|
| Module Template | `helpers.go` | `moduleStruct`, `moduleEnabled` definitions |
| Handler Examples | `admin.go`, `bans.go` | Command handlers with permissions |
| Watcher Examples | `antiflood.go`, `filters.go` | Group 4-10 watchers, `ContinueGroups` |
| Callback Codec | `callback_codec.go` | Versioned callback encoding |
| Permissions | `chat_permissions.go` | Chat permission helpers |

## CONVENTIONS

### Module Structure
```go
var myModule = moduleStruct{
    moduleName:   "MyModule",
    handlerGroup: 0,  // -1=early, 0=standard, 4-10=watchers
}

func LoadMyModule(dispatcher *ext.Dispatcher) {
    // Register handlers
    dispatcher.AddHandlerToGroup(handlers.NewCommand("cmd", myModule.handler), myModule.handlerGroup)
    
    // Enable in help system
    HelpModule.AbleMap.Store(myModule.moduleName, true)
}
```

### Handler Return Values
- `return ext.EndGroups` — Stop propagation (commands)
- `return ext.ContinueGroups` — Continue (watchers/monitors)

### Handler Groups
| Group | Purpose | Example |
|-------|---------|---------|
| -1 | Early interception | Connection checks |
| 0 | Standard commands | Most modules |
| 4-10 | Watchers/monitors | antiflood(4), filters(9), blacklists(9) |

### Method Receivers
```go
// Value receiver, unnamed (typical)
func (moduleStruct) handler(b *gotgbot.Bot, ctx *ext.Context) error

// Named only when accessing fields
func (m moduleStruct) handlerWithFields(b *gotgbot.Bot, ctx *ext.Context) error
```

### Command Registration
```go
// Aliases
helpers.MultiCommand(dispatcher, []string{"ban", "dban"}, banHandler)

// Disableable commands
helpers.AddCmdToDisableable("ban")
```

### Nil Safety (CRITICAL)
```go
// ALWAYS check before accessing .User
if ctx.EffectiveSender == nil {
    return ext.EndGroups  // Channel message
}
```

## ANTI-PATTERNS

| Pattern | Why Wrong | Correct |
|---------|-----------|---------|
| `strings.Split(data, ".")` | Legacy, unsafe | `callbackcodec.Decode(data)` |
| Double callback answer | `RequireUserAdmin` already answers | Check return, don't answer again |
| Fire-and-forget DB | Loses errors | Synchronous or proper wrapper |
| Ignore `err` from DB ops | Nil pointer panics | Always check `err != nil` |
| `IsAnonymousChannel() \|\| !IsLinkedChannel()` | Matches almost everything | Test with multiple message types |

## CALLBACK DATA

Always use versioned codec:
```go
// Encode
data := callbackcodec.Encode("namespace", map[string]string{"id": "123"})
// Produces: "namespace|v1|id=123"

// Decode
decoded, err := callbackcodec.Decode(data)
```

## ADDING NEW MODULES

1. Create `alita/db/my_module_db.go` with operations
2. Implement `alita/modules/my_module.go` with handlers
3. Create `LoadMyModule(dispatcher)` function
4. Call from `alita/main.go:LoadModules()`
5. Add translation keys to ALL `locales/*.yaml` files
