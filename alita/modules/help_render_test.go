//go:build testtools

package modules

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/i18n"
)

type helpFakeBotClient struct {
	response json.RawMessage
	err      error
}

func (f helpFakeBotClient) RequestWithContext(context.Context, string, string, map[string]any, *gotgbot.RequestOpts) (json.RawMessage, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.response, nil
}

func (f helpFakeBotClient) GetAPIURL(*gotgbot.RequestOpts) string {
	return "https://api.telegram.org"
}

func (f helpFakeBotClient) FileURL(token string, path string, _ *gotgbot.RequestOpts) string {
	return "https://api.telegram.org/file/bot" + token + "/" + path
}

func helpTestTranslator(t *testing.T) *i18n.Translator {
	t.Helper()

	tr, err := i18n.NewTestTranslator(`
help_info_about_header: "About Alita"
help_bot_intro: "Intro. "
help_news_channel_text: "News."
help_pm_intro: "Hi %s. "
help_all_commands_usage: "Use commands."
help_button_about_me: "About me"
help_button_news_channel: "News"
help_button_support_group: "Support"
help_button_configuration: "Config"
common_back_arrow_alt: "Back"
help_button_about: "About"
help_button_add_to_chat: "Add"
help_button_commands_help: "Commands"
help_button_language: "Language"
`)
	if err != nil {
		t.Fatalf("NewTestTranslator() error = %v", err)
	}
	return tr
}

func TestHelpTextRendering(t *testing.T) {
	t.Parallel()

	tr := helpTestTranslator(t)
	if got := getAboutText(tr); got != "About Alita" {
		t.Fatalf("getAboutText() = %q", got)
	}
	if got := getStartHelp(tr); got != "Intro. News." {
		t.Fatalf("getStartHelp() = %q", got)
	}
	if got := getMainHelp(tr, "Div"); got != "Hi Div. Use commands." {
		t.Fatalf("getMainHelp() = %q", got)
	}
}

func TestHelpKeyboardsUseCallbackCodecAndBotUsername(t *testing.T) {
	t.Parallel()

	tr := helpTestTranslator(t)
	aboutKb := getAboutKb(tr)
	if len(aboutKb.InlineKeyboard) != 4 {
		t.Fatalf("getAboutKb() rows = %d, want 4", len(aboutKb.InlineKeyboard))
	}
	if aboutKb.InlineKeyboard[0][0].Text != "About me" {
		t.Fatalf("about button text = %q", aboutKb.InlineKeyboard[0][0].Text)
	}
	if !strings.HasPrefix(aboutKb.InlineKeyboard[0][0].CallbackData, "about|v1|") {
		t.Fatalf("about callback = %q, want encoded about callback", aboutKb.InlineKeyboard[0][0].CallbackData)
	}

	startKb := getStartMarkup(tr, "AlitaRobot")
	if len(startKb.InlineKeyboard) != 4 {
		t.Fatalf("getStartMarkup() rows = %d, want 4", len(startKb.InlineKeyboard))
	}
	if got := startKb.InlineKeyboard[1][0].Url; got != "https://t.me/AlitaRobot?startgroup=botstart" {
		t.Fatalf("add-to-chat URL = %q", got)
	}
	if !strings.HasPrefix(startKb.InlineKeyboard[2][0].CallbackData, "helpq|v1|") {
		t.Fatalf("commands callback = %q, want encoded help callback", startKb.InlineKeyboard[2][0].CallbackData)
	}
}

func resetCachedBotUsername() {
	cachedBotUsernameMu.Lock()
	cachedBotUsername = ""
	cachedBotUsernameMu.Unlock()
}

func TestGetBotUsernameCachesStructAndGetMeFallbacks(t *testing.T) {
	resetCachedBotUsername()
	t.Cleanup(resetCachedBotUsername)

	if got := getBotUsername(&gotgbot.Bot{User: gotgbot.User{Username: "StructBot"}}); got != "StructBot" {
		t.Fatalf("getBotUsername(struct) = %q", got)
	}
	if got := getBotUsername(nil); got != "StructBot" {
		t.Fatalf("getBotUsername(cached) = %q", got)
	}

	resetCachedBotUsername()
	bot := &gotgbot.Bot{
		Token: "123:test",
		BotClient: helpFakeBotClient{response: json.RawMessage(
			`{"id":123,"is_bot":true,"first_name":"Alita","username":"GetMeBot"}`,
		)},
	}
	if got := getBotUsername(bot); got != "GetMeBot" {
		t.Fatalf("getBotUsername(getMe) = %q", got)
	}

	resetCachedBotUsername()
	if got := getBotUsername(nil); got != "" {
		t.Fatalf("getBotUsername(nil) = %q, want empty", got)
	}
}
