//go:build testtools

package panel

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	_ "time/tzdata"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/greetings"
	"github.com/divkix/Alita_Robot/alita/db/models"
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

func countChatRows(t *testing.T, model any, chatID int64) int64 {
	t.Helper()
	var count int64
	if err := db.DB.Model(model).Where("chat_id = ?", chatID).Count(&count).Error; err != nil {
		t.Fatalf("count %T: %v", model, err)
	}
	return count
}

func TestSettingsGetResponseShape(t *testing.T) {
	fixture, chatID := newSettingsFixture(t)

	recorder := panelRequest(t, fixture.handler, http.MethodGet, settingsPath(chatID), readFixture(t, "initdata_main.txt"), "", "")
	body := decodeSettingsResponse(t, recorder)
	if body.ReadOnly || body.ReadOnlyReason != "" {
		t.Fatalf("read_only = %v/%q, want writable for an admin with a bot admin", body.ReadOnly, body.ReadOnlyReason)
	}

	wantModules := []string{"language", "greetings", "rules", "captcha", "antiflood", "antispam", "antiraid", "warns", "digest", "reports", "timezone", "ephemeral", "nightmode", "locks", "log_channel"}
	if len(body.Modules) != len(wantModules) {
		t.Fatalf("modules = %d, want %d", len(body.Modules), len(wantModules))
	}
	for i, key := range wantModules {
		if body.Modules[i].Key != key {
			t.Fatalf("module %d = %q, want %q", i, body.Modules[i].Key, key)
		}
	}

	if findSettingsField(t, body, "rules.rules_text").Writable {
		t.Fatal("rules.rules_text must be read-only")
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
		"warns":       &models.WarnSettings{},
		"digest":      &models.DigestSettings{},
		"reports":     &models.ReportChatSettings{},
		"timezone":    &models.ChatTimezone{},
		"ephemeral":   &models.EphemeralSettings{},
		"nightmode":   &models.NightModeSettings{},
		"warn_users":  &models.Warns{},
		"warn_entry":  &models.WarnEntry{},
		"lock":        &models.LockSettings{},
		"log_channel": &models.LogChannel{},
	} {
		if count := countChatRows(t, model, chatID); count != 0 {
			t.Fatalf("GET created %d %s rows", count, name)
		}
	}
}

func TestSettingsPatchValidationWritesNothing(t *testing.T) {
	raw := readFixture(t, "initdata_main.txt")

	tests := []struct {
		name     string
		body     string
		wantKeys []string
	}{
		{name: "unknown field", body: `{"changes":{"greetings.not_a_setting":true}}`, wantKeys: []string{"greetings.not_a_setting"}},
		{name: "unknown module", body: `{"changes":{"nope":true}}`, wantKeys: []string{"nope"}},
		{name: "wrong type", body: `{"changes":{"greetings.welcome_enabled":"yes"}}`, wantKeys: []string{"greetings.welcome_enabled"}},
		{name: "read-only field", body: `{"changes":{"locks.active":[]}}`, wantKeys: []string{"locks.active"}},
		{name: "read-only rules", body: `{"changes":{"rules.rules_text":"new rules"}}`, wantKeys: []string{"rules.rules_text"}},
		{name: "read-only log channel", body: `{"changes":{"log_channel.bound":true}}`, wantKeys: []string{"log_channel.bound"}},
		{name: "flood fractional duration", body: `{"changes":{"antiflood.action":"tmute","antiflood.action_time":60.5,"greetings.welcome_enabled":false}}`, wantKeys: []string{"antiflood.action_time"}},
		{name: "flood string duration", body: `{"changes":{"antiflood.action":"tmute","antiflood.action_time":"60","greetings.welcome_enabled":false}}`, wantKeys: []string{"antiflood.action_time"}},
		{name: "flood null duration", body: `{"changes":{"antiflood.action":"tmute","antiflood.action_time":null,"greetings.welcome_enabled":false}}`, wantKeys: []string{"antiflood.action_time"}},
		{name: "warn limit fraction", body: `{"changes":{"warns.limit":3.5}}`, wantKeys: []string{"warns.limit"}},
		{name: "warn limit wrong type", body: `{"changes":{"warns.limit":"3"}}`, wantKeys: []string{"warns.limit"}},
		{name: "warn mode wrong type", body: `{"changes":{"warns.mode":true}}`, wantKeys: []string{"warns.mode"}},
		{name: "warn duration wrong type", body: `{"changes":{"warns.mode":"tban","warns.action_time":"60"}}`, wantKeys: []string{"warns.action_time"}},
		{name: "warn expiry wrong type", body: `{"changes":{"warns.expiry":false}}`, wantKeys: []string{"warns.expiry"}},
		{name: "digest enabled wrong type", body: `{"changes":{"digest.enabled":"true"}}`, wantKeys: []string{"digest.enabled"}},
		{name: "digest enabled null", body: `{"changes":{"digest.enabled":null}}`, wantKeys: []string{"digest.enabled"}},
		{name: "digest destination wrong type", body: `{"changes":{"digest.destination":true}}`, wantKeys: []string{"digest.destination"}},
		{name: "digest destination null", body: `{"changes":{"digest.destination":null}}`, wantKeys: []string{"digest.destination"}},
		{name: "reports enabled wrong type", body: `{"changes":{"reports.enabled":"true"}}`, wantKeys: []string{"reports.enabled"}},
		{name: "reports enabled null", body: `{"changes":{"reports.enabled":null}}`, wantKeys: []string{"reports.enabled"}},
		{name: "report threshold fraction", body: `{"changes":{"reports.mute_threshold":2.5}}`, wantKeys: []string{"reports.mute_threshold"}},
		{name: "report threshold string", body: `{"changes":{"reports.mute_threshold":"2"}}`, wantKeys: []string{"reports.mute_threshold"}},
		{name: "report threshold bool", body: `{"changes":{"reports.mute_threshold":true}}`, wantKeys: []string{"reports.mute_threshold"}},
		{name: "report threshold null", body: `{"changes":{"reports.mute_threshold":null}}`, wantKeys: []string{"reports.mute_threshold"}},
		{name: "report threshold array", body: `{"changes":{"reports.mute_threshold":[]}}`, wantKeys: []string{"reports.mute_threshold"}},
		{name: "report threshold object", body: `{"changes":{"reports.mute_threshold":{}}}`, wantKeys: []string{"reports.mute_threshold"}},
		{name: "timezone number", body: `{"changes":{"timezone.name":5}}`, wantKeys: []string{"timezone.name"}},
		{name: "timezone bool", body: `{"changes":{"timezone.name":true}}`, wantKeys: []string{"timezone.name"}},
		{name: "timezone null", body: `{"changes":{"timezone.name":null}}`, wantKeys: []string{"timezone.name"}},
		{name: "timezone array", body: `{"changes":{"timezone.name":[]}}`, wantKeys: []string{"timezone.name"}},
		{name: "timezone object", body: `{"changes":{"timezone.name":{}}}`, wantKeys: []string{"timezone.name"}},
		{name: "nightmode enabled string", body: `{"changes":{"nightmode.enabled":"true"}}`, wantKeys: []string{"nightmode.enabled"}},
		{name: "nightmode enabled null", body: `{"changes":{"nightmode.enabled":null}}`, wantKeys: []string{"nightmode.enabled"}},
		{name: "nightmode start wrong type", body: `{"changes":{"nightmode.enabled":true,"nightmode.start":2300,"nightmode.end":"07:00","nightmode.lock_types":"media"}}`, wantKeys: []string{"nightmode.start"}},
		{name: "nightmode end null", body: `{"changes":{"nightmode.enabled":true,"nightmode.start":"23:00","nightmode.end":null,"nightmode.lock_types":"media"}}`, wantKeys: []string{"nightmode.end"}},
		{name: "nightmode lock types array", body: `{"changes":{"nightmode.enabled":true,"nightmode.start":"23:00","nightmode.end":"07:00","nightmode.lock_types":["media"]}}`, wantKeys: []string{"nightmode.lock_types"}},
		{name: "nightmode lock types null", body: `{"changes":{"nightmode.enabled":true,"nightmode.start":"23:00","nightmode.end":"07:00","nightmode.lock_types":null}}`, wantKeys: []string{"nightmode.lock_types"}},
	}

	// Each case gets its own panel: the table outgrows one user's rate limit.
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture, chatID := newSettingsFixture(t)
			recorder := panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), raw, test.body, "application/json")
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
			if len(body.Saved) != 0 || len(body.Failed) != 0 {
				t.Fatalf("saved = %v, failed = %v, want no attempted writes", body.Saved, body.Failed)
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
				"antiflood": &models.AntifloodSettings{},
				"warns":     &models.WarnSettings{},
				"digest":    &models.DigestSettings{},
				"reports":   &models.ReportChatSettings{},
				"timezone":  &models.ChatTimezone{},
				"ephemeral": &models.EphemeralSettings{},
				"nightmode": &models.NightModeSettings{},
			} {
				if count := countChatRows(t, model, chatID); count != 0 {
					t.Fatalf("%s: %d %s rows written", test.name, count, name)
				}
			}
		})
	}

	fixture, chatID := newSettingsFixture(t)
	empty := panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), raw, `{"changes":{}}`, "application/json")
	requireAPIError(t, empty, http.StatusBadRequest, "invalid_settings")
}

func TestSettingsPatchReportsInvalidJSONWritesNothing(t *testing.T) {
	for name, payload := range map[string]string{
		"unfinished threshold": `{"changes":{"greetings.welcome_enabled":false,"reports.enabled":false,"reports.mute_threshold":`,
		"trailing comma":       `{"changes":{"greetings.welcome_enabled":false,"reports.enabled":false,"reports.mute_threshold":2,}}`,
		"changes array":        `{"changes":[{"reports.enabled":false,"reports.mute_threshold":2}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			fixture, chatID := newSettingsFixture(t)
			recorder := panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), readFixture(t, "initdata_main.txt"),
				payload, "application/json")
			requireAPIError(t, recorder, http.StatusBadRequest, "invalid_body")
			for name, model := range map[string]any{
				"greetings": &models.GreetingSettings{},
				"reports":   &models.ReportChatSettings{},
			} {
				if count := countChatRows(t, model, chatID); count != 0 {
					t.Fatalf("invalid JSON created %d %s rows", count, name)
				}
			}
		})
	}
}

func TestSettingsPatchReportsEverySavedField(t *testing.T) {
	fixture, chatID := newSettingsFixture(t)
	raw := readFixture(t, "initdata_main.txt")

	body := `{"changes":{
		"language.code":"es",
		"greetings.welcome_enabled":false,
		"greetings.clean_welcome":true,
		"greetings.goodbye_enabled":true,
		"greetings.clean_goodbye":true,
		"greetings.clean_service":true,
		"rules.private_rules":true,
		"antiflood.limit":5,
		"antiflood.action":"tban",
		"antiflood.action_time":7200,
		"antiflood.delete_messages":true
	}}`
	recorder := panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), raw, body, "application/json")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
	}
	var patch settingsPatchResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &patch); err != nil {
		t.Fatalf("decode patch response: %v", err)
	}
	if len(patch.Saved) != 11 || len(patch.Failed) != 0 {
		t.Fatalf("saved = %v, failed = %v; want all 11 saved", patch.Saved, patch.Failed)
	}

	if len(patch.Modules) == 0 || patch.ReadOnly || patch.ReadOnlyReason != "" {
		t.Fatalf("patch response omitted writable settings: %+v", patch)
	}
}

func TestSettingsPatchCaptchaPreservesTelegramTitle(t *testing.T) {
	for _, change := range []struct {
		key, value string
	}{
		{"captcha.enabled", "true"},
		{"captcha.mode", `"text"`},
		{"captcha.timeout", "5"},
		{"captcha.action", `"ban"`},
		{"captcha.max_attempts", "7"},
	} {
		for _, existingBlank := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/existingBlank=%t", change.key, existingBlank), func(t *testing.T) {
				fixture, chatID := newSettingsFixture(t)
				if existingBlank {
					if err := db.DB.Model(&models.Chat{}).Where("chat_id = ?", chatID).Update("chat_name", "").Error; err != nil {
						t.Fatalf("clear stored chat title: %v", err)
					}
				} else if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error; err != nil {
					t.Fatalf("remove seeded chat: %v", err)
				}
				recorder := panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), readFixture(t, "initdata_main.txt"),
					fmt.Sprintf(`{"changes":{%q:%s}}`, change.key, change.value), "application/json")
				if recorder.Code != http.StatusOK {
					t.Fatalf("PATCH status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
				}
				var response settingsPatchResponse
				if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
					t.Fatalf("decode patch: %v", err)
				}
				if !containsString(response.Saved, change.key) {
					t.Fatalf("saved = %v, want %s", response.Saved, change.key)
				}
				var chat models.Chat
				if err := db.DB.Where("chat_id = ?", chatID).Take(&chat).Error; err != nil {
					t.Fatalf("load captcha chat: %v", err)
				}
				if title := fmt.Sprintf("Panel Chat %d", chatID); chat.ChatName != title {
					t.Fatalf("captcha chat name = %q, want Telegram title %q", chat.ChatName, title)
				}
			})
		}
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
	if field := findSettingsField(t, body, "timezone.name"); !field.Writable {
		t.Fatalf("read-only timezone lost its writable capability: %+v", field)
	}
	if count := countChatRows(t, &models.ChatTimezone{}, chatID); count != 0 {
		t.Fatalf("read-only GET created %d timezone rows", count)
	}
	for _, key := range []string{"nightmode.enabled", "nightmode.start", "nightmode.end", "nightmode.lock_types"} {
		if field := findSettingsField(t, body, key); !field.Writable {
			t.Fatalf("read-only %s lost its writable capability: %+v", key, field)
		}
	}
	if count := countChatRows(t, &models.NightModeSettings{}, chatID); count != 0 {
		t.Fatalf("read-only GET created %d nightmode rows", count)
	}

	write := panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), raw,
		`{"changes":{"greetings.welcome_enabled":false}}`, "application/json")
	requireAPIError(t, write, http.StatusForbidden, "not_admin")
	if count := countChatRows(t, &models.GreetingSettings{}, chatID); count != 0 {
		t.Fatalf("non-admin write created %d greeting rows", count)
	}
	warnWrite := panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), raw,
		`{"changes":{"warns.limit":7,"warns.mode":"tban","warns.action_time":60,"warns.expiry":60}}`, "application/json")
	requireAPIError(t, warnWrite, http.StatusForbidden, "not_admin")
	if count := countChatRows(t, &models.WarnSettings{}, chatID); count != 0 {
		t.Fatalf("non-admin write created %d warn settings rows", count)
	}
	digestWrite := panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), raw,
		`{"changes":{"digest.enabled":true,"digest.destination":"group"}}`, "application/json")
	requireAPIError(t, digestWrite, http.StatusForbidden, "not_admin")
	if count := countChatRows(t, &models.DigestSettings{}, chatID); count != 0 {
		t.Fatalf("non-admin write created %d digest settings rows", count)
	}
	for _, changes := range []string{`{"reports.enabled":false}`, `{"reports.mute_threshold":2}`} {
		reportWrite := panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), raw,
			`{"changes":`+changes+`}`, "application/json")
		requireAPIError(t, reportWrite, http.StatusForbidden, "not_admin")
		if count := countChatRows(t, &models.ReportChatSettings{}, chatID); count != 0 {
			t.Fatalf("non-admin write created %d report settings rows", count)
		}
	}
	timezoneWrite := panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), raw,
		`{"changes":{"timezone.name":"Asia/Kolkata"}}`, "application/json")
	requireAPIError(t, timezoneWrite, http.StatusForbidden, "not_admin")
	if count := countChatRows(t, &models.ChatTimezone{}, chatID); count != 0 {
		t.Fatalf("non-admin write created %d timezone rows", count)
	}
	nightWrite := panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), raw,
		`{"changes":{"nightmode.enabled":true,"nightmode.start":"23:00","nightmode.end":"07:00","nightmode.lock_types":"media,url"}}`, "application/json")
	requireAPIError(t, nightWrite, http.StatusForbidden, "not_admin")
	if count := countChatRows(t, &models.NightModeSettings{}, chatID); count != 0 {
		t.Fatalf("non-admin write created %d nightmode rows", count)
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
	if field := findSettingsField(t, body, "timezone.name"); !field.Writable {
		t.Fatalf("read-only timezone lost its writable capability: %+v", field)
	}
	if count := countChatRows(t, &models.ChatTimezone{}, chatID); count != 0 {
		t.Fatalf("read-only GET created %d timezone rows", count)
	}
	for _, key := range []string{"nightmode.enabled", "nightmode.start", "nightmode.end", "nightmode.lock_types"} {
		if field := findSettingsField(t, body, key); !field.Writable {
			t.Fatalf("read-only %s lost its writable capability: %+v", key, field)
		}
	}
	if count := countChatRows(t, &models.NightModeSettings{}, chatID); count != 0 {
		t.Fatalf("read-only GET created %d nightmode rows", count)
	}

	write := panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), raw,
		`{"changes":{"captcha.enabled":true}}`, "application/json")
	requireAPIError(t, write, http.StatusConflict, "bot_not_admin")
	if count := countChatRows(t, &models.CaptchaSettings{}, chatID); count != 0 {
		t.Fatalf("refused write created %d captcha rows", count)
	}
	warnWrite := panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), raw,
		`{"changes":{"warns.limit":7,"warns.mode":"tmute","warns.action_time":60,"warns.expiry":60}}`, "application/json")
	requireAPIError(t, warnWrite, http.StatusConflict, "bot_not_admin")
	if count := countChatRows(t, &models.WarnSettings{}, chatID); count != 0 {
		t.Fatalf("refused write created %d warn settings rows", count)
	}
	digestWrite := panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), raw,
		`{"changes":{"digest.enabled":true,"digest.destination":"log"}}`, "application/json")
	requireAPIError(t, digestWrite, http.StatusConflict, "bot_not_admin")
	if count := countChatRows(t, &models.DigestSettings{}, chatID); count != 0 {
		t.Fatalf("refused write created %d digest settings rows", count)
	}
	for _, changes := range []string{`{"reports.enabled":false}`, `{"reports.mute_threshold":2}`} {
		reportWrite := panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), raw,
			`{"changes":`+changes+`}`, "application/json")
		requireAPIError(t, reportWrite, http.StatusConflict, "bot_not_admin")
		if count := countChatRows(t, &models.ReportChatSettings{}, chatID); count != 0 {
			t.Fatalf("refused write created %d report settings rows", count)
		}
	}
	timezoneWrite := panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), raw,
		`{"changes":{"timezone.name":"Asia/Kolkata"}}`, "application/json")
	requireAPIError(t, timezoneWrite, http.StatusConflict, "bot_not_admin")
	if count := countChatRows(t, &models.ChatTimezone{}, chatID); count != 0 {
		t.Fatalf("refused write created %d timezone rows", count)
	}
	nightWrite := panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), raw,
		`{"changes":{"nightmode.enabled":true,"nightmode.start":"23:00","nightmode.end":"07:00","nightmode.lock_types":"media,url"}}`, "application/json")
	requireAPIError(t, nightWrite, http.StatusConflict, "bot_not_admin")
	if count := countChatRows(t, &models.NightModeSettings{}, chatID); count != 0 {
		t.Fatalf("refused write created %d nightmode rows", count)
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

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
