//go:build testtools

package panel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/aispam"
	"github.com/divkix/Alita_Robot/alita/db/antiflood"
	"github.com/divkix/Alita_Robot/alita/db/antiraid"
	"github.com/divkix/Alita_Robot/alita/db/captcha"
	"github.com/divkix/Alita_Robot/alita/db/greetings"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/rules"
	"github.com/divkix/Alita_Robot/alita/db/spamdbsettings"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

const settingsFixtureUserID = 279058397

func newSettingsFixture(t *testing.T) (panelFixture, int64) {
	t.Helper()
	fixture := newPanelFixture(t)
	chatID := uniquePanelChatID()
	fixture.client.admins[chatID] = adminListJSON(settingsFixtureUserID, 999)
	// The fixture admin is the one who may write every module; refusal tests
	// take the right away again.
	fixture.client.rights[settingsFixtureUserID] = panelFakeRights{canChangeInfo: true, canRestrictMembers: true}
	seedPanelChat(t, chatID, fmt.Sprintf("Settings Chat %d", chatID), fixtureNow)
	return fixture, chatID
}

func settingsPath(chatID int64) string {
	return "/panel/api/v1/chats/" + strconv.FormatInt(chatID, 10) + "/settings"
}

func decodeSettingsResponse(t *testing.T, recorder *httptest.ResponseRecorder) settingsResponse {
	t.Helper()
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
	}
	var body settingsResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode settings: %v", err)
	}
	return body
}

func findSettingsField(t *testing.T, body settingsResponse, key string) settingsField {
	t.Helper()
	for _, module := range body.Modules {
		for _, field := range module.Fields {
			if module.Key+"."+field.Key == key {
				return field
			}
		}
	}
	t.Fatalf("settings response has no field %q", key)
	return settingsField{}
}

func fieldBool(t *testing.T, body settingsResponse, key string) bool {
	t.Helper()
	value, ok := findSettingsField(t, body, key).Value.(bool)
	if !ok {
		t.Fatalf("%s value is %T, want bool", key, findSettingsField(t, body, key).Value)
	}
	return value
}

func fieldInt(t *testing.T, body settingsResponse, key string) int {
	t.Helper()
	value, ok := findSettingsField(t, body, key).Value.(float64)
	if !ok {
		t.Fatalf("%s value is %T, want number", key, findSettingsField(t, body, key).Value)
	}
	return int(value)
}

func fieldString(t *testing.T, body settingsResponse, key string) string {
	t.Helper()
	value, ok := findSettingsField(t, body, key).Value.(string)
	if !ok {
		t.Fatalf("%s value is %T, want string", key, findSettingsField(t, body, key).Value)
	}
	return value
}

func countChatRows(t *testing.T, model any, chatID int64) int64 {
	t.Helper()
	var count int64
	if err := db.DB.Model(model).Where("chat_id = ?", chatID).Count(&count).Error; err != nil {
		t.Fatalf("count %T: %v", model, err)
	}
	return count
}

func TestSettingsGetReportsCurrentValuesAndDefaults(t *testing.T) {
	fixture, chatID := newSettingsFixture(t)

	recorder := panelRequest(t, fixture.handler, http.MethodGet, settingsPath(chatID), readFixture(t, "initdata_main.txt"), "", "")
	body := decodeSettingsResponse(t, recorder)
	if body.ReadOnly || body.ReadOnlyReason != "" {
		t.Fatalf("read_only = %v/%q, want writable for an admin with a bot admin", body.ReadOnly, body.ReadOnlyReason)
	}

	wantModules := []string{"language", "greetings", "rules", "captcha", "antiflood", "antispam", "antiraid", "locks", "log_channel"}
	if len(body.Modules) != len(wantModules) {
		t.Fatalf("modules = %d, want %d", len(body.Modules), len(wantModules))
	}
	for i, key := range wantModules {
		if body.Modules[i].Key != key {
			t.Fatalf("module %d = %q, want %q", i, body.Modules[i].Key, key)
		}
	}

	if got := fieldString(t, body, "language.code"); got != "en" {
		t.Fatalf("language.code = %q, want en", got)
	}
	if options := findSettingsField(t, body, "language.code").Options; len(options) != 7 {
		t.Fatalf("language options = %v, want the seven locales", options)
	}
	if !fieldBool(t, body, "greetings.welcome_enabled") {
		t.Fatal("greetings.welcome_enabled = false, want the repository default true")
	}
	for _, key := range []string{"greetings.clean_welcome", "greetings.goodbye_enabled", "greetings.clean_goodbye", "greetings.clean_service"} {
		if fieldBool(t, body, key) {
			t.Fatalf("%s = true, want the default false", key)
		}
	}
	if findSettingsField(t, body, "rules.rules_text").Writable {
		t.Fatal("rules.rules_text must be read-only")
	}
	if fieldBool(t, body, "rules.private_rules") {
		t.Fatal("rules.private_rules = true, want false")
	}
	if fieldString(t, body, "captcha.mode") != "math" || fieldInt(t, body, "captcha.timeout") != 2 ||
		fieldString(t, body, "captcha.action") != "kick" || fieldInt(t, body, "captcha.max_attempts") != 3 {
		t.Fatalf("captcha defaults = %+v", body.Modules[3])
	}
	if fieldInt(t, body, "antiflood.limit") != 0 || fieldString(t, body, "antiflood.action") != "mute" ||
		fieldBool(t, body, "antiflood.delete_messages") {
		t.Fatalf("antiflood defaults = %+v", body.Modules[4])
	}
	if fieldBool(t, body, "antispam.aispam_enabled") || fieldBool(t, body, "antispam.spamdb_enabled") {
		t.Fatal("antispam features must default off")
	}
	if fieldString(t, body, "antispam.spamdb_action") != "ban" ||
		!fieldBool(t, body, "antispam.spamdb_cas") || fieldBool(t, body, "antispam.spamdb_lols") ||
		fieldBool(t, body, "antispam.spamdb_shared") {
		t.Fatalf("spamdb defaults = %+v", body.Modules[5])
	}
	if fieldInt(t, body, "antiraid.raid_time") != 21600 || fieldInt(t, body, "antiraid.raid_action_time") != 3600 ||
		fieldInt(t, body, "antiraid.auto_threshold") != 0 {
		t.Fatalf("antiraid defaults = %+v", body.Modules[6])
	}
	if findSettingsField(t, body, "locks.active").Writable || findSettingsField(t, body, "log_channel.bound").Writable {
		t.Fatal("locks and log channel summaries must be read-only")
	}

	// A GET must not create the optional rows several getters would.
	for name, model := range map[string]any{
		"greetings":   &models.GreetingSettings{},
		"rules":       &models.RulesSettings{},
		"captcha":     &models.CaptchaSettings{},
		"antiflood":   &models.AntifloodSettings{},
		"antiraid":    &models.AntiRaidSettings{},
		"spam_db":     &models.SpamDBSettings{},
		"ai_spam":     &models.AISpamSettings{},
		"lock":        &models.LockSettings{},
		"log_channel": &models.LogChannel{},
	} {
		if count := countChatRows(t, model, chatID); count != 0 {
			t.Fatalf("GET created %d %s rows", count, name)
		}
	}
}

func TestSettingsPatchValidationWritesNothing(t *testing.T) {
	fixture, chatID := newSettingsFixture(t)
	raw := readFixture(t, "initdata_main.txt")
	target := settingsPath(chatID)

	tests := []struct {
		name     string
		body     string
		wantKeys []string
	}{
		{name: "unknown field", body: `{"changes":{"greetings.not_a_setting":true}}`, wantKeys: []string{"greetings.not_a_setting"}},
		{name: "unknown module", body: `{"changes":{"nope":true}}`, wantKeys: []string{"nope"}},
		{name: "wrong type", body: `{"changes":{"greetings.welcome_enabled":"yes"}}`, wantKeys: []string{"greetings.welcome_enabled"}},
		{name: "captcha timeout range", body: `{"changes":{"captcha.timeout":11}}`, wantKeys: []string{"captcha.timeout"}},
		{name: "language option", body: `{"changes":{"language.code":"xx"}}`, wantKeys: []string{"language.code"}},
		{name: "read-only field", body: `{"changes":{"locks.active":[]}}`, wantKeys: []string{"locks.active"}},
		{name: "antiflood range", body: `{"changes":{"antiflood.limit":2}}`, wantKeys: []string{"antiflood.limit"}},
		{name: "last spam database", body: `{"changes":{"antispam.spamdb_cas":false}}`, wantKeys: []string{"antispam.spamdb_cas"}},
		{name: "no spam database left", body: `{"changes":{"antispam.spamdb_cas":false,"antispam.spamdb_shared":false}}`, wantKeys: []string{"antispam.spamdb_cas", "antispam.spamdb_shared"}},
		{name: "valid beside invalid", body: `{"changes":{"greetings.welcome_enabled":false,"captcha.timeout":99}}`, wantKeys: []string{"captcha.timeout"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := panelRequest(t, fixture.handler, http.MethodPatch, target, raw, test.body, "application/json")
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body %s)", recorder.Code, recorder.Body.String())
			}
			var body settingsPatchResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body %q: %v", recorder.Body.String(), err)
			}
			if body.Error == nil || body.Error.Code != "invalid_settings" {
				t.Fatalf("error = %+v, want invalid_settings", body.Error)
			}
			for _, key := range test.wantKeys {
				if body.Errors[key] == "" {
					t.Fatalf("errors = %v, want an entry for %q", body.Errors, key)
				}
			}

			// Nothing was written, including the valid key sent next to an
			// invalid one.
			for name, model := range map[string]any{
				"greetings": &models.GreetingSettings{},
				"captcha":   &models.CaptchaSettings{},
			} {
				if count := countChatRows(t, model, chatID); count != 0 {
					t.Fatalf("%s: %d %s rows written", test.name, count, name)
				}
			}
		})
	}

	empty := panelRequest(t, fixture.handler, http.MethodPatch, target, raw, `{"changes":{}}`, "application/json")
	requireAPIError(t, empty, http.StatusBadRequest, "invalid_settings")
}

func TestSettingsPatchAppliesThroughSettersAndInvalidatesCache(t *testing.T) {
	fixture, chatID := newSettingsFixture(t)
	raw := readFixture(t, "initdata_main.txt")

	// Read every getter first: each answer is cached, so a setter that failed
	// to invalidate its key would leave the old value visible below.
	greetings.GetGreetingSettings(chatID)
	if _, err := captcha.GetCaptchaSettings(chatID); err != nil {
		t.Fatalf("seed captcha cache: %v", err)
	}
	antiflood.GetFlood(chatID)
	antiraid.GetAntiRaidSettings(chatID)
	rules.GetChatRulesInfo(chatID)
	if _, err := spamdbsettings.GetSpamDBSettingsContext(context.Background(), chatID); err != nil {
		t.Fatalf("seed spamdb cache: %v", err)
	}
	if got := lang.GetGroupLanguage(context.Background(), chatID); got != "en" {
		t.Fatalf("seeded language = %q, want en", got)
	}

	body := `{"changes":{
		"language.code":"es",
		"greetings.welcome_enabled":false,
		"greetings.clean_welcome":true,
		"greetings.goodbye_enabled":true,
		"greetings.clean_goodbye":true,
		"greetings.clean_service":true,
		"rules.private_rules":true,
		"captcha.enabled":true,
		"captcha.mode":"text",
		"captcha.timeout":5,
		"captcha.action":"ban",
		"captcha.max_attempts":2,
		"antiflood.limit":5,
		"antiflood.action":"ban",
		"antiflood.delete_messages":true,
		"antispam.spamdb_action":"log",
		"antispam.spamdb_lols":true,
		"antiraid.raid_time":7200,
		"antiraid.raid_action_time":1800,
		"antiraid.auto_threshold":5
	}}`
	recorder := panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), raw, body, "application/json")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
	}
	var patch settingsPatchResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &patch); err != nil {
		t.Fatalf("decode patch response: %v", err)
	}
	if len(patch.Saved) != 20 || len(patch.Failed) != 0 {
		t.Fatalf("saved = %v, failed = %v; want all 20 saved", patch.Saved, patch.Failed)
	}

	// The bot's own getters answer with the new values at once.
	if got := lang.GetGroupLanguage(context.Background(), chatID); got != "es" {
		t.Fatalf("language after patch = %q, want es", got)
	}
	greeting := greetings.GetGreetingSettings(chatID)
	if greeting.WelcomeSettings.ShouldWelcome || !greeting.WelcomeSettings.CleanWelcome ||
		!greeting.GoodbyeSettings.ShouldGoodbye || !greeting.GoodbyeSettings.CleanGoodbye || !greeting.ShouldCleanService {
		t.Fatalf("greeting settings after patch = %+v", greeting)
	}
	if !rules.GetChatRulesInfo(chatID).Private {
		t.Fatal("private rules after patch = false, want true")
	}
	captchaSettings, err := captcha.GetCaptchaSettings(chatID)
	if err != nil {
		t.Fatalf("captcha settings after patch: %v", err)
	}
	if !captchaSettings.Enabled || captchaSettings.CaptchaMode != "text" || captchaSettings.Timeout != 5 ||
		captchaSettings.FailureAction != "ban" || captchaSettings.MaxAttempts != 2 {
		t.Fatalf("captcha settings after patch = %+v", captchaSettings)
	}
	flood := antiflood.GetFlood(chatID)
	if flood.Limit != 5 || flood.Action != "ban" || !flood.DeleteAntifloodMessage {
		t.Fatalf("antiflood settings after patch = %+v", flood)
	}
	spamdb, err := spamdbsettings.GetSpamDBSettingsContext(context.Background(), chatID)
	if err != nil {
		t.Fatalf("spam db settings after patch: %v", err)
	}
	if spamdb.Action != "log" || !spamdb.CheckLols || !spamdb.CheckCAS {
		t.Fatalf("spam db settings after patch = %+v", spamdb)
	}
	raid := antiraid.GetAntiRaidSettings(chatID)
	if raid.RaidTime != 7200 || raid.RaidActionTime != 1800 || raid.AutoAntiRaidThreshold != 5 {
		t.Fatalf("antiraid settings after patch = %+v", raid)
	}

	// The next GET reflects the same values and marks them changed.
	after := decodeSettingsResponse(t, panelRequest(t, fixture.handler, http.MethodGet, settingsPath(chatID), raw, "", ""))
	if got := fieldString(t, after, "language.code"); got != "es" || !findSettingsField(t, after, "language.code").Changed {
		t.Fatalf("GET language = %q changed=%v, want es changed", got, findSettingsField(t, after, "language.code").Changed)
	}
	if fieldBool(t, after, "greetings.welcome_enabled") || !findSettingsField(t, after, "greetings.welcome_enabled").Changed {
		t.Fatal("GET greetings.welcome_enabled did not reflect the patch")
	}
	if fieldInt(t, after, "captcha.timeout") != 5 || fieldString(t, after, "captcha.mode") != "text" {
		t.Fatalf("GET captcha = %+v", after.Modules[3])
	}
}

func TestSettingsNonAdminGetsReadOnlyAndCannotWrite(t *testing.T) {
	fixture := newPanelFixture(t)
	chatID := uniquePanelChatID()
	fixture.client.admins[chatID] = adminListJSON(999)
	seedPanelChat(t, chatID, "Read-only Chat", fixtureNow)
	raw := readFixture(t, "initdata_main.txt")

	recorder := panelRequest(t, fixture.handler, http.MethodGet, settingsPath(chatID), raw, "", "")
	body := decodeSettingsResponse(t, recorder)
	if !body.ReadOnly || body.ReadOnlyReason != "not_admin" {
		t.Fatalf("read_only = %v/%q, want true/not_admin", body.ReadOnly, body.ReadOnlyReason)
	}
	if got := fieldString(t, body, "language.code"); got != "en" {
		t.Fatalf("read-only GET language = %q, want en", got)
	}

	write := panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), raw,
		`{"changes":{"greetings.welcome_enabled":false}}`, "application/json")
	requireAPIError(t, write, http.StatusForbidden, "not_admin")
	if count := countChatRows(t, &models.GreetingSettings{}, chatID); count != 0 {
		t.Fatalf("non-admin write created %d greeting rows", count)
	}
}

func TestSettingsBotNotAdminIsReadOnlyAndRejectsWrites(t *testing.T) {
	fixture := newPanelFixture(t)
	fixture.client.botAdmin = false
	chatID := uniquePanelChatID()
	// The bot is deliberately absent from the admin list so IsBotAdmin falls
	// through to getChatMember, which the fake answers as a plain member.
	fixture.client.admins[chatID] = adminListJSON(settingsFixtureUserID)
	seedPanelChat(t, chatID, "Bot Demoted Chat", fixtureNow)
	raw := readFixture(t, "initdata_main.txt")

	recorder := panelRequest(t, fixture.handler, http.MethodGet, settingsPath(chatID), raw, "", "")
	body := decodeSettingsResponse(t, recorder)
	if !body.ReadOnly || body.ReadOnlyReason != "bot_not_admin" {
		t.Fatalf("read_only = %v/%q, want true/bot_not_admin", body.ReadOnly, body.ReadOnlyReason)
	}

	write := panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), raw,
		`{"changes":{"captcha.enabled":true}}`, "application/json")
	requireAPIError(t, write, http.StatusConflict, "bot_not_admin")
	if count := countChatRows(t, &models.CaptchaSettings{}, chatID); count != 0 {
		t.Fatalf("refused write created %d captcha rows", count)
	}
}

func TestSettingsPatchRefusesGloballyDisabledFeatures(t *testing.T) {
	rawLocale, err := os.ReadFile(filepath.Join("..", "..", "locales", "en.yml"))
	if err != nil {
		t.Fatalf("read locales/en.yml: %v", err)
	}
	restoreI18n, err := i18n.OverrideLocalesForTest(map[string]string{"en": string(rawLocale)})
	if err != nil {
		t.Fatalf("override locales: %v", err)
	}
	t.Cleanup(restoreI18n)

	fixture, chatID := newSettingsFixture(t)
	raw := readFixture(t, "initdata_main.txt")
	target := settingsPath(chatID)

	restore := *config.AppConfig
	t.Cleanup(func() { *config.AppConfig = restore })

	config.AppConfig.EnableAISpam = false
	config.AppConfig.TypeSafeAPIKey = "test-key"
	recorder := panelRequest(t, fixture.handler, http.MethodPatch, target, raw,
		`{"changes":{"antispam.aispam_enabled":true}}`, "application/json")
	assertFieldRefusal(t, recorder, "antispam.aispam_enabled", "switched it off globally")
	if count := countChatRows(t, &models.AISpamSettings{}, chatID); count != 0 {
		t.Fatalf("refused AI spam write created %d rows", count)
	}

	config.AppConfig.EnableAISpam = true
	config.AppConfig.TypeSafeAPIKey = ""
	recorder = panelRequest(t, fixture.handler, http.MethodPatch, target, raw,
		`{"changes":{"antispam.aispam_enabled":true}}`, "application/json")
	assertFieldRefusal(t, recorder, "antispam.aispam_enabled", "has not configured an API key")

	config.AppConfig.EnableSpamDB = false
	recorder = panelRequest(t, fixture.handler, http.MethodPatch, target, raw,
		`{"changes":{"antispam.spamdb_enabled":true}}`, "application/json")
	assertFieldRefusal(t, recorder, "antispam.spamdb_enabled", "switched it off")
	if count := countChatRows(t, &models.SpamDBSettings{}, chatID); count != 0 {
		t.Fatalf("refused spam database write created %d rows", count)
	}

	// Turning the feature off is never refused by the kill switch.
	config.AppConfig.EnableAISpam = false
	if err := aispam.SetAISpamEnabled(chatID, true); err != nil {
		t.Fatalf("seed AI spam: %v", err)
	}
	off := panelRequest(t, fixture.handler, http.MethodPatch, target, raw,
		`{"changes":{"antispam.aispam_enabled":false}}`, "application/json")
	if off.Code != http.StatusOK {
		t.Fatalf("disable status = %d, want 200 (body %s)", off.Code, off.Body.String())
	}
	if aispam.IsAISpamEnabled(chatID) {
		t.Fatal("AI spam stayed on after the panel disabled it")
	}
}

// Shared bans is a third spam database in the panel: off by default, refused
// while the operator has not enabled the network, and part of the at-least-one
// rule.
func TestSettingsSharedSpamDatabase(t *testing.T) {
	rawLocale, err := os.ReadFile(filepath.Join("..", "..", "locales", "en.yml"))
	if err != nil {
		t.Fatalf("read locales/en.yml: %v", err)
	}
	restoreI18n, err := i18n.OverrideLocalesForTest(map[string]string{"en": string(rawLocale)})
	if err != nil {
		t.Fatalf("override locales: %v", err)
	}
	t.Cleanup(restoreI18n)

	fixture, chatID := newSettingsFixture(t)
	raw := readFixture(t, "initdata_main.txt")
	target := settingsPath(chatID)

	restore := *config.AppConfig
	t.Cleanup(func() { *config.AppConfig = restore })

	body := decodeSettingsResponse(t, panelRequest(t, fixture.handler, http.MethodGet, target, raw, "", ""))
	if fieldBool(t, body, "antispam.spamdb_shared") {
		t.Fatal("antispam.spamdb_shared = true, want the shared source off by default")
	}

	// Without the operator's network the panel refuses the same way the
	// command does, and writes nothing.
	config.AppConfig.EnableReputation = false
	recorder := panelRequest(t, fixture.handler, http.MethodPatch, target, raw,
		`{"changes":{"antispam.spamdb_shared":true}}`, "application/json")
	assertFieldRefusal(t, recorder, "antispam.spamdb_shared", "shared reputation")
	if count := countChatRows(t, &models.SpamDBSettings{}, chatID); count != 0 {
		t.Fatalf("refused shared write created %d rows", count)
	}

	// Enables run before disables, so switching shared on and CAS off in one
	// batch does not trip the last-database constraint.
	config.AppConfig.EnableReputation = true
	recorder = panelRequest(t, fixture.handler, http.MethodPatch, target, raw,
		`{"changes":{"antispam.spamdb_shared":true,"antispam.spamdb_cas":false}}`, "application/json")
	if recorder.Code != http.StatusOK {
		t.Fatalf("enable shared status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
	}
	settings, err := spamdbsettings.GetSpamDBSettingsContext(context.Background(), chatID)
	if err != nil {
		t.Fatalf("GetSpamDBSettingsContext() error = %v", err)
	}
	if !settings.CheckShared || settings.CheckCAS {
		t.Fatalf("spam db settings after patch = %+v, want shared only", settings)
	}

	// Shared is the last database now, so the panel refuses to leave none on.
	recorder = panelRequest(t, fixture.handler, http.MethodPatch, target, raw,
		`{"changes":{"antispam.spamdb_shared":false}}`, "application/json")
	assertFieldRefusal(t, recorder, "antispam.spamdb_shared", "at least one spam database must stay on")
	settings, err = spamdbsettings.GetSpamDBSettingsContext(context.Background(), chatID)
	if err != nil {
		t.Fatalf("GetSpamDBSettingsContext() error = %v", err)
	}
	if !settings.CheckShared {
		t.Fatal("Shared bans was switched off as the last database")
	}
}

func assertFieldRefusal(t *testing.T, recorder *httptest.ResponseRecorder, key, wantText string) {
	t.Helper()
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body %s)", recorder.Code, recorder.Body.String())
	}
	var body settingsPatchResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if message := body.Errors[key]; !strings.Contains(message, wantText) {
		t.Fatalf("error for %s = %q, want it to contain %q", key, message, wantText)
	}
}

func TestSettingsPatchReportsPartialFailure(t *testing.T) {
	fixture, chatID := newSettingsFixture(t)
	raw := readFixture(t, "initdata_main.txt")

	// Make one setter fail for real: its table is gone while the others still
	// write. AutoMigrate restores it for the rest of the package.
	if err := db.DB.Migrator().DropTable(&models.CaptchaSettings{}); err != nil {
		t.Fatalf("drop captcha table: %v", err)
	}
	t.Cleanup(func() {
		if err := db.DB.AutoMigrate(&models.CaptchaSettings{}); err != nil {
			t.Errorf("restore captcha table: %v", err)
		}
	})

	recorder := panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), raw,
		`{"changes":{"greetings.welcome_enabled":false,"captcha.enabled":true}}`, "application/json")
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (body %s)", recorder.Code, recorder.Body.String())
	}
	var body settingsPatchResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error == nil || body.Error.Code != "partial_failure" {
		t.Fatalf("error = %+v, want partial_failure", body.Error)
	}
	if !containsString(body.Saved, "greetings.welcome_enabled") {
		t.Fatalf("saved = %v, want greetings.welcome_enabled", body.Saved)
	}
	if body.Failed["captcha.enabled"] == "" {
		t.Fatalf("failed = %v, want captcha.enabled", body.Failed)
	}

	// The field that saved is visible through the command's getter.
	if greetings.GetGreetingSettings(chatID).WelcomeSettings.ShouldWelcome {
		t.Fatal("the greeting write reported as saved is not visible")
	}
}

// Turning captcha off from the panel runs the same cleanup as /captcha off, and
// a cleanup failure is reported instead of being hidden behind a saved field.
func TestSettingsPatchCaptchaOffRunsTheCommandCleanup(t *testing.T) {
	fixture, chatID := newSettingsFixture(t)
	raw := readFixture(t, "initdata_main.txt")
	if err := captcha.SetCaptchaEnabled(chatID, true); err != nil {
		t.Fatalf("SetCaptchaEnabled() error = %v", err)
	}

	var cleaned []int64
	var cleanupErr error
	fixture.handler = New(&gotgbot.Bot{
		Token:     fixtureBotToken,
		BotClient: fixture.client,
		User:      gotgbot.User{Id: 999, IsBot: true, FirstName: "Alita"},
	}, Deps{
		SessionMaxAge: 24 * time.Hour,
		LookupChats:   sqliteLookup,
		Now:           func() time.Time { return fixtureNow },
		DisableCaptcha: func(_ *gotgbot.Bot, id int64) error {
			cleaned = append(cleaned, id)
			return cleanupErr
		},
	})

	recorder := panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), raw,
		`{"changes":{"captcha.enabled":false}}`, "application/json")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
	}
	if len(cleaned) != 1 || cleaned[0] != chatID {
		t.Fatalf("cleanup ran for %v, want once for %d", cleaned, chatID)
	}

	if err := captcha.SetCaptchaEnabled(chatID, true); err != nil {
		t.Fatalf("SetCaptchaEnabled() error = %v", err)
	}
	cleanupErr = fmt.Errorf("telegram refused the unmute")
	recorder = panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), raw,
		`{"changes":{"captcha.enabled":false}}`, "application/json")
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (body %s)", recorder.Code, recorder.Body.String())
	}
	var body settingsPatchResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if containsString(body.Saved, "captcha.enabled") || body.Failed["captcha.enabled"] == "" {
		t.Fatalf("saved = %v, failed = %v, want captcha.enabled reported as failed", body.Saved, body.Failed)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
