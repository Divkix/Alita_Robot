package cache

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/eko/gocache/lib/v4/marshaler"
	"github.com/hashicorp/golang-lru/v2/expirable"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/utils/metrics"
)

// The local layer is a short-TTL, size-bounded, in-process copy of what
// GetFromCacheOrLoad reads from Redis. It stores msgpack bytes, never Go
// values: every hit decodes a fresh copy, so a caller that mutates its result
// cannot corrupt what the next caller sees.
//
// Other replicas can serve a value for up to CACHE_LOCAL_TTL seconds after a
// write; the replica that wrote evicts its own copy in DeleteCache.

// localLayer binds the LRU to the marshaler it mirrors. When the backing
// store is swapped (InitCache, tests), the LRU is purged so it never serves
// entries that the new store does not hold.
type localLayer struct {
	lru   *expirable.LRU[string, []byte] // nil when the layer is off
	owner *marshaler.Marshaler
	valid bool
	ttl   int
	size  int
}

var (
	localMu    sync.Mutex
	localState atomic.Pointer[localLayer]

	// beforeLocalSetHook runs at the start of localSet, i.e. after the Redis
	// read (or Redis write, on a load) and before the local population. Tests
	// set it to stage a DeleteCache in that window; it is nil in production.
	beforeLocalSetHook func(key string)

	captchaPendingPrefix = CacheKey("captcha_pending") + ":"
)

// skipLocal reports whether key must never be served from process memory.
//
// List here every key whose freshness matters more than a Redis round trip:
// a value written on another replica (or outside DeleteCache) would otherwise
// be served stale for up to CACHE_LOCAL_TTL seconds.
func skipLocal(key string) bool {
	// Captcha pending flag gates whether a new member's messages are held;
	// a replica must see a flag set elsewhere immediately.
	return strings.HasPrefix(key, captchaPendingPrefix)
}

func localSettings() (ttl, size int) {
	cfg := config.AppConfig
	if cfg == nil {
		return config.DefaultCacheLocalTTLSeconds, config.DefaultCacheLocalMaxEntries
	}
	if cfg.DisableCache || cfg.CacheLocalTTLSeconds <= 0 {
		return 0, 0
	}
	size = cfg.CacheLocalMaxEntries
	if size <= 0 {
		size = config.DefaultCacheLocalMaxEntries
	}
	return cfg.CacheLocalTTLSeconds, size
}

// localFor returns the local LRU mirroring m, or nil when the layer is off.
func localFor(m *marshaler.Marshaler) *expirable.LRU[string, []byte] {
	if st := localState.Load(); st != nil && st.valid && st.owner == m {
		return st.lru
	}
	return bindLocal(m)
}

func bindLocal(m *marshaler.Marshaler) *expirable.LRU[string, []byte] {
	localMu.Lock()
	defer localMu.Unlock()
	prev := localState.Load()
	if prev != nil && prev.valid && prev.owner == m {
		return prev.lru
	}
	ttl, size := localSettings()
	next := &localLayer{owner: m, valid: true, ttl: ttl, size: size}
	switch {
	case ttl <= 0:
		// Layer off.
	case prev != nil && prev.lru != nil && prev.ttl == ttl && prev.size == size:
		// Same settings, new backing store: reuse the LRU (each one owns a
		// cleanup goroutine that never exits) but drop its contents.
		prev.lru.Purge()
		next.lru = prev.lru
	default:
		next.lru = expirable.NewLRU[string, []byte](size, nil, time.Duration(ttl)*time.Second)
	}
	if prev != nil && prev.lru != nil && prev.lru != next.lru {
		prev.lru.Purge()
	}
	localState.Store(next)
	return next.lru
}

func localGet(lru *expirable.LRU[string, []byte], key string) ([]byte, bool) {
	raw, ok := lru.Get(key)
	if ok {
		metrics.CacheLocalHits.Inc()
	} else {
		metrics.CacheLocalMisses.Inc()
	}
	return raw, ok
}

// localSet stores raw only if no DeleteCache for key's stripe ran since gen
// was read. The re-check after Add closes the window where DeleteCache runs
// between the first check and the Add; DeleteCache evicts again after its
// generation bump for the same reason.
func localSet(lru *expirable.LRU[string, []byte], key string, raw []byte, gen uint64) {
	if beforeLocalSetHook != nil {
		beforeLocalSetHook(key)
	}
	if generationFor(key).Load() != gen {
		return
	}
	lru.Add(key, raw)
	if generationFor(key).Load() != gen {
		lru.Remove(key)
	}
}

func localDelete(key string) {
	if st := localState.Load(); st != nil && st.lru != nil {
		st.lru.Remove(key)
	}
}
