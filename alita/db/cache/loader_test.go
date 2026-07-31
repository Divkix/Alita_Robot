//go:build testtools

package cache

import (
	"testing"
	"time"

	utilsCache "github.com/divkix/Alita_Robot/alita/utils/cache"
)

func TestDeleteCachePreventsInFlightLoaderFromRepopulatingStaleValue(t *testing.T) {
	utilsCache.SetupTestMemoryMarshaler(t)

	const key = "alita:test:loader-race"
	started := make(chan struct{})
	release := make(chan struct{})
	result := make(chan string, 1)

	go func() {
		got, err := GetFromCacheOrLoad(key, time.Minute, func() (string, error) {
			close(started)
			<-release
			return "stale", nil
		})
		if err != nil {
			result <- "error: " + err.Error()
			return
		}
		result <- got
	}()

	<-started
	DeleteCache(key)
	close(release)

	if got := <-result; got != "stale" {
		t.Fatalf("GetFromCacheOrLoad() = %q, want caller snapshot stale", got)
	}

	var cached string
	if _, err := utilsCache.GetMarshal().Get(utilsCache.Context, key, &cached); err == nil {
		t.Fatalf("cache contains %q after invalidation raced with load", cached)
	}
}
