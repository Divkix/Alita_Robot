package modules

import (
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
)

func newWarnReplyContext(
	bot *gotgbot.Bot,
	chat gotgbot.Chat,
	admin gotgbot.User,
	target gotgbot.User,
	text string,
) *ext.Context {
	ctx := newModuleMessageContext(bot, chat, admin, text)
	ctx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
		MessageId: 202,
		Date:      1,
		Chat:      chat,
		From:      &target,
		Text:      "message being warned",
	}
	return ctx
}

func TestWarnSettingsCommandsUpdateAndDisplay(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	limitCtx := newModuleMessageContext(bot, chat, admin, "/setwarnlimit 5")
	if err := warnsModule.setWarnLimit(bot, limitCtx); err != ext.EndGroups {
		t.Fatalf("setWarnLimit() error = %v, want EndGroups", err)
	}
	if got := db.GetWarnSetting(chat.Id).WarnLimit; got != 5 {
		t.Fatalf("warn limit = %d, want 5", got)
	}

	invalidCtx := newModuleMessageContext(bot, chat, admin, "/setwarnlimit 0")
	if err := warnsModule.setWarnLimit(bot, invalidCtx); err != ext.EndGroups {
		t.Fatalf("setWarnLimit invalid error = %v, want EndGroups", err)
	}
	if got := db.GetWarnSetting(chat.Id).WarnLimit; got != 5 {
		t.Fatalf("invalid warn limit changed setting to %d", got)
	}

	modeCtx := newModuleMessageContext(bot, chat, admin, "/setwarnmode ban")
	if err := warnsModule.setWarnMode(bot, modeCtx); err != ext.EndGroups {
		t.Fatalf("setWarnMode() error = %v, want EndGroups", err)
	}
	if got := db.GetWarnSetting(chat.Id).WarnMode; got != "ban" {
		t.Fatalf("warn mode = %q, want ban", got)
	}

	displayCtx := newModuleMessageContext(bot, chat, admin, "/warnings")
	if err := warnsModule.warnings(bot, displayCtx); err != ext.EndGroups {
		t.Fatalf("warnings() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) < 4 {
		t.Fatalf("sendMessage calls = %d, want at least 4", len(calls))
	}
}

func TestWarnReplyStoresReasonAndWarnsListsIt(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	target := gotgbot.User{Id: 42, FirstName: "Member"}

	warnCtx := newWarnReplyContext(bot, chat, admin, target, "/warn too noisy")
	if err := warnsModule.warnUser(bot, warnCtx); err != ext.EndGroups {
		t.Fatalf("warnUser() error = %v, want EndGroups", err)
	}
	numWarns, reasons := db.GetWarns(target.Id, chat.Id)
	if numWarns != 1 {
		t.Fatalf("numWarns = %d, want 1", numWarns)
	}
	if len(reasons) != 1 || reasons[0] != "too noisy" {
		t.Fatalf("reasons = %v, want [too noisy]", reasons)
	}

	listCtx := newModuleMessageContext(bot, chat, admin, "/warns 42")
	if err := warnsModule.warns(bot, listCtx); err != ext.EndGroups {
		t.Fatalf("warns() error = %v, want EndGroups", err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) < 2 {
		t.Fatalf("sendMessage calls = %d, want warn and list replies", len(calls))
	}
	lastText := calls[len(calls)-1].Params["text"].(string)
	if !strings.Contains(lastText, "too noisy") {
		t.Fatalf("warn list text = %q, want reason", lastText)
	}
}

func TestWarnLimitPunishesAndResetsWarnings(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	target := gotgbot.User{Id: 42, FirstName: "Member"}
	if err := db.SetWarnLimit(chat.Id, 1); err != nil {
		t.Fatalf("SetWarnLimit() error = %v", err)
	}
	if err := db.SetWarnMode(chat.Id, "ban"); err != nil {
		t.Fatalf("SetWarnMode() error = %v", err)
	}

	warnCtx := newWarnReplyContext(bot, chat, admin, target, "/warn limit reached")
	if err := warnsModule.warnUser(bot, warnCtx); err != ext.EndGroups {
		t.Fatalf("warnUser() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("banChatMember"); len(calls) != 1 {
		t.Fatalf("banChatMember calls = %d, want 1", len(calls))
	}
	if numWarns, _ := db.GetWarns(target.Id, chat.Id); numWarns != 0 {
		t.Fatalf("numWarns after punishment = %d, want reset to 0", numWarns)
	}
}

func TestSilentWarnDeletesCommandAndStoresReason(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	target := gotgbot.User{Id: 42, FirstName: "Member"}

	ctx := newWarnReplyContext(bot, chat, admin, target, "/swarn quiet reason")
	if err := warnsModule.sWarnUser(bot, ctx); err != ext.EndGroups {
		t.Fatalf("sWarnUser() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != 1 {
		t.Fatalf("deleteMessage calls = %d, want command deletion", len(calls))
	}
	numWarns, reasons := db.GetWarns(target.Id, chat.Id)
	if numWarns != 1 {
		t.Fatalf("numWarns = %d, want 1", numWarns)
	}
	if len(reasons) != 1 || reasons[0] != "quiet reason" {
		t.Fatalf("reasons = %v, want [quiet reason]", reasons)
	}
}

func TestDeleteWarnDeletesReplyAndStoresReason(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	target := gotgbot.User{Id: 42, FirstName: "Member"}

	ctx := newWarnReplyContext(bot, chat, admin, target, "/dwarn remove this")
	if err := warnsModule.dWarnUser(bot, ctx); err != ext.EndGroups {
		t.Fatalf("dWarnUser() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != 1 {
		t.Fatalf("deleteMessage calls = %d, want replied message deletion", len(calls))
	}
	numWarns, reasons := db.GetWarns(target.Id, chat.Id)
	if numWarns != 1 {
		t.Fatalf("numWarns = %d, want 1", numWarns)
	}
	if len(reasons) != 1 || reasons[0] != "remove this" {
		t.Fatalf("reasons = %v, want [remove this]", reasons)
	}
}

func TestRemoveWarnAndResetWarnsCommands(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	target := gotgbot.User{Id: 42, FirstName: "Member"}
	db.WarnUser(target.Id, chat.Id, "first")

	removeCtx := newModuleMessageContext(bot, chat, admin, "/rmwarn 42")
	if err := warnsModule.removeWarn(bot, removeCtx); err != ext.EndGroups {
		t.Fatalf("removeWarn() error = %v, want EndGroups", err)
	}
	if numWarns, _ := db.GetWarns(target.Id, chat.Id); numWarns != 0 {
		t.Fatalf("numWarns after remove = %d, want 0", numWarns)
	}

	db.WarnUser(target.Id, chat.Id, "one")
	db.WarnUser(target.Id, chat.Id, "two")
	resetCtx := newModuleMessageContext(bot, chat, admin, "/resetwarns 42")
	if err := warnsModule.resetWarns(bot, resetCtx); err != ext.EndGroups {
		t.Fatalf("resetWarns() error = %v, want EndGroups", err)
	}
	if numWarns, _ := db.GetWarns(target.Id, chat.Id); numWarns != 0 {
		t.Fatalf("numWarns after reset = %d, want 0", numWarns)
	}
}

func TestRmWarnButtonRemovesWarning(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	target := gotgbot.User{Id: 42, FirstName: "Member"}
	db.WarnUser(target.Id, chat.Id, "button")
	data := encodeCallbackData("rmWarn", map[string]string{"u": "42"}, "rmWarn.42")

	ctx := newModuleCallbackContext(bot, chat, admin, data)
	if err := warnsModule.rmWarnButton(bot, ctx); err != ext.EndGroups {
		t.Fatalf("rmWarnButton() error = %v, want EndGroups", err)
	}
	if numWarns, _ := db.GetWarns(target.Id, chat.Id); numWarns != 0 {
		t.Fatalf("numWarns after callback remove = %d, want 0", numWarns)
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 1 {
		t.Fatalf("answerCallbackQuery calls = %d, want 1", len(calls))
	}
}

func TestResetAllWarnsConfirmationAndCallback(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	db.WarnUser(42, chat.Id, "first")
	data := encodeCallbackData("rmAllChatWarns", map[string]string{"a": "yes"}, "rmAllChatWarns.yes")

	confirmCtx := newModuleMessageContext(bot, chat, admin, "/resetallwarns")
	if err := warnsModule.resetAllWarns(bot, confirmCtx); err != ext.EndGroups {
		t.Fatalf("resetAllWarns() error = %v, want EndGroups", err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want confirmation reply", len(calls))
	}
	if calls[0].Params["reply_markup"] == nil {
		t.Fatal("resetAllWarns confirmation did not include reply_markup")
	}

	callbackCtx := newModuleCallbackContext(bot, chat, admin, data)
	if err := warnsModule.warnsButtonHandler(bot, callbackCtx); err != ext.EndGroups {
		t.Fatalf("warnsButtonHandler() error = %v, want EndGroups", err)
	}
	if got := db.GetAllChatWarns(chat.Id); got != 0 {
		t.Fatalf("GetAllChatWarns() = %d, want 0 after reset all", got)
	}
}
