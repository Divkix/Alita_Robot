//go:build testtools

package cache

import (
	"sync"
	"time"

	"github.com/eko/gocache/lib/v4/marshaler"
	"github.com/hashicorp/golang-lru/v2/expirable"
)

// ResetLocalForTest empties the in-process read-through layer and makes the
// next lookup re-read CACHE_LOCAL_TTL and CACHE_LOCAL_MAX_ENTRIES from
// config.AppConfig. Tests that write rows directly (bypassing the repository
// functions that call DeleteCache) and then read through the cache call it.
func ResetLocalForTest() {
	resetLocalForTest()
}

func resetLocalForTest() {
	localMu.Lock()
	defer localMu.Unlock()
	st := localState.Load()
	if st == nil {
		return
	}
	if st.lru != nil {
		st.lru.Purge()
	}
	// Keep the LRU for reuse when the settings are unchanged: each one owns a
	// cleanup goroutine that never exits.
	localState.Store(&localLayer{lru: st.lru, ttl: st.ttl, size: st.size})
}

var (
	shortLocalLRUOnce sync.Once
	shortLocalLRU     *expirable.LRU[string, []byte]
)

// shortLocalTTL is the entry lifetime useShortLocalTTLForTest installs.
const shortLocalTTL = 100 * time.Millisecond

// useShortLocalTTLForTest binds the local layer for m to an LRU whose entries
// live for shortLocalTTL, so a test can watch one expire without waiting out a
// whole CACHE_LOCAL_TTL second. The LRU is created once, because its cleanup
// goroutine never exits. The layer records a ttl localSettings never returns,
// so the next bindLocal replaces it instead of reusing it.
func useShortLocalTTLForTest(m *marshaler.Marshaler) {
	shortLocalLRUOnce.Do(func() {
		shortLocalLRU = expirable.NewLRU[string, []byte](1000, nil, shortLocalTTL)
	})
	localMu.Lock()
	defer localMu.Unlock()
	shortLocalLRU.Purge()
	if st := localState.Load(); st != nil && st.lru != nil && st.lru != shortLocalLRU {
		st.lru.Purge()
	}
	localState.Store(&localLayer{lru: shortLocalLRU, owner: m, valid: true, ttl: -1, size: 1000})
}
