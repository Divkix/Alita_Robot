# Research: gotgbot v2 Multi-Bot Single-Process — Dispatcher/Updater per Token (#803)

> Branch: `research/gotgbot-multi-bot` · Part of #801 · Grilling 2026-08-08 locked (same-process shared tables + `bot_id`/`owner_id`, namespaced cache `alita:{module}:{bot_id}:{id}`, per-user cap 1–2 + global cap + 1h Redis cooldown, DM-only `/clone` with delete+`getMe`, minimal lifecycle + revoked detection) · 2026-08-08  
> Scope: `main.go` (`newBotAPITransport`, `newConfiguredDispatcher`, `postInit`, `shutdownManager`), `alita/main.go` (`LoadModules`/`LoadAllModules`), `alita/utils/httpserver/*`, `alita/modules/registry.go`, `alita/utils/cache/*`, `go.mod` `gotgbot v2.0.0-rc.35` · `ctx7` `/paulsonoflars/gotgbot` + local mod source `v2@v2.0.0-rc.35`

## 1. Feasibility — Yes

**Answer: yes, N bots in one Go process sharing the same binary is a first-class gotgbot v2 pattern.** No sidecar process per clone is needed.

Evidence (`ext/updater.go`, `ext/botmapping.go`, `ext/dispatcher.go` at `v2@v2.0.0-rc.35`):

- `gotgbot.NewBot(token, &gotgbot.BotOpts{BotClient: &gotgbot.BaseBotClient{...}})` is stateless per token. `Bot` is a plain struct `{Token, User, BotClient}`. `NewBot` calls `getMe` to verify/populate `User`, then returns. Reusable `*http.Transport` pointer is shared safely (the `http.Client` is copied by value in `BaseBotClient`; pooling survives because `Transport` is a pointer — see `main.go:118` comment). You can call `NewBot` N times with different tokens using the same transport.
- `ext.Updater` owns a `botMapping {mapping map[token]botData, urlMapping map[path]token, mux sync.RWMutex}`. Each `botData` has `bot *gotgbot.Bot`, `updateChan chan json.RawMessage`, `pollingContextCloser`, `updateWriterControl *sync.WaitGroup`, `urlPath`, `webhookSecret`, `stopUpdates chan struct{}`. Polling and webhook bots share this map.
  - `StartPolling(b, opts)` → `botMapping.addPollingBot(b, cancel)` → `go dispatcher.Start(b, bData.updateChan)` + `go pollingLoop(ctx, bData, ...)`. Each call adds one entry; many polls can run concurrently.
  - `AddWebhook(b, urlPath, opts)` → `botMapping.addWebhookBot(b, urlPath, secret)` → `go dispatcher.Start(b, bData.updateChan)`. `StartServer(opts)` listens once and routes via `GetHandlerFunc("/")` → `botMapping.getHandlerFunc` → `getBotFromURL(r.URL.Path)` → `updateChan <- bytes`. `SetAllBotWebhooks(domain, opts)` iterates `getBots()` and calls `bot.SetWebhook(domain+path)` per bot.
  - `StopBot(token)` / `StopAllBots()` → `botData.stop()` (close `stopUpdates`, cancel polling ctx, `Wait` writers, `close(updateChan)`).
  - Guards: `ErrBotAlreadyExists` (duplicate token), `ErrBotUrlPathAlreadyExists` (duplicate `urlPath`).

So the library **was designed for multi-bot** on one `Updater`/one `Dispatcher`. The minimal alternative — one `Updater` + one `Dispatcher` + N `Bot`s each with its own `updateChan` — is idiomatic gotgbot. The per-Bot seam below keeps Alita simple by giving each clone its own `Dispatcher`/`Updater` pair instead of sharing one, for reasons in §3.

## 2. Per-Bot Seam — One Bot + One Dispatcher + One Updater

### 2.1 What the seam is

- `gotgbot.Bot` — identity + transport (token-scoped, cheap, no goroutines).
- `ext.Dispatcher` — handler registry + `limiter chan struct{}` + `waitGroup` + `Processor`. Owns `handlers handlerMapping` (groups → `[]Handler`). `AddHandler`/`AddHandlerToGroup` mutate this map. `ProcessUpdate(b, u, data)` iterates groups ascending, `CheckUpdate`→`HandleUpdate`, `Error`/`Panic` hooks, `Processor` hook. `Start(b, updates <-chan json.RawMessage)` loops the channel, enforces `limiter`, spawns goroutine per update that calls `processRawUpdate(b, raw)`.
- `ext.Updater` — transport. Either `pollingLoop` (getUpdates long-poll, offset tracking) or HTTP webhook handler. Holds `Dispatcher UpdateDispatcher` and `botMapping`.

Polling vs webhook:

- Polling: `updater.StartPolling(bot, &PollingOpts{DropPendingUpdates, GetUpdatesOpts{AllowedUpdates, Timeout}})` — blocks only to create goroutines, then `Idle()` parks main. Each clone needs its own `Updater` if doing polling (one `getUpdates` loop per token). `DeleteWebhook` before polling is required (`main.go:309`).
- Webhook: `updater.AddWebhook(bot, "/webhook/<bot_id>", &AddWebhookOpts{SecretToken})` + `updater.StartServer(WebhookOpts{ListenAddr, CertFile/KeyFile, ReadTimeout})` + `updater.SetAllBotWebhooks(domain, &gotgbot.SetWebhookOpts{AllowedUpdates,...})`. `GetHandlerFunc` routes by path. Custom `httpserver.Server` can proxy to this handler or stay as Alita's single `/webhook` path (see §4).

### 2.2 Handler registration is per-Dispatcher, not per-Bot

`ext.Dispatcher` is Bot-agnostic storage; `Bot` is only passed at dispatch time (`ProcessUpdate(b, u)`). Concretely:

- `alita/main.go:LoadModules(d)` → `modules.ResetHelpRegistry()` → `defer modules.LoadHelp(d)` → `modules.LoadAllModules(d)` which stable-sorts `registry []registeredModule{name, priority, load func(*ext.Dispatcher)}` ascending and calls each loader as `load(dispatcher)`. Those loaders do `dispatcher.AddHandler(...)` / `AddHandlerToGroup(..., group)` and set `DefaultHelpRegistry().AbleMap[name]=true`.
- Priority groups observed: `-10` captcha pending, `-5` antiraid, `-1` users tracker, `0` commands, `4` antiflood, `8` reports, `9` filters, `10` pins etc. See `alita/modules/*` `LoadXxx` and `AGENTS.md:Handler Priority`.
- Dispatch semantics: groups processed ascending; within a group first matching handler runs then `break` to next group unless it returns `ContinueGroups` (stay in group) or `EndGroups` (stop all). The `Bot` pointer seen by `handler.HandleUpdate(b, ctx)` is the one from `Dispatcher.Start(b, ch)` or `dispatcher.ProcessUpdate(bot, update)`.

**Therefore modules must be re-registered per Dispatcher.** If you give clone A its own `ext.NewDispatcher(opts)` you must call `LoadModulesFor(dispatcherA)` so it has its own `handlers` map. Reusing the primary dispatcher's handler map for clones would be incomplete; sharing a single `Dispatcher` is an alternative (see §2.3).

### 2.3 Two viable topologies

| # | `Updater` | `Dispatcher` | `Bot` | Handler registration | Isolation | Trade |
|---|-----------|--------------|-------|----------------------|-----------|-------|
| **M1 — per-clone pair (recommended)** | 1 per clone | 1 per clone | 1 per clone | `LoadModulesFor(dispatcher)` called once per dispatcher | Max: `limiter`, `waitGroup`, `handlers`, error/panic hooks isolated; no `handlers` mutation race across clones | Slightly more goroutines/memory (one `limiter` chan per clone, one polling loop per clone). Simpler reasoning; fits Alita's locked shared-DB + namespaced cache model (§3). |
| M2 — shared singletons | 1 total | 1 total | N | Load once; all bots feed same channel via `addPollingBot`/`addWebhookBot` | Minimal: one `limiter`/`waitGroup` shared, handlers shared. But cross-bot `limiter` contention and global-state hazards (§3) are harder to isolate. | Cheaper, but hazards coalesce; stop/restart of one bot touches shared `waitGroup`. |

Both are “yes + seam” per ticket phrasing. This doc recommends **M1** for Alita clones (one Bot + one Dispatcher + one Updater per clone token) because the codebase already has global-state hazards that benefit from per-clone `limiter`/`handlers` isolation and because `manager` lifecycle (add/remove clone without affecting primary) is simpler when `Stop()` is per-dispatcher.

### 2.4 Code snippet — per-Bot seam

Reuse existing `newBotAPITransport`/`resolveBotAPIURL`/`newConfiguredDispatcher`/`resolveBotUsername`/`postInit` pattern from `main.go:355-490`.

```go
// Transport is shared process-wide — same pointer seen in gotgbot/v2 #801 notes.
// New in alita/clones.Manager.
type cloneInstance struct {
    BotID      int64
    Username   string
    Bot        *gotgbot.Bot
    Dispatcher *ext.Dispatcher
    Updater    *ext.Updater
    cancel     context.CancelFunc // not needed for ext.Updater; kept if custom loops added
}

func newCloneInstance(token, apiServer string, transport *http.Transport, maxRoutines int) (*cloneInstance, error) {
    // Same construction as main.go:124-136
    b, err := gotgbot.NewBot(token, &gotgbot.BotOpts{
        BotClient: &gotgbot.BaseBotClient{
            Client: http.Client{Transport: transport, Timeout: constants.LongTimeout},
            DefaultRequestOpts: &gotgbot.RequestOpts{
                Timeout: time.Duration(constants.LongTimeout),
                APIURL:  resolveBotAPIURL(apiServer),
            },
        },
    })
    if err != nil {
        // 401/400 → invalid/revoked token (see research/clone-token-security.md)
        return nil, err
    }
    username := resolveBotUsername(b)

    dispatcher := newConfiguredDispatcher(maxRoutines) // ext.NewDispatcher with TracingProcessor + dispatcherErrorHandler

    // Re-entrant variant — see §5: no global ResetHelpRegistry().
    LoadModulesFor(dispatcher)

    // Optional: per-clone startup side-effects isolated
    // postInitForClone(b, dispatcher, username) // sets commands, sends startup to clone's log group, starts captcha lifecycle per-bot

    return &cloneInstance{BotID: b.Id, Username: username, Bot: b, Dispatcher: dispatcher}, nil
}

// Polling mode
func (c *cloneInstance) StartPolling(updater *ext.Updater) error {
    c.Updater = updater // ext.NewUpdater(c.Dispatcher, nil) — M1
    // Delete stale webhook first (idempotent)
    _, _ = c.Bot.DeleteWebhook(&gotgbot.DeleteWebhookOpts{DropPendingUpdates: true})
    return c.Updater.StartPolling(c.Bot, &ext.PollingOpts{
        DropPendingUpdates: true,
        GetUpdatesOpts: &gotgbot.GetUpdatesOpts{AllowedUpdates: config.AppConfig.AllowedUpdates},
    })
}

// Webhook mode — M1
func (c *cloneInstance) AddWebhook(updater *ext.Updater, domain, secret string) error {
    // URL path is namespaced — prevents ErrBotUrlPathAlreadyExists
    urlPath := fmt.Sprintf("/webhook/clone/%d", c.BotID)
    c.Updater = updater // ext.NewUpdater(c.Dispatcher, nil)
    if err := c.Updater.AddWebhook(c.Bot, urlPath, &ext.AddWebhookOpts{SecretToken: secret}); err != nil {
        return err
    }
    // Caller aggregates all addWebhook calls then calls updater.SetAllBotWebhooks(domain, opts)
    // or per-bot c.Bot.SetWebhook(domain+urlPath, &gotgbot.SetWebhookOpts{SecretToken: secret, AllowedUpdates: ...})
    return nil
}

func (c *cloneInstance) Stop() error {
    // ext.Updater.Stop() stops its dispatcher + polling loops + webhook server.
    // For per-clone updaters, stop the clone's updater; main bot's updater stays up.
    if c.Updater != nil {
        _ = c.Updater.StopBot(c.Bot.Token) // stops that bot's polling loop & channel
        c.Dispatcher.Stop()                // drains waitGroup
    }
    return nil
}
```

`LoadModulesFor` is the re-entrant extraction of `alita/main.go:LoadModules`:

```go
// alita/modules — rename current LoadModules body to loadModulesInto
func LoadModulesFor(d *ext.Dispatcher) {
    // Do NOT call ResetHelpRegistry() here — see §3.1
    defer LoadHelp(d)
    LoadAllModules(d) // sorted by priority, calls each RegisterLegacyModule loader with this dispatcher
}
```

Primary bot keeps `LoadModules(d)` (which does `ResetHelpRegistry()` once at boot before any clone is created). Clones call `LoadModulesFor`.

## 3. Reuse Hazards — What Breaks If We Share a Single Dispatcher / Global Maps

Sharing a single `Dispatcher` for many bots is *functionally* correct for routing (gotgbot supports N Bots on one Dispatcher, see `botmapping.addWebhookBot` calling `go dispatcher.Start(b, ch)` per bot). Alita hazards are at the **application layer** — global maps and Redis key collisions.

### 3.1 `AbleMap` / `DefaultHelpRegistry` singleton (`alita/modules/core.go`, `registry.go`, `alita/main.go`)

- Structure: package-global `defaultHelpRegistry = &moduleStruct{AbleMap map[string]bool, helpableKb, AltHelpOptions, ...}` guarded by `ableMapMu sync.RWMutex`. Each loader does `DefaultHelpRegistry().AbleMap["T"] = true` (no lock per-loader; safe only because `LoadModules` is single-threaded at startup). Readers via `GetAbleMap()` take `RLock` and copy; `help.go` readers do likewise.
- `alita/main.go:LoadModules` → `ResetHelpRegistry()` **wipes** all three maps under lock.
- Hazard if reusing single dispatcher: **none** beyond lifecycle — just don't call `ResetHelpRegistry()` after boot. If using M1 (per-clone dispatcher), the naive `LoadModulesFor` that calls `ResetHelpRegistry()` would wipe the primary's map and concurrent clones' entries. **Fix**: split `LoadModules` into `Reset + LoadModulesFor` (once) and `LoadModulesFor` (no reset) for clones. Multiple `LoadModulesFor` calls concurrently must not race on `AbleMap` writes: each loader writes same keys (`"Admin":true`), so concurrent writes of identical value under `ableMapMu` would need the per-write lock `SetAbleMap` style. Current loaders write the map directly without `Lock`; concurrent clone startup could race. Easiest safe path: serialize clone `LoadModulesFor` under a global `loadMu sync.Mutex`, or make loaders use `SetAbleMap`/`ableMapMu` consistently. Mutation is idempotent; readers already `RLock`.
- Higher-level: `AbleMap` is feature-enable list, not bot-scoped — all bots have same modules enabled. Namespacing `AbleMap` per bot_id is unnecessary; the locked decision says “every clone has *every* feature”. Keep one global map.

### 3.2 `floodMu` + `syncHelperMap` (`alita/modules/antiflood.go`)

- `var floodMu sync.Map` // `map[floodKey]*sync.Mutex` global — per-key `*sync.Mutex` protecting Load→mutate→Store RMW on `antifloodModule.syncHelperMap`.
- `antifloodModule syncHelperMap sync.Map` — global `map[floodKey]floodControl{userId, messageCount, messageIDs, lastActivity}` with a 5-min `cleanupLoop` (deletes entries idle >600s, with `TryLock` on `floodMu` entry).
- `floodKey {chatId, userId}` — **not namespaced by `bot_id`**. Current keys are per chat-user across the single bot. In multi-bot same-process, two different bots seeing the same `(chat,user)` pair (if they share a group, or a user is in groups with different bots) would **cross-contaminate** flood counts and mutexes. Even if clones join disjoint groups, conceptually the count is wrong to share — clone A's spam in chat X should not count against primary's view of X.
- `adminCheckSemaphore chan struct{}(50)` and `maxConcurrentMsgDeletions=5` are per-`antifloodModule` (global). Shared across bots — contention but not correctness issue; still, per-bot `limiter` from Dispatcher already gates concurrency.
- Hazard if single shared dispatcher: **amplified** — polling loops of many bots feed same `syncHelperMap`, interleaving RMWs on same key. With per-clone dispatcher, handlers are still the *same Go functions* closing over the same global `floodMu`/`syncHelperMap` — so per-clone dispatcher **does not fix** the key-collision bug; it only isolates `limiter`. **Fix required regardless of topology**: namespace keys as `type floodKey struct{ botId, chatId, userId int64 }` or at least `{chatId, userId, botId}` and update `updateFlood`/`cleanupOnce` accordingly. Cache/DB antiflood settings already per-chat (`antiflood_settings`/`GetAntifloodSettingsCached` keyed by `chat_id`); DB rows will also need `bot_id` per locked schema `bot_id`/`owner_id` — but the in-memory flood counter is separate from DB settings, so immediate fix is key namespacing plus audit of `cleanupOnce` `floodMu.Delete` to match new key.
- `ponytail: global floodMi leaked` — small ceiling: per-clone dispatcher without key fix leaks flood counts across bots sharing a chat; per-bot `limiter` does not bound `floodMu` growth. Upgrade: namespaced key.

### 3.3 Admin cache + restricted cache (`alita/utils/cache/*`)

- `adminCacheKey = "alita:adminCache:%d" % chatId`, TTL 30m, negative cache 2m / 30s. `LoadAdminCache` stores `AdminCache{ChatId, UserInfo, UserMap, Cached}`. `IsChatRestricted` stores `alita:restricted:%d` (TTL 30m) + `alita:restricted_probe:%d`. Generic `cache.CacheKey("antiflood", chatId)`, `"blacklist"`, `"captcha_settings"`, `"approvals"` etc. (`ttl.go`).
- All keys today are **chat-scoped, not bot-scoped**. In shared-tables design per #801/806, the locked cache namespace is `alita:{module}:{bot_id}:{id}`. Current code still writes single-tenant keys, so hazard: clone writing `alita:adminCache:123` overwrites primary's admin view of chat 123; restricted flag set by clone silences primary's sends to chat 123, and negative-cache for “bot not admin” from one bot poisons the other (bot A's non-admin status cached → bot B thinks it is not admin).
- Hazard matrix:
  - Shared dispatcher + shared cache: **high** — all bots read/write same Redis keys, interleaving stores. `LoadAdminCache` checks `b.GetChatMember(b.Id)` (bot's own membership) then `getChatAdministrators`; two bots with different admin states in same chat will thrash each other's cache.
  - Per-clone dispatcher does not fix cache key collision — still same Redis cluster, same key strings.
- **Fix (locked)**: namespace Redis keys with `bot_id` as `fmt.Sprintf("alita:adminCache:%d:%d", botId, chatId)` etc., or prefix `alita:{botId}:adminCache:{chatId}` to match `alita:{module}:{bot_id}:{id}`. Change `adminCacheKey`/`restrictedChatKey` signatures to take `botId`. During cutover, keep **read-migration**: try namespaced key, miss→ try legacy `alita:adminCache:{chat}` for one TTL window, then write back namespaced. Also update `cache.CacheKey` callers (antiflood `cache.CacheKey("antiflood", chatID)` etc.) to `cache.CacheKey("antiflood", botID, chatID)` — or introduce `CacheKeyFor(botId, module, id)`.
- Singleflight: gocache's `Manager.Get` + `syncHelperMap` do per-key singleflight at Redis layer only if configured; otherwise concurrent `LoadAdminCache` per chat may fan-out `getChatAdministrators` N times (3 retries). Namespacing spreads load; also add `singleflight.Group` per namespaced key if needed.

### 3.4 Handler-group ordering & `ContinueGroups`/`EndGroups`

- Global group order is shared logic; with per-clone dispatcher the order is cloned identically (loaders called with same priority). With single dispatcher, order is exactly the same. No hazard beyond: returning `ContinueGroups` vs `EndGroups` is evaluated per update against one `Bot`'s handlers — correct whether sharded or shared.
- `ext.Dispatcher` `Processor` (here `tracing.TracingProcessor`) is per-Dispatcher; per-clone dispatcher lets spans carry `bot_id` attribute. Shared dispatcher would need `bot_id` injected from `b.Id` in context data, not dispatcher construction — doable but noisier.

### 3.5 Other global-state spots to audit (non-exhaustive)

- `chat_status.IsApproved` / `approvals.IsUserApproved` — per-chat whitelist; needs `bot_id` scoping (linked to restricted keys).
- `antiraid` Redis state `alita:antiraid:state:{chat}` — already Redis, also needs namespacing.
- `captcha` pending maps and `StartCaptchaLifecycle` — today one lifecycle per process; with clones need per-bot captcha state or per-bot lifecycle (each `captchaModule` would need bot-scoped maps or separate instance).
- `backups/backup.go` cache invalidation `cacheKey("antiflood", chatID)` etc. — same key hazard.
- Stats collectors / `monitoring` `GlobalCollector`, `activityMonitor` — global; per-clone metrics need `bot_id` label, not separate collectors.
- Any `sync.Once` in modules (e.g., antiraid expiry poller `modules.StopAntiRaidExpiryPoller()`, `StopCaptchaLifecycle()`) — single registration shared; per-clone start/stop must not stop global poller for other bots.

**Summary**: `Dispatcher` handles routing correctly for N Bots even when shared. The breakage is **key collisions and in-memory maps that lack `bot_id`**. Per-clone `Dispatcher` isolates `limiter`/`handlers` but does **not** by itself fix `floodMu`/`syncHelperMap`/`adminCache`/`restrictedCache`/`antiraid`/`captcha` sharing. Those need explicit namespacing/`bot_id` threading (§5).

## 4. Webhook Multiplexing Options — Single Path Today → N Bots

Today (`alita/utils/httpserver/server.go`):

- `Server{bot *gotgbot.Bot, dispatcher *ext.Dispatcher, secret string, webhookEnabled bool}` — one bot, one dispatcher.
- `RegisterWebhook(bot, dispatcher, secret, domain)` → `mux.HandleFunc("/webhook", webhookHandler)` + `bot.SetWebhook(domain+"/webhook", &gotgbot.SetWebhookOpts{SecretToken: secret, AllowedUpdates, DropPendingUpdates})`. Path is static and secret-free; auth via `X-Telegram-Bot-Api-Secret-Token` header vs `s.secret` (constant-time compare). Handler optionally does tracing, `validateWebhook`, reads up to 10MB, `json.Unmarshal` `Update`, then `go func{ dispatcher.ProcessUpdate(bot, update, data{tracingCtx}) }` and returns `200 OK` quickly.
- `Start()` runs one `http.Server` on `HTTPPort` with `/health`, `/metrics` (auth), `/db_metrics`, optional `/debug/pprof/*`, plus `/webhook` when enabled.

For N clones, Telegram must deliver updates for each bot token to this process. Gotgbot's `Updater.SetAllBotWebhooks` expects **distinct `urlPath` per bot** (enforced by `ErrBotUrlPathAlreadyExists`), because `getHandlerFunc` dispatches by `r.URL.Path`. Two designs satisfy Alita while respecting that:

### Option A — Per-bot path (recommended, uses gotgbot webhook server)

- Keep Alita's `httpserver.Server` for `/health`, `/metrics`, `Start/Stop` lifecycle, but **delegate webhook routing to `ext.Updater.GetHandlerFunc`** or add per-clone mux entries.
- For each clone: `urlPath := fmt.Sprintf("/webhook/clone/%d", bot.Id)` (botId stable; alternatively full path `/webhook/<bot_username>`). Call `updater.AddWebhook(cloneBot, urlPath, &AddWebhookOpts{SecretToken: perCloneSecret})`. Primary uses `/webhook` or `/webhook/primary` consistently.
- After all `AddWebhook`, `updater.SetAllBotWebhooks(domain, &gotgbot.SetWebhookOpts{AllowedUpdates, DropPendingUpdates})` or per-bot `cloneBot.SetWebhook(domain+urlPath, optsWithSecret)`.
- Start once: `updater.StartServer(WebhookOpts{ListenAddr: fmt.Sprintf(":%d", httpPort), CertFile/KeyFile, ReadTimeout, SecretToken: ""})` — or reuse Alita's `http.Server` by `mux.Handle(updater.GetHandlerFunc("/"))` monting. In this design Alita's custom `webhookHandler` is **replaced** for clones; the library's `getHandlerFunc` already checks `X-Telegram-Bot-Api-Secret-Token` per bot and does `updateChan <- bytes`, and the per-bot `Dispatcher.Start` consumes it.
- Pros: leverages library-tested routing, per-bot secret isolation, path uniqueness guard, no manual multiplexing, works with `AddWebhook`’s `urlMapping` dedup. Minimal Alita code change.
- Cons: changes webhook URL surface (`/webhook/clone/123` vs `/webhook`); requires Telegram `setWebhook` per bot. Old primary path `/webhook` can be kept as alias pointing to primary bot's `urlPath` for zero-downtime cutover.

### Option B — Single path + secret-header multiplexing (keeps current `/webhook`)

- Keep one `mux.HandleFunc("/webhook", multiplexHandler)` where handler maintains `map[secretToken]*cloneInstance` (or `map[botId]*cloneInstance` keyed by header lookup, with `bot_id` fallback).
- On POST: `headerSecret := r.Header.Get("X-Telegram-Bot-Api-Secret-Token")`; constant-time lookup `secret → botData` → parse body → `dispatcherForBot.ProcessUpdate(botForSecret, update)`. If header missing/empty, `401`. Requires per-clone `SecretToken` distinct and strong (lock says per-token secret).
- Pros: single webhook URL (`https://domain/webhook`) for all bots; no path registry, no Telegram `setWebhook` path churn; DNS/simple reverse-proxy keeps working.
- Cons: must implement routing correctly (constant-time, concurrent map), body only read once then demultiplexed (can't use gotgbot's `updateChan` path without replicating it). You lose `botMapping.urlMapping` guard and must maintain your own `secret → bot` map with `RWMutex`. `getMe` validated secret still needs mapping; Telegram **does** send `X-Telegram-Bot-Api-Secret-Token` per webhook, so multiplexing by header is valid, but you must not fall back to URL prefix. Also violates gotgbot's expectation that each bot has unique `urlPath` — you'd bypass `ext.Updater.StartServer` entirely and keep `httpserver.Server`.
- Viable as **short-term** if zero-downtime path migration is hard (keep old primary on `/webhook`, clones on `/webhook/clone/*` is hybrid A+B).

### Option C — Separate port / separate `Updater.StartServer` per clone (not recommended)

- One `http.Server` per clone on distinct `ListenAddr` ports, each serving its own `/webhook`. Gotgbot supports per-Updater `ListenAddr`.
- Cons: port exhaustion, complicates health/metrics, still needs external routing (LB path→port), wastes file descriptors. Mentioned only for completeness; choose A or B.

**Minimal migration**: A is smallest diff if `httpserver.Server` is taught to proxy to `updater.GetHandlerFunc` (one line `mux.Handle("/", updater.GetHandlerFunc("/"))` plus per-clone `AddWebhook`). Keep `/health` etc. on same server; webhook handler becomes library-owned. If keeping custom handler is preferred, choose B's single-path secret map but add thorough tests for missing/duplicate secrets (gotgbot's `ErrBotUrlPathAlreadyExists` becomes `ErrSecretAlreadyExists` in your map).

Polling alternative (keeps growing clones simple during rollout): stay on polling (`USE_WEBHOOKS=false`), one `Updater.StartPolling` per clone. No HTTP multiplexing needed; scaling is `getUpdates` loops (one goroutine per clone). Webhook can be added later after `research/gotgbot-multi-bot` lands without changing per-Bot seam.

## 5. Minimal Diff Proposal — `alita/clones.Manager` Outline Respecting Locked Shared-Tables Decision

Locked direction (2026-08-08): same-process, shared tables `bot_id`/`owner_id` per domain table, Redis keys `alita:{module}:{bot_id}:{id}`, still full feature parity, per-user 1–2 cap + global cap + 1h Redis cooldown (backup pattern), DM-only `/clone` delete+`getMe`, minimal lifecycle + revoked detection (`getMe 401 → inactive`).

**Where the seam plugs into `main.go`** (topology M1):

```go
// main.go (new symbols only)
func newBotAPITransport(...) *http.Transport { /* unchanged */ }
func newConfiguredDispatcher(maxRoutines int) *ext.Dispatcher { /* unchanged: TracingProcessor + dispatcherErrorHandler */ }
func postInit(b *gotgbot.Bot, d *ext.Dispatcher, username, mode string) { /* alita.LoadModules(d); captcha lifecycle; setMyCommands; send startup; */ }

// New: re-entrant loader
// alita/main.go
func LoadModulesFor(d *ext.Dispatcher) { defer modules.LoadHelp(d); modules.LoadAllModules(d) }
// keep LoadModules for primary: ResetHelpRegistry(); LoadModulesFor(d)

// alita/utils/cache — namespaced keys (graduable, one TTL window migration)
// func adminCacheKey(botId, chatId int64) string { return fmt.Sprintf("alita:adminCache:%d:%d", botId, chatId) }
// func restrictedChatKey(botId, chatId int64) string { ... }
// cache.CacheKeyFor(botId int64, module string, id any) string

// alita/utils/httpserver — either delegate to Updater or multiplex (Option A/B)
// func (s *Server) RegisterCloneWebhooks(updater *ext.Updater, clones []*gotgbot.Bot, secrets map[int64]string, domain string) error
// or: func (s *Server) RegisterMultiplexedWebhook(manager *clones.Manager) // single /webhook + secret map
```

**`alita/clones.Manager` outline (no new interfaces, no speculative abstractions — ponytail ladder: reuse stdlib + existing patterns):**

```go
package clones

import (
    "context"
    "crypto/aes"
    "crypto/cipher"
    "net/http"
    "sync"

    "github.com/PaulSonOfLars/gotgbot/v2"
    "github.com/PaulSonOfLars/gotgbot/v2/ext"
    "github.com/redis/go-redis/v9"

    "github.com/divkix/Alita_Robot/alita/config"
    "github.com/divkix/Alita_Robot/alita/db"
    "github.com/divkix/Alita_Robot/alita/utils/cache"
)

// Manager owns clones in the same process. Invariants locked per §5 header.
// Reuses: newBotAPITransport helper (via passed transport pointer), newConfiguredDispatcher, resolveBotAPIURL,
//         cache.Manager singleflight/Redis cooldown pattern from backup, logredact.RegisterSecret.
type Manager struct {
    mu          sync.RWMutex
    byToken     map[string]*instance // key: plaintext hash or token (never logged) → instance; map protected by mu
    byBotID     map[int64]*instance
    transport   *http.Transport // shared *http.Transport from main.go (pooling)
    apiServer   string
    maxRoutines int
    redis       *redis.Client // cache.GetRedisClient() for cap/cooldown INCR/EXPIRE
    // webhook vs polling determined by config.AppConfig.UseWebhooks; manager aggregates AddWebhook then one StartServer/SetAllBotWebhooks
}

type instance struct {
    Bot        *gotgbot.Bot
    Dispatcher *ext.Dispatcher
    Updater    *ext.Updater // one per clone; StopBot/Stop drains its dispatcher
    OwnerID    int64  // Telegram user who owns clone (clone owner ≠ OWNER_ID the primary operator)
    BotID      int64  // bot.Id (from getMe, authoritative)
    Username   string
    // DB row id / encrypted token never exposed; token plaintext only in Bot.Token and logredact registry
}

// New — caller passes the already-built shared transport (same pointer as primary Bot's BaseBotClient)
func New(transport *http.Transport, apiServer string, maxRoutines int) *Manager {
    return &Manager{byToken: map[string]*instance{}, byBotID: map[int64]*instance{}, transport: transport, apiServer: apiServer, maxRoutines: maxRoutines, redis: cache.GetRedisClient()}
}

// Add validates via getMe (uses same BaseBotClient as primary), enforces caps+cooldown, encrypts, persists, registers dispatcher/updater.
// Must be DM-only; caller deletes the user's /clone message before calling (best-effort deleteMessage, ignore 400).
// Never logs raw token — caller wraps with logredact.Scrub; Manager calls logredact.RegisterSecret(token) for lifetime.
func (m *Manager) Add(ctx context.Context, ownerID int64, token string, chatID int64, msgID int64) (*instance, error) {
    // 1. shape check: `\d{6,}:[A-Za-z0-9_-]{30,}`
    // 2. Redis cooldown: INCR alita:clones:cooldown:{ownerID} EX 3600 → if >1 reject (reuse backup ratelimit)
    // 3. Caps: SELECT count(*) WHERE owner_id=ownerID (cap 1–2) + global count (cap N) under transaction / advisory lock
    // 4. b, err := gotgbot.NewBot(token, &gotgbot.BotOpts{BotClient: &gotgbot.BaseBotClient{Client: http.Client{Transport: m.transport}, DefaultRequestOpts: &gotgbot.RequestOpts{APIURL: resolveBotAPIURL(m.apiServer)}}})
    //    username := resolveBotUsername(b) // GetMe already called
    //    // optional: b.GetWebhookInfo → DeleteWebhook(drop_pending)
    // 5. enc := aesGCMSeal(token, keyFromEnv(CLONE_TOKEN_ENC_KEY)) // stdlib crypto/aes + cipher.NewGCM — no new dep
    // 6. INSERT ... bot_id=b.Id, owner_id=ownerID, token_enc=enc, is_active=true, bot_username=username  ON CONFLICT(token_hash) → rotation path
    // 7. dispatcher := newConfiguredDispatcher(m.maxRoutines); LoadModulesFor(dispatcher)
    // 8. updater := ext.NewUpdater(dispatcher, nil)
    //    if polling: updater.StartPolling(b, &ext.PollingOpts{DropPendingUpdates: true, GetUpdatesOpts: &gotgbot.GetUpdatesOpts{AllowedUpdates: config.AppConfig.AllowedUpdates}})
    //    else: updater.AddWebhook(b, fmt.Sprintf("/webhook/clone/%d", b.Id), &ext.AddWebhookOpts{SecretToken: perCloneSecret}); // aggregation caller does SetAllBotWebhooks later
    // 9. postInitForClone(b, dispatcher, username) // setMyCommands (private scope en), optional startup message to manager log group
    //10. map insert under mu; shutdownManager.RegisterHandler(func() error { m.Remove(b.Id); return nil })
    // Returns instance; caller replies "Clone @username live. /myclones to manage." (no token echo)
}

func (m *Manager) List(ownerID int64) []*instance { /* RLock, filter by OwnerID */ }
func (m *Manager) Remove(botID int64) error { /* WLock: StopBot+Dispatcher.Stop, DELETE ... WHERE bot_id=botID AND owner_id=caller, deregister secret */ }
func (m *Manager) Rotate(ownerID int64, oldBotID int64, newToken string) error { /* validate new token getMe, re-encrypt, update Bot.Token, re-SetWebhook */ }

// Restore boots existing rows after restart: SELECT ... WHERE is_active=true
// For each row: decrypt (aesGCMOpen via CLONE_TOKEN_ENC_KEY), NewBot+NewDispatcher+LoadModulesFor+NewUpdater+StartPolling/AddWebhook.
// If NewBot/getMe returns 401 → UPDATE is_active=false (revoked detection; BotFather /revoke flow) — do not crash.
// Called once from main.go after postInit(primary) before httpServer.Start / updater.Idle.

func (m *Manager) StopAll() { /* RLock snapshot, each inst.Updater.StopBot(token); inst.Dispatcher.Stop() */ }

// For webhook multi: Manager aggregates updaters; easiest is one shared ext.Updater for all clone webhooks:
// alt: singleUpdater := ext.NewUpdater(sharedDispatcher??, nil) is NOT per-clone dispatcher — pick one:
//   - per-clone Updater per instance (M1 polling) keeps StopBot isolated; for webhook, one Updater holding all clone Bots with per-clone Dispatcher attached via dispatcher.Start per bot is valid (gotgbot allows N bots/one updater/many dispatchers? Actually Updater.Dispatcher is single field; for webhook per-bot Dispatcher you need one Updater per clone, or one Updater with custom mapping. Simpler: polling M1, webhook Option A with single Updater + per-clone urlPath + single shared Updater.Dispatcher that is clones' dispatcher? → choose M1 for polling, A for webhook with single webhook Updater that owns one dispatcher cloned via LoadModulesFor that one dispatcher, plus per-clone Bots all feeding that dispatcher. Either topology works; manager abstracts it.
```

Why this shape is minimal (ponytail):

- Reuses `newBotAPITransport`'s `*http.Transport` pointer (connection pooling already shared); no extra config.
- Reuses `gotgbot.BaseBotClient` + `resolveBotAPIURL` for `API_SERVER` overrides.
- Reuses `newConfiguredDispatcher` (TracingProcessor + `dispatcherErrorHandler` with `helpers.IsExpectedTelegramError`) and `postInit` pattern (`LoadModulesFor`, `StartCaptchaLifecycle`, `SetMyCommands`, `SendMessage` startup) — no second set of error/captcha code.
- Reuses `cache.Manager`/`GetRedisClient()` singleflight + Redis `INCR` cooldown pattern from `alita/db/backup` (`1h Redis cooldown`).
- Reuses stdlib `crypto/aes`/`cipher.NewGCM` (ponytail ladder: no `libsodium` dep; single `CLONE_TOKEN_ENC_KEY` 32-byte key, `aes.NewCipher` + `cipher.NewGCM`, nonce per row, `logredact.RegisterSecret(token)` immediately after validation — matches `research/clone-token-security` doc's stdlib recommendation).
- No new interface with one implementation: `instance` is private; `Manager` exposes `Add/List/Remove/Rotate/Restore/StopAll`. Shutdown manager `RegisterHandler` LIFO ordering preserved: `closeDBConnections` registered last, clone updaters registered after it so they stop before DB closes (mirrors `main.go:203` comment).

**DB/cache cutover respecting lock:**

- Tables: add `bot_id BIGINT NOT NULL` + `owner_id BIGINT NOT NULL` to each domain table covering 17 modules per `alita/db/backup` export list (or one `clones` master + per-domain `bot_id` FK; migrations SHA-256 immutable). Queries become `WHERE bot_id = $bot_id AND chat_id = $id`. Existing rows get `bot_id = <primary bot_id>` backfill.
- Cache: `cache.CacheKeyFor(botId, module, id)` or overload `CacheKey(botId, ...args)` — one function change, all `cache.CacheKey("antiflood", chatId)` calls become `cache.CacheKeyFor(botId, "antiflood", chatId)`. Antiflood `adminCheckSemaphore` remains global but now per-bot key isolation prevents logical leakage.
- Flood `floodKey` includes `botId`. Migration: one deployment with dual-read (namespaced→legacy fallback) then flip after TTL.

## 6. Source Map

- `main.go:355-490`: `newBotAPITransport`, `resolveBotAPIURL`, `resolveBotUsername`, `newConfiguredDispatcher`, `dispatcherErrorHandler`, `postInit`, `shutdownManager`, `httpserver.New/RegisterWebhook/Start` — primary lifecycle seam clone clones copy.
- `alita/main.go:46-57`: `LoadModules` = `ResetHelpRegistry` + `defer LoadHelp` + `LoadAllModules` (priority-sorted `registry`). New `LoadModulesFor` is split.
- `alita/modules/registry.go:22-48`, `core.go:9-67`: `registry []registeredModule`, `RegisterLegacyModule`, `LoadAllModules` (stable sort ascending), `DefaultHelpRegistry` singleton + `ableMapMu` + `AbleMap`/`helpableKb`/`AltHelpOptions` — global-write hazard.
- `alita/modules/antiflood.go:36-117`: `floodKey{chatId,userId}`, `floodMu sync.Map`, `syncHelperMap sync.Map`, `adminCheckSemaphore`, `cleanupOnce` — per-key mutex + 5-min loop — needs `botId` in key.
- `alita/utils/httpserver/server.go:37-380`: `Server{bot,dispatcher,secret,mux,port,dispatchWG}`, `RegisterWebhook` single `"/webhook"` + `validateWebhook` header check, `webhookHandler` goroutine `dispatcher.ProcessUpdate(bot, update, data{tracingCtx})` — multiplex options A/B.
- `alita/utils/cache/cache.go` + `adminCache.go:19-230` + `restrictedCache.go:22-142` + `ttl.go`: `Manager *cache.Cache[any]` gocache Redis, `AdminCache{ChatId,UserInfo,UserMap,Cached}`, `LoadAdminCache`/`GetAdminCacheList`/`Invalidate`, `restrictedChatKey`/`IsChatRestricted`, `CacheTTLWarnSettings/Antiflood/...` — key namespacing needed.
- `go.mod:7`: `github.com/PaulSonOfLars/gotgbot/v2 v2.0.0-rc.35`.
- Library source `v2@v2.0.0-rc.35` (`ext/dispatcher.go:49-335`, `ext/updater.go:25-390`, `ext/botmapping.go:16-234`, `ext/webhook.go:8-36`): `UpdateDispatcher{Start(b,chan),Stop}`, `Dispatcher{Processor,Error,Panic,limiter,waitGroup,handlers}`, `Updater{Dispatcher,botMapping,webhookServer,Logger}`, `botMapping.addPollingBot/addWebhookBot/addBot/removeBot/getHandlerFunc`, `PollingOpts`, `WebhookOpts`, `BaseBotClient`, `RequestOpts`, per-bot secret header check.

---

Findings landed on `research/gotgbot-multi-bot`; see #801 wayfinder and `research/clone-token-security.md` for token lifecycle complement.
