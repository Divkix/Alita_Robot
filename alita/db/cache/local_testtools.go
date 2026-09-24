//go:build testtools

package cache

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
