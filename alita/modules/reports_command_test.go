package modules

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
)

func newReportReplyContext(
	bot *gotgbot.Bot,
	chat gotgbot.Chat,
	reporter gotgbot.User,
	target gotgbot.User,
	text string,
) *ext.Context {
	ctx := newModuleMessageContext(bot, chat, reporter, text)
	ctx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
		MessageId: 505,
		Date:      1,
		Chat:      chat,
		From:      &target,
		Text:      "reported message",
	}
	return ctx
}

func TestReportRequiresReplyAndSendsAdminReport(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Report Chat"}
	reporter := gotgbot.User{Id: 42, FirstName: "Reporter"}
	target := gotgbot.User{Id: 43, FirstName: "Target"}

	noReplyCtx := newModuleMessageContext(bot, chat, reporter, "/report")
	if err := reportsModule.report(bot, noReplyCtx); err != ext.EndGroups {
		t.Fatalf("report no reply error = %v, want EndGroups", err)
	}

	reportCtx := newReportReplyContext(bot, chat, reporter, target, "/report spam")
	if err := reportsModule.report(bot, reportCtx); err != ext.EndGroups {
		t.Fatalf("report reply error = %v, want EndGroups", err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) < 2 {
		t.Fatalf("sendMessage calls = %d, want validation and report messages", len(calls))
	}
	last := calls[len(calls)-1]
	if last.Params["reply_markup"] == nil {
		t.Fatal("report message did not include action buttons")
	}
}

func TestReportsSettingsBlockUnblockAndShowBlocklist(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Report Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	target := gotgbot.User{Id: 42, FirstName: "Member"}

	offCtx := newModuleMessageContext(bot, chat, admin, "/reports off")
	if err := reportsModule.reports(bot, offCtx); err != ext.EndGroups {
		t.Fatalf("reports off error = %v, want EndGroups", err)
	}
	if db.GetChatReportSettings(chat.Id).Status {
		t.Fatal("reports setting is still enabled after /reports off")
	}

	onCtx := newModuleMessageContext(bot, chat, admin, "/reports on")
	if err := reportsModule.reports(bot, onCtx); err != ext.EndGroups {
		t.Fatalf("reports on error = %v, want EndGroups", err)
	}
	if !db.GetChatReportSettings(chat.Id).Status {
		t.Fatal("reports setting is still disabled after /reports on")
	}

	blockCtx := newReportReplyContext(bot, chat, admin, target, "/reports block")
	if err := reportsModule.reports(bot, blockCtx); err != ext.EndGroups {
		t.Fatalf("reports block error = %v, want EndGroups", err)
	}
	if got := db.GetChatReportSettings(chat.Id).BlockedList; len(got) != 1 || got[0] != target.Id {
		t.Fatalf("blocked list = %v, want target user", got)
	}

	showCtx := newModuleMessageContext(bot, chat, admin, "/reports showblocklist")
	if err := reportsModule.reports(bot, showCtx); err != ext.EndGroups {
		t.Fatalf("reports showblocklist error = %v, want EndGroups", err)
	}

	unblockCtx := newReportReplyContext(bot, chat, admin, target, "/reports unblock")
	if err := reportsModule.reports(bot, unblockCtx); err != ext.EndGroups {
		t.Fatalf("reports unblock error = %v, want EndGroups", err)
	}
	if got := db.GetChatReportSettings(chat.Id).BlockedList; len(got) != 0 {
		t.Fatalf("blocked list after unblock = %v, want empty", got)
	}
}

func TestReportActionCallbacksBanDeleteAndResolve(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Report Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	banData := encodeCallbackData("report", map[string]string{
		"a": "ban",
		"u": "42",
		"m": "505",
	}, "report.ban=42=505")
	banCtx := newModuleCallbackContext(bot, chat, admin, banData)
	if err := reportsModule.markResolvedButtonHandler(bot, banCtx); err != ext.EndGroups {
		t.Fatalf("report ban callback error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("banChatMember"); len(calls) != 1 {
		t.Fatalf("banChatMember calls = %d, want ban action", len(calls))
	}

	deleteData := encodeCallbackData("report", map[string]string{
		"a": "delete",
		"u": "42",
		"m": "505",
	}, "report.delete=42=505")
	deleteCtx := newModuleCallbackContext(bot, chat, admin, deleteData)
	if err := reportsModule.markResolvedButtonHandler(bot, deleteCtx); err != ext.EndGroups {
		t.Fatalf("report delete callback error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != 1 {
		t.Fatalf("deleteMessage calls = %d, want delete action", len(calls))
	}

	resolvedData := encodeCallbackData("report", map[string]string{
		"a": "resolved",
		"u": "42",
		"m": "505",
	}, "report.resolved=42=505")
	resolvedCtx := newModuleCallbackContext(bot, chat, admin, resolvedData)
	if err := reportsModule.markResolvedButtonHandler(bot, resolvedCtx); err != ext.EndGroups {
		t.Fatalf("report resolved callback error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("editMessageText"); len(calls) != 3 {
		t.Fatalf("editMessageText calls = %d, want one edit per callback", len(calls))
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 3 {
		t.Fatalf("answerCallbackQuery calls = %d, want one answer per callback", len(calls))
	}
}
