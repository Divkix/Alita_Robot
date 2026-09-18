package cache

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/marshaler"
	gocache_store "github.com/eko/gocache/store/redis/v4"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/config"
)

var (
	Context     = context.Background()
	marshal     *marshaler.Marshaler
	Manager     *cache.Cache[any]
	redisClient *redis.Client
	marshalMu   sync.RWMutex
)

// DataCachePrefix separates replaceable snapshots from operational Redis state.
const DataCachePrefix = "alita:cache:"

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

// GetCacheManager returns the gocache manager under the same lock as the
// marshaler: split direct Manager reads/writes race InitCache and health probes.
func GetCacheManager() *cache.Cache[any] {
	marshalMu.RLock()
	defer marshalMu.RUnlock()
	return Manager
}

// SetCacheState swaps marshal+manager+redis client atomically.
func SetCacheState(m *marshaler.Marshaler, mgr *cache.Cache[any], client *redis.Client) {
	marshalMu.Lock()
	defer marshalMu.Unlock()
	marshal = m
	Manager = mgr
	redisClient = client
}

func GetCacheState() (*marshaler.Marshaler, *cache.Cache[any], *redis.Client) {
	marshalMu.RLock()
	defer marshalMu.RUnlock()
	return marshal, Manager, redisClient
}

type AdminCache struct {
	ChatId   int64
	UserInfo []gotgbot.MergedChatMember
	UserMap  map[int64]gotgbot.MergedChatMember
	Cached   bool
	// Negative marks an authoritative empty result (bot not admin). Transient
	// failures leave Cached=false so callers fall back to per-user GetChatMember.
	Negative bool
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
	client := redis.NewClient(options)

	maxRetries := 5
	var pingErr error
	for attempt := range maxRetries {
		pingErr = client.Ping(Context).Err()
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
		_ = client.Close()
		if config.AppConfig != nil && config.AppConfig.DisableCache {
			log.Warnf("[Cache] Redis unavailable in DISABLE_CACHE mode — continuing without Redis (caching/states degraded): %v", pingErr)
			return nil
		}
		return fmt.Errorf("failed to connect to Redis after %d attempts: %w", maxRetries, pingErr)
	}

	if config.AppConfig.ClearCacheOnStartup {
		if err := clearWithClient(client); err != nil {
			log.Warnf("[Cache] Failed to clear caches on startup: %v", err)
		}
	}

	redisStore := gocache_store.NewRedis(client)
	cacheManager := cache.New[any](redisStore)

	// Single locked publish: readers never observe marshal-without-manager.
	SetCacheState(marshaler.New(cacheManager), cacheManager, client)

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
	client := GetRedisClient()
	if client == nil {
		return fmt.Errorf("redis client not initialized")
	}
	return clearWithClient(client)
}

func clearWithClient(client *redis.Client) error {
	ctx, cancel := ContextWithTimeout()
	defer cancel()
	var cursor uint64
	for {
		keys, next, err := client.Scan(ctx, cursor, DataCachePrefix+"*", 100).Result()
		if err != nil {
			return fmt.Errorf("failed to scan cached data: %w", err)
		}
		if len(keys) != 0 {
			if err := client.Unlink(ctx, keys...).Err(); err != nil {
				return fmt.Errorf("failed to clear cached data: %w", err)
			}
		}
		if next == 0 {
			return nil
		}
		cursor = next
	}
}

func GetRedisClient() *redis.Client {
	marshalMu.RLock()
	defer marshalMu.RUnlock()
	return redisClient
}

func IsRedisAvailable() bool {
	marshalMu.RLock()
	defer marshalMu.RUnlock()
	return redisClient != nil
}

func DisableRedisForTest() (restore func()) {
	marshalMu.Lock()
	previous := redisClient
	redisClient = nil
	marshalMu.Unlock()
	return func() {
		marshalMu.Lock()
		redisClient = previous
		marshalMu.Unlock()
	}
}
