package modules

import (
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/reports"
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

func TestReportRejectsInvalidReplyTargetsAndDisabledChats(t *testing.T) {
	tests := []struct {
		name        string
		reporter    gotgbot.User
		target      *gotgbot.User
		disable     bool
		wantReplies int
	}{
		{
			name:        "channel post target",
			reporter:    gotgbot.User{Id: 42, FirstName: "Reporter"},
			target:      nil,
			wantReplies: 1,
		},
		{
			name:        "self report",
			reporter:    gotgbot.User{Id: 42, FirstName: "Reporter"},
			target:      &gotgbot.User{Id: 42, FirstName: "Reporter"},
			wantReplies: 1,
		},
		{
			name:        "special reporter",
			reporter:    gotgbot.User{Id: 777000, FirstName: "Telegram"},
			target:      &gotgbot.User{Id: 43, FirstName: "Target"},
			wantReplies: 1,
		},
		{
			name:        "special target",
			reporter:    gotgbot.User{Id: 42, FirstName: "Reporter"},
			target:      &gotgbot.User{Id: 777000, FirstName: "Telegram"},
			wantReplies: 1,
		},
		{
			name:        "disabled chat stays silent",
			reporter:    gotgbot.User{Id: 42, FirstName: "Reporter"},
			target:      &gotgbot.User{Id: 43, FirstName: "Target"},
			disable:     true,
			wantReplies: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Report Chat"}
			if tt.disable {
				if err := reports.SetChatReportStatus(chat.Id, false); err != nil {
					t.Fatalf("SetChatReportStatus setup error = %v", err)
				}
			}
			ctx := newModuleMessageContext(bot, chat, tt.reporter, "/report")
			ctx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
				MessageId: 505,
				Date:      1,
				Chat:      chat,
				From:      tt.target,
				Text:      "reported message",
			}

			if err := reportsModule.report(bot, ctx); err != ext.EndGroups {
				t.Fatalf("report error = %v, want EndGroups", err)
			}
			if calls := client.callsFor("sendMessage"); len(calls) != tt.wantReplies {
				t.Fatalf("sendMessage calls = %d, want %d", len(calls), tt.wantReplies)
			}
		})
	}
}

func TestReportSkipsBlockedAdminBotAndAdminTargets(t *testing.T) {
	t.Run("blocked reporter command is deleted", func(t *testing.T) {
		client := newModuleBotClient()
		bot := newModuleTestBot(client)
		chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Report Chat"}
		reporter := gotgbot.User{Id: 42, FirstName: "Reporter"}
		target := gotgbot.User{Id: 43, FirstName: "Target"}
		if err := reports.BlockReportUser(chat.Id, reporter.Id); err != nil {
			t.Fatalf("BlockReportUser setup error = %v", err)
		}

		ctx := newReportReplyContext(bot, chat, reporter, target, "/report")
		if err := reportsModule.report(bot, ctx); err != ext.EndGroups {
			t.Fatalf("report(blocked) error = %v, want EndGroups", err)
		}
		if calls := client.callsFor("deleteMessage"); len(calls) != 1 {
			t.Fatalf("deleteMessage calls = %d, want blocked report command deletion", len(calls))
		}
		if calls := client.callsFor("sendMessage"); len(calls) != 0 {
			t.Fatalf("sendMessage calls = %d, want blocked reporter to stay silent", len(calls))
		}
	})

	t.Run("admin reporter", func(t *testing.T) {
		client := newModuleBotClient()
		client.responses["getChatMember"] = []byte(
			`{"status":"administrator","user":{"id":42,"is_bot":false,"first_name":"Reporter"},"can_delete_messages":true}`,
		)
		bot := newModuleTestBot(client)
		chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Report Chat"}
		reporter := gotgbot.User{Id: 42, FirstName: "Reporter"}
		target := gotgbot.User{Id: 43, FirstName: "Target"}

		ctx := newReportReplyContext(bot, chat, reporter, target, "/report")
		if err := reportsModule.report(bot, ctx); err != ext.EndGroups {
			t.Fatalf("report(admin reporter) error = %v, want EndGroups", err)
		}
		if calls := client.callsFor("sendMessage"); len(calls) != 1 {
			t.Fatalf("sendMessage calls = %d, want admin reporter warning", len(calls))
		}
	})

	for _, tt := range []struct {
		name   string
		target gotgbot.User
	}{
		{name: "bot target", target: gotgbot.User{Id: 999, FirstName: "Alita", IsBot: true}},
		{name: "admin target", target: gotgbot.User{Id: 777000, FirstName: "Telegram"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Report Chat"}
			reporter := gotgbot.User{Id: 42, FirstName: "Reporter"}

			ctx := newReportReplyContext(bot, chat, reporter, tt.target, "/report")
			if err := reportsModule.report(bot, ctx); err != ext.EndGroups {
				t.Fatalf("report(%s) error = %v, want EndGroups", tt.name, err)
			}
			if calls := client.callsFor("sendMessage"); len(calls) != 1 {
				t.Fatalf("sendMessage calls = %d, want target warning", len(calls))
			}
		})
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
	if reports.GetChatReportSettings(chat.Id).Status {
		t.Fatal("reports setting is still enabled after /reports off")
	}

	onCtx := newModuleMessageContext(bot, chat, admin, "/reports on")
	if err := reportsModule.reports(bot, onCtx); err != ext.EndGroups {
		t.Fatalf("reports on error = %v, want EndGroups", err)
	}
	if !reports.GetChatReportSettings(chat.Id).Status {
		t.Fatal("reports setting is still disabled after /reports on")
	}

	blockCtx := newReportReplyContext(bot, chat, admin, target, "/reports block")
	if err := reportsModule.reports(bot, blockCtx); err != ext.EndGroups {
		t.Fatalf("reports block error = %v, want EndGroups", err)
	}
	if got := reports.GetChatReportSettings(chat.Id).BlockedList; len(got) != 1 || got[0] != target.Id {
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
	if got := reports.GetChatReportSettings(chat.Id).BlockedList; len(got) != 0 {
		t.Fatalf("blocked list after unblock = %v, want empty", got)
	}
}

func TestReportsSettingsPrivateAndValidationBranches(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	privateChat := gotgbot.Chat{Id: admin.Id, Type: "private", FirstName: "Telegram"}
	connectedChat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Connected Report Chat"}
	client.responses["getChat"] = []byte(`{"id":-100777,"type":"supergroup","title":"Connected Report Chat"}`)
	connections.ConnectId(admin.Id, connectedChat.Id)
	t.Cleanup(func() {
		connections.DisconnectId(admin.Id)
	})

	privateOnCtx := newModuleMessageContext(bot, privateChat, admin, "/reports on")
	if err := reportsModule.reports(bot, privateOnCtx); err != ext.EndGroups {
		t.Fatalf("private reports on error = %v, want EndGroups", err)
	}
	if !reports.GetUserReportSettings(admin.Id).Status {
		t.Fatal("private report setting did not enable")
	}

	privateOffCtx := newModuleMessageContext(bot, privateChat, admin, "/reports off")
	if err := reportsModule.reports(bot, privateOffCtx); err != ext.EndGroups {
		t.Fatalf("private reports off error = %v, want EndGroups", err)
	}
	if reports.GetUserReportSettings(admin.Id).Status {
		t.Fatal("private report setting did not disable")
	}

	for _, text := range []string{
		"/reports",
		"/reports maybe",
		"/reports block",
		"/reports unblock",
		"/reports showblocklist",
	} {
		ctx := newModuleMessageContext(bot, privateChat, admin, text)
		if err := reportsModule.reports(bot, ctx); err != ext.EndGroups {
			t.Fatalf("%s error = %v, want EndGroups", text, err)
		}
	}

	groupChat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Report Chat"}
	for _, text := range []string{"/reports block", "/reports unblock", "/reports unknown"} {
		ctx := newModuleMessageContext(bot, groupChat, admin, text)
		if err := reportsModule.reports(bot, ctx); err != ext.EndGroups {
			t.Fatalf("%s group error = %v, want EndGroups", text, err)
		}
	}

	nilTargetCtx := newModuleMessageContext(bot, groupChat, admin, "/reports block")
	nilTargetCtx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{MessageId: 505, Chat: groupChat}
	if err := reportsModule.reports(bot, nilTargetCtx); err != ext.EndGroups {
		t.Fatalf("block nil target error = %v, want EndGroups", err)
	}

	if calls := client.callsFor("sendMessage"); len(calls) != 11 {
		t.Fatalf("sendMessage calls = %d, want one response per settings command", len(calls))
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

func TestReportActionCallbacksKickAndInvalidData(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Report Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	kickCtx := newModuleCallbackContext(bot, chat, admin, "report.kick=42=505")
	if err := reportsModule.markResolvedButtonHandler(bot, kickCtx); err != ext.EndGroups {
		t.Fatalf("report kick callback error = %v, want EndGroups", err)
	}
	// Wait for the async kick-unban goroutine to complete
	time.Sleep(1500 * time.Millisecond)
	if calls := client.callsFor("banChatMember"); len(calls) != 1 {
		t.Fatalf("banChatMember calls = %d, want kick ban action", len(calls))
	}
	if calls := client.callsFor("unbanChatMember"); len(calls) != 1 {
		t.Fatalf("unbanChatMember calls = %d, want kick unban action", len(calls))
	}

	for _, data := range []string{
		"report.invalid",
		encodeCallbackData("report", map[string]string{"a": "ban", "u": "nan", "m": "505"}, "report.ban=nan=505"),
		encodeCallbackData("report", map[string]string{"a": "ban", "u": "42", "m": "nan"}, "report.ban=42=nan"),
	} {
		ctx := newModuleCallbackContext(bot, chat, admin, data)
		if err := reportsModule.markResolvedButtonHandler(bot, ctx); err != ext.EndGroups {
			t.Fatalf("report invalid callback %q error = %v, want EndGroups", data, err)
		}
	}

	if calls := client.callsFor("editMessageText"); len(calls) != 1 {
		t.Fatalf("editMessageText calls = %d, want only valid kick edit", len(calls))
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 4 {
		t.Fatalf("answerCallbackQuery calls = %d, want kick plus three invalid acknowledgements", len(calls))
	}
}
