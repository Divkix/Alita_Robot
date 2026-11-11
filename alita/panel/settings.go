package panel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"slices"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/locks"
	"github.com/divkix/Alita_Robot/alita/db/logchannels"
	"github.com/divkix/Alita_Robot/alita/db/rules"
	"github.com/divkix/Alita_Robot/alita/i18n"
	chatsettings "github.com/divkix/Alita_Robot/alita/settings"
)

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

func declaredSettingsField(ctx context.Context, chatID int64, declaration *chatsettings.Declaration) settingsField {
	state := declaration.ReadContext(ctx, chatID)
	_, key, _ := strings.Cut(declaration.Key, ".")
	var fieldType string
	switch declaration.Type {
	case chatsettings.Switch:
		fieldType = "bool"
	case chatsettings.Number, chatsettings.Duration:
		fieldType = "int"
	case chatsettings.Choice, chatsettings.Text:
		fieldType = "string"
	default:
		panic("unmapped setting type")
	}
	return settingsField{
		Key: key, Type: fieldType, Value: state.Value, Default: declaration.Default,
		Changed: state.Changed, Writable: true, Options: declaration.Options,
	}
}

// panelSettings reads every supported setting and marks the response read-only
// for a non-admin caller, or one where the bot cannot act.
func panelSettings(ctx context.Context, access chatAccess) settingsResponse {
	chatID := access.ChatID

	rulesRow := rules.GetChatRulesInfo(chatID)

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
			declaredSettingsField(ctx, chatID, chatsettings.Language),
		}},
		{Key: "greetings", Label: "Greetings", Fields: []settingsField{
			declaredSettingsField(ctx, chatID, chatsettings.Welcome),
			declaredSettingsField(ctx, chatID, chatsettings.CleanWelcome),
			declaredSettingsField(ctx, chatID, chatsettings.Goodbye),
			declaredSettingsField(ctx, chatID, chatsettings.CleanGoodbye),
			declaredSettingsField(ctx, chatID, chatsettings.CleanService),
		}},
		{Key: "rules", Label: "Rules", Fields: []settingsField{
			// /setrules converts the admin's Markdown to Telegram HTML before
			// storing it; the panel has no editor for that, so it reports the
			// text and writes only the toggle.
			settingsString("rules_text", rulesRow.Rules, "", false, nil),
			declaredSettingsField(ctx, chatID, chatsettings.PrivateRules),
		}},
		{Key: "captcha", Label: "Captcha", Fields: []settingsField{
			declaredSettingsField(ctx, chatID, chatsettings.CaptchaEnabled),
			declaredSettingsField(ctx, chatID, chatsettings.CaptchaMode),
			declaredSettingsField(ctx, chatID, chatsettings.CaptchaTimeout),
			declaredSettingsField(ctx, chatID, chatsettings.CaptchaAction),
			declaredSettingsField(ctx, chatID, chatsettings.CaptchaMaxAttempts),
		}},
		{Key: "antiflood", Label: "Antiflood", Fields: []settingsField{
			declaredSettingsField(ctx, chatID, chatsettings.FloodLimit),
			declaredSettingsField(ctx, chatID, chatsettings.FloodMode),
			declaredSettingsField(ctx, chatID, chatsettings.FloodActionTime),
			declaredSettingsField(ctx, chatID, chatsettings.FloodDelete),
		}},
		{Key: "antispam", Label: "Antispam", Fields: []settingsField{
			declaredSettingsField(ctx, chatID, chatsettings.AISpamEnabled),
			declaredSettingsField(ctx, chatID, chatsettings.SpamDBEnabled),
			declaredSettingsField(ctx, chatID, chatsettings.SpamDBAction),
			declaredSettingsField(ctx, chatID, chatsettings.SpamDBCAS),
			declaredSettingsField(ctx, chatID, chatsettings.SpamDBLols),
			declaredSettingsField(ctx, chatID, chatsettings.SpamDBShared),
			declaredSettingsField(ctx, chatID, chatsettings.ProbationEnabled),
			declaredSettingsField(ctx, chatID, chatsettings.ProbationTime),
			declaredSettingsField(ctx, chatID, chatsettings.ProbationMessages),
			declaredSettingsField(ctx, chatID, chatsettings.ProbationMinLength),
			declaredSettingsField(ctx, chatID, chatsettings.ProbationBlockLinks),
			declaredSettingsField(ctx, chatID, chatsettings.ProbationBlockForwards),
			declaredSettingsField(ctx, chatID, chatsettings.ProbationBlockMedia),
			declaredSettingsField(ctx, chatID, chatsettings.ProbationNotice),
		}},
		{Key: "antiraid", Label: "Antiraid", Fields: []settingsField{
			declaredSettingsField(ctx, chatID, chatsettings.RaidTime),
			declaredSettingsField(ctx, chatID, chatsettings.RaidActionTime),
			declaredSettingsField(ctx, chatID, chatsettings.AutoAntiRaidThreshold),
		}},
		{Key: "warns", Label: "Warnings", Fields: []settingsField{
			declaredSettingsField(ctx, chatID, chatsettings.WarnLimit),
			declaredSettingsField(ctx, chatID, chatsettings.WarnMode),
			declaredSettingsField(ctx, chatID, chatsettings.WarnActionTime),
			declaredSettingsField(ctx, chatID, chatsettings.WarnExpiry),
		}},
		{Key: "digest", Label: "Weekly digest", Fields: []settingsField{
			declaredSettingsField(ctx, chatID, chatsettings.DigestEnabled),
			declaredSettingsField(ctx, chatID, chatsettings.DigestDestination),
		}},
		{Key: "reports", Label: "Reports", Fields: []settingsField{
			declaredSettingsField(ctx, chatID, chatsettings.ReportsEnabled),
			declaredSettingsField(ctx, chatID, chatsettings.ReportMuteThreshold),
		}},
		{Key: "timezone", Label: "Timezone", Fields: []settingsField{
			declaredSettingsField(ctx, chatID, chatsettings.Timezone),
		}},
		{Key: "ephemeral", Label: "Private answers", Fields: []settingsField{
			declaredSettingsField(ctx, chatID, chatsettings.EphemeralReplies),
		}},
		{Key: "nightmode", Label: "Night mode", Fields: []settingsField{
			declaredSettingsField(ctx, chatID, chatsettings.NightModeEnabled),
			declaredSettingsField(ctx, chatID, chatsettings.NightModeStart),
			declaredSettingsField(ctx, chatID, chatsettings.NightModeEnd),
			declaredSettingsField(ctx, chatID, chatsettings.NightModeLockTypes),
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
	writeJSON(w, http.StatusOK, panelSettings(r.Context(), access))
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

	current := settingsIndex(panelSettings(r.Context(), access))
	input := chatsettings.WriteContext{Context: r.Context(), ChatID: access.ChatID, Title: access.Title, Bot: p.bot}
	updates, fieldErrors := validateSettingsChanges(input, body.Changes, current)
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
	updated := panelSettings(r.Context(), access)
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
func validateSettingsChanges(input chatsettings.WriteContext, changes map[string]json.RawMessage, index map[string]settingsField) (map[string]any, map[string]string) {
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
		updates[key] = value
	}

	for key, err := range chatsettings.ValidateChanges(input, updates) {
		fieldErrors[key] = settingError(err)
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

func settingError(err error) string {
	var inputError *chatsettings.NightModeInputError
	if errors.As(err, &inputError) {
		return settingsRefusal(string(inputError.Refusal), i18n.TranslationParams{
			"input": html.EscapeString(inputError.Input), "type": html.EscapeString(inputError.LockType),
		})
	}
	if key := chatsettings.RefusalKey(err); key != "" {
		return settingsRefusal(key)
	}
	return err.Error()
}

// settingsRefusal reuses the command's English wording without the HTML that
// Telegram renders and the panel would show literally.
func settingsRefusal(localeKey string, params ...i18n.TranslationParams) string {
	text, _ := i18n.MustNewTranslator("en").GetString(localeKey, params...)
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

// applySettingsChanges writes through declarations and reuses each shared write's
// outcome for every requested field. Boolean enables run before disables so
// switching spam databases never momentarily leaves none enabled.
func applySettingsChanges(input chatsettings.WriteContext, updates map[string]any) ([]string, map[string]string) {
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
	for _, key := range ordered {
		var err error
		if declaration, declared := chatsettings.Lookup(key); declared {
			writeKey := declaration.WriteKey()
			var done bool
			err, done = applied[writeKey]
			if !done {
				err = declaration.WriteChanges(input, updates)
				applied[writeKey] = err
			}
		} else {
			err = fmt.Errorf("unknown setting %q", key)
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
