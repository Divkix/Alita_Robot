//go:build testtools

package modules

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db/cache"
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
	withAISpamTestConfig(t, "test-key-123456", true, config.AppConfig.MessageDump)

	for _, toggle := range settingsToggles {
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

func TestSettingsToggleRefusals(t *testing.T) {
	limited := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Limited"}
	limitedJSON := fmt.Sprintf(`{"status":"administrator","user":{"id":%d,"is_bot":false,"first_name":"Limited"},"can_delete_messages":true}`, limited.Id)

	tests := []struct {
		name    string
		user    func(f adminChatFixture) gotgbot.User
		setup   func(t *testing.T, f adminChatFixture)
		key     string
		value   string
		command string
		denyKey string
		// atWrite marks a refusal by the write itself, after which the
		// summary is drawn again like after any failed write.
		atWrite bool
	}{
		{
			name: "member", key: "we", value: "off", command: "/welcome",
			user:    func(f adminChatFixture) gotgbot.User { return f.member },
			denyKey: "connections_is_user_connected_user_not_admin",
		},
		{
			name: "admin without change info", key: "cw", value: "on", command: "/cleanwelcome",
			user:    func(adminChatFixture) gotgbot.User { return limited },
			denyKey: "chat_status_change_info_button_error",
		},
		{
			name: "admin who is not the owner", key: "an", value: "on", command: "/anonadmin",
			user:    func(f adminChatFixture) gotgbot.User { return f.admin },
			denyKey: "chat_status_owner_button_error",
		},
		{
			name: "bot not admin", key: "dd", value: "on", command: "/disabledel",
			user: func(f adminChatFixture) gotgbot.User { return f.admin },
			setup: func(_ *testing.T, f adminChatFixture) {
				adminJSON := fmt.Sprintf(`{"status":"administrator","user":{"id":%d,"is_bot":false,"first_name":"Admin"},%s}`, f.admin.Id, fixtureAdminRights)
				f.client.adminsByChat[fmt.Sprint(f.chat.Id)] = []byte("[" + adminJSON + "]")
			},
			denyKey: "connections_is_user_connected_bot_not_admin",
		},
		{
			name: "aispam without a key", key: "ai", value: "on", command: "/aispam",
			user: func(f adminChatFixture) gotgbot.User { return f.admin },
			setup: func(t *testing.T, _ adminChatFixture) {
				withAISpamTestConfig(t, "", true, config.AppConfig.MessageDump)
			},
			denyKey: "aispam_no_key",
			atWrite: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := settingsToggleFixture(t)
			f.addAdmins(limitedJSON)
			f.client.membersByUser[fmt.Sprint(limited.Id)] = []byte(limitedJSON)
			if tt.setup != nil {
				tt.setup(t, f)
			}
			before := settingsRowValue(t, f.chat.Id, tt.command)

			pressSettingsToggle(t, f, tt.user(f), tt.key, tt.value)

			want, _ := i18n.MustNewTranslator("en").GetString(tt.denyKey)
			if text, alert := lastAnswer(t, f.client); !alert || text != want {
				t.Fatalf("answer = %q (alert %v), want alert %q", text, alert, want)
			}
			if got := settingsRowValue(t, f.chat.Id, tt.command); got != before {
				t.Fatalf("%s = %q after a refused tap, want %q", tt.command, got, before)
			}
			if calls := f.client.callsFor("editMessageText"); !tt.atWrite && len(calls) != 0 {
				t.Fatalf("a refused tap edited the summary: %v", calls[0].Params)
			}
		})
	}
}

// A forged button with a value the command does not take changes nothing.
func TestSettingsToggleRejectsForgedValues(t *testing.T) {
	f := settingsToggleFixture(t)

	for _, forged := range [][2]string{{"cm", "emoji"}, {"fm", "tban"}, {"we", "maybe"}, {"zz", "on"}} {
		pressSettingsToggle(t, f, f.admin, forged[0], forged[1])
	}

	if got := settingsRowValue(t, f.chat.Id, "/captchamode"); got != settingsDefaultCaptchaMode {
		t.Fatalf("/captchamode = %q after a forged tap", got)
	}
	if got := settingsRowValue(t, f.chat.Id, "/setfloodmode"); got != settingsDefaultFloodAction {
		t.Fatalf("/setfloodmode = %q after a forged tap", got)
	}
	if calls := f.client.callsFor("editMessageText"); len(calls) != 0 {
		t.Fatalf("a forged tap edited the summary: %v", calls[0].Params)
	}
}
