package cache

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/marshaler"
	gocache_store "github.com/eko/gocache/store/redis/v4"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
)

var (
	Context     = context.Background()
	marshal     *marshaler.Marshaler
	Manager     *cache.Cache[any]
	redisClient *redis.Client
	marshalMu   sync.RWMutex
)

// ContextWithTimeout returns a context with a 5-second timeout derived from the background Context.
// Use this for cache Get/Set/Delete operations to avoid indefinite blocking when Redis is unavailable.
func ContextWithTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(Context, 5*time.Second)
}

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

// InitCache initializes the Redis-only cache system.
// It establishes connection to Redis and returns an error if initialization fails.
// When DISABLE_CACHE=true, Redis is optional: if unreachable it logs a warning and continues
// with caching disabled (all DB reads go direct, antiraid/join state degrades to in-memory).
func InitCache() error {
	if config.AppConfig != nil && config.AppConfig.DisableCache {
		log.Warn("[Cache] DISABLE_CACHE=true — bypassing read-through cache; every DB read will hit Postgres directly")
	}
	options, err := newRedisOptions(config.AppConfig)
	if err != nil {
		if config.AppConfig != nil && config.AppConfig.DisableCache {
			log.Warnf("[Cache] Redis options error in bypass mode, continuing without Redis: %v", err)
			return nil
		}
		return err
	}
	redisClient = redis.NewClient(options)

	// Test Redis connection with retry logic
	maxRetries := 5
	var pingErr error
	for attempt := range maxRetries {
		pingErr = redisClient.Ping(Context).Err()
		if pingErr == nil {
			break
		}

		log.WithFields(log.Fields{
			"attempt": attempt + 1,
			"error":   pingErr,
		}).Warning("[Cache] Failed to connect to Redis, retrying...")

		if attempt < maxRetries-1 {
			// Exponential backoff: 1s, 2s, 4s, 8s
			time.Sleep(time.Duration(1<<attempt) * time.Second)
		}
	}
	if pingErr != nil {
		if config.AppConfig != nil && config.AppConfig.DisableCache {
			log.Warnf("[Cache] Redis unavailable in DISABLE_CACHE mode — continuing without Redis (caching/states degraded): %v", pingErr)
			redisClient = nil
			return nil
		}
		return fmt.Errorf("failed to connect to Redis after %d attempts: %w", maxRetries, pingErr)
	}

	// Clear all caches on startup if configured to do so
	if config.AppConfig.ClearCacheOnStartup {
		if err := ClearAllCaches(); err != nil {
			log.Warnf("[Cache] Failed to clear caches on startup: %v", err)
		}
	}

	// Initialize cache manager with Redis only
	redisStore := gocache_store.NewRedis(redisClient)
	cacheManager := cache.New[any](redisStore)

	// Initializes marshaler
	SetMarshal(marshaler.New(cacheManager))
	Manager = cacheManager

	return nil
}

func newRedisOptions(cfg *config.Config) (*redis.Options, error) {
	if cfg.RedisURL == "" {
		return &redis.Options{
			Addr:     cfg.RedisAddress,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		}, nil
	}

	options, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("parse REDIS_URL: %w", err)
	}
	if os.Getenv("REDIS_PASSWORD") != "" {
		options.Password = cfg.RedisPassword
	}
	if os.Getenv("REDIS_DB") != "" {
		options.DB = cfg.RedisDB
	}
	return options, nil
}

// ClearAllCaches clears all cache entries from Redis using FLUSHDB.
// This function is called on bot startup to ensure fresh data and eliminate cache coherence issues.
// Since Redis is dedicated to the bot, FLUSHDB safely clears all keys in the current database.
func ClearAllCaches() error {
	if redisClient == nil {
		return fmt.Errorf("redis client not initialized")
	}

	log.Info("[Cache] Clearing all caches using FLUSHDB...")

	// Use FLUSHDB to clear all keys in current database
	// This is safe since Redis is dedicated to the bot
	if err := redisClient.FlushDB(Context).Err(); err != nil {
		return fmt.Errorf("failed to flush database: %w", err)
	}

	log.Info("[Cache] Successfully cleared all cache entries")
	return nil
}

// GetRedisClient exposes the internal Redis client for modules that need
// low-level Redis operations (e.g., sorted sets for rate-limiting).
// Returns nil if cache has not been initialized.
func GetRedisClient() *redis.Client {
	return redisClient
}

// IsRedisAvailable returns whether the Redis client is initialized.
func IsRedisAvailable() bool {
	return redisClient != nil
}

// DisableRedisForTest clears the Redis client until restore is called.
func DisableRedisForTest() (restore func()) {
	previous := redisClient
	redisClient = nil
	return func() {
		redisClient = previous
	}
}
