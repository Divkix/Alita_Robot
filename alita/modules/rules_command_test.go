package modules

import (
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
)

func uniqueModuleChatID() int64 {
	return -1000000000000 - time.Now().UnixNano()%1_000_000_000
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

	rules := db.GetChatRulesInfo(chatID)
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
	db.SetChatRules(chatID, "existing rules")
	chat := gotgbot.Chat{Id: chatID, Type: "supergroup", Title: "Rules Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	ctx := newModuleMessageContext(bot, chat, user, "/setrules")

	if err := rulesModule.setRules(bot, ctx); err != ext.EndGroups {
		t.Fatalf("setRules() error = %v, want EndGroups", err)
	}

	if got := db.GetChatRulesInfo(chatID).Rules; got != "existing rules" {
		t.Fatalf("stored rules = %q, want existing rules untouched", got)
	}
}

func TestSendRulesUsesPrivateButtonWhenEnabled(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chatID := uniqueModuleChatID()
	db.SetChatRules(chatID, "Read before posting")
	db.SetPrivateRules(chatID, true)
	db.SetChatRulesButton(chatID, "Read rules")

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
