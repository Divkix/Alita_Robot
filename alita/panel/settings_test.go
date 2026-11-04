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
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	_ "time/tzdata"

	"github.com/PaulSonOfLars/gotgbot/v2"

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
	"github.com/divkix/Alita_Robot/alita/db/logchannels"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/nightmode"
	"github.com/divkix/Alita_Robot/alita/db/reports"
	"github.com/divkix/Alita_Robot/alita/db/rules"
	"github.com/divkix/Alita_Robot/alita/db/spamdbsettings"
	"github.com/divkix/Alita_Robot/alita/db/warns"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

const settingsFixtureUserID = 279058397

func newSettingsFixture(t *testing.T) (panelFixture, int64) {
	t.Helper()
	fixture := newPanelFixture(t)
	fixture.panel.deps.NightModeLockTypes = []string{"all", "media", "photo", "url"}
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

	wantModules := []string{"language", "greetings", "rules", "captcha", "antiflood", "antispam", "antiraid", "warns", "digest", "reports", "timezone", "nightmode", "locks", "log_channel"}
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
		fieldInt(t, body, "antiflood.action_time") != 0 || fieldBool(t, body, "antiflood.delete_messages") {
		t.Fatalf("antiflood defaults = %+v", body.Modules[4])
	}
	if options := findSettingsField(t, body, "antiflood.action").Options; !slices.Equal(options, []string{"ban", "kick", "mute", "tban", "tmute"}) {
		t.Fatalf("antiflood action options = %v, want ban/kick/mute/tban/tmute", options)
	}
	actionTime := findSettingsField(t, body, "antiflood.action_time")
	if actionTime.Type != "int" || !actionTime.Writable || actionTime.Default != float64(0) || actionTime.Changed {
		t.Fatalf("antiflood.action_time metadata = %+v, want writable int with unchanged default 0", actionTime)
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
	if fieldInt(t, body, "warns.limit") != 3 || fieldString(t, body, "warns.mode") != "mute" ||
		fieldInt(t, body, "warns.action_time") != 0 || fieldInt(t, body, "warns.expiry") != 0 {
		t.Fatalf("warns defaults = %+v", body.Modules[7])
	}
	for _, key := range []string{"warns.limit", "warns.mode", "warns.action_time", "warns.expiry"} {
		field := findSettingsField(t, body, key)
		if !field.Writable || field.Changed || field.Default != field.Value {
			t.Fatalf("%s metadata = %+v, want writable and unchanged at its default", key, field)
		}
	}
	options := findSettingsField(t, body, "warns.mode").Options
	if len(options) != 5 {
		t.Fatalf("warn modes = %v, want five supported actions", options)
	}
	for _, mode := range []string{"ban", "kick", "mute", "tban", "tmute"} {
		if !containsString(options, mode) {
			t.Fatalf("warn modes = %v, missing %q", options, mode)
		}
	}
	if fieldBool(t, body, "digest.enabled") || fieldString(t, body, "digest.destination") != "" {
		t.Fatalf("digest defaults = %+v", body.Modules[8])
	}
	for _, key := range []string{"digest.enabled", "digest.destination"} {
		field := findSettingsField(t, body, key)
		if !field.Writable || field.Changed || field.Default != field.Value {
			t.Fatalf("%s metadata = %+v, want writable and unchanged at its default", key, field)
		}
	}
	destinationOptions := findSettingsField(t, body, "digest.destination").Options
	if len(destinationOptions) != 3 || destinationOptions[0] != "" || destinationOptions[1] != "log" || destinationOptions[2] != "group" {
		t.Fatalf("digest destinations = %v, want default, log and group", destinationOptions)
	}
	if !fieldBool(t, body, "reports.enabled") || fieldInt(t, body, "reports.mute_threshold") != 0 {
		t.Fatalf("reports defaults = %+v, want enabled with auto-mute off", body.Modules[9])
	}
	for _, key := range []string{"reports.enabled", "reports.mute_threshold"} {
		field := findSettingsField(t, body, key)
		if !field.Writable || field.Changed || field.Default != field.Value {
			t.Fatalf("%s metadata = %+v, want writable and unchanged at its default", key, field)
		}
	}
	timezone := findSettingsField(t, body, "timezone.name")
	if timezone.Type != "string" || timezone.Value != "UTC" || timezone.Default != "UTC" ||
		!timezone.Writable || timezone.Changed || len(timezone.Options) != 0 {
		t.Fatalf("timezone.name metadata = %+v, want writable text at its UTC default", timezone)
	}
	for key, want := range map[string]any{
		"nightmode.enabled":    false,
		"nightmode.start":      "00:00",
		"nightmode.end":        "00:00",
		"nightmode.lock_types": "",
	} {
		field := findSettingsField(t, body, key)
		wantType := "string"
		if key == "nightmode.enabled" {
			wantType = "bool"
		}
		if field.Type != wantType || field.Value != want || field.Default != want ||
			!field.Writable || field.Changed || len(field.Options) != 0 {
			t.Fatalf("%s metadata = %+v, want writable %s at default %v", key, field, wantType, want)
		}
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
		"warns":       &models.WarnSettings{},
		"digest":      &models.DigestSettings{},
		"reports":     &models.ReportChatSettings{},
		"timezone":    &models.ChatTimezone{},
		"nightmode":   &models.NightModeSettings{},
		"warn_users":  &models.Warns{},
		"warn_entry":  &models.WarnEntry{},
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
	raw := readFixture(t, "initdata_main.txt")

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
		{name: "flood tban missing duration", body: `{"changes":{"antiflood.action":"tban","greetings.welcome_enabled":false}}`, wantKeys: []string{"antiflood.action"}},
		{name: "flood tmute missing duration", body: `{"changes":{"antiflood.action":"tmute","greetings.welcome_enabled":false}}`, wantKeys: []string{"antiflood.action"}},
		{name: "flood zero duration", body: `{"changes":{"antiflood.action":"tban","antiflood.action_time":0,"greetings.welcome_enabled":false}}`, wantKeys: []string{"antiflood.action_time"}},
		{name: "flood negative duration", body: `{"changes":{"antiflood.action":"tban","antiflood.action_time":-1,"greetings.welcome_enabled":false}}`, wantKeys: []string{"antiflood.action_time"}},
		{name: "flood duration below minimum", body: `{"changes":{"antiflood.action":"tmute","antiflood.action_time":59,"greetings.welcome_enabled":false}}`, wantKeys: []string{"antiflood.action_time"}},
		{name: "flood duration above maximum", body: `{"changes":{"antiflood.action":"tban","antiflood.action_time":31622401,"greetings.welcome_enabled":false}}`, wantKeys: []string{"antiflood.action_time"}},
		{name: "flood fractional duration", body: `{"changes":{"antiflood.action":"tmute","antiflood.action_time":60.5,"greetings.welcome_enabled":false}}`, wantKeys: []string{"antiflood.action_time"}},
		{name: "flood string duration", body: `{"changes":{"antiflood.action":"tmute","antiflood.action_time":"60","greetings.welcome_enabled":false}}`, wantKeys: []string{"antiflood.action_time"}},
		{name: "flood null duration", body: `{"changes":{"antiflood.action":"tmute","antiflood.action_time":null,"greetings.welcome_enabled":false}}`, wantKeys: []string{"antiflood.action_time"}},
		{name: "flood duration alone while untimed", body: `{"changes":{"antiflood.action_time":3600,"greetings.welcome_enabled":false}}`, wantKeys: []string{"antiflood.action_time"}},
		{name: "flood zero duration alone while untimed", body: `{"changes":{"antiflood.action_time":0,"greetings.welcome_enabled":false}}`, wantKeys: []string{"antiflood.action_time"}},
		{name: "flood invalid duration with untimed action", body: `{"changes":{"antiflood.action":"ban","antiflood.action_time":59,"greetings.welcome_enabled":false}}`, wantKeys: []string{"antiflood.action_time"}},
		{name: "flood warn is not supported", body: `{"changes":{"antiflood.action":"warn","greetings.welcome_enabled":false}}`, wantKeys: []string{"antiflood.action"}},
		{name: "last spam database", body: `{"changes":{"antispam.spamdb_cas":false}}`, wantKeys: []string{"antispam.spamdb_cas"}},
		{name: "no spam database left", body: `{"changes":{"antispam.spamdb_cas":false,"antispam.spamdb_shared":false}}`, wantKeys: []string{"antispam.spamdb_cas", "antispam.spamdb_shared"}},
		{name: "valid beside invalid", body: `{"changes":{"greetings.welcome_enabled":false,"captcha.timeout":99}}`, wantKeys: []string{"captcha.timeout"}},
		{name: "warn limit below minimum", body: `{"changes":{"warns.limit":0}}`, wantKeys: []string{"warns.limit"}},
		{name: "warn limit above maximum", body: `{"changes":{"warns.limit":101}}`, wantKeys: []string{"warns.limit"}},
		{name: "warn limit fraction", body: `{"changes":{"warns.limit":3.5}}`, wantKeys: []string{"warns.limit"}},
		{name: "warn limit wrong type", body: `{"changes":{"warns.limit":"3"}}`, wantKeys: []string{"warns.limit"}},
		{name: "warn mode unknown", body: `{"changes":{"warns.mode":"restrict"}}`, wantKeys: []string{"warns.mode"}},
		{name: "warn mode wrong type", body: `{"changes":{"warns.mode":true}}`, wantKeys: []string{"warns.mode"}},
		{name: "warn timed ban without duration", body: `{"changes":{"warns.mode":"tban"}}`, wantKeys: []string{"warns.mode"}},
		{name: "warn timed mute without duration", body: `{"changes":{"warns.mode":"tmute"}}`, wantKeys: []string{"warns.mode"}},
		{name: "warn timed action with zero duration", body: `{"changes":{"warns.mode":"tban","warns.action_time":0}}`, wantKeys: []string{"warns.mode", "warns.action_time"}},
		{name: "warn duration below minimum", body: `{"changes":{"warns.mode":"tmute","warns.action_time":59}}`, wantKeys: []string{"warns.action_time"}},
		{name: "warn duration above maximum", body: fmt.Sprintf(`{"changes":{"warns.mode":"tban","warns.action_time":%d}}`, models.MaxActionTime+1), wantKeys: []string{"warns.action_time"}},
		{name: "warn duration wrong type", body: `{"changes":{"warns.mode":"tban","warns.action_time":"60"}}`, wantKeys: []string{"warns.action_time"}},
		{name: "warn duration alone while untimed", body: `{"changes":{"warns.action_time":60}}`, wantKeys: []string{"warns.action_time"}},
		{name: "warn zero duration alone while untimed", body: `{"changes":{"warns.action_time":0}}`, wantKeys: []string{"warns.action_time"}},
		{name: "warn duration with untimed mode", body: `{"changes":{"warns.mode":"ban","warns.action_time":60}}`, wantKeys: []string{"warns.mode", "warns.action_time"}},
		{name: "warn expiry negative", body: `{"changes":{"warns.expiry":-1}}`, wantKeys: []string{"warns.expiry"}},
		{name: "warn expiry below minimum", body: `{"changes":{"warns.expiry":59}}`, wantKeys: []string{"warns.expiry"}},
		{name: "warn expiry above maximum", body: fmt.Sprintf(`{"changes":{"warns.expiry":%d}}`, models.MaxWarnExpiry+1), wantKeys: []string{"warns.expiry"}},
		{name: "warn expiry wrong type", body: `{"changes":{"warns.expiry":false}}`, wantKeys: []string{"warns.expiry"}},
		{name: "invalid warn blocks other modules", body: `{"changes":{"greetings.welcome_enabled":false,"warns.limit":7,"warns.mode":"tban","warns.expiry":60}}`, wantKeys: []string{"warns.mode"}},
		{name: "invalid module blocks valid warns", body: `{"changes":{"captcha.timeout":99,"warns.limit":7,"warns.mode":"tban","warns.action_time":60,"warns.expiry":60}}`, wantKeys: []string{"captcha.timeout"}},
		{name: "digest enabled wrong type", body: `{"changes":{"digest.enabled":"true"}}`, wantKeys: []string{"digest.enabled"}},
		{name: "digest enabled null", body: `{"changes":{"digest.enabled":null}}`, wantKeys: []string{"digest.enabled"}},
		{name: "digest destination unknown", body: `{"changes":{"digest.destination":"default"}}`, wantKeys: []string{"digest.destination"}},
		{name: "digest destination wrong type", body: `{"changes":{"digest.destination":true}}`, wantKeys: []string{"digest.destination"}},
		{name: "digest destination null", body: `{"changes":{"digest.destination":null}}`, wantKeys: []string{"digest.destination"}},
		{name: "digest log without log channel", body: `{"changes":{"digest.destination":"log"}}`, wantKeys: []string{"digest.destination"}},
		{name: "invalid digest blocks other settings", body: `{"changes":{"greetings.welcome_enabled":false,"digest.enabled":false,"digest.destination":"default"}}`, wantKeys: []string{"digest.destination"}},
		{name: "invalid module blocks valid digest", body: `{"changes":{"captcha.timeout":99,"digest.enabled":false,"digest.destination":"group"}}`, wantKeys: []string{"captcha.timeout"}},
		{name: "reports enabled wrong type", body: `{"changes":{"reports.enabled":"true"}}`, wantKeys: []string{"reports.enabled"}},
		{name: "reports enabled null", body: `{"changes":{"reports.enabled":null}}`, wantKeys: []string{"reports.enabled"}},
		{name: "report threshold negative", body: `{"changes":{"reports.mute_threshold":-1}}`, wantKeys: []string{"reports.mute_threshold"}},
		{name: "report threshold below minimum", body: fmt.Sprintf(`{"changes":{"reports.mute_threshold":%d}}`, models.ReportMuteThresholdMin-1), wantKeys: []string{"reports.mute_threshold"}},
		{name: "report threshold above maximum", body: fmt.Sprintf(`{"changes":{"reports.mute_threshold":%d}}`, models.ReportMuteThresholdMax+1), wantKeys: []string{"reports.mute_threshold"}},
		{name: "report threshold fraction", body: `{"changes":{"reports.mute_threshold":2.5}}`, wantKeys: []string{"reports.mute_threshold"}},
		{name: "report threshold string", body: `{"changes":{"reports.mute_threshold":"2"}}`, wantKeys: []string{"reports.mute_threshold"}},
		{name: "report threshold bool", body: `{"changes":{"reports.mute_threshold":true}}`, wantKeys: []string{"reports.mute_threshold"}},
		{name: "report threshold null", body: `{"changes":{"reports.mute_threshold":null}}`, wantKeys: []string{"reports.mute_threshold"}},
		{name: "report threshold array", body: `{"changes":{"reports.mute_threshold":[]}}`, wantKeys: []string{"reports.mute_threshold"}},
		{name: "report threshold object", body: `{"changes":{"reports.mute_threshold":{}}}`, wantKeys: []string{"reports.mute_threshold"}},
		{name: "invalid report blocks other settings", body: `{"changes":{"greetings.welcome_enabled":false,"reports.enabled":false,"reports.mute_threshold":1}}`, wantKeys: []string{"reports.mute_threshold"}},
		{name: "invalid module blocks valid reports", body: `{"changes":{"captcha.timeout":99,"reports.enabled":false,"reports.mute_threshold":2}}`, wantKeys: []string{"captcha.timeout"}},
		{name: "timezone unknown", body: `{"changes":{"timezone.name":"Mars/Olympus"}}`, wantKeys: []string{"timezone.name"}},
		{name: "timezone empty", body: `{"changes":{"timezone.name":""}}`, wantKeys: []string{"timezone.name"}},
		{name: "timezone blank", body: `{"changes":{"timezone.name":"   "}}`, wantKeys: []string{"timezone.name"}},
		{name: "timezone local", body: `{"changes":{"timezone.name":"Local"}}`, wantKeys: []string{"timezone.name"}},
		{name: "timezone fixed offset", body: `{"changes":{"timezone.name":"+05:30"}}`, wantKeys: []string{"timezone.name"}},
		{name: "timezone path traversal", body: `{"changes":{"timezone.name":"../UTC"}}`, wantKeys: []string{"timezone.name"}},
		{name: "timezone number", body: `{"changes":{"timezone.name":5}}`, wantKeys: []string{"timezone.name"}},
		{name: "timezone bool", body: `{"changes":{"timezone.name":true}}`, wantKeys: []string{"timezone.name"}},
		{name: "timezone null", body: `{"changes":{"timezone.name":null}}`, wantKeys: []string{"timezone.name"}},
		{name: "timezone array", body: `{"changes":{"timezone.name":[]}}`, wantKeys: []string{"timezone.name"}},
		{name: "timezone object", body: `{"changes":{"timezone.name":{}}}`, wantKeys: []string{"timezone.name"}},
		{name: "invalid timezone blocks other settings", body: `{"changes":{"greetings.welcome_enabled":false,"timezone.name":"Mars/Olympus"}}`, wantKeys: []string{"timezone.name"}},
		{name: "invalid module blocks valid timezone", body: `{"changes":{"captcha.timeout":99,"timezone.name":"Asia/Kolkata"}}`, wantKeys: []string{"captcha.timeout"}},
		{name: "nightmode enabled string", body: `{"changes":{"nightmode.enabled":"true"}}`, wantKeys: []string{"nightmode.enabled"}},
		{name: "nightmode enabled null", body: `{"changes":{"nightmode.enabled":null}}`, wantKeys: []string{"nightmode.enabled"}},
		{name: "nightmode start after day", body: `{"changes":{"greetings.welcome_enabled":false,"nightmode.enabled":true,"nightmode.start":"24:00","nightmode.end":"07:00","nightmode.lock_types":"media"}}`, wantKeys: []string{"nightmode.start"}},
		{name: "nightmode end after day", body: `{"changes":{"greetings.welcome_enabled":false,"nightmode.enabled":true,"nightmode.start":"23:00","nightmode.end":"24:00","nightmode.lock_types":"media"}}`, wantKeys: []string{"nightmode.end"}},
		{name: "nightmode malformed start", body: `{"changes":{"greetings.welcome_enabled":false,"nightmode.enabled":true,"nightmode.start":"23:0","nightmode.end":"07:00","nightmode.lock_types":"media"}}`, wantKeys: []string{"nightmode.start"}},
		{name: "nightmode malformed end", body: `{"changes":{"greetings.welcome_enabled":false,"nightmode.enabled":true,"nightmode.start":"23:00","nightmode.end":"not-a-time","nightmode.lock_types":"media"}}`, wantKeys: []string{"nightmode.end"}},
		{name: "nightmode start wrong type", body: `{"changes":{"nightmode.enabled":true,"nightmode.start":2300,"nightmode.end":"07:00","nightmode.lock_types":"media"}}`, wantKeys: []string{"nightmode.start"}},
		{name: "nightmode end null", body: `{"changes":{"nightmode.enabled":true,"nightmode.start":"23:00","nightmode.end":null,"nightmode.lock_types":"media"}}`, wantKeys: []string{"nightmode.end"}},
		{name: "nightmode same times", body: `{"changes":{"greetings.welcome_enabled":false,"nightmode.enabled":true,"nightmode.start":"07:00","nightmode.end":"07:00","nightmode.lock_types":"media"}}`, wantKeys: []string{"nightmode.start", "nightmode.end"}},
		{name: "nightmode unknown lock type", body: `{"changes":{"greetings.welcome_enabled":false,"nightmode.enabled":true,"nightmode.start":"23:00","nightmode.end":"07:00","nightmode.lock_types":"media,nope"}}`, wantKeys: []string{"nightmode.lock_types"}},
		{name: "nightmode excluded bots", body: `{"changes":{"greetings.welcome_enabled":false,"nightmode.enabled":true,"nightmode.start":"23:00","nightmode.end":"07:00","nightmode.lock_types":"media, BOTS"}}`, wantKeys: []string{"nightmode.lock_types"}},
		{name: "nightmode lock types array", body: `{"changes":{"nightmode.enabled":true,"nightmode.start":"23:00","nightmode.end":"07:00","nightmode.lock_types":["media"]}}`, wantKeys: []string{"nightmode.lock_types"}},
		{name: "nightmode lock types null", body: `{"changes":{"nightmode.enabled":true,"nightmode.start":"23:00","nightmode.end":"07:00","nightmode.lock_types":null}}`, wantKeys: []string{"nightmode.lock_types"}},
		{name: "invalid module blocks valid nightmode", body: `{"changes":{"captcha.timeout":99,"nightmode.enabled":true,"nightmode.start":"23:00","nightmode.end":"07:00","nightmode.lock_types":"media,url"}}`, wantKeys: []string{"captcha.timeout"}},
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

func TestSettingsPatchWarnsAppliesThroughSettersAndInvalidatesCache(t *testing.T) {
	fixture, chatID := newSettingsFixture(t)
	raw := readFixture(t, "initdata_main.txt")
	target := settingsPath(chatID)

	// Each step warms the shared warns cache before writing. Separate PATCHes
	// ensure another setter cannot hide a missing cache invalidation.
	for _, test := range []struct {
		name       string
		changes    map[string]any
		limit      int
		mode       string
		actionTime int
		expiry     int
	}{
		{"minimum limit", map[string]any{"warns.limit": 1}, 1, "mute", 0, 0},
		{"maximum limit", map[string]any{"warns.limit": 100}, 100, "mute", 0, 0},
		{"ban", map[string]any{"warns.mode": "ban"}, 100, "ban", 0, 0},
		{"kick", map[string]any{"warns.mode": "kick"}, 100, "kick", 0, 0},
		{"mute", map[string]any{"warns.mode": "mute"}, 100, "mute", 0, 0},
		{"timed ban minimum", map[string]any{"warns.mode": "tban", "warns.action_time": models.MinActionTime}, 100, "tban", models.MinActionTime, 0},
		{"timed mute reuses stored duration", map[string]any{"warns.mode": "tmute"}, 100, "tmute", models.MinActionTime, 0},
		{"duration alone while timed", map[string]any{"warns.action_time": models.MaxActionTime}, 100, "tmute", models.MaxActionTime, 0},
		{"timed ban reuses stored duration", map[string]any{"warns.mode": "tban"}, 100, "tban", models.MaxActionTime, 0},
		{"ban clears duration", map[string]any{"warns.mode": "ban"}, 100, "ban", 0, 0},
		{"timed mute maximum", map[string]any{"warns.mode": "tmute", "warns.action_time": models.MaxActionTime}, 100, "tmute", models.MaxActionTime, 0},
		{"kick with explicit zero clears duration", map[string]any{"warns.mode": "kick", "warns.action_time": 0}, 100, "kick", 0, 0},
		{"timed ban", map[string]any{"warns.mode": "tban", "warns.action_time": 120}, 100, "tban", 120, 0},
		{"mute clears duration", map[string]any{"warns.mode": "mute"}, 100, "mute", 0, 0},
		{"minimum expiry", map[string]any{"warns.expiry": models.MinWarnExpiry}, 100, "mute", 0, models.MinWarnExpiry},
		{"maximum expiry", map[string]any{"warns.expiry": models.MaxWarnExpiry}, 100, "mute", 0, models.MaxWarnExpiry},
		{"disable expiry", map[string]any{"warns.expiry": 0}, 100, "mute", 0, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			warns.GetWarnSetting(chatID)
			payload, err := json.Marshal(map[string]any{"changes": test.changes})
			if err != nil {
				t.Fatalf("encode changes: %v", err)
			}
			recorder := panelRequest(t, fixture.handler, http.MethodPatch, target, raw, string(payload), "application/json")
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
			}
			var patch settingsPatchResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &patch); err != nil {
				t.Fatalf("decode patch: %v", err)
			}
			if len(patch.Saved) != len(test.changes) || len(patch.Failed) != 0 {
				t.Fatalf("saved = %v, failed = %v, want every change saved", patch.Saved, patch.Failed)
			}
			for key := range test.changes {
				if !containsString(patch.Saved, key) {
					t.Fatalf("saved = %v, missing %s", patch.Saved, key)
				}
			}

			var stored models.WarnSettings
			if err := db.DB.Where("chat_id = ?", chatID).Take(&stored).Error; err != nil {
				t.Fatalf("load warn settings: %v", err)
			}
			for source, settings := range map[string]*models.WarnSettings{"database": &stored, "getter": warns.GetWarnSetting(chatID)} {
				if settings.WarnLimit != test.limit || settings.WarnMode != test.mode ||
					settings.WarnActionTime != test.actionTime || settings.WarnExpiry != test.expiry {
					t.Fatalf("%s warns = %+v, want limit=%d mode=%s action_time=%d expiry=%d",
						source, settings, test.limit, test.mode, test.actionTime, test.expiry)
				}
			}
			after := decodeSettingsResponse(t, panelRequest(t, fixture.handler, http.MethodGet, target, raw, "", ""))
			if fieldInt(t, after, "warns.limit") != test.limit || fieldString(t, after, "warns.mode") != test.mode ||
				fieldInt(t, after, "warns.action_time") != test.actionTime || fieldInt(t, after, "warns.expiry") != test.expiry {
				t.Fatal("GET warns does not reflect the saved settings")
			}
			for _, key := range []string{"warns.limit", "warns.mode", "warns.action_time", "warns.expiry"} {
				field := findSettingsField(t, after, key)
				if field.Changed != (field.Value != field.Default) {
					t.Fatalf("%s changed = %v for value=%v default=%v", key, field.Changed, field.Value, field.Default)
				}
			}
		})
	}
}

func TestSettingsPatchWarnsValidationPreservesStoredSettings(t *testing.T) {
	for _, test := range []struct {
		name       string
		mode       string
		actionTime int
		changes    string
		errorKey   string
	}{
		{"ban rejects duration alone", "ban", 0, `{"warns.action_time":60}`, "warns.action_time"},
		{"kick rejects zero duration alone", "kick", 0, `{"warns.action_time":0}`, "warns.action_time"},
		{"mute rejects duration alone", "mute", 0, `{"warns.action_time":60}`, "warns.action_time"},
		{"timed duration cannot become zero", "tban", 120, `{"warns.limit":7,"warns.action_time":0,"warns.expiry":60}`, "warns.action_time"},
		{"timed duration cannot become too short", "tmute", 120, `{"warns.limit":7,"warns.action_time":59}`, "warns.action_time"},
		{"untimed mode rejects nonzero duration", "tban", 120, `{"warns.mode":"ban","warns.action_time":60}`, "warns.action_time"},
		{"invalid expiry blocks mode and time", "tban", 120, `{"greetings.welcome_enabled":false,"warns.limit":7,"warns.mode":"tmute","warns.action_time":180,"warns.expiry":59}`, "warns.expiry"},
		{"zero stored duration cannot enable timed mode", "ban", 0, `{"warns.limit":7,"warns.mode":"tban","warns.expiry":60}`, "warns.mode"},
		{"short stored duration cannot enable timed mode", "ban", models.MinActionTime - 1, `{"warns.limit":7,"warns.mode":"tmute","warns.expiry":60}`, "warns.mode"},
		{"long stored duration cannot enable timed mode", "ban", models.MaxActionTime + 1, `{"warns.limit":7,"warns.mode":"tban","warns.expiry":60}`, "warns.mode"},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture, chatID := newSettingsFixture(t)
			if err := warns.SetWarnLimit(chatID, 8); err != nil {
				t.Fatalf("seed warn limit: %v", err)
			}
			if err := warns.SetWarnMode(chatID, test.mode, test.actionTime); err != nil {
				t.Fatalf("seed warn mode: %v", err)
			}
			if err := warns.SetWarnExpiry(chatID, 300); err != nil {
				t.Fatalf("seed warn expiry: %v", err)
			}
			warns.GetWarnSetting(chatID)

			recorder := panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), readFixture(t, "initdata_main.txt"),
				`{"changes":`+test.changes+`}`, "application/json")
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body %s)", recorder.Code, recorder.Body.String())
			}
			var patch settingsPatchResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &patch); err != nil {
				t.Fatalf("decode patch: %v", err)
			}
			if patch.Error == nil || patch.Error.Code != "invalid_settings" || patch.Errors[test.errorKey] == "" ||
				len(patch.Saved) != 0 || len(patch.Failed) != 0 {
				t.Fatalf("patch = %+v, want validation failure for %s without writes", patch, test.errorKey)
			}
			var stored models.WarnSettings
			if err := db.DB.Where("chat_id = ?", chatID).Take(&stored).Error; err != nil {
				t.Fatalf("load warn settings: %v", err)
			}
			for source, settings := range map[string]*models.WarnSettings{"database": &stored, "getter": warns.GetWarnSetting(chatID)} {
				if settings.WarnLimit != 8 || settings.WarnMode != test.mode ||
					settings.WarnActionTime != test.actionTime || settings.WarnExpiry != 300 {
					t.Fatalf("refused PATCH changed %s warns: %+v", source, settings)
				}
			}
			if count := countChatRows(t, &models.GreetingSettings{}, chatID); count != 0 {
				t.Fatalf("refused PATCH created %d greeting rows", count)
			}
		})
	}
}

func TestSettingsPatchDigestAppliesThroughSettersAndInvalidatesCache(t *testing.T) {
	fixture, chatID := newSettingsFixture(t)
	raw := readFixture(t, "initdata_main.txt")
	target := settingsPath(chatID)
	restore := *config.AppConfig
	t.Cleanup(func() { *config.AppConfig = restore })
	config.AppConfig.EnableDigest = true
	// A log destination needs a bound log channel, as /digest log does.
	if err := logchannels.Set(chatID, "panel digest", -1_009_999_000_001); err != nil {
		t.Fatalf("bind log channel: %v", err)
	}

	// Warm the shared digest cache before each setter, so another setting's
	// write cannot hide a missing invalidation.
	for _, test := range []struct {
		name        string
		key         string
		value       any
		enabled     bool
		destination string
	}{
		{"enable", "digest.enabled", true, true, ""},
		{"log channel", "digest.destination", "log", true, "log"},
		{"group", "digest.destination", "group", true, "group"},
		{"disable preserves destination", "digest.enabled", false, false, "group"},
		{"reset destination", "digest.destination", "", false, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := digestsettings.GetContext(context.Background(), chatID); err != nil {
				t.Fatalf("warm digest cache: %v", err)
			}
			payload, err := json.Marshal(map[string]any{"changes": map[string]any{test.key: test.value}})
			if err != nil {
				t.Fatalf("encode changes: %v", err)
			}
			recorder := panelRequest(t, fixture.handler, http.MethodPatch, target, raw, string(payload), "application/json")
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
			}
			var patch settingsPatchResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &patch); err != nil {
				t.Fatalf("decode patch: %v", err)
			}
			if len(patch.Saved) != 1 || patch.Saved[0] != test.key || len(patch.Failed) != 0 {
				t.Fatalf("saved = %v, failed = %v, want %s saved", patch.Saved, patch.Failed, test.key)
			}

			var stored models.DigestSettings
			if err := db.DB.Where("chat_id = ?", chatID).Take(&stored).Error; err != nil {
				t.Fatalf("load digest settings: %v", err)
			}
			settings, err := digestsettings.GetContext(context.Background(), chatID)
			if err != nil {
				t.Fatalf("get digest settings: %v", err)
			}
			for source, settings := range map[string]*models.DigestSettings{"database": &stored, "getter": settings} {
				if settings.Enabled != test.enabled || settings.Destination != test.destination {
					t.Fatalf("%s digest = %+v, want enabled=%t destination=%q", source, settings, test.enabled, test.destination)
				}
			}
			after := decodeSettingsResponse(t, panelRequest(t, fixture.handler, http.MethodGet, target, raw, "", ""))
			if fieldBool(t, after, "digest.enabled") != test.enabled || fieldString(t, after, "digest.destination") != test.destination {
				t.Fatal("GET digest does not reflect the saved settings")
			}
			if field := findSettingsField(t, after, "digest.enabled"); field.Default != false || field.Changed != test.enabled {
				t.Fatalf("digest.enabled metadata = %+v, want default false and changed=%t", field, test.enabled)
			}
			if field := findSettingsField(t, after, "digest.destination"); field.Default != "" || field.Changed != (test.destination != "") {
				t.Fatalf("digest.destination metadata = %+v, want default empty and changed=%t", field, test.destination != "")
			}
		})
	}
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

func TestSettingsPatchReportsAppliesThroughSettersAndInvalidatesCache(t *testing.T) {
	fixture, chatID := newSettingsFixture(t)
	raw := readFixture(t, "initdata_main.txt")
	target := settingsPath(chatID)

	// Warm the shared report cache before each field's PATCH, so another
	// setter cannot hide a missing invalidation.
	for _, test := range []struct {
		name      string
		key       string
		value     any
		enabled   bool
		threshold int
	}{
		{"disable reports", "reports.enabled", false, false, 0},
		{"minimum threshold", "reports.mute_threshold", models.ReportMuteThresholdMin, false, models.ReportMuteThresholdMin},
		{"maximum threshold", "reports.mute_threshold", models.ReportMuteThresholdMax, false, models.ReportMuteThresholdMax},
		{"reset enabled preserves threshold", "reports.enabled", true, true, models.ReportMuteThresholdMax},
		{"reset threshold", "reports.mute_threshold", 0, true, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			reports.GetChatReportSettings(chatID)
			payload, err := json.Marshal(map[string]any{"changes": map[string]any{test.key: test.value}})
			if err != nil {
				t.Fatalf("encode changes: %v", err)
			}
			recorder := panelRequest(t, fixture.handler, http.MethodPatch, target, raw, string(payload), "application/json")
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
			}
			var patch settingsPatchResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &patch); err != nil {
				t.Fatalf("decode patch: %v", err)
			}
			if len(patch.Saved) != 1 || patch.Saved[0] != test.key || len(patch.Failed) != 0 {
				t.Fatalf("saved = %v, failed = %v, want %s saved", patch.Saved, patch.Failed, test.key)
			}
			var stored models.ReportChatSettings
			if err := db.DB.Where("chat_id = ?", chatID).Take(&stored).Error; err != nil {
				t.Fatalf("load report settings: %v", err)
			}
			for source, settings := range map[string]*models.ReportChatSettings{"database": &stored, "getter": reports.GetChatReportSettings(chatID)} {
				if settings.Enabled != test.enabled || settings.Status != test.enabled || settings.MuteThreshold != test.threshold {
					t.Fatalf("%s reports = %+v, want enabled/status=%t threshold=%d", source, settings, test.enabled, test.threshold)
				}
			}
			after := decodeSettingsResponse(t, panelRequest(t, fixture.handler, http.MethodGet, target, raw, "", ""))
			for source, body := range map[string]settingsResponse{"PATCH": {Modules: patch.Modules}, "GET": after} {
				if fieldBool(t, body, "reports.enabled") != test.enabled || fieldInt(t, body, "reports.mute_threshold") != test.threshold {
					t.Fatalf("%s reports does not reflect the saved settings", source)
				}
				if field := findSettingsField(t, body, "reports.enabled"); field.Default != true || field.Changed != !test.enabled {
					t.Fatalf("%s reports.enabled metadata = %+v, want default true and changed=%t", source, field, !test.enabled)
				}
				if field := findSettingsField(t, body, "reports.mute_threshold"); field.Default != float64(0) || field.Changed != (test.threshold != 0) {
					t.Fatalf("%s reports.mute_threshold metadata = %+v, want default zero and changed=%t", source, field, test.threshold != 0)
				}
			}
		})
	}
}

func TestSettingsPatchTimezoneAppliesThroughSetterAndInvalidatesCache(t *testing.T) {
	fixture, chatID := newSettingsFixture(t)
	raw := readFixture(t, "initdata_main.txt")
	target := settingsPath(chatID)
	previous := "UTC"

	for _, test := range []struct {
		name      string
		input     string
		canonical string
	}{
		{"canonical spelling", " asia/kolkata ", "Asia/Kolkata"},
		{"replace timezone", "America/New_York", "America/New_York"},
		{"reset timezone", "UTC", "UTC"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Warm the old value before every PATCH to expose a missing
			// invalidation even when replacing or resetting an existing row.
			if name, err := chattz.Name(chatID); err != nil || name != previous {
				t.Fatalf("warm timezone = %q/%v, want %q", name, err, previous)
			}
			payload, err := json.Marshal(map[string]any{"changes": map[string]any{"timezone.name": test.input}})
			if err != nil {
				t.Fatalf("encode changes: %v", err)
			}
			recorder := panelRequest(t, fixture.handler, http.MethodPatch, target, raw, string(payload), "application/json")
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
			}
			var patch settingsPatchResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &patch); err != nil {
				t.Fatalf("decode patch: %v", err)
			}
			if patch.Error != nil || len(patch.Saved) != 1 || patch.Saved[0] != "timezone.name" ||
				len(patch.Failed) != 0 || len(patch.Errors) != 0 {
				t.Fatalf("patch = %+v, want timezone.name saved without errors", patch)
			}
			var stored models.ChatTimezone
			if err := db.DB.Where("chat_id = ?", chatID).Take(&stored).Error; err != nil {
				t.Fatalf("load timezone: %v", err)
			}
			if stored.Timezone != test.canonical {
				t.Fatalf("stored timezone = %q, want %q", stored.Timezone, test.canonical)
			}
			if name, err := chattz.Name(chatID); err != nil || name != test.canonical {
				t.Fatalf("cached timezone = %q/%v, want %q", name, err, test.canonical)
			}
			after := decodeSettingsResponse(t, panelRequest(t, fixture.handler, http.MethodGet, target, raw, "", ""))
			for source, body := range map[string]settingsResponse{"PATCH": {Modules: patch.Modules}, "GET": after} {
				field := findSettingsField(t, body, "timezone.name")
				if field.Value != test.canonical || field.Default != "UTC" || !field.Writable ||
					field.Changed != (test.canonical != "UTC") {
					t.Fatalf("%s timezone.name = %+v, want %q with UTC default", source, field, test.canonical)
				}
			}
		})
		previous = test.canonical
	}
}

func TestSettingsPatchNightmodeAppliesThroughSettersAndInvalidatesCache(t *testing.T) {
	fixture, chatID := newSettingsFixture(t)
	raw := readFixture(t, "initdata_main.txt")
	target := settingsPath(chatID)

	for _, test := range []struct {
		name        string
		changes     map[string]any
		enabled     bool
		startMinute int
		endMinute   int
		types       string
	}{
		{"enable and normalize", map[string]any{
			"nightmode.enabled": true, "nightmode.start": "23:00", "nightmode.end": "07:00",
			"nightmode.lock_types": " URL,photo,MEDIA,photo, all,URL ",
		}, true, 1380, 420, "all,media,photo,url"},
		{"start at midnight", map[string]any{"nightmode.start": "00:00"}, true, 0, 420, "all,media,photo,url"},
		{"replace start", map[string]any{"nightmode.start": "23:15"}, true, 1395, 420, "all,media,photo,url"},
		{"end at midnight", map[string]any{"nightmode.end": "00:00"}, true, 1395, 0, "all,media,photo,url"},
		{"replace end", map[string]any{"nightmode.end": "06:45"}, true, 1395, 405, "all,media,photo,url"},
		{"replace types", map[string]any{"nightmode.lock_types": " URL , Photo,URL "}, true, 1395, 405, "photo,url"},
		{"disable preserves configuration", map[string]any{"nightmode.enabled": false}, false, 1395, 405, "photo,url"},
		{"re-enable stored configuration", map[string]any{"nightmode.enabled": true}, true, 1395, 405, "photo,url"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Warm before every field change: a sibling write must not mask a
			// missing cache invalidation.
			if _, err := nightmode.GetContext(context.Background(), chatID); err != nil {
				t.Fatalf("warm nightmode cache: %v", err)
			}
			payload, err := json.Marshal(map[string]any{"changes": test.changes})
			if err != nil {
				t.Fatalf("encode changes: %v", err)
			}
			recorder := panelRequest(t, fixture.handler, http.MethodPatch, target, raw, string(payload), "application/json")
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
			}
			var patch settingsPatchResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &patch); err != nil {
				t.Fatalf("decode patch: %v", err)
			}
			if patch.Error != nil || len(patch.Saved) != len(test.changes) || len(patch.Failed) != 0 || len(patch.Errors) != 0 {
				t.Fatalf("patch = %+v, want every changed nightmode field saved", patch)
			}
			for key := range test.changes {
				if !containsString(patch.Saved, key) {
					t.Fatalf("saved = %v, missing %s", patch.Saved, key)
				}
			}
			var stored models.NightModeSettings
			if err := db.DB.Where("chat_id = ?", chatID).Take(&stored).Error; err != nil {
				t.Fatalf("load nightmode: %v", err)
			}
			cached, err := nightmode.GetContext(context.Background(), chatID)
			if err != nil {
				t.Fatalf("get nightmode: %v", err)
			}
			for source, settings := range map[string]*models.NightModeSettings{"database": &stored, "getter": cached} {
				if settings.Enabled != test.enabled || settings.StartMinute != test.startMinute ||
					settings.EndMinute != test.endMinute || settings.LockTypes != test.types {
					t.Fatalf("%s nightmode = %+v, want enabled=%t window=%d-%d types=%q",
						source, settings, test.enabled, test.startMinute, test.endMinute, test.types)
				}
			}
			after := decodeSettingsResponse(t, panelRequest(t, fixture.handler, http.MethodGet, target, raw, "", ""))
			for source, body := range map[string]settingsResponse{"PATCH": {Modules: patch.Modules}, "GET": after} {
				for key, want := range map[string]any{
					"nightmode.enabled":    test.enabled,
					"nightmode.start":      fmt.Sprintf("%02d:%02d", test.startMinute/60, test.startMinute%60),
					"nightmode.end":        fmt.Sprintf("%02d:%02d", test.endMinute/60, test.endMinute%60),
					"nightmode.lock_types": test.types,
				} {
					field := findSettingsField(t, body, key)
					if field.Value != want || !field.Writable || field.Changed != (field.Default != want) {
						t.Fatalf("%s %s metadata = %+v, want value %v with correct changed state", source, key, field, want)
					}
				}
			}
		})
	}
}

func TestSettingsPatchNightmodeRefusesMissingTypesAndDisabledEdits(t *testing.T) {
	rawLocale, err := os.ReadFile(filepath.Join("..", "..", "locales", "en.yml"))
	if err != nil {
		t.Fatalf("read locales/en.yml: %v", err)
	}
	restoreI18n, err := i18n.OverrideLocalesForTest(map[string]string{"en": string(rawLocale)})
	if err != nil {
		t.Fatalf("override locales: %v", err)
	}
	t.Cleanup(restoreI18n)
	raw := readFixture(t, "initdata_main.txt")

	for _, types := range []string{"", " , , "} {
		t.Run(fmt.Sprintf("missing types %q", types), func(t *testing.T) {
			fixture, chatID := newSettingsFixture(t)
			payload := fmt.Sprintf(`{"changes":{"greetings.welcome_enabled":false,"nightmode.enabled":true,"nightmode.start":"23:00","nightmode.end":"07:00","nightmode.lock_types":%q}}`, types)
			recorder := panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), raw, payload, "application/json")
			assertFieldRefusal(t, recorder, "nightmode.enabled", "Give a start time, an end time and the lock types")
			for name, model := range map[string]any{
				"greetings": &models.GreetingSettings{},
				"nightmode": &models.NightModeSettings{},
			} {
				if count := countChatRows(t, model, chatID); count != 0 {
					t.Fatalf("refused batch wrote %d %s rows", count, name)
				}
			}
		})
	}
	t.Run("enable without stored types", func(t *testing.T) {
		fixture, chatID := newSettingsFixture(t)
		recorder := panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), raw,
			`{"changes":{"greetings.welcome_enabled":false,"nightmode.enabled":true}}`, "application/json")
		assertFieldRefusal(t, recorder, "nightmode.enabled", "Give a start time, an end time and the lock types")
		for name, model := range map[string]any{"greetings": &models.GreetingSettings{}, "nightmode": &models.NightModeSettings{}} {
			if count := countChatRows(t, model, chatID); count != 0 {
				t.Fatalf("refused batch wrote %d %s rows", count, name)
			}
		}
	})

	for _, stored := range []bool{false, true} {
		t.Run(fmt.Sprintf("off stored=%t", stored), func(t *testing.T) {
			fixture, chatID := newSettingsFixture(t)
			target := settingsPath(chatID)
			var before models.NightModeSettings
			if stored {
				if err := nightmode.Enable(chatID, 1380, 420, []string{"media", "url"}); err != nil {
					t.Fatalf("seed nightmode: %v", err)
				}
			}
			off := panelRequest(t, fixture.handler, http.MethodPatch, target, raw,
				`{"changes":{"nightmode.enabled":false}}`, "application/json")
			decodeSettingsResponse(t, off)
			if stored {
				if err := db.DB.Where("chat_id = ?", chatID).Take(&before).Error; err != nil {
					t.Fatalf("load disabled nightmode: %v", err)
				}
				if before.Enabled || before.StartMinute != 1380 || before.EndMinute != 420 || before.LockTypes != "media,url" {
					t.Fatalf("disable changed stored configuration: %+v", before)
				}
			}
			for _, test := range []struct {
				name    string
				changes string
				key     string
			}{
				{"start", `"nightmode.start":"22:00"`, "nightmode.start"},
				{"end", `"nightmode.end":"08:00"`, "nightmode.end"},
				{"types", `"nightmode.lock_types":"photo"`, "nightmode.lock_types"},
				{"explicit off", `"nightmode.enabled":false,"nightmode.start":"22:00"`, "nightmode.start"},
				{"transient enable then disable", `"nightmode.enabled":true,"nightmode.start":"22:00","nightmode.enabled":false`, "nightmode.start"},
			} {
				t.Run(test.name, func(t *testing.T) {
					recorder := panelRequest(t, fixture.handler, http.MethodPatch, target, raw,
						`{"changes":{"greetings.welcome_enabled":false,`+test.changes+`}}`, "application/json")
					requireAPIError(t, recorder, http.StatusBadRequest, "invalid_settings")
					var patch settingsPatchResponse
					if err := json.Unmarshal(recorder.Body.Bytes(), &patch); err != nil {
						t.Fatalf("decode refusal: %v", err)
					}
					if patch.Errors[test.key] == "" || len(patch.Saved) != 0 || len(patch.Failed) != 0 {
						t.Fatalf("patch = %+v, want %s refused before writes", patch, test.key)
					}
					if count := countChatRows(t, &models.GreetingSettings{}, chatID); count != 0 {
						t.Fatalf("refused batch wrote %d greetings rows", count)
					}
					if stored {
						var after models.NightModeSettings
						if err := db.DB.Where("chat_id = ?", chatID).Take(&after).Error; err != nil {
							t.Fatalf("load nightmode after refusal: %v", err)
						}
						if after != before {
							t.Fatalf("refused batch changed nightmode from %+v to %+v", before, after)
						}
					} else if count := countChatRows(t, &models.NightModeSettings{}, chatID); count != 0 {
						t.Fatalf("refused batch wrote %d nightmode rows", count)
					}
				})
			}
		})
	}
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
		"antiflood.action":"tban",
		"antiflood.action_time":7200,
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
	if len(patch.Saved) != 21 || len(patch.Failed) != 0 {
		t.Fatalf("saved = %v, failed = %v; want all 21 saved", patch.Saved, patch.Failed)
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
	if flood.Limit != 5 || flood.Action != "tban" || flood.FloodActionTime != 7200 || !flood.DeleteAntifloodMessage {
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
	if fieldString(t, after, "antiflood.action") != "tban" || fieldInt(t, after, "antiflood.action_time") != 7200 ||
		!findSettingsField(t, after, "antiflood.action_time").Changed {
		t.Fatalf("GET antiflood did not reflect the timed action: %+v", after.Modules[4])
	}
}

func TestSettingsPatchAntifloodDurationPersistence(t *testing.T) {
	for _, test := range []struct {
		name       string
		action     string
		actionTime int
		changes    string
		wantAction string
		wantTime   int
		wantSaved  []string
	}{
		{name: "reuse stored duration", action: "tban", actionTime: 5400, changes: `{"antiflood.action":"tmute"}`, wantAction: "tmute", wantTime: 5400, wantSaved: []string{"antiflood.action"}},
		{name: "duration alone while timed", action: "tmute", actionTime: 5400, changes: `{"antiflood.action_time":3600}`, wantAction: "tmute", wantTime: 3600, wantSaved: []string{"antiflood.action_time"}},
		{name: "minimum duration", action: "mute", changes: fmt.Sprintf(`{"antiflood.action":"tban","antiflood.action_time":%d}`, models.MinActionTime), wantAction: "tban", wantTime: models.MinActionTime, wantSaved: []string{"antiflood.action", "antiflood.action_time"}},
		{name: "maximum duration", action: "mute", changes: fmt.Sprintf(`{"antiflood.action":"tmute","antiflood.action_time":%d}`, models.MaxActionTime), wantAction: "tmute", wantTime: models.MaxActionTime, wantSaved: []string{"antiflood.action", "antiflood.action_time"}},
		{name: "clear for ban", action: "tban", actionTime: 5400, changes: `{"antiflood.action":"ban"}`, wantAction: "ban", wantSaved: []string{"antiflood.action"}},
		{name: "clear for mute", action: "tmute", actionTime: 5400, changes: `{"antiflood.action":"mute"}`, wantAction: "mute", wantSaved: []string{"antiflood.action"}},
		{name: "clear for kick", action: "tban", actionTime: 5400, changes: `{"antiflood.action":"kick"}`, wantAction: "kick", wantSaved: []string{"antiflood.action"}},
		{name: "untimed clears submitted duration", action: "tmute", actionTime: 5400, changes: `{"antiflood.action":"ban","antiflood.action_time":7200}`, wantAction: "ban", wantSaved: []string{"antiflood.action", "antiflood.action_time"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture, chatID := newSettingsFixture(t)
			if err := antiflood.SetFloodMode(chatID, test.action, test.actionTime); err != nil {
				t.Fatalf("seed flood action: %v", err)
			}
			antiflood.GetFlood(chatID)
			raw := readFixture(t, "initdata_main.txt")
			recorder := panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), raw,
				`{"changes":`+test.changes+`}`, "application/json")
			if recorder.Code != http.StatusOK {
				t.Fatalf("PATCH status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
			}
			var response settingsPatchResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode patch: %v", err)
			}
			if !slices.Equal(response.Saved, test.wantSaved) || len(response.Failed) != 0 {
				t.Fatalf("saved = %v, failed = %v, want saved %v", response.Saved, response.Failed, test.wantSaved)
			}
			var stored models.AntifloodSettings
			if err := db.DB.Where("chat_id = ?", chatID).Take(&stored).Error; err != nil {
				t.Fatalf("load flood settings: %v", err)
			}
			if stored.Action != test.wantAction || stored.FloodActionTime != test.wantTime {
				t.Fatalf("stored flood action = %s/%d, want %s/%d", stored.Action, stored.FloodActionTime, test.wantAction, test.wantTime)
			}
			cached := antiflood.GetFlood(chatID)
			if cached.Action != test.wantAction || cached.FloodActionTime != test.wantTime {
				t.Fatalf("cached flood action = %s/%d, want %s/%d", cached.Action, cached.FloodActionTime, test.wantAction, test.wantTime)
			}
			after := decodeSettingsResponse(t, panelRequest(t, fixture.handler, http.MethodGet, settingsPath(chatID), raw, "", ""))
			if fieldString(t, after, "antiflood.action") != test.wantAction || fieldInt(t, after, "antiflood.action_time") != test.wantTime {
				t.Fatalf("GET flood action did not reflect the patch: %+v", after.Modules[4])
			}
		})
	}
}

func TestSettingsPatchAntifloodRejectsUnusableStoredDuration(t *testing.T) {
	for _, actionTime := range []int{0, models.MinActionTime - 1, models.MaxActionTime + 1} {
		t.Run(strconv.Itoa(actionTime), func(t *testing.T) {
			fixture, chatID := newSettingsFixture(t)
			legacy := models.AntifloodSettings{ChatId: chatID, Action: "tban", FloodActionTime: actionTime}
			if err := db.DB.Create(&legacy).Error; err != nil {
				t.Fatalf("seed legacy flood action: %v", err)
			}
			antiflood.GetFlood(chatID)
			recorder := panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), readFixture(t, "initdata_main.txt"),
				`{"changes":{"antiflood.action":"tmute","greetings.welcome_enabled":false}}`, "application/json")
			assertFieldRefusal(t, recorder, "antiflood.action", "require a duration")
			var stored models.AntifloodSettings
			if err := db.DB.Where("chat_id = ?", chatID).Take(&stored).Error; err != nil {
				t.Fatalf("load flood settings: %v", err)
			}
			if stored.Action != "tban" || stored.FloodActionTime != actionTime {
				t.Fatalf("refused patch changed flood action to %s/%d", stored.Action, stored.FloodActionTime)
			}
			if count := countChatRows(t, &models.GreetingSettings{}, chatID); count != 0 {
				t.Fatalf("refused patch wrote %d greeting rows", count)
			}
		})
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
	fixture.panel.deps.NightModeLockTypes = []string{"all", "media", "photo", "url"}
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
	if field := findSettingsField(t, body, "timezone.name"); field.Value != "UTC" || !field.Writable {
		t.Fatalf("read-only timezone = %+v, want UTC with writable capability retained", field)
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
	fixture.panel.deps.NightModeLockTypes = []string{"all", "media", "photo", "url"}
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
	if field := findSettingsField(t, body, "timezone.name"); field.Value != "UTC" || !field.Writable {
		t.Fatalf("read-only timezone = %+v, want UTC with writable capability retained", field)
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
	config.AppConfig.EnableDigest = false
	for _, missingConfig := range []bool{false, true} {
		t.Run(fmt.Sprintf("digest/config_missing=%t", missingConfig), func(t *testing.T) {
			fixture, chatID := newSettingsFixture(t)
			target := settingsPath(chatID)
			if missingConfig {
				previous := config.AppConfig
				config.AppConfig = nil
				t.Cleanup(func() { config.AppConfig = previous })
			}
			refused := panelRequest(t, fixture.handler, http.MethodPatch, target, raw,
				`{"changes":{"greetings.welcome_enabled":false,"digest.enabled":true,"digest.destination":"log"}}`, "application/json")
			assertFieldRefusal(t, refused, "digest.enabled", "The digest is disabled by the bot operator.")
			for name, model := range map[string]any{
				"greetings": &models.GreetingSettings{},
				"digest":    &models.DigestSettings{},
			} {
				if count := countChatRows(t, model, chatID); count != 0 {
					t.Fatalf("refused digest batch created %d %s rows", count, name)
				}
			}

			// The operator's kill switch blocks enabling, not disabling or
			// choosing where a future digest should go.
			if err := digestsettings.SetEnabled(chatID, true); err != nil {
				t.Fatalf("seed digest: %v", err)
			}
			off := panelRequest(t, fixture.handler, http.MethodPatch, target, raw,
				`{"changes":{"digest.enabled":false,"digest.destination":"group"}}`, "application/json")
			if off.Code != http.StatusOK {
				t.Fatalf("disable digest status = %d, want 200 (body %s)", off.Code, off.Body.String())
			}
			settings, err := digestsettings.GetContext(context.Background(), chatID)
			if err != nil {
				t.Fatalf("get disabled digest settings: %v", err)
			}
			if settings.Enabled || settings.Destination != "group" {
				t.Fatalf("digest after disabling = %+v, want disabled with group destination", settings)
			}
			after := decodeSettingsResponse(t, panelRequest(t, fixture.handler, http.MethodGet, target, raw, "", ""))
			if fieldBool(t, after, "digest.enabled") || fieldString(t, after, "digest.destination") != "group" {
				t.Fatal("globally disabled GET digest does not reflect saved settings")
			}
		})
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
	if err := captcha.SetCaptchaEnabled(chatID, "Captcha Chat", true); err != nil {
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

	if err := captcha.SetCaptchaEnabled(chatID, "Captcha Chat", true); err != nil {
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
