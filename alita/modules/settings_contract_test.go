//go:build testtools

package modules

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/aispam"
	"github.com/divkix/Alita_Robot/alita/db/antiflood"
	"github.com/divkix/Alita_Robot/alita/db/antiraid"
	"github.com/divkix/Alita_Robot/alita/db/captcha"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/chattz"
	"github.com/divkix/Alita_Robot/alita/db/logchannels"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/nightmode"
	"github.com/divkix/Alita_Robot/alita/db/spamdbsettings"
	"github.com/divkix/Alita_Robot/alita/db/warns"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/panel"
	"github.com/divkix/Alita_Robot/alita/settings"
	"github.com/divkix/Alita_Robot/alita/utils/keyboard"
)

type settingContractEdge struct {
	value  any
	input  string
	label  string
	accept bool
}

type settingContract struct {
	key           string
	command       string
	defaultValue  any
	defaultLabel  string
	statusToken   string
	refusalKey    string
	refusals      map[string]string
	seedInvalid   func(int64) error
	edges         []settingContractEdge
	button        string
	noPanel       bool
	changeInfo    bool
	restrict      bool
	botAdmin      bool
	silentMember  bool
	owner         bool
	statusOn      string
	statusOff     string
	setOn         string
	setOff        string
	directChecks  bool
	refusalParams map[string]i18n.TranslationParams
	helpRefusal   bool
}

// Expectations come from the command contract, not the declarations. Later
// settings add rows here, including their optional button input and side effect.
func TestDeclaredSettingsAcrossSurfaces(t *testing.T) {
	previousConfig := *config.AppConfig
	t.Cleanup(func() { *config.AppConfig = previousConfig })
	config.AppConfig.TypeSafeAPIKey = "contract-test-key"
	config.AppConfig.EnableAISpam = true
	config.AppConfig.EnableSpamDB = true
	config.AppConfig.EnableReputation = true
	config.AppConfig.EnableDigest = true
	useRealLocales(t)
	for _, setting := range []settingContract{
		{
			key: "antiraid.raid_action_time", command: "/raidactiontime",
			botAdmin:     true,
			defaultValue: 3600, defaultLabel: "1h", statusToken: "Action: ",
			seedInvalid: func(chatID int64) error { return antiraid.SetRaidActionTime(chatID, 5) },
			refusalKey:  "antiraid_invalid_duration",
			edges: []settingContractEdge{
				{29, "29s", "29s", false},
				{30, "30s", "30s", true},
				{31622400, "366d", "366d", true},
				{31622401, "31622401s", "31622401s", false},
			},
		},
		{
			key: "antiraid.raid_time", command: "/raidtime",
			botAdmin:     true,
			defaultValue: 21600, defaultLabel: "6h", statusToken: "Raid: ",
			refusalKey: "antiraid_invalid_duration",
			seedInvalid: func(chatID int64) error {
				if err := antiraid.SetRaidTime(chatID, 60); err != nil {
					return err
				}
				return antiraid.SetRaidTime(chatID, 0)
			},
			edges: []settingContractEdge{
				{0, "0s", "0s", false},
				{1, "1s", "1s", true},
				{31622400, "366d", "366d", true},
				{31622401, "31622401s", "31622401s", false},
			},
		},
		{
			key: "antiraid.auto_threshold", command: "/autoantiraid",
			botAdmin:     true,
			defaultValue: 0, defaultLabel: "off", statusToken: "Auto: ",
			refusalKey: "antiraid_invalid_threshold",
			seedInvalid: func(chatID int64) error {
				return db.DB.Create(&models.AntiRaidSettings{ChatID: chatID, AutoAntiRaidThreshold: -1}).Error
			},
			edges: []settingContractEdge{
				{-1, "-1", "-1", false},
				{0, "off", "off", true},
				{1, "1", "1", true},
				{2147483647, "2147483647", "2147483647", true},
				{2147483648, "2147483648", "2147483648", false},
			},
		},
		{
			key: "greetings.goodbye_enabled", command: "/goodbye", button: "gb",
			defaultValue: true, defaultLabel: "on",
			statusToken: "I am currently goodbying users: <code>",
			refusalKey:  "greetings_goodbye_invalid",
			setOn:       "greetings_goodbye_enable", setOff: "greetings_goodbye_disable",
			edges: []settingContractEdge{{false, "off", "off", true}, {true, "on", "on", true}, {"maybe", "maybe", "maybe", false}},
		},
		{
			key: "greetings.welcome_enabled", command: "/welcome", button: "we",
			defaultValue: true, defaultLabel: "on",
			statusToken: "I am currently welcoming users: <code>",
			refusalKey:  "greetings_welcome_invalid_option",
			setOn:       "greetings_welcome_enabled", setOff: "greetings_welcome_disabled",
			edges: []settingContractEdge{{false, "off", "off", true}, {true, "on", "on", true}, {"maybe", "maybe", "maybe", false}},
		},
		{
			key: "greetings.clean_welcome", command: "/cleanwelcome", button: "cw", changeInfo: true,
			defaultValue: false, defaultLabel: "off",
			statusOn: "greetings_clean_welcome_should", statusOff: "greetings_clean_welcome_not",
			refusalKey: "greetings_clean_welcome_invalid_option",
			setOn:      "greetings_clean_welcome_enable", setOff: "greetings_clean_welcome_disable",
			edges: []settingContractEdge{{false, "off", "off", true}, {true, "on", "on", true}, {"maybe", "maybe", "maybe", false}},
		},
		{
			key: "greetings.clean_goodbye", command: "/cleangoodbye", button: "cg", changeInfo: true,
			defaultValue: false, defaultLabel: "off",
			statusOn: "greetings_clean_goodbye_should", statusOff: "greetings_clean_goodbye_not",
			refusalKey: "greetings_clean_goodbye_invalid_option",
			setOn:      "greetings_clean_goodbye_enable", setOff: "greetings_clean_goodbye_disable",
			edges: []settingContractEdge{{false, "off", "off", true}, {true, "on", "on", true}, {"maybe", "maybe", "maybe", false}},
		},
		{
			key: "greetings.clean_service", command: "/cleanservice", button: "cs", changeInfo: true, botAdmin: true,
			defaultValue: false, defaultLabel: "off",
			statusOn: "greetings_clean_service_should", statusOff: "greetings_clean_service_not",
			refusalKey: "greetings_clean_service_invalid_option",
			setOn:      "greetings_clean_service_enable", setOff: "greetings_clean_service_disable",
			edges: []settingContractEdge{{false, "off", "off", true}, {true, "on", "on", true}, {"maybe", "maybe", "maybe", false}},
		},
		{
			key: "greetings.auto_approve", command: "/autoapprove", button: "aa", changeInfo: true, botAdmin: true, noPanel: true,
			defaultValue: false, defaultLabel: "off",
			statusOn: "greetings_auto_approve_enabled", statusOff: "greetings_auto_approve_disabled",
			refusalKey: "greetings_auto_approve_invalid_option",
			setOn:      "greetings_auto_approve_enable", setOff: "greetings_auto_approve_disable",
			edges: []settingContractEdge{{false, "off", "off", true}, {true, "on", "on", true}, {"maybe", "maybe", "maybe", false}},
		},
		{
			key: "captcha.enabled", command: "/captcha", button: "cp", botAdmin: true,
			defaultValue: false, defaultLabel: "off", statusToken: "Status: <code>",
			refusalKey: "captcha_usage", setOn: "captcha_enabled_success", setOff: "captcha_disabled_success",
			edges: []settingContractEdge{{false, "off", "off", true}, {true, "on", "on", true}, {"maybe", "maybe", "maybe", false}},
		},
		{
			key: "captcha.join_requests", command: "/captchajoin", button: "cj", noPanel: true,
			defaultValue: false, defaultLabel: "off", statusToken: "Join requests in PM: <code>",
			refusalKey: "captcha_join_requests_usage", setOn: "captcha_join_requests_enabled", setOff: "captcha_join_requests_disabled",
			edges: []settingContractEdge{{false, "off", "off", true}, {true, "on", "on", true}, {"maybe", "maybe", "maybe", false}},
		},
		{
			key: "captcha.mode", command: "/captchamode", button: "cm",
			defaultValue: "math", defaultLabel: "math", statusToken: "Mode: <code>", refusalKey: "captcha_mode_invalid",
			seedInvalid: invalidCaptchaStoredValue("captcha_mode"),
			edges:       []settingContractEdge{{"math", "math", "math", true}, {"text", "text", "text", true}, {"image", "image", "image", false}},
		},
		{
			key: "captcha.timeout", command: "/captchatime",
			defaultValue: 2, defaultLabel: "2", statusToken: "Timeout: <code>", refusalKey: "captcha_timeout_invalid",
			seedInvalid: invalidCaptchaStoredValue("timeout"),
			edges:       []settingContractEdge{{0, "0", "0", false}, {1, "1", "1", true}, {10, "10", "10", true}, {11, "11", "11", false}},
		},
		{
			key: "captcha.action", command: "/captchaaction", button: "ca",
			defaultValue: "kick", defaultLabel: "kick", statusToken: "Failure Action: <code>", refusalKey: "captcha_action_invalid",
			seedInvalid: invalidCaptchaStoredValue("failure_action"),
			edges:       []settingContractEdge{{"kick", "kick", "kick", true}, {"ban", "ban", "ban", true}, {"mute", "mute", "mute", true}, {"warn", "warn", "warn", false}},
		},
		{
			key: "captcha.max_attempts", command: "/captchamaxattempts", botAdmin: true,
			defaultValue: 3, defaultLabel: "3", statusToken: "Max Attempts: <code>", refusalKey: "captcha_max_attempts_invalid",
			seedInvalid: invalidCaptchaStoredValue("max_attempts"),
			edges:       []settingContractEdge{{0, "0", "0", false}, {1, "1", "1", true}, {10, "10", "10", true}, {11, "11", "11", false}},
		},
		{
			key: "antiflood.limit", command: "/setflood", botAdmin: true,
			defaultValue: 0, defaultLabel: "off", refusalKey: "antiflood_errors_set_in_limit",
			seedInvalid: func(chatID int64) error { return antiflood.SetFlood(chatID, 2) },
			edges:       []settingContractEdge{{0, "off", "off", true}, {3, "3", "3", true}, {-1, "-1", "-1", false}, {1, "1", "1", false}, {2, "2", "2", false}, {100, "100", "100", true}, {101, "101", "101", false}},
		},
		{
			key: "antiflood.delete_messages", command: "/delflood", button: "fd", botAdmin: true,
			defaultValue: false, defaultLabel: "off",
			statusOn: "antiflood_flood_deleter_already_enabled", statusOff: "antiflood_flood_deleter_already_disabled",
			setOn: "antiflood_flood_deleter_enabled", setOff: "antiflood_flood_deleter_disabled",
			refusalKey: "antiflood_flood_deleter_invalid_option",
			edges:      []settingContractEdge{{false, "off", "off", true}, {true, "on", "on", true}, {"maybe", "maybe", "maybe", false}},
		},
		{
			key: "warns.limit", command: "/setwarnlimit", botAdmin: true,
			defaultValue: 3, defaultLabel: "3", refusalKey: "warns_limit_range_error",
			seedInvalid: func(chatID int64) error { return warns.SetWarnLimit(chatID, 101) },
			edges:       []settingContractEdge{{0, "0", "0", false}, {1, "1", "1", true}, {100, "100", "100", true}, {101, "101", "101", false}},
		},
		{
			key: "warns.expiry", command: "/setwarntime", botAdmin: true,
			defaultValue: 0, defaultLabel: "off", refusalKey: "extraction_time_limit_exceeded",
			seedInvalid: func(chatID int64) error { return warns.SetWarnExpiry(chatID, 5) },
			edges:       []settingContractEdge{{0, "off", "off", true}, {60, "1m", "1m", true}, {31622400, "366d", "366d", true}, {31622460, "527041m", "527041m", false}},
		},
		{
			key: "antiflood.action", command: "/setfloodmode", button: "fm", botAdmin: true,
			defaultValue: "mute", defaultLabel: "mute", refusalKey: "antiflood_setfloodmode_unknown_type",
			seedInvalid: func(chatID int64) error {
				return db.DB.Model(&models.AntifloodSettings{}).Create(map[string]any{"chat_id": chatID, "action": nil, "flood_limit": 0}).Error
			},
			edges: []settingContractEdge{{"mute", "mute", "mute", true}, {"ban", "ban", "ban", true}, {"kick", "kick", "kick", true}, {"warn", "warn", "warn", false}},
		},
		{
			key: "warns.mode", command: "/setwarnmode", botAdmin: true,
			defaultValue: "mute", defaultLabel: "mute", refusalKey: "warns_mode_unknown",
			seedInvalid: func(chatID int64) error {
				return db.DB.Model(&models.WarnSettings{}).Create(map[string]any{"chat_id": chatID, "warn_mode": nil, "warn_limit": 3}).Error
			},
			edges: []settingContractEdge{{"mute", "mute", "mute", true}, {"ban", "ban", "ban", true}, {"kick", "kick", "kick", true}, {"warn", "warn", "warn", false}},
		},
		{
			key: "antiflood.action_time", command: "/setfloodmode", botAdmin: true,
			defaultValue: 0, defaultLabel: "mute", refusalKey: "extraction_time_limit_exceeded",
			seedInvalid: func(chatID int64) error {
				return db.DB.Create(&models.AntifloodSettings{ChatId: chatID, Action: "tmute", FloodActionTime: 5}).Error
			},
			refusals: map[string]string{"tmute 0m": "extraction_invalid_time_amount", "tmute 59s": "extraction_invalid_time_type", "tmute 31622401s": "extraction_invalid_time_type"},
			edges:    []settingContractEdge{{0, "tmute 0m", "tmute 0m", false}, {60, "tmute 1m", "tmute 1m", true}, {59, "tmute 59s", "tmute 59s", false}, {3600, "tmute 1h", "tmute 1h", true}, {31622400, "tban 366d", "tban 366d", true}, {31622401, "tmute 31622401s", "tmute 31622401s", false}, {31622460, "tmute 527041m", "tmute 527041m", false}},
		},
		{
			key: "warns.action_time", command: "/setwarnmode", botAdmin: true,
			defaultValue: 0, defaultLabel: "mute", refusalKey: "extraction_time_limit_exceeded",
			seedInvalid: func(chatID int64) error { return warns.SetWarnMode(chatID, "tmute", 5) },
			refusals:    map[string]string{"tmute 0m": "extraction_invalid_time_amount", "tmute 59s": "extraction_invalid_time_type", "tmute 31622401s": "extraction_invalid_time_type"},
			edges:       []settingContractEdge{{0, "tmute 0m", "tmute 0m", false}, {60, "tmute 1m", "tmute 1m", true}, {59, "tmute 59s", "tmute 59s", false}, {3600, "tmute 1h", "tmute 1h", true}, {31622400, "tban 366d", "tban 366d", true}, {31622401, "tmute 31622401s", "tmute 31622401s", false}, {31622460, "tmute 527041m", "tmute 527041m", false}},
		},
		{
			key: "antispam.probation_time", command: "/probation time", botAdmin: true, restrict: true,
			defaultValue: 86400, defaultLabel: "1d", refusalKey: "probation_invalid_time",
			seedInvalid: func(chatID int64) error {
				return db.DB.Model(&models.ProbationSettings{}).Create(map[string]any{"chat_id": chatID, "duration_seconds": 5}).Error
			},
			edges: []settingContractEdge{{3599, "3599s", "3599s", false}, {3600, "1h", "1h", true}, {2592000, "30d", "30d", true}, {2592001, "2592001s", "2592001s", false}},
		},
		{
			key: "antispam.probation_messages", command: "/probation messages", botAdmin: true, restrict: true,
			defaultValue: 5, defaultLabel: "5", statusToken: "<b>", refusalKey: "probation_invalid_messages",
			seedInvalid: func(chatID int64) error {
				return db.DB.Model(&models.ProbationSettings{}).Create(map[string]any{"chat_id": chatID, "message_count": 101}).Error
			},
			edges: []settingContractEdge{{0, "0", "0", false}, {1, "1", "1", true}, {100, "100", "100", true}, {101, "101", "101", false}},
		},
		{
			key: "antispam.probation_min_length", command: "/probation minlen", botAdmin: true, restrict: true,
			defaultValue: 20, defaultLabel: "20", statusToken: "<b>", refusalKey: "probation_invalid_minlen",
			seedInvalid: func(chatID int64) error {
				return db.DB.Model(&models.ProbationSettings{}).Create(map[string]any{"chat_id": chatID, "min_length": 501}).Error
			},
			edges: []settingContractEdge{{0, "0", "0", false}, {1, "1", "1", true}, {500, "500", "500", true}, {501, "501", "501", false}},
		},
		{
			key: "antispam.probation_enabled", command: "/probation", button: "pb", botAdmin: true, restrict: true,
			defaultValue: false, defaultLabel: "off", statusToken: "probation is <b>", refusalKey: "probation_usage",
			edges: []settingContractEdge{{false, "off", "off", true}, {true, "on", "on", true}, {"maybe", "maybe", "maybe", false}},
		},
		{
			key: "antispam.probation_notice", command: "/probation notice", button: "pv", botAdmin: true, restrict: true,
			defaultValue: false, defaultLabel: "off", statusToken: "<b>Notice:</b> ", refusalKey: "probation_usage",
			edges: []settingContractEdge{{false, "off", "off", true}, {true, "on", "on", true}, {"maybe", "maybe", "maybe", false}},
		},
		{
			key: "antispam.probation_block_links", command: "/probation block", botAdmin: true, restrict: true,
			defaultValue: true, defaultLabel: "links, forwards", refusalKey: "probation_invalid_block",
			edges: []settingContractEdge{{false, "forwards", "forwards", true}, {true, "links,forwards", "links, forwards", true}, {"video", "video", "video", false}},
		},
		{
			key: "antispam.probation_block_forwards", command: "/probation block", botAdmin: true, restrict: true,
			defaultValue: true, defaultLabel: "links, forwards", refusalKey: "probation_invalid_block",
			edges: []settingContractEdge{{false, "links", "links", true}, {true, "links,forwards", "links, forwards", true}, {"video", "video", "video", false}},
		},
		{
			key: "antispam.probation_block_media", command: "/probation block", botAdmin: true, restrict: true,
			defaultValue: false, defaultLabel: "links, forwards", refusalKey: "probation_invalid_block",
			edges: []settingContractEdge{{false, "links,forwards", "links, forwards", true}, {true, "links,forwards,media", "links, forwards, media", true}, {"video", "video", "video", false}},
		},
		{
			key: "antispam.spamdb_action", command: "/spamdb action",
			defaultValue: "ban", defaultLabel: "ban", refusalKey: "spamdb_invalid_action",
			seedInvalid: func(chatID int64) error {
				return db.DB.Model(&models.SpamDBSettings{}).Create(map[string]any{"chat_id": chatID, "action": nil}).Error
			},
			edges: []settingContractEdge{{"ban", "ban", "ban", true}, {"mute", "mute", "mute", true}, {"log", "log", "log", true}, {"warn", "warn", "warn", false}},
		},
		{
			key: "antispam.aispam_enabled", command: "/aispam", button: "ai",
			defaultValue: false, defaultLabel: "off", statusToken: "filter is <b>", refusalKey: "aispam_invalid_option",
			setOn: "aispam_enabled_success", setOff: "aispam_disabled_success",
			edges: []settingContractEdge{{false, "off", "off", true}, {true, "on", "on", true}, {"maybe", "maybe", "maybe", false}},
		},
		{
			key: "antispam.spamdb_enabled", command: "/spamdb",
			defaultValue: false, defaultLabel: "off", statusToken: "check is <b>", refusalKey: "spamdb_invalid_option",
			setOn: "spamdb_enabled_success", setOff: "spamdb_disabled_success",
			edges: []settingContractEdge{{false, "off", "off", true}, {true, "on", "on", true}, {"maybe", "maybe", "maybe", false}},
		},
		{
			key: "antispam.spamdb_cas", command: "/spamdb db cas",
			defaultValue: true, defaultLabel: "on", statusToken: "CAS (", refusalKey: "spamdb_db_usage",
			edges:    []settingContractEdge{{false, "off", "off", false}, {true, "on", "on", true}, {"maybe", "maybe", "maybe", false}},
			refusals: map[string]string{"off": "spamdb_db_last"},
		},
		{
			key: "antispam.spamdb_lols", command: "/spamdb db lols",
			defaultValue: false, defaultLabel: "off", statusToken: "lols.bot (", refusalKey: "spamdb_db_usage",
			edges: []settingContractEdge{{false, "off", "off", true}, {true, "on", "on", true}, {"maybe", "maybe", "maybe", false}},
		},
		{
			key: "antispam.spamdb_shared", command: "/spamdb db shared",
			defaultValue: false, defaultLabel: "off", statusToken: "Shared bans (", refusalKey: "spamdb_db_usage",
			edges: []settingContractEdge{{false, "off", "off", true}, {true, "on", "on", true}, {"maybe", "maybe", "maybe", false}},
		},
		{
			key: "reports.enabled", command: "/reports", button: "rp", botAdmin: true,
			defaultValue: true, defaultLabel: "on", refusalKey: "reports_invalid_input",
			statusOn: "reports_status_enabled_group", statusOff: "reports_status_disabled_group",
			setOn: "reports_turned_on_group", setOff: "reports_turned_off_group",
			edges: []settingContractEdge{{false, "off", "off", true}, {true, "on", "on", true}, {"maybe", "maybe", "maybe", false}},
		},
		{
			key: "reports.mute_threshold", command: "/reports mute", botAdmin: true, restrict: true,
			defaultValue: 0, defaultLabel: "off", refusalKey: "reports_automute_usage",
			seedInvalid: func(chatID int64) error {
				return db.DB.Model(&models.ReportChatSettings{}).Create(map[string]any{"chat_id": chatID, "status": true, "mute_threshold": -1}).Error
			},
			refusalParams: map[string]i18n.TranslationParams{"1": {"min": 2, "max": 20}, "21": {"min": 2, "max": 20}, "-1": {"min": 2, "max": 20}},
			edges:         []settingContractEdge{{0, "off", "off", true}, {2, "2", "2", true}, {1, "1", "1", false}, {20, "20", "20", true}, {21, "21", "21", false}, {-1, "-1", "-1", false}},
		},
		{
			key: "rules.private_rules", command: "/privaterules", button: "pr", botAdmin: true,
			defaultValue: false, defaultLabel: "off", refusalKey: "pins_input_not_recognized",
			edges: []settingContractEdge{{false, "off", "off", true}, {true, "on", "on", true}, {"maybe", "maybe", "maybe", false}},
		},
		{
			key: "digest.enabled", command: "/digest", botAdmin: true, directChecks: true,
			defaultValue: false, defaultLabel: "off", refusalKey: "digest_usage", helpRefusal: true,
			edges: []settingContractEdge{{false, "off", "off", true}, {true, "on", "on", true}, {"maybe", "maybe", "maybe", false}},
		},
		{
			key: "digest.destination", command: "/digest", botAdmin: true, directChecks: true,
			defaultValue: "", defaultLabel: "the log channel", refusalKey: "digest_usage", helpRefusal: true,
			edges: []settingContractEdge{{"", "", "the log channel", true}, {"group", "here", "this group", true}, {"log", "log", "the log channel", true}, {"other", "other", "other", false}},
		},
		{
			key: "timezone.name", command: "/timezone", changeInfo: true, directChecks: true,
			defaultValue: "UTC", defaultLabel: "UTC", statusToken: "timezone is <b>", refusalKey: "timezone_invalid", helpRefusal: true,
			refusalParams: map[string]i18n.TranslationParams{"Not/A_Zone": {"input": "Not/A_Zone"}},
			seedInvalid: func(chatID int64) error {
				return db.DB.Create(&models.ChatTimezone{ChatID: chatID, Timezone: "Local"}).Error
			},
			edges: []settingContractEdge{{"UTC", "UTC", "UTC", true}, {"Asia/Kolkata", "asia/kolkata", "Asia/Kolkata", true}, {"Europe/Berlin", "Europe/Berlin", "Europe/Berlin", true}, {"Not/A_Zone", "Not/A_Zone", "Not/A_Zone", false}, {"Local", "Local", "Local", false}, {"+05:30", "+05:30", "+05:30", false}, {"../UTC", "../UTC", "../UTC", false}},
		},
		{
			key: "language.code", command: "/lang group", directChecks: true,
			defaultValue: "en", defaultLabel: "en", refusalKey: "language_invalid_selection",
			seedInvalid: func(chatID int64) error {
				return db.DB.Model(&models.Chat{}).Where("chat_id = ?", chatID).Update("language", "xx").Error
			},
			edges: []settingContractEdge{{"en", "en", "en", true}, {"es", "es", "es", true}, {"fr", "fr", "fr", true}, {"hi", "hi", "hi", true}, {"id", "id", "id", true}, {"pt", "pt", "pt", true}, {"ru", "ru", "ru", true}, {"xx", "xx", "xx", false}},
		},
		{
			key: "nightmode.enabled", command: "/nightmode", botAdmin: true, directChecks: true,
			defaultValue: false, defaultLabel: "off", refusalKey: "nightmode_invalid_time", helpRefusal: true,
			refusalParams: map[string]i18n.TranslationParams{"maybe": {"input": "maybe"}},
			edges:         []settingContractEdge{{false, "off", "off", true}, {true, "23:00 07:00 media,url", "23:00-07:00 media, url", true}, {"maybe", "maybe", "maybe", false}},
		},
		{
			key: "nightmode.start", command: "/nightmode", botAdmin: true, directChecks: true,
			defaultValue: "00:00", defaultLabel: "off", refusalKey: "nightmode_invalid_time", helpRefusal: true,
			refusals:      map[string]string{"07:00 07:00 media,url": "nightmode_same_times"},
			refusalParams: map[string]i18n.TranslationParams{"24:00 07:00 media,url": {"input": "24:00"}, "-1:00 07:00 media,url": {"input": "-1:00"}},
			seedInvalid: func(chatID int64) error {
				return db.DB.Create(&models.NightModeSettings{ChatID: chatID, StartMinute: -1}).Error
			},
			edges: []settingContractEdge{{"00:00", "0:00 07:00 media,url", "00:00-07:00 media, url", true}, {"23:59", "23:59 07:00 media,url", "23:59-07:00 media, url", true}, {"24:00", "24:00 07:00 media,url", "24:00", false}, {"-1:00", "-1:00 07:00 media,url", "-1:00", false}, {"07:00", "07:00 07:00 media,url", "07:00", false}},
		},
		{
			key: "nightmode.end", command: "/nightmode", botAdmin: true, directChecks: true,
			defaultValue: "00:00", defaultLabel: "off", refusalKey: "nightmode_invalid_time", helpRefusal: true,
			refusalParams: map[string]i18n.TranslationParams{"23:00 24:00 media,url": {"input": "24:00"}},
			seedInvalid: func(chatID int64) error {
				return db.DB.Create(&models.NightModeSettings{ChatID: chatID, EndMinute: 1440}).Error
			},
			edges: []settingContractEdge{{"00:00", "23:00 0:00 media,url", "23:00-00:00 media, url", true}, {"23:59", "23:00 23:59 media,url", "23:00-23:59 media, url", true}, {"24:00", "23:00 24:00 media,url", "24:00", false}},
		},
		{
			key: "nightmode.lock_types", command: "/nightmode", botAdmin: true, directChecks: true,
			defaultValue: "", defaultLabel: "off", refusalKey: "nightmode_invalid_type", helpRefusal: true,
			refusals:      map[string]string{"23:00 07:00": "nightmode_no_types"},
			refusalParams: map[string]i18n.TranslationParams{"23:00 07:00 bots": {"type": "bots"}, "23:00 07:00 unknown": {"type": "unknown"}},
			seedInvalid: func(chatID int64) error {
				return db.DB.Create(&models.NightModeSettings{ChatID: chatID, LockTypes: "bots"}).Error
			},
			edges: []settingContractEdge{{"media,url", "23:00 07:00 MEDIA,url,media", "23:00-07:00 media, url", true}, {"all", "23:00 07:00 all", "23:00-07:00 all", true}, {"bots", "23:00 07:00 bots", "bots", false}, {"unknown", "23:00 07:00 unknown", "unknown", false}, {"", "23:00 07:00", "empty", false}},
		},
		{
			key: "notes.private_notes", command: "/privnote", button: "pn", noPanel: true, directChecks: true,
			defaultValue: false, defaultLabel: "off",
			statusOn: "notes_private_status_on", statusOff: "notes_private_status_off",
			setOn: "notes_private_enabled", setOff: "notes_private_disabled", refusalKey: "notes_private_invalid_option",
			edges: []settingContractEdge{{false, "off", "off", true}, {true, "on", "on", true}, {"maybe", "maybe", "maybe", false}},
		},
		{
			key: "pins.anti_channel_pin", command: "/antichannelpin", button: "ap", noPanel: true, botAdmin: true,
			defaultValue: false, defaultLabel: "off",
			statusOn: "pins_antichannelpin_current_enabled", statusOff: "pins_antichannelpin_current_disabled",
			setOn: "pins_antichannelpin_enabled", setOff: "pins_antichannelpin_disabled", refusalKey: "pins_input_not_recognized",
			edges: []settingContractEdge{{false, "off", "off", true}, {true, "on", "on", true}, {"maybe", "maybe", "maybe", false}},
		},
		{
			key: "pins.clean_linked", command: "/cleanlinked", button: "cl", noPanel: true, botAdmin: true,
			defaultValue: false, defaultLabel: "off",
			statusOn: "pins_cleanlinked_current_enabled", statusOff: "pins_cleanlinked_current_disabled",
			setOn: "pins_cleanlinked_enabled", setOff: "pins_cleanlinked_disabled", refusalKey: "pins_input_not_recognized",
			edges: []settingContractEdge{{false, "off", "off", true}, {true, "on", "on", true}, {"maybe", "maybe", "maybe", false}},
		},
		{
			key: "connections.allow_connect", command: "/allowconnect", button: "ac", noPanel: true, silentMember: true,
			defaultValue: true, defaultLabel: "on",
			statusOn: "connections_allow_connect_currently_on", statusOff: "connections_allow_connect_currently_off",
			setOn: "connections_allow_connect_turned_on", setOff: "connections_allow_connect_turned_off", refusalKey: "connections_invalid_option",
			edges: []settingContractEdge{{false, "off", "off", true}, {true, "on", "on", true}, {"maybe", "maybe", "maybe", false}},
		},
		{
			key: "admin.anon_admin", command: "/anonadmin", button: "an", noPanel: true, botAdmin: true, owner: true,
			defaultValue: false, defaultLabel: "off",
			statusOn: "admin_anon_admin_enabled", statusOff: "admin_anon_admin_disabled",
			setOn: "admin_anon_admin_enabled_now", setOff: "admin_anon_admin_already_disabled", refusalKey: "admin_anon_admin_invalid_arg",
			edges: []settingContractEdge{{false, "off", "off", true}, {true, "on", "on", true}, {"maybe", "maybe", "maybe", false}},
		},
	} {
		if setting.key == "nightmode.lock_types" {
			// Expectations come from the locks owner, not the declaration's attachment.
			for _, lockType := range locksModule.getLockMapAsArray() {
				if lockType != "bots" && lockType != "all" {
					setting.edges = append(setting.edges, settingContractEdge{
						lockType, "23:00 07:00 " + lockType, "23:00-07:00 " + lockType, true,
					})
				}
			}
		}
		t.Run(setting.key, func(t *testing.T) {
			t.Run("default without creating a row", func(t *testing.T) {
				f := newSettingContractFixture(t, setting)
				assertContractRead(t, f, setting, setting.defaultValue, setting.defaultLabel, false)
				assertContractRowCount(t, f, setting, 0)
			})
			t.Run("invalid stored value uses default", func(t *testing.T) {
				if setting.seedInvalid == nil {
					return
				}
				f := newSettingContractFixture(t, setting)
				if err := setting.seedInvalid(f.chat.Id); err != nil {
					t.Fatal(err)
				}
				if isContractModeDuration(setting) {
					assertContractRead(t, f, setting, 3600, "tmute 1h", false)
				} else {
					assertContractRead(t, f, setting, setting.defaultValue, setting.defaultLabel, false)
				}
				if setting.key == "timezone.name" && chattz.Location(f.chat.Id).String() != "UTC" {
					t.Fatal("invalid timezone did not use UTC for local-time features")
				}
			})
			for _, rights := range []struct {
				name        string
				callerAdmin bool
				botAdmin    bool
				changeInfo  bool
				restrict    bool
				owner       bool
			}{
				{"member", false, true, false, setting.restrict, false},
				{"exact right", true, true, setting.changeInfo, setting.restrict, setting.owner},
				{"admin not owner", true, true, setting.changeInfo, setting.restrict, false},
				{"no change info", true, true, false, setting.restrict, setting.owner},
				{"no restrict", true, true, setting.changeInfo, false, setting.owner},
				{"bot member", true, false, setting.changeInfo, setting.restrict, setting.owner},
			} {
				t.Run("rights/"+rights.name, func(t *testing.T) {
					f := newSettingContractFixture(t, setting)
					contractRights(f, rights.changeInfo, rights.botAdmin, rights.restrict)
					if rights.owner {
						contractMakeOwner(f)
					}
					actor := f.member
					if rights.callerAdmin {
						actor = f.admin
					}
					edge := setting.edges[1]
					accepted := rights.callerAdmin && (!setting.owner || rights.owner) && (!setting.botAdmin || rights.botAdmin) && (!setting.changeInfo || rights.changeInfo) && (!setting.restrict || rights.restrict)
					refusal := "connections_is_user_connected_user_not_admin"
					if setting.directChecks {
						refusal = "chat_status_user_admin_cmd_error"
					}
					if setting.owner && !rights.owner {
						refusal = "chat_status_owner_cmd_error"
					}
					if setting.botAdmin && !rights.botAdmin {
						refusal = "connections_is_user_connected_bot_not_admin"
						if setting.directChecks {
							refusal = "chat_status_bot_not_admin"
						}
					} else if rights.callerAdmin && setting.changeInfo && !rights.changeInfo {
						refusal = "chat_status_change_info_cmd_error"
					} else if rights.callerAdmin && setting.restrict && !rights.restrict {
						refusal = "chat_status_restrict_cmd_error"
					}
					reply := contractSet(t, f, actor, setting, edge.input)
					wantReply := translated(t, refusal)
					if setting.silentMember && !rights.callerAdmin {
						wantReply = ""
					}
					if !accepted && reply != wantReply {
						t.Fatalf("command refusal = %q, want %q", reply, wantReply)
					}
					if !accepted {
						assertContractRowCount(t, f, setting, 0)
					}
					if setting.button != "" {
						contractPress(t, f, actor, setting.button, edge.input)
						text, alert := lastAnswer(t, f.client)
						want := reply
						if (setting.directChecks || setting.silentMember) && !rights.callerAdmin {
							want = translated(t, "connections_is_user_connected_user_not_admin")
						}
						if setting.owner && !rights.owner && rights.botAdmin {
							want = translated(t, "chat_status_owner_button_error")
							if !rights.callerAdmin {
								want = translated(t, "connections_is_user_connected_user_not_admin")
							}
						}
						if accepted == alert || (!accepted && text != want) {
							t.Fatalf("button answer = %q alert %t, want %q accepted %t", text, alert, want, accepted)
						}
					}
					if !setting.noPanel {
						response := contractPanelRequest(t, f, actor, http.MethodPatch, contractChanges(t, contractSettingChanges(setting, edge)))
						if accepted && rights.botAdmin {
							if response.Code != http.StatusOK {
								t.Fatalf("exact right refused: %d %s", response.Code, response.Body.String())
							}
						} else if response.Code != http.StatusForbidden && response.Code != http.StatusConflict {
							t.Fatalf("panel accepted missing right: %d %s", response.Code, response.Body.String())
						}
					}
					if !accepted {
						assertContractRowCount(t, f, setting, 0)
					} else if rights.botAdmin {
						assertContractRead(t, f, setting, edge.value, edge.label, edge.value != setting.defaultValue)
					}
				})
			}
			for _, edge := range setting.edges {
				t.Run(edge.input, func(t *testing.T) {
					if setting.noPanel {
						return
					}
					f := newSettingContractFixture(t, setting)
					assertContractRead(t, f, setting, setting.defaultValue, setting.defaultLabel, false)
					changes := contractSettingChanges(setting, edge)
					if !edge.accept {
						sibling := "antiraid.raid_time"
						if sibling == setting.key {
							sibling = "antiraid.raid_action_time"
						}
						changes[sibling] = 60
					}
					payload, err := json.Marshal(map[string]any{"changes": changes})
					if err != nil {
						t.Fatal(err)
					}
					response := contractPanelRequest(t, f, f.admin, http.MethodPatch, string(payload))
					var result struct {
						Saved  []string          `json:"saved"`
						Failed map[string]string `json:"failed"`
						Errors map[string]string `json:"errors"`
					}
					if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
						t.Fatal(err)
					}
					if edge.accept {
						if response.Code != http.StatusOK {
							t.Fatalf("panel refused %v: %d %s", edge.value, response.Code, response.Body.String())
						}
						if !slices.Contains(result.Saved, setting.key) || len(result.Failed) != 0 {
							t.Fatalf("per-field outcomes = %+v", result)
						}
						assertContractRead(t, f, setting, edge.value, edge.label, edge.value != setting.defaultValue)
					} else {
						if response.Code != http.StatusBadRequest || result.Errors[setting.key] == "" || len(result.Saved) != 0 || len(result.Failed) != 0 {
							t.Fatalf("panel accepted invalid %v: %d %s", edge.value, response.Code, response.Body.String())
						}
						assertContractRowCount(t, f, setting, 0)
					}
				})
				t.Run("command/"+edge.input, func(t *testing.T) {
					f := newSettingContractFixture(t, setting)
					assertContractRead(t, f, setting, setting.defaultValue, setting.defaultLabel, false)
					reply := contractSet(t, f, f.admin, setting, edge.input)
					if edge.accept {
						if setting.setOn != "" {
							key := setting.setOff
							if edge.value == true {
								key = setting.setOn
							}
							want := contractSwitchReply(t, f, key)
							if reply != want {
								t.Fatalf("command reply = %q, want %q", reply, want)
							}
						} else if edge.input == "off" && setting.command == "/autoantiraid" {
							if reply != translated(t, "antiraid_auto_disabled") {
								t.Fatalf("off reply = %q", reply)
							}
						} else if strings.HasPrefix(setting.command, "/spamdb db ") {
							key := "spamdb_db_disabled"
							if edge.value == true {
								key = "spamdb_db_enabled_idle"
							}
							name := strings.TrimPrefix(setting.command, "/spamdb db ")
							database := map[string]string{"cas": "CAS", "lols": "lols.bot", "shared": "Shared bans"}[name]
							want := translatedWith(t, key, i18n.TranslationParams{"database": database})
							if !strings.HasPrefix(reply, want) {
								t.Fatalf("database reply = %q, want %q", reply, want)
							}
						} else if !isContractRemaining(setting) && edge.input != "off" && !strings.Contains(reply, lastContractWord(edge.label)) {
							t.Fatalf("command reply = %q, want accepted %s", reply, edge.label)
						}
						assertContractRead(t, f, setting, edge.value, edge.label, edge.value != setting.defaultValue)
					} else {
						key := setting.refusalKey
						if specific := setting.refusals[edge.input]; specific != "" {
							key = specific
						}
						refusal := translated(t, key)
						if setting.key == "timezone.name" {
							refusal = translatedWith(t, key, i18n.TranslationParams{"input": edge.input})
						}
						if params := setting.refusalParams[edge.input]; params != nil {
							refusal = translatedWith(t, key, params)
						}
						if strings.Contains(refusal, "%s") {
							refusal = fmt.Sprintf(refusal, lastContractWord(edge.input))
						}
						if key == "spamdb_db_last" {
							refusal = translatedWith(t, key, i18n.TranslationParams{"database": "CAS"})
						}
						if reply != refusal && ((!setting.helpRefusal && key != "probation_usage") || !strings.HasPrefix(reply, refusal+"\n")) {
							t.Fatalf("command accepted invalid value: %q, want %q", reply, refusal)
						}
						assertContractRead(t, f, setting, setting.defaultValue, setting.defaultLabel, false)
					}
				})
				if setting.button != "" {
					t.Run("button/"+edge.input, func(t *testing.T) {
						f := newSettingContractFixture(t, setting)
						assertContractRead(t, f, setting, setting.defaultValue, setting.defaultLabel, false)
						contractPress(t, f, f.admin, setting.button, edge.input)
						text, alert := lastAnswer(t, f.client)
						if edge.accept {
							if alert {
								t.Fatalf("button refused %s: %q", edge.input, text)
							}
							assertContractRead(t, f, setting, edge.value, edge.label, edge.value != setting.defaultValue)
						} else {
							if text != translated(t, "common_callback_invalid_request") {
								t.Fatalf("button accepted invalid %s: %q", edge.input, text)
							}
							assertContractRead(t, f, setting, setting.defaultValue, setting.defaultLabel, false)
							assertContractRowCount(t, f, setting, 0)
						}
					})
				}
			}
			if setting.key == "antiflood.action" || setting.key == "warns.mode" {
				testContractModeRelationship(t, setting)
			}
			if setting.key == "antispam.spamdb_action" {
				t.Run("invalid action enforces default ban", func(t *testing.T) {
					f := newSettingContractFixture(t, setting)
					if err := setting.seedInvalid(f.chat.Id); err != nil {
						t.Fatal(err)
					}
					if err := spamdbsettings.SetSpamDBEnabled(f.chat.Id, true); err != nil {
						t.Fatal(err)
					}
					dispatcher, _ := withSpamDBCAS(t, true, http.StatusOK, spamDBCASListed)
					spamDBJoin(t, dispatcher, f.bot, f.chat, f.member)
					assertParams(t, f.client.onlyCall(t, "banChatMember"), targetParams(f.chat.Id, f.member.Id))
					var row models.SpamDBSettings
					if err := db.DB.Where("chat_id = ?", f.chat.Id).Take(&row).Error; err != nil || row.Action != "" {
						t.Fatalf("normalization repaired persistence: %+v, %v", row, err)
					}
				})
			}
			testContractSpamPreconditions(t, setting)
			if strings.HasPrefix(setting.key, "digest.") {
				testContractDigestPreconditions(t, setting)
			}
			if strings.HasPrefix(setting.command, "/spamdb db ") {
				testContractSpamDatabases(t, setting)
			}
			if setting.restrict {
				t.Run("bot needs restrict-members", func(t *testing.T) {
					f := newSettingContractFixture(t, setting)
					f.client.membersByChat[fmt.Sprintf("%d|999", f.chat.Id)] = json.RawMessage(`{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"}}`)
					f.client.adminsByChat[fmt.Sprint(f.chat.Id)] = json.RawMessage("[" + string(f.client.membersByChat[fmt.Sprintf("%d|999", f.chat.Id)]) + "," + string(f.client.membersByUser[fmt.Sprint(f.admin.Id)]) + "]")
					contractDispatch(t, f, f.admin, setting.command+" "+setting.edges[1].input)
					reply := fmt.Sprint(lastCall(t, f.client, "sendMessage").Params["text"])
					if reply != translated(t, "chat_status_bot_restrict_group_error") {
						t.Fatalf("bot-right refusal = %q", reply)
					}
					if setting.button != "" {
						contractPress(t, f, f.admin, setting.button, setting.edges[1].input)
						if text, alert := lastAnswer(t, f.client); !alert || text != reply {
							t.Fatalf("button bot-right refusal = %q alert %t", text, alert)
						}
					}
					response := contractPanelRequest(t, f, f.admin, http.MethodPatch, contractChanges(t, contractSettingChanges(setting, setting.edges[1])))
					if response.Code != http.StatusConflict {
						t.Fatalf("panel bot-right refusal = %d %s", response.Code, response.Body.String())
					}
					assertContractRowCount(t, f, setting, 0)
				})
			}
			if setting.key == "antispam.probation_block_links" {
				testContractProbationBlocks(t, setting)
			}
			if setting.key == "warns.expiry" {
				t.Run("invalid preferences enforce declared defaults", func(t *testing.T) {
					f := newSettingContractFixture(t, setting)
					contractRights(f, false, true, true)
					if err := db.DB.Model(&models.WarnSettings{}).Create(map[string]any{
						"chat_id": f.chat.Id, "warn_limit": 101, "warn_mode": nil, "warn_expiry": 5,
					}).Error; err != nil {
						t.Fatal(err)
					}
					seedDatedWarns(t, f.chat.Id, f.member.Id, []string{"first", "second"}, []time.Duration{2 * time.Hour, time.Hour})
					dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
					LoadWarns(dispatcher)
					ctx := newWarnReplyContext(f.bot, f.chat, f.admin, f.member, "/warn third")
					if err := dispatcher.ProcessUpdate(f.bot, ctx.Update, nil); err != nil {
						t.Fatal(err)
					}
					assertParams(t, f.client.onlyCall(t, "restrictChatMember"), targetParams(f.chat.Id, f.member.Id))
					if count, _ := warns.GetWarns(f.member.Id, f.chat.Id); count != 0 {
						t.Fatalf("warnings after the default-limit mute = %d, want reset", count)
					}
				})
			}
			if setting.changeInfo && !setting.noPanel {
				t.Run("rights remain independent in a batch", func(t *testing.T) {
					f := newSettingContractFixture(t, setting)
					contractRights(f, false, true, true)
					response := contractPanelRequest(t, f, f.admin, http.MethodPatch, contractChanges(t, map[string]any{
						setting.key: setting.edges[1].value, "reports.mute_threshold": 2,
					}))
					if response.Code != http.StatusForbidden {
						t.Fatalf("batch bypassed change-info: %d %s", response.Code, response.Body.String())
					}
					assertContractRowCount(t, f, setting, 0)
				})
			}
			if strings.HasPrefix(setting.key, "greetings.") {
				t.Run("write failure", func(t *testing.T) {
					f := newSettingContractFixture(t, setting)
					assertContractRead(t, f, setting, setting.defaultValue, setting.defaultLabel, false)
					name := "fail_greeting_write"
					if err := db.DB.Callback().Create().Before("gorm:create").Register(name, func(tx *gorm.DB) {
						if tx.Statement.Table == "greetings" {
							_ = tx.AddError(errors.New("greeting write failed"))
						}
					}); err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { _ = db.DB.Callback().Create().Remove(name) })
					contractDispatch(t, f, f.admin, setting.command+" on")
					reply := fmt.Sprint(lastCall(t, f.client, "sendMessage").Params["text"])
					if reply != translated(t, "common_settings_save_failed") {
						t.Fatalf("write failure reply = %q", reply)
					}
					contractPress(t, f, f.admin, setting.button, "on")
					if text, alert := lastAnswer(t, f.client); !alert || text != reply {
						t.Fatalf("button failure = %q alert %t, command = %q", text, alert, reply)
					}
					if !setting.noPanel {
						response := contractPanelRequest(t, f, f.admin, http.MethodPatch, contractChanges(t, map[string]any{setting.key: true}))
						var result struct {
							Saved  []string          `json:"saved"`
							Failed map[string]string `json:"failed"`
						}
						if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
							t.Fatal(err)
						}
						if response.Code != http.StatusInternalServerError || len(result.Saved) != 0 || result.Failed[setting.key] == "" {
							t.Fatalf("panel failure = %d %s", response.Code, response.Body.String())
						}
					}
					assertContractRowCount(t, f, setting, 0)
				})
			}
			if setting.key == "captcha.enabled" {
				for _, surface := range []string{"command", "button", "panel"} {
					t.Run("release/"+surface, func(t *testing.T) {
						f := newSettingContractFixture(t, setting)
						if err := captcha.SetCaptchaEnabled(f.chat.Id, f.chat.Title, true); err != nil {
							t.Fatal(err)
						}
						assertContractRead(t, f, setting, true, "on", true)
						for _, userID := range []int64{f.member.Id, f.member.Id + 1} {
							if err := captcha.CreateMutedUser(userID, f.chat.Id, time.Now().Add(time.Hour)); err != nil {
								t.Fatal(err)
							}
						}
						attempt, err := captcha.CreateCaptchaAttemptPreMessage(f.member.Id, f.chat.Id, "7", 2)
						if err != nil {
							t.Fatal(err)
						}
						if err := captcha.UpdateCaptchaAttemptMessageID(attempt.ID, 123); err != nil {
							t.Fatal(err)
						}
						if err := captcha.StoreMessageForCaptcha(f.member.Id, f.chat.Id, attempt.ID, db.TEXT, "pending", "", ""); err != nil {
							t.Fatal(err)
						}
						switch surface {
						case "command":
							contractDispatch(t, f, f.admin, "/captcha off")
						case "button":
							contractPress(t, f, f.admin, "cp", "off")
						case "panel":
							response := contractPanelRequest(t, f, f.admin, http.MethodPatch, contractChanges(t, map[string]any{setting.key: false}))
							if response.Code != http.StatusOK {
								t.Fatalf("disable = %d %s", response.Code, response.Body.String())
							}
						}
						calls := f.client.callsFor("restrictChatMember")
						if len(calls) != 2 {
							t.Fatalf("restored members = %d, want both held members", len(calls))
						}
						for _, call := range calls {
							permissions, ok := call.Params["permissions"].(gotgbot.ChatPermissions)
							if !ok {
								t.Fatalf("permissions type = %T", call.Params["permissions"])
							}
							if !permissions.CanSendMessages {
								t.Fatalf("held member was not unmuted: %+v", call)
							}
						}
						if current, err := captcha.GetCaptchaAttemptByID(attempt.ID); err != nil || current != nil {
							t.Fatalf("pending attempt = %+v, %v", current, err)
						}
						if count, err := captcha.CountStoredMessagesForAttempt(attempt.ID); err != nil || count != 0 {
							t.Fatalf("held messages = %d, %v", count, err)
						}
						if users, err := captcha.GetMutedUsersForChat(f.chat.Id); err != nil || len(users) != 0 {
							t.Fatalf("held users = %+v, %v", users, err)
						}
						assertContractRead(t, f, setting, false, "off", false)
					})
					t.Run("release failure/"+surface, func(t *testing.T) {
						f := newSettingContractFixture(t, setting)
						if err := captcha.SetCaptchaEnabled(f.chat.Id, f.chat.Title, true); err != nil {
							t.Fatal(err)
						}
						assertContractRead(t, f, setting, true, "on", true)
						if err := captcha.CreateMutedUser(f.member.Id, f.chat.Id, time.Now().Add(time.Hour)); err != nil {
							t.Fatal(err)
						}
						f.client.errors["restrictChatMember"] = errors.New("telegram refused the unmute")
						switch surface {
						case "command":
							contractDispatch(t, f, f.admin, "/captcha off")
							if reply := fmt.Sprint(lastCall(t, f.client, "sendMessage").Params["text"]); reply != translated(t, "captcha_disable_failed") {
								t.Fatalf("release failure reply = %q", reply)
							}
						case "button":
							contractPress(t, f, f.admin, "cp", "off")
							if text, alert := lastAnswer(t, f.client); !alert || text != translated(t, "captcha_disable_failed") {
								t.Fatalf("release failure answer = %q, alert %t", text, alert)
							}
						case "panel":
							response := contractPanelRequest(t, f, f.admin, http.MethodPatch, contractChanges(t, map[string]any{setting.key: false, "antiraid.raid_time": 60}))
							var result struct {
								Saved  []string          `json:"saved"`
								Failed map[string]string `json:"failed"`
							}
							if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
								t.Fatal(err)
							}
							if response.Code != http.StatusInternalServerError || slices.Contains(result.Saved, setting.key) || !slices.Contains(result.Saved, "antiraid.raid_time") || !strings.Contains(result.Failed[setting.key], "captcha was turned off, but pending users could not be unmuted") {
								t.Fatalf("release failure = %d %s", response.Code, response.Body.String())
							}
						}
						if held, err := captcha.GetMutedUsersForChat(f.chat.Id); err != nil || len(held) != 1 {
							t.Fatalf("failed release lost held member: %+v, %v", held, err)
						}
						assertContractRead(t, f, setting, false, "off", false)
						assertContractRowCount(t, f, setting, 1)
					})
					t.Run("release not attached/"+surface, func(t *testing.T) {
						f := newSettingContractFixture(t, setting)
						handler := panel.New(f.bot, panel.Deps{})
						settings.AttachCaptchaRelease(nil)
						t.Cleanup(func() { settings.AttachCaptchaRelease(disableCaptchaForChat) })
						switch surface {
						case "command":
							contractDispatch(t, f, f.admin, "/captcha off")
							if reply := fmt.Sprint(lastCall(t, f.client, "sendMessage").Params["text"]); reply != translated(t, "captcha_disable_failed") {
								t.Fatalf("unbound write reply = %q", reply)
							}
						case "button":
							contractPress(t, f, f.admin, "cp", "off")
							if text, alert := lastAnswer(t, f.client); !alert || text != translated(t, "captcha_disable_failed") {
								t.Fatalf("unbound write answer = %q, alert %t", text, alert)
							}
						case "panel":
							response := contractPanelRequestTo(t, f, f.admin, http.MethodPatch, contractChanges(t, map[string]any{setting.key: false, "antiraid.raid_time": 60}), handler)
							if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "captcha release is not attached") {
								t.Fatalf("unbound batch = %d %s", response.Code, response.Body.String())
							}
							assertContractRowCount(t, f, settingContract{key: "antiraid.raid_time"}, 0)
						}
						assertContractRowCount(t, f, setting, 0)
						if len(f.client.callsFor("restrictChatMember")) != 0 {
							t.Fatal("unbound write changed member permissions")
						}
					})
				}
				t.Run("panel requires release at construction", func(t *testing.T) {
					settings.AttachCaptchaRelease(nil)
					t.Cleanup(func() { settings.AttachCaptchaRelease(disableCaptchaForChat) })
					defer func() {
						if failure := recover(); failure == nil || !strings.Contains(fmt.Sprint(failure), "captcha release is not attached") {
							t.Fatalf("construction failure = %v", failure)
						}
					}()
					panel.New(newModuleTestBot(newModuleBotClient()), panel.Deps{})
				})
			}
		})
	}
	t.Run("night mode uses startup lock types and one shared write", func(t *testing.T) {
		f := newSettingContractFixture(t, settingContract{})
		name := fmt.Sprintf("contract-night-once-%d", f.chat.Id)
		written := false
		if err := db.DB.Callback().Create().Before("gorm:create").Register(name, func(tx *gorm.DB) {
			if tx.Statement.Table == "night_mode_settings" {
				if written {
					_ = tx.AddError(errors.New("night mode was persisted more than once"))
				}
				written = true
			}
		}); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.DB.Callback().Create().Remove(name) })
		response := contractPanelRequest(t, f, f.admin, http.MethodPatch, contractChanges(t, map[string]any{
			"nightmode.enabled": true, "nightmode.start": "23:00", "nightmode.end": "7:00", "nightmode.lock_types": " URL,media, MEDIA ",
		}))
		if response.Code != http.StatusOK {
			t.Fatalf("night mode batch = %d %s", response.Code, response.Body.String())
		}
		var result struct {
			Saved []string `json:"saved"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || len(result.Saved) != 4 {
			t.Fatalf("night outcomes = %+v, %v", result, err)
		}
		for _, field := range []settingContract{
			{key: "nightmode.enabled", command: "/nightmode", defaultValue: false},
			{key: "nightmode.start", command: "/nightmode", defaultValue: "00:00"},
			{key: "nightmode.end", command: "/nightmode", defaultValue: "00:00"},
			{key: "nightmode.lock_types", command: "/nightmode", defaultValue: ""},
		} {
			value := map[string]any{"nightmode.enabled": true, "nightmode.start": "23:00", "nightmode.end": "07:00", "nightmode.lock_types": "media,url"}[field.key]
			assertContractRead(t, f, field, value, "23:00-07:00 media, url", true)
		}
	})
	t.Run("night mode requires startup lock types", func(t *testing.T) {
		f := newSettingContractFixture(t, settingContract{})
		handler := panel.New(f.bot, panel.Deps{})
		types := slices.DeleteFunc(slices.Clone(locksModule.getLockMapAsArray()), func(value string) bool { return value == "bots" })
		settings.AttachNightModeLockTypes(nil)
		t.Cleanup(func() { settings.AttachNightModeLockTypes(types) })
		contractDispatch(t, f, f.admin, "/nightmode off")
		if reply := fmt.Sprint(lastCall(t, f.client, "sendMessage").Params["text"]); reply != translated(t, "common_settings_save_failed") {
			t.Fatalf("unattached night mode command = %q", reply)
		}
		response := contractPanelRequestTo(t, f, f.admin, http.MethodPatch, contractChanges(t, map[string]any{
			"nightmode.enabled": false, "antiraid.raid_time": 60,
		}), handler)
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "night mode lock types are not attached") {
			t.Fatalf("unattached night mode panel = %d %s", response.Code, response.Body.String())
		}
		assertContractRowCount(t, f, settingContract{key: "nightmode.enabled"}, 0)
		assertContractRowCount(t, f, settingContract{key: "antiraid.raid_time"}, 0)
		defer func() {
			if failure := recover(); failure == nil || !strings.Contains(fmt.Sprint(failure), "night mode lock types are not attached") {
				t.Fatalf("unattached startup = %v", failure)
			}
		}()
		panel.New(f.bot, panel.Deps{})
	})
	t.Run("night mode preserves disabled windows and refuses disabled edits", func(t *testing.T) {
		f := newSettingContractFixture(t, settingContract{})
		contractDispatch(t, f, f.admin, "/nightmode 23:00 07:00 media,url")
		contractDispatch(t, f, f.admin, "/nightmode off")
		for _, key := range []string{"nightmode.start", "nightmode.end", "nightmode.lock_types"} {
			value := map[string]any{"nightmode.start": "22:00", "nightmode.end": "08:00", "nightmode.lock_types": "photo"}[key]
			response := contractPanelRequest(t, f, f.admin, http.MethodPatch, contractChanges(t, map[string]any{key: value, "antiraid.raid_time": 60}))
			if response.Code != http.StatusBadRequest {
				t.Fatalf("disabled edit = %d %s", response.Code, response.Body.String())
			}
			assertContractRowCount(t, f, settingContract{key: "antiraid.raid_time"}, 0)
		}
		row, err := nightmode.Get(f.chat.Id)
		if err != nil || row.Enabled || row.StartMinute != 1380 || row.EndMinute != 420 || row.LockTypes != "media,url" {
			t.Fatalf("disabled window = %+v, %v", row, err)
		}
		response := contractPanelRequest(t, f, f.admin, http.MethodPatch, contractChanges(t, map[string]any{"nightmode.enabled": true}))
		if response.Code != http.StatusOK {
			t.Fatalf("re-enable stored window = %d %s", response.Code, response.Body.String())
		}
	})
	t.Run("night mode shared failure leaves other fields saved", func(t *testing.T) {
		f := newSettingContractFixture(t, settingContract{})
		name := fmt.Sprintf("contract-night-failure-%d", f.chat.Id)
		if err := db.DB.Callback().Create().Before("gorm:create").Register(name, func(tx *gorm.DB) {
			if tx.Statement.Table == "night_mode_settings" {
				_ = tx.AddError(errors.New("night mode save failed"))
			}
		}); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.DB.Callback().Create().Remove(name) })
		response := contractPanelRequest(t, f, f.admin, http.MethodPatch, contractChanges(t, map[string]any{
			"nightmode.enabled": true, "nightmode.start": "23:00", "nightmode.end": "07:00", "nightmode.lock_types": "media,url",
			"antiraid.raid_time": 60,
		}))
		var result struct {
			Saved  []string          `json:"saved"`
			Failed map[string]string `json:"failed"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if response.Code != http.StatusInternalServerError || !slices.Equal(result.Saved, []string{"antiraid.raid_time"}) || len(result.Failed) != 4 {
			t.Fatalf("shared failure = %d %s", response.Code, response.Body.String())
		}
		for _, key := range []string{"nightmode.enabled", "nightmode.start", "nightmode.end", "nightmode.lock_types"} {
			if result.Failed[key] != "night mode save failed" {
				t.Fatalf("%s failure = %q", key, result.Failed[key])
			}
		}
		assertContractRowCount(t, f, settingContract{key: "nightmode.enabled"}, 0)
		assertContractRead(t, f, settingContract{key: "antiraid.raid_time", command: "/raidtime", defaultValue: 21600, statusToken: "Raid: "}, 60, "1m", true)
	})
}

func TestAntiRaidRepairsStoredTimesThroughDispatcher(t *testing.T) {
	useRealLocales(t)
	withMiniredis(t)
	for _, surface := range []string{"command", "settings button", "status button", "automatic"} {
		t.Run(surface, func(t *testing.T) {
			f := newAdminChatFixture(t)
			if err := antiraid.SetRaidTime(f.chat.Id, 60); err != nil {
				t.Fatal(err)
			}
			if err := antiraid.SetRaidTime(f.chat.Id, 0); err != nil {
				t.Fatal(err)
			}
			if err := antiraid.SetRaidActionTime(f.chat.Id, 5); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _, _ = antiRaidModule.disableRaid(f.chat.Id) })
			dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
			LoadAntiRaid(dispatcher)
			LoadConnections(dispatcher)
			started := time.Now().Unix()
			switch surface {
			case "command":
				contractDispatch(t, f, f.admin, "/antiraid on")
				if reply := fmt.Sprint(lastCall(t, f.client, "sendMessage").Params["text"]); reply != "AntiRaid enabled for 6h." {
					t.Fatalf("enable reply = %q", reply)
				}
			case "settings button", "status button":
				namespace, fields := "settings", map[string]string{"a": "t", "c": fmt.Sprint(f.chat.Id), "k": "ar", "v": "on"}
				if surface == "status button" {
					namespace, fields = "antiraid", map[string]string{"a": "on", "c": fmt.Sprint(f.chat.Id)}
				}
				update := newModuleCallbackContext(f.bot, privateChatOf(f.admin), f.admin, encodeCallbackData(namespace, fields)).Update
				if err := dispatcher.ProcessUpdate(f.bot, update, nil); err != nil {
					t.Fatal(err)
				}
			case "automatic":
				if err := antiraid.SetAutoAntiRaidThreshold(f.chat.Id, 1); err != nil {
					t.Fatal(err)
				}
			}
			newcomer := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Newcomer"}
			before := time.Now().Unix()
			update := &gotgbot.Update{UpdateId: 3, Message: &gotgbot.Message{
				MessageId: 103, Date: before, Chat: f.chat, From: &newcomer, NewChatMembers: []gotgbot.User{newcomer},
			}}
			if err := dispatcher.ProcessUpdate(f.bot, update, nil); err != nil {
				t.Fatal(err)
			}
			ban := f.client.onlyCall(t, "banChatMember")
			until, err := strconv.ParseInt(fmt.Sprint(ban.Params["until_date"]), 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			if until < before+3600 || until > time.Now().Unix()+3600 {
				t.Fatalf("ban until = %d, want the default hour after %d", until, before)
			}
			assertParams(t, ban, map[string]any{"chat_id": f.chat.Id, "user_id": newcomer.Id})
			contractDispatch(t, f, f.admin, "/antiraid")
			reply := fmt.Sprint(lastCall(t, f.client, "sendMessage").Params["text"])
			if !strings.Contains(reply, "AntiRaid ACTIVE") || !strings.Contains(reply, "Raid: 6h | Action: 1h") {
				t.Fatalf("repaired raid status = %q", reply)
			}
			var expiry string
			if _, err := fmt.Sscanf(reply, "AntiRaid ACTIVE | Expires: %s", &expiry); err != nil {
				t.Fatal(err)
			}
			seconds, ok := parseDuration(expiry)
			if !ok || seconds > 21600 || seconds < 21600-int(time.Now().Unix()-started)-1 {
				t.Fatalf("raid expires in %q, want default six hours", expiry)
			}
		})
	}
}

func newSettingContractFixture(t *testing.T, setting settingContract) adminChatFixture {
	t.Helper()
	f := newAdminChatFixture(t)
	contractRights(f, setting.changeInfo, true, setting.restrict)
	if setting.owner {
		contractMakeOwner(f)
	}
	if err := chats.EnsureChatInDb(f.chat.Id, f.chat.Title); err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(setting.key, "digest.") {
		if err := logchannels.Set(f.chat.Id, f.chat.Title, -10099990001); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func contractRights(f adminChatFixture, changeInfo, botAdmin, restrict bool) {
	admin := fmt.Sprintf(`{"status":"administrator","user":{"id":%d,"first_name":"Admin"},"can_change_info":%t,"can_restrict_members":%t}`, f.admin.Id, changeInfo, restrict)
	bot := fmt.Sprintf(`{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"},"can_restrict_members":%t}`, restrict)
	if !botAdmin {
		bot = `{"status":"member","user":{"id":999,"is_bot":true,"first_name":"Alita"}}`
	}
	f.client.membersByUser[fmt.Sprint(f.admin.Id)] = json.RawMessage(admin)
	f.client.membersByChat = map[string]json.RawMessage{fmt.Sprintf("%d|999", f.chat.Id): json.RawMessage(bot)}
	f.client.adminsByChat[fmt.Sprint(f.chat.Id)] = json.RawMessage("[" + admin + "]")
	if botAdmin {
		f.client.adminsByChat[fmt.Sprint(f.chat.Id)] = json.RawMessage("[" + bot + "," + admin + "]")
	}
}

func contractPress(t *testing.T, f adminChatFixture, actor gotgbot.User, key, value string) {
	t.Helper()
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadConnections(dispatcher)
	data := encodeCallbackData("settings", map[string]string{"a": "t", "c": fmt.Sprint(f.chat.Id), "k": key, "v": value})
	if err := dispatcher.ProcessUpdate(f.bot, newModuleCallbackContext(f.bot, privateChatOf(actor), actor, data).Update, nil); err != nil {
		t.Fatal(err)
	}
}

func contractDispatch(t *testing.T, f adminChatFixture, actor gotgbot.User, text string) {
	t.Helper()
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadAntiRaid(dispatcher)
	LoadGreetings(dispatcher)
	LoadCaptcha(dispatcher)
	LoadAntiflood(dispatcher)
	LoadWarns(dispatcher)
	LoadProbation(dispatcher)
	LoadSpamDB(dispatcher)
	LoadAISpam(dispatcher)
	LoadConnections(dispatcher)
	LoadReports(dispatcher)
	LoadRules(dispatcher)
	LoadDigest(dispatcher)
	loadTimezone(dispatcher)
	loadNightMode(dispatcher)
	LoadLanguage(dispatcher)
	LoadNotes(dispatcher)
	LoadPin(dispatcher)
	LoadAdmin(dispatcher)
	if err := dispatcher.ProcessUpdate(f.bot, newModuleMessageContext(f.bot, f.chat, actor, text).Update, nil); err != nil {
		t.Fatal(err)
	}
}

func contractSet(t *testing.T, f adminChatFixture, actor gotgbot.User, setting settingContract, input string) string {
	t.Helper()
	if setting.key != "language.code" {
		before := len(f.client.callsFor("sendMessage"))
		contractDispatch(t, f, actor, setting.command+" "+input)
		if len(f.client.callsFor("sendMessage")) == before {
			return ""
		}
		return fmt.Sprint(lastCall(t, f.client, "sendMessage").Params["text"])
	}
	contractDispatch(t, f, actor, "/lang group")
	if actor.Id != f.admin.Id {
		return fmt.Sprint(lastCall(t, f.client, "sendMessage").Params["text"])
	}
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadLanguage(dispatcher)
	data := encodeCallbackData("change_language", map[string]string{"l": input})
	if err := dispatcher.ProcessUpdate(f.bot, newModuleCallbackContext(f.bot, f.chat, actor, data).Update, nil); err != nil {
		t.Fatal(err)
	}
	text, _ := lastAnswer(t, f.client)
	return text
}

func isContractRemaining(setting settingContract) bool {
	for _, prefix := range []string{"reports.", "rules.", "digest.", "timezone.", "language.", "nightmode."} {
		if strings.HasPrefix(setting.key, prefix) {
			return true
		}
	}
	return false
}

func testContractDigestPreconditions(t *testing.T, setting settingContract) {
	t.Helper()
	for _, edge := range setting.edges {
		if !edge.accept {
			continue
		}
		t.Run("operator off/"+edge.input, func(t *testing.T) {
			f := newSettingContractFixture(t, setting)
			config.AppConfig.EnableDigest = false
			t.Cleanup(func() { config.AppConfig.EnableDigest = true })
			reply := contractSet(t, f, f.admin, setting, edge.input)
			if reply != translated(t, "digest_globally_disabled") {
				t.Fatalf("digest operator refusal = %q", reply)
			}
			response := contractPanelRequest(t, f, f.admin, http.MethodPatch, contractChanges(t, map[string]any{setting.key: edge.value, "antiraid.raid_time": 60}))
			var result struct {
				Errors map[string]string `json:"errors"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if response.Code != http.StatusBadRequest || result.Errors[setting.key] != "❌ The digest is disabled by the bot operator." {
				t.Fatalf("digest panel refusal = %d %s", response.Code, response.Body.String())
			}
			assertContractRowCount(t, f, setting, 0)
			assertContractRowCount(t, f, settingContract{key: "antiraid.raid_time"}, 0)
		})
	}
	if setting.key == "digest.destination" {
		t.Run("log needs a destination", func(t *testing.T) {
			f := newSettingContractFixture(t, settingContract{})
			reply := contractSet(t, f, f.admin, setting, "log")
			if reply != translated(t, "digest_log_unavailable") {
				t.Fatalf("digest command log refusal = %q", reply)
			}
			response := contractPanelRequest(t, f, f.admin, http.MethodPatch, contractChanges(t, map[string]any{setting.key: "log", "antiraid.raid_time": 60}))
			if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "This chat has no log channel") {
				t.Fatalf("digest panel log refusal = %d %s", response.Code, response.Body.String())
			}
			assertContractRowCount(t, f, setting, 0)
			assertContractRowCount(t, f, settingContract{key: "antiraid.raid_time"}, 0)
		})
	}
}

func contractRemainingStatus(t *testing.T, f adminChatFixture, setting settingContract, value any, label, text string) bool {
	t.Helper()
	switch setting.key {
	case "reports.mute_threshold":
		want := translated(t, "reports_automute_status_off")
		if value != 0 {
			want = translatedWith(t, "reports_automute_status_on", i18n.TranslationParams{"count": value})
		}
		return strings.Contains(text, want)
	case "rules.private_rules":
		key := "rules_private_current_group"
		if value == true {
			key = "rules_private_current_pm"
		}
		return text == fmt.Sprintf(translated(t, key), f.chat.Title) || text == translated(t, key)
	case "digest.enabled":
		key := "digest_status_disabled"
		if value == true {
			key = "digest_status_enabled"
		}
		return strings.Contains(text, translated(t, key))
	case "digest.destination":
		return strings.Contains(text, "Destination: <b>"+label+"</b>") && strings.Contains(text, "(the default)") == (value == "")
	case "timezone.name":
		return strings.Contains(text, "timezone is <b>"+fmt.Sprint(value)+"</b>")
	case "language.code":
		tr := i18n.MustNewTranslator(value.(string))
		want, err := tr.GetString("language_current_group")
		if err != nil {
			t.Fatal(err)
		}
		return text == fmt.Sprintf(want, keyboard.GetLangFormat(value.(string)))
	default:
		if strings.HasPrefix(setting.key, "nightmode.") {
			if label == "off" {
				return text == translated(t, "nightmode_status_off")
			}
			window, types, _ := strings.Cut(label, " ")
			start, end, _ := strings.Cut(window, "-")
			return strings.Contains(text, "from <b>"+start+"</b> to <b>"+end+"</b>") && strings.Contains(text, "<code>"+types+"</code>")
		}
	}
	return false
}

func assertContractRead(t *testing.T, f adminChatFixture, setting settingContract, value any, label string, changed bool) {
	t.Helper()
	command := "/antiraid"
	if strings.HasPrefix(setting.key, "captcha.") {
		command = "/captcha"
	} else if strings.HasPrefix(setting.key, "warns.") {
		command = "/warnings"
	} else if strings.HasPrefix(setting.key, "antiflood.") && setting.key != "antiflood.delete_messages" {
		command = "/flood"
	} else if strings.HasPrefix(setting.key, "antispam.probation_") {
		command = "/probation"
	} else if strings.HasPrefix(setting.key, "antispam.spamdb_") {
		command = "/spamdb"
	} else if setting.button != "" {
		command = setting.command
	}
	if isContractRemaining(setting) {
		command = setting.command
		if strings.HasPrefix(setting.key, "nightmode.") {
			command = "/nightmode"
		}
	}
	before := len(f.client.callsFor("sendMessage"))
	contractDispatch(t, f, f.admin, command)
	replies := f.client.callsFor("sendMessage")[before:]
	found := false
	for _, reply := range replies {
		text := fmt.Sprint(reply.Params["text"])
		if setting.statusOn != "" {
			key := setting.statusOff
			if value == true {
				key = setting.statusOn
			}
			found = found || strings.HasPrefix(text, contractSwitchReply(t, f, key))
			continue
		}
		if isContractRemaining(setting) {
			found = found || contractRemainingStatus(t, f, setting, value, label, text)
			continue
		}
		if strings.HasPrefix(setting.key, "captcha.") {
			if enabled, ok := value.(bool); ok {
				label = "disabled"
				if enabled {
					label = "enabled"
				}
			}
		}
		if strings.HasPrefix(setting.key, "antiflood.") || strings.HasPrefix(setting.key, "warns.") {
			found = found || strings.Contains(text, label) || strings.Contains(text, "not currently enforcing") || (isContractModeDuration(setting) && strings.Contains(text, lastContractWord(label))) || (value == 0 && strings.Contains(text, "never"))
		} else if strings.Contains(text, setting.statusToken+fmt.Sprint(value)) || strings.Contains(text, setting.statusToken+label) {
			found = true
		}
	}
	if !found && setting.key != "antispam.spamdb_action" {
		t.Fatalf("status replies = %v, want %s%v", replies, setting.statusToken, value)
	}
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadConnections(dispatcher)
	data := encodeCallbackData("settings", map[string]string{"a": "r", "c": fmt.Sprint(f.chat.Id)})
	if err := dispatcher.ProcessUpdate(f.bot, newModuleCallbackContext(f.bot, privateChatOf(f.admin), f.admin, data).Update, nil); err != nil {
		t.Fatal(err)
	}
	summaryLabel := label
	if enabled, ok := value.(bool); ok && !strings.Contains(setting.key, "probation_block_") {
		summaryLabel = "off"
		if enabled {
			summaryLabel = "on"
		}
	}
	summaryChanged := changed
	if isContractModeDuration(setting) {
		summaryChanged = label != "mute"
	}
	if strings.HasPrefix(setting.key, "nightmode.") {
		summaryLabel, summaryChanged = label, label != "off"
	}
	if !strings.HasPrefix(setting.key, "antispam.spamdb_") && !strings.HasPrefix(setting.key, "digest.") {
		assertContractSummary(t, fmt.Sprint(lastCall(t, f.client, "editMessageText").Params["text"]), setting.command, summaryLabel, summaryChanged)
	}
	if setting.button != "" {
		var values []string
		for _, edge := range setting.edges {
			if edge.accept {
				values = append(values, edge.input)
			}
		}
		next := values[(slices.Index(values, summaryLabel)+1)%len(values)]
		if actual := toggleButtons(t, markupOf(t, lastCall(t, f.client, "editMessageText")))[setting.button]; actual != next {
			t.Fatalf("button offers %q, want %q", actual, next)
		}
	}
	if setting.noPanel {
		return
	}
	response := contractPanelRequest(t, f, f.admin, http.MethodGet, "")
	var body struct {
		Modules []struct {
			Key    string `json:"key"`
			Fields []struct {
				Key     string          `json:"key"`
				Value   json.RawMessage `json:"value"`
				Default json.RawMessage `json:"default"`
				Changed bool            `json:"changed"`
			} `json:"fields"`
		} `json:"modules"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	for _, module := range body.Modules {
		for _, field := range module.Fields {
			if module.Key+"."+field.Key != setting.key {
				continue
			}
			def := setting.defaultValue
			if isContractModeDuration(setting) {
				def = 3600
				changed = value != 0 && value != 3600
			}
			if string(field.Value) != contractJSON(t, value) || string(field.Default) != contractJSON(t, def) || field.Changed != changed {
				t.Fatalf("panel field = %+v, want value %v default %v changed %t", field, value, def, changed)
			}
			return
		}
	}
	t.Fatalf("panel missing %s: %d %s", setting.key, response.Code, response.Body.String())
}

func assertContractSummary(t *testing.T, text, command, label string, changed bool) {
	t.Helper()
	line := "<code>" + command + "</code> — " + label
	if changed {
		line = "• " + line
	}
	for _, actual := range strings.Split(text, "\n") {
		if actual == line {
			return
		}
	}
	t.Fatalf("summary missing line %q: %s", line, text)
}

func contractJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func contractChanges(t *testing.T, changes map[string]any) string {
	t.Helper()
	return contractJSON(t, map[string]any{"changes": changes})
}

func assertContractRowCount(t *testing.T, f adminChatFixture, setting settingContract, want int64) {
	t.Helper()
	if setting.key == "language.code" {
		return // The panel fixture already needs an existing chat row.
	}
	var model any = &models.AntiRaidSettings{}
	if strings.HasPrefix(setting.key, "greetings.") {
		model = &models.GreetingSettings{}
	} else if strings.HasPrefix(setting.key, "captcha.") {
		model = &models.CaptchaSettings{}
	} else if strings.HasPrefix(setting.key, "antiflood.") {
		model = &models.AntifloodSettings{}
	} else if strings.HasPrefix(setting.key, "warns.") {
		model = &models.WarnSettings{}
	} else if strings.HasPrefix(setting.key, "antispam.probation_") {
		model = &models.ProbationSettings{}
	} else if strings.HasPrefix(setting.key, "antispam.spamdb_") {
		model = &models.SpamDBSettings{}
	} else if setting.key == "antispam.aispam_enabled" {
		model = &models.AISpamSettings{}
	} else if strings.HasPrefix(setting.key, "reports.") {
		model = &models.ReportChatSettings{}
	} else if strings.HasPrefix(setting.key, "rules.") {
		model = &models.RulesSettings{}
	} else if strings.HasPrefix(setting.key, "digest.") {
		model = &models.DigestSettings{}
	} else if strings.HasPrefix(setting.key, "timezone.") {
		model = &models.ChatTimezone{}
	} else if strings.HasPrefix(setting.key, "nightmode.") {
		model = &models.NightModeSettings{}
	} else if strings.HasPrefix(setting.key, "notes.") {
		model = &models.NotesSettings{}
	} else if strings.HasPrefix(setting.key, "pins.") {
		model = &models.PinSettings{}
	} else if strings.HasPrefix(setting.key, "connections.") {
		model = &models.ConnectionChatSettings{}
	} else if strings.HasPrefix(setting.key, "admin.") {
		model = &models.AdminSettings{}
	}
	var count int64
	if err := db.DB.Model(model).Where("chat_id = ?", f.chat.Id).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("settings rows = %d, want %d", count, want)
	}
}

func isContractModeDuration(setting settingContract) bool {
	return setting.key == "antiflood.action_time" || setting.key == "warns.action_time"
}

func lastContractWord(label string) string {
	words := strings.Fields(label)
	return words[len(words)-1]
}

func contractSettingChanges(setting settingContract, edge settingContractEdge) map[string]any {
	changes := map[string]any{setting.key: edge.value}
	if strings.HasPrefix(setting.key, "nightmode.") && edge.input != "off" && setting.key != "nightmode.enabled" {
		changes["nightmode.enabled"] = true
		changes["nightmode.start"] = "23:00"
		changes["nightmode.end"] = "07:00"
		changes["nightmode.lock_types"] = "media,url"
		changes[setting.key] = edge.value
	}
	if setting.key == "nightmode.enabled" && edge.value == true {
		changes["nightmode.start"], changes["nightmode.end"], changes["nightmode.lock_types"] = "23:00", "07:00", "media,url"
	}
	if isContractModeDuration(setting) {
		key := "antiflood.action"
		if strings.HasPrefix(setting.key, "warns.") {
			key = "warns.mode"
		}
		changes[key] = strings.Fields(edge.input)[0]
	}
	return changes
}

func contractPanelRequest(t *testing.T, f adminChatFixture, actor gotgbot.User, method, body string) *httptest.ResponseRecorder {
	t.Helper()
	return contractPanelRequestTo(t, f, actor, method, body, panel.New(f.bot, panel.Deps{}))
}

func contractPanelRequestTo(t *testing.T, f adminChatFixture, actor gotgbot.User, method, body string, handler http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	now := time.Now()
	user, err := json.Marshal(actor)
	if err != nil {
		t.Fatal(err)
	}
	values := url.Values{"auth_date": {strconv.FormatInt(now.Unix(), 10)}, "user": {string(user)}}
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	_, _ = secret.Write([]byte(f.bot.Token))
	signature := hmac.New(sha256.New, secret.Sum(nil))
	_, _ = signature.Write([]byte("auth_date=" + values.Get("auth_date") + "\nuser=" + values.Get("user")))
	values.Set("hash", hex.EncodeToString(signature.Sum(nil)))
	request := httptest.NewRequest(method, fmt.Sprintf("/panel/api/v1/chats/%d/settings", f.chat.Id), strings.NewReader(body))
	request.Header.Set("X-Telegram-Init-Data", values.Encode())
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

// NULL passes the schema's CHECK constraints but decodes as an invalid zero
// value, representing old stored settings without weakening the test schema.
func invalidCaptchaStoredValue(field string) func(int64) error {
	return func(chatID int64) error {
		return db.DB.Model(&models.CaptchaSettings{}).Create(map[string]any{"chat_id": chatID, field: nil}).Error
	}
}

func testContractModeRelationship(t *testing.T, setting settingContract) {
	t.Helper()
	module, _, _ := strings.Cut(setting.key, ".")
	timeKey := module + ".action_time"
	for _, tc := range []struct {
		name    string
		mode    string
		seconds any
		accept  bool
	}{
		{"untimed duration", "ban", 60, false},
		{"untimed zero", "ban", 0, true},
		{"untimed omitted", "ban", nil, true},
		{"timed missing", "tban", nil, false},
		{"timed zero", "tmute", 0, false},
		{"timed minute", "tmute", 60, true},
	} {
		t.Run("relationship/"+tc.name, func(t *testing.T) {
			f := newSettingContractFixture(t, setting)
			changes := map[string]any{setting.key: tc.mode}
			if tc.seconds != nil {
				changes[timeKey] = tc.seconds
			}
			response := contractPanelRequest(t, f, f.admin, http.MethodPatch, contractChanges(t, changes))
			if !tc.accept {
				if response.Code != http.StatusBadRequest {
					t.Fatalf("relationship accepted: %d %s", response.Code, response.Body.String())
				}
				assertContractRowCount(t, f, setting, 0)
				return
			}
			if response.Code != http.StatusOK {
				t.Fatalf("relationship refused: %d %s", response.Code, response.Body.String())
			}
			label := tc.mode
			if tc.seconds == 60 {
				label += " 1m"
			}
			assertContractRead(t, f, setting, tc.mode, label, true)
		})
	}
	t.Run("untimed refuses unused command duration", func(t *testing.T) {
		f := newSettingContractFixture(t, setting)
		contractDispatch(t, f, f.admin, setting.command+" ban 1m")
		assertContractRowCount(t, f, setting, 0)
		assertContractRead(t, f, setting, "mute", "mute", false)
		contractDispatch(t, f, f.admin, setting.command+" ban 0")
		assertContractRead(t, f, setting, "ban", "ban", true)
	})
	t.Run("untimed duration only", func(t *testing.T) {
		f := newSettingContractFixture(t, setting)
		response := contractPanelRequest(t, f, f.admin, http.MethodPatch, contractChanges(t, map[string]any{timeKey: 60, "antiraid.raid_time": 60}))
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), timeKey) {
			t.Fatalf("unused duration = %d %s", response.Code, response.Body.String())
		}
		assertContractRowCount(t, f, setting, 0)
		assertContractRowCount(t, f, settingContract{key: "antiraid.raid_time"}, 0)
		response = contractPanelRequest(t, f, f.admin, http.MethodPatch, contractChanges(t, map[string]any{timeKey: 0}))
		if response.Code != http.StatusOK {
			t.Fatalf("inactive duration = %d %s", response.Code, response.Body.String())
		}
		assertContractRead(t, f, setting, "mute", "mute", false)
	})
	t.Run("timed invalid stored duration uses default without repairing read", func(t *testing.T) {
		f := newSettingContractFixture(t, setting)
		if module == "antiflood" {
			if err := antiflood.SetFloodMode(f.chat.Id, "tmute", 5); err != nil {
				t.Fatal(err)
			}
		} else if err := warns.SetWarnMode(f.chat.Id, "tmute", 5); err != nil {
			t.Fatal(err)
		}
		assertContractRead(t, f, setting, "tmute", "tmute 1h", true)
		var seconds int
		if module == "antiflood" {
			seconds = antiflood.GetFlood(f.chat.Id).FloodActionTime
		} else {
			seconds = warns.GetWarnSetting(f.chat.Id).WarnActionTime
		}
		if seconds != 5 {
			t.Fatalf("read repaired cached/stored duration to %d", seconds)
		}
		response := contractPanelRequest(t, f, f.admin, http.MethodPatch, contractChanges(t, map[string]any{setting.key: "tban"}))
		if response.Code != http.StatusOK {
			t.Fatalf("normalized duration refused: %d %s", response.Code, response.Body.String())
		}
		assertContractRead(t, f, setting, "tban", "tban 1h", true)
	})
	for _, surface := range []string{"command", "panel", "button"} {
		if surface == "button" && setting.button == "" {
			continue
		}
		t.Run("untimed clears/"+surface, func(t *testing.T) {
			f := newSettingContractFixture(t, setting)
			contractDispatch(t, f, f.admin, setting.command+" tmute 1h")
			switch surface {
			case "command":
				contractDispatch(t, f, f.admin, setting.command+" ban")
			case "button":
				contractPress(t, f, f.admin, setting.button, "ban")
			default:
				response := contractPanelRequest(t, f, f.admin, http.MethodPatch, contractChanges(t, map[string]any{setting.key: "ban"}))
				if response.Code != http.StatusOK {
					t.Fatalf("untimed save = %d %s", response.Code, response.Body.String())
				}
			}
			assertContractRead(t, f, setting, "ban", "ban", true)
			var seconds int
			if module == "antiflood" {
				seconds = antiflood.GetFlood(f.chat.Id).FloodActionTime
			} else {
				seconds = warns.GetWarnSetting(f.chat.Id).WarnActionTime
			}
			if seconds != 0 {
				t.Fatalf("untimed duration persisted as %d", seconds)
			}
		})
		if surface == "button" {
			t.Run("button refuses timed modes", func(t *testing.T) {
				f := newSettingContractFixture(t, setting)
				for _, value := range []string{"tmute", "tban"} {
					contractPress(t, f, f.admin, setting.button, value)
					if text, _ := lastAnswer(t, f.client); text != translated(t, "common_callback_invalid_request") {
						t.Fatalf("timed button = %q", text)
					}
				}
				assertContractRowCount(t, f, setting, 0)
			})
		}
	}
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprintf("shared write/failure=%t", fail), func(t *testing.T) {
			f := newSettingContractFixture(t, setting)
			table := "antiflood_settings"
			if module == "warns" {
				table = "warns_settings"
			}
			written := false
			name := "contract_shared_mode_write"
			if err := db.DB.Callback().Create().Before("gorm:create").Register(name, func(tx *gorm.DB) {
				if tx.Statement.Table == table {
					if fail || written {
						_ = tx.AddError(errors.New("shared mode write refused"))
					}
					written = true
				}
			}); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = db.DB.Callback().Create().Remove(name) })
			response := contractPanelRequest(t, f, f.admin, http.MethodPatch, contractChanges(t, map[string]any{
				setting.key: "tban", timeKey: 7200, "antiraid.raid_time": 60,
			}))
			var result struct {
				Saved  []string          `json:"saved"`
				Failed map[string]string `json:"failed"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if !slices.Contains(result.Saved, "antiraid.raid_time") {
				t.Fatalf("independent field lost: %d %s", response.Code, response.Body.String())
			}
			for _, key := range []string{setting.key, timeKey} {
				if fail {
					if response.Code != http.StatusInternalServerError || slices.Contains(result.Saved, key) || result.Failed[key] != "shared mode write refused" {
						t.Fatalf("shared failure = %d %s", response.Code, response.Body.String())
					}
				} else if response.Code != http.StatusOK || !slices.Contains(result.Saved, key) || len(result.Failed) != 0 {
					t.Fatalf("shared success = %d %s", response.Code, response.Body.String())
				}
			}
			if fail {
				assertContractRowCount(t, f, setting, 0)
			} else {
				assertContractRead(t, f, setting, "tban", "tban 2h", true)
			}
		})
	}
}

func testContractProbationBlocks(t *testing.T, setting settingContract) {
	t.Helper()
	t.Run("block at least one kind before any write", func(t *testing.T) {
		f := newSettingContractFixture(t, setting)
		contractDispatch(t, f, f.admin, "/probation block")
		if reply := fmt.Sprint(lastCall(t, f.client, "sendMessage").Params["text"]); reply != translated(t, "probation_invalid_block") {
			t.Fatalf("empty block refusal = %q", reply)
		}
		response := contractPanelRequest(t, f, f.admin, http.MethodPatch, contractChanges(t, map[string]any{
			"antispam.probation_block_links": false, "antispam.probation_block_forwards": false,
			"antispam.probation_block_media": false, "antiraid.raid_time": 60,
		}))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("empty block batch = %d %s", response.Code, response.Body.String())
		}
		assertContractRowCount(t, f, setting, 0)
		assertContractRowCount(t, f, settingContract{key: "antiraid.raid_time"}, 0)
	})
	t.Run("block kinds swap together", func(t *testing.T) {
		f := newSettingContractFixture(t, setting)
		for _, changes := range []map[string]any{
			{"antispam.probation_block_links": false, "antispam.probation_block_forwards": false, "antispam.probation_block_media": true},
			{"antispam.probation_block_links": true, "antispam.probation_block_forwards": true, "antispam.probation_block_media": false},
		} {
			response := contractPanelRequest(t, f, f.admin, http.MethodPatch, contractChanges(t, changes))
			var result struct {
				Saved  []string          `json:"saved"`
				Failed map[string]string `json:"failed"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if response.Code != http.StatusOK || len(result.Saved) != 3 || len(result.Failed) != 0 {
				t.Fatalf("block swap = %d %s", response.Code, response.Body.String())
			}
			label := "media"
			if changes[setting.key] == true {
				label = "links, forwards"
			}
			assertContractRead(t, f, setting, changes[setting.key], label, changes[setting.key] != setting.defaultValue)
		}
	})
}

func testContractSpamPreconditions(t *testing.T, setting settingContract) {
	t.Helper()
	var reasons []string
	switch setting.key {
	case "antispam.aispam_enabled":
		reasons = []string{"aispam_no_key", "aispam_globally_disabled"}
	case "antispam.spamdb_enabled":
		reasons = []string{"spamdb_globally_disabled"}
	case "antispam.spamdb_shared":
		reasons = []string{"spamdb_db_shared_disabled"}
	}
	for _, reason := range reasons {
		t.Run("operator/"+reason, func(t *testing.T) {
			f := newSettingContractFixture(t, setting)
			previous := *config.AppConfig
			t.Cleanup(func() { *config.AppConfig = previous })
			switch reason {
			case "aispam_no_key":
				config.AppConfig.TypeSafeAPIKey = ""
				config.AppConfig.EnableAISpam = false // Missing key wins over the operator switch.
			case "aispam_globally_disabled":
				config.AppConfig.EnableAISpam = false
			case "spamdb_globally_disabled":
				config.AppConfig.EnableSpamDB = false
			case "spamdb_db_shared_disabled":
				config.AppConfig.EnableReputation = false
			}
			contractDispatch(t, f, f.admin, setting.command+" on")
			reply := fmt.Sprint(lastCall(t, f.client, "sendMessage").Params["text"])
			if reply != translated(t, reason) {
				t.Fatalf("operator refusal = %q, want %s", reply, reason)
			}
			if setting.button != "" {
				contractPress(t, f, f.admin, setting.button, "on")
				if text, alert := lastAnswer(t, f.client); !alert || text != reply {
					t.Fatalf("operator button refusal = %q alert %t", text, alert)
				}
			}
			response := contractPanelRequest(t, f, f.admin, http.MethodPatch, contractChanges(t, map[string]any{setting.key: true, "antiraid.raid_time": 60}))
			var result struct {
				Errors map[string]string `json:"errors"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if response.Code != http.StatusBadRequest || result.Errors[setting.key] != reply {
				t.Fatalf("operator batch = %d %s", response.Code, response.Body.String())
			}
			assertContractRowCount(t, f, setting, 0)
			assertContractRowCount(t, f, settingContract{key: "antiraid.raid_time"}, 0)
			seedEnabled := func() {
				var err error
				switch setting.key {
				case "antispam.aispam_enabled":
					err = aispam.SetAISpamEnabled(f.chat.Id, true)
				case "antispam.spamdb_enabled":
					err = spamdbsettings.SetSpamDBEnabled(f.chat.Id, true)
				case "antispam.spamdb_shared":
					err = spamdbsettings.SetSpamDBDatabase(f.chat.Id, "shared", true)
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			// Operator preconditions forbid enabling, never switching off.
			seedEnabled()
			contractDispatch(t, f, f.admin, setting.command+" off")
			assertContractRead(t, f, setting, false, "off", false)
			if setting.button != "" {
				seedEnabled()
				contractPress(t, f, f.admin, setting.button, "off")
				if text, alert := lastAnswer(t, f.client); alert {
					t.Fatalf("operator blocked disable button: %q", text)
				}
			}
			seedEnabled()
			response = contractPanelRequest(t, f, f.admin, http.MethodPatch, contractChanges(t, map[string]any{setting.key: false}))
			if response.Code != http.StatusOK {
				t.Fatalf("operator blocked disable panel: %d %s", response.Code, response.Body.String())
			}
			assertContractRead(t, f, setting, false, "off", false)
		})
	}
}

func testContractSpamDatabases(t *testing.T, setting settingContract) {
	t.Helper()
	name := strings.TrimPrefix(setting.command, "/spamdb db ")
	databaseName := map[string]string{"cas": "CAS", "lols": "lols.bot", "shared": "Shared bans"}[name]
	t.Run("last database refused", func(t *testing.T) {
		f := newSettingContractFixture(t, setting)
		if err := spamdbsettings.SetSpamDBDatabase(f.chat.Id, name, true); err != nil {
			t.Fatal(err)
		}
		if name != "cas" {
			if err := spamdbsettings.SetSpamDBDatabase(f.chat.Id, "cas", false); err != nil {
				t.Fatal(err)
			}
		}
		contractDispatch(t, f, f.admin, setting.command+" off")
		reply := fmt.Sprint(lastCall(t, f.client, "sendMessage").Params["text"])
		want := strings.ReplaceAll(translated(t, "spamdb_db_last"), "{database}", databaseName)
		if reply != want {
			t.Fatalf("last database reply = %q, want %q", reply, want)
		}
		response := contractPanelRequest(t, f, f.admin, http.MethodPatch, contractChanges(t, map[string]any{setting.key: false, "antiraid.raid_time": 60}))
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "at least one spam database must stay on") {
			t.Fatalf("last database batch = %d %s", response.Code, response.Body.String())
		}
		assertContractRowCount(t, f, settingContract{key: "antiraid.raid_time"}, 0)
		row, err := spamdbsettings.GetSpamDBSettingsContext(t.Context(), f.chat.Id)
		if err != nil || len(spamDBSources(row)) != 1 {
			t.Fatalf("last database changed: %+v, %v", row, err)
		}
	})
	if name == "cas" {
		return
	}
	t.Run("failed enable preserves last database and independent outcomes", func(t *testing.T) {
		f := newSettingContractFixture(t, setting)
		assertContractRead(t, f, setting, false, "off", false)
		callback := "fail_spam_database_enable"
		if err := db.DB.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
			if tx.Statement.Table == "spam_db_settings" {
				_ = tx.AddError(errors.New("database enable failed"))
			}
		}); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.DB.Callback().Create().Remove(callback) })
		response := contractPanelRequest(t, f, f.admin, http.MethodPatch, contractChanges(t, map[string]any{
			setting.key: true, "antispam.spamdb_cas": false, "antiraid.raid_time": 60,
		}))
		var result struct {
			Saved  []string          `json:"saved"`
			Failed map[string]string `json:"failed"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if response.Code != http.StatusInternalServerError || len(result.Saved) != 1 || result.Saved[0] != "antiraid.raid_time" ||
			result.Failed[setting.key] == "" || result.Failed["antispam.spamdb_cas"] != "at least one spam database must stay on" {
			t.Fatalf("failed enable = %d %s", response.Code, response.Body.String())
		}
		assertContractRead(t, f, setting, false, "off", false)
		assertContractRowCount(t, f, setting, 0)
	})
	for _, reversed := range []bool{false, true} {
		t.Run(fmt.Sprintf("switchover/reversed=%t", reversed), func(t *testing.T) {
			f := newSettingContractFixture(t, setting)
			for _, casOn := range []bool{false, true} {
				first := fmt.Sprintf(`"antispam.spamdb_cas":%t`, casOn)
				second := fmt.Sprintf(`%q:%t`, setting.key, !casOn)
				if reversed {
					first, second = second, first
				}
				response := contractPanelRequest(t, f, f.admin, http.MethodPatch, `{"changes":{`+first+`,`+second+`}}`)
				var result struct {
					Saved  []string          `json:"saved"`
					Failed map[string]string `json:"failed"`
				}
				if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				if response.Code != http.StatusOK || len(result.Saved) != 2 || len(result.Failed) != 0 {
					t.Fatalf("switchover = %d %s", response.Code, response.Body.String())
				}
				row, err := spamdbsettings.GetSpamDBSettingsContext(t.Context(), f.chat.Id)
				if err != nil || row.CheckCAS != casOn || (name == "lols" && row.CheckLols == casOn) || (name == "shared" && row.CheckShared == casOn) {
					t.Fatalf("switchover settings = %+v, %v", row, err)
				}
			}
		})
	}
}

func contractSwitchReply(t *testing.T, f adminChatFixture, key string) string {
	t.Helper()
	text := translated(t, key)
	if strings.Contains(text, "%s") {
		text = fmt.Sprintf(text, f.chat.Title)
	}
	return text
}

func contractMakeOwner(f adminChatFixture) {
	from := fmt.Sprintf(`"status":"administrator","user":{"id":%d,`, f.admin.Id)
	to := fmt.Sprintf(`"status":"creator","user":{"id":%d,`, f.admin.Id)
	f.client.membersByUser[fmt.Sprint(f.admin.Id)] = json.RawMessage(strings.ReplaceAll(string(f.client.membersByUser[fmt.Sprint(f.admin.Id)]), from, to))
	f.client.adminsByChat[fmt.Sprint(f.chat.Id)] = json.RawMessage(strings.ReplaceAll(string(f.client.adminsByChat[fmt.Sprint(f.chat.Id)]), from, to))
}
