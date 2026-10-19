package modules

import (
	"slices"
	"strings"
	"testing"
	"time"

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

func TestReportReasonReadFromCaption(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Report Chat"}
	reporter := gotgbot.User{Id: 42, FirstName: "Reporter"}
	target := gotgbot.User{Id: 43, FirstName: "Target"}

	// A photo captioned "/report scam": the command is in the caption and
	// the message has no text.
	ctx := newReportReplyContext(bot, chat, reporter, target, "")
	ctx.EffectiveMessage.Caption = "/report scam"
	if err := reportsModule.report(bot, ctx); err != ext.EndGroups {
		t.Fatalf("report error = %v, want EndGroups", err)
	}

	rows := storedReports(t, chat.Id)
	if len(rows) != 1 || rows[0].Reason != "scam" {
		t.Fatalf("stored reports = %+v, want one with the caption reason %q", rows, "scam")
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

func TestReportStoresShortSnippetOfMessage(t *testing.T) {
	long := strings.Repeat("ж", models.MaxReportSnippetRunes+50)
	for name, tc := range map[string]struct {
		text, caption, want string
	}{
		"long text":    {text: long, want: strings.Repeat("ж", models.MaxReportSnippetRunes)},
		"caption":      {caption: "photo caption", want: "photo caption"},
		"no text":      {want: ""},
		"text wins":    {text: "text", caption: "caption", want: "text"},
		"short intact": {text: "buy cheap now", want: "buy cheap now"},
	} {
		t.Run(name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Report Chat"}
			reporter := gotgbot.User{Id: 42, FirstName: "Reporter"}
			target := gotgbot.User{Id: 43, FirstName: "Target"}

			ctx := newReportReplyContext(bot, chat, reporter, target, "/report spam")
			ctx.EffectiveMessage.ReplyToMessage.Text = tc.text
			ctx.EffectiveMessage.ReplyToMessage.Caption = tc.caption
			if err := reportsModule.report(bot, ctx); err != ext.EndGroups {
				t.Fatalf("report error = %v, want EndGroups", err)
			}

			rows := storedReports(t, chat.Id)
			if len(rows) != 1 || rows[0].Snippet != tc.want {
				t.Fatalf("stored rows = %+v, want snippet %q", rows, tc.want)
			}
		})
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

func TestSweepDeletesReportsOver90DaysOld(t *testing.T) {
	chatID := uniqueModuleChatID()
	now := time.Now()
	// 3: still open, made 91 days ago; 4: open, made 89 days ago.
	for msgID, madeAt := range map[int64]time.Time{
		3: now.Add(-91 * 24 * time.Hour),
		4: now.Add(-89 * 24 * time.Hour),
	} {
		if _, err := reports.Create(&models.Report{
			ChatId: chatID, ReportedUserId: 43, ReportedMessageId: msgID, ReporterId: 42,
		}); err != nil {
			t.Fatalf("Create(%d) error = %v", msgID, err)
		}
		if err := db.DB.Model(&models.Report{}).
			Where("chat_id = ? AND reported_message_id = ?", chatID, msgID).
			Update("created_at", madeAt).Error; err != nil {
			t.Fatalf("backdate report %d: %v", msgID, err)
		}
	}
	for msgID, closedAt := range map[int64]time.Time{
		1: now.Add(-91 * 24 * time.Hour),
		2: now.Add(-89 * 24 * time.Hour),
	} {
		if _, err := reports.Create(&models.Report{
			ChatId: chatID, ReportedUserId: 43, ReportedMessageId: msgID, ReporterId: 42,
		}); err != nil {
			t.Fatalf("Create(%d) error = %v", msgID, err)
		}
		if _, err := reports.Resolve(chatID, msgID, models.ReportStatusDismissed, 7); err != nil {
			t.Fatalf("Resolve(%d) error = %v", msgID, err)
		}
		if err := db.DB.Model(&models.Report{}).
			Where("chat_id = ? AND reported_message_id = ?", chatID, msgID).
			Update("resolved_at", closedAt).Error; err != nil {
			t.Fatalf("backdate report %d: %v", msgID, err)
		}
	}

	sweepExpiredReports(now)

	rows := storedReports(t, chatID)
	var left []int64
	for _, row := range rows {
		left = append(left, row.ReportedMessageId)
	}
	slices.Sort(left)
	if !slices.Equal(left, []int64{2, 4}) {
		t.Fatalf("reports left = %v, want [2 4]: closed or made within 90 days", left)
	}
}

func TestLegacyButtonRefusesNonAdmin(t *testing.T) {
	for _, action := range []string{"kick", "ban", "delete", "resolved"} {
		t.Run(action, func(t *testing.T) {
			f := newReportCardFixture(t, false)
			seedCallbackAdmins(t, f.chat.Id,
				gotgbot.MergedChatMember{
					Status: "administrator", User: gotgbot.User{Id: 999, IsBot: true},
					CanRestrictMembers: true, CanDeleteMessages: true,
				},
				gotgbot.MergedChatMember{Status: "creator", User: gotgbot.User{Id: reportCardAdminID, FirstName: "Group Admin"}},
			)
			f.report(t, f.reporter, "/report spam")
			f.client.calls = nil

			data := encodeCallbackData("report", map[string]string{"a": action, "u": "43", "m": "505"})
			if err := reportsModule.markResolvedButtonHandler(f.bot, newModuleCallbackContext(f.bot, f.chat, f.reporter, data)); err != ext.EndGroups {
				t.Fatalf("legacy %s by a member error = %v, want EndGroups", action, err)
			}

			rows := storedReports(t, f.chat.Id)
			if len(rows) != 1 || rows[0].Status != models.ReportStatusOpen || rows[0].ResolvedBy != nil {
				t.Fatalf("stored reports = %+v, want the report left open", rows)
			}
			for _, method := range []string{"banChatMember", "unbanChatMember", "deleteMessage", "editMessageText"} {
				if calls := f.client.callsFor(method); len(calls) != 0 {
					t.Fatalf("%s calls = %d, want none for a member", method, len(calls))
				}
			}
			if calls := f.client.callsFor("answerCallbackQuery"); len(calls) != 1 {
				t.Fatalf("answerCallbackQuery calls = %d, want one refusal", len(calls))
			}
		})
	}
}

func TestLegacyButtonResolvesRows(t *testing.T) {
	admin := gotgbot.User{Id: reportCardAdminID, FirstName: "Group Admin"}
	for action, status := range map[string]string{
		"kick":     models.ReportStatusKicked,
		"ban":      models.ReportStatusBanned,
		"delete":   models.ReportStatusDeleted,
		"resolved": models.ReportStatusDismissed,
	} {
		t.Run(action, func(t *testing.T) {
			f := newReportCardFixture(t, false)
			seedCallbackAdmins(t, f.chat.Id,
				gotgbot.MergedChatMember{
					Status: "administrator", User: gotgbot.User{Id: 999, IsBot: true},
					CanRestrictMembers: true, CanDeleteMessages: true,
				},
				gotgbot.MergedChatMember{Status: "creator", User: admin},
			)
			f.report(t, f.reporter, "/report spam")
			data := encodeCallbackData("report", map[string]string{"a": action, "u": "43", "m": "505"})
			if err := reportsModule.markResolvedButtonHandler(f.bot, newModuleCallbackContext(f.bot, f.chat, admin, data)); err != ext.EndGroups {
				t.Fatalf("legacy %s error = %v, want EndGroups", action, err)
			}

			rows := storedReports(t, f.chat.Id)
			if len(rows) != 1 || rows[0].Status != status {
				t.Fatalf("stored reports = %+v, want one with status %q", rows, status)
			}
			if rows[0].ResolvedBy == nil || *rows[0].ResolvedBy != admin.Id || rows[0].ResolvedAt == nil {
				t.Fatalf("resolved_by = %v, resolved_at = %v; want %d and a time", rows[0].ResolvedBy, rows[0].ResolvedAt, admin.Id)
			}
		})
	}
}
