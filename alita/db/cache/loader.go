package cache

import (
	"fmt"
	"time"

	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/eko/gocache/lib/v4/store"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/singleflight"
)

var cacheGroup singleflight.Group

// GetFromCacheOrLoad is a generic helper to get from cache or load from database with stampede protection.
func GetFromCacheOrLoad[T any](key string, ttl time.Duration, loader func() (T, error)) (T, error) {
	var result T

	m := cache.GetMarshal()
	if m == nil {
		return loader()
	}

	_, err := m.Get(cache.Context, key, &result)
	if err == nil {
		return result, nil
	}

	resCh := make(chan struct {
		val T
		err error
	}, 1)

	go func() {
		defer error_handling.RecoverFromPanic("cache", "GetFromCacheOrLoad")

		v, err, shared := cacheGroup.Do(key, func() (interface{}, error) {
			val, err := loader()
			if err != nil {
				return nil, err
			}
			if err := m.Set(cache.Context, key, val, store.WithExpiration(ttl)); err != nil {
				log.Debugf("[Cache] Failed to set cache for key %s: %v", key, err)
			}
			return val, nil
		})

		if shared {
			log.Debugf("[Cache] Shared cache load for key: %s", key)
		}

		if err != nil {
			resCh <- struct {
				val T
				err error
			}{result, err}
			return
		}

		resCh <- struct {
			val T
			err error
		}{v.(T), nil}
	}()

	select {
	case res := <-resCh:
		return res.val, res.err
	case <-time.After(30 * time.Second):
		cacheGroup.Forget(key)
		log.Errorf("[Cache] Timeout loading key %s after 30s", key)
		return result, fmt.Errorf("cache load timeout for key %s", key)
	}
}

// DeleteCache is a helper to delete a value from cache.
// Logs debug information if deletion fails but does not return errors.
func DeleteCache(key string) {
	m := cache.GetMarshal()
	if m == nil {
		return
	}

	err := m.Delete(cache.Context, key)
	if err != nil {
		log.Debugf("[Cache] Failed to delete cache for key %s: %v", key, err)
	}
}
