package config

import (
	"os"
	"testing"
)

func skipIfNoConfig(t *testing.T) {
	t.Helper()
	if os.Getenv("BOT_TOKEN") == "" {
		t.Skip("skipping: BOT_TOKEN not set (config.init() would fatalf)")
	}
}

func validBaseConfig() *Config {
	return &Config{
		BotToken:                "test-token",
		OwnerId:                 1,
		MessageDump:             1,
		DatabaseURL:             "postgres://localhost/test",
		RedisAddress:            "localhost:6379",
		HTTPPort:                8080,
		ChatValidationWorkers:   10,
		DatabaseWorkers:         5,
		MessagePipelineWorkers:  4,
		BulkOperationWorkers:    4,
		CacheWorkers:            3,
		StatsCollectionWorkers:  2,
		MaxConcurrentOperations: 50,
		OperationTimeoutSeconds: 30,
	}
}

func TestValidateConfig(t *testing.T) {
	t.Parallel()
	skipIfNoConfig(t)

	tests := []struct {
		name    string
		setup   func(*Config)
		wantErr bool
	}{
		// Required field validations
		{
			name:    "valid base config succeeds",
			setup:   func(_ *Config) {},
			wantErr: false,
		},
		{
			name:    "empty BotToken returns error",
			setup:   func(c *Config) { c.BotToken = "" },
			wantErr: true,
		},
		{
			name:    "OwnerId zero returns error",
			setup:   func(c *Config) { c.OwnerId = 0 },
			wantErr: true,
		},
		{
			name:    "MessageDump zero returns error",
			setup:   func(c *Config) { c.MessageDump = 0 },
			wantErr: true,
		},
		{
			name:    "empty DatabaseURL returns error",
			setup:   func(c *Config) { c.DatabaseURL = "" },
			wantErr: true,
		},
		{
			name:    "empty RedisAddress returns error",
			setup:   func(c *Config) { c.RedisAddress = "" },
			wantErr: true,
		},
		// Webhook validations
		{
			name: "UseWebhooks with empty domain returns error",
			setup: func(c *Config) {
				c.UseWebhooks = true
				c.WebhookDomain = ""
				c.WebhookSecret = "secret"
			},
			wantErr: true,
		},
		{
			name: "UseWebhooks with empty secret returns error",
			setup: func(c *Config) {
				c.UseWebhooks = true
				c.WebhookDomain = "example.com"
				c.WebhookSecret = ""
			},
			wantErr: true,
		},
		{
			name: "UseWebhooks false with no domain succeeds",
			setup: func(c *Config) {
				c.UseWebhooks = false
				c.WebhookDomain = ""
				c.WebhookSecret = ""
			},
			wantErr: false,
		},
		{
			name: "UseWebhooks true with both domain and secret succeeds",
			setup: func(c *Config) {
				c.UseWebhooks = true
				c.WebhookDomain = "example.com"
				c.WebhookSecret = "mysecret"
			},
			wantErr: false,
		},
		// HTTP port validations
		{
			name:    "HTTPPort zero returns error",
			setup:   func(c *Config) { c.HTTPPort = 0 },
			wantErr: true,
		},
		{
			name:    "HTTPPort 70000 returns error",
			setup:   func(c *Config) { c.HTTPPort = 70000 },
			wantErr: true,
		},
		{
			name:    "HTTPPort 65535 succeeds",
			setup:   func(c *Config) { c.HTTPPort = 65535 },
			wantErr: false,
		},
		{
			name:    "HTTPPort 1 succeeds",
			setup:   func(c *Config) { c.HTTPPort = 1 },
			wantErr: false,
		},
		// Worker pool validations
		{
			name:    "ChatValidationWorkers zero returns error",
			setup:   func(c *Config) { c.ChatValidationWorkers = 0 },
			wantErr: true,
		},
		{
			name:    "ChatValidationWorkers 101 returns error",
			setup:   func(c *Config) { c.ChatValidationWorkers = 101 },
			wantErr: true,
		},
		{
			name:    "ChatValidationWorkers 1 succeeds",
			setup:   func(c *Config) { c.ChatValidationWorkers = 1 },
			wantErr: false,
		},
		{
			name:    "DatabaseWorkers zero returns error",
			setup:   func(c *Config) { c.DatabaseWorkers = 0 },
			wantErr: true,
		},
		{
			name:    "DatabaseWorkers 51 returns error",
			setup:   func(c *Config) { c.DatabaseWorkers = 51 },
			wantErr: true,
		},
		{
			name:    "MessagePipelineWorkers zero returns error",
			setup:   func(c *Config) { c.MessagePipelineWorkers = 0 },
			wantErr: true,
		},
		{
			name:    "MessagePipelineWorkers 51 returns error",
			setup:   func(c *Config) { c.MessagePipelineWorkers = 51 },
			wantErr: true,
		},
		{
			name:    "BulkOperationWorkers zero returns error",
			setup:   func(c *Config) { c.BulkOperationWorkers = 0 },
			wantErr: true,
		},
		{
			name:    "BulkOperationWorkers 21 returns error",
			setup:   func(c *Config) { c.BulkOperationWorkers = 21 },
			wantErr: true,
		},
		{
			name:    "CacheWorkers zero returns error",
			setup:   func(c *Config) { c.CacheWorkers = 0 },
			wantErr: true,
		},
		{
			name:    "CacheWorkers 21 returns error",
			setup:   func(c *Config) { c.CacheWorkers = 21 },
			wantErr: true,
		},
		{
			name:    "StatsCollectionWorkers zero returns error",
			setup:   func(c *Config) { c.StatsCollectionWorkers = 0 },
			wantErr: true,
		},
		{
			name:    "StatsCollectionWorkers 11 returns error",
			setup:   func(c *Config) { c.StatsCollectionWorkers = 11 },
			wantErr: true,
		},
		// Performance limit validations
		{
			name:    "MaxConcurrentOperations zero returns error",
			setup:   func(c *Config) { c.MaxConcurrentOperations = 0 },
			wantErr: true,
		},
		{
			name:    "MaxConcurrentOperations negative returns error",
			setup:   func(c *Config) { c.MaxConcurrentOperations = -1 },
			wantErr: true,
		},
		{
			name:    "MaxConcurrentOperations 1001 returns error",
			setup:   func(c *Config) { c.MaxConcurrentOperations = 1001 },
			wantErr: true,
		},
		{
			name:    "OperationTimeoutSeconds zero returns error",
			setup:   func(c *Config) { c.OperationTimeoutSeconds = 0 },
			wantErr: true,
		},
		{
			name:    "OperationTimeoutSeconds 301 returns error",
			setup:   func(c *Config) { c.OperationTimeoutSeconds = 301 },
			wantErr: true,
		},
		{
			name:    "OperationTimeoutSeconds 300 succeeds",
			setup:   func(c *Config) { c.OperationTimeoutSeconds = 300 },
			wantErr: false,
		},
		// Dispatcher optional field validation
		{
			name:    "DispatcherMaxRoutines zero is allowed",
			setup:   func(c *Config) { c.DispatcherMaxRoutines = 0 },
			wantErr: false,
		},
		{
			name:    "DispatcherMaxRoutines 1 succeeds",
			setup:   func(c *Config) { c.DispatcherMaxRoutines = 1 },
			wantErr: false,
		},
		{
			name:    "DispatcherMaxRoutines 1000 succeeds",
			setup:   func(c *Config) { c.DispatcherMaxRoutines = 1000 },
			wantErr: false,
		},
		// DB pool optional field validation
		{
			name:    "DBMaxIdleConns 101 returns error",
			setup:   func(c *Config) { c.DBMaxIdleConns = 101 },
			wantErr: true,
		},
		{
			name:    "DBMaxIdleConns 0 is allowed (optional field)",
			setup:   func(c *Config) { c.DBMaxIdleConns = 0 },
			wantErr: false,
		},
		{
			name:    "DBMaxIdleConns 100 succeeds",
			setup:   func(c *Config) { c.DBMaxIdleConns = 100 },
			wantErr: false,
		},
		{
			name:    "DBMaxOpenConns 1001 returns error",
			setup:   func(c *Config) { c.DBMaxOpenConns = 1001 },
			wantErr: true,
		},
		{
			name:    "DBMaxOpenConns 0 is allowed (optional field)",
			setup:   func(c *Config) { c.DBMaxOpenConns = 0 },
			wantErr: false,
		},
		{
			name:    "DBMaxOpenConns 1000 succeeds",
			setup:   func(c *Config) { c.DBMaxOpenConns = 1000 },
			wantErr: false,
		},
		// All workers at minimum 1 succeeds
		{
			name: "all workers at minimum 1 succeeds",
			setup: func(c *Config) {
				c.ChatValidationWorkers = 1
				c.DatabaseWorkers = 1
				c.MessagePipelineWorkers = 1
				c.BulkOperationWorkers = 1
				c.CacheWorkers = 1
				c.StatsCollectionWorkers = 1
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := validBaseConfig()
			tc.setup(cfg)

			err := ValidateConfig(cfg)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error but got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestSetDefaults(t *testing.T) {
	t.Parallel()
	skipIfNoConfig(t)

	t.Run("zero config gets defaults", func(t *testing.T) {
		t.Parallel()

		cfg := &Config{}
		cfg.setDefaults()

		if cfg.ApiServer != "https://api.telegram.org" {
			t.Errorf("ApiServer: got %q, want %q", cfg.ApiServer, "https://api.telegram.org")
		}
		if cfg.WorkingMode != "worker" {
			t.Errorf("WorkingMode: got %q, want %q", cfg.WorkingMode, "worker")
		}
		if cfg.RedisAddress != "localhost:6379" {
			t.Errorf("RedisAddress: got %q, want %q", cfg.RedisAddress, "localhost:6379")
		}
		if cfg.RedisDB != 1 {
			t.Errorf("RedisDB: got %d, want %d", cfg.RedisDB, 1)
		}
		if cfg.HTTPPort != 8080 {
			t.Errorf("HTTPPort: got %d, want %d", cfg.HTTPPort, 8080)
		}
		if cfg.ChatValidationWorkers != 10 {
			t.Errorf("ChatValidationWorkers: got %d, want %d", cfg.ChatValidationWorkers, 10)
		}
		if cfg.DatabaseWorkers != 5 {
			t.Errorf("DatabaseWorkers: got %d, want %d", cfg.DatabaseWorkers, 5)
		}
		if cfg.BulkOperationWorkers != 4 {
			t.Errorf("BulkOperationWorkers: got %d, want %d", cfg.BulkOperationWorkers, 4)
		}
		if cfg.CacheWorkers != 3 {
			t.Errorf("CacheWorkers: got %d, want %d", cfg.CacheWorkers, 3)
		}
		if cfg.StatsCollectionWorkers != 2 {
			t.Errorf("StatsCollectionWorkers: got %d, want %d", cfg.StatsCollectionWorkers, 2)
		}
		if cfg.DBMaxIdleConns != 50 {
			t.Errorf("DBMaxIdleConns: got %d, want %d", cfg.DBMaxIdleConns, 50)
		}
		if cfg.DBMaxOpenConns != 200 {
			t.Errorf("DBMaxOpenConns: got %d, want %d", cfg.DBMaxOpenConns, 200)
		}
		if cfg.DBConnMaxLifetimeMin != 240 {
			t.Errorf("DBConnMaxLifetimeMin: got %d, want %d", cfg.DBConnMaxLifetimeMin, 240)
		}
		if cfg.DBConnMaxIdleTimeMin != 60 {
			t.Errorf("DBConnMaxIdleTimeMin: got %d, want %d", cfg.DBConnMaxIdleTimeMin, 60)
		}
		if cfg.MigrationsPath != "migrations" {
			t.Errorf("MigrationsPath: got %q, want %q", cfg.MigrationsPath, "migrations")
		}
		if !cfg.ClearCacheOnStartup {
			t.Errorf("ClearCacheOnStartup: got false, want true")
		}
		if cfg.MaxConcurrentOperations != 50 {
			t.Errorf("MaxConcurrentOperations: got %d, want %d", cfg.MaxConcurrentOperations, 50)
		}
		if cfg.OperationTimeoutSeconds != 30 {
			t.Errorf("OperationTimeoutSeconds: got %d, want %d", cfg.OperationTimeoutSeconds, 30)
		}
		if cfg.DispatcherMaxRoutines != 200 {
			t.Errorf("DispatcherMaxRoutines: got %d, want %d", cfg.DispatcherMaxRoutines, 200)
		}
		if cfg.ResourceMaxGoroutines != 1000 {
			t.Errorf("ResourceMaxGoroutines: got %d, want %d", cfg.ResourceMaxGoroutines, 1000)
		}
		if cfg.ResourceMaxMemoryMB != 500 {
			t.Errorf("ResourceMaxMemoryMB: got %d, want %d", cfg.ResourceMaxMemoryMB, 500)
		}
		if cfg.ResourceGCThresholdMB != 400 {
			t.Errorf("ResourceGCThresholdMB: got %d, want %d", cfg.ResourceGCThresholdMB, 400)
		}
	})

	t.Run("pre-set ApiServer preserved", func(t *testing.T) {
		t.Parallel()

		cfg := &Config{ApiServer: "custom"}
		cfg.setDefaults()

		if cfg.ApiServer != "custom" {
			t.Errorf("ApiServer: got %q, want %q", cfg.ApiServer, "custom")
		}
	})

	t.Run("pre-set RedisDB preserved", func(t *testing.T) {
		t.Parallel()

		cfg := &Config{RedisDB: 5}
		cfg.setDefaults()

		if cfg.RedisDB != 5 {
			t.Errorf("RedisDB: got %d, want %d", cfg.RedisDB, 5)
		}
	})

	t.Run("pre-set HTTPPort preserved", func(t *testing.T) {
		t.Parallel()

		cfg := &Config{HTTPPort: 3000}
		cfg.setDefaults()

		if cfg.HTTPPort != 3000 {
			t.Errorf("HTTPPort: got %d, want %d", cfg.HTTPPort, 3000)
		}
	})

	t.Run("backward compat WebhookPort", func(t *testing.T) {
		t.Parallel()

		cfg := &Config{WebhookPort: 9090, HTTPPort: 0}
		cfg.setDefaults()

		if cfg.HTTPPort != 9090 {
			t.Errorf("HTTPPort: got %d, want %d (expected WebhookPort to be used)", cfg.HTTPPort, 9090)
		}
	})

	t.Run("ClearCacheOnStartup unconditional", func(t *testing.T) {
		t.Parallel()

		cfg := &Config{ClearCacheOnStartup: false}
		cfg.setDefaults()

		if !cfg.ClearCacheOnStartup {
			t.Errorf("ClearCacheOnStartup: got false, want true (setDefaults always sets this to true)")
		}
	})

	t.Run("Debug false enables monitoring", func(t *testing.T) {
		t.Parallel()

		cfg := &Config{Debug: false}
		cfg.setDefaults()

		if !cfg.EnablePerformanceMonitoring {
			t.Errorf("EnablePerformanceMonitoring: got false, want true when Debug=false")
		}
		if !cfg.EnableBackgroundStats {
			t.Errorf("EnableBackgroundStats: got false, want true when Debug=false")
		}
	})
}

// TestGetRedisAddress tests the getRedisAddress() helper. The top-level test does
// NOT call t.Parallel() because t.Setenv() (used in subtests) is incompatible with
// parallel execution at the enclosing level — Go enforces this at runtime.
func TestGetRedisAddress(t *testing.T) {
	skipIfNoConfig(t)

	t.Run("REDIS_ADDRESS set returns it directly", func(t *testing.T) {
		t.Setenv("REDIS_ADDRESS", "myhost:1234")
		t.Setenv("REDIS_URL", "")
		t.Setenv("REDIS_PASSWORD", "")

		got := getRedisAddress()
		if got != "myhost:1234" {
			t.Errorf("got %q, want %q", got, "myhost:1234")
		}
	})

	t.Run("REDIS_ADDRESS empty falls back to REDIS_URL host:port", func(t *testing.T) {
		t.Setenv("REDIS_ADDRESS", "")
		t.Setenv("REDIS_URL", "redis://user:pass@host:6380")
		t.Setenv("REDIS_PASSWORD", "")

		got := getRedisAddress()
		if got != "host:6380" {
			t.Errorf("got %q, want %q", got, "host:6380")
		}
	})

	t.Run("both REDIS_ADDRESS and REDIS_URL empty returns empty string", func(t *testing.T) {
		t.Setenv("REDIS_ADDRESS", "")
		t.Setenv("REDIS_URL", "")
		t.Setenv("REDIS_PASSWORD", "")

		got := getRedisAddress()
		if got != "" {
			t.Errorf("got %q, want empty string", got)
		}
	})

	t.Run("REDIS_ADDRESS takes priority over REDIS_URL", func(t *testing.T) {
		t.Setenv("REDIS_ADDRESS", "x")
		t.Setenv("REDIS_URL", "redis://host:9999")
		t.Setenv("REDIS_PASSWORD", "")

		got := getRedisAddress()
		if got != "x" {
			t.Errorf("got %q, want %q", got, "x")
		}
	})

	t.Run("REDIS_URL with invalid percent-encoding returns empty string", func(t *testing.T) {
		t.Setenv("REDIS_ADDRESS", "")
		t.Setenv("REDIS_URL", "not-a-valid-url-%%%")
		t.Setenv("REDIS_PASSWORD", "")

		got := getRedisAddress()
		if got != "" {
			t.Errorf("got %q, want empty string on parse error", got)
		}
	})
}

// TestGetRedisPassword tests the getRedisPassword() helper. The top-level test does
// NOT call t.Parallel() because t.Setenv() (used in subtests) is incompatible with
// parallel execution at the enclosing level — Go enforces this at runtime.
func TestGetRedisPassword(t *testing.T) {
	skipIfNoConfig(t)

	t.Run("REDIS_PASSWORD set returns it directly", func(t *testing.T) {
		t.Setenv("REDIS_PASSWORD", "secret")
		t.Setenv("REDIS_URL", "")
		t.Setenv("REDIS_ADDRESS", "")

		got := getRedisPassword()
		if got != "secret" {
			t.Errorf("got %q, want %q", got, "secret")
		}
	})

	t.Run("REDIS_PASSWORD empty extracts password from REDIS_URL", func(t *testing.T) {
		t.Setenv("REDIS_PASSWORD", "")
		t.Setenv("REDIS_URL", "redis://user:pass123@host:6380")
		t.Setenv("REDIS_ADDRESS", "")

		got := getRedisPassword()
		if got != "pass123" {
			t.Errorf("got %q, want %q", got, "pass123")
		}
	})

	t.Run("both REDIS_PASSWORD and REDIS_URL empty returns empty string", func(t *testing.T) {
		t.Setenv("REDIS_PASSWORD", "")
		t.Setenv("REDIS_URL", "")
		t.Setenv("REDIS_ADDRESS", "")

		got := getRedisPassword()
		if got != "" {
			t.Errorf("got %q, want empty string", got)
		}
	})

	t.Run("REDIS_URL without userinfo returns empty string", func(t *testing.T) {
		t.Setenv("REDIS_PASSWORD", "")
		t.Setenv("REDIS_URL", "redis://host:6380")
		t.Setenv("REDIS_ADDRESS", "")

		got := getRedisPassword()
		if got != "" {
			t.Errorf("got %q, want empty string when no userinfo in URL", got)
		}
	})

	t.Run("REDIS_URL with username but no password returns empty string", func(t *testing.T) {
		t.Setenv("REDIS_PASSWORD", "")
		t.Setenv("REDIS_URL", "redis://user@host:6380")
		t.Setenv("REDIS_ADDRESS", "")

		got := getRedisPassword()
		if got != "" {
			t.Errorf("got %q, want empty string when user has no password", got)
		}
	})
}
