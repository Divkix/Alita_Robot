//go:build testtools

package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	gocache "github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/marshaler"
	"github.com/eko/gocache/lib/v4/store"
	gocache_store "github.com/eko/gocache/store/redis/v4"
	"github.com/redis/go-redis/v9"

	utilsCache "github.com/divkix/Alita_Robot/alita/utils/cache"
)

// benchRealRedisCache wires the real gocache -> Redis -> msgpack stack against an
// in-process Redis so the numbers reflect the production read path without
// depending on an external server.
func benchRealRedisCache(b *testing.B) {
	b.Helper()
	mr := miniredis.RunT(b)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	redisStore := gocache_store.NewRedis(client)
	manager := gocache.New[any](redisStore)
	previousMarshal, previousManager, previousClient := utilsCache.GetCacheState()
	utilsCache.SetCacheState(marshaler.New(manager), manager, client)
	b.Cleanup(func() {
		_ = client.Close()
		utilsCache.SetCacheState(previousMarshal, previousManager, previousClient)
	})
}

func BenchmarkGetFromCacheOrLoadHitMemory(b *testing.B) {
	restore := utilsCache.InitTestMarshal()
	b.Cleanup(restore)

	ctx := context.Background()
	key := "alita:cache:bench:lang:1"
	if _, err := GetFromCacheOrLoad(ctx, key, time.Hour, func(context.Context) (string, error) {
		return "en", nil
	}); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := GetFromCacheOrLoad(ctx, key, time.Hour, func(context.Context) (string, error) {
			b.Fatal("loader must not run on a hit")
			return "", nil
		}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetFromCacheOrLoadHitRedis(b *testing.B) {
	benchRealRedisCache(b)

	ctx := context.Background()
	key := "alita:cache:bench:lang:1"
	if _, err := GetFromCacheOrLoad(ctx, key, time.Hour, func(context.Context) (string, error) {
		return "en", nil
	}); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := GetFromCacheOrLoad(ctx, key, time.Hour, func(context.Context) (string, error) {
			b.Fatal("loader must not run on a hit")
			return "", nil
		}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetFromCacheOrLoadHitRedisParallel(b *testing.B) {
	benchRealRedisCache(b)

	ctx := context.Background()
	key := "alita:cache:bench:lang:1"
	if _, err := GetFromCacheOrLoad(ctx, key, time.Hour, func(context.Context) (string, error) {
		return "en", nil
	}); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := GetFromCacheOrLoad(ctx, key, time.Hour, func(context.Context) (string, error) {
				return "", nil
			}); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkCacheKey(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = CacheKey("chat_lang", int64(-1001234567890))
	}
}

// BenchmarkMarshalerGetHit is the raw cache read with no GetFromCacheOrLoad
// wrapper, so the delta against the wrapper benchmarks is pure wrapper overhead.
func BenchmarkMarshalerGetHit(b *testing.B) {
	benchRealRedisCache(b)

	ctx := context.Background()
	key := "alita:cache:bench:lang:1"
	m := utilsCache.GetMarshal()
	if err := m.Set(ctx, key, "en", store.WithExpiration(time.Hour)); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		var out string
		if _, err := m.Get(ctx, key, &out); err != nil {
			b.Fatal(err)
		}
	}
}
