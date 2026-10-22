//go:build testtools

package modules

import (
	"fmt"
	"os"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

// The setup wizard opened from the PM menu must answer in the language of the
// user who pressed the button, not always in English.
func TestBotConfigWizardUsesTheUserLanguage(t *testing.T) {
	withMiniredis(t)
	locales := map[string]string{}
	for _, code := range []string{"en", "ru"} {
		raw, err := os.ReadFile("../../locales/" + code + ".yml")
		if err != nil {
			t.Fatalf("read %s.yml: %v", code, err)
		}
		locales[code] = string(raw)
	}
	restore, err := i18n.OverrideLocalesForTest(locales)
	if err != nil {
		t.Fatalf("OverrideLocalesForTest: %v", err)
	}
	t.Cleanup(restore)

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 4601, FirstName: "Ivan"}
	privateChat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Ivan"}
	if err := lang.ChangeUserLanguage(user.Id, "ru"); err != nil {
		t.Fatalf("ChangeUserLanguage() error = %v", err)
	}
	t.Cleanup(func() {
		if err := lang.ChangeUserLanguage(user.Id, "en"); err != nil {
			t.Errorf("ChangeUserLanguage() cleanup error = %v", err)
		}
	})

	ru := i18n.MustNewTranslator("ru")
	en := i18n.MustNewTranslator("en")
	label := func(key string) string {
		t.Helper()
		got, _ := ru.GetString(key)
		english, _ := en.GetString(key)
		if got == "" || got == english {
			t.Fatalf("locales ru %s = %q, en = %q; want a distinct Russian value", key, got, english)
		}
		return got
	}
	step2, _ := ru.GetString("help_configuration_step-2")

	steps := []struct {
		step    string
		text    string
		buttons [][]string // button labels per keyboard row
	}{
		{"step1", label("help_configuration_step-1"), [][]string{{label("help_button_add_alita")}, {label("common_done")}}},
		{"step2", fmt.Sprintf(step2, bot.Username), [][]string{{label("common_done")}}},
		{"step3", label("help_configuration_step-3"), [][]string{{label("common_continue")}}},
	}
	for _, tc := range steps {
		resetCalls(client)
		ctx := newModuleCallbackContext(bot, privateChat, user, encodeCallbackData("configuration", map[string]string{"s": tc.step}))
		if err := DefaultHelpRegistry().botConfig(bot, ctx); err != ext.EndGroups {
			t.Fatalf("botConfig(%s) error = %v, want EndGroups", tc.step, err)
		}
		edits := client.callsFor("editMessageText")
		if len(edits) != 1 {
			t.Fatalf("%s: editMessageText calls = %d, want 1", tc.step, len(edits))
		}
		if got := edits[0].Params["text"]; got != tc.text {
			t.Errorf("%s text = %q, want the Russian %q", tc.step, got, tc.text)
		}
		markup, ok := edits[0].Params["reply_markup"].(gotgbot.InlineKeyboardMarkup)
		if !ok {
			t.Fatalf("%s reply_markup = %T, want InlineKeyboardMarkup", tc.step, edits[0].Params["reply_markup"])
		}
		if len(markup.InlineKeyboard) != len(tc.buttons) {
			t.Fatalf("%s keyboard rows = %d, want %d", tc.step, len(markup.InlineKeyboard), len(tc.buttons))
		}
		for i, row := range tc.buttons {
			for j, want := range row {
				if got := markup.InlineKeyboard[i][j].Text; got != want {
					t.Errorf("%s button [%d][%d] = %q, want the Russian %q", tc.step, i, j, got, want)
				}
			}
		}
	}
}
