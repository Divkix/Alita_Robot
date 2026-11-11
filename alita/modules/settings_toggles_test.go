//go:build testtools

package modules

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/captcha"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/federations"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

// settingsToggleFixture is an admin fixture whose group is in the database and
// in a federation, so every toggle row is in its summary.
func settingsToggleFixture(t *testing.T) adminChatFixture {
	t.Helper()
	useRealLocales(t)
	f := newAdminChatFixture(t)
	if err := chats.EnsureChatInDb(f.chat.Id, f.chat.Title); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	fed, err := federations.CreateFederation(f.admin.Id, fmt.Sprintf("Toggle Fed %d", f.chat.Id))
	if err != nil {
		t.Fatalf("CreateFederation() error = %v", err)
	}
	if err := federations.JoinFed(f.chat.Id, f.chat.Title, fed.FedID); err != nil {
		t.Fatalf("JoinFed() error = %v", err)
	}
	return f
}

func settingsRowValue(t *testing.T, chatID int64, command string) string {
	t.Helper()
	cache.ResetLocalForTest()
	for _, section := range collectChatSettings(context.Background(), chatID) {
		for _, row := range section.Rows {
			if row.Command == command {
				return row.Value
			}
		}
	}
	t.Fatalf("summary has no %s row", command)
	return ""
}

func pressSettingsToggle(t *testing.T, f adminChatFixture, user gotgbot.User, key, value string) {
	t.Helper()
	pressSettingsButton(t, f.bot, user, map[string]string{"a": "t", "c": fmt.Sprint(f.chat.Id), "k": key, "v": value})
}

// lastAnswer is the text of the last callback answer and whether it was an alert.
func lastAnswer(t *testing.T, client *moduleBotClient) (string, bool) {
	t.Helper()
	call := lastCall(t, client, "answerCallbackQuery")
	return fmt.Sprint(call.Params["text"]), fmt.Sprint(call.Params["show_alert"]) == "true"
}

// toggleButtons maps each toggle button's key to the value it sets.
func toggleButtons(t *testing.T, kb gotgbot.InlineKeyboardMarkup) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, row := range kb.InlineKeyboard {
		for _, button := range row {
			decoded, ok := decodeCallbackData(button.CallbackData, "settings")
			if !ok {
				t.Fatalf("button %q has no settings callback (over the cap?)", button.Text)
			}
			if action, _ := decoded.Field("a"); action != "t" {
				continue
			}
			key, _ := decoded.Field("k")
			value, _ := decoded.Field("v")
			out[key] = value
		}
	}
	return out
}

// Every toggle has a button on the summary, and it offers the next value.
func TestSettingsSummaryOffersEveryToggle(t *testing.T) {
	f := settingsToggleFixture(t)
	tr := i18n.MustNewTranslator("en")

	text, kb := settingsSummaryView(newModuleCallbackContext(f.bot, privateChatOf(f.admin), f.admin, ""), tr, &f.chat)
	buttons := toggleButtons(t, kb)

	for _, toggle := range settingsToggles {
		want := toggle.next(settingsRowValue(t, f.chat.Id, toggle.command))
		if got, ok := buttons[toggle.key]; !ok || got != want {
			t.Errorf("%s button value = %q (present %v), want %q", toggle.command, got, ok, want)
		}
	}
	if hint, _ := tr.GetString("settings_summary_toggle_hint"); !strings.Contains(text, hint) {
		t.Errorf("summary text lacks the toggle hint: %q", text)
	}
}

// A tap by an admin with every right sets each value of each toggle, and the
// summary shown again has it.
func TestSettingsToggleSetsEveryValue(t *testing.T) {
	f := settingsToggleFixture(t)
	f.makeOwner()

	for _, toggle := range settingsToggles {
		if toggle.declaration != nil {
			continue // Declared values are exercised across all surfaces.
		}
		for range toggle.values {
			value := toggle.next(settingsRowValue(t, f.chat.Id, toggle.command))
			pressSettingsToggle(t, f, f.admin, toggle.key, value)
			if text, alert := lastAnswer(t, f.client); alert {
				t.Fatalf("%s → %s refused: %q", toggle.command, value, text)
			}
			if got := settingsRowValue(t, f.chat.Id, toggle.command); got != value {
				t.Fatalf("%s after a tap for %q = %q", toggle.command, value, got)
			}
			edit := lastCall(t, f.client, "editMessageText")
			if text := fmt.Sprint(edit.Params["text"]); !strings.Contains(text, toggle.command+"</code> — "+value) {
				t.Fatalf("summary after %s → %s does not show it: %q", toggle.command, value, text)
			}
			if got := toggleButtons(t, markupOf(t, edit))[toggle.key]; got != toggle.next(value) {
				t.Fatalf("%s button after a tap for %q offers %q, want %q", toggle.command, value, got, toggle.next(value))
			}
		}
	}
}

func TestSettingsCaptchaTogglesPreserveChatTitle(t *testing.T) {
	useRealLocales(t)
	for _, toggle := range []struct {
		key, command, value string
	}{
		{"cp", "/captcha", "on"},
		{"cm", "/captchamode", "text"},
		{"ca", "/captchaaction", "ban"},
	} {
		for _, existingBlank := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/existingBlank=%t", toggle.key, existingBlank), func(t *testing.T) {
				f := newAdminChatFixture(t)
				if existingBlank {
					if err := chats.EnsureChatInDb(f.chat.Id, ""); err != nil {
						t.Fatalf("create empty-name chat: %v", err)
					}
				}
				pressSettingsToggle(t, f, f.admin, toggle.key, toggle.value)
				if text, alert := lastAnswer(t, f.client); alert {
					t.Fatalf("captcha toggle refused: %q", text)
				}
				assertCaptchaChatTitle(t, f.chat)
				if got := settingsRowValue(t, f.chat.Id, toggle.command); got != toggle.value {
					t.Fatalf("%s after toggle = %q, want %q", toggle.command, got, toggle.value)
				}
				if settings, err := captcha.GetCaptchaSettings(f.chat.Id); err != nil || settings == nil {
					t.Fatalf("GetCaptchaSettings() = (%+v, %v), want saved settings", settings, err)
				}
			})
		}
	}
}

// A button carries the value it sets, so a stale one sets that value again
// instead of flipping a change made since.
func TestSettingsToggleStaleButtonSetsItsValue(t *testing.T) {
	f := settingsToggleFixture(t)

	pressSettingsToggle(t, f, f.admin, "dd", "on")
	pressSettingsToggle(t, f, f.admin, "dd", "on")

	if got := settingsRowValue(t, f.chat.Id, "/disabledel"); got != "on" {
		t.Fatalf("/disabledel after two taps for on = %q, want on", got)
	}
}

// A forged button with a value the command does not take changes nothing.
func TestSettingsToggleRejectsForgedValues(t *testing.T) {
	f := settingsToggleFixture(t)

	for _, forged := range [][2]string{{"fm", "tban"}, {"zz", "on"}} {
		pressSettingsToggle(t, f, f.admin, forged[0], forged[1])
	}

	if calls := f.client.callsFor("editMessageText"); len(calls) != 0 {
		t.Fatalf("a forged tap edited the summary: %v", calls[0].Params)
	}
}
