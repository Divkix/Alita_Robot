package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

// TestIsCliModeActive tests the isCliModeActive helper. It does NOT call
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

// TestLoadConfig tests the LoadConfig helper. The top-level test does NOT call
// t.Parallel() because t.Setenv() is incompatible with parallel execution.
func TestLoadConfig(t *testing.T) {
	skipIfNoConfig(t)

	t.Run("returns error when required fields missing", func(t *testing.T) {
		// Ensure required env vars are empty
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
		t.Setenv("CHAT_VALIDATION_WORKERS", "5")
		t.Setenv("DATABASE_WORKERS", "3")
		t.Setenv("MESSAGE_PIPELINE_WORKERS", "2")
		t.Setenv("BULK_OPERATION_WORKERS", "1")
		t.Setenv("CACHE_WORKERS", "1")
		t.Setenv("STATS_COLLECTION_WORKERS", "1")
		t.Setenv("MAX_CONCURRENT_OPERATIONS", "20")
		t.Setenv("OPERATION_TIMEOUT_SECONDS", "10")

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
		if cfg.ChatValidationWorkers != 5 {
			t.Errorf("ChatValidationWorkers: got %d, want %d", cfg.ChatValidationWorkers, 5)
		}
		if cfg.OperationTimeout != 10*time.Second {
			t.Errorf("OperationTimeout: got %v, want %v", cfg.OperationTimeout, 10*time.Second)
		}
		// Defaults should have been applied
		if cfg.ApiServer != "https://api.telegram.org" {
			t.Errorf("ApiServer: got %q, want %q", cfg.ApiServer, "https://api.telegram.org")
		}
		if cfg.BotVersion == "" {
			t.Errorf("BotVersion: got empty string, want non-empty")
		}
		// AllowedUpdates should be populated
		if len(cfg.AllowedUpdates) == 0 {
			t.Errorf("AllowedUpdates: expected non-empty slice")
		}
		// ValidLangCodes should default to ["en"]
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
		t.Setenv("CHAT_VALIDATION_WORKERS", "1")
		t.Setenv("DATABASE_WORKERS", "1")
		t.Setenv("MESSAGE_PIPELINE_WORKERS", "1")
		t.Setenv("BULK_OPERATION_WORKERS", "1")
		t.Setenv("CACHE_WORKERS", "1")
		t.Setenv("STATS_COLLECTION_WORKERS", "1")
		t.Setenv("MAX_CONCURRENT_OPERATIONS", "1")
		t.Setenv("OPERATION_TIMEOUT_SECONDS", "1")

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
		t.Setenv("CHAT_VALIDATION_WORKERS", "1")
		t.Setenv("DATABASE_WORKERS", "1")
		t.Setenv("MESSAGE_PIPELINE_WORKERS", "1")
		t.Setenv("BULK_OPERATION_WORKERS", "1")
		t.Setenv("CACHE_WORKERS", "1")
		t.Setenv("STATS_COLLECTION_WORKERS", "1")
		t.Setenv("MAX_CONCURRENT_OPERATIONS", "1")
		t.Setenv("OPERATION_TIMEOUT_SECONDS", "1")

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !cfg.EnablePPROF {
			t.Errorf("EnablePPROF: got false, want true")
		}
	})
}

func TestValidateConfigPure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(*Config)
		wantErr string
	}{
		{name: "valid base config", setup: func(_ *Config) {}},
		{
			name:    "missing bot token",
			setup:   func(c *Config) { c.BotToken = "" },
			wantErr: "BOT_TOKEN is required",
		},
		{
			name:    "missing owner",
			setup:   func(c *Config) { c.OwnerId = 0 },
			wantErr: "OWNER_ID is required",
		},
		{
			name:    "missing message dump",
			setup:   func(c *Config) { c.MessageDump = 0 },
			wantErr: "MESSAGE_DUMP is required",
		},
		{
			name:    "missing database URL",
			setup:   func(c *Config) { c.DatabaseURL = "" },
			wantErr: "DATABASE_URL is required",
		},
		{
			name:    "missing redis address",
			setup:   func(c *Config) { c.RedisAddress = "" },
			wantErr: "REDIS_ADDRESS or REDIS_URL is required",
		},
		{
			name: "webhook requires domain",
			setup: func(c *Config) {
				c.UseWebhooks = true
				c.WebhookDomain = ""
				c.WebhookSecret = "secret"
			},
			wantErr: "WEBHOOK_DOMAIN is required",
		},
		{
			name: "webhook requires secret",
			setup: func(c *Config) {
				c.UseWebhooks = true
				c.WebhookDomain = "https://example.com"
				c.WebhookSecret = ""
			},
			wantErr: "WEBHOOK_SECRET is required",
		},
		{
			name:    "invalid HTTP port",
			setup:   func(c *Config) { c.HTTPPort = 70000 },
			wantErr: "HTTP_PORT must be between 1 and 65535",
		},
		{
			name:    "invalid chat validation workers",
			setup:   func(c *Config) { c.ChatValidationWorkers = 101 },
			wantErr: "CHAT_VALIDATION_WORKERS must be between 1 and 100",
		},
		{
			name:    "invalid database workers",
			setup:   func(c *Config) { c.DatabaseWorkers = 51 },
			wantErr: "DATABASE_WORKERS must be between 1 and 50",
		},
		{
			name:    "invalid message pipeline workers",
			setup:   func(c *Config) { c.MessagePipelineWorkers = 51 },
			wantErr: "MESSAGE_PIPELINE_WORKERS must be between 1 and 50",
		},
		{
			name:    "invalid bulk workers",
			setup:   func(c *Config) { c.BulkOperationWorkers = 21 },
			wantErr: "BULK_OPERATION_WORKERS must be between 1 and 20",
		},
		{
			name:    "invalid cache workers",
			setup:   func(c *Config) { c.CacheWorkers = 21 },
			wantErr: "CACHE_WORKERS must be between 1 and 20",
		},
		{
			name:    "invalid stats workers",
			setup:   func(c *Config) { c.StatsCollectionWorkers = 11 },
			wantErr: "STATS_COLLECTION_WORKERS must be between 1 and 10",
		},
		{
			name:    "invalid max concurrent operations",
			setup:   func(c *Config) { c.MaxConcurrentOperations = 1001 },
			wantErr: "MAX_CONCURRENT_OPERATIONS must be between 1 and 1000",
		},
		{
			name:    "invalid operation timeout",
			setup:   func(c *Config) { c.OperationTimeoutSeconds = 301 },
			wantErr: "OPERATION_TIMEOUT_SECONDS must be between 1 and 300",
		},
		{
			name:    "invalid dispatcher routines",
			setup:   func(c *Config) { c.DispatcherMaxRoutines = 1001 },
			wantErr: "DISPATCHER_MAX_ROUTINES must be between 1 and 1000",
		},
		{
			name:    "invalid idle connections",
			setup:   func(c *Config) { c.DBMaxIdleConns = 101 },
			wantErr: "DB_MAX_IDLE_CONNS must be between 1 and 100",
		},
		{
			name:    "invalid open connections",
			setup:   func(c *Config) { c.DBMaxOpenConns = 1001 },
			wantErr: "DB_MAX_OPEN_CONNS must be between 1 and 1000",
		},
		{
			name:    "invalid connection lifetime",
			setup:   func(c *Config) { c.DBConnMaxLifetimeMin = 1441 },
			wantErr: "DB_CONN_MAX_LIFETIME_MIN must be between 1 and 1440 minutes",
		},
		{
			name:    "invalid idle time",
			setup:   func(c *Config) { c.DBConnMaxIdleTimeMin = 61 },
			wantErr: "DB_CONN_MAX_IDLE_TIME_MIN must be between 1 and 60 minutes",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := validBaseConfig()
			cfg.DispatcherMaxRoutines = 200
			cfg.DBMaxIdleConns = 50
			cfg.DBMaxOpenConns = 200
			cfg.DBConnMaxLifetimeMin = 240
			cfg.DBConnMaxIdleTimeMin = 60
			tc.setup(cfg)

			err := ValidateConfig(cfg)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateConfig() error = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("ValidateConfig() error = nil, want %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("ValidateConfig() error = %q, want substring %q", err.Error(), tc.wantErr)
			}
		})
	}
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
			name:         "url supplies host and password",
			redisURL:     "redis://:secret@example.com:6380/1",
			wantAddr:     "example.com:6380",
			wantPassword: "secret",
		},
		{
			name:     "invalid url returns empty values",
			redisURL: "://bad-url",
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
