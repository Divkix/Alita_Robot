package metrics

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/redis/go-redis/v9"
)

func TestRedisHookCountsCommandsAndPipelines(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	client.AddHook(RedisHook{})

	ctx := context.Background()
	getBefore := testutil.ToFloat64(RedisCommands.WithLabelValues("get"))
	setBefore := testutil.ToFloat64(RedisCommands.WithLabelValues("set"))
	delBefore := testutil.ToFloat64(RedisCommands.WithLabelValues("del"))

	if err := client.Get(ctx, "missing").Err(); err != redis.Nil {
		t.Fatalf("Get() error = %v, want redis.Nil", err)
	}

	pipe := client.Pipeline()
	pipe.Set(ctx, "k", "v", 0)
	pipe.Del(ctx, "k")
	if _, err := pipe.Exec(ctx); err != nil {
		t.Fatalf("pipeline Exec() error = %v", err)
	}

	if got := testutil.ToFloat64(RedisCommands.WithLabelValues("get")) - getBefore; got != 1 {
		t.Fatalf("get commands delta = %v, want 1", got)
	}
	if got := testutil.ToFloat64(RedisCommands.WithLabelValues("set")) - setBefore; got != 1 {
		t.Fatalf("set commands delta = %v, want 1", got)
	}
	if got := testutil.ToFloat64(RedisCommands.WithLabelValues("del")) - delBefore; got != 1 {
		t.Fatalf("del commands delta = %v, want 1", got)
	}
}
