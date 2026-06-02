package modules

import (
	"crypto/rand"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/divkix/Alita_Robot/alita/db/rules"
)

func uniqueModuleChatID() int64 {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		panic("uniqueModuleChatID: crypto/rand failed: " + err.Error())
	}
	n := int64(binary.BigEndian.Uint64(buf[:]) & 0x7fffffffffffffff)
	return -1000000000000 - n
}

func TestSetRulesStoresCommandTextAndReplies(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chatID := uniqueModuleChatID()
	chat := gotgbot.Chat{Id: chatID, Type: "supergroup", Title: "Rules Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	ctx := newModuleMessageContext(bot, chat, user, "/setrules **Be kind**")

	if err := rulesModule.setRules(bot, ctx); err != ext.EndGroups {
		t.Fatalf("setRules() error = %v, want EndGroups", err)
	}

	rules := rules.GetChatRulesInfo(chatID)
	if !strings.Contains(rules.Rules, "Be kind") {
		t.Fatalf("stored rules = %q, want command text", rules.Rules)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
}

func TestSetRulesWithoutTextDoesNotOverwriteExistingRules(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chatID := uniqueModuleChatID()
	rules.SetChatRules(chatID, "existing rules")
	chat := gotgbot.Chat{Id: chatID, Type: "supergroup", Title: "Rules Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	ctx := newModuleMessageContext(bot, chat, user, "/setrules")

	if err := rulesModule.setRules(bot, ctx); err != ext.EndGroups {
		t.Fatalf("setRules() error = %v, want EndGroups", err)
	}

	if got := rules.GetChatRulesInfo(chatID).Rules; got != "existing rules" {
		t.Fatalf("stored rules = %q, want existing rules untouched", got)
	}
}

func TestSendRulesUsesPrivateButtonWhenEnabled(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chatID := uniqueModuleChatID()
	rules.SetChatRules(chatID, "Read before posting")
	rules.SetPrivateRules(chatID, true)
	rules.SetChatRulesButton(chatID, "Read rules")

	chat := gotgbot.Chat{Id: chatID, Type: "supergroup", Title: "Rules Chat"}
	user := gotgbot.User{Id: 42, FirstName: "Member"}
	ctx := newModuleMessageContext(bot, chat, user, "/rules")

	if err := rulesModule.sendRules(bot, ctx); err != ext.EndGroups {
		t.Fatalf("sendRules() error = %v, want EndGroups", err)
	}

	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
	replyMarkup, ok := calls[0].Params["reply_markup"].(gotgbot.InlineKeyboardMarkup)
	if !ok {
		t.Fatalf("reply_markup = %#v, want InlineKeyboardMarkup", calls[0].Params["reply_markup"])
	}
	button := replyMarkup.InlineKeyboard[0][0]
	if button.Text != "Read rules" {
		t.Fatalf("rules button text = %q, want custom text", button.Text)
	}
	if !strings.Contains(button.Url, "start=rules_") {
		t.Fatalf("rules button URL = %q, want rules deep link", button.Url)
	}
}

func TestPrivateRulesTogglesAndReportsCurrentState(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chatID := uniqueModuleChatID()
	chat := gotgbot.Chat{Id: chatID, Type: "supergroup", Title: "Rules Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	onCtx := newModuleMessageContext(bot, chat, user, "/privaterules on")
	if err := rulesModule.privaterules(bot, onCtx); err != ext.EndGroups {
		t.Fatalf("privaterules on error = %v, want EndGroups", err)
	}
	if !rules.GetChatRulesInfo(chatID).Private {
		t.Fatal("private rules were not enabled")
	}

	currentCtx := newModuleMessageContext(bot, chat, user, "/privaterules")
	if err := rulesModule.privaterules(bot, currentCtx); err != ext.EndGroups {
		t.Fatalf("privaterules current error = %v, want EndGroups", err)
	}

	offCtx := newModuleMessageContext(bot, chat, user, "/privaterules false")
	if err := rulesModule.privaterules(bot, offCtx); err != ext.EndGroups {
		t.Fatalf("privaterules off error = %v, want EndGroups", err)
	}
	if rules.GetChatRulesInfo(chatID).Private {
		t.Fatal("private rules stayed enabled after off")
	}

	if calls := client.callsFor("sendMessage"); len(calls) != 3 {
		t.Fatalf("sendMessage calls = %d, want 3", len(calls))
	}
}

func TestRulesButtonSetViewAndReset(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chatID := uniqueModuleChatID()
	chat := gotgbot.Chat{Id: chatID, Type: "supergroup", Title: "Rules Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	setCtx := newModuleMessageContext(bot, chat, user, "/rulesbutton House Rules")
	if err := rulesModule.rulesBtn(bot, setCtx); err != ext.EndGroups {
		t.Fatalf("rulesBtn set error = %v, want EndGroups", err)
	}
	if got := rules.GetChatRulesInfo(chatID).RulesBtn; got != "House Rules" {
		t.Fatalf("RulesBtn = %q, want custom button", got)
	}

	viewCtx := newModuleMessageContext(bot, chat, user, "/rulesbutton")
	if err := rulesModule.rulesBtn(bot, viewCtx); err != ext.EndGroups {
		t.Fatalf("rulesBtn view error = %v, want EndGroups", err)
	}

	resetCtx := newModuleMessageContext(bot, chat, user, "/resetrulesbutton")
	if err := rulesModule.resetRulesBtn(bot, resetCtx); err != ext.EndGroups {
		t.Fatalf("resetRulesBtn error = %v, want EndGroups", err)
	}
	if got := rules.GetChatRulesInfo(chatID).RulesBtn; got != "" {
		t.Fatalf("RulesBtn = %q, want reset to empty", got)
	}

	if calls := client.callsFor("sendMessage"); len(calls) != 3 {
		t.Fatalf("sendMessage calls = %d, want 3", len(calls))
	}
}

func TestRulesButtonRejectsOverlongText(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chatID := uniqueModuleChatID()
	chat := gotgbot.Chat{Id: chatID, Type: "supergroup", Title: "Rules Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	longButton := strings.Repeat("x", 31)

	ctx := newModuleMessageContext(bot, chat, user, "/rulesbutton "+longButton)
	if err := rulesModule.rulesBtn(bot, ctx); err != ext.EndGroups {
		t.Fatalf("rulesBtn overlong error = %v, want EndGroups", err)
	}
	if got := rules.GetChatRulesInfo(chatID).RulesBtn; got != "" {
		t.Fatalf("RulesBtn = %q, want overlong text rejected", got)
	}
}

func TestClearRulesRemovesStoredRules(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chatID := uniqueModuleChatID()
	rules.SetChatRules(chatID, "existing rules")
	chat := gotgbot.Chat{Id: chatID, Type: "supergroup", Title: "Rules Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	ctx := newModuleMessageContext(bot, chat, user, "/clearrules")

	if err := rulesModule.clearRules(bot, ctx); err != ext.EndGroups {
		t.Fatalf("clearRules() error = %v, want EndGroups", err)
	}
	if got := rules.GetChatRulesInfo(chatID).Rules; got != "" {
		t.Fatalf("Rules = %q, want cleared", got)
	}
}
