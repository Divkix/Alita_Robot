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

func TestNewRedisOptionsTLSVerification(t *testing.T) {
	t.Setenv("REDIS_PASSWORD", "")
	t.Setenv("REDIS_DB", "")

	cases := []struct {
		name       string
		url        string
		skipVerify bool
		wantTLS    bool
		wantSkip   bool
	}{
		{name: "rediss with opt-in skips verification", url: "rediss://:secret@redis.example.com:6380", skipVerify: true, wantTLS: true, wantSkip: true},
		{name: "rediss without opt-in verifies", url: "rediss://:secret@redis.example.com:6380", wantTLS: true},
		{name: "plain redis with opt-in has no TLS", url: "redis://:secret@redis.example.com:6379", skipVerify: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			options, err := newRedisOptions(&config.Config{
				RedisURL:                   tc.url,
				RedisTLSInsecureSkipVerify: tc.skipVerify,
			})
			if err != nil {
				t.Fatalf("newRedisOptions() error = %v", err)
			}
			if (options.TLSConfig != nil) != tc.wantTLS {
				t.Fatalf("TLSConfig = %v, want TLS %v", options.TLSConfig, tc.wantTLS)
			}
			if options.TLSConfig != nil && options.TLSConfig.InsecureSkipVerify != tc.wantSkip {
				t.Fatalf("InsecureSkipVerify = %v, want %v", options.TLSConfig.InsecureSkipVerify, tc.wantSkip)
			}
		})
	}
}

func TestNewRedisOptionsDirectAddressIgnoresTLSSkip(t *testing.T) {
	options, err := newRedisOptions(&config.Config{
		RedisAddress:               "redis:6379",
		RedisTLSInsecureSkipVerify: true,
	})
	if err != nil {
		t.Fatalf("newRedisOptions() error = %v", err)
	}
	if options.TLSConfig != nil {
		t.Fatalf("TLSConfig = %v, want nil for REDIS_ADDRESS", options.TLSConfig)
	}
}
