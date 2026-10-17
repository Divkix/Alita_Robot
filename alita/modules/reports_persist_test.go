package modules

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/reports"
)

func storedReports(t *testing.T, chatID int64) []models.Report {
	t.Helper()
	var rows []models.Report
	if err := db.DB.Where("chat_id = ?", chatID).Order("id").Find(&rows).Error; err != nil {
		t.Fatalf("load stored reports: %v", err)
	}
	return rows
}

func TestReportPersistsRow(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Report Chat"}
	reporter := gotgbot.User{Id: 42, FirstName: "Reporter"}
	target := gotgbot.User{Id: 43, FirstName: "Target"}

	ctx := newReportReplyContext(bot, chat, reporter, target, "/report spam links")
	if err := reportsModule.report(bot, ctx); err != ext.EndGroups {
		t.Fatalf("report error = %v, want EndGroups", err)
	}

	rows := storedReports(t, chat.Id)
	if len(rows) != 1 {
		t.Fatalf("stored reports = %d, want 1", len(rows))
	}
	got := rows[0]
	if got.ReporterId != reporter.Id || got.ReportedUserId != target.Id || got.ReportedMessageId != 505 {
		t.Fatalf("stored report = %+v, want reporter 42, target 43, message 505", got)
	}
	if got.Reason != "spam links" {
		t.Fatalf("stored reason = %q, want %q", got.Reason, "spam links")
	}
	if got.Status != models.ReportStatusOpen {
		t.Fatalf("stored status = %q, want open", got.Status)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 || calls[0].Params["reply_markup"] == nil {
		t.Fatalf("group reply calls = %d, want the unchanged admin report with buttons", len(calls))
	}

	again := newReportReplyContext(bot, chat, reporter, target, "/report again")
	if err := reportsModule.report(bot, again); err != ext.EndGroups {
		t.Fatalf("repeat report error = %v, want EndGroups", err)
	}
	if rows := storedReports(t, chat.Id); len(rows) != 1 {
		t.Fatalf("stored reports after repeat = %d, want 1", len(rows))
	}
}

func TestAdminMentionPersistsRow(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Report Chat"}
	reporter := gotgbot.User{Id: 42, FirstName: "Reporter"}
	target := gotgbot.User{Id: 43, FirstName: "Target"}

	ctx := newReportReplyContext(bot, chat, reporter, target, "scam link @admin")
	if err := reportsModule.report(bot, ctx); err != ext.EndGroups {
		t.Fatalf("report error = %v, want EndGroups", err)
	}

	rows := storedReports(t, chat.Id)
	if len(rows) != 1 {
		t.Fatalf("stored reports = %d, want 1", len(rows))
	}
	if rows[0].Reason != "scam link @admin" {
		t.Fatalf("stored reason = %q, want the full message text", rows[0].Reason)
	}
}

func TestReportReasonIsCapped(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Report Chat"}
	reporter := gotgbot.User{Id: 42, FirstName: "Reporter"}
	target := gotgbot.User{Id: 43, FirstName: "Target"}

	long := make([]rune, maxReportReasonRunes+50)
	for i := range long {
		long[i] = 'ж'
	}
	ctx := newReportReplyContext(bot, chat, reporter, target, "/report "+string(long))
	if err := reportsModule.report(bot, ctx); err != ext.EndGroups {
		t.Fatalf("report error = %v, want EndGroups", err)
	}

	rows := storedReports(t, chat.Id)
	if len(rows) != 1 {
		t.Fatalf("stored reports = %d, want 1", len(rows))
	}
	if got := len([]rune(rows[0].Reason)); got != maxReportReasonRunes {
		t.Fatalf("stored reason runes = %d, want %d", got, maxReportReasonRunes)
	}
}

func TestReportNotPersistedWhenRejected(t *testing.T) {
	reporter := gotgbot.User{Id: 42, FirstName: "Reporter"}
	target := gotgbot.User{Id: 43, FirstName: "Target"}

	t.Run("blocked reporter", func(t *testing.T) {
		client := newModuleBotClient()
		bot := newModuleTestBot(client)
		chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Report Chat"}
		if err := reports.BlockReportUser(chat.Id, reporter.Id); err != nil {
			t.Fatalf("BlockReportUser setup error = %v", err)
		}
		ctx := newReportReplyContext(bot, chat, reporter, target, "/report")
		if err := reportsModule.report(bot, ctx); err != ext.EndGroups {
			t.Fatalf("report error = %v, want EndGroups", err)
		}
		if rows := storedReports(t, chat.Id); len(rows) != 0 {
			t.Fatalf("stored reports = %d, want 0 for a blocked reporter", len(rows))
		}
	})

	t.Run("reports disabled", func(t *testing.T) {
		client := newModuleBotClient()
		bot := newModuleTestBot(client)
		chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Report Chat"}
		if err := reports.SetChatReportStatus(chat.Id, false); err != nil {
			t.Fatalf("SetChatReportStatus setup error = %v", err)
		}
		ctx := newReportReplyContext(bot, chat, reporter, target, "/report")
		if err := reportsModule.report(bot, ctx); err != ext.EndGroups {
			t.Fatalf("report error = %v, want EndGroups", err)
		}
		if rows := storedReports(t, chat.Id); len(rows) != 0 {
			t.Fatalf("stored reports = %d, want 0 when reports are off", len(rows))
		}
	})

	t.Run("admin target", func(t *testing.T) {
		client := newModuleBotClient()
		bot := newModuleTestBot(client)
		chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Report Chat"}
		seedCallbackAdmins(t, chat.Id, gotgbot.MergedChatMember{User: target, Status: "administrator"})
		ctx := newReportReplyContext(bot, chat, reporter, target, "/report")
		if err := reportsModule.report(bot, ctx); err != ext.EndGroups {
			t.Fatalf("report error = %v, want EndGroups", err)
		}
		if rows := storedReports(t, chat.Id); len(rows) != 0 {
			t.Fatalf("stored reports = %d, want 0 for an admin target", len(rows))
		}
		if calls := client.callsFor("sendMessage"); len(calls) != 1 || calls[0].Params["reply_markup"] != nil {
			t.Fatalf("group reply calls = %d, want one plain refusal without report buttons", len(calls))
		}
	})

	t.Run("special account target", func(t *testing.T) {
		client := newModuleBotClient()
		bot := newModuleTestBot(client)
		chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Report Chat"}
		telegram := gotgbot.User{Id: 777000, FirstName: "Telegram"}
		ctx := newReportReplyContext(bot, chat, reporter, telegram, "/report")
		if err := reportsModule.report(bot, ctx); err != ext.EndGroups {
			t.Fatalf("report error = %v, want EndGroups", err)
		}
		if rows := storedReports(t, chat.Id); len(rows) != 0 {
			t.Fatalf("stored reports = %d, want 0 for a special account target", len(rows))
		}
	})
}

func TestReportStillReachesGroupWhenStoreFails(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Report Chat"}
	reporter := gotgbot.User{Id: 42, FirstName: "Reporter"}
	target := gotgbot.User{Id: 43, FirstName: "Target"}

	if err := db.DB.Migrator().DropTable(&models.Report{}); err != nil {
		t.Fatalf("drop reports table: %v", err)
	}
	t.Cleanup(func() {
		if err := db.DB.AutoMigrate(&models.Report{}); err != nil {
			t.Fatalf("restore reports table: %v", err)
		}
	})

	ctx := newReportReplyContext(bot, chat, reporter, target, "/report spam")
	if err := reportsModule.report(bot, ctx); err != ext.EndGroups {
		t.Fatalf("report error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 || calls[0].Params["reply_markup"] == nil {
		t.Fatalf("group reply calls = %d, want the admin report with buttons despite the store failure", len(calls))
	}
}
