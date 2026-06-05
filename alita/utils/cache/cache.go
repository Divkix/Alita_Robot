package cache

import (
	"context"
	"sync"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/coocood/freecache"
	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/marshaler"
	gocache_store "github.com/eko/gocache/store/freecache/v4"
	log "github.com/sirupsen/logrus"
)

var (
	Context   = context.Background()
	marshal   *marshaler.Marshaler
	Manager   *cache.Cache[any]
	marshalMu sync.RWMutex
	cacheSize = 100 * 1024 * 1024 // 100MB
	freeCache *freecache.Cache
)

// GetMarshal returns the active cache marshaler when initialized.
func GetMarshal() *marshaler.Marshaler {
	marshalMu.RLock()
	defer marshalMu.RUnlock()
	return marshal
}

// SetMarshal updates the active cache marshaler.
func SetMarshal(m *marshaler.Marshaler) {
	marshalMu.Lock()
	defer marshalMu.Unlock()
	marshal = m
}

type AdminCache struct {
	ChatId   int64
	UserInfo []gotgbot.MergedChatMember
	UserMap  map[int64]gotgbot.MergedChatMember // O(1) lookup map
	Cached   bool
}

// InitCache initializes the in-memory cache system using freecache.
func InitCache() error {
	log.Info("[Cache] Initializing in-memory cache...")

	freeCache = freecache.NewCache(cacheSize)
	freecacheStore := gocache_store.NewFreecache(freeCache)

	cacheManager := cache.New[any](freecacheStore)

	// Initializes marshaler
	SetMarshal(marshaler.New(cacheManager))
	Manager = cacheManager

	// Clear all caches on startup if configured to do so
	if config.AppConfig.ClearCacheOnStartup {
		if err := ClearAllCaches(); err != nil {
			log.Warnf("[Cache] Failed to clear caches on startup: %v", err)
		}
	}

	log.Info("[Cache] In-memory cache initialized successfully")
	return nil
}

// ClearAllCaches clears all cache entries from memory.
func ClearAllCaches() error {
	if freeCache == nil {
		return nil
	}

	log.Info("[Cache] Clearing all in-memory caches...")
	freeCache.Clear()
	log.Info("[Cache] Successfully cleared all cache entries")
	return nil
}

// GetRedisClient is a dummy function for backward compatibility
func GetRedisClient() interface{} {
	return nil
}

// IsRedisAvailable always returns false now as we use in-memory cache
func IsRedisAvailable() bool {
	return false
}

// DisableRedisForTest dummy for tests
func DisableRedisForTest() (restore func()) {
	return func() {}
}
