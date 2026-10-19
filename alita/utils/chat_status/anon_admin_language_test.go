//go:build testtools

package chat_status

import (
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/db/admin"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

// The verification keyboard an anonymous admin gets is built in the language of
// the group the command was sent in.
func TestAnonAdminKeyboardUsesGroupLanguage(t *testing.T) {
	skipIfNoDb(t)

	restoreI18n, err := i18n.OverrideLocalesForTest(map[string]string{
		"en": "chat_status_anon_confirm: \"en:confirm\"\nchat_status_anon_prove_admin: \"en:prove\"\n",
		"ru": "chat_status_anon_confirm: \"ru:confirm\"\nchat_status_anon_prove_admin: \"ru:prove\"\n",
	})
	if err != nil {
		t.Fatalf("OverrideLocalesForTest: %v", err)
	}
	t.Cleanup(restoreI18n)

	client := &paramRecordingChatStatusClient{}
	bot := &gotgbot.Bot{
		Token:     "999:test",
		BotClient: client,
		User:      gotgbot.User{Id: 999, IsBot: true, FirstName: "Bot"},
	}
	chat := &gotgbot.Chat{Id: -100123456791, Type: "supergroup", Title: "Anon Chat"}
	msg := &gotgbot.Message{MessageId: 780, Date: 1, Chat: *chat, Text: "/ban 42"}
	if err := lang.ChangeGroupLanguage(chat.Id, "ru"); err != nil {
		t.Fatalf("ChangeGroupLanguage() error = %v", err)
	}
	if err := admin.SetAnonAdminMode(chat.Id, false); err != nil {
		t.Fatalf("SetAnonAdminMode(false) error = %v", err)
	}

	isAdmin, shouldReturn := checkAnonAdmin(bot, chat, msg, &gotgbot.Sender{Chat: chat, ChatId: chat.Id})
	if isAdmin || !shouldReturn {
		t.Fatalf("checkAnonAdmin(verify) = (%v, %v), want false, true", isAdmin, shouldReturn)
	}
	if len(client.calls) != 1 || client.calls[0].method != "sendMessage" {
		t.Fatalf("calls = %+v, want one sendMessage", client.calls)
	}
	params := client.calls[0].params
	if got := fmt.Sprint(params["text"]); got != "ru:confirm" {
		t.Errorf("keyboard text = %q, want the group's language (ru:confirm)", got)
	}
	markup, ok := params["reply_markup"].(gotgbot.InlineKeyboardMarkup)
	if !ok || len(markup.InlineKeyboard) != 1 || len(markup.InlineKeyboard[0]) != 1 {
		t.Fatalf("reply_markup = %#v, want one inline button", params["reply_markup"])
	}
	if got := markup.InlineKeyboard[0][0].Text; !strings.HasPrefix(got, "ru:") {
		t.Errorf("button text = %q, want the group's language (ru:prove)", got)
	}
}
