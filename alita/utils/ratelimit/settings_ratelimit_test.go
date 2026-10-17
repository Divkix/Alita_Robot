//go:build testtools

package ratelimit

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

func TestAcquireSettingsPickerPausesPerUserInRedis(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cache.SetRedisClientForTest(t, client)
	const cooldown = 5 * time.Second

	if allowed, _ := AcquireSettingsPicker(7001, cooldown); !allowed {
		t.Fatal("first build was refused")
	}
	allowed, remaining := AcquireSettingsPicker(7001, cooldown)
	if allowed || remaining <= 0 || remaining > cooldown {
		t.Fatalf("second build = (%v, %v), want refused with time left in (0, %v]", allowed, remaining, cooldown)
	}
	if allowed, _ := AcquireSettingsPicker(7002, cooldown); !allowed {
		t.Fatal("another user's build was refused")
	}

	server.FastForward(cooldown)
	if allowed, _ := AcquireSettingsPicker(7001, cooldown); !allowed {
		t.Fatal("build after the pause was refused")
	}
}

func TestAcquireSettingsPickerWithoutCooldownNeverPauses(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cache.SetRedisClientForTest(t, client)

	for range 3 {
		if allowed, _ := AcquireSettingsPicker(7003, 0); !allowed {
			t.Fatal("build refused with the pause off")
		}
	}
	if keys := server.Keys(); len(keys) != 0 {
		t.Fatalf("pause off left keys %v", keys)
	}
}
