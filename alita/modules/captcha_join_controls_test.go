//go:build testtools

package modules

import (
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/captcha"
	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

func TestCaptchaJoinCommandShowsStateAndStaysIndependent(t *testing.T) {
	useRealLocales(t)
	for _, regularEnabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("regular=%t", regularEnabled), func(t *testing.T) {
			f := newAdminChatFixture(t)
			if err := captcha.SetCaptchaEnabled(f.chat.Id, f.chat.Title, regularEnabled); err != nil {
				t.Fatalf("SetCaptchaEnabled() error = %v", err)
			}
			for _, step := range []struct {
				arg, replyKey string
				joinEnabled   bool
			}{
				{"", "captcha_join_requests_current", false},
				{" on", "captcha_join_requests_enabled", true},
				{"", "captcha_join_requests_current", true},
				{" maybe", "captcha_join_requests_usage", true},
				{" off", "captcha_join_requests_disabled", false},
				{"", "captcha_join_requests_current", false},
			} {
				ctx := newModuleMessageContext(f.bot, f.chat, f.admin, "/captchajoin"+step.arg)
				if err := captchaModule.captchaJoinCommand(f.bot, ctx); err != nil {
					t.Fatalf("captchajoin%q error = %v", step.arg, err)
				}
				settings, err := captcha.GetCaptchaSettings(f.chat.Id)
				if err != nil || settings == nil {
					t.Fatalf("GetCaptchaSettings() = (%+v, %v)", settings, err)
				}
				if settings.JoinRequests != step.joinEnabled || settings.Enabled != regularEnabled {
					t.Fatalf("captchajoin%q saved %+v, want join=%t regular=%t", step.arg, settings, step.joinEnabled, regularEnabled)
				}
				statusKey := "captcha_status_disabled"
				if step.joinEnabled {
					statusKey = "captcha_status_enabled"
				}
				want := translatedWith(t, step.replyKey, i18n.TranslationParams{"status": translated(t, statusKey)})
				if text := fmt.Sprint(lastCall(t, f.client, "sendMessage").Params["text"]); text != want {
					t.Fatalf("captchajoin%q reply = %q, want %q", step.arg, text, want)
				}
			}
			assertCaptchaChatTitle(t, f.chat)
		})
	}
}

func TestCaptchaStatusIncludesJoinRequestSetting(t *testing.T) {
	useRealLocales(t)
	f := newAdminChatFixture(t)
	if err := captcha.SetCaptchaJoinRequests(f.chat.Id, f.chat.Title, true); err != nil {
		t.Fatalf("SetCaptchaJoinRequests() error = %v", err)
	}
	ctx := newModuleMessageContext(f.bot, f.chat, f.admin, "/captcha")
	if err := captchaModule.captchaCommand(f.bot, ctx); err != nil {
		t.Fatalf("captchaCommand() error = %v", err)
	}
	want := translatedWith(t, "captcha_settings_join_requests", i18n.TranslationParams{"status": translated(t, "captcha_status_enabled")})
	if text := fmt.Sprint(lastCall(t, f.client, "sendMessage").Params["text"]); !strings.Contains(text, want) {
		t.Fatalf("captcha status = %q, want join-request line %q", text, want)
	}
}

func TestCaptchaJoinCommandInPMTargetsNamedGroupAndRefusesMember(t *testing.T) {
	f, connected := pmSettingsFixture(t)
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadCaptcha(dispatcher)

	processPM(t, dispatcher, f, f.admin.Id, "/captchajoin @fixturegroup on")
	settings, err := captcha.GetCaptchaSettings(f.chat.Id)
	if err != nil || settings == nil || !settings.JoinRequests {
		t.Fatalf("named group settings = (%+v, %v), want join captcha enabled", settings, err)
	}
	other, err := captcha.GetCaptchaSettings(connected)
	if err != nil || other == nil || other.JoinRequests {
		t.Fatalf("connected group settings = (%+v, %v), want unchanged", other, err)
	}
	if conn := connections.Connection(f.admin.Id); !conn.Connected || conn.ChatId != connected {
		t.Fatalf("connection = %+v, want it kept on %d", conn, connected)
	}
	processPM(t, dispatcher, f, f.admin.Id, "/captchajoin @fixturegroup")
	want := translatedWith(t, "captcha_join_requests_current", i18n.TranslationParams{"status": translated(t, "captcha_status_enabled")})
	line := translatedWith(t, "connections_pm_group_confirm_target", i18n.TranslationParams{"title": f.chat.Title, "id": f.chat.Id})
	if text := fmt.Sprint(lastCall(t, f.client, "sendMessage").Params["text"]); text != want+"\n\n"+line {
		t.Fatalf("PM state = %q, want state and named group", text)
	}

	processPM(t, dispatcher, f, f.member.Id, "/captchajoin @fixturegroup off")
	settings, err = captcha.GetCaptchaSettings(f.chat.Id)
	if err != nil || settings == nil || !settings.JoinRequests {
		t.Fatalf("member changed settings: (%+v, %v)", settings, err)
	}
	if text := fmt.Sprint(lastCall(t, f.client, "sendMessage").Params["text"]); !strings.Contains(text, translated(t, "connections_is_user_connected_user_not_admin")) {
		t.Fatalf("member refusal = %q, want the admin-rights refusal", text)
	}
}

func TestCaptchaJoinAnonymousAdminHandlerUsesAdminGate(t *testing.T) {
	useRealLocales(t)
	f := newAdminChatFixture(t)
	LoadCaptcha(ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1}))
	memberCtx := newModuleMessageContext(f.bot, f.chat, f.member, "/captchajoin on")
	if err := HandleAnonymousAdmin(f.bot, memberCtx, "captchajoin"); err != ext.EndGroups {
		t.Fatalf("member anonymous handler error = %v, want EndGroups", err)
	}
	settings, err := captcha.GetCaptchaSettings(f.chat.Id)
	if err != nil || settings == nil || settings.JoinRequests {
		t.Fatalf("member enabled join captcha: (%+v, %v)", settings, err)
	}
	adminCtx := newModuleMessageContext(f.bot, f.chat, f.admin, "/captchajoin on")
	if err := HandleAnonymousAdmin(f.bot, adminCtx, "captchajoin"); err != nil {
		t.Fatalf("admin anonymous handler error = %v", err)
	}
	settings, err = captcha.GetCaptchaSettings(f.chat.Id)
	if err != nil || settings == nil || !settings.JoinRequests {
		t.Fatalf("post-proof admin did not enable join captcha: (%+v, %v)", settings, err)
	}
}

func TestSettingsJoinCaptchaTogglePersistsAndChecksAdmin(t *testing.T) {
	useRealLocales(t)
	f := newAdminChatFixture(t)
	if err := captcha.SetCaptchaEnabled(f.chat.Id, f.chat.Title, true); err != nil {
		t.Fatalf("SetCaptchaEnabled() error = %v", err)
	}
	if got := settingsRowValue(t, f.chat.Id, "/captchajoin"); got != "off" {
		t.Fatalf("initial captchajoin row = %q, want off", got)
	}
	pressSettingsToggle(t, f, f.member, "cj", "on")
	if text, alert := lastAnswer(t, f.client); !alert || text != translated(t, "connections_is_user_connected_user_not_admin") {
		t.Fatalf("member toggle answer = %q (alert %v), want admin refusal", text, alert)
	}
	if got := settingsRowValue(t, f.chat.Id, "/captchajoin"); got != "off" {
		t.Fatalf("member changed captchajoin row to %q", got)
	}
	for _, value := range []string{"on", "on", "off"} {
		pressSettingsToggle(t, f, f.admin, "cj", value)
		if text, alert := lastAnswer(t, f.client); alert {
			t.Fatalf("admin toggle to %s refused: %q", value, text)
		}
		if got := settingsRowValue(t, f.chat.Id, "/captchajoin"); got != value {
			t.Fatalf("captchajoin row = %q, want %q", got, value)
		}
		edit := lastCall(t, f.client, "editMessageText")
		if text := fmt.Sprint(edit.Params["text"]); !strings.Contains(text, "/captchajoin</code> — "+value) {
			t.Fatalf("updated summary does not show captchajoin=%s: %q", value, text)
		}
		wantNext := "on"
		if value == "on" {
			wantNext = "off"
		}
		if got := toggleButtons(t, markupOf(t, edit))["cj"]; got != wantNext {
			t.Fatalf("join captcha button offers %q, want %q", got, wantNext)
		}
		if got := settingsRowValue(t, f.chat.Id, "/captcha"); got != "on" {
			t.Fatalf("join toggle changed regular captcha to %q", got)
		}
	}
	assertCaptchaChatTitle(t, f.chat)
}
