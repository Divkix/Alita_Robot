package config

import (
	"strings"
	"testing"
)

// validConfig returns a minimal Config that passes ValidateConfig.
func validConfig() *Config {
	return &Config{
		BotToken:                "test-token",
		OwnerId:                 12345,
		MessageDump:             67890,
		DatabaseURL:             "postgres://localhost/db",
		RedisAddress:            "localhost:6379",
		HTTPPort:                8080,
		ChatValidationWorkers:   5,
		DatabaseWorkers:         5,
		MessagePipelineWorkers:  5,
		BulkOperationWorkers:    5,
		CacheWorkers:            5,
		StatsCollectionWorkers:  5,
		MaxConcurrentOperations: 50,
		OperationTimeoutSeconds: 30,
	}
}

// --- ValidateConfig tests ---

func TestValidateConfig_Valid(t *testing.T) {
	if err := ValidateConfig(validConfig()); err != nil {
		t.Errorf("ValidateConfig(validConfig()) unexpected error: %v", err)
	}
}

func TestValidateConfig_MissingBotToken(t *testing.T) {
	cfg := validConfig()
	cfg.BotToken = ""
	err := ValidateConfig(cfg)
	if err == nil || !strings.Contains(err.Error(), "BOT_TOKEN") {
		t.Errorf("expected BOT_TOKEN error, got: %v", err)
	}
}

func TestValidateConfig_ZeroOwnerId(t *testing.T) {
	cfg := validConfig()
	cfg.OwnerId = 0
	err := ValidateConfig(cfg)
	if err == nil || !strings.Contains(err.Error(), "OWNER_ID") {
		t.Errorf("expected OWNER_ID error, got: %v", err)
	}
}

func TestValidateConfig_ZeroMessageDump(t *testing.T) {
	cfg := validConfig()
	cfg.MessageDump = 0
	err := ValidateConfig(cfg)
	if err == nil || !strings.Contains(err.Error(), "MESSAGE_DUMP") {
		t.Errorf("expected MESSAGE_DUMP error, got: %v", err)
	}
}

func TestValidateConfig_MissingDatabaseURL(t *testing.T) {
	cfg := validConfig()
	cfg.DatabaseURL = ""
	err := ValidateConfig(cfg)
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Errorf("expected DATABASE_URL error, got: %v", err)
	}
}

func TestValidateConfig_MissingRedisAddress(t *testing.T) {
	cfg := validConfig()
	cfg.RedisAddress = ""
	err := ValidateConfig(cfg)
	if err == nil || !strings.Contains(err.Error(), "REDIS_ADDRESS") {
		t.Errorf("expected REDIS_ADDRESS error, got: %v", err)
	}
}

func TestValidateConfig_WebhooksWithoutDomain(t *testing.T) {
	cfg := validConfig()
	cfg.UseWebhooks = true
	cfg.WebhookDomain = ""
	cfg.WebhookSecret = "secret"
	err := ValidateConfig(cfg)
	if err == nil || !strings.Contains(err.Error(), "WEBHOOK_DOMAIN") {
		t.Errorf("expected WEBHOOK_DOMAIN error, got: %v", err)
	}
}

func TestValidateConfig_WebhooksWithoutSecret(t *testing.T) {
	cfg := validConfig()
	cfg.UseWebhooks = true
	cfg.WebhookDomain = "example.com"
	cfg.WebhookSecret = ""
	err := ValidateConfig(cfg)
	if err == nil || !strings.Contains(err.Error(), "WEBHOOK_SECRET") {
		t.Errorf("expected WEBHOOK_SECRET error, got: %v", err)
	}
}

func TestValidateConfig_WebhooksValid(t *testing.T) {
	cfg := validConfig()
	cfg.UseWebhooks = true
	cfg.WebhookDomain = "example.com"
	cfg.WebhookSecret = "secret"
	if err := ValidateConfig(cfg); err != nil {
		t.Errorf("unexpected error for valid webhook config: %v", err)
	}
}

func TestValidateConfig_InvalidHTTPPort_Zero(t *testing.T) {
	cfg := validConfig()
	cfg.HTTPPort = 0
	err := ValidateConfig(cfg)
	if err == nil || !strings.Contains(err.Error(), "HTTP_PORT") {
		t.Errorf("expected HTTP_PORT error, got: %v", err)
	}
}

func TestValidateConfig_InvalidHTTPPort_TooHigh(t *testing.T) {
	cfg := validConfig()
	cfg.HTTPPort = 65536
	err := ValidateConfig(cfg)
	if err == nil || !strings.Contains(err.Error(), "HTTP_PORT") {
		t.Errorf("expected HTTP_PORT error for 65536, got: %v", err)
	}
}

func TestValidateConfig_WorkerBounds(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
		errMsg string
	}{
		{"ChatValidation=0", func(c *Config) { c.ChatValidationWorkers = 0 }, "CHAT_VALIDATION"},
		{"ChatValidation=101", func(c *Config) { c.ChatValidationWorkers = 101 }, "CHAT_VALIDATION"},
		{"Database=0", func(c *Config) { c.DatabaseWorkers = 0 }, "DATABASE_WORKERS"},
		{"Database=51", func(c *Config) { c.DatabaseWorkers = 51 }, "DATABASE_WORKERS"},
		{"Pipeline=0", func(c *Config) { c.MessagePipelineWorkers = 0 }, "MESSAGE_PIPELINE"},
		{"Bulk=21", func(c *Config) { c.BulkOperationWorkers = 21 }, "BULK_OPERATION"},
		{"Cache=21", func(c *Config) { c.CacheWorkers = 21 }, "CACHE_WORKERS"},
		{"Stats=11", func(c *Config) { c.StatsCollectionWorkers = 11 }, "STATS_COLLECTION"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.mutate(cfg)
			err := ValidateConfig(cfg)
			if err == nil || !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("expected error containing %q, got: %v", tt.errMsg, err)
			}
		})
	}
}

func TestValidateConfig_MaxConcurrentOps_Zero(t *testing.T) {
	cfg := validConfig()
	cfg.MaxConcurrentOperations = 0
	err := ValidateConfig(cfg)
	if err == nil || !strings.Contains(err.Error(), "MAX_CONCURRENT") {
		t.Errorf("expected MAX_CONCURRENT error, got: %v", err)
	}
}

func TestValidateConfig_OperationTimeout_Zero(t *testing.T) {
	cfg := validConfig()
	cfg.OperationTimeoutSeconds = 0
	err := ValidateConfig(cfg)
	if err == nil || !strings.Contains(err.Error(), "OPERATION_TIMEOUT") {
		t.Errorf("expected OPERATION_TIMEOUT error, got: %v", err)
	}
}

func TestValidateConfig_DispatcherRoutines_OutOfRange(t *testing.T) {
	cfg := validConfig()
	cfg.DispatcherMaxRoutines = 1001
	err := ValidateConfig(cfg)
	if err == nil || !strings.Contains(err.Error(), "DISPATCHER") {
		t.Errorf("expected DISPATCHER error, got: %v", err)
	}
}

func TestValidateConfig_DispatcherRoutines_ZeroIsOK(t *testing.T) {
	cfg := validConfig()
	cfg.DispatcherMaxRoutines = 0 // 0 means "use default", so validation skips it
	if err := ValidateConfig(cfg); err != nil {
		t.Errorf("unexpected error for DispatcherMaxRoutines=0: %v", err)
	}
}

// --- setDefaults tests ---

func TestSetDefaults_ApiServer(t *testing.T) {
	cfg := &Config{}
	cfg.setDefaults()
	if cfg.ApiServer != "https://api.telegram.org" {
		t.Errorf("ApiServer = %q, want default", cfg.ApiServer)
	}
}

func TestSetDefaults_ApiServer_NotOverwritten(t *testing.T) {
	cfg := &Config{ApiServer: "https://custom.api"}
	cfg.setDefaults()
	if cfg.ApiServer != "https://custom.api" {
		t.Errorf("ApiServer was overwritten, got %q", cfg.ApiServer)
	}
}

func TestSetDefaults_HTTPPort(t *testing.T) {
	cfg := &Config{}
	cfg.setDefaults()
	if cfg.HTTPPort != 8080 {
		t.Errorf("HTTPPort = %d, want 8080", cfg.HTTPPort)
	}
}

func TestSetDefaults_HTTPPort_NotOverwritten(t *testing.T) {
	cfg := &Config{HTTPPort: 9090}
	cfg.setDefaults()
	if cfg.HTTPPort != 9090 {
		t.Errorf("HTTPPort was overwritten, got %d", cfg.HTTPPort)
	}
}

func TestSetDefaults_WorkerDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.setDefaults()
	if cfg.ChatValidationWorkers != 10 {
		t.Errorf("ChatValidationWorkers = %d, want 10", cfg.ChatValidationWorkers)
	}
	if cfg.DatabaseWorkers != 5 {
		t.Errorf("DatabaseWorkers = %d, want 5", cfg.DatabaseWorkers)
	}
	if cfg.BulkOperationWorkers != 4 {
		t.Errorf("BulkOperationWorkers = %d, want 4", cfg.BulkOperationWorkers)
	}
	if cfg.CacheWorkers != 3 {
		t.Errorf("CacheWorkers = %d, want 3", cfg.CacheWorkers)
	}
	if cfg.StatsCollectionWorkers != 2 {
		t.Errorf("StatsCollectionWorkers = %d, want 2", cfg.StatsCollectionWorkers)
	}
}

func TestSetDefaults_MigrationsPath(t *testing.T) {
	cfg := &Config{}
	cfg.setDefaults()
	if cfg.MigrationsPath != "migrations" {
		t.Errorf("MigrationsPath = %q, want %q", cfg.MigrationsPath, "migrations")
	}
}

func TestSetDefaults_MigrationsPath_NotOverwritten(t *testing.T) {
	cfg := &Config{MigrationsPath: "/custom/path"}
	cfg.setDefaults()
	if cfg.MigrationsPath != "/custom/path" {
		t.Errorf("MigrationsPath overwritten, got %q", cfg.MigrationsPath)
	}
}

func TestSetDefaults_DBConnectionPool(t *testing.T) {
	cfg := &Config{}
	cfg.setDefaults()
	if cfg.DBMaxIdleConns != 50 {
		t.Errorf("DBMaxIdleConns = %d, want 50", cfg.DBMaxIdleConns)
	}
	if cfg.DBMaxOpenConns != 200 {
		t.Errorf("DBMaxOpenConns = %d, want 200", cfg.DBMaxOpenConns)
	}
}

func TestSetDefaults_RedisDB(t *testing.T) {
	cfg := &Config{}
	cfg.setDefaults()
	if cfg.RedisDB != 1 {
		t.Errorf("RedisDB = %d, want 1", cfg.RedisDB)
	}
}

func TestSetDefaults_InactivityThreshold(t *testing.T) {
	cfg := &Config{}
	cfg.setDefaults()
	if cfg.InactivityThresholdDays != 30 {
		t.Errorf("InactivityThresholdDays = %d, want 30", cfg.InactivityThresholdDays)
	}
	if cfg.ActivityCheckInterval != 1 {
		t.Errorf("ActivityCheckInterval = %d, want 1", cfg.ActivityCheckInterval)
	}
}

// --- Redis helper tests ---
// NOTE: must NOT use t.Parallel() for env var tests

func TestGetRedisAddress_DirectEnv(t *testing.T) {
	t.Setenv("REDIS_ADDRESS", "myhost:6380")
	t.Setenv("REDIS_URL", "")
	got := getRedisAddress()
	if got != "myhost:6380" {
		t.Errorf("getRedisAddress() = %q, want %q", got, "myhost:6380")
	}
}

func TestGetRedisAddress_FallbackURL(t *testing.T) {
	t.Setenv("REDIS_ADDRESS", "")
	t.Setenv("REDIS_URL", "redis://user:pass@redishost:6379")
	got := getRedisAddress()
	if got != "redishost:6379" {
		t.Errorf("getRedisAddress() = %q, want %q", got, "redishost:6379")
	}
}

func TestGetRedisAddress_Empty(t *testing.T) {
	t.Setenv("REDIS_ADDRESS", "")
	t.Setenv("REDIS_URL", "")
	got := getRedisAddress()
	if got != "" {
		t.Errorf("getRedisAddress() = %q, want empty", got)
	}
}

func TestGetRedisAddress_InvalidURL(t *testing.T) {
	t.Setenv("REDIS_ADDRESS", "")
	t.Setenv("REDIS_URL", "://bad url")
	got := getRedisAddress()
	// Invalid URL: should return empty or best-effort (implementation logs warning)
	_ = got // Just verify no panic
}

func TestGetRedisPassword_DirectEnv(t *testing.T) {
	t.Setenv("REDIS_PASSWORD", "secret123")
	t.Setenv("REDIS_URL", "")
	got := getRedisPassword()
	if got != "secret123" {
		t.Errorf("getRedisPassword() = %q, want %q", got, "secret123")
	}
}

func TestGetRedisPassword_FallbackURL(t *testing.T) {
	t.Setenv("REDIS_PASSWORD", "")
	t.Setenv("REDIS_URL", "redis://user:mypassword@host:6379")
	got := getRedisPassword()
	if got != "mypassword" {
		t.Errorf("getRedisPassword() = %q, want %q", got, "mypassword")
	}
}

func TestGetRedisPassword_NoPassword(t *testing.T) {
	t.Setenv("REDIS_PASSWORD", "")
	t.Setenv("REDIS_URL", "redis://host:6379")
	got := getRedisPassword()
	if got != "" {
		t.Errorf("getRedisPassword() = %q, want empty", got)
	}
}

func TestGetRedisPassword_Empty(t *testing.T) {
	t.Setenv("REDIS_PASSWORD", "")
	t.Setenv("REDIS_URL", "")
	got := getRedisPassword()
	if got != "" {
		t.Errorf("getRedisPassword() = %q, want empty", got)
	}
}
