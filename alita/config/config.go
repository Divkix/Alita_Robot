package config

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"runtime"
	"time"

	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
)

// getRedisAddress returns the Redis address from REDIS_ADDRESS or parses it from REDIS_URL (Heroku format)
// REDIS_URL format: redis://user:password@host:port
// Returns: host:port
func getRedisAddress() string {
	if addr := os.Getenv("REDIS_ADDRESS"); addr != "" {
		return addr
	}

	// Fallback to parsing REDIS_URL (Heroku provides this)
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		return ""
	}

	parsed, err := url.Parse(redisURL)
	if err != nil {
		log.Warnf("Failed to parse REDIS_URL: %v", err)
		return ""
	}

	return parsed.Host
}

// getRedisPassword returns the Redis password from REDIS_PASSWORD or extracts it from REDIS_URL
func getRedisPassword() string {
	if pass := os.Getenv("REDIS_PASSWORD"); pass != "" {
		return pass
	}

	// Fallback to extracting from REDIS_URL
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		return ""
	}

	parsed, err := url.Parse(redisURL)
	if err != nil {
		return ""
	}

	if parsed.User != nil {
		pass, _ := parsed.User.Password()
		return pass
	}

	return ""
}

// Config holds all configuration for the bot
type Config struct {
	// Core configuration
	BotToken    string `validate:"required"`
	BotVersion  string
	ApiServer   string
	WorkingMode string
	Debug       bool

	// Bot settings
	OwnerId            int64 `validate:"required,min=1"`
	MessageDump        int64 `validate:"required,min=1"`
	DropPendingUpdates bool
	AllowedUpdates     []string
	ValidLangCodes     []string

	// Database configuration
	DatabaseURL string `validate:"required"`

	// Database connection pool configuration
	DBMaxIdleConns       int `validate:"min=1,max=100"`
	DBMaxOpenConns       int `validate:"min=1,max=1000"`
	DBConnMaxLifetimeMin int `validate:"min=1,max=1440"` // Max lifetime in minutes
	DBConnMaxIdleTimeMin int `validate:"min=1,max=60"`   // Max idle time in minutes

	// Redis configuration
	RedisAddress  string `validate:"required"`
	RedisPassword string
	RedisDB       int

	// HTTP Server configuration (unified server for health, metrics, webhook)
	HTTPPort int `validate:"min=1,max=65535"`

	// Webhook configuration
	UseWebhooks   bool
	WebhookDomain string
	WebhookSecret string
	WebhookPort   int `validate:"min=1,max=65535"` // Deprecated: use HTTPPort instead

	// Worker pool configuration for concurrent processing
	ChatValidationWorkers  int `validate:"min=1,max=100"`
	DatabaseWorkers        int `validate:"min=1,max=50"`
	MessagePipelineWorkers int `validate:"min=1,max=50"`
	BulkOperationWorkers   int `validate:"min=1,max=20"`
	CacheWorkers           int `validate:"min=1,max=20"`
	StatsCollectionWorkers int `validate:"min=1,max=10"`

	// Safety and performance limits
	MaxConcurrentOperations     int           `validate:"min=1,max=1000"`
	OperationTimeoutSeconds     int           `validate:"min=1,max=300"`
	OperationTimeout            time.Duration // Computed from OperationTimeoutSeconds
	EnablePerformanceMonitoring bool
	EnableBackgroundStats       bool
	DispatcherMaxRoutines       int `validate:"min=1,max=1000"` // Max concurrent goroutines for dispatcher

	// Cache configuration - Redis only (no local cache configuration needed)
	ClearCacheOnStartup bool // Whether to clear all caches on bot startup

	// Activity monitoring configuration
	InactivityThresholdDays int  `validate:"min=1,max=365"` // Days before marking a chat as inactive
	ActivityCheckInterval   int  `validate:"min=1,max=24"`  // Hours between activity checks
	EnableAutoCleanup       bool // Whether to automatically mark inactive chats

	// Performance optimization settings
	EnableQueryPrefetching bool // Enable query batching and prefetching

	EnableCachePrewarming       bool // Enable cache prewarming on startup
	EnableAsyncProcessing       bool // Enable async processing for non-critical operations
	EnableResponseCaching       bool // Enable response caching
	ResponseCacheTTL            int  `validate:"min=1,max=3600"` // Response cache TTL in seconds
	EnableBatchRequests         bool // Enable batch API requests
	BatchRequestTimeoutMS       int  `validate:"min=10,max=5000"` // Batch request timeout in milliseconds
	EnableHTTPConnectionPooling bool // Enable HTTP connection pooling
	HTTPMaxIdleConns            int  `validate:"min=10,max=1000"` // HTTP connection pool size
	HTTPMaxIdleConnsPerHost     int  `validate:"min=5,max=500"`   // HTTP connections per host

	// Database migration settings
	AutoMigrate           bool   // Enable automatic database migrations on startup
	AutoMigrateSilentFail bool   // Continue running even if migrations fail
	MigrationsPath        string // Path to migration files (defaults to migrations)

	// Resource monitoring limits
	ResourceMaxGoroutines int `validate:"min=100,max=10000"` // Maximum goroutines before triggering cleanup
	ResourceMaxMemoryMB   int `validate:"min=100,max=10000"` // Maximum memory usage in MB
	ResourceGCThresholdMB int `validate:"min=100,max=5000"`  // Memory threshold for triggering GC
}

// AppConfig is the global configuration instance - the single source of truth.
// All code should access configuration via config.AppConfig.FieldName
var AppConfig *Config

// ValidateConfig validates the configuration struct and returns an error if any required
// fields are missing or values are outside acceptable ranges.
func ValidateConfig(cfg *Config) error {
	if cfg.BotToken == "" {
		return fmt.Errorf("BOT_TOKEN is required")
	}
	if cfg.OwnerId == 0 {
		return fmt.Errorf("OWNER_ID is required and must be greater than 0")
	}
	if cfg.MessageDump == 0 {
		return fmt.Errorf("MESSAGE_DUMP is required and must be greater than 0")
	}
	if cfg.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.RedisAddress == "" {
		return fmt.Errorf("REDIS_ADDRESS or REDIS_URL is required")
	}

	// Validate webhook configuration if webhooks are enabled
	if cfg.UseWebhooks {
		if cfg.WebhookDomain == "" {
			return fmt.Errorf("WEBHOOK_DOMAIN is required when USE_WEBHOOKS is enabled")
		}
		if cfg.WebhookSecret == "" {
			return fmt.Errorf("WEBHOOK_SECRET is required when USE_WEBHOOKS is enabled for security")
		}
	}

	// Validate HTTP port
	if cfg.HTTPPort <= 0 || cfg.HTTPPort > 65535 {
		return fmt.Errorf("HTTP_PORT must be between 1 and 65535")
	}

	// Validate worker configurations
	if cfg.ChatValidationWorkers <= 0 || cfg.ChatValidationWorkers > 100 {
		return fmt.Errorf("CHAT_VALIDATION_WORKERS must be between 1 and 100")
	}
	if cfg.DatabaseWorkers <= 0 || cfg.DatabaseWorkers > 50 {
		return fmt.Errorf("DATABASE_WORKERS must be between 1 and 50")
	}
	if cfg.MessagePipelineWorkers <= 0 || cfg.MessagePipelineWorkers > 50 {
		return fmt.Errorf("MESSAGE_PIPELINE_WORKERS must be between 1 and 50")
	}
	if cfg.BulkOperationWorkers <= 0 || cfg.BulkOperationWorkers > 20 {
		return fmt.Errorf("BULK_OPERATION_WORKERS must be between 1 and 20")
	}
	if cfg.CacheWorkers <= 0 || cfg.CacheWorkers > 20 {
		return fmt.Errorf("CACHE_WORKERS must be between 1 and 20")
	}
	if cfg.StatsCollectionWorkers <= 0 || cfg.StatsCollectionWorkers > 10 {
		return fmt.Errorf("STATS_COLLECTION_WORKERS must be between 1 and 10")
	}

	// Cache validation removed - using Redis only

	// Validate performance limits
	if cfg.MaxConcurrentOperations <= 0 || cfg.MaxConcurrentOperations > 1000 {
		return fmt.Errorf("MAX_CONCURRENT_OPERATIONS must be between 1 and 1000")
	}
	if cfg.OperationTimeoutSeconds <= 0 || cfg.OperationTimeoutSeconds > 300 {
		return fmt.Errorf("OPERATION_TIMEOUT_SECONDS must be between 1 and 300")
	}
	if cfg.DispatcherMaxRoutines != 0 && (cfg.DispatcherMaxRoutines < 1 || cfg.DispatcherMaxRoutines > 1000) {
		return fmt.Errorf("DISPATCHER_MAX_ROUTINES must be between 1 and 1000")
	}

	// Validate database connection pool configuration
	if cfg.DBMaxIdleConns != 0 && (cfg.DBMaxIdleConns < 1 || cfg.DBMaxIdleConns > 100) {
		return fmt.Errorf("DB_MAX_IDLE_CONNS must be between 1 and 100")
	}
	if cfg.DBMaxOpenConns != 0 && (cfg.DBMaxOpenConns < 1 || cfg.DBMaxOpenConns > 1000) {
		return fmt.Errorf("DB_MAX_OPEN_CONNS must be between 1 and 1000")
	}
	if cfg.DBConnMaxLifetimeMin != 0 && (cfg.DBConnMaxLifetimeMin < 1 || cfg.DBConnMaxLifetimeMin > 1440) {
		return fmt.Errorf("DB_CONN_MAX_LIFETIME_MIN must be between 1 and 1440 minutes")
	}
	if cfg.DBConnMaxIdleTimeMin != 0 && (cfg.DBConnMaxIdleTimeMin < 1 || cfg.DBConnMaxIdleTimeMin > 60) {
		return fmt.Errorf("DB_CONN_MAX_IDLE_TIME_MIN must be between 1 and 60 minutes")
	}

	return nil
}

// LoadConfig loads configuration from environment variables, applies defaults,
// validates the configuration, and returns a populated Config instance.
func LoadConfig() (*Config, error) {
	// load goenv config
	_ = godotenv.Load() // Ignore error as .env file is optional

	cfg := &Config{
		// Core configuration
		BotToken:    os.Getenv("BOT_TOKEN"),
		BotVersion:  "2.1.3",
		ApiServer:   os.Getenv("API_SERVER"),
		WorkingMode: "worker",
		Debug:       typeConvertor{str: os.Getenv("DEBUG")}.Bool(),

		// Bot settings
		OwnerId:            typeConvertor{str: os.Getenv("OWNER_ID")}.Int64(),
		MessageDump:        typeConvertor{str: os.Getenv("MESSAGE_DUMP")}.Int64(),
		DropPendingUpdates: typeConvertor{str: os.Getenv("DROP_PENDING_UPDATES")}.Bool(),

		// Database configuration
		DatabaseURL: os.Getenv("DATABASE_URL"),

		// Database connection pool configuration
		DBMaxIdleConns:       typeConvertor{str: os.Getenv("DB_MAX_IDLE_CONNS")}.Int(),
		DBMaxOpenConns:       typeConvertor{str: os.Getenv("DB_MAX_OPEN_CONNS")}.Int(),
		DBConnMaxLifetimeMin: typeConvertor{str: os.Getenv("DB_CONN_MAX_LIFETIME_MIN")}.Int(),
		DBConnMaxIdleTimeMin: typeConvertor{str: os.Getenv("DB_CONN_MAX_IDLE_TIME_MIN")}.Int(),

		// Redis configuration (supports both REDIS_ADDRESS and REDIS_URL for Heroku compatibility)
		RedisAddress:  getRedisAddress(),
		RedisPassword: getRedisPassword(),
		RedisDB:       typeConvertor{str: os.Getenv("REDIS_DB")}.Int(),

		// HTTP Server configuration
		HTTPPort: typeConvertor{str: os.Getenv("HTTP_PORT")}.Int(),

		// Webhook configuration
		UseWebhooks:   typeConvertor{str: os.Getenv("USE_WEBHOOKS")}.Bool(),
		WebhookDomain: os.Getenv("WEBHOOK_DOMAIN"),
		WebhookSecret: os.Getenv("WEBHOOK_SECRET"),
		WebhookPort:   typeConvertor{str: os.Getenv("WEBHOOK_PORT")}.Int(),

		// Worker pool configuration
		ChatValidationWorkers:  typeConvertor{str: os.Getenv("CHAT_VALIDATION_WORKERS")}.Int(),
		DatabaseWorkers:        typeConvertor{str: os.Getenv("DATABASE_WORKERS")}.Int(),
		MessagePipelineWorkers: typeConvertor{str: os.Getenv("MESSAGE_PIPELINE_WORKERS")}.Int(),
		BulkOperationWorkers:   typeConvertor{str: os.Getenv("BULK_OPERATION_WORKERS")}.Int(),
		CacheWorkers:           typeConvertor{str: os.Getenv("CACHE_WORKERS")}.Int(),
		StatsCollectionWorkers: typeConvertor{str: os.Getenv("STATS_COLLECTION_WORKERS")}.Int(),

		// Safety and performance limits
		MaxConcurrentOperations:     typeConvertor{str: os.Getenv("MAX_CONCURRENT_OPERATIONS")}.Int(),
		OperationTimeoutSeconds:     typeConvertor{str: os.Getenv("OPERATION_TIMEOUT_SECONDS")}.Int(),
		EnablePerformanceMonitoring: typeConvertor{str: os.Getenv("ENABLE_PERFORMANCE_MONITORING")}.Bool(),
		EnableBackgroundStats:       typeConvertor{str: os.Getenv("ENABLE_BACKGROUND_STATS")}.Bool(),
		DispatcherMaxRoutines:       typeConvertor{str: os.Getenv("DISPATCHER_MAX_ROUTINES")}.Int(),

		// Cache configuration - Redis only
		ClearCacheOnStartup: typeConvertor{str: os.Getenv("CLEAR_CACHE_ON_STARTUP")}.Bool(),

		// Activity monitoring configuration
		InactivityThresholdDays: typeConvertor{str: os.Getenv("INACTIVITY_THRESHOLD_DAYS")}.Int(),
		ActivityCheckInterval:   typeConvertor{str: os.Getenv("ACTIVITY_CHECK_INTERVAL")}.Int(),
		EnableAutoCleanup:       typeConvertor{str: os.Getenv("ENABLE_AUTO_CLEANUP")}.Bool(),

		// Performance optimization settings
		EnableQueryPrefetching:      typeConvertor{str: os.Getenv("ENABLE_QUERY_PREFETCHING")}.Bool(),
		EnableCachePrewarming:       typeConvertor{str: os.Getenv("ENABLE_CACHE_PREWARMING")}.Bool(),
		EnableAsyncProcessing:       typeConvertor{str: os.Getenv("ENABLE_ASYNC_PROCESSING")}.Bool(),
		EnableResponseCaching:       typeConvertor{str: os.Getenv("ENABLE_RESPONSE_CACHING")}.Bool(),
		ResponseCacheTTL:            typeConvertor{str: os.Getenv("RESPONSE_CACHE_TTL")}.Int(),
		EnableBatchRequests:         typeConvertor{str: os.Getenv("ENABLE_BATCH_REQUESTS")}.Bool(),
		BatchRequestTimeoutMS:       typeConvertor{str: os.Getenv("BATCH_REQUEST_TIMEOUT_MS")}.Int(),
		EnableHTTPConnectionPooling: typeConvertor{str: os.Getenv("ENABLE_HTTP_CONNECTION_POOLING")}.Bool(),
		HTTPMaxIdleConns:            typeConvertor{str: os.Getenv("HTTP_MAX_IDLE_CONNS")}.Int(),
		HTTPMaxIdleConnsPerHost:     typeConvertor{str: os.Getenv("HTTP_MAX_IDLE_CONNS_PER_HOST")}.Int(),

		// Database migration settings
		AutoMigrate:           typeConvertor{str: os.Getenv("AUTO_MIGRATE")}.Bool(),
		AutoMigrateSilentFail: typeConvertor{str: os.Getenv("AUTO_MIGRATE_SILENT_FAIL")}.Bool(),
		MigrationsPath:        os.Getenv("MIGRATIONS_PATH"),

		// Resource monitoring limits
		ResourceMaxGoroutines: typeConvertor{str: os.Getenv("RESOURCE_MAX_GOROUTINES")}.Int(),
		ResourceMaxMemoryMB:   typeConvertor{str: os.Getenv("RESOURCE_MAX_MEMORY_MB")}.Int(),
		ResourceGCThresholdMB: typeConvertor{str: os.Getenv("RESOURCE_GC_THRESHOLD_MB")}.Int(),
	}

	// Set defaults
	cfg.setDefaults()

	// Validate configuration
	if err := ValidateConfig(cfg); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	// Set computed values
	cfg.OperationTimeout = time.Duration(cfg.OperationTimeoutSeconds) * time.Second

	// Set allowed updates and valid language codes
	cfg.AllowedUpdates = []string{
		"message",
		"edited_message",
		"channel_post",
		"edited_channel_post",
		"inline_query",
		"chosen_inline_result",
		"callback_query",
		"shipping_query",
		"pre_checkout_query",
		"poll",
		"poll_answer",
		"my_chat_member",
		"chat_member",
		"chat_join_request",
	}

	cfg.ValidLangCodes = typeConvertor{str: os.Getenv("ENABLED_LOCALES")}.StringArray()
	if (len(cfg.ValidLangCodes) == 1 && cfg.ValidLangCodes[0] == "") || (len(cfg.ValidLangCodes) == 0) {
		cfg.ValidLangCodes = []string{"en"}
	}

	return cfg, nil
}

// setDefaults sets default values for configuration fields that are not provided
// via environment variables. It calculates appropriate defaults based on system
// resources and production best practices.
func (cfg *Config) setDefaults() {
	if cfg.ApiServer == "" {
		cfg.ApiServer = "https://api.telegram.org"
	}
	if cfg.WorkingMode == "" {
		cfg.WorkingMode = "worker"
	}
	if cfg.RedisAddress == "" {
		cfg.RedisAddress = "localhost:6379"
	}
	if cfg.RedisDB == 0 {
		cfg.RedisDB = 1
	}

	// Handle HTTPPort with backward compatibility for WebhookPort
	if cfg.HTTPPort == 0 {
		if cfg.WebhookPort != 0 {
			// Use deprecated WebhookPort for backward compatibility
			cfg.HTTPPort = cfg.WebhookPort
			log.Warn("[Config] WEBHOOK_PORT is deprecated, please use HTTP_PORT instead")
		} else {
			// Default to 8080
			cfg.HTTPPort = 8080
		}
	}

	// Keep WebhookPort for backward compatibility (default to 8081 if not set)
	if cfg.WebhookPort == 0 {
		cfg.WebhookPort = 8081 // Legacy default
	}

	// Set default values for worker pool configurations
	cpuCount := runtime.NumCPU()

	if cfg.ChatValidationWorkers == 0 {
		cfg.ChatValidationWorkers = 10
	}
	if cfg.DatabaseWorkers == 0 {
		cfg.DatabaseWorkers = 5
	}
	if cfg.MessagePipelineWorkers == 0 {
		cfg.MessagePipelineWorkers = cpuCount
		if cfg.MessagePipelineWorkers > 8 {
			cfg.MessagePipelineWorkers = 8
		}
	}
	if cfg.BulkOperationWorkers == 0 {
		cfg.BulkOperationWorkers = 4
	}
	if cfg.CacheWorkers == 0 {
		cfg.CacheWorkers = 3
	}
	if cfg.StatsCollectionWorkers == 0 {
		cfg.StatsCollectionWorkers = 2
	}

	// Set cache defaults (optimized values for better performance)
	// Cache defaults removed - using Redis only

	// Set activity monitoring defaults
	if cfg.InactivityThresholdDays == 0 {
		cfg.InactivityThresholdDays = 30 // 30 days before marking as inactive
	}
	if cfg.ActivityCheckInterval == 0 {
		cfg.ActivityCheckInterval = 1 // Check every hour
	}
	// EnableAutoCleanup defaults to true unless explicitly set to false

	// Set database connection pool defaults (optimized for performance)
	if cfg.DBMaxIdleConns == 0 {
		cfg.DBMaxIdleConns = 50 // Keep more connections warm
	}
	if cfg.DBMaxOpenConns == 0 {
		cfg.DBMaxOpenConns = 200 // Handle burst traffic better
	}
	if cfg.DBConnMaxLifetimeMin == 0 {
		cfg.DBConnMaxLifetimeMin = 240 // 4 hours - reuse connections longer
	}
	if cfg.DBConnMaxIdleTimeMin == 0 {
		cfg.DBConnMaxIdleTimeMin = 60 // 1 hour - keep idle connections longer
	}

	// Set default safety limits
	if cfg.MaxConcurrentOperations == 0 {
		cfg.MaxConcurrentOperations = 50
	}
	if cfg.OperationTimeoutSeconds == 0 {
		cfg.OperationTimeoutSeconds = 30
	}
	if cfg.DispatcherMaxRoutines == 0 {
		cfg.DispatcherMaxRoutines = 200 // Optimized for better throughput
	}

	// Set cache defaults
	// ClearCacheOnStartup defaults to true for better reliability
	cfg.ClearCacheOnStartup = true

	// Enable monitoring by default in production
	if !cfg.Debug {
		if !cfg.EnablePerformanceMonitoring {
			cfg.EnablePerformanceMonitoring = true
		}
		if !cfg.EnableBackgroundStats {
			cfg.EnableBackgroundStats = true
		}
	}

	// DATABASE_URL is required - no default for security
	// This ensures production systems explicitly configure their database connection

	// Set performance optimization defaults (enabled by default for better performance)
	if !cfg.EnableQueryPrefetching {
		cfg.EnableQueryPrefetching = true
	}
	if !cfg.EnableCachePrewarming {
		cfg.EnableCachePrewarming = true
	}
	if !cfg.EnableAsyncProcessing {
		cfg.EnableAsyncProcessing = true
	}
	if !cfg.EnableResponseCaching {
		cfg.EnableResponseCaching = true
	}
	if cfg.ResponseCacheTTL == 0 {
		cfg.ResponseCacheTTL = 30 // 30 seconds
	}
	if !cfg.EnableBatchRequests {
		cfg.EnableBatchRequests = true
	}
	if cfg.BatchRequestTimeoutMS == 0 {
		cfg.BatchRequestTimeoutMS = 100 // 100ms
	}
	if !cfg.EnableHTTPConnectionPooling {
		cfg.EnableHTTPConnectionPooling = true
	}
	if cfg.HTTPMaxIdleConns == 0 {
		cfg.HTTPMaxIdleConns = 100
	}
	if cfg.HTTPMaxIdleConnsPerHost == 0 {
		cfg.HTTPMaxIdleConnsPerHost = 50
	}

	// Set database migration defaults
	if cfg.MigrationsPath == "" {
		cfg.MigrationsPath = "migrations"
	}
	// AutoMigrate defaults to false for backward compatibility
	// AutoMigrateSilentFail defaults to false

	// Set resource monitoring defaults
	if cfg.ResourceMaxGoroutines == 0 {
		cfg.ResourceMaxGoroutines = 1000
	}
	if cfg.ResourceMaxMemoryMB == 0 {
		cfg.ResourceMaxMemoryMB = 500
	}
	if cfg.ResourceGCThresholdMB == 0 {
		cfg.ResourceGCThresholdMB = 400
	}
}

// init initializes the logging configuration, loads the global configuration
// from environment variables, validates it, and sets up global variables for
// backward compatibility. This function is called automatically at package import.
func init() {
	// set logger config
	log.SetLevel(log.DebugLevel)
	// SetReportCaller will be configured after debug mode is determined
	log.SetFormatter(
		&log.JSONFormatter{
			DisableHTMLEscape: true,
			PrettyPrint:       false,
			CallerPrettyfier: func(f *runtime.Frame) (string, string) {
				return f.Function, fmt.Sprintf("%s:%d", path.Base(f.File), f.Line)
			},
		},
	)

	// Load the structured configuration
	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("[Config] Failed to load configuration: %v", err)
	}

	// Set global configuration instance
	AppConfig = cfg

	// Configure logger based on debug mode
	if cfg.Debug {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}
	log.SetReportCaller(cfg.Debug) // Only enable stack traces in debug mode

	log.Info("[Config] Configuration loaded and validated successfully")
}
