//go:build testtools

package panel

import (
	"context"
	"net/http"
	"testing"

	"github.com/divkix/Alita_Robot/alita/db/captcha"
	"github.com/divkix/Alita_Robot/alita/db/greetings"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

// A greeting toggle needs the caller's can_change_info, the right /welcome
// checks; without it the panel must refuse and write nothing.
func TestSettingsPatchGreetingsNeedChangeInfo(t *testing.T) {
	fixture, chatID := newSettingsFixture(t)
	raw := readFixture(t, "initdata_main.txt")
	delete(fixture.client.rights, settingsFixtureUserID)

	recorder := panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), raw,
		`{"changes":{"greetings.welcome_enabled":false}}`, "application/json")
	requireAPIError(t, recorder, http.StatusForbidden, "missing_permission")
	if count := countChatRows(t, &models.GreetingSettings{}, chatID); count != 0 {
		t.Fatalf("refused greeting write created %d rows", count)
	}

	fixture.client.rights[settingsFixtureUserID] = panelFakeRights{canChangeInfo: true}
	recorder = panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), raw,
		`{"changes":{"greetings.welcome_enabled":false}}`, "application/json")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
	}
	if greetings.GetGreetingSettings(chatID).WelcomeSettings.ShouldWelcome {
		t.Fatal("greeting write with can_change_info did not apply")
	}
}

// Captcha restricts members, so it needs the caller's can_restrict_members.
func TestSettingsPatchCaptchaNeedsCallerRestrict(t *testing.T) {
	fixture, chatID := newSettingsFixture(t)
	raw := readFixture(t, "initdata_main.txt")
	delete(fixture.client.rights, settingsFixtureUserID)

	recorder := panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), raw,
		`{"changes":{"captcha.enabled":true}}`, "application/json")
	requireAPIError(t, recorder, http.StatusForbidden, "missing_permission")
	if count := countChatRows(t, &models.CaptchaSettings{}, chatID); count != 0 {
		t.Fatalf("refused captcha write created %d rows", count)
	}
}

// A module that restricts members is refused while the bot cannot restrict,
// and succeeds once it can.
func TestSettingsPatchCaptchaNeedsBotRestrict(t *testing.T) {
	fixture, chatID := newSettingsFixture(t)
	raw := readFixture(t, "initdata_main.txt")
	fixture.client.botRights.canRestrictMembers = false

	recorder := panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), raw,
		`{"changes":{"captcha.enabled":true}}`, "application/json")
	requireAPIError(t, recorder, http.StatusConflict, "bot_missing_permission")
	if count := countChatRows(t, &models.CaptchaSettings{}, chatID); count != 0 {
		t.Fatalf("refused captcha write created %d rows", count)
	}

	fixture.client.botRights.canRestrictMembers = true
	recorder = panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), raw,
		`{"changes":{"captcha.enabled":true}}`, "application/json")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
	}
	settings, err := captcha.GetCaptchaSettings(chatID)
	if err != nil {
		t.Fatalf("captcha settings after patch: %v", err)
	}
	if !settings.Enabled {
		t.Fatal("captcha stayed off after a permitted write")
	}
}

// The batch's strongest requirement gates the whole batch: a permitted
// language change is not written when a greeting change beside it is refused.
func TestSettingsPatchPermissionRefusalWritesNothing(t *testing.T) {
	fixture, chatID := newSettingsFixture(t)
	raw := readFixture(t, "initdata_main.txt")
	delete(fixture.client.rights, settingsFixtureUserID)

	recorder := panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), raw,
		`{"changes":{"language.code":"es","greetings.clean_welcome":true}}`, "application/json")
	requireAPIError(t, recorder, http.StatusForbidden, "missing_permission")
	if got := lang.GetGroupLanguage(context.Background(), chatID); got != "en" {
		t.Fatalf("language after refused batch = %q, want en", got)
	}
	if count := countChatRows(t, &models.GreetingSettings{}, chatID); count != 0 {
		t.Fatalf("refused batch created %d greeting rows", count)
	}
}

// The chat creator holds every right even when getChatMember reports none.
func TestSettingsPatchCreatorSkipsRights(t *testing.T) {
	fixture, chatID := newSettingsFixture(t)
	raw := readFixture(t, "initdata_main.txt")
	delete(fixture.client.rights, settingsFixtureUserID)
	fixture.client.statuses[settingsFixtureUserID] = "creator"

	recorder := panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), raw,
		`{"changes":{"greetings.welcome_enabled":false}}`, "application/json")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
	}
	if greetings.GetGreetingSettings(chatID).WelcomeSettings.ShouldWelcome {
		t.Fatal("creator greeting write did not apply")
	}
}
