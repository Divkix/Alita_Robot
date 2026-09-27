package config

import (
	"fmt"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/utils/constants"
	"github.com/divkix/Alita_Robot/alita/utils/logredact"
)

// Commit identifies the build. Local builds report "dev"; release builds inject
// the short commit SHA with
// -ldflags "-X github.com/divkix/Alita_Robot/alita/config.Commit=<short sha>".
var Commit = "dev"

// isCliModeActive returns true if the program is running with CLI flags
// that should skip database initialization (--version, --health, -v).
// This allows init() functions to return early without requiring DB connection.
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

func getHTTPPort() int {
	value := os.Getenv("HTTP_PORT")
	if value == "" {
		value = os.Getenv("PORT")
	}
	return typeConvertor{str: value}.Int()
}

// Config holds the runtime configuration of the single bot instance.
// BOT_TOKEN and OWNER_ID are required; every other field is optional and
// falls back to a default.
type Config struct {
	// Required
	BotToken string
	OwnerId  int64

	// Optional
	SQLitePath  string
	HTTPPort    int
	LogLevel    log.Level
	MessageDump int64 // 0 disables dump-chat notices
	UseWebhooks bool

	// Required only when UseWebhooks is enabled
	WebhookDomain string
	WebhookSecret string

	// Optional integrations
	TypeSafeAPIKey string
	OwnerChatIDs   []int64

	// Derived
	AllowedUpdates []string
}

// AppConfig is the global configuration instance - the single source of truth.
// All code should access configuration via config.AppConfig.FieldName
var AppConfig *Config

// ValidateConfig validates the configuration struct and returns an error if any
// required fields are missing or values are outside acceptable ranges.
func ValidateConfig(cfg *Config) error {
	if cfg.BotToken == "" {
		return fmt.Errorf("BOT_TOKEN is required")
	}
	if cfg.OwnerId <= 0 {
		return fmt.Errorf("OWNER_ID is required and must be greater than 0")
	}
	if cfg.SQLitePath == "" {
		return fmt.Errorf("SQLITE_PATH must not be empty")
	}
	if cfg.HTTPPort <= 0 || cfg.HTTPPort > 65535 {
		return fmt.Errorf("HTTP_PORT must be between 1 and 65535")
	}
	for _, id := range cfg.OwnerChatIDs {
		if id >= 0 {
			return fmt.Errorf("OWNER_CHAT_IDS must contain negative group chat IDs")
		}
	}

	// Webhook credentials only matter when webhook delivery is enabled.
	if cfg.UseWebhooks {
		if cfg.WebhookDomain == "" {
			return fmt.Errorf("WEBHOOK_DOMAIN is required when USE_WEBHOOKS is enabled")
		}
		if cfg.WebhookSecret == "" {
			return fmt.Errorf("WEBHOOK_SECRET is required when USE_WEBHOOKS is enabled for security")
		}
	}

	return nil
}

// parseLogLevel resolves LOG_LEVEL to a logrus level, defaulting to info when
// unset. An unrecognized value is an error so a typo cannot silently mute logs.
func parseLogLevel(value string) (log.Level, error) {
	if value == "" {
		return log.InfoLevel, nil
	}
	level, err := log.ParseLevel(value)
	if err != nil {
		return log.InfoLevel, fmt.Errorf("LOG_LEVEL %q is not a valid level: %w", value, err)
	}
	return level, nil
}

// LoadConfig loads configuration from environment variables, applies defaults,
// validates the configuration, and returns a populated Config instance.
func LoadConfig() (*Config, error) {
	// load goenv config
	_ = godotenv.Load() // Ignore error as .env file is optional

	logLevel, err := parseLogLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		return nil, err
	}
	var ownerChatIDs []int64
	if value := os.Getenv("OWNER_CHAT_IDS"); value != "" {
		for _, part := range strings.Split(value, ",") {
			id, parseErr := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
			if parseErr != nil || id >= 0 {
				return nil, fmt.Errorf("OWNER_CHAT_IDS must contain comma-separated negative group chat IDs")
			}
			ownerChatIDs = append(ownerChatIDs, id)
		}
	}

	cfg := &Config{
		BotToken: os.Getenv("BOT_TOKEN"),
		OwnerId:  typeConvertor{str: os.Getenv("OWNER_ID")}.Int64(),

		SQLitePath:  os.Getenv("SQLITE_PATH"),
		HTTPPort:    getHTTPPort(),
		LogLevel:    logLevel,
		MessageDump: typeConvertor{str: os.Getenv("MESSAGE_DUMP")}.Int64(),
		UseWebhooks: typeConvertor{str: os.Getenv("USE_WEBHOOKS")}.Bool(),

		WebhookDomain: os.Getenv("WEBHOOK_DOMAIN"),
		WebhookSecret: os.Getenv("WEBHOOK_SECRET"),

		TypeSafeAPIKey: os.Getenv("TYPESAFE_API_KEY"),
		OwnerChatIDs:   ownerChatIDs,
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

	return cfg, nil
}

// setDefaults fills in the optional fields that were not supplied through the
// environment.
func (cfg *Config) setDefaults() {
	if cfg.SQLitePath == "" {
		cfg.SQLitePath = constants.DefaultSQLitePath
	}
	if cfg.HTTPPort == 0 {
		cfg.HTTPPort = constants.DefaultHTTPPort
	}
}

// init initializes the logging configuration, loads the global configuration
// from environment variables, validates it, and sets up global variables for
// backward compatibility. This function is called automatically at package import.
func init() {
	// Skip config validation when running in CLI mode (--version, --health)
	// This allows these flags to work without requiring valid configuration.
	if isCliModeActive() {
		AppConfig = &Config{}
		return
	}

	// set logger config
	log.SetLevel(log.InfoLevel)
	log.SetFormatter(
		&log.JSONFormatter{
			DisableHTMLEscape: true,
			PrettyPrint:       false,
			CallerPrettyfier: func(f *runtime.Frame) (string, string) {
				return f.Function, fmt.Sprintf("%s:%d", path.Base(f.File), f.Line)
			},
		},
	)

	// Install the sensitive-data redaction hook before any configuration is
	// loaded. Structural patterns (bot tokens, bearer tokens) are scrubbed
	// immediately; exact secrets are registered below once known.
	logredact.Install(nil)

	// Load the structured configuration
	cfg, err := LoadConfig()
	if err != nil {
		// If essential env vars are missing (e.g., during unit tests), provide zero-value config
		if os.Getenv("BOT_TOKEN") == "" {
			AppConfig = &Config{}
			return
		}
		log.Fatalf("[Config] Failed to load configuration: %v", err)
	}

	// Set global configuration instance
	AppConfig = cfg

	// Register the now-known exact secrets so they are scrubbed from any log
	// line that happens to include them verbatim.
	logredact.RegisterSecret(
		cfg.BotToken,
		cfg.WebhookSecret,
		cfg.TypeSafeAPIKey,
	)

	log.SetLevel(cfg.LogLevel)
	// Caller reporting is expensive, so it is limited to the debug levels.
	log.SetReportCaller(cfg.LogLevel >= log.DebugLevel)

	log.Info("[Config] Configuration loaded and validated successfully")
}
