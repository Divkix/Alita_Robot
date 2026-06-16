//go:build testtools

package modules

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

// withMiniredis starts an in-process Redis server (miniredis), installs a
// *redis.Client pointing at it via cache.SetRedisClientForTest, and stops the
// server when t finishes. Tests that exercise Redis-dependent paths (trackJoin,
// checkExpiredRaids) call this instead of t.Skip so coverage goals are met
// without a live Redis server.
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

	cache.SetRedisClientForTest(t, client)
}
