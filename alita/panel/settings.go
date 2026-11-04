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
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/aispam"
	"github.com/divkix/Alita_Robot/alita/db/antiflood"
	"github.com/divkix/Alita_Robot/alita/db/antiraid"
	"github.com/divkix/Alita_Robot/alita/db/captcha"
	"github.com/divkix/Alita_Robot/alita/db/chattz"
	"github.com/divkix/Alita_Robot/alita/db/digestsettings"
	"github.com/divkix/Alita_Robot/alita/db/greetings"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/locks"
	"github.com/divkix/Alita_Robot/alita/db/logchannels"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/nightmode"
	"github.com/divkix/Alita_Robot/alita/db/probation"
	"github.com/divkix/Alita_Robot/alita/db/reports"
	"github.com/divkix/Alita_Robot/alita/db/rules"
	"github.com/divkix/Alita_Robot/alita/db/spamdbsettings"
	"github.com/divkix/Alita_Robot/alita/db/warns"
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

func readWarnSettings(chatID int64) models.WarnSettings {
	if row, ok := readSettingsRow[models.WarnSettings](chatID); ok {
		return *row
	}
	return models.WarnSettings{ChatId: chatID, WarnLimit: 3, WarnMode: "mute"}
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
	warn := readWarnSettings(chatID)
	digestRow := &models.DigestSettings{ChatID: chatID}
	if row, err := digestsettings.GetContext(context.Background(), chatID); err == nil && row != nil {
		digestRow = row
	}
	reportRow := &models.ReportChatSettings{ChatId: chatID, Enabled: true, Status: true}
	if row, ok := readSettingsRow[models.ReportChatSettings](chatID); ok {
		reportRow = row
	}
	timezone := chattz.DefaultName
	if name, err := chattz.Name(chatID); err == nil {
		timezone = name
	}
	night := &models.NightModeSettings{ChatID: chatID}
	if row, err := nightmode.GetContext(context.Background(), chatID); err == nil && row != nil {
		night = row
	}

	spamDB := &models.SpamDBSettings{ChatID: chatID, Action: spamdbsettings.ActionBan, CheckCAS: true}
	if row, err := spamdbsettings.GetSpamDBSettingsContext(context.Background(), chatID); err == nil && row != nil {
		spamDB = row
	}

	probationRow := models.DefaultProbationSettings(chatID)
	if row, err := probation.GetSettings(context.Background(), chatID); err == nil && row != nil {
		probationRow = row
	}
	probationDefault := models.DefaultProbationSettings(chatID)

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
			settingsString("action", flood.Action, "mute", true, []string{"ban", "kick", "mute", "tban", "tmute"}),
			settingsInt("action_time", flood.FloodActionTime, 0, true),
			settingsBool("delete_messages", flood.DeleteAntifloodMessage, false, true),
		}},
		{Key: "antispam", Label: "Antispam", Fields: []settingsField{
			settingsBool("aispam_enabled", aispam.IsAISpamEnabled(chatID), false, true),
			settingsBool("spamdb_enabled", spamDB.Enabled, false, true),
			settingsString("spamdb_action", spamDB.Action, spamdbsettings.ActionBan, true,
				[]string{spamdbsettings.ActionBan, spamdbsettings.ActionMute, spamdbsettings.ActionLog}),
			settingsBool("spamdb_cas", spamDB.CheckCAS, true, true),
			settingsBool("spamdb_lols", spamDB.CheckLols, false, true),
			settingsBool("spamdb_shared", spamDB.CheckShared, false, true),
			settingsBool("probation_enabled", probationRow.Enabled, false, true),
			settingsInt("probation_time", probationRow.DurationSeconds, probationDefault.DurationSeconds, true),
			settingsInt("probation_messages", probationRow.MessageCount, probationDefault.MessageCount, true),
			settingsInt("probation_min_length", probationRow.MinLength, probationDefault.MinLength, true),
			settingsBool("probation_block_links", probationRow.BlockLinks, true, true),
			settingsBool("probation_block_forwards", probationRow.BlockForwards, true, true),
			settingsBool("probation_block_media", probationRow.BlockMedia, false, true),
			settingsBool("probation_notice", probationRow.Notice, false, true),
		}},
		{Key: "antiraid", Label: "Antiraid", Fields: []settingsField{
			settingsInt("raid_time", raid.RaidTime, 21600, true),
			settingsInt("raid_action_time", raid.RaidActionTime, 3600, true),
			settingsInt("auto_threshold", raid.AutoAntiRaidThreshold, 0, true),
		}},
		{Key: "warns", Label: "Warnings", Fields: []settingsField{
			settingsInt("limit", warn.WarnLimit, 3, true),
			settingsString("mode", warn.WarnMode, "mute", true, []string{"ban", "kick", "mute", "tban", "tmute"}),
			settingsInt("action_time", warn.WarnActionTime, 0, true),
			settingsInt("expiry", warn.WarnExpiry, 0, true),
		}},
		{Key: "digest", Label: "Weekly digest", Fields: []settingsField{
			settingsBool("enabled", digestRow.Enabled, false, true),
			settingsString("destination", digestRow.Destination, "", true, []string{"", "log", "group"}),
		}},
		{Key: "reports", Label: "Reports", Fields: []settingsField{
			settingsBool("enabled", reportRow.Status, true, true),
			settingsInt("mute_threshold", reportRow.MuteThreshold, 0, true),
		}},
		{Key: "timezone", Label: "Timezone", Fields: []settingsField{
			settingsString("name", timezone, chattz.DefaultName, true, nil),
		}},
		{Key: "nightmode", Label: "Night mode", Fields: []settingsField{
			settingsBool("enabled", night.Enabled, false, true),
			settingsString("start", nightClock(night.StartMinute), "00:00", true, nil),
			settingsString("end", nightClock(night.EndMinute), "00:00", true, nil),
			settingsString("lock_types", night.LockTypes, "", true, nil),
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

	current := settingsIndex(panelSettings(access))
	updates, fieldErrors := validateSettingsChanges(body.Changes, current, p.deps.NightModeLockTypes)
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

	saved, failed := applySettingsChanges(access.ChatID, access.Title, updates, current)
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
func validateSettingsChanges(changes map[string]json.RawMessage, index map[string]settingsField, nightTypes []string) (map[string]any, map[string]string) {
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

	validateAntifloodModeChanges(index, updates, fieldErrors)
	validateSpamDatabaseChanges(index, updates, fieldErrors)
	// /probation block takes at least one kind; turning probation off is how
	// a chat blocks nothing.
	if !anyKeyOn(index, updates, probationBlockKeys) {
		for _, key := range probationBlockKeys {
			if _, changed := updates[key]; changed {
				fieldErrors[key] = "probation must block at least one of links, forwards and media"
			}
		}
	}

	validateWarnModeChanges(index, updates, fieldErrors)
	if message := validateNightModeChanges(index, updates, nightTypes, fieldErrors); message != "" {
		for _, key := range nightModeKeys {
			if _, changed := updates[key]; changed {
				fieldErrors[key] = message
			}
		}
	}

	// Same refusal as /digest log: a log destination needs a bound log channel.
	if destination, _ := updates["digest.destination"].(string); destination == digestsettings.DestinationLog {
		if bound, _ := index["log_channel.bound"].Value.(bool); !bound {
			fieldErrors["digest.destination"] = settingsRefusal("digest_log_unavailable")
		}
	}

	return updates, fieldErrors
}

// validateAntifloodModeChanges checks the effective action and duration
// together: tban/tmute need a usable duration, other actions take none.
func validateAntifloodModeChanges(index map[string]settingsField, updates map[string]any, fieldErrors map[string]string) {
	action, actionTime := antifloodModeValues(index, updates)
	_, actionChanged := updates["antiflood.action"]
	_, timeChanged := updates["antiflood.action_time"]
	if !actionChanged && !timeChanged {
		return
	}
	if action == "tban" || action == "tmute" {
		if !models.ValidActionTime(actionTime) {
			if actionChanged {
				fieldErrors["antiflood.action"] = "temporary flood actions require a duration between 60 and 31622400 seconds"
			}
			if timeChanged {
				fieldErrors["antiflood.action_time"] = "must be between 60 and 31622400 seconds"
			}
		}
	} else if timeChanged && !actionChanged {
		fieldErrors["antiflood.action_time"] = "requires a tban or tmute flood action"
	}
}

// validateSpamDatabaseChanges mirrors the table constraint that refuses a row
// with no spam database left on. It checks the batch as a whole, so enabling
// one and disabling another is allowed.
func validateSpamDatabaseChanges(index map[string]settingsField, updates map[string]any, fieldErrors map[string]string) {
	databases := []string{"antispam.spamdb_cas", "antispam.spamdb_lols", "antispam.spamdb_shared"}
	anyOn, anyChanged := false, false
	for _, key := range databases {
		on, changed := updates[key].(bool)
		if !changed {
			value, _ := index[key].Value.(bool)
			on = value
		}
		anyOn = anyOn || on
		anyChanged = anyChanged || changed
	}
	if anyChanged && !anyOn {
		for _, key := range databases {
			if _, changed := updates[key]; changed {
				fieldErrors[key] = "at least one spam database must stay on"
			}
		}
	}
}

// antifloodModeValues resolves the action and duration from the same snapshot
// used to validate the batch. Untimed actions always clear the duration.
func antifloodModeValues(index map[string]settingsField, updates map[string]any) (string, int) {
	action, changed := updates["antiflood.action"].(string)
	if !changed {
		action, _ = index["antiflood.action"].Value.(string)
	}
	actionTime, changed := updates["antiflood.action_time"].(int)
	if !changed {
		actionTime, _ = index["antiflood.action_time"].Value.(int)
	}
	if action != "tban" && action != "tmute" {
		actionTime = 0
	}
	return action, actionTime
}

var nightModeKeys = []string{"nightmode.enabled", "nightmode.start", "nightmode.end", "nightmode.lock_types"}

func nightClock(minute int) string {
	return fmt.Sprintf("%02d:%02d", minute/60, minute%60)
}

// validateNightModeChanges validates the entire effective configuration before
// any write. Disabled windows stay intact; editing them requires enabling in
// the same batch rather than briefly enabling a supposedly disabled feature.
func validateNightModeChanges(index map[string]settingsField, updates map[string]any, validTypes []string, fieldErrors map[string]string) string {
	changed, configChanged := false, false
	for _, key := range nightModeKeys {
		if fieldErrors[key] != "" {
			return ""
		}
		if _, ok := updates[key]; ok {
			changed = true
			configChanged = configChanged || key != "nightmode.enabled"
		}
	}
	if !changed {
		return ""
	}
	value := func(key string) any {
		if v, ok := updates[key]; ok {
			return v
		}
		return index[key].Value
	}
	if !value("nightmode.enabled").(bool) {
		if configChanged {
			return "turn night mode on in the same save to edit its window or lock types"
		}
		return ""
	}
	var types []string
	for field := range strings.SplitSeq(value("nightmode.lock_types").(string), ",") {
		lockType := strings.ToLower(strings.TrimSpace(field))
		if lockType == "" {
			continue
		}
		if !slices.Contains(validTypes, lockType) {
			return "unknown night mode lock type: " + field
		}
		types = append(types, lockType)
	}
	if len(types) == 0 {
		return settingsRefusal("nightmode_no_types")
	}
	start, _ := nightmode.ParseClock(value("nightmode.start").(string))
	end, _ := nightmode.ParseClock(value("nightmode.end").(string))
	if start == end {
		return settingsRefusal("nightmode_same_times")
	}
	if _, changed := updates["nightmode.lock_types"]; changed {
		slices.Sort(types)
		updates["nightmode.lock_types"] = strings.Join(slices.Compact(types), ",")
	}
	return ""
}

// validateWarnModeChanges checks the effective mode and duration together, so
// a batch can switch to a timed action without depending on field order.
func validateWarnModeChanges(index map[string]settingsField, updates map[string]any, fieldErrors map[string]string) {
	mode, modeChanged := updates["warns.mode"].(string)
	seconds, timeChanged := updates["warns.action_time"].(int)
	if !modeChanged && !timeChanged {
		return
	}
	if !modeChanged {
		mode, _ = index["warns.mode"].Value.(string)
	}
	if !timeChanged {
		seconds, _ = index["warns.action_time"].Value.(int)
	}
	message := ""
	if mode == "tban" || mode == "tmute" {
		if !models.ValidActionTime(seconds) {
			message = "a timed action needs between 60 and 31622400 seconds"
		}
	} else if timeChanged && (!modeChanged || seconds != 0) {
		message = "action duration is only available for tban and tmute"
	}
	if message != "" {
		for _, key := range []string{"warns.mode", "warns.action_time"} {
			if _, changed := updates[key]; changed {
				fieldErrors[key] = message
			}
		}
	}
}

// probationBlockKeys are the panel fields of /probation block.
var probationBlockKeys = []string{"antispam.probation_block_links", "antispam.probation_block_forwards", "antispam.probation_block_media"}

// anyKeyOn reports whether one of keys is on after updates, or true when
// updates changes none of them.
func anyKeyOn(index map[string]settingsField, updates map[string]any, keys []string) bool {
	anyOn, anyChanged := false, false
	for _, key := range keys {
		on, changed := updates[key].(bool)
		if !changed {
			on, _ = index[key].Value.(bool)
		}
		anyOn = anyOn || on
		anyChanged = anyChanged || changed
	}
	return anyOn || !anyChanged
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
		if fieldKey == "action_time" {
			if n := value.(int); n != 0 && !models.ValidActionTime(n) {
				return "must be 0 (unset) or between 60 and 31622400 seconds"
			}
		}
	case "antispam":
		return validateAntispamValue(fieldKey, value)
	case "antiraid":
		return validateAntiraidValue(fieldKey, value)
	case "warns":
		return validateWarnValue(fieldKey, value)
	case "digest":
		if fieldKey == "enabled" && value.(bool) && (config.AppConfig == nil || !config.AppConfig.EnableDigest) {
			return settingsRefusal("digest_globally_disabled")
		}
	case "reports":
		if fieldKey == "mute_threshold" {
			if n := value.(int); n != 0 && (n < models.ReportMuteThresholdMin || n > models.ReportMuteThresholdMax) {
				return "must be 0 (off) or between 2 and 20"
			}
		}
	case "timezone":
		if _, _, err := chattz.Parse(value.(string)); err != nil {
			return "must be an IANA timezone such as Asia/Kolkata or UTC"
		}
	case "nightmode":
		if fieldKey == "start" || fieldKey == "end" {
			if _, ok := nightmode.ParseClock(value.(string)); !ok {
				return "must be a 24-hour time such as 23:00 or 07:00"
			}
		}
	}
	return ""
}

func validateWarnValue(field string, value any) string {
	switch field {
	case "limit":
		if n := value.(int); n < 1 || n > 100 {
			return "must be between 1 and 100"
		}
	case "action_time":
		if n := value.(int); n != 0 && !models.ValidActionTime(n) {
			return "must be between 60 and 31622400 seconds"
		}
	case "expiry":
		if !models.ValidWarnExpiry(value.(int)) {
			return "must be 0 (never) or between 60 and 31622400 seconds"
		}
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
	case "spamdb_shared":
		if value.(bool) && (config.AppConfig == nil || !config.AppConfig.EnableReputation) {
			return settingsRefusal("spamdb_db_shared_disabled")
		}
	case "probation_time":
		if n := value.(int); n < int(models.ProbationDurationMin/time.Second) || n > int(models.ProbationDurationMax/time.Second) {
			return "must be between 3600 and 2592000 seconds (1 hour to 30 days)"
		}
	case "probation_messages":
		if n := value.(int); n < models.ProbationMessagesMin || n > models.ProbationMessagesMax {
			return "must be between 1 and 100"
		}
	case "probation_min_length":
		if n := value.(int); n < models.ProbationMinLengthMin || n > models.ProbationMinLengthMax {
			return "must be between 1 and 500"
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
// direction without tripping the last-database constraint. Flood action and
// duration share one setter call, with outcomes reported for each requested key.
func applySettingsChanges(chatID int64, chatName string, updates map[string]any, current map[string]settingsField) ([]string, map[string]string) {
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
	floodAction, floodActionTime := antifloodModeValues(current, updates)
	var floodModeErr, warnModeErr, nightErr error
	floodModeApplied, warnModeApplied, nightApplied := false, false, false
	for _, key := range ordered {
		var err error
		switch key {
		case "antiflood.action", "antiflood.action_time":
			if !floodModeApplied {
				floodModeErr = antiflood.SetFloodMode(chatID, floodAction, floodActionTime)
				floodModeApplied = true
			}
			err = floodModeErr
		case "warns.mode", "warns.action_time":
			if !warnModeApplied {
				warnModeErr = applyWarnModeChange(chatID, updates)
				warnModeApplied = true
			}
			err = warnModeErr
		case "nightmode.enabled", "nightmode.start", "nightmode.end", "nightmode.lock_types":
			if !nightApplied {
				nightErr = applyNightModeChanges(chatID, updates)
				nightApplied = true
			}
			err = nightErr
		default:
			err = applySettingChange(chatID, chatName, key, updates[key])
		}
		if err != nil {
			log.WithError(err).Warnf("[Panel] save %s for chat %d", key, chatID)
			failed[key] = err.Error()
			continue
		}
		saved = append(saved, key)
	}
	return saved, failed
}

func applySettingChange(chatID int64, chatName, key string, value any) error {
	moduleKey, fieldKey, _ := strings.Cut(key, ".")
	switch moduleKey {
	case "language":
		return lang.ChangeGroupLanguage(chatID, value.(string))
	case "greetings":
		return applyGreetingChange(chatID, fieldKey, value)
	case "rules":
		return rules.SetPrivateRules(chatID, value.(bool))
	case "captcha":
		return applyCaptchaChange(chatID, chatName, fieldKey, value)
	case "antiflood":
		return applyAntifloodChange(chatID, fieldKey, value)
	case "antispam":
		return applyAntispamChange(chatID, fieldKey, value)
	case "antiraid":
		return applyAntiraidChange(chatID, fieldKey, value)
	case "warns":
		switch fieldKey {
		case "limit":
			return warns.SetWarnLimit(chatID, value.(int))
		case "expiry":
			return warns.SetWarnExpiry(chatID, value.(int))
		}
	case "digest":
		switch fieldKey {
		case "enabled":
			return digestsettings.SetEnabled(chatID, value.(bool))
		case "destination":
			return digestsettings.SetDestination(chatID, value.(string))
		}
	case "reports":
		switch fieldKey {
		case "enabled":
			return reports.SetChatReportStatus(chatID, value.(bool))
		case "mute_threshold":
			return reports.SetChatReportMuteThreshold(chatID, value.(int))
		}
	case "timezone":
		_, err := chattz.Set(chatID, value.(string))
		return err
	}
	return fmt.Errorf("unknown setting %q", key)
}

func applyWarnModeChange(chatID int64, updates map[string]any) error {
	row := readWarnSettings(chatID)
	if mode, ok := updates["warns.mode"].(string); ok {
		row.WarnMode = mode
	}
	if seconds, ok := updates["warns.action_time"].(int); ok {
		row.WarnActionTime = seconds
	}
	if row.WarnMode != "tban" && row.WarnMode != "tmute" {
		row.WarnActionTime = 0
	}
	return warns.SetWarnMode(chatID, row.WarnMode, row.WarnActionTime)
}

func applyNightModeChanges(chatID int64, updates map[string]any) error {
	if enabled, changed := updates["nightmode.enabled"].(bool); changed && !enabled {
		return nightmode.Disable(chatID)
	}
	row, err := nightmode.Get(chatID)
	if err != nil {
		return err
	}
	if row == nil {
		return errors.New("night mode settings are unavailable")
	}
	start, end, types := row.StartMinute, row.EndMinute, row.Types()
	if clock, changed := updates["nightmode.start"].(string); changed {
		start, _ = nightmode.ParseClock(clock)
	}
	if clock, changed := updates["nightmode.end"].(string); changed {
		end, _ = nightmode.ParseClock(clock)
	}
	if names, changed := updates["nightmode.lock_types"].(string); changed {
		types = strings.Split(names, ",")
	}
	return nightmode.Enable(chatID, start, end, types)
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

func applyCaptchaChange(chatID int64, chatName, field string, value any) error {
	switch field {
	case "enabled":
		return captcha.SetCaptchaEnabled(chatID, chatName, value.(bool))
	case "mode":
		return captcha.SetCaptchaMode(chatID, chatName, value.(string))
	case "timeout":
		return captcha.SetCaptchaTimeout(chatID, chatName, value.(int))
	case "action":
		return captcha.SetCaptchaFailureAction(chatID, chatName, value.(string))
	case "max_attempts":
		return captcha.SetCaptchaMaxAttempts(chatID, chatName, value.(int))
	}
	return fmt.Errorf("unknown captcha setting %q", field)
}

func applyAntifloodChange(chatID int64, field string, value any) error {
	switch field {
	case "limit":
		return antiflood.SetFlood(chatID, value.(int))
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
	case "spamdb_shared":
		return spamdbsettings.SetSpamDBDatabase(chatID, spamdbsettings.DatabaseShared, value.(bool))
	case "probation_enabled":
		return probation.SetEnabled(chatID, value.(bool))
	case "probation_time":
		return probation.SetDuration(chatID, time.Duration(value.(int))*time.Second)
	case "probation_messages":
		return probation.SetMessageCount(chatID, value.(int))
	case "probation_min_length":
		return probation.SetMinLength(chatID, value.(int))
	case "probation_block_links":
		return probation.SetBlock(chatID, probation.BlockLinks, value.(bool))
	case "probation_block_forwards":
		return probation.SetBlock(chatID, probation.BlockForwards, value.(bool))
	case "probation_block_media":
		return probation.SetBlock(chatID, probation.BlockMedia, value.(bool))
	case "probation_notice":
		return probation.SetNotice(chatID, value.(bool))
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
