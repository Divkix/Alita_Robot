package cache

import (
	"testing"

	"github.com/divkix/Alita_Robot/alita/config"
)

func TestNewRedisOptionsPreservesURLSemantics(t *testing.T) {
	t.Setenv("REDIS_PASSWORD", "")
	t.Setenv("REDIS_DB", "")

	options, err := newRedisOptions(&config.Config{
		RedisURL: "rediss://alice:secret@redis.example.com:6380/0",
	})
	if err != nil {
		t.Fatalf("newRedisOptions() error = %v", err)
	}
	if options.Addr != "redis.example.com:6380" {
		t.Fatalf("Addr = %q, want redis.example.com:6380", options.Addr)
	}
	if options.Username != "alice" || options.Password != "secret" {
		t.Fatalf("credentials = %q/%q, want alice/secret", options.Username, options.Password)
	}
	if options.DB != 0 {
		t.Fatalf("DB = %d, want 0", options.DB)
	}
	if options.TLSConfig == nil {
		t.Fatal("TLSConfig = nil for rediss URL")
	}
}

func TestNewRedisOptionsUsesDirectConfiguration(t *testing.T) {
	options, err := newRedisOptions(&config.Config{
		RedisAddress:  "redis:6379",
		RedisPassword: "secret",
		RedisDB:       0,
	})
	if err != nil {
		t.Fatalf("newRedisOptions() error = %v", err)
	}
	if options.Addr != "redis:6379" || options.Password != "secret" || options.DB != 0 {
		t.Fatalf("options = %#v", options)
	}
}
