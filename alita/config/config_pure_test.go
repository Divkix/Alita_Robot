package config

import (
	"os"
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
		if cfg.BotVersion != "2.1.3" {
			t.Errorf("BotVersion: got %q, want %q", cfg.BotVersion, "2.1.3")
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
