//go:build testtools

package cache

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
	utilsCache "github.com/divkix/Alita_Robot/alita/utils/cache"
)

// setupLocalRedis wires the real gocache -> Redis -> msgpack stack against
// miniredis with the local layer set to ttlSeconds (0 disables it).
func setupLocalRedis(t *testing.T, ttlSeconds int) *miniredis.Miniredis {
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
	cfg.CacheLocalTTLSeconds = ttlSeconds
	cfg.CacheLocalMaxEntries = 1000
	config.AppConfig = &cfg
	resetLocalForTest()

	t.Cleanup(func() {
		beforeLocalSetHook = nil
		config.AppConfig = previousConfig
		resetLocalForTest()
		_ = client.Close()
		utilsCache.SetCacheState(previousMarshal, previousManager, previousClient)
	})
	return mr
}

// countingLoader returns value and counts how often the database would be hit.
func countingLoader[T any](calls *atomic.Int32, value T) func(context.Context) (T, error) {
	return func(context.Context) (T, error) {
		calls.Add(1)
		return value, nil
	}
}

func TestLocalHitAvoidsRedis(t *testing.T) {
	mr := setupLocalRedis(t, 10)
	ctx := context.Background()
	key := CacheKey("local_test_hit", int64(1))
	var calls atomic.Int32

	if got, err := GetFromCacheOrLoad(ctx, key, time.Minute, countingLoader(&calls, "v1")); err != nil || got != "v1" {
		t.Fatalf("first load = %q, %v; want v1, nil", got, err)
	}
	mr.FlushAll()
	mr.Close()

	got, err := GetFromCacheOrLoad(ctx, key, time.Minute, countingLoader(&calls, "from-db"))
	if err != nil || got != "v1" {
		t.Fatalf("second read = %q, %v; want v1 from the local layer", got, err)
	}
	if n := calls.Load(); n != 1 {
		t.Fatalf("loader calls = %d, want 1", n)
	}
}

func TestLocalPopulatedFromRedisHit(t *testing.T) {
	mr := setupLocalRedis(t, 10)
	ctx := context.Background()
	key := CacheKey("local_test_redis_hit", int64(1))
	if err := utilsCache.GetMarshal().Set(ctx, key, "from-redis", store.WithExpiration(time.Minute)); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32

	if got, err := GetFromCacheOrLoad(ctx, key, time.Minute, countingLoader(&calls, "from-db")); err != nil || got != "from-redis" {
		t.Fatalf("first read = %q, %v; want from-redis", got, err)
	}
	mr.FlushAll()
	if got, err := GetFromCacheOrLoad(ctx, key, time.Minute, countingLoader(&calls, "from-db")); err != nil || got != "from-redis" {
		t.Fatalf("second read = %q, %v; want from-redis from the local layer", got, err)
	}
	if n := calls.Load(); n != 0 {
		t.Fatalf("loader calls = %d, want 0", n)
	}
}

func TestDeleteCacheEvictsLocal(t *testing.T) {
	setupLocalRedis(t, 10)
	ctx := context.Background()
	key := CacheKey("local_test_delete", int64(1))
	var calls atomic.Int32

	if _, err := GetFromCacheOrLoad(ctx, key, time.Minute, countingLoader(&calls, "old")); err != nil {
		t.Fatal(err)
	}
	DeleteCache(key)
	got, err := GetFromCacheOrLoad(ctx, key, time.Minute, countingLoader(&calls, "new"))
	if err != nil || got != "new" {
		t.Fatalf("read after DeleteCache = %q, %v; want new", got, err)
	}
	if n := calls.Load(); n != 2 {
		t.Fatalf("loader calls = %d, want 2", n)
	}
}

// A DeleteCache that lands between the Redis read and the local population
// must win: the value read before the delete must not reach the local layer.
func TestDeleteDuringRedisHitDoesNotPopulateLocal(t *testing.T) {
	setupLocalRedis(t, 10)
	ctx := context.Background()
	key := CacheKey("local_test_race_read", int64(1))
	if err := utilsCache.GetMarshal().Set(ctx, key, "stale", store.WithExpiration(time.Minute)); err != nil {
		t.Fatal(err)
	}
	var fired atomic.Bool
	beforeLocalSetHook = func(k string) {
		if k == key && fired.CompareAndSwap(false, true) {
			DeleteCache(key)
		}
	}

	if got, err := GetFromCacheOrLoad(ctx, key, time.Minute, func(context.Context) (string, error) {
		t.Fatal("loader must not run on a Redis hit")
		return "", nil
	}); err != nil || got != "stale" {
		t.Fatalf("racing read = %q, %v; want stale (read before the delete)", got, err)
	}
	if !fired.Load() {
		t.Fatal("test hook did not run; the race was not staged")
	}

	var calls atomic.Int32
	got, err := GetFromCacheOrLoad(ctx, key, time.Minute, countingLoader(&calls, "fresh"))
	if err != nil || got != "fresh" {
		t.Fatalf("read after racing delete = %q, %v; want fresh (stale copy survived locally)", got, err)
	}
	if n := calls.Load(); n != 1 {
		t.Fatalf("loader calls = %d, want 1", n)
	}
}

// Same race on the load path: a DeleteCache between the Redis SET and the
// local population must keep the loaded value out of the local layer.
func TestDeleteDuringLoadDoesNotPopulateLocal(t *testing.T) {
	setupLocalRedis(t, 10)
	ctx := context.Background()
	key := CacheKey("local_test_race_load", int64(1))
	var fired atomic.Bool
	beforeLocalSetHook = func(k string) {
		if k == key && fired.CompareAndSwap(false, true) {
			DeleteCache(key)
		}
	}
	var calls atomic.Int32

	if got, err := GetFromCacheOrLoad(ctx, key, time.Minute, countingLoader(&calls, "stale")); err != nil || got != "stale" {
		t.Fatalf("racing load = %q, %v; want stale", got, err)
	}
	if !fired.Load() {
		t.Fatal("test hook did not run; the race was not staged")
	}
	got, err := GetFromCacheOrLoad(ctx, key, time.Minute, countingLoader(&calls, "fresh"))
	if err != nil || got != "fresh" {
		t.Fatalf("read after racing delete = %q, %v; want fresh (stale copy survived locally)", got, err)
	}
	if n := calls.Load(); n != 2 {
		t.Fatalf("loader calls = %d, want 2", n)
	}
}

func TestLocalEntryExpiresAfterTTL(t *testing.T) {
	setupLocalRedis(t, 1)
	ctx := context.Background()
	key := CacheKey("local_test_ttl", int64(1))
	var calls atomic.Int32

	if _, err := GetFromCacheOrLoad(ctx, key, time.Minute, countingLoader(&calls, "v1")); err != nil {
		t.Fatal(err)
	}
	// Another replica updates Redis without touching this replica's memory.
	if err := utilsCache.GetMarshal().Set(ctx, key, "v2", store.WithExpiration(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if got, _ := GetFromCacheOrLoad(ctx, key, time.Minute, countingLoader(&calls, "db")); got != "v1" {
		t.Fatalf("read within TTL = %q, want v1 from the local layer", got)
	}
	time.Sleep(1100 * time.Millisecond)
	if got, _ := GetFromCacheOrLoad(ctx, key, time.Minute, countingLoader(&calls, "db")); got != "v2" {
		t.Fatalf("read after TTL = %q, want v2 from Redis", got)
	}
	if n := calls.Load(); n != 1 {
		t.Fatalf("loader calls = %d, want 1", n)
	}
}

func TestLocalHitsReturnIndependentCopies(t *testing.T) {
	setupLocalRedis(t, 10)
	ctx := context.Background()
	key := CacheKey("local_test_copy", int64(1))
	var calls atomic.Int32

	if _, err := GetFromCacheOrLoad(ctx, key, time.Minute, countingLoader(&calls, []string{"a", "b"})); err != nil {
		t.Fatal(err)
	}
	first, err := GetFromCacheOrLoad(ctx, key, time.Minute, countingLoader(&calls, []string{"db"}))
	if err != nil {
		t.Fatal(err)
	}
	first[0] = "mutated"
	second, err := GetFromCacheOrLoad(ctx, key, time.Minute, countingLoader(&calls, []string{"db"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 2 || second[0] != "a" || second[1] != "b" {
		t.Fatalf("second read = %v, want [a b]; a caller's mutation leaked into the cache", second)
	}
	if n := calls.Load(); n != 1 {
		t.Fatalf("loader calls = %d, want 1", n)
	}
}

func TestLocalLayerDisabledWithZeroTTL(t *testing.T) {
	mr := setupLocalRedis(t, 0)
	ctx := context.Background()
	key := CacheKey("local_test_disabled", int64(1))
	var calls atomic.Int32

	if _, err := GetFromCacheOrLoad(ctx, key, time.Minute, countingLoader(&calls, "v1")); err != nil {
		t.Fatal(err)
	}
	if !mr.Exists(key) {
		t.Fatal("value was not written to Redis")
	}
	mr.FlushAll()
	got, err := GetFromCacheOrLoad(ctx, key, time.Minute, countingLoader(&calls, "v2"))
	if err != nil || got != "v2" {
		t.Fatalf("read with layer off = %q, %v; want v2 from the loader", got, err)
	}
	if n := calls.Load(); n != 2 {
		t.Fatalf("loader calls = %d, want 2", n)
	}
}

func TestExcludedKeysNeverServedLocally(t *testing.T) {
	mr := setupLocalRedis(t, 10)
	ctx := context.Background()
	key := CacheKey("captcha_pending", int64(-100123))
	var calls atomic.Int32

	if _, err := GetFromCacheOrLoad(ctx, key, time.Minute, countingLoader(&calls, true)); err != nil {
		t.Fatal(err)
	}
	// Written elsewhere (another replica): must be visible immediately.
	if err := utilsCache.GetMarshal().Set(ctx, key, false, store.WithExpiration(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if got, _ := GetFromCacheOrLoad(ctx, key, time.Minute, countingLoader(&calls, true)); got {
		t.Fatal("captcha_pending read = true, want false written to Redis")
	}
	mr.FlushAll()
	if _, err := GetFromCacheOrLoad(ctx, key, time.Minute, countingLoader(&calls, true)); err != nil {
		t.Fatal(err)
	}
	if n := calls.Load(); n != 2 {
		t.Fatalf("loader calls = %d, want 2", n)
	}
}

func TestSkipLocalMatchesOnlyCaptchaPendingKeys(t *testing.T) {
	if !skipLocal(CacheKey("captcha_pending", int64(-1001))) {
		t.Fatal("captcha_pending key not excluded")
	}
	if skipLocal(CacheKey("captcha_settings", int64(-1001))) {
		t.Fatal("captcha_settings key excluded, want it cached locally")
	}
}

func TestSwappingBackingStoreDropsLocalEntries(t *testing.T) {
	setupLocalRedis(t, 10)
	ctx := context.Background()
	key := CacheKey("local_test_swap", int64(1))
	var calls atomic.Int32

	if _, err := GetFromCacheOrLoad(ctx, key, time.Minute, countingLoader(&calls, "old-store")); err != nil {
		t.Fatal(err)
	}
	utilsCache.SetupTestMemoryMarshaler(t)
	got, err := GetFromCacheOrLoad(ctx, key, time.Minute, countingLoader(&calls, "new-store"))
	if err != nil || got != "new-store" {
		t.Fatalf("read after store swap = %q, %v; want new-store", got, err)
	}
}

// The load path hands Redis pre-encoded bytes; another replica (or this one
// after its local copy is gone) must decode them to the same value.
func TestLoadedValueRoundTripsThroughRedis(t *testing.T) {
	setupLocalRedis(t, 10)
	ctx := context.Background()
	key := CacheKey("local_test_roundtrip", int64(1))
	type settings struct {
		ChatID int64
		Words  []string
		On     bool
	}
	want := settings{ChatID: -1001, Words: []string{"a", "b"}, On: true}
	var calls atomic.Int32

	if _, err := GetFromCacheOrLoad(ctx, key, time.Minute, countingLoader(&calls, want)); err != nil {
		t.Fatal(err)
	}
	var fromRedis settings
	if _, err := utilsCache.GetMarshal().Get(ctx, key, &fromRedis); err != nil {
		t.Fatalf("Redis read error = %v", err)
	}
	if fromRedis.ChatID != want.ChatID || !fromRedis.On || len(fromRedis.Words) != 2 || fromRedis.Words[1] != "b" {
		t.Fatalf("Redis value = %+v, want %+v", fromRedis, want)
	}
	resetLocalForTest()
	got, err := GetFromCacheOrLoad(ctx, key, time.Minute, countingLoader(&calls, settings{}))
	if err != nil || got.ChatID != want.ChatID || len(got.Words) != 2 {
		t.Fatalf("read after local reset = %+v, %v; want %+v", got, err, want)
	}
	if n := calls.Load(); n != 1 {
		t.Fatalf("loader calls = %d, want 1", n)
	}
}
