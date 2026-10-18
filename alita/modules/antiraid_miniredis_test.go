//go:build testtools

package modules

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	gocache "github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/marshaler"
	gocache_store "github.com/eko/gocache/store/redis/v4"
	"github.com/redis/go-redis/v9"

	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

// withMiniredis starts an in-process Redis server (miniredis), points both the
// cache marshaler and the raw Redis client at it, and restores the previous
// cache state when t finishes. Antiraid is Redis-only, so tests of its
// Redis-dependent paths (trackJoin, checkExpiredRaids) use this to run against
// real Redis semantics without a live server.
func withMiniredis(t *testing.T) {
	t.Helper()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() error = %v", err)
	}
	t.Cleanup(mr.Close)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	t.Cleanup(func() { _ = client.Close() })

	previousMarshal, previousManager, previousClient := cache.GetCacheState()
	manager := gocache.New[any](gocache_store.NewRedis(client))
	cache.SetCacheState(marshaler.New(manager), manager, client)
	t.Cleanup(func() {
		cache.SetCacheState(previousMarshal, previousManager, previousClient)
	})
}
