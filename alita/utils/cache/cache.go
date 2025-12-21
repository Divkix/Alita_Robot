package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/marshaler"
	redis_store "github.com/eko/gocache/store/redis/v4"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
)

var (
	Context     = context.Background()
	Marshal     *marshaler.Marshaler
	Manager     *cache.Cache[any]
	redisClient *redis.Client
)

type AdminCache struct {
	ChatId   int64
	UserInfo []gotgbot.MergedChatMember
	Cached   bool
}

// InitCache initializes the Redis-only cache system.
// It establishes connection to Redis and returns an error if initialization fails.
func InitCache() error {
	// Initialize Redis client
	redisClient = redis.NewClient(&redis.Options{
		Addr:     config.AppConfig.RedisAddress,
		Password: config.AppConfig.RedisPassword, // no password set
		DB:       config.AppConfig.RedisDB,       // use default DB
	})

	// Test Redis connection with retry logic
	maxRetries := 5
	var pingErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
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
		return fmt.Errorf("failed to connect to Redis after %d attempts: %w", maxRetries, pingErr)
	}

	// Clear all caches on startup if configured to do so
	if config.AppConfig.ClearCacheOnStartup {
		if err := ClearAllCaches(); err != nil {
			log.Warnf("[Cache] Failed to clear caches on startup: %v", err)
		}
	}

	// Initialize cache manager with Redis only
	redisStore := redis_store.NewRedis(redisClient)
	cacheManager := cache.New[any](redisStore)

	// Initializes marshaler
	Marshal = marshaler.New(cacheManager)
	Manager = cacheManager

	return nil
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
