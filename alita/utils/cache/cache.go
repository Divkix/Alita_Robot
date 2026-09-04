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

func ContextWithTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(Context, 5*time.Second)
}

func GetMarshal() *marshaler.Marshaler {
	marshalMu.RLock()
	defer marshalMu.RUnlock()
	return marshal
}

func SetMarshal(m *marshaler.Marshaler) {
	marshalMu.Lock()
	defer marshalMu.Unlock()
	marshal = m
}

type AdminCache struct {
	ChatId   int64
	UserInfo []gotgbot.MergedChatMember
	UserMap  map[int64]gotgbot.MergedChatMember
	Cached   bool
}

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

	if config.AppConfig.ClearCacheOnStartup {
		if err := ClearAllCaches(); err != nil {
			log.Warnf("[Cache] Failed to clear caches on startup: %v", err)
		}
	}

	redisStore := gocache_store.NewRedis(redisClient)
	cacheManager := cache.New[any](redisStore)

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

func ClearAllCaches() error {
	if redisClient == nil {
		return fmt.Errorf("redis client not initialized")
	}

	log.Info("[Cache] Clearing all caches using FLUSHDB...")

	if err := redisClient.FlushDB(Context).Err(); err != nil {
		return fmt.Errorf("failed to flush database: %w", err)
	}

	log.Info("[Cache] Successfully cleared all cache entries")
	return nil
}

func GetRedisClient() *redis.Client {
	return redisClient
}

func IsRedisAvailable() bool {
	return redisClient != nil
}

func DisableRedisForTest() (restore func()) {
	previous := redisClient
	redisClient = nil
	return func() {
		redisClient = previous
	}
}
