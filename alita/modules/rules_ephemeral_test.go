//go:build testtools

package modules

import (
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/db/rules"
)

// privateRulesChat is an admin fixture whose rules open in PM.
func privateRulesChat(t *testing.T) adminChatFixture {
	t.Helper()
	useRealLocales(t)
	f := newAdminChatFixture(t)
	if err := rules.SetChatRules(f.chat.Id, "Be kind to each other"); err != nil {
		t.Fatalf("SetChatRules() error = %v", err)
	}
	if err := rules.SetPrivateRules(f.chat.Id, true); err != nil {
		t.Fatalf("SetPrivateRules() error = %v", err)
	}
	cleanupEphemeral(t, f.chat.Id)
	return f
}

// assertRulesPMButton fails unless call is today's public answer: a reply
// carrying the button that opens the rules in PM.
func assertRulesPMButton(t *testing.T, call moduleBotCall, chatID int64) {
	t.Helper()
	assertParams(t, call, map[string]any{"chat_id": chatID, "text": translated(t, "rules_click_for_rules")})
	kb, _ := call.Params["reply_markup"].(gotgbot.InlineKeyboardMarkup)
	if len(kb.InlineKeyboard) == 0 || !strings.Contains(kb.InlineKeyboard[0][0].Url, fmt.Sprintf("start=rules_%d", chatID)) {
		t.Fatalf("reply_markup = %+v, want the PM rules button", call.Params["reply_markup"])
	}
	if ephemeralReceiverOf(call) != 0 {
		t.Fatal("the public answer was sent ephemerally")
	}
}

func TestPrivateRulesWithEphemeralOffSendTheRulesPMButton(t *testing.T) {
	f := privateRulesChat(t)
	aispamProcessUpdate(t, rulesDispatcher(), f.bot, f.chat, f.member, 1201, "/rules")
	assertRulesPMButton(t, f.client.onlyCall(t, "sendMessage"), f.chat.Id)
}

func TestPrivateRulesWithEphemeralOnShowTheRulesOnlyToTheAsker(t *testing.T) {
	f := privateRulesChat(t)
	enableEphemeral(t, f.chat.Id)
	aispamProcessUpdate(t, rulesDispatcher(), f.bot, f.chat, f.member, 1202, "/rules")

	call := f.client.onlyCall(t, "sendMessage")
	if got := ephemeralReceiverOf(call); got != f.member.Id {
		t.Fatalf("receiver = %d, want the asker %d", got, f.member.Id)
	}
	assertParams(t, call, map[string]any{"chat_id": f.chat.Id})
	if text := fmt.Sprint(call.Params["text"]); !strings.Contains(text, "Be kind to each other") {
		t.Fatalf("ephemeral text = %q, want the rules", text)
	}
}

func TestPrivateRulesFallBackToThePMButtonWhenTheEphemeralSendFails(t *testing.T) {
	f := privateRulesChat(t)
	enableEphemeral(t, f.chat.Id)
	f.bot.BotClient = ephemeralRefusingClient{f.client}
	aispamProcessUpdate(t, rulesDispatcher(), f.bot, f.chat, f.member, 1203, "/rules")

	calls := f.client.callsFor("sendMessage")
	if len(calls) != 2 || ephemeralReceiverOf(calls[0]) != f.member.Id {
		t.Fatalf("sendMessage calls = %+v, want the refused ephemeral send then the public answer", calls)
	}
	assertRulesPMButton(t, calls[1], f.chat.Id)
}

// A moderator replying /rules to a newcomer answers publicly, as before.
func TestPrivateRulesSentAsAReplyStayPublic(t *testing.T) {
	f := privateRulesChat(t)
	enableEphemeral(t, f.chat.Id)
	update := aispamMessageUpdate(f.chat, f.admin, 1204, "/rules")
	update.Message.ReplyToMessage = &gotgbot.Message{MessageId: 50, Date: 1, Chat: f.chat, From: &f.member, Text: "hi"}
	aispamProcessRawUpdate(t, rulesDispatcher(), f.bot, update)

	call := f.client.onlyCall(t, "sendMessage")
	assertRulesPMButton(t, call, f.chat.Id)
	if rp, _ := call.Params["reply_parameters"].(*gotgbot.ReplyParameters); rp == nil || rp.MessageId != 50 {
		t.Fatalf("reply_parameters = %+v, want a reply to the newcomer's message", call.Params["reply_parameters"])
	}
}

// From PM, /rules naming the group answers in PM; nothing is ephemeral.
func TestRulesFromPMWithEphemeralOnStayInPM(t *testing.T) {
	f := privateRulesChat(t)
	enableEphemeral(t, f.chat.Id)
	processPM(t, rulesDispatcher(), f, f.admin.Id, fmt.Sprintf("/rules %d", f.chat.Id))

	if sends := ephemeralSends(f.client); len(sends) != 0 {
		t.Fatalf("ephemeral sends = %+v, want none from PM", sends)
	}
	if !repliedIn(f, f.admin.Id, "Be kind to each other") {
		t.Fatalf("the rules did not come to PM; sent:\n%s", sentText(f.client))
	}
}
