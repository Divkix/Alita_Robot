//go:build testtools

package modules

import (
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/captcha"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

func captchaSettingsRows(t *testing.T, chatID int64) int64 {
	t.Helper()
	var count int64
	if err := db.DB.Model(&models.CaptchaSettings{}).Where("chat_id = ?", chatID).Count(&count).Error; err != nil {
		t.Fatalf("count captcha_settings rows: %v", err)
	}
	return count
}

// joinMessageButtons returns the callback data of every button on the join
// message the bot sent.
func joinMessageButtons(t *testing.T, client *moduleBotClient) []string {
	t.Helper()
	call := client.onlyCall(t, "sendMessage")
	markup, ok := call.Params["reply_markup"].(gotgbot.InlineKeyboardMarkup)
	if !ok {
		return nil
	}
	var data []string
	for _, row := range markup.InlineKeyboard {
		for _, button := range row {
			data = append(data, button.CallbackData)
		}
	}
	return data
}

// pressCaptchaJoinButton sends the press of the join message's button by from
// through a dispatcher with the captcha handlers loaded.
func pressCaptchaJoinButton(t *testing.T, bot *gotgbot.Bot, chat gotgbot.Chat, from gotgbot.User, data string) {
	t.Helper()
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadCaptcha(dispatcher)
	update := newModuleCallbackContext(bot, chat, from, data).Update
	if err := dispatcher.ProcessUpdate(bot, update, nil); err != nil {
		t.Fatalf("ProcessUpdate(captcha button) error = %v", err)
	}
}

func TestBotJoinedSupergroupOffersCaptchaButtonWithoutWritingSettings(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Fresh Group"}
	ctx := newModuleMessageContext(bot, chat, gotgbot.User{Id: 42, FirstName: "Member"}, "bot joined")

	if err := botJoinedGroup(bot, ctx); err != ext.ContinueGroups {
		t.Fatalf("botJoinedGroup() error = %v, want ContinueGroups", err)
	}
	buttons := joinMessageButtons(t, client)
	if len(buttons) != 1 {
		t.Fatalf("join message buttons = %q, want the captcha button", buttons)
	}
	if _, ok := decodeCallbackData(buttons[0], captchaJoinNamespace); !ok {
		t.Fatalf("button data %q is not a %s callback", buttons[0], captchaJoinNamespace)
	}
	if rows := captchaSettingsRows(t, chat.Id); rows != 0 {
		t.Fatalf("captcha_settings rows = %d after join, want none until the button is pressed", rows)
	}
}

func TestBotJoinedSupergroupWithCaptchaOnOffersNoButton(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Rejoined Group"}
	if err := captcha.SetCaptchaEnabled(chat.Id, true); err != nil {
		t.Fatalf("SetCaptchaEnabled setup error = %v", err)
	}
	ctx := newModuleMessageContext(bot, chat, gotgbot.User{Id: 42, FirstName: "Member"}, "bot joined")

	if err := botJoinedGroup(bot, ctx); err != ext.ContinueGroups {
		t.Fatalf("botJoinedGroup() error = %v, want ContinueGroups", err)
	}
	if buttons := joinMessageButtons(t, client); len(buttons) != 0 {
		t.Fatalf("join message buttons = %q, want none when captcha is already on", buttons)
	}
}

func TestCaptchaJoinButtonEnablesCaptchaForAdmin(t *testing.T) {
	restore, err := i18n.OverrideManagerForTest(`captcha_join_enabled: "Captcha is on now."`)
	if err != nil {
		t.Fatalf("OverrideManagerForTest: %v", err)
	}
	t.Cleanup(restore)
	f := newAdminChatFixture(t)
	// Cache the default (off) settings so the press must invalidate them.
	if settings, err := captcha.GetCaptchaSettings(f.chat.Id); err != nil || settings.Enabled {
		t.Fatalf("GetCaptchaSettings() before press = %+v, %v; want off", settings, err)
	}
	data, ok := mustCallbackData(captchaJoinNamespace, nil)
	if !ok {
		t.Fatal("captcha button data could not be encoded")
	}

	pressCaptchaJoinButton(t, f.bot, f.chat, f.admin, data)

	if rows := captchaSettingsRows(t, f.chat.Id); rows != 1 {
		t.Fatalf("captcha_settings rows = %d, want the row the press created", rows)
	}
	settings, err := captcha.GetCaptchaSettings(f.chat.Id)
	if err != nil {
		t.Fatalf("GetCaptchaSettings() after press error = %v", err)
	}
	if !settings.Enabled {
		t.Fatal("captcha enabled = false after the admin's press, want true (cache not invalidated?)")
	}
	confirmation, _ := i18n.MustNewTranslator("en").GetString("captcha_join_enabled")
	answerCall := f.client.onlyCall(t, "answerCallbackQuery")
	assertParams(t, answerCall, map[string]any{"text": confirmation})
	edit := f.client.onlyCall(t, "editMessageText")
	if text := fmt.Sprint(edit.Params["text"]); !strings.HasSuffix(text, "\n\n"+confirmation) {
		t.Fatalf("edited join message = %q, want the confirmation appended", text)
	}
	if markup, ok := edit.Params["reply_markup"].(gotgbot.InlineKeyboardMarkup); ok && len(markup.InlineKeyboard) != 0 {
		t.Fatalf("edited join message keyboard = %+v, want the button removed", markup.InlineKeyboard)
	}

	// A second press reports captcha as on and writes nothing new.
	pressCaptchaJoinButton(t, f.bot, f.chat, f.admin, data)
	if calls := f.client.callsFor("answerCallbackQuery"); len(calls) != 2 {
		t.Fatalf("answerCallbackQuery calls = %d, want an answer to the repeated press", len(calls))
	}
	if rows := captchaSettingsRows(t, f.chat.Id); rows != 1 {
		t.Fatalf("captcha_settings rows = %d after a repeated press, want 1", rows)
	}
}

func TestCaptchaJoinButtonRefusesNonAdmin(t *testing.T) {
	f := newAdminChatFixture(t)
	data, ok := mustCallbackData(captchaJoinNamespace, nil)
	if !ok {
		t.Fatal("captcha button data could not be encoded")
	}

	pressCaptchaJoinButton(t, f.bot, f.chat, f.member, data)

	if rows := captchaSettingsRows(t, f.chat.Id); rows != 0 {
		t.Fatalf("captcha_settings rows = %d after a member's press, want none", rows)
	}
	settings, err := captcha.GetCaptchaSettings(f.chat.Id)
	if err != nil {
		t.Fatalf("GetCaptchaSettings() error = %v", err)
	}
	if settings.Enabled {
		t.Fatal("captcha enabled after a member's press, want off")
	}
	f.client.onlyCall(t, "answerCallbackQuery")
	if calls := f.client.callsFor("editMessageText"); len(calls) != 0 {
		t.Fatalf("editMessageText calls = %d, want the join message untouched", len(calls))
	}
}
