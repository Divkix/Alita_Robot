package panel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/chattz"
	"github.com/divkix/Alita_Robot/alita/db/digestsettings"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/locks"
	"github.com/divkix/Alita_Robot/alita/db/logchannels"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/nightmode"
	"github.com/divkix/Alita_Robot/alita/db/reports"
	"github.com/divkix/Alita_Robot/alita/db/rules"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/settings"
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

func declaredSettingsField(chatID int64, declaration *settings.Declaration) settingsField {
	state := declaration.Read(chatID)
	_, key, _ := strings.Cut(declaration.Key, ".")
	var fieldType string
	switch declaration.Type {
	case settings.Switch:
		fieldType = "bool"
	case settings.Number, settings.Duration:
		fieldType = "int"
	case settings.Choice:
		fieldType = "string"
	default:
		panic("unmapped setting type")
	}
	return settingsField{
		Key: key, Type: fieldType, Value: state.Value, Default: declaration.Default,
		Changed: state.Changed, Writable: true, Options: declaration.Options,
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

	rulesRow := readRulesSettings(chatID)

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
			declaredSettingsField(chatID, settings.Welcome),
			declaredSettingsField(chatID, settings.CleanWelcome),
			declaredSettingsField(chatID, settings.Goodbye),
			declaredSettingsField(chatID, settings.CleanGoodbye),
			declaredSettingsField(chatID, settings.CleanService),
		}},
		{Key: "rules", Label: "Rules", Fields: []settingsField{
			// /setrules converts the admin's Markdown to Telegram HTML before
			// storing it; the panel has no editor for that, so it reports the
			// text and writes only the toggle.
			settingsString("rules_text", rulesRow.Rules, "", false, nil),
			settingsBool("private_rules", rulesRow.Private, false, true),
		}},
		{Key: "captcha", Label: "Captcha", Fields: []settingsField{
			declaredSettingsField(chatID, settings.CaptchaEnabled),
			declaredSettingsField(chatID, settings.CaptchaMode),
			declaredSettingsField(chatID, settings.CaptchaTimeout),
			declaredSettingsField(chatID, settings.CaptchaAction),
			declaredSettingsField(chatID, settings.CaptchaMaxAttempts),
		}},
		{Key: "antiflood", Label: "Antiflood", Fields: []settingsField{
			declaredSettingsField(chatID, settings.FloodLimit),
			declaredSettingsField(chatID, settings.FloodMode),
			declaredSettingsField(chatID, settings.FloodActionTime),
			declaredSettingsField(chatID, settings.FloodDelete),
		}},
		{Key: "antispam", Label: "Antispam", Fields: []settingsField{
			declaredSettingsField(chatID, settings.AISpamEnabled),
			declaredSettingsField(chatID, settings.SpamDBEnabled),
			declaredSettingsField(chatID, settings.SpamDBAction),
			declaredSettingsField(chatID, settings.SpamDBCAS),
			declaredSettingsField(chatID, settings.SpamDBLols),
			declaredSettingsField(chatID, settings.SpamDBShared),
			declaredSettingsField(chatID, settings.ProbationEnabled),
			declaredSettingsField(chatID, settings.ProbationTime),
			declaredSettingsField(chatID, settings.ProbationMessages),
			declaredSettingsField(chatID, settings.ProbationMinLength),
			declaredSettingsField(chatID, settings.ProbationBlockLinks),
			declaredSettingsField(chatID, settings.ProbationBlockForwards),
			declaredSettingsField(chatID, settings.ProbationBlockMedia),
			declaredSettingsField(chatID, settings.ProbationNotice),
		}},
		{Key: "antiraid", Label: "Antiraid", Fields: []settingsField{
			declaredSettingsField(chatID, settings.RaidTime),
			declaredSettingsField(chatID, settings.RaidActionTime),
			declaredSettingsField(chatID, settings.AutoAntiRaidThreshold),
		}},
		{Key: "warns", Label: "Warnings", Fields: []settingsField{
			declaredSettingsField(chatID, settings.WarnLimit),
			declaredSettingsField(chatID, settings.WarnMode),
			declaredSettingsField(chatID, settings.WarnActionTime),
			declaredSettingsField(chatID, settings.WarnExpiry),
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
	input := settings.WriteContext{ChatID: access.ChatID, Title: access.Title, Bot: p.bot}
	updates, fieldErrors := validateSettingsChanges(input, body.Changes, current, p.deps.NightModeLockTypes)
	if len(fieldErrors) > 0 {
		writeJSON(w, http.StatusBadRequest, settingsPatchResponse{
			Error:  &apiError{Code: "invalid_settings", Message: "one or more settings are invalid"},
			Errors: fieldErrors,
		})
		return
	}

	// Check every declared right before the first write; Telegram rights
	// such as change-info and restrict-members are independent.
	if !p.requirePermission(w, r, access, session.User.ID, settingsPermission(updates)) {
		return
	}

	saved, failed := applySettingsChanges(input, updates)
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
func validateSettingsChanges(input settings.WriteContext, changes map[string]json.RawMessage, index map[string]settingsField, nightTypes []string) (map[string]any, map[string]string) {
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

	for key, err := range settings.ValidateChanges(input, updates) {
		fieldErrors[key] = settingError(err)
	}

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
	if declaration, ok := settings.Lookup(key); ok {
		if err := declaration.Validate(value); err != nil {
			return err.Error()
		}
		return ""
	}
	if len(field.Options) > 0 {
		text, _ := value.(string)
		if !slices.Contains(field.Options, text) {
			return "must be one of: " + strings.Join(field.Options, ", ")
		}
	}

	moduleKey, fieldKey, _ := strings.Cut(key, ".")
	switch moduleKey {
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

func settingError(err error) string {
	if key := settings.RefusalKey(err); key != "" {
		return settingsRefusal(key)
	}
	return err.Error()
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
func applySettingsChanges(input settings.WriteContext, updates map[string]any) ([]string, map[string]string) {
	chatID := input.ChatID
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
	applied := make(map[string]error)
	var nightErr error
	nightApplied := false
	for _, key := range ordered {
		var err error
		if declaration, declared := settings.Lookup(key); declared {
			writeKey := declaration.WriteKey()
			var done bool
			err, done = applied[writeKey]
			if !done {
				err = declaration.WriteChanges(input, updates)
				applied[writeKey] = err
			}
		} else {
			switch key {
			case "nightmode.enabled", "nightmode.start", "nightmode.end", "nightmode.lock_types":
				if !nightApplied {
					nightErr = applyNightModeChanges(chatID, updates)
					nightApplied = true
				}
				err = nightErr
			default:
				err = applySettingChange(input, key, updates[key])
			}
		}
		if err != nil {
			log.WithError(err).Warnf("[Panel] save %s for chat %d", key, chatID)
			failed[key] = settingError(err)
			continue
		}
		saved = append(saved, key)
	}
	return saved, failed
}

func applySettingChange(input settings.WriteContext, key string, value any) error {
	chatID := input.ChatID
	moduleKey, fieldKey, _ := strings.Cut(key, ".")
	switch moduleKey {
	case "language":
		return lang.ChangeGroupLanguage(chatID, value.(string))
	case "rules":
		return rules.SetPrivateRules(chatID, value.(bool))
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
