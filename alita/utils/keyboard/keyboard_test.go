package keyboard

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db"
)

type keyboardBotClient struct{}

func (keyboardBotClient) RequestWithContext(_ context.Context, _ string, method string, params map[string]any, _ *gotgbot.RequestOpts) (json.RawMessage, error) {
	switch method {
	case "getChat":
		return json.RawMessage(`{"id":-1001,"type":"supergroup","title":"Keyboard Chat"}`), nil
	case "getChatAdministrators":
		return json.RawMessage(`[{"status":"administrator","user":{"id":777000,"is_bot":false,"first_name":"Telegram"}},{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Keyboard Bot"}}]`), nil
	case "getChatMember":
		if fmt.Sprint(params["user_id"]) == "999" {
			return json.RawMessage(`{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Keyboard Bot"}}`), nil
		}
		return json.RawMessage(`{"status":"member","user":{"id":42,"is_bot":false,"first_name":"Member"}}`), nil
	default:
		return json.RawMessage(`true`), nil
	}
}

func (keyboardBotClient) GetAPIURL(*gotgbot.RequestOpts) string {
	return gotgbot.DefaultAPIURL
}

func (keyboardBotClient) FileURL(token string, path string, _ *gotgbot.RequestOpts) string {
	return gotgbot.DefaultAPIURL + "/file/bot" + token + "/" + path
}

func newKeyboardBot() *gotgbot.Bot {
	return &gotgbot.Bot{
		Token:     "999:test",
		BotClient: keyboardBotClient{},
		User:      gotgbot.User{Id: 999, IsBot: true, Username: "KeyboardBot"},
	}
}

func TestBuildKeyboardGroupsSameLineButtons(t *testing.T) {
	t.Parallel()

	buttons := []db.Button{
		{Name: "one", Url: "https://one.example"},
		{Name: "two", Url: "https://two.example", SameLine: true},
		{Name: "three", Url: "https://three.example"},
	}

	got := BuildKeyboard(buttons)
	if len(got) != 2 {
		t.Fatalf("row count = %d, want 2", len(got))
	}
	if len(got[0]) != 2 {
		t.Fatalf("first row button count = %d, want 2", len(got[0]))
	}
	if got[0][0].Text != "one" || got[0][1].Text != "two" {
		t.Fatalf("first row texts = %#v, want one and two", got[0])
	}
	if got[1][0].Text != "three" {
		t.Fatalf("second row first text = %q, want three", got[1][0].Text)
	}
}

func TestMakeLanguageKeyboardSkipsUnavailableLanguages(t *testing.T) {
	originalCodes := config.AppConfig.ValidLangCodes
	config.AppConfig.ValidLangCodes = []string{"bad-code"}
	defer func() { config.AppConfig.ValidLangCodes = originalCodes }()

	got := MakeLanguageKeyboard()
	if got != nil {
		t.Fatalf("keyboard for unavailable language = %#v, want nil", got)
	}
}

func TestInitButtonsReflectsAdminStatus(t *testing.T) {
	bot := newKeyboardBot()

	adminKb := InitButtons(bot, -1001, 777000)
	if len(adminKb.InlineKeyboard) != 2 {
		t.Fatalf("InitButtons(admin) rows = %d, want admin and user rows", len(adminKb.InlineKeyboard))
	}
	if adminKb.InlineKeyboard[0][0].Text == "" || adminKb.InlineKeyboard[1][0].Text == "" {
		t.Fatalf("InitButtons(admin) has empty button text: %#v", adminKb.InlineKeyboard)
	}

	userKb := InitButtons(bot, -1001, 42)
	if len(userKb.InlineKeyboard) != 1 {
		t.Fatalf("InitButtons(user) rows = %d, want only user row", len(userKb.InlineKeyboard))
	}
}
