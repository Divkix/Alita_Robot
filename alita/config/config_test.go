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
