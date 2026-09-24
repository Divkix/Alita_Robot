package cache

import (
	"context"
	"fmt"
	"hash/fnv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/eko/gocache/lib/v4/store"
	"github.com/hashicorp/golang-lru/v2/expirable"
	log "github.com/sirupsen/logrus"
	"github.com/vmihailenco/msgpack/v5"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

const generationStripes = 4096

var (
	// Per-stripe epochs: a fixed array of counters (32 KB) instead of one
	// counter per key ever seen, so memory stays bounded. DeleteCache bumps
	// only its own key's stripe.
	cacheGenerations [generationStripes]atomic.Uint64
	loadWaitTimeout  = 30 * time.Second
	loadsMu          sync.Mutex
	loads            = make(map[string]*cacheLoad)
)

// generationFor returns the generation counter for key's stripe.
//
// Two keys can share a stripe. A DeleteCache on one then also bumps the
// other's counter, so an in-flight load of the other key skips caching its
// result (or deletes what it just wrote). That costs at most one extra cache
// miss; it can never make a stale value stick, which is all the guard exists
// to prevent.
func generationFor(key string) *atomic.Uint64 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return &cacheGenerations[h.Sum32()%generationStripes]
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
	lru := localFor(m)
	if lru != nil && skipLocal(key) {
		lru = nil
	}
	if lru != nil {
		if raw, ok := localGet(lru, key); ok {
			var v T
			if err := msgpack.Unmarshal(raw, &v); err == nil {
				return v, nil
			}
			localDelete(key) // corrupt entry: drop it and fall through
		}
	}
	// Read before the Redis GET: a DeleteCache that races the read bumps it,
	// and the local copy is then skipped (see localSet).
	gen := generationFor(key).Load()
	// The read keeps its own short bound so a slow cache cannot eat the whole
	// load budget, and a hit then allocates one timer instead of two.
	readCtx, readCancel := context.WithTimeout(ctx, 5*time.Second)
	var cached T
	_, err := m.Get(readCtx, key, &cached)
	readCancel()
	if err == nil {
		if lru != nil {
			if raw, encErr := msgpack.Marshal(cached); encErr == nil {
				localSet(lru, key, raw, gen)
			}
		}
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
		var lru *expirable.LRU[string, []byte]
		if !skipLocal(key) {
			lru = localFor(m)
		}
		// Encode once: Redis gets the bytes verbatim (RawMessage) and the
		// local layer keeps the same bytes.
		var toSet any = value
		var encoded []byte
		if lru != nil {
			if raw, encErr := msgpack.Marshal(value); encErr == nil {
				encoded = raw
				toSet = msgpack.RawMessage(raw)
			}
		}
		setCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := m.Set(setCtx, key, toSet, store.WithExpiration(ttl))
		cancel()
		if err != nil {
			log.Debugf("[Cache] Failed to set cache for key %s: %v", key, err)
		} else if generation != generationFor(key).Load() {
			DeleteCache(key)
		} else if encoded != nil {
			localSet(lru, key, encoded, generation)
		}
	}
	return value, nil
}

func DeleteCache(key string) {
	// Evict locally first so this replica stops serving the old value at once.
	localDelete(key)
	loadsMu.Lock()
	delete(loads, key)
	loadsMu.Unlock()
	generationFor(key).Add(1)
	// And again after the bump: a reader that passed localSet's generation
	// check before the bump may have added its copy after the first evict.
	localDelete(key)
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
