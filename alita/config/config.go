package config

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"runtime"
	"strconv"

	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/utils/logredact"
)

func isCliModeActive() bool {
	if len(os.Args) < 2 {
		return false
	}

	for _, arg := range os.Args[1:] {
		switch arg {
		case "--version", "-version", "-v", "--health", "-health":
			return true
		}
	}
	return false
}

// REDIS_URL format: redis://user:password@host:port (standard Redis URL format)
func getRedisAddress() string {
	if addr := os.Getenv("REDIS_ADDRESS"); addr != "" {
		return addr
	}

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

func getRedisURL() string {
	if os.Getenv("REDIS_ADDRESS") != "" {
		return ""
	}
	return os.Getenv("REDIS_URL")
}

func getRedisPassword() string {
	if pass := os.Getenv("REDIS_PASSWORD"); pass != "" {
		return pass
	}
	if os.Getenv("REDIS_ADDRESS") != "" {
		return ""
	}

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

func parseRedisDB() (int, error) {
	raw := os.Getenv("REDIS_DB")
	if raw == "" {
		return 1, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("REDIS_DB must be an integer 0-15")
	}
	if v < 0 || v > 15 {
		return 0, fmt.Errorf("REDIS_DB must be an integer 0-15")
	}
	return v, nil
}

func getHTTPPort() int {
	value := os.Getenv("HTTP_PORT")
	if value == "" {
		value = os.Getenv("PORT")
	}
	return typeConvertor{str: value}.Int()
}

type Config struct {
	BotToken    string `validate:"required"`
	BotVersion  string
	ApiServer   string
	WorkingMode string
	Debug       bool

	OwnerId            int64 `validate:"required,min=1"`
	MessageDump        int64 `validate:"required,min=1"`
	DropPendingUpdates bool
	AllowedUpdates     []string
	ValidLangCodes     []string

	DatabaseURL string `validate:"required"`

	DBMaxIdleConns       int `validate:"min=1,max=100"`
	DBMaxOpenConns       int `validate:"min=1,max=1000"`
	DBConnMaxLifetimeMin int `validate:"min=1,max=1440"`
	DBConnMaxIdleTimeMin int `validate:"min=1,max=60"`

	EnableDBMonitoring bool `env:"ENABLE_DB_MONITORING" envDefault:"false"`

	RedisAddress  string `validate:"required"`
	RedisURL      string
	RedisPassword string
	RedisDB       int

	HTTPPort int `validate:"min=1,max=65535"`

	UseWebhooks   bool
	WebhookDomain string
	WebhookSecret string

	EnablePerformanceMonitoring bool
	EnableBackgroundStats       bool
	DispatcherMaxRoutines       int `validate:"min=1,max=1000"`
	ClearCacheOnStartup         bool
	DisableCache                bool
	InactivityThresholdDays     int `validate:"min=1,max=365"`
	ActivityCheckInterval       int `validate:"min=1,max=24"`
	EnableAutoCleanup           bool

	HTTPMaxIdleConns        int `validate:"min=10,max=1000"`
	HTTPMaxIdleConnsPerHost int `validate:"min=5,max=500"`

	AutoMigrate           bool
	AutoMigrateSilentFail bool
	MigrationsPath        string

	ResourceMaxGoroutines int `validate:"min=100,max=10000"`
	ResourceMaxMemoryMB   int `validate:"min=100,max=10000"`
	ResourceGCThresholdMB int `validate:"min=100,max=5000"`

	EnablePPROF bool

	MetricsAuthToken string
}

var AppConfig *Config

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
	if !cfg.DisableCache && cfg.RedisAddress == "" {
		return fmt.Errorf("REDIS_ADDRESS or REDIS_URL is required")
	}

	if cfg.UseWebhooks {
		if cfg.WebhookDomain == "" {
			return fmt.Errorf("WEBHOOK_DOMAIN is required when USE_WEBHOOKS is enabled")
		}
		if cfg.WebhookSecret == "" {
			return fmt.Errorf("WEBHOOK_SECRET is required when USE_WEBHOOKS is enabled for security")
		}
	}

	if cfg.HTTPPort <= 0 || cfg.HTTPPort > 65535 {
		return fmt.Errorf("HTTP_PORT must be between 1 and 65535")
	}

	if cfg.DispatcherMaxRoutines != 0 && (cfg.DispatcherMaxRoutines < 1 || cfg.DispatcherMaxRoutines > 1000) {
		return fmt.Errorf("DISPATCHER_MAX_ROUTINES must be between 1 and 1000")
	}

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

	if cfg.RedisDB < 0 || cfg.RedisDB > 15 {
		return fmt.Errorf("REDIS_DB must be an integer 0-15")
	}

	return nil
}

func LoadConfig() (*Config, error) {
	_ = godotenv.Load()

	redisDB, err := parseRedisDB()
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		BotToken:    os.Getenv("BOT_TOKEN"),
		BotVersion:  "2.22.6",
		ApiServer:   os.Getenv("API_SERVER"),
		WorkingMode: "worker",
		Debug:       typeConvertor{str: os.Getenv("DEBUG")}.Bool(),

		OwnerId:            typeConvertor{str: os.Getenv("OWNER_ID")}.Int64(),
		MessageDump:        typeConvertor{str: os.Getenv("MESSAGE_DUMP")}.Int64(),
		DropPendingUpdates: typeConvertor{str: os.Getenv("DROP_PENDING_UPDATES")}.Bool(),

		DatabaseURL: os.Getenv("DATABASE_URL"),

		DBMaxIdleConns:       typeConvertor{str: os.Getenv("DB_MAX_IDLE_CONNS")}.Int(),
		DBMaxOpenConns:       typeConvertor{str: os.Getenv("DB_MAX_OPEN_CONNS")}.Int(),
		DBConnMaxLifetimeMin: typeConvertor{str: os.Getenv("DB_CONN_MAX_LIFETIME_MIN")}.Int(),
		DBConnMaxIdleTimeMin: typeConvertor{str: os.Getenv("DB_CONN_MAX_IDLE_TIME_MIN")}.Int(),

		EnableDBMonitoring: typeConvertor{str: os.Getenv("ENABLE_DB_MONITORING")}.Bool(),

		RedisAddress:  getRedisAddress(),
		RedisURL:      getRedisURL(),
		RedisPassword: getRedisPassword(),
		RedisDB:       redisDB,

		HTTPPort: getHTTPPort(),

		UseWebhooks:   typeConvertor{str: os.Getenv("USE_WEBHOOKS")}.Bool(),
		WebhookDomain: os.Getenv("WEBHOOK_DOMAIN"),
		WebhookSecret: os.Getenv("WEBHOOK_SECRET"),

		EnablePerformanceMonitoring: typeConvertor{str: os.Getenv("ENABLE_PERFORMANCE_MONITORING")}.Bool(),
		EnableBackgroundStats:       typeConvertor{str: os.Getenv("ENABLE_BACKGROUND_STATS")}.Bool(),
		DispatcherMaxRoutines:       typeConvertor{str: os.Getenv("DISPATCHER_MAX_ROUTINES")}.Int(),

		ClearCacheOnStartup: typeConvertor{str: os.Getenv("CLEAR_CACHE_ON_STARTUP")}.Bool(),
		DisableCache:        typeConvertor{str: os.Getenv("DISABLE_CACHE")}.Bool(),

		InactivityThresholdDays: typeConvertor{str: os.Getenv("INACTIVITY_THRESHOLD_DAYS")}.Int(),
		ActivityCheckInterval:   typeConvertor{str: os.Getenv("ACTIVITY_CHECK_INTERVAL")}.Int(),
		EnableAutoCleanup:       typeConvertor{str: os.Getenv("ENABLE_AUTO_CLEANUP")}.Bool(),

		HTTPMaxIdleConns:        typeConvertor{str: os.Getenv("HTTP_MAX_IDLE_CONNS")}.Int(),
		HTTPMaxIdleConnsPerHost: typeConvertor{str: os.Getenv("HTTP_MAX_IDLE_CONNS_PER_HOST")}.Int(),

		AutoMigrate:           typeConvertor{str: os.Getenv("AUTO_MIGRATE")}.Bool(),
		AutoMigrateSilentFail: typeConvertor{str: os.Getenv("AUTO_MIGRATE_SILENT_FAIL")}.Bool(),
		MigrationsPath:        os.Getenv("MIGRATIONS_PATH"),

		ResourceMaxGoroutines: typeConvertor{str: os.Getenv("RESOURCE_MAX_GOROUTINES")}.Int(),
		ResourceMaxMemoryMB:   typeConvertor{str: os.Getenv("RESOURCE_MAX_MEMORY_MB")}.Int(),
		ResourceGCThresholdMB: typeConvertor{str: os.Getenv("RESOURCE_GC_THRESHOLD_MB")}.Int(),

		EnablePPROF: typeConvertor{str: os.Getenv("ENABLE_PPROF")}.Bool(),

		MetricsAuthToken: os.Getenv("METRICS_AUTH_TOKEN"),
	}

	cfg.setDefaults()

	if err := ValidateConfig(cfg); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

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
	if cfg.RedisDB == 0 && os.Getenv("REDIS_DB") != "0" {
		cfg.RedisDB = 1
	}

	if cfg.HTTPPort == 0 {
		cfg.HTTPPort = 8080
	}

	if cfg.InactivityThresholdDays == 0 {
		cfg.InactivityThresholdDays = 30
	}
	if cfg.ActivityCheckInterval == 0 {
		cfg.ActivityCheckInterval = 1
	}
	if os.Getenv("ENABLE_AUTO_CLEANUP") == "" {
		cfg.EnableAutoCleanup = true
	}

	if cfg.DBMaxIdleConns == 0 {
		cfg.DBMaxIdleConns = 50
	}
	if cfg.DBMaxOpenConns == 0 {
		cfg.DBMaxOpenConns = 200
	}
	if cfg.DBConnMaxLifetimeMin == 0 {
		cfg.DBConnMaxLifetimeMin = 240
	}
	if cfg.DBConnMaxIdleTimeMin == 0 {
		cfg.DBConnMaxIdleTimeMin = 60
	}

	if cfg.DispatcherMaxRoutines == 0 {
		cfg.DispatcherMaxRoutines = 200
	}

	if os.Getenv("CLEAR_CACHE_ON_STARTUP") == "" {
		cfg.ClearCacheOnStartup = true
	}

	if !cfg.Debug {
		if os.Getenv("ENABLE_PERFORMANCE_MONITORING") == "" {
			cfg.EnablePerformanceMonitoring = true
		}
		if os.Getenv("ENABLE_BACKGROUND_STATS") == "" {
			cfg.EnableBackgroundStats = true
		}
	}

	if cfg.HTTPMaxIdleConns == 0 {
		cfg.HTTPMaxIdleConns = 100
	}
	if cfg.HTTPMaxIdleConnsPerHost == 0 {
		cfg.HTTPMaxIdleConnsPerHost = 50
	}

	if cfg.MigrationsPath == "" {
		cfg.MigrationsPath = "migrations"
	}

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

func init() {
	if isCliModeActive() {
		AppConfig = &Config{}
		return
	}

	log.SetLevel(log.DebugLevel)
	log.SetFormatter(
		&log.JSONFormatter{
			DisableHTMLEscape: true,
			PrettyPrint:       false,
			CallerPrettyfier: func(f *runtime.Frame) (string, string) {
				return f.Function, fmt.Sprintf("%s:%d", path.Base(f.File), f.Line)
			},
		},
	)

	logredact.Install(nil)

	cfg, err := LoadConfig()
	if err != nil {
		if os.Getenv("BOT_TOKEN") == "" {
			AppConfig = &Config{}
			return
		}
		log.Fatalf("[Config] Failed to load configuration: %v", err)
	}

	AppConfig = cfg

	logredact.RegisterSecret(
		cfg.BotToken,
		cfg.DatabaseURL,
		cfg.RedisPassword,
		cfg.WebhookSecret,
		cfg.MetricsAuthToken,
	)

	if cfg.Debug {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}
	log.SetReportCaller(cfg.Debug)

	log.Info("[Config] Configuration loaded and validated successfully")
}
