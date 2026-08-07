package cache

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/eko/gocache/lib/v4/store"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/singleflight"
)

var (
	cacheGroup      singleflight.Group
	cacheGeneration atomic.Uint64
	loadWaitTimeout = 30 * time.Second
)

// GetFromCacheOrLoad is a generic helper to get from cache or load from database with stampede protection.
func GetFromCacheOrLoad[T any](key string, ttl time.Duration, loader func() (T, error)) (T, error) {
	var result T

	m := cache.GetMarshal()
	if m == nil {
		return loader()
	}

	ctx, cancel := cache.ContextWithTimeout()
	_, err := m.Get(ctx, key, &result)
	cancel()
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
			generation := cacheGeneration.Load()
			val, err := loader()
			if err != nil {
				return nil, err
			}

			// ponytail: one global generation avoids unbounded per-key bookkeeping;
			// shard it only if unrelated writes measurably suppress cache fills.
			if generation == cacheGeneration.Load() {
				ctxSet, cancelSet := cache.ContextWithTimeout()
				setErr := m.Set(ctxSet, key, val, store.WithExpiration(ttl))
				cancelSet()
				if setErr != nil {
					log.Debugf("[Cache] Failed to set cache for key %s: %v", key, setErr)
				} else if generation != cacheGeneration.Load() {
					// An invalidation raced with Set after the first check. Delete
					// the value so an old database snapshot cannot survive it.
					ctxDel, cancelDel := cache.ContextWithTimeout()
					if err := m.Delete(ctxDel, key); err != nil {
						log.Debugf("[Cache] Failed to delete raced cache value for key %s: %v", key, err)
					}
					cancelDel()
				}
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

	timer := time.NewTimer(loadWaitTimeout)
	defer timer.Stop()
	select {
	case res := <-resCh:
		return res.val, res.err
	case <-timer.C:
		cacheGroup.Forget(key)
		log.Errorf("[Cache] Timeout loading key %s after %s", key, loadWaitTimeout)
		var zero T
		return zero, fmt.Errorf("cache load timed out for key %s", key)
	}
}
// DeleteCache is a helper to delete a value from cache.
// Logs debug information if deletion fails but does not return errors.
func DeleteCache(key string) {
	// Increment before deleting so an already-running loader cannot repopulate
	// the key with a database snapshot read before the write committed.
	cacheGeneration.Add(1)

	m := cache.GetMarshal()
	if m == nil {
		return
	}

	ctx, cancel := cache.ContextWithTimeout()
	err := m.Delete(ctx, key)
	cancel()
	if err != nil {
		// Retry once with fresh context
		ctx2, cancel2 := cache.ContextWithTimeout()
		err2 := m.Delete(ctx2, key)
		cancel2()
		if err2 != nil {
			log.Debugf("[Cache] Failed to delete cache for key %s: %v", key, err2)
		}
	}
}
