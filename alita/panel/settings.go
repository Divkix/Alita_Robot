package panel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"slices"
	"strings"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/aispam"
	"github.com/divkix/Alita_Robot/alita/db/antiflood"
	"github.com/divkix/Alita_Robot/alita/db/antiraid"
	"github.com/divkix/Alita_Robot/alita/db/captcha"
	"github.com/divkix/Alita_Robot/alita/db/greetings"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/locks"
	"github.com/divkix/Alita_Robot/alita/db/logchannels"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/rules"
	"github.com/divkix/Alita_Robot/alita/db/spamdbsettings"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

// languageCodes is the set /lang offers and ChangeGroupLanguage accepts.
var languageCodes = []string{"en", "es", "fr", "hi", "id", "pt", "ru"}

// settingsField is one editable or readable setting. Type is one of bool, int,
// string or list; Options lists the accepted strings when it is an enum.
// Writable is false for values the panel only reports.
type settingsField struct {
	Key      string   `json:"key"`
	Type     string   `json:"type"`
	Value    any      `json:"value"`
	Default  any      `json:"default"`
	Changed  bool     `json:"changed"`
	Writable bool     `json:"writable"`
	Options  []string `json:"options,omitempty"`
}

type settingsModule struct {
	Key    string          `json:"key"`
	Label  string          `json:"label"`
	Fields []settingsField `json:"fields"`
}

// settingsResponse is the GET shape. Modules and fields keep a fixed order so a
// form can be rebuilt from it. ReadOnly is true unless the caller is an admin
// and the bot can act in the chat.
type settingsResponse struct {
	Modules        []settingsModule `json:"modules"`
	ReadOnly       bool             `json:"read_only"`
	ReadOnlyReason string           `json:"read_only_reason"`
}

type settingsPatchRequest struct {
	Changes map[string]json.RawMessage `json:"changes"`
}

// settingsPatchResponse reports per-field outcomes so a partial database
// failure is visible: every applied key is in Saved, every failed one in
// Failed with the setter's error. Errors is field-level validation output and
// is only present when nothing was written.
type settingsPatchResponse struct {
	Error          *apiError         `json:"error,omitempty"`
	Errors         map[string]string `json:"errors,omitempty"`
	Saved          []string          `json:"saved"`
	Failed         map[string]string `json:"failed"`
	Modules        []settingsModule  `json:"modules"`
	ReadOnly       bool              `json:"read_only"`
	ReadOnlyReason string            `json:"read_only_reason"`
}

func settingsBool(key string, value, def bool, writable bool) settingsField {
	return settingsField{Key: key, Type: "bool", Value: value, Default: def, Changed: value != def, Writable: writable}
}

func settingsInt(key string, value, def int, writable bool) settingsField {
	return settingsField{Key: key, Type: "int", Value: value, Default: def, Changed: value != def, Writable: writable}
}

func settingsString(key, value, def string, writable bool, options []string) settingsField {
	return settingsField{
		Key:      key,
		Type:     "string",
		Value:    value,
		Default:  def,
		Changed:  value != def,
		Writable: writable,
		Options:  options,
	}
}

// readSettingsRow loads a settings row without creating it. Several repository
// getters insert a row on a miss; a GET must not.
func readSettingsRow[T any](chatID int64) (*T, bool) {
	if db.DB == nil {
		return nil, false
	}
	var row T
	err := db.DB.Where("chat_id = ?", chatID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false
	}
	if err != nil {
		log.Warnf("[Panel] read %T for chat %d: %v", row, chatID, err)
		return nil, false
	}
	return &row, true
}

// readGreetingSettings loads the greeting row without creating it. A missing
// row, or one with nil embedded settings, reads the defaults the repository
// would use: welcome on, goodbye off.
func readGreetingSettings(chatID int64) models.GreetingSettings {
	defaults := models.GreetingSettings{
		ChatID:          chatID,
		WelcomeSettings: &models.WelcomeSettings{ShouldWelcome: true},
		GoodbyeSettings: &models.GoodbyeSettings{},
	}
	row, ok := readSettingsRow[models.GreetingSettings](chatID)
	if !ok {
		return defaults
	}
	out := *row
	if out.WelcomeSettings == nil {
		out.WelcomeSettings = defaults.WelcomeSettings
	}
	if out.GoodbyeSettings == nil {
		out.GoodbyeSettings = defaults.GoodbyeSettings
	}
	return out
}

func readRulesSettings(chatID int64) models.RulesSettings {
	row, ok := readSettingsRow[models.RulesSettings](chatID)
	if !ok {
		return models.RulesSettings{ChatId: chatID}
	}
	return *row
}

// panelSettings reads every supported setting and marks the response read-only
// for a non-admin caller, or one where the bot cannot act.
func panelSettings(access chatAccess) settingsResponse {
	chatID := access.ChatID

	language := lang.GetGroupLanguage(context.Background(), chatID)
	if language == "" {
		language = "en"
	}

	greeting := readGreetingSettings(chatID)
	rulesRow := readRulesSettings(chatID)

	captchaRow := &models.CaptchaSettings{
		ChatID:        chatID,
		CaptchaMode:   "math",
		Timeout:       2,
		FailureAction: "kick",
		MaxAttempts:   3,
	}
	if row, err := captcha.GetCaptchaSettings(chatID); err == nil && row != nil {
		captchaRow = row
	}

	flood := antiflood.GetFlood(chatID)
	if flood.Action == "" {
		flood.Action = "mute"
	}

	raid := antiraid.GetAntiRaidSettings(chatID)

	spamDB := &models.SpamDBSettings{ChatID: chatID, Action: spamdbsettings.ActionBan, CheckCAS: true}
	if row, err := spamdbsettings.GetSpamDBSettingsContext(context.Background(), chatID); err == nil && row != nil {
		spamDB = row
	}

	locked := make([]string, 0)
	for perm, on := range locks.GetChatLocks(chatID) {
		if on {
			locked = append(locked, perm)
		}
	}
	slices.Sort(locked)

	logChannel := logchannels.Get(chatID)

	modules := []settingsModule{
		{Key: "language", Label: "Language", Fields: []settingsField{
			settingsString("code", language, "en", true, languageCodes),
		}},
		{Key: "greetings", Label: "Greetings", Fields: []settingsField{
			settingsBool("welcome_enabled", greeting.WelcomeSettings.ShouldWelcome, true, true),
			settingsBool("clean_welcome", greeting.WelcomeSettings.CleanWelcome, false, true),
			settingsBool("goodbye_enabled", greeting.GoodbyeSettings.ShouldGoodbye, false, true),
			settingsBool("clean_goodbye", greeting.GoodbyeSettings.CleanGoodbye, false, true),
			settingsBool("clean_service", greeting.ShouldCleanService, false, true),
		}},
		{Key: "rules", Label: "Rules", Fields: []settingsField{
			// /setrules converts the admin's Markdown to Telegram HTML before
			// storing it; the panel has no editor for that, so it reports the
			// text and writes only the toggle.
			settingsString("rules_text", rulesRow.Rules, "", false, nil),
			settingsBool("private_rules", rulesRow.Private, false, true),
		}},
		{Key: "captcha", Label: "Captcha", Fields: []settingsField{
			settingsBool("enabled", captchaRow.Enabled, false, true),
			settingsString("mode", captchaRow.CaptchaMode, "math", true, []string{"math", "text"}),
			settingsInt("timeout", captchaRow.Timeout, 2, true),
			settingsString("action", captchaRow.FailureAction, "kick", true, []string{"kick", "ban", "mute"}),
			settingsInt("max_attempts", captchaRow.MaxAttempts, 3, true),
		}},
		{Key: "antiflood", Label: "Antiflood", Fields: []settingsField{
			settingsInt("limit", flood.Limit, 0, true),
			settingsString("action", flood.Action, "mute", true, []string{"ban", "kick", "mute"}),
			settingsBool("delete_messages", flood.DeleteAntifloodMessage, false, true),
		}},
		{Key: "antispam", Label: "Antispam", Fields: []settingsField{
			settingsBool("aispam_enabled", aispam.IsAISpamEnabled(chatID), false, true),
			settingsBool("spamdb_enabled", spamDB.Enabled, false, true),
			settingsString("spamdb_action", spamDB.Action, spamdbsettings.ActionBan, true,
				[]string{spamdbsettings.ActionBan, spamdbsettings.ActionMute, spamdbsettings.ActionLog}),
			settingsBool("spamdb_cas", spamDB.CheckCAS, true, true),
			settingsBool("spamdb_lols", spamDB.CheckLols, false, true),
		}},
		{Key: "antiraid", Label: "Antiraid", Fields: []settingsField{
			settingsInt("raid_time", raid.RaidTime, 21600, true),
			settingsInt("raid_action_time", raid.RaidActionTime, 3600, true),
			settingsInt("auto_threshold", raid.AutoAntiRaidThreshold, 0, true),
		}},
		{Key: "locks", Label: "Locks", Fields: []settingsField{
			{Key: "active", Type: "list", Value: locked, Default: []string{}, Changed: len(locked) > 0, Writable: false},
		}},
		{Key: "log_channel", Label: "Log channel", Fields: []settingsField{
			settingsBool("bound", logChannel != nil, false, false),
		}},
	}

	response := settingsResponse{Modules: modules}
	if access.Role != RoleAdmin {
		response.ReadOnly = true
		response.ReadOnlyReason = "not_admin"
	} else if !access.BotAdmin {
		response.ReadOnly = true
		response.ReadOnlyReason = "bot_not_admin"
	}
	return response
}

// settingsIndex maps "<module>.<field>" to its metadata.
func settingsIndex(response settingsResponse) map[string]settingsField {
	index := make(map[string]settingsField)
	for _, module := range response.Modules {
		for _, field := range module.Fields {
			index[module.Key+"."+field.Key] = field
		}
	}
	return index
}

// panelSettingsHandler answers GET with the current values and defaults.
func (p *Panel) handleSettings(w http.ResponseWriter, r *http.Request) {
	_, access, ok := p.chatRequest(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, panelSettings(access))
}

// handleUpdateSettings applies a batch of "<module>.<field>" changes. Every key
// and value is validated before the first write, so a rejected request changes
// nothing. A setter failure keeps the other writes and reports which fields
// saved and which did not.
func (p *Panel) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	session, access, ok := p.chatRequest(w, r)
	if !ok {
		return
	}
	if !requireWritable(w, access) {
		return
	}

	var body settingsPatchRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body", "request body must be a JSON object with a changes map")
		return
	}
	if len(body.Changes) == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid_settings", "no settings were provided")
		return
	}

	current := panelSettings(access)
	updates, fieldErrors := validateSettingsChanges(body.Changes, current)
	if len(fieldErrors) > 0 {
		writeJSON(w, http.StatusBadRequest, settingsPatchResponse{
			Error:  &apiError{Code: "invalid_settings", Message: "one or more settings are invalid"},
			Errors: fieldErrors,
		})
		return
	}

	// The equivalent command checks a right per setting; check the batch's
	// strongest requirement once, before the first write.
	if !p.requirePermission(w, r, access, session.User.ID, settingsPermission(updates)) {
		return
	}

	saved, failed := applySettingsChanges(access.ChatID, updates)
	if enabled, ok := updates["captcha.enabled"].(bool); ok && !enabled && slices.Contains(saved, "captcha.enabled") && p.deps.DisableCaptcha != nil {
		// Same cleanup /captcha off runs: unmute users still pending a captcha.
		if err := p.deps.DisableCaptcha(p.bot, access.ChatID); err != nil {
			log.WithError(err).Warnf("[Panel] captcha cleanup for chat %d", access.ChatID)
			saved = slices.DeleteFunc(saved, func(key string) bool { return key == "captcha.enabled" })
			failed["captcha.enabled"] = "captcha was turned off, but pending users could not be unmuted: " + err.Error()
		}
	}
	updated := panelSettings(access)
	response := settingsPatchResponse{
		Saved:          saved,
		Failed:         failed,
		Modules:        updated.Modules,
		ReadOnly:       updated.ReadOnly,
		ReadOnlyReason: updated.ReadOnlyReason,
	}
	if len(failed) > 0 {
		response.Error = &apiError{Code: "partial_failure", Message: "some settings were saved and some were not"}
		writeJSON(w, http.StatusInternalServerError, response)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

// validateSettingsChanges decodes and validates every change against the
// current snapshot. It returns decoded values, or field-level errors; callers
// write nothing when errors is non-empty.
func validateSettingsChanges(changes map[string]json.RawMessage, current settingsResponse) (map[string]any, map[string]string) {
	index := settingsIndex(current)
	updates := make(map[string]any, len(changes))
	fieldErrors := make(map[string]string)

	for key, raw := range changes {
		moduleKey, fieldKey, ok := strings.Cut(key, ".")
		if !ok || moduleKey == "" || fieldKey == "" {
			fieldErrors[key] = "unknown setting"
			continue
		}
		field, ok := index[key]
		if !ok {
			fieldErrors[key] = "unknown setting"
			continue
		}
		if !field.Writable {
			fieldErrors[key] = "this setting is read-only"
			continue
		}
		value, message := decodeSettingValue(field, raw)
		if message != "" {
			fieldErrors[key] = message
			continue
		}
		if message := validateSettingValue(key, field, value); message != "" {
			fieldErrors[key] = message
			continue
		}
		updates[key] = value
	}

	// The table constraint refuses a row with no spam database left on. Check
	// the batch as a whole so enabling one and disabling the other is allowed.
	cas, casChanged := updates["antispam.spamdb_cas"].(bool)
	if !casChanged {
		value, _ := index["antispam.spamdb_cas"].Value.(bool)
		cas = value
	}
	lols, lolsChanged := updates["antispam.spamdb_lols"].(bool)
	if !lolsChanged {
		value, _ := index["antispam.spamdb_lols"].Value.(bool)
		lols = value
	}
	if (casChanged || lolsChanged) && !cas && !lols {
		for _, key := range []string{"antispam.spamdb_cas", "antispam.spamdb_lols"} {
			if _, changed := updates[key]; changed {
				fieldErrors[key] = "at least one spam database must stay on"
			}
		}
	}

	return updates, fieldErrors
}

func decodeSettingValue(field settingsField, raw json.RawMessage) (any, string) {
	switch field.Type {
	case "bool":
		var value *bool
		if err := json.Unmarshal(raw, &value); err != nil || value == nil {
			return nil, "expected true or false"
		}
		return *value, ""
	case "int":
		var value *int
		if err := json.Unmarshal(raw, &value); err != nil || value == nil {
			return nil, "expected an integer"
		}
		return *value, ""
	case "string":
		var value *string
		if err := json.Unmarshal(raw, &value); err != nil || value == nil {
			return nil, "expected a string"
		}
		return *value, ""
	default:
		return nil, "unknown setting"
	}
}

func validateSettingValue(key string, field settingsField, value any) string {
	if len(field.Options) > 0 {
		text, _ := value.(string)
		if !slices.Contains(field.Options, text) {
			return "must be one of: " + strings.Join(field.Options, ", ")
		}
	}

	moduleKey, fieldKey, _ := strings.Cut(key, ".")
	switch moduleKey {
	case "captcha":
		return validateCaptchaValue(fieldKey, value)
	case "antiflood":
		if fieldKey == "limit" {
			if n := value.(int); n != 0 && (n < 3 || n > 100) {
				return "must be 0 (off) or between 3 and 100"
			}
		}
	case "antispam":
		return validateAntispamValue(fieldKey, value)
	case "antiraid":
		return validateAntiraidValue(fieldKey, value)
	}
	return ""
}

func validateCaptchaValue(field string, value any) string {
	switch field {
	case "timeout":
		if n := value.(int); n < 1 || n > 10 {
			return "must be between 1 and 10 minutes"
		}
	case "max_attempts":
		if n := value.(int); n < 1 || n > 10 {
			return "must be between 1 and 10"
		}
	}
	return ""
}

func validateAntiraidValue(field string, value any) string {
	if field != "raid_time" && field != "raid_action_time" && field != "auto_threshold" {
		return ""
	}
	if n := value.(int); n < 0 || n > math.MaxInt32 {
		if field == "auto_threshold" {
			return "must be between 0 and 2147483647"
		}
		return "must be between 0 and 2147483647 seconds"
	}
	return ""
}

func validateAntispamValue(field string, value any) string {
	switch field {
	case "aispam_enabled":
		if !value.(bool) {
			return ""
		}
		if config.AppConfig == nil || config.AppConfig.TypeSafeAPIKey == "" {
			return settingsRefusal("aispam_no_key")
		}
		if !config.AppConfig.EnableAISpam {
			return settingsRefusal("aispam_globally_disabled")
		}
	case "spamdb_enabled":
		if value.(bool) && (config.AppConfig == nil || !config.AppConfig.EnableSpamDB) {
			return settingsRefusal("spamdb_globally_disabled")
		}
	}
	return ""
}

// settingsRefusal reuses the command's English wording without the HTML that
// Telegram renders and the panel would show literally.
func settingsRefusal(localeKey string) string {
	text, _ := i18n.MustNewTranslator("en").GetString(localeKey)
	if text == "" {
		return localeKey
	}
	return plainText(text)
}

func plainText(text string) string {
	var b strings.Builder
	inTag := false
	for _, r := range text {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// applySettingsChanges writes through the same repository setters commands
// use, so each one invalidates the caches its command would. Boolean enables
// run before disables so switching the spam databases can proceed in either
// direction without tripping the last-database constraint.
func applySettingsChanges(chatID int64, updates map[string]any) ([]string, map[string]string) {
	keys := make([]string, 0, len(updates))
	for key := range updates {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	// Apply every enabled boolean first: switching the spam databases on and
	// off in one batch must not momentarily leave none on.
	ordered := make([]string, 0, len(keys))
	for _, key := range keys {
		if value, ok := updates[key].(bool); ok && value {
			ordered = append(ordered, key)
		}
	}
	for _, key := range keys {
		if value, ok := updates[key].(bool); ok && value {
			continue
		}
		ordered = append(ordered, key)
	}

	saved := make([]string, 0, len(keys))
	failed := make(map[string]string)
	for _, key := range ordered {
		if err := applySettingChange(chatID, key, updates[key]); err != nil {
			log.WithError(err).Warnf("[Panel] save %s for chat %d", key, chatID)
			failed[key] = err.Error()
			continue
		}
		saved = append(saved, key)
	}
	return saved, failed
}

func applySettingChange(chatID int64, key string, value any) error {
	moduleKey, fieldKey, _ := strings.Cut(key, ".")
	switch moduleKey {
	case "language":
		return lang.ChangeGroupLanguage(chatID, value.(string))
	case "greetings":
		return applyGreetingChange(chatID, fieldKey, value)
	case "rules":
		return rules.SetPrivateRules(chatID, value.(bool))
	case "captcha":
		return applyCaptchaChange(chatID, fieldKey, value)
	case "antiflood":
		return applyAntifloodChange(chatID, fieldKey, value)
	case "antispam":
		return applyAntispamChange(chatID, fieldKey, value)
	case "antiraid":
		return applyAntiraidChange(chatID, fieldKey, value)
	}
	return fmt.Errorf("unknown setting %q", key)
}

func applyGreetingChange(chatID int64, field string, value any) error {
	switch field {
	case "welcome_enabled":
		return greetings.SetWelcomeToggle(chatID, value.(bool))
	case "clean_welcome":
		return greetings.SetCleanWelcomeSetting(chatID, value.(bool))
	case "goodbye_enabled":
		return greetings.SetGoodbyeToggle(chatID, value.(bool))
	case "clean_goodbye":
		return greetings.SetCleanGoodbyeSetting(chatID, value.(bool))
	case "clean_service":
		return greetings.SetShouldCleanService(chatID, value.(bool))
	}
	return fmt.Errorf("unknown greeting setting %q", field)
}

func applyCaptchaChange(chatID int64, field string, value any) error {
	switch field {
	case "enabled":
		return captcha.SetCaptchaEnabled(chatID, value.(bool))
	case "mode":
		return captcha.SetCaptchaMode(chatID, value.(string))
	case "timeout":
		return captcha.SetCaptchaTimeout(chatID, value.(int))
	case "action":
		return captcha.SetCaptchaFailureAction(chatID, value.(string))
	case "max_attempts":
		return captcha.SetCaptchaMaxAttempts(chatID, value.(int))
	}
	return fmt.Errorf("unknown captcha setting %q", field)
}

func applyAntifloodChange(chatID int64, field string, value any) error {
	switch field {
	case "limit":
		return antiflood.SetFlood(chatID, value.(int))
	case "action":
		return antiflood.SetFloodMode(chatID, value.(string))
	case "delete_messages":
		return antiflood.SetFloodMsgDel(chatID, value.(bool))
	}
	return fmt.Errorf("unknown antiflood setting %q", field)
}

func applyAntispamChange(chatID int64, field string, value any) error {
	switch field {
	case "aispam_enabled":
		return aispam.SetAISpamEnabled(chatID, value.(bool))
	case "spamdb_enabled":
		return spamdbsettings.SetSpamDBEnabled(chatID, value.(bool))
	case "spamdb_action":
		return spamdbsettings.SetSpamDBAction(chatID, value.(string))
	case "spamdb_cas":
		return spamdbsettings.SetSpamDBDatabase(chatID, spamdbsettings.DatabaseCAS, value.(bool))
	case "spamdb_lols":
		return spamdbsettings.SetSpamDBDatabase(chatID, spamdbsettings.DatabaseLols, value.(bool))
	}
	return fmt.Errorf("unknown antispam setting %q", field)
}

func applyAntiraidChange(chatID int64, field string, value any) error {
	switch field {
	case "raid_time":
		return antiraid.SetRaidTime(chatID, value.(int))
	case "raid_action_time":
		return antiraid.SetRaidActionTime(chatID, value.(int))
	case "auto_threshold":
		return antiraid.SetAutoAntiRaidThreshold(chatID, value.(int))
	}
	return fmt.Errorf("unknown antiraid setting %q", field)
}
