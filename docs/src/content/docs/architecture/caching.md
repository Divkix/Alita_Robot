---
title: Caching Architecture
description: Redis caching implementation and patterns in Alita Robot.
---

Alita Robot uses Redis as its caching layer to reduce database load and improve response times. This document explains the caching architecture, patterns, and best practices.

## Cache Configuration

`alita/utils/cache/cache.go` initializes Redis with connection retries and exposes a nil-safe marshaler. `alita/db/cache/` provides keys, TTLs, shared database loads, and invalidation.

`REDIS_URL` supports credentials in the URL; `REDIS_PASSWORD` overrides the password. `REDIS_ADDRESS` selects a direct address. `REDIS_DB` defaults to 1 and accepts an explicit 0. See the [environment reference](/api-reference/environment/) for configuration.

## TTL Values

Cache Time-To-Live (TTL) values are defined in `alita/db/cache/ttl.go`:

| Constant | Duration | Used For |
|----------|----------|----------|
| `CacheTTLChatSettings` | 30 minutes | Chat configuration |
| `CacheTTLLanguage` | 1 hour | Language preferences |
| `CacheTTLFilterList` | 30 minutes | Message filters |
| `CacheTTLBlacklist` | 30 minutes | Blacklisted words |
| `CacheTTLGreetings` | 30 minutes | Welcome/goodbye messages |
| `CacheTTLNotesList` | 30 minutes | Saved notes |
| `CacheTTLNotesSettings` | 30 minutes | Notes configuration |
| `CacheTTLWarnSettings` | 30 minutes | Warning configuration |
| `CacheTTLAntiflood` | 30 minutes | Flood protection settings |
| `CacheTTLDisabledCmds` | 30 minutes | Disabled commands list |
| `CacheTTLAntiRaid` | 30 minutes | Anti-raid settings |
| `CacheTTLApprovals` | 30 minutes | Approved users list |
| `CacheTTLCaptchaSettings` | 30 minutes | Captcha verification settings |

:::tip[TTL selection strategy]
Choose TTL based on how frequently data changes:
- **Rarely changed** (language preferences): 1 hour
- **Occasionally changed** (chat settings, filters): 30 minutes
- **Highly dynamic** (anonymous admin verification): 20 seconds
- **Never use infinite TTL** -- always set an upper bound to prevent stale data accumulation.
:::

## Key Patterns

Disposable snapshots use `alita:cache:`. Operational state uses its existing `alita:` domain keys:

| Key Pattern | Description |
|-------------|-------------|
| `alita:cache:chat_settings:{chatId}` | Legacy invalidation target only — settings are read via `alita:cache:chat:{chatId}` |
| `alita:cache:user_lang:{userId}` | User language preference |
| `alita:cache:chat_lang:{chatId}` | Chat language preference |
| `alita:cache:filter_list:{chatId}` | List of filters for chat |
| `alita:cache:blacklist:{chatId}` | Blacklist settings |
| `alita:cache:warn_settings:{chatId}` | Warning settings |
| `alita:cache:disabled_cmds:{chatId}` | Disabled commands |
| `alita:anonAdmin:{chatId}:{msgId}` | Anonymous admin verification (20s TTL) |
| `alita:cache:adminCache:{chatId}` | Cached admin list for a chat (30min TTL) |
| `alita:cache:captcha_settings:{chatId}` | Captcha settings (30 min TTL) |
| `alita:cache:approvals:{chatId}` | Approved users list (30 min TTL) |
| `alita:antiraid:state:{chatId}` | Live anti-raid state (TTL covers the requested raid expiry plus one polling interval) |
| `alita:antiraid:joins:{chatId}` | Anti-raid join tracking (60s counting window) |
| `alita:cache:locks_map:{chatId}` | Lock status (1 hour TTL, from optimized queries) |
| `alita:cache:user:{userId}` | User basic info (1 hour TTL, from optimized queries) |
| `alita:cache:chat:{chatId}` | Chat basic info (30 min TTL, from optimized queries) |
| `alita:cache:antiflood:{chatId}` | Antiflood settings (30 min TTL, from optimized queries) |
| `alita:cache:channel:{chatId}` | Channel settings (30 min TTL, from optimized queries) |

### Anonymous Admin Verification Flow

When an anonymous admin uses a command, the bot:
1. Stores the original message in cache with key `alita:anonAdmin:{chatId}:{msgId}`
2. Sends a verification button to the chat
3. When clicked, the callback handler retrieves the original message from cache via `cache.GetMarshal().Get`
4. The bot verifies the user is an admin and executes the original command

```go
// Store original message for anonymous admin
cache.GetMarshal().Set(
    cache.Context,
    fmt.Sprintf("alita:anonAdmin:%d:%d", chatId, msgId),
    originalMessage,
    store.WithExpiration(20*time.Second),  // Short TTL - button expires quickly
)

// Retrieve when verification button is clicked
var originalMsg gotgbot.Message
_, err := cache.GetMarshal().Get(
    cache.Context,
    fmt.Sprintf("alita:anonAdmin:%d:%d", chatId, msgId),
    &originalMsg,
)
```

:::note[Why 20 seconds?]
The anonymous admin verification window is intentionally short. If the admin does not click the verification button within 20 seconds, the cached message expires and the command is silently dropped. This prevents stale command executions and reduces cache memory usage.
:::

Generate database cache keys with `cache.CacheKey("domain", id, ...)` from `alita/db/cache`. The domain must match the corresponding writer's invalidation key; package names and cache domains sometimes differ.

## Stampede Protection

`alita/db/cache.GetFromCacheOrLoad(ctx, key, ttl, loader)` shares one database load among concurrent callers for the same key. Each caller keeps its own cancellation and deadline. The shared query has a 30-second deadline and is canceled when its last waiting caller leaves.

The loader receives a `context.Context`; pass it to `DB.WithContext(ctx)` or the context-aware database wrappers. A cache read failure falls through to the loader once. Loader failures propagate to the caller, without a second database query. Failed or canceled loads are not cached. Invalidation prevents an older load from repopulating a stale value.

```go
return cache.GetFromCacheOrLoad(ctx, cache.CacheKey("settings", chatID), time.Minute,
    func(ctx context.Context) (*Settings, error) {
        var settings Settings
        err := database.WithContext(ctx).Where("chat_id = ?", chatID).First(&settings).Error
        return &settings, err
    },
)
```

## Cache Invalidation

:::caution[The most important rule]
Every database write function that modifies cached data MUST call the corresponding cache invalidation function. Missing invalidation is the most common caching bug and causes users to see stale data for up to the TTL duration (30 minutes to 1 hour).
:::

After a successful write, call `cache.DeleteCache(cache.CacheKey("domain", id))` from `alita/db/cache`. This detaches older pending loads and prevents them from storing stale data. Direct marshaler deletion bypasses that protection.

Invalidate every affected representation. For example, filter writes invalidate both `filter_list` and `filters_optimized`; language writes invalidate the language key and the associated chat or user snapshot.

## Admin Cache

`alita/utils/cache/adminCache.go` verifies the bot's status before fetching administrators with `ReturnBots: true`. Concurrent lookups share one Telegram request. Results are stored synchronously and include a user-ID map for permission checks.

`BotUpdates` invalidates permissions for both `ChatMember` and `MyChatMember` events involving an administrator or creator, including permission edits that keep the same status. Invalidation detaches pending requests and prevents a delayed response from restoring old permissions. Promotion, demotion, and manual clearing use the same invalidation path.

Use `InvalidateAdminCache(chatID)` from `alita/utils/cache` for event handling, or `ClearAdminCache(chatID)` when the caller needs to report a deletion error.

## CLEAR_CACHE_ON_STARTUP

`CLEAR_CACHE_ON_STARTUP` defaults to `false`. Normal restarts preserve cached data to avoid a burst of database queries.

When explicitly enabled, `ClearAllCaches` uses Redis `SCAN` and `UNLINK` to remove only `alita:cache:*` entries. Anti-raid state, join tracking, pending confirmations, and unrelated application keys remain intact. Use this option when cached data needs to be rebuilt after a format change.

Old cache keys from versions predating the `alita:cache:` namespace expire through their existing TTLs. Deployments should finish replacing the old version before relying on invalidation across instances, because older instances still use the old cache keys.

## Database Errors and Deadlines

A missing settings row may use documented defaults. A database outage is an error: return it from loaders and setters so handlers can report that a change could not be saved. Do not cache outage defaults or announce success from an equality check against them.

Handlers obtain their context with `tracing.UpdateContext(ctx)` and pass it to context-aware repositories. Polling and webhook updates carry a 30-second context deadline. Database calls using that context stop when it expires; code that ignores the context must still finish on its own. Webhook shutdown drains accepted work, then cancels remaining update contexts if the shutdown deadline expires.

Database queries inherit loader deadlines through `WithContext`. Queries without an explicit deadline also receive a 30-second GORM default after startup migrations finish. Startup migrations retain their existing transaction behavior.

## Cache Monitoring

Monitor cache performance via:

1. **Logs**: Cache hits/misses logged at Debug level
2. **Redis CLI**: `redis-cli INFO stats` for hit rates
3. **Metrics**: Prometheus metrics (if enabled)

```bash
# Check cache key count
redis-cli DBSIZE

# View all Alita keys
redis-cli --scan --pattern "alita:*"

# Check specific key TTL
redis-cli TTL "alita:cache:chat:123456789"

# Memory usage
redis-cli MEMORY USAGE "alita:cache:chat:123456789"
```

:::tip[Cache operations]
Use `GetFromCacheOrLoad()` and `DeleteCache()` from `alita/db/cache` for database snapshots. Use `cache.GetMarshal()` from `alita/utils/cache` for operational state, and check for nil before access.
:::

## Next Steps

- [Architecture Overview](/architecture/) - High-level design
- [Module Pattern](/architecture/module-pattern) - Using cache in modules
- [Request Flow](/architecture/request-flow) - When cache is accessed
