//go:build testtools

package panel

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/divkix/Alita_Robot/alita/db/captcha"
	"github.com/divkix/Alita_Robot/alita/db/chattz"
	"github.com/divkix/Alita_Robot/alita/db/greetings"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/reports"
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

// /timezone needs the caller's can_change_info; a refusal must leave even
// admin-only settings in the same batch untouched.
func TestSettingsPatchTimezoneNeedsChangeInfo(t *testing.T) {
	fixture, chatID := newSettingsFixture(t)
	raw := readFixture(t, "initdata_main.txt")
	delete(fixture.client.rights, settingsFixtureUserID)
	changes := `{"changes":{"language.code":"es","timezone.name":"asia/kolkata"}}`

	recorder := panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), raw,
		changes, "application/json")
	requireAPIError(t, recorder, http.StatusForbidden, "missing_permission")
	if count := countChatRows(t, &models.ChatTimezone{}, chatID); count != 0 {
		t.Fatalf("refused timezone write created %d rows", count)
	}
	if got := lang.GetGroupLanguage(context.Background(), chatID); got != "en" {
		t.Fatalf("language after refused timezone batch = %q, want en", got)
	}

	fixture.client.rights[settingsFixtureUserID] = panelFakeRights{canChangeInfo: true}
	recorder = panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), raw,
		changes, "application/json")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
	}
	if name, err := chattz.Name(chatID); err != nil || name != "Asia/Kolkata" {
		t.Fatalf("timezone after permitted write = %q/%v, want Asia/Kolkata", name, err)
	}
	if got := lang.GetGroupLanguage(context.Background(), chatID); got != "es" {
		t.Fatalf("language after permitted timezone batch = %q, want es", got)
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

// /reports on/off needs an admin, not can_restrict_members on either member.
func TestSettingsPatchReportEnabledOnlyNeedsAdmin(t *testing.T) {
	fixture, chatID := newSettingsFixture(t)
	raw := readFixture(t, "initdata_main.txt")
	delete(fixture.client.rights, settingsFixtureUserID)
	fixture.client.botRights.canRestrictMembers = false

	for _, enabled := range []bool{false, true} {
		recorder := panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), raw,
			fmt.Sprintf(`{"changes":{"reports.enabled":%t}}`, enabled), "application/json")
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
		}
		settings := reports.GetChatReportSettings(chatID)
		if settings.Enabled != enabled || settings.Status != enabled {
			t.Fatalf("reports after admin-only write = %+v, want enabled/status=%t", settings, enabled)
		}
	}
}

// Both setting and resetting the auto-mute threshold need caller and bot
// can_restrict_members; refusal prevents every change in a mixed batch.
func TestSettingsPatchReportThresholdNeedsCallerAndBotRestrict(t *testing.T) {
	for _, test := range []struct {
		name           string
		callerRestrict bool
		botRestrict    bool
		threshold      int
		status         int
		code           string
	}{
		{"caller cannot enable", false, true, 2, http.StatusForbidden, "missing_permission"},
		{"bot cannot enable", true, false, 2, http.StatusConflict, "bot_missing_permission"},
		{"caller cannot reset", false, true, 0, http.StatusForbidden, "missing_permission"},
		{"bot cannot reset", true, false, 0, http.StatusConflict, "bot_missing_permission"},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture, chatID := newSettingsFixture(t)
			fixture.client.rights[settingsFixtureUserID] = panelFakeRights{canRestrictMembers: test.callerRestrict}
			fixture.client.botRights.canRestrictMembers = test.botRestrict

			recorder := panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), readFixture(t, "initdata_main.txt"),
				fmt.Sprintf(`{"changes":{"language.code":"es","reports.enabled":false,"reports.mute_threshold":%d}}`, test.threshold),
				"application/json")
			requireAPIError(t, recorder, test.status, test.code)
			if count := countChatRows(t, &models.ReportChatSettings{}, chatID); count != 0 {
				t.Fatalf("refused batch created %d report settings rows", count)
			}
			if got := lang.GetGroupLanguage(context.Background(), chatID); got != "en" {
				t.Fatalf("language after refused batch = %q, want en", got)
			}
		})
	}
}
