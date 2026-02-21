---
title: Request Flow
description: How Telegram updates are processed through the Alita Robot pipeline.
---

This document details how Telegram updates flow through Alita Robot, from reception to response.

## Update Processing Pipeline

```
+-------------+     +----------------+     +------------+     +----------+
|  Telegram   | --> |  HTTP/Polling  | --> | Dispatcher | --> | Handlers |
|   Bot API   |     |    Receiver    |     |  (Router)  |     | (Modules)|
+-------------+     +----------------+     +------------+     +----------+
                                                |                   |
                                                |                   v
                                                |            +------+------+
                                                |            | Permission  |
                                                |            |   Checks    |
                                                |            +------+------+
                                                |                   |
                                                |                   v
                                                |            +------+------+
                                                |            |  Database   |
                                                |            |  / Cache    |
                                                |            +------+------+
                                                |                   |
                                                v                   v
                                          +----------+       +----------+
                                          |  Error   |       | Response |
                                          | Handler  |       |  to User |
                                          +----------+       +----------+
```

## Initialization Sequence

When the bot starts (`main.go`), it performs these steps in order:

### 1. Health Check Mode (Optional)
```go
if os.Args[1] == "--health" {
    // HTTP GET to /health, exit with status code
    os.Exit(0)
}
```

### 2. Panic Recovery Setup
```go
defer func() {
    if r := recover(); r != nil {
        log.Errorf("[Main] Panic recovered: %v", r)
        os.Exit(1)
    }
}()
```

### 3. Locale Manager Initialization
```go
localeManager := i18n.GetManager()
localeManager.Initialize(&Locales, "locales", i18n.DefaultManagerConfig())
```

### 4. OpenTelemetry Tracing Initialization
```go
tracing.InitTracing()
```

`tracing.InitTracing()` sets up distributed tracing with OTLP or console exporters based on environment configuration (`OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_SERVICE_NAME`, `OTEL_TRACES_SAMPLE_RATE`).

### 5. HTTP Transport Configuration
```go
httpTransport := &http.Transport{
    MaxIdleConns:        config.AppConfig.HTTPMaxIdleConns,
    MaxIdleConnsPerHost: config.AppConfig.HTTPMaxIdleConnsPerHost,
    IdleConnTimeout:     120 * time.Second,
    ForceAttemptHTTP2:   true,
}
```

### 6. Bot Client Creation
```go
b, err := gotgbot.NewBot(config.AppConfig.BotToken, &gotgbot.BotOpts{
    BotClient: &gotgbot.BaseBotClient{
        Client: http.Client{Transport: transport, Timeout: 30 * time.Second},
    },
})
```

### 7. Connection Pre-warming
```go
go func() {
    for i := 0; i < 3; i++ {
        b.GetMe(nil)  // Establish connection pool
        time.Sleep(100 * time.Millisecond)
    }
}()
```

### 8. Initial Checks
```go
alita.InitialChecks(b)  // Validates config, initializes cache
```

### 9. Async Processor (If Enabled)
```go
if config.AppConfig.EnableAsyncProcessing {
    async.InitializeAsyncProcessor()
}
```

### 10. Dispatcher Creation
```go
dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{
    Error:       errorHandler,
    MaxRoutines: config.AppConfig.DispatcherMaxRoutines,
})
```

The dispatcher uses a `TracingProcessor` wrapper for trace context propagation in polling mode, ensuring that OpenTelemetry spans are correctly associated with each update.

### 11. Monitoring Systems
```go
statsCollector = monitoring.NewBackgroundStatsCollector()
autoRemediation = monitoring.NewAutoRemediationManager(statsCollector)
activityMonitor = monitoring.NewActivityMonitor()
```

### 12. HTTP Server & Mode Selection
```go
httpServer := httpserver.New(config.AppConfig.HTTPPort)
httpServer.RegisterHealth()
httpServer.RegisterMetrics()

if config.AppConfig.UseWebhooks {
    httpServer.RegisterWebhook(b, dispatcher, secret, domain)
} else {
    updater.StartPolling(b, pollingOpts)
}
```

## Module Loading Order

After dispatcher creation, modules are loaded via `alita/main.go`:

```go
func LoadModules(dispatcher *ext.Dispatcher) {
    // Initialize help system first
    modules.HelpModule.AbleMap.Init()

    // Load help LAST (deferred) to collect all commands
    defer modules.LoadHelp(dispatcher)

    // Core modules (order matters for handler priority)
    modules.LoadBotUpdates(dispatcher)   // Bot status tracking
    modules.LoadAntispam(dispatcher)     // Spam protection
    modules.LoadLanguage(dispatcher)     // Language settings
    modules.LoadAdmin(dispatcher)        // Admin commands
    modules.LoadPin(dispatcher)          // Pin management
    modules.LoadMisc(dispatcher)         // Misc utilities
    modules.LoadBans(dispatcher)         // Ban/kick commands
    modules.LoadMutes(dispatcher)        // Mute commands
    modules.LoadPurges(dispatcher)       // Message purging
    modules.LoadUsers(dispatcher)        // User tracking
    modules.LoadReports(dispatcher)      // Report system
    modules.LoadDev(dispatcher)          // Developer tools
    modules.LoadLocks(dispatcher)        // Chat locks
    modules.LoadFilters(dispatcher)      // Message filters
    modules.LoadAntiflood(dispatcher)    // Flood protection
    modules.LoadNotes(dispatcher)        // Notes system
    modules.LoadConnections(dispatcher)  // Chat connections
    modules.LoadDisabling(dispatcher)    // Command disabling
    modules.LoadRules(dispatcher)        // Rules management
    modules.LoadWarns(dispatcher)        // Warning system
    modules.LoadGreetings(dispatcher)    // Welcome messages
    modules.LoadCaptcha(dispatcher)      // CAPTCHA verification
    modules.LoadBlacklists(dispatcher)   // Blacklist system
    modules.LoadMkdCmd(dispatcher)       // Markdown commands
}
```

## Handler Registration Pattern

Each module registers handlers using gotgbot's handler system:

```go
func LoadBans(dispatcher *ext.Dispatcher) {
    // Register module in help system
    HelpModule.AbleMap.Store(bansModule.moduleName, true)

    // Command handlers
    dispatcher.AddHandler(handlers.NewCommand("ban", bansModule.ban))
    dispatcher.AddHandler(handlers.NewCommand("kick", bansModule.kick))
    dispatcher.AddHandler(handlers.NewCommand("unban", bansModule.unban))

    // Callback query handlers
    dispatcher.AddHandler(handlers.NewCallback(
        callbackquery.Prefix("restrict."),
        bansModule.restrictButtonHandler,
    ))
}
```

### Handler Types

| Type | Registration | Trigger |
|------|--------------|---------|
| Command | `handlers.NewCommand("cmd", fn)` | `/cmd` messages |
| Callback | `handlers.NewCallback(filter, fn)` | Button presses |
| Message | `handlers.NewMessage(filter, fn)` | Text messages |
| ChatMember | `handlers.NewChatMemberUpdated(filter, fn)` | Member updates |

### Handler Groups

Handlers can be assigned to groups for priority control:

```go
// Negative group = higher priority (runs first)
dispatcher.AddHandlerToGroup(handler, -10)

// Group 0 = default
dispatcher.AddHandler(handler)  // Same as group 0

// Positive group = lower priority
dispatcher.AddHandlerToGroup(handler, 10)
```

## Permission Check Flow

Most admin commands follow this permission checking pattern:

```go
func (m moduleStruct) ban(b *gotgbot.Bot, ctx *ext.Context) error {
    chat := ctx.EffectiveChat
    user := ctx.EffectiveSender.User
    msg := ctx.EffectiveMessage

    // 1. Require group chat (not private)
    if !chat_status.RequireGroup(b, ctx, nil, false) {
        return ext.EndGroups
    }

    // 2. Require user to be admin
    if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id, false) {
        return ext.EndGroups
    }

    // 3. Require bot to be admin
    if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
        return ext.EndGroups
    }

    // 4. Check specific permission (restrict members)
    if !chat_status.CanUserRestrict(b, ctx, nil, user.Id, false) {
        return ext.EndGroups
    }

    // 5. Check bot has same permission
    if !chat_status.CanBotRestrict(b, ctx, nil, false) {
        return ext.EndGroups
    }

    // Proceed with ban logic...
}
```

### Permission Functions Reference

| Function | Purpose | When to Use |
|----------|---------|-------------|
| `RequireGroup` | Ensures chat is group/supergroup | Group-only commands |
| `RequirePrivate` | Ensures chat is private | PM-only commands |
| `RequireUserAdmin` | User must be admin | Admin commands |
| `RequireBotAdmin` | Bot must be admin | Commands needing bot admin |
| `RequireUserOwner` | User must be creator | Owner-only commands |
| `CanUserRestrict` | User can ban/mute | Ban/mute commands |
| `CanBotRestrict` | Bot can ban/mute | Ban/mute commands |
| `CanUserDelete` | User can delete messages | Purge commands |
| `CanBotDelete` | Bot can delete messages | Purge commands |
| `CanUserPin` | User can pin messages | Pin commands |
| `CanBotPin` | Bot can pin messages | Pin commands |
| `CanUserPromote` | User can promote/demote | Admin management |
| `CanBotPromote` | Bot can promote/demote | Admin management |
| `IsUserAdmin` | Check if user is admin | Conditional logic |
| `IsUserInChat` | Check if user is member | User validation |
| `IsUserBanProtected` | Check if user is protected | Before ban/kick |

## Response Patterns

### Handler Return Values

```go
// Stop processing, no more handlers run
return ext.EndGroups

// Continue to next handler in same group
return ext.ContinueGroups

// Error propagates to dispatcher error handler
return err
```

### Response Actions

```go
// Reply to the triggering message
msg.Reply(b, "Response text", &gotgbot.SendMessageOpts{
    ParseMode: helpers.HTML,
})

// Send new message to chat
b.SendMessage(chat.Id, "Message text", nil)

// Edit existing message
msg.EditText(b, "New text", nil)

// Delete message
msg.Delete(b, nil)

// Answer callback query
query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
    Text: "Notification text",
})
```

## Async Processing

Non-critical operations can be processed asynchronously:

```go
// Fire and forget (with panic recovery)
go func() {
    defer func() {
        if r := recover(); r != nil {
            log.Error("Panic in async operation")
        }
    }()

    // Async work here
}()

// With timeout protection
go func() {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    select {
    case <-time.After(2 * time.Second):
        // Do delayed work
    case <-ctx.Done():
        log.Warn("Operation timed out")
    }
}()
```

## Error Handling in Request Flow

### Dispatcher Error Handler

```go
Error: func(b *gotgbot.Bot, ctx *ext.Context, err error) ext.DispatcherAction {
    // 1. Recover from panics
    defer error_handling.RecoverFromPanic("DispatcherErrorHandler", "Main")

    // 2. Extract context for logging
    logFields := log.Fields{
        "update_id":  ctx.UpdateId,
        "error_type": fmt.Sprintf("%T", err),
    }

    // 3. Check for expected/suppressible errors
    if helpers.IsExpectedTelegramError(err) {
        log.WithFields(logFields).Warn("Expected Telegram API error")
        return ext.DispatcherActionNoop
    }

    // 4. Log the error
    log.WithFields(logFields).Error("Handler error")

    // 5. Continue processing other updates
    return ext.DispatcherActionNoop
}
```

### Common Error Patterns

```go
// Log and return error (propagates to dispatcher)
if err != nil {
    log.Error(err)
    return err
}

// Log but continue (non-fatal)
if err != nil {
    log.Warn("Non-fatal error:", err)
}

// Silent failure for expected cases
_, _ = msg.Delete(b, nil)  // Ignore delete errors
```

## Next Steps

- [Module Pattern](/architecture/module-pattern) - Creating new feature modules
- [Caching](/architecture/caching) - Redis cache integration
- [Project Structure](/architecture/project-structure) - File organization
