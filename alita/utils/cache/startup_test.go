package cache

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/divkix/Alita_Robot/alita/config"
)

func TestStartupClearingPreservesOperationalState(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	for _, key := range []string{"alita:cache:antiflood:-1001", "alita:antiraid:state:-1001", "alita:note_overwrite:token", "other-app:key"} {
		if err := client.Set(Context, key, "value", 0).Err(); err != nil {
			t.Fatal(err)
		}
	}
	originalConfig := config.AppConfig
	originalMarshal, originalManager, originalClient := GetCacheState()
	config.AppConfig = &config.Config{RedisAddress: server.Addr(), ClearCacheOnStartup: true}
	t.Cleanup(func() {
		if client := GetRedisClient(); client != nil {
			_ = client.Close()
		}
		config.AppConfig = originalConfig
		SetCacheState(originalMarshal, originalManager, originalClient)
	})
	if err := InitCache(); err != nil {
		t.Fatal(err)
	}
	if server.Exists("alita:cache:antiflood:-1001") {
		t.Error("disposable cache survived explicit startup clearing")
	}
	for _, key := range []string{"alita:antiraid:state:-1001", "alita:note_overwrite:token", "other-app:key"} {
		if !server.Exists(key) {
			t.Errorf("startup deleted operational/unrelated key %s", key)
		}
	}
}
