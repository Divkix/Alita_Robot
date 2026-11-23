//go:build testtools

package cache_test

// The skip-local prefixes are registered by domain init functions; importing
// the domains here (for their init side effects) is the only way an external
// test can observe them, since cache cannot import its consumers.
import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	gocache "github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/marshaler"
	"github.com/eko/gocache/lib/v4/store"
	gocache_store "github.com/eko/gocache/store/redis/v4"
	"github.com/redis/go-redis/v9"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	_ "github.com/divkix/Alita_Robot/alita/db/captcha"
	_ "github.com/divkix/Alita_Robot/alita/db/probation"
	utilsCache "github.com/divkix/Alita_Robot/alita/utils/cache"
)

// setupLocalRedis wires the real gocache -> Redis -> msgpack stack against
// miniredis with the local layer enabled, mirroring local_test.go's helper
// which the external test package cannot reach.
func setupLocalRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	manager := gocache.New[any](gocache_store.NewRedis(client))
	previousMarshal, previousManager, previousClient := utilsCache.GetCacheState()
	utilsCache.SetCacheState(marshaler.New(manager), manager, client)

	previousConfig := config.AppConfig
	cfg := config.Config{}
	if previousConfig != nil {
		cfg = *previousConfig
	}
	cfg.DisableCache = false
	cfg.CacheLocalTTLSeconds = 10
	cfg.CacheLocalMaxEntries = 1000
	config.AppConfig = &cfg
	cache.ResetLocalForTest()

	t.Cleanup(func() {
		config.AppConfig = previousConfig
		cache.ResetLocalForTest()
		_ = client.Close()
		utilsCache.SetCacheState(previousMarshal, previousManager, previousClient)
	})
	return mr
}

func countingLoader[T any](calls *atomic.Int32, value T) func(context.Context) (T, error) {
	return func(context.Context) (T, error) {
		calls.Add(1)
		return value, nil
	}
}

// A freshness-critical key written on another replica must be visible
// immediately: it never enters the in-process layer.
func TestExcludedKeysNeverServedLocally(t *testing.T) {
	mr := setupLocalRedis(t)
	ctx := context.Background()
	key := cache.CacheKey("captcha_pending", int64(-100123))
	var calls atomic.Int32

	if _, err := cache.GetFromCacheOrLoad(ctx, key, time.Minute, countingLoader(&calls, true)); err != nil {
		t.Fatal(err)
	}
	// Written elsewhere (another replica): must be visible immediately.
	if err := utilsCache.GetMarshal().Set(ctx, key, false, store.WithExpiration(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if got, _ := cache.GetFromCacheOrLoad(ctx, key, time.Minute, countingLoader(&calls, true)); got {
		t.Fatal("captcha_pending read = true, want false written to Redis")
	}
	mr.FlushAll()
	if _, err := cache.GetFromCacheOrLoad(ctx, key, time.Minute, countingLoader(&calls, true)); err != nil {
		t.Fatal(err)
	}
	if n := calls.Load(); n != 2 {
		t.Fatalf("loader calls = %d, want 2", n)
	}
}

// The registered prefixes match the freshness-critical keys only: sibling
// modules of the same domain stay cached locally.
func TestFreshnessCriticalKeysBypassLocalLayer(t *testing.T) {
	mr := setupLocalRedis(t)
	ctx := context.Background()

	memberKey := cache.CacheKey("probation_member", int64(-1001), int64(42))
	var memberCalls atomic.Int32
	if _, err := cache.GetFromCacheOrLoad(ctx, memberKey, time.Minute, countingLoader(&memberCalls, true)); err != nil {
		t.Fatal(err)
	}
	mr.FlushAll()
	if _, err := cache.GetFromCacheOrLoad(ctx, memberKey, time.Minute, countingLoader(&memberCalls, true)); err != nil {
		t.Fatal(err)
	}
	if n := memberCalls.Load(); n != 2 {
		t.Fatalf("probation_member loader calls = %d, want 2 (no local copy)", n)
	}

	settingsKey := cache.CacheKey("probation_settings", int64(-1001))
	var settingsCalls atomic.Int32
	if _, err := cache.GetFromCacheOrLoad(ctx, settingsKey, time.Minute, countingLoader(&settingsCalls, true)); err != nil {
		t.Fatal(err)
	}
	mr.FlushAll()
	if _, err := cache.GetFromCacheOrLoad(ctx, settingsKey, time.Minute, countingLoader(&settingsCalls, true)); err != nil {
		t.Fatal(err)
	}
	if n := settingsCalls.Load(); n != 1 {
		t.Fatalf("probation_settings loader calls = %d, want 1 (served locally)", n)
	}

	captchaKey := cache.CacheKey("captcha_settings", int64(-1001))
	var captchaCalls atomic.Int32
	if _, err := cache.GetFromCacheOrLoad(ctx, captchaKey, time.Minute, countingLoader(&captchaCalls, true)); err != nil {
		t.Fatal(err)
	}
	mr.FlushAll()
	if _, err := cache.GetFromCacheOrLoad(ctx, captchaKey, time.Minute, countingLoader(&captchaCalls, true)); err != nil {
		t.Fatal(err)
	}
	if n := captchaCalls.Load(); n != 1 {
		t.Fatalf("captcha_settings loader calls = %d, want 1 (served locally)", n)
	}
}
