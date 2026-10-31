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
	"github.com/divkix/Alita_Robot/alita/utils/metrics"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
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

// contextTimeout is the budget ContextWithTimeout grants; a var so tests can shrink it.
var contextTimeout = 5 * time.Second

func ContextWithTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(Context, contextTimeout)
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
	Cached   bool
	// Negative marks an authoritative empty result (bot not admin). Transient
	// failures leave Cached=false so callers fall back to per-user GetChatMember.
	Negative bool
	// MemberLookupForbidden marks a Negative list after Telegram refused a
	// member lookup with CHAT_ADMIN_REQUIRED: until the list expires or the
	// bot's rights change, nobody's status can be read, so IsUserAdmin
	// answers false without asking again.
	MemberLookupForbidden bool
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
	client.AddHook(metrics.RedisHook{})
	client.AddHook(tracing.RedisHook{})

	if config.AppConfig.ClearCacheOnStartup {
		clearOnStartup(client)
	}

	redisStore := gocache_store.NewRedis(client)
	cacheManager := cache.New[any](redisStore)

	// Single locked publish: readers never observe marshal-without-manager.
	SetCacheState(marshaler.New(cacheManager), cacheManager, client)

	return nil
}

// newRedisOptions builds the shared client's options. ContextTimeoutEnabled
// makes socket reads and writes stop at the caller's context deadline; without
// it go-redis applies that deadline only to the pool wait and retry backoff,
// and a hung Redis holds every caller for the full Read/WriteTimeout.
// RedisTLSInsecureSkipVerify affects only a rediss:// REDIS_URL.
func newRedisOptions(cfg *config.Config) (*redis.Options, error) {
	if cfg.RedisURL == "" {
		return &redis.Options{
			Addr:                  cfg.RedisAddress,
			Password:              cfg.RedisPassword,
			DB:                    cfg.RedisDB,
			ContextTimeoutEnabled: true,
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
	if options.TLSConfig != nil && cfg.RedisTLSInsecureSkipVerify {
		// Heroku Redis serves rediss:// with a self-signed certificate that
		// cannot be verified; skipping is opt-in via REDIS_TLS_INSECURE_SKIP_VERIFY.
		options.TLSConfig.InsecureSkipVerify = true
		log.Warn("[Cache] REDIS_TLS_INSECURE_SKIP_VERIFY=true: Redis TLS certificate verification is disabled")
	}
	options.ContextTimeoutEnabled = true
	return options, nil
}

// clearOnStartup wipes the data cache before the bot serves traffic. A clear
// that stops early leaves stale snapshots behind, so it is an Error, but not
// fatal: the bot still starts, as it did before.
func clearOnStartup(client *redis.Client) {
	if err := clearWithClient(client); err != nil {
		log.Errorf("[Cache] Startup cache clear did not finish, stale %s* keys may remain: %v", DataCachePrefix, err)
	}
}

func ClearAllCaches() error {
	client := GetRedisClient()
	if client == nil {
		return fmt.Errorf("redis client not initialized")
	}
	return clearWithClient(client)
}

// clearWithClient deletes every DataCachePrefix key. Each SCAN and UNLINK gets
// its own ContextWithTimeout: a done context fails the next command at the pool
// wait, so one budget for the whole walk would abort a large keyspace midway.
// On failure the error says how many keys were already removed.
func clearWithClient(client *redis.Client) error {
	var cursor uint64
	deleted := 0
	for {
		keys, next, err := scanDataKeys(client, cursor)
		if err != nil {
			return fmt.Errorf("failed to scan cached data after clearing %d keys: %w", deleted, err)
		}
		if len(keys) != 0 {
			if err := unlinkKeys(client, keys); err != nil {
				return fmt.Errorf("failed to clear cached data after clearing %d keys: %w", deleted, err)
			}
			deleted += len(keys)
		}
		if next == 0 {
			return nil
		}
		cursor = next
	}
}

func scanDataKeys(client *redis.Client, cursor uint64) ([]string, uint64, error) {
	ctx, cancel := ContextWithTimeout()
	defer cancel()
	return client.Scan(ctx, cursor, DataCachePrefix+"*", 100).Result()
}

func unlinkKeys(client *redis.Client, keys []string) error {
	ctx, cancel := ContextWithTimeout()
	defer cancel()
	return client.Unlink(ctx, keys...).Err()
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
