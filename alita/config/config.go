package config

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"time"

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

// DefaultPanelSessionMaxAge is how long a Mini App initData payload is accepted
// after its auth_date when PANEL_SESSION_MAX_AGE is unset.
const DefaultPanelSessionMaxAge = 24 * time.Hour

// getPanelSessionMaxAge reads PANEL_SESSION_MAX_AGE as a Go duration. Unlike
// the lenient integer settings, a typo fails startup: a panel that silently
// accepted stale initData would be a security regression.
func getPanelSessionMaxAge() (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv("PANEL_SESSION_MAX_AGE"))
	if raw == "" {
		return DefaultPanelSessionMaxAge, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("PANEL_SESSION_MAX_AGE must be a positive Go duration such as 24h")
	}
	return d, nil
}

// DefaultDigestInterval is how often the weekly digest scheduler looks for
// chats due a digest when DIGEST_INTERVAL is unset.
const DefaultDigestInterval = time.Hour

// getDigestInterval reads DIGEST_INTERVAL as a Go duration. Like
// PANEL_SESSION_MAX_AGE, a typo fails startup instead of silently running on
// a cadence nobody asked for.
func getDigestInterval() (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv("DIGEST_INTERVAL"))
	if raw == "" {
		return DefaultDigestInterval, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("DIGEST_INTERVAL must be a positive Go duration such as 1h")
	}
	return d, nil
}

func getHTTPPort() int {
	value := os.Getenv("HTTP_PORT")
	if value == "" {
		value = os.Getenv("PORT")
	}
	return typeConvertor{str: value}.Int()
}

// Shared reputation network defaults. The window is capped at the 180-day
// moderation-event retention: a longer window could never be observed in full
// because older rows are deleted, so it would only look like a wider one.
const (
	DefaultReputationMinChats   = 3
	DefaultReputationWindowDays = 30
	MaxReputationWindowDays     = 180
)

// getReputationInt reads one shared-reputation integer setting. Unlike the
// lenient typeConvertor integers, a typo fails startup: a silently lowered
// threshold would list users on evidence the operator never chose, and a
// silently widened window would keep an old mistake alive.
func getReputationInt(name string, fallback int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", name)
	}
	return v, nil
}

// Local read-through cache defaults (CACHE_LOCAL_TTL, CACHE_LOCAL_MAX_ENTRIES).
const (
	DefaultCacheLocalTTLSeconds = 10
	DefaultCacheLocalMaxEntries = 50000
)

// getCacheLocalTTL reads CACHE_LOCAL_TTL. Unlike other integers, 0 is a real
// setting (layer off), so only an unset or unparsable value falls back to the
// default; a typo must not silently disable the layer.
func getCacheLocalTTL() int {
	raw := os.Getenv("CACHE_LOCAL_TTL")
	if raw == "" {
		return DefaultCacheLocalTTLSeconds
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		log.WithError(err).WithField("value", raw).Warn("Failed to convert CACHE_LOCAL_TTL to int, using default")
		return DefaultCacheLocalTTLSeconds
	}
	return v
}

type Config struct {
	BotToken    string `validate:"required"`
	BotVersion  string
	ApiServer   string
	WorkingMode string
	Debug       bool

	OwnerId            int64 `validate:"required,min=1"`
	MessageDump        int64 `validate:"required"`
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
	// RedisTLSInsecureSkipVerify skips certificate verification for a
	// rediss:// REDIS_URL. Heroku Redis needs it: its certificate is self-signed.
	RedisTLSInsecureSkipVerify bool

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

	// CacheLocalTTLSeconds bounds how long a replica serves a read-through
	// value from process memory before asking Redis again; 0 turns the local
	// layer off.
	CacheLocalTTLSeconds int `validate:"min=0,max=300"`
	CacheLocalMaxEntries int `validate:"min=100,max=1000000"`

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

	// AI spam filter (TypeSafe Jev). EnableAISpam is a global kill switch; the
	// feature is inert without TypeSafeAPIKey.
	EnableAISpam   bool
	TypeSafeAPIKey string

	// Join-time CAS (cas.chat) and lols.bot spam database check. EnableSpamDB
	// is the operator's kill switch; each chat still opts in with /spamdb on.
	// The CAS API key is optional until CAS starts requiring it; lols.bot
	// takes none.
	EnableSpamDB bool
	CASAPIKey    string

	// Shared reputation network: join-time lookups over moderation_events,
	// counting distinct chats that removed a user inside a rolling window.
	// EnableReputation is the operator's kill switch and is off by default;
	// each chat still opts in through its spam database settings.
	EnableReputation     bool
	ReputationMinChats   int
	ReputationWindowDays int

	// Weekly digest of the daily activity counters and the /digest command.
	// EnableDigest is the operator's kill switch; it is on unless explicitly
	// disabled, and the feature stays inert until a chat opts in.
	// DigestInterval overrides the scheduler tick (DIGEST_INTERVAL), which
	// exists for tests and unusual deployments; it defaults to an hour.
	EnableDigest   bool
	DigestInterval time.Duration

	// Admin panel (Telegram Mini App). Disabled by default; when enabled the
	// bot's own HTTP server serves it under /panel.
	PanelEnabled          bool
	PanelPublicURL        string
	PanelMiniAppShortName string
	PanelSessionMaxAge    time.Duration
}

var AppConfig *Config

func ValidateConfig(cfg *Config) error {
	if cfg.BotToken == "" {
		return fmt.Errorf("BOT_TOKEN is required")
	}
	if cfg.OwnerId == 0 {
		return fmt.Errorf("OWNER_ID is required")
	}
	if cfg.MessageDump == 0 {
		return fmt.Errorf("MESSAGE_DUMP is required (the log channel ID, e.g. -100xxxxxxxxxx)")
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

	if cfg.CacheLocalTTLSeconds < 0 || cfg.CacheLocalTTLSeconds > 300 {
		return fmt.Errorf("CACHE_LOCAL_TTL must be between 0 and 300 seconds")
	}
	if cfg.CacheLocalMaxEntries != 0 && (cfg.CacheLocalMaxEntries < 100 || cfg.CacheLocalMaxEntries > 1000000) {
		return fmt.Errorf("CACHE_LOCAL_MAX_ENTRIES must be between 100 and 1000000")
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

	if cfg.ReputationMinChats < 2 {
		return fmt.Errorf("REPUTATION_MIN_CHATS must be an integer of at least 2")
	}
	if cfg.ReputationWindowDays < 1 || cfg.ReputationWindowDays > MaxReputationWindowDays {
		return fmt.Errorf("REPUTATION_WINDOW_DAYS must be an integer between 1 and %d", MaxReputationWindowDays)
	}

	if cfg.PanelSessionMaxAge < 0 {
		return fmt.Errorf("PANEL_SESSION_MAX_AGE must be a positive duration")
	}
	if cfg.DigestInterval < 0 {
		return fmt.Errorf("DIGEST_INTERVAL must be a positive duration")
	}
	if cfg.PanelEnabled {
		if cfg.PanelPublicURL == "" {
			return fmt.Errorf("PANEL_PUBLIC_URL is required when PANEL_ENABLED is true")
		}
		publicURL, err := url.Parse(cfg.PanelPublicURL)
		if err != nil || publicURL.Scheme == "" || publicURL.Host == "" {
			return fmt.Errorf("PANEL_PUBLIC_URL must be an absolute URL such as https://panel.example.com")
		}
		if publicURL.Scheme != "https" && (!cfg.Debug || publicURL.Scheme != "http") {
			return fmt.Errorf("PANEL_PUBLIC_URL must use https:// (http:// is allowed only with DEBUG=true)")
		}
	}

	return nil
}

func LoadConfig() (*Config, error) {
	_ = godotenv.Load()

	redisDB, err := parseRedisDB()
	if err != nil {
		return nil, err
	}

	panelSessionMaxAge, err := getPanelSessionMaxAge()
	if err != nil {
		return nil, err
	}

	reputationMinChats, err := getReputationInt("REPUTATION_MIN_CHATS", DefaultReputationMinChats)
	if err != nil {
		return nil, err
	}
	reputationWindowDays, err := getReputationInt("REPUTATION_WINDOW_DAYS", DefaultReputationWindowDays)
	if err != nil {
		return nil, err
	}
	digestInterval, err := getDigestInterval()
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		BotToken:    os.Getenv("BOT_TOKEN"),
		BotVersion:  "2.26.5",
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

		RedisTLSInsecureSkipVerify: typeConvertor{str: os.Getenv("REDIS_TLS_INSECURE_SKIP_VERIFY")}.Bool(),

		HTTPPort: getHTTPPort(),

		UseWebhooks:   typeConvertor{str: os.Getenv("USE_WEBHOOKS")}.Bool(),
		WebhookDomain: os.Getenv("WEBHOOK_DOMAIN"),
		WebhookSecret: os.Getenv("WEBHOOK_SECRET"),

		EnablePerformanceMonitoring: typeConvertor{str: os.Getenv("ENABLE_PERFORMANCE_MONITORING")}.Bool(),
		EnableBackgroundStats:       typeConvertor{str: os.Getenv("ENABLE_BACKGROUND_STATS")}.Bool(),
		DispatcherMaxRoutines:       typeConvertor{str: os.Getenv("DISPATCHER_MAX_ROUTINES")}.Int(),

		ClearCacheOnStartup: typeConvertor{str: os.Getenv("CLEAR_CACHE_ON_STARTUP")}.Bool(),
		DisableCache:        typeConvertor{str: os.Getenv("DISABLE_CACHE")}.Bool(),

		CacheLocalTTLSeconds: getCacheLocalTTL(),
		CacheLocalMaxEntries: typeConvertor{str: os.Getenv("CACHE_LOCAL_MAX_ENTRIES")}.Int(),

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

		EnableAISpam:   typeConvertor{str: os.Getenv("ENABLE_AISPAM")}.Bool(),
		TypeSafeAPIKey: os.Getenv("TYPESAFE_API_KEY"),

		EnableSpamDB: typeConvertor{str: os.Getenv("ENABLE_SPAMDB")}.Bool(),
		CASAPIKey:    os.Getenv("CAS_API_KEY"),

		EnableReputation:     typeConvertor{str: os.Getenv("ENABLE_REPUTATION")}.Bool(),
		ReputationMinChats:   reputationMinChats,
		ReputationWindowDays: reputationWindowDays,

		EnableDigest:   typeConvertor{str: os.Getenv("ENABLE_DIGEST")}.Bool(),
		DigestInterval: digestInterval,

		PanelEnabled:          typeConvertor{str: os.Getenv("PANEL_ENABLED")}.Bool(),
		PanelPublicURL:        strings.TrimSpace(os.Getenv("PANEL_PUBLIC_URL")),
		PanelMiniAppShortName: strings.TrimPrefix(strings.TrimSpace(os.Getenv("PANEL_MINIAPP_SHORTNAME")), "@"),
		PanelSessionMaxAge:    panelSessionMaxAge,
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

	if cfg.CacheLocalMaxEntries == 0 {
		cfg.CacheLocalMaxEntries = DefaultCacheLocalMaxEntries
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

	// Global kill switch: on unless explicitly disabled. The feature stays
	// inert without TYPESAFE_API_KEY, so an unset flag is not a decision to
	// delete anything.
	if os.Getenv("ENABLE_AISPAM") == "" {
		cfg.EnableAISpam = true
	}

	// On unless explicitly disabled: every chat starts opted out, so an unset
	// flag sends nothing to CAS.
	if os.Getenv("ENABLE_SPAMDB") == "" {
		cfg.EnableSpamDB = true
	}

	// On unless explicitly disabled. The counters themselves always record;
	// this switch only stops the digest and refuses /digest.
	if os.Getenv("ENABLE_DIGEST") == "" {
		cfg.EnableDigest = true
	}

	if cfg.DigestInterval == 0 {
		cfg.DigestInterval = DefaultDigestInterval
	}

	if cfg.PanelSessionMaxAge == 0 {
		cfg.PanelSessionMaxAge = DefaultPanelSessionMaxAge
	}
	cfg.PanelPublicURL = strings.TrimRight(cfg.PanelPublicURL, "/")
}

// registerConfigSecrets keeps every credential in cfg out of the logs.
func registerConfigSecrets(cfg *Config) {
	logredact.RegisterSecret(
		cfg.BotToken,
		cfg.DatabaseURL,
		cfg.RedisPassword,
		cfg.WebhookSecret,
		cfg.MetricsAuthToken,
		cfg.TypeSafeAPIKey,
		cfg.CASAPIKey,
	)
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
			// Keep the local cache layer at its production default so test
			// binaries exercise the same read path as a deployed bot.
			AppConfig = &Config{
				CacheLocalTTLSeconds: DefaultCacheLocalTTLSeconds,
				CacheLocalMaxEntries: DefaultCacheLocalMaxEntries,
			}
			return
		}
		log.Fatalf("[Config] Failed to load configuration: %v", err)
	}

	AppConfig = cfg

	registerConfigSecrets(cfg)

	if cfg.Debug {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}
	log.SetReportCaller(cfg.Debug)

	log.Info("[Config] Configuration loaded and validated successfully")
}
