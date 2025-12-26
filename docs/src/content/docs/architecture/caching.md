---
title: Caching Architecture
description: Redis caching implementation and patterns in Alita Robot.
---

Alita Robot uses Redis as its caching layer to reduce database load and improve response times. This document explains the caching architecture, patterns, and best practices.

## Cache Configuration

The cache is initialized in `alita/utils/cache/cache.go`:

```go
package cache

import (
    "context"

    "github.com/eko/gocache/lib/v4/cache"
    "github.com/eko/gocache/lib/v4/marshaler"
    redis_store "github.com/eko/gocache/store/redis/v4"
    "github.com/redis/go-redis/v9"
)

var (
    Context     = context.Background()
    Marshal     *marshaler.Marshaler
    Manager     *cache.Cache[any]
    redisClient *redis.Client
)

func InitCache() error {
    // Initialize Redis client
    redisClient = redis.NewClient(&redis.Options{
        Addr:     config.AppConfig.RedisAddress,
        Password: config.AppConfig.RedisPassword,
        DB:       config.AppConfig.RedisDB,
    })

    // Test connection with retry logic
    maxRetries := 5
    for attempt := 0; attempt < maxRetries; attempt++ {
        if err := redisClient.Ping(Context).Err(); err == nil {
            break
        }
        time.Sleep(time.Duration(1<<attempt) * time.Second)  // Exponential backoff
    }

    // Clear cache on startup if configured
    if config.AppConfig.ClearCacheOnStartup {
        ClearAllCaches()
    }

    // Initialize cache manager
    redisStore := redis_store.NewRedis(redisClient)
    cacheManager := cache.New[any](redisStore)
    Marshal = marshaler.New(cacheManager)
    Manager = cacheManager

    return nil
}
```

## TTL Values

Cache Time-To-Live (TTL) values are defined in `alita/db/cache_helpers.go`:

| Constant | Duration | Used For |
|----------|----------|----------|
| `CacheTTLChatSettings` | 30 minutes | Chat configuration |
| `CacheTTLLanguage` | 1 hour | Language preferences |
| `CacheTTLFilterList` | 30 minutes | Message filters |
| `CacheTTLBlacklist` | 30 minutes | Blacklisted words |
| `CacheTTLGreetings` | 30 minutes | Welcome/goodbye messages |
| `CacheTTLNotesList` | 30 minutes | Saved notes |
| `CacheTTLWarnSettings` | 30 minutes | Warning configuration |
| `CacheTTLAntiflood` | 30 minutes | Flood protection settings |
| `CacheTTLDisabledCmds` | 30 minutes | Disabled commands list |

```go
const (
    CacheTTLChatSettings = 30 * time.Minute
    CacheTTLLanguage     = 1 * time.Hour
    CacheTTLFilterList   = 30 * time.Minute
    CacheTTLBlacklist    = 30 * time.Minute
    CacheTTLGreetings    = 30 * time.Minute
    CacheTTLNotesList    = 30 * time.Minute
    CacheTTLWarnSettings = 30 * time.Minute
    CacheTTLAntiflood    = 30 * time.Minute
    CacheTTLDisabledCmds = 30 * time.Minute
)
```

## Key Patterns

All cache keys use the `alita:` prefix for namespace isolation:

| Key Pattern | Description |
|-------------|-------------|
| `alita:chat_settings:{chatId}` | Chat settings object |
| `alita:user_lang:{userId}` | User language preference |
| `alita:chat_lang:{chatId}` | Chat language preference |
| `alita:filter_list:{chatId}` | List of filters for chat |
| `alita:blacklist:{chatId}` | Blacklist settings |
| `alita:warn_settings:{chatId}` | Warning settings |
| `alita:disabled_cmds:{chatId}` | Disabled commands |
| `alita:anonAdmin:{chatId}:{msgId}` | Anonymous admin verification |

### Key Generator Functions

```go
func chatSettingsCacheKey(chatID int64) string {
    return fmt.Sprintf("alita:chat_settings:%d", chatID)
}

func userLanguageCacheKey(userID int64) string {
    return fmt.Sprintf("alita:user_lang:%d", userID)
}

func chatLanguageCacheKey(chatID int64) string {
    return fmt.Sprintf("alita:chat_lang:%d", chatID)
}

func filterListCacheKey(chatID int64) string {
    return fmt.Sprintf("alita:filter_list:%d", chatID)
}

func blacklistCacheKey(chatID int64) string {
    return fmt.Sprintf("alita:blacklist:%d", chatID)
}

func warnSettingsCacheKey(chatID int64) string {
    return fmt.Sprintf("alita:warn_settings:%d", chatID)
}

func disabledCommandsCacheKey(chatID int64) string {
    return fmt.Sprintf("alita:disabled_cmds:%d", chatID)
}
```

## Stampede Protection

The cache uses singleflight to prevent cache stampede (thundering herd problem):

```go
import "golang.org/x/sync/singleflight"

var cacheGroup singleflight.Group

func getFromCacheOrLoad[T any](key string, ttl time.Duration, loader func() (T, error)) (T, error) {
    var result T

    if cache.Marshal == nil {
        return loader()  // Cache not initialized
    }

    // Try cache first
    _, err := cache.Marshal.Get(cache.Context, key, &result)
    if err == nil {
        return result, nil  // Cache hit
    }

    // Cache miss - use singleflight with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    resultChan := make(chan sfResult, 1)

    go func() {
        defer error_handling.RecoverFromPanic("getFromCacheOrLoad", "cache_helpers")

        // Only ONE goroutine executes this, others wait
        v, err, _ := cacheGroup.Do(key, func() (any, error) {
            // Load from database
            data, loadErr := loader()
            if loadErr != nil {
                return data, loadErr
            }

            // Store in cache
            cache.Marshal.Set(cache.Context, key, data, store.WithExpiration(ttl))
            return data, nil
        })

        resultChan <- sfResult{value: v, err: err}
    }()

    select {
    case res := <-resultChan:
        if typedResult, ok := res.value.(T); ok {
            return typedResult, res.err
        }
        return result, res.err
    case <-ctx.Done():
        cacheGroup.Forget(key)  // Cleanup on timeout
        return result, fmt.Errorf("cache load timeout for key %s", key)
    }
}
```

### How Singleflight Works

```
Request 1  ──┐
Request 2  ──┼──> singleflight.Do(key) ──> loader() ──> result
Request 3  ──┘                                  │
                                                │
                  All requests get same result <┘
```

Without singleflight, if cache expires and 100 requests arrive simultaneously:
- **Bad**: 100 database queries
- **Good**: 1 database query, 99 requests wait and share result

## Cache Invalidation

When data changes, invalidate the cache:

```go
func deleteCache(key string) {
    if cache.Marshal == nil {
        return
    }

    err := cache.Marshal.Delete(cache.Context, key)
    if err != nil {
        log.Debugf("[Cache] Failed to delete cache for key %s: %v", key, err)
    }
}
```

### Example: Updating Chat Settings

```go
func SetChatSettings(chatID int64, settings ChatSettings) error {
    // Update database
    tx := db.Session(&gorm.Session{}).Where("chat_id = ?", chatID).
        Assign(settings).FirstOrCreate(&settings)
    if tx.Error != nil {
        return tx.Error
    }

    // Invalidate cache - IMPORTANT!
    deleteCache(chatSettingsCacheKey(chatID))

    return nil
}
```

## Admin Cache

Admin lists are cached specially for performance:

```go
type AdminCache struct {
    ChatId   int64
    UserInfo []gotgbot.MergedChatMember
    Cached   bool
}

// LoadAdminCache fetches and caches admin list
func LoadAdminCache(b *gotgbot.Bot, chatID int64) AdminCache {
    // Check if already cached
    found, adminCache := GetAdminCacheList(chatID)
    if found && adminCache.Cached {
        return adminCache
    }

    // Fetch from Telegram API
    admins, err := b.GetChatAdministrators(chatID, nil)
    if err != nil {
        return AdminCache{ChatId: chatID, Cached: false}
    }

    // Build cache
    var memberList []gotgbot.MergedChatMember
    for _, admin := range admins {
        memberList = append(memberList, admin.MergeChatMember())
    }

    cache := AdminCache{
        ChatId:   chatID,
        UserInfo: memberList,
        Cached:   true,
    }

    // Store in Redis
    SetAdminCacheList(chatID, cache)

    return cache
}
```

### Admin Cache Lookup

```go
func GetAdminCacheUser(chatID int64, userID int64) (bool, gotgbot.MergedChatMember) {
    found, adminCache := GetAdminCacheList(chatID)
    if !found || !adminCache.Cached {
        return false, gotgbot.MergedChatMember{}
    }

    for _, member := range adminCache.UserInfo {
        if member.User.Id == userID {
            return true, member
        }
    }

    return false, gotgbot.MergedChatMember{}
}
```

## CLEAR_CACHE_ON_STARTUP

The `CLEAR_CACHE_ON_STARTUP` environment variable controls cache clearing:

```go
if config.AppConfig.ClearCacheOnStartup {
    ClearAllCaches()
}

func ClearAllCaches() error {
    if redisClient == nil {
        return fmt.Errorf("redis client not initialized")
    }

    log.Info("[Cache] Clearing all caches using FLUSHDB...")

    // FLUSHDB clears all keys in current database
    if err := redisClient.FlushDB(Context).Err(); err != nil {
        return fmt.Errorf("failed to flush database: %w", err)
    }

    log.Info("[Cache] Successfully cleared all cache entries")
    return nil
}
```

**When to enable:**
- After schema changes
- When debugging cache issues
- After significant code changes affecting cached data

**When to disable (production):**
- Normal operations
- To preserve cache across restarts
- To reduce database load during deployment

## Best Practices

### 1. Always Invalidate on Updates

```go
// BAD - Cache becomes stale
func UpdateSettings(chatID int64, settings Settings) {
    db.Save(&settings)
    // Missing cache invalidation!
}

// GOOD - Cache stays consistent
func UpdateSettings(chatID int64, settings Settings) {
    db.Save(&settings)
    deleteCache(settingsCacheKey(chatID))  // Invalidate!
}
```

### 2. Use Appropriate TTLs

```go
// Frequently accessed, rarely changed -> longer TTL
CacheTTLLanguage = 1 * time.Hour

// Frequently changed -> shorter TTL
CacheTTLAntiflood = 30 * time.Minute

// Highly dynamic -> very short or no cache
anonChatMapExpiration = 20 * time.Second
```

### 3. Handle Cache Misses Gracefully

```go
func GetSettings(chatID int64) *Settings {
    result, err := getFromCacheOrLoad(
        settingsCacheKey(chatID),
        CacheTTLSettings,
        func() (*Settings, error) {
            var settings Settings
            tx := db.Where("chat_id = ?", chatID).First(&settings)
            if tx.Error != nil {
                // Return default, not error
                return &Settings{ChatID: chatID, Enabled: false}, nil
            }
            return &settings, nil
        },
    )
    if err != nil {
        // Return safe default on cache error
        return &Settings{ChatID: chatID, Enabled: false}
    }
    return result
}
```

### 4. Use Consistent Key Patterns

```go
// GOOD - Consistent prefix and format
"alita:chat_settings:{chatId}"
"alita:user_lang:{userId}"
"alita:filter_list:{chatId}"

// BAD - Inconsistent patterns
"settings-{chatId}"
"user:{userId}:language"
"chatFilters{chatId}"
```

### 5. Set Timeout on Cache Operations

```go
// Prevent hanging on Redis issues
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

select {
case result := <-resultChan:
    return result
case <-ctx.Done():
    cacheGroup.Forget(key)  // Cleanup
    return defaultValue, ctx.Err()
}
```

## Cache Monitoring

Monitor cache performance via:

1. **Logs**: Cache hits/misses logged at Debug level
2. **Redis CLI**: `redis-cli INFO stats` for hit rates
3. **Metrics**: Prometheus metrics (if enabled)

```bash
# Check cache key count
redis-cli DBSIZE

# View all Alita keys
redis-cli KEYS "alita:*"

# Check specific key TTL
redis-cli TTL "alita:chat_settings:123456789"

# Memory usage
redis-cli MEMORY USAGE "alita:chat_settings:123456789"
```

## Next Steps

- [Architecture Overview](/architecture/) - High-level design
- [Module Pattern](/architecture/module-pattern) - Using cache in modules
- [Request Flow](/architecture/request-flow) - When cache is accessed
