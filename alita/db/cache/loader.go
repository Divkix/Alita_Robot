package cache

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/eko/gocache/lib/v4/store"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

var (
	// Per-key epochs: a write in chat A must not discard an in-flight load for
	// chat B. DeleteCache bumps only its own key.
	cacheGenerations sync.Map // string -> *atomic.Uint64
	loadWaitTimeout  = 30 * time.Second
	loadsMu          sync.Mutex
	loads            = make(map[string]*cacheLoad)
)

func generationFor(key string) *atomic.Uint64 {
	gen, _ := cacheGenerations.LoadOrStore(key, &atomic.Uint64{})
	return gen.(*atomic.Uint64)
}

type cacheLoad struct {
	done    chan struct{}
	cancel  context.CancelFunc
	waiters int
	value   any
	err     error
}

func GetFromCacheOrLoad[T any](ctx context.Context, key string, ttl time.Duration, loader func(context.Context) (T, error)) (T, error) {
	var zero T
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	m := cache.GetMarshal()
	if m == nil || (config.AppConfig != nil && config.AppConfig.DisableCache) {
		// Cache disabled: run the loader with the caller's context, allocating no timers.
		return loader(ctx)
	}
	// The read keeps its own short bound so a slow cache cannot eat the whole
	// load budget, and a hit then allocates one timer instead of two.
	readCtx, readCancel := context.WithTimeout(ctx, 5*time.Second)
	var cached T
	_, err := m.Get(readCtx, key, &cached)
	readCancel()
	if err == nil {
		return cached, nil
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}

	// Only waiting on a concurrent load needs the wider budget.
	ctx, cancel := context.WithTimeout(ctx, loadWaitTimeout)
	defer cancel()

	loadsMu.Lock()
	call := loads[key]
	if call == nil {
		loadCtx, loadCancel := context.WithTimeout(context.WithoutCancel(ctx), loadWaitTimeout)
		call = &cacheLoad{done: make(chan struct{}), cancel: loadCancel}
		loads[key] = call
		go func() {
			value, loadErr := runCacheLoader(loadCtx, key, ttl, loader)
			loadsMu.Lock()
			call.value, call.err = value, loadErr
			if loads[key] == call {
				delete(loads, key)
			}
			close(call.done)
			loadsMu.Unlock()
			loadCancel()
		}()
	}
	call.waiters++
	loadsMu.Unlock()
	defer func() {
		loadsMu.Lock()
		call.waiters--
		if call.waiters == 0 {
			call.cancel()
			if loads[key] == call {
				delete(loads, key)
			}
		}
		loadsMu.Unlock()
	}()
	select {
	case <-ctx.Done():
		return zero, ctx.Err()
	case <-call.done:
		if call.err != nil {
			return zero, call.err
		}
		return call.value.(T), nil
	}
}
func runCacheLoader[T any](ctx context.Context, key string, ttl time.Duration, loader func(context.Context) (T, error)) (value T, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("cache loader panic for %s: %v", key, recovered)
			log.Error(err)
		}
	}()
	generation := generationFor(key).Load()
	value, err = loader(ctx)
	if err != nil {
		return value, err
	}
	if err := ctx.Err(); err != nil {
		return value, err
	}
	m := cache.GetMarshal()
	if m != nil && generation == generationFor(key).Load() {
		setCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := m.Set(setCtx, key, value, store.WithExpiration(ttl))
		cancel()
		if err != nil {
			log.Debugf("[Cache] Failed to set cache for key %s: %v", key, err)
		} else if generation != generationFor(key).Load() {
			DeleteCache(key)
		}
	}
	return value, nil
}

func DeleteCache(key string) {
	loadsMu.Lock()
	delete(loads, key)
	loadsMu.Unlock()
	generationFor(key).Add(1)
	if config.AppConfig != nil && config.AppConfig.DisableCache {
		return
	}
	m := cache.GetMarshal()
	if m == nil {
		return
	}

	ctx, cancel := cache.ContextWithTimeout()
	err := m.Delete(ctx, key)
	cancel()
	if err != nil {
		ctx2, cancel2 := cache.ContextWithTimeout()
		err2 := m.Delete(ctx2, key)
		cancel2()
		if err2 != nil {
			log.Debugf("[Cache] Failed to delete cache for key %s: %v", key, err2)
		}
	}
}
