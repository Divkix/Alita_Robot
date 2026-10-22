package config

import (
	"os"
	"testing"
	"time"
)

// t.Parallel() because it mutates os.Args and must run sequentially.
func TestIsCliModeActive(t *testing.T) {
	saveArgs := os.Args
	defer func() { os.Args = saveArgs }()

	t.Run("no args returns false", func(t *testing.T) {
		os.Args = []string{"binary"}
		if isCliModeActive() {
			t.Errorf("isCliModeActive() = true, want false")
		}
	})

	t.Run("single positional arg returns false", func(t *testing.T) {
		os.Args = []string{"binary", "start"}
		if isCliModeActive() {
			t.Errorf("isCliModeActive() = true, want false")
		}
	})

	t.Run("--version returns true", func(t *testing.T) {
		os.Args = []string{"binary", "--version"}
		if !isCliModeActive() {
			t.Errorf("isCliModeActive() = false, want true")
		}
	})

	t.Run("-v returns true", func(t *testing.T) {
		os.Args = []string{"binary", "-v"}
		if !isCliModeActive() {
			t.Errorf("isCliModeActive() = false, want true")
		}
	})

	t.Run("--health returns true", func(t *testing.T) {
		os.Args = []string{"binary", "--health"}
		if !isCliModeActive() {
			t.Errorf("isCliModeActive() = false, want true")
		}
	})

	t.Run("mixed args with flag returns true", func(t *testing.T) {
		os.Args = []string{"binary", "run", "--version"}
		if !isCliModeActive() {
			t.Errorf("isCliModeActive() = false, want true")
		}
	})

	t.Run("-version returns true", func(t *testing.T) {
		os.Args = []string{"binary", "-version"}
		if !isCliModeActive() {
			t.Errorf("isCliModeActive() = false, want true")
		}
	})

	t.Run("-health returns true", func(t *testing.T) {
		os.Args = []string{"binary", "-health"}
		if !isCliModeActive() {
			t.Errorf("isCliModeActive() = false, want true")
		}
	})
}

// t.Parallel() because t.Setenv() is incompatible with parallel execution.
func TestLoadConfig(t *testing.T) {
	t.Run("returns error when required fields missing", func(t *testing.T) {
		t.Setenv("BOT_TOKEN", "")
		t.Setenv("OWNER_ID", "")
		t.Setenv("MESSAGE_DUMP", "")
		t.Setenv("DATABASE_URL", "")
		t.Setenv("REDIS_ADDRESS", "")
		t.Setenv("REDIS_URL", "")

		_, err := LoadConfig()
		if err == nil {
			t.Fatalf("expected error for missing required fields, got nil")
		}
	})

	t.Run("loads config with all required env vars", func(t *testing.T) {
		t.Setenv("BOT_TOKEN", "test-token")
		t.Setenv("OWNER_ID", "12345")
		t.Setenv("MESSAGE_DUMP", "67890")
		t.Setenv("DATABASE_URL", "postgres://localhost/test")
		t.Setenv("REDIS_ADDRESS", "redis:6379")
		t.Setenv("REDIS_PASSWORD", "")
		t.Setenv("REDIS_URL", "")
		t.Setenv("HTTP_PORT", "9090")

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.BotToken != "test-token" {
			t.Errorf("BotToken: got %q, want %q", cfg.BotToken, "test-token")
		}
		if cfg.OwnerId != 12345 {
			t.Errorf("OwnerId: got %d, want %d", cfg.OwnerId, 12345)
		}
		if cfg.MessageDump != 67890 {
			t.Errorf("MessageDump: got %d, want %d", cfg.MessageDump, 67890)
		}
		if cfg.DatabaseURL != "postgres://localhost/test" {
			t.Errorf("DatabaseURL: got %q, want %q", cfg.DatabaseURL, "postgres://localhost/test")
		}
		if cfg.RedisAddress != "redis:6379" {
			t.Errorf("RedisAddress: got %q, want %q", cfg.RedisAddress, "redis:6379")
		}
		if cfg.HTTPPort != 9090 {
			t.Errorf("HTTPPort: got %d, want %d", cfg.HTTPPort, 9090)
		}
		if cfg.ApiServer != "https://api.telegram.org" {
			t.Errorf("ApiServer: got %q, want %q", cfg.ApiServer, "https://api.telegram.org")
		}
		if cfg.BotVersion == "" {
			t.Errorf("BotVersion: got empty string, want non-empty")
		}
		if len(cfg.AllowedUpdates) == 0 {
			t.Errorf("AllowedUpdates: expected non-empty slice")
		}
		if len(cfg.ValidLangCodes) != 1 || cfg.ValidLangCodes[0] != "en" {
			t.Errorf("ValidLangCodes: got %v, want [en]", cfg.ValidLangCodes)
		}
	})

	t.Run("webhook config loaded correctly", func(t *testing.T) {
		t.Setenv("BOT_TOKEN", "tk")
		t.Setenv("OWNER_ID", "1")
		t.Setenv("MESSAGE_DUMP", "1")
		t.Setenv("DATABASE_URL", "postgres://localhost/test")
		t.Setenv("REDIS_ADDRESS", "localhost:6379")
		t.Setenv("USE_WEBHOOKS", "true")
		t.Setenv("WEBHOOK_DOMAIN", "example.com")
		t.Setenv("WEBHOOK_SECRET", "shh")
		t.Setenv("HTTP_PORT", "8080")

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !cfg.UseWebhooks {
			t.Errorf("UseWebhooks: got false, want true")
		}
		if cfg.WebhookDomain != "example.com" {
			t.Errorf("WebhookDomain: got %q, want %q", cfg.WebhookDomain, "example.com")
		}
		if cfg.WebhookSecret != "shh" {
			t.Errorf("WebhookSecret: got %q, want %q", cfg.WebhookSecret, "shh")
		}
	})

	t.Run("ENABLE_PPROF parsed as bool", func(t *testing.T) {
		t.Setenv("BOT_TOKEN", "tk")
		t.Setenv("OWNER_ID", "1")
		t.Setenv("MESSAGE_DUMP", "1")
		t.Setenv("DATABASE_URL", "postgres://localhost/test")
		t.Setenv("REDIS_ADDRESS", "localhost:6379")
		t.Setenv("ENABLE_PPROF", "yes")
		t.Setenv("HTTP_PORT", "8080")

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !cfg.EnablePPROF {
			t.Errorf("EnablePPROF: got false, want true")
		}
	})

	t.Run("local cache settings", func(t *testing.T) {
		cases := []struct {
			name, ttl, maxEntries string
			wantTTL, wantMax      int
		}{
			{name: "unset uses defaults", wantTTL: 10, wantMax: 50000},
			{name: "explicit zero disables", ttl: "0", wantTTL: 0, wantMax: 50000},
			{name: "typo falls back to default", ttl: "ten", maxEntries: "lots", wantTTL: 10, wantMax: 50000},
			{name: "explicit values kept", ttl: "30", maxEntries: "1000", wantTTL: 30, wantMax: 1000},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Setenv("BOT_TOKEN", "tk")
				t.Setenv("OWNER_ID", "1")
				t.Setenv("MESSAGE_DUMP", "1")
				t.Setenv("DATABASE_URL", "postgres://localhost/test")
				t.Setenv("REDIS_ADDRESS", "localhost:6379")
				t.Setenv("HTTP_PORT", "8080")
				t.Setenv("CACHE_LOCAL_TTL", tc.ttl)
				t.Setenv("CACHE_LOCAL_MAX_ENTRIES", tc.maxEntries)

				cfg, err := LoadConfig()
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if cfg.CacheLocalTTLSeconds != tc.wantTTL {
					t.Errorf("CacheLocalTTLSeconds: got %d, want %d", cfg.CacheLocalTTLSeconds, tc.wantTTL)
				}
				if cfg.CacheLocalMaxEntries != tc.wantMax {
					t.Errorf("CacheLocalMaxEntries: got %d, want %d", cfg.CacheLocalMaxEntries, tc.wantMax)
				}
			})
		}
	})

	t.Run("out-of-range CACHE_LOCAL_TTL is rejected", func(t *testing.T) {
		t.Setenv("BOT_TOKEN", "tk")
		t.Setenv("OWNER_ID", "1")
		t.Setenv("MESSAGE_DUMP", "1")
		t.Setenv("DATABASE_URL", "postgres://localhost/test")
		t.Setenv("REDIS_ADDRESS", "localhost:6379")
		t.Setenv("HTTP_PORT", "8080")
		t.Setenv("CACHE_LOCAL_TTL", "301")

		if _, err := LoadConfig(); err == nil {
			t.Fatal("LoadConfig() error = nil, want CACHE_LOCAL_TTL range error")
		}
	})

	t.Run("CLEAR_CACHE_ON_STARTUP", func(t *testing.T) {
		for env, want := range map[string]bool{"": false, "false": false, "true": true} {
			t.Run("env="+env, func(t *testing.T) {
				t.Setenv("BOT_TOKEN", "tk")
				t.Setenv("OWNER_ID", "1")
				t.Setenv("MESSAGE_DUMP", "1")
				t.Setenv("DATABASE_URL", "postgres://localhost/test")
				t.Setenv("REDIS_ADDRESS", "localhost:6379")
				t.Setenv("HTTP_PORT", "8080")
				t.Setenv("CLEAR_CACHE_ON_STARTUP", env)

				cfg, err := LoadConfig()
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if cfg.ClearCacheOnStartup != want {
					t.Fatalf("ClearCacheOnStartup = %v, want %v", cfg.ClearCacheOnStartup, want)
				}
			})
		}
	})

	t.Run("REDIS_TLS_INSECURE_SKIP_VERIFY", func(t *testing.T) {
		for env, want := range map[string]bool{"": false, "false": false, "true": true, "yes": true} {
			t.Run("env="+env, func(t *testing.T) {
				t.Setenv("BOT_TOKEN", "tk")
				t.Setenv("OWNER_ID", "1")
				t.Setenv("MESSAGE_DUMP", "1")
				t.Setenv("DATABASE_URL", "postgres://localhost/test")
				t.Setenv("REDIS_ADDRESS", "")
				t.Setenv("REDIS_URL", "rediss://:secret@redis.example.com:6380")
				t.Setenv("HTTP_PORT", "8080")
				t.Setenv("REDIS_TLS_INSECURE_SKIP_VERIFY", env)

				cfg, err := LoadConfig()
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if cfg.RedisTLSInsecureSkipVerify != want {
					t.Fatalf("RedisTLSInsecureSkipVerify = %v, want %v", cfg.RedisTLSInsecureSkipVerify, want)
				}
			})
		}
	})

	t.Run("ENABLE_DIGEST", func(t *testing.T) {
		for _, tc := range []struct {
			env  string
			want bool
		}{
			{"", true},
			{"true", true},
			{"false", false},
		} {
			t.Run("env="+tc.env, func(t *testing.T) {
				setRequiredEnv(t)
				t.Setenv("ENABLE_DIGEST", tc.env)

				cfg, err := LoadConfig()
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if cfg.EnableDigest != tc.want {
					t.Fatalf("EnableDigest = %v, want %v", cfg.EnableDigest, tc.want)
				}
			})
		}
	})

	t.Run("DIGEST_INTERVAL", func(t *testing.T) {
		for _, tc := range []struct {
			env  string
			want time.Duration
		}{
			{"", time.Hour},
			{"30m", 30 * time.Minute},
			{"6h", 6 * time.Hour},
		} {
			t.Run("env="+tc.env, func(t *testing.T) {
				setRequiredEnv(t)
				t.Setenv("DIGEST_INTERVAL", tc.env)

				cfg, err := LoadConfig()
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if cfg.DigestInterval != tc.want {
					t.Fatalf("DigestInterval = %v, want %v", cfg.DigestInterval, tc.want)
				}
			})
		}
	})
}

// A digest cadence typo fails startup instead of silently running hourly.
func TestDigestConfigRejectsInvalidInterval(t *testing.T) {
	for _, raw := range []string{"soon", "0s", "-5m"} {
		t.Run("env="+raw, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv("DIGEST_INTERVAL", raw)
			if _, err := LoadConfig(); err == nil {
				t.Fatalf("LoadConfig() accepted DIGEST_INTERVAL=%q", raw)
			}
		})
	}
}

func setPanelTestRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("BOT_TOKEN", "tk")
	t.Setenv("OWNER_ID", "1")
	t.Setenv("MESSAGE_DUMP", "1")
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("REDIS_ADDRESS", "redis:6379")
	t.Setenv("REDIS_URL", "")
	t.Setenv("HTTP_PORT", "8080")
}

func TestPanelConfigDefaultsDisabled(t *testing.T) {
	setPanelTestRequiredEnv(t)
	t.Setenv("PANEL_ENABLED", "")
	t.Setenv("PANEL_PUBLIC_URL", "")
	t.Setenv("PANEL_MINIAPP_SHORTNAME", "")
	t.Setenv("PANEL_SESSION_MAX_AGE", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.PanelEnabled {
		t.Fatal("PanelEnabled = true with no PANEL_ENABLED, want false")
	}
	if cfg.PanelPublicURL != "" {
		t.Fatalf("PanelPublicURL = %q, want empty", cfg.PanelPublicURL)
	}
	if cfg.PanelSessionMaxAge != 24*time.Hour {
		t.Fatalf("PanelSessionMaxAge = %v, want 24h", cfg.PanelSessionMaxAge)
	}
}

func TestPanelConfigEnabled(t *testing.T) {
	setPanelTestRequiredEnv(t)
	t.Setenv("PANEL_ENABLED", "true")
	t.Setenv("PANEL_PUBLIC_URL", "https://panel.example.com/")
	t.Setenv("PANEL_MINIAPP_SHORTNAME", "@alita_panel")
	t.Setenv("PANEL_SESSION_MAX_AGE", "12h")
	t.Setenv("DEBUG", "false")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.PanelEnabled {
		t.Fatal("PanelEnabled = false, want true")
	}
	if cfg.PanelPublicURL != "https://panel.example.com" {
		t.Fatalf("PanelPublicURL = %q, want the trailing slash trimmed", cfg.PanelPublicURL)
	}
	if cfg.PanelMiniAppShortName != "alita_panel" {
		t.Fatalf("PanelMiniAppShortName = %q, want the leading @ trimmed", cfg.PanelMiniAppShortName)
	}
	if cfg.PanelSessionMaxAge != 12*time.Hour {
		t.Fatalf("PanelSessionMaxAge = %v, want 12h", cfg.PanelSessionMaxAge)
	}
}

func TestPanelConfigValidation(t *testing.T) {
	t.Run("public URL required when enabled", func(t *testing.T) {
		setPanelTestRequiredEnv(t)
		t.Setenv("PANEL_ENABLED", "true")
		t.Setenv("PANEL_PUBLIC_URL", "")
		if _, err := LoadConfig(); err == nil {
			t.Fatal("expected an error for PANEL_ENABLED without PANEL_PUBLIC_URL")
		}
	})

	t.Run("http rejected unless debug", func(t *testing.T) {
		setPanelTestRequiredEnv(t)
		t.Setenv("PANEL_ENABLED", "true")
		t.Setenv("PANEL_PUBLIC_URL", "http://panel.example.com")
		t.Setenv("DEBUG", "false")
		if _, err := LoadConfig(); err == nil {
			t.Fatal("expected an error for a non-HTTPS PANEL_PUBLIC_URL")
		}

		t.Setenv("DEBUG", "true")
		if _, err := LoadConfig(); err != nil {
			t.Fatalf("unexpected error with DEBUG=true: %v", err)
		}
	})

	t.Run("invalid session age fails startup", func(t *testing.T) {
		setPanelTestRequiredEnv(t)
		t.Setenv("PANEL_SESSION_MAX_AGE", "24 hours")
		if _, err := LoadConfig(); err == nil {
			t.Fatal("expected an error for a non-duration PANEL_SESSION_MAX_AGE")
		}
	})
}

func TestRedisEnvParsingPure(t *testing.T) {
	tests := []struct {
		name         string
		redisAddr    string
		redisPass    string
		redisURL     string
		wantAddr     string
		wantPassword string
	}{
		{
			name:         "explicit address and password take precedence",
			redisAddr:    "redis.internal:6379",
			redisPass:    "direct-password",
			redisURL:     "redis://:url-password@example.com:6380",
			wantAddr:     "redis.internal:6379",
			wantPassword: "direct-password",
		},
		{
			name:      "explicit address ignores URL credentials",
			redisAddr: "redis.internal:6379",
			redisURL:  "redis://:stale@example.com:6380",
			wantAddr:  "redis.internal:6379",
		},
		{
			name:         "url supplies host and password",
			redisURL:     "redis://:secret@example.com:6380/1",
			wantAddr:     "example.com:6380",
			wantPassword: "secret",
		},
		{
			name:         "url with username supplies its password",
			redisURL:     "redis://user:pass123@host:6380",
			wantAddr:     "host:6380",
			wantPassword: "pass123",
		},
		{
			name:     "url without userinfo has no password",
			redisURL: "redis://host:6380",
			wantAddr: "host:6380",
		},
		{
			name:     "url with username only has no password",
			redisURL: "redis://user@host:6380",
			wantAddr: "host:6380",
		},
		{
			name:     "invalid url returns empty values",
			redisURL: "://bad-url",
		},
		{
			name:     "invalid percent-encoding returns empty values",
			redisURL: "not-a-valid-url-%%%",
		},
		{
			name: "empty env returns empty values",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("REDIS_ADDRESS", tc.redisAddr)
			t.Setenv("REDIS_PASSWORD", tc.redisPass)
			t.Setenv("REDIS_URL", tc.redisURL)

			if got := getRedisAddress(); got != tc.wantAddr {
				t.Fatalf("getRedisAddress() = %q, want %q", got, tc.wantAddr)
			}
			if got := getRedisPassword(); got != tc.wantPassword {
				t.Fatalf("getRedisPassword() = %q, want %q", got, tc.wantPassword)
			}
		})
	}
}
