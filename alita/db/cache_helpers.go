package db

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/eko/gocache/lib/v4/store"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/singleflight"
)

const (
	// Cache expiration times
	CacheTTLChatSettings    = 30 * time.Minute
	CacheTTLLanguage        = 1 * time.Hour
	CacheTTLFilterList      = 30 * time.Minute
	CacheTTLBlacklist       = 30 * time.Minute
	CacheTTLGreetings       = 30 * time.Minute
	CacheTTLNotesList       = 30 * time.Minute
	CacheTTLWarnSettings    = 30 * time.Minute
	CacheTTLAntiflood       = 30 * time.Minute
	CacheTTLDisabledCmds    = 30 * time.Minute
	CacheTTLCaptchaSettings = 30 * time.Minute
)

// CacheKey generates a cache key with the alita prefix and any number of ID segments.
// Usage: CacheKey("chat_settings", chatID) → "alita:chat_settings:123"
// Usage: CacheKey("lock", chatID, lockType) → "alita:lock:123:photos"
func CacheKey(module string, ids ...any) string {
	var b strings.Builder
	b.Grow(32 + len(ids)*20)
	b.WriteString("alita:")
	b.WriteString(module)
	for _, id := range ids {
		b.WriteByte(':')
		switch v := id.(type) {
		case int64:
			b.WriteString(strconv.FormatInt(v, 10))
		case int:
			b.WriteString(strconv.Itoa(v))
		case string:
			b.WriteString(v)
		default:
			b.WriteString(fmt.Sprint(id))
		}
	}
	return b.String()
}

// Singleflight group for preventing cache stampede
var (
	cacheGroup singleflight.Group
)

// getFromCacheOrLoad is a generic helper to get from cache or load from database with stampede protection.
// Uses singleflight to deduplicate concurrent cache misses.
func getFromCacheOrLoad[T any](key string, ttl time.Duration, loader func() (T, error)) (T, error) {
	var result T

	if cache.Marshal == nil {
		// Cache not initialized, load directly
		return loader()
	}

	// Try to get from cache
	_, err := cache.Marshal.Get(cache.Context, key, &result)
	if err == nil {
		// Cache hit
		return result, nil
	}

	// Cache miss, use singleflight to prevent stampede
	v, err, _ := cacheGroup.Do(key, func() (any, error) {
		// Load from database
		data, loadErr := loader()
		if loadErr != nil {
			return data, loadErr
		}

		// Store in cache
		cacheErr := cache.Marshal.Set(cache.Context, key, data, store.WithExpiration(ttl))
		if cacheErr != nil {
			log.Debugf("[Cache] Failed to set cache for key %s: %v", key, cacheErr)
		}

		return data, nil
	})
	if err != nil {
		return result, err
	}

	// Type assert the result with panic recovery
	func() {
		defer func() {
			if r := recover(); r != nil {
				log.Errorf("[Cache] Panic during type assertion for key %s: %v", key, r)
				var zero T
				result = zero
			}
		}()

		if typedResult, ok := v.(T); ok {
			result = typedResult
		} else {
			log.Errorf("[Cache] Type assertion failed for key %s: expected %T, got %T", key, result, v)
			var zero T
			result = zero
		}
	}()

	return result, nil
}

// deleteCache is a helper to delete a value from cache.
// Logs debug information if deletion fails but does not return errors.
func deleteCache(key string) {
	if cache.Marshal == nil {
		return
	}

	err := cache.Marshal.Delete(cache.Context, key)
	if err != nil {
		log.Debugf("[Cache] Failed to delete cache for key %s: %v", key, err)
	}
}
