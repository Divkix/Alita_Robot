//go:build testtools

package modules

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/captcha"
	"github.com/divkix/Alita_Robot/alita/db/greetings"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

// newCaptchaJoinChat returns a captcha-enabled supergroup with a welcome text.
func newCaptchaJoinChat(t *testing.T) gotgbot.Chat {
	t.Helper()
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Captcha Rights Chat"}
	if err := captcha.SetCaptchaEnabled(chat.Id, chat.Title, true); err != nil {
		t.Fatalf("SetCaptchaEnabled setup error = %v", err)
	}
	if err := greetings.SetWelcomeText(chat.Id, "Welcome {first}", "", nil, db.TEXT); err != nil {
		t.Fatalf("SetWelcomeText setup error = %v", err)
	}
	return chat
}

func countText(texts []string, want string) int {
	n := 0
	for _, text := range texts {
		if text == want {
			n++
		}
	}
	return n
}

func assertNoCaptchaAttempt(t *testing.T, userID, chatID int64) {
	t.Helper()
	attempt, err := captcha.GetCaptchaAttempt(userID, chatID)
	if err != nil {
		t.Fatalf("GetCaptchaAttempt error = %v", err)
	}
	if attempt != nil {
		t.Fatalf("captcha attempt %d remained for user %d, want none", attempt.ID, userID)
	}
}

// rightsNoticeText loads a locale holding the missing-right notice, so the
// notice can be told apart from the welcome, and returns its text.
func rightsNoticeText(t *testing.T) string {
	t.Helper()
	restore, err := i18n.OverrideManagerForTest(`captcha_bot_cannot_restrict: "Captcha needs the Ban users right."`)
	if err != nil {
		t.Fatalf("OverrideManagerForTest: %v", err)
	}
	t.Cleanup(restore)
	text, err := i18n.MustNewTranslator("en").GetString("captcha_bot_cannot_restrict")
	if err != nil || text == "" {
		t.Fatalf("captcha_bot_cannot_restrict translation missing: %v", err)
	}
	return text
}

func TestCaptchaJoinWithoutRestrictRightSendsWelcomeAndOneNotice(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := newCaptchaJoinChat(t)
	// The bot is a plain member: it cannot mute anyone.
	client.membersByChat = map[string]json.RawMessage{
		fmt.Sprintf("%d|%d", chat.Id, bot.Id): json.RawMessage(
			`{"status":"member","user":{"id":999,"is_bot":true,"first_name":"Alita"}}`,
		),
	}
	notice := rightsNoticeText(t)

	first := gotgbot.User{Id: 4801, FirstName: "First"}
	clearRecentJoinProcessing(chat.Id, first.Id)
	if err := processSingleNewMember(bot, &ext.Context{EffectiveChat: &chat}, first, true); err != nil {
		t.Fatalf("processSingleNewMember(first) error = %v", err)
	}

	if calls := client.callsFor("restrictChatMember"); len(calls) != 0 {
		t.Fatalf("restrictChatMember calls = %d, want no mute without the right", len(calls))
	}
	if calls := client.callsFor("sendPhoto"); len(calls) != 0 {
		t.Fatalf("sendPhoto calls = %d, want no captcha challenge", len(calls))
	}
	assertNoCaptchaAttempt(t, first.Id, chat.Id)
	texts := sentTexts(client)
	if len(texts) != 2 || countText(texts, notice) != 1 {
		t.Fatalf("messages sent = %q, want the welcome and one missing-right notice", texts)
	}

	second := gotgbot.User{Id: 4802, FirstName: "Second"}
	clearRecentJoinProcessing(chat.Id, second.Id)
	if err := processSingleNewMember(bot, &ext.Context{EffectiveChat: &chat}, second, true); err != nil {
		t.Fatalf("processSingleNewMember(second) error = %v", err)
	}
	assertNoCaptchaAttempt(t, second.Id, chat.Id)
	texts = sentTexts(client)
	if len(texts) != 3 || countText(texts, notice) != 1 {
		t.Fatalf("messages sent = %q, want a second welcome and no repeated notice", texts)
	}
}

func TestCaptchaJoinWhenMuteLacksRightSendsWelcome(t *testing.T) {
	client := newModuleBotClient()
	// The bot looks able to restrict, but Telegram refuses the mute.
	client.errors["restrictChatMember"] = &gotgbot.TelegramError{
		Method:      "restrictChatMember",
		Code:        400,
		Description: "Bad Request: not enough rights to restrict/unrestrict chat member",
	}
	bot := newModuleTestBot(client)
	chat := newCaptchaJoinChat(t)
	member := gotgbot.User{Id: 4803, FirstName: "Newcomer"}
	notice := rightsNoticeText(t)

	clearRecentJoinProcessing(chat.Id, member.Id)
	if err := processSingleNewMember(bot, &ext.Context{EffectiveChat: &chat}, member, true); err != nil {
		t.Fatalf("processSingleNewMember() error = %v", err)
	}

	if calls := client.callsFor("restrictChatMember"); len(calls) != 1 {
		t.Fatalf("restrictChatMember calls = %d, want the one refused mute", len(calls))
	}
	if calls := client.callsFor("sendPhoto"); len(calls) != 0 {
		t.Fatalf("sendPhoto calls = %d, want no captcha challenge", len(calls))
	}
	assertNoCaptchaAttempt(t, member.Id, chat.Id)
	texts := sentTexts(client)
	if len(texts) != 2 || countText(texts, notice) != 1 {
		t.Fatalf("messages sent = %q, want the welcome and one missing-right notice", texts)
	}
}
