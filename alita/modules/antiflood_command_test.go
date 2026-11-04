package modules

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/antiflood"
	"github.com/divkix/Alita_Robot/alita/db/approvals"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

func resetAntifloodState(t *testing.T) {
	t.Helper()

	antifloodModule.syncHelperMap.Range(func(key, _ any) bool {
		antifloodModule.syncHelperMap.Delete(key)
		return true
	})
}

func TestAntifloodCommandsUpdateSettingsAndDisplay(t *testing.T) {
	resetAntifloodState(t)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Flood Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	setCtx := newModuleMessageContext(bot, chat, admin, "/setflood 4")
	if err := antifloodModule.setFlood(bot, setCtx); err != ext.EndGroups {
		t.Fatalf("setFlood() error = %v, want EndGroups", err)
	}
	if got := antiflood.GetFlood(chat.Id).Limit; got != 4 {
		t.Fatalf("flood limit = %d, want 4", got)
	}

	modeCtx := newModuleMessageContext(bot, chat, admin, "/setfloodmode ban")
	if err := antifloodModule.setFloodMode(bot, modeCtx); err != ext.EndGroups {
		t.Fatalf("setFloodMode() error = %v, want EndGroups", err)
	}
	if got := antiflood.GetFlood(chat.Id).Action; got != "ban" {
		t.Fatalf("flood action = %q, want ban", got)
	}

	deleteCtx := newModuleMessageContext(bot, chat, admin, "/delflood on")
	if err := antifloodModule.setFloodDeleter(bot, deleteCtx); err != ext.EndGroups {
		t.Fatalf("setFloodDeleter() error = %v, want EndGroups", err)
	}
	if got := antiflood.GetFlood(chat.Id).DeleteAntifloodMessage; !got {
		t.Fatal("DeleteAntifloodMessage = false, want true")
	}

	showCtx := newModuleMessageContext(bot, chat, admin, "/flood")
	if err := antifloodModule.flood(bot, showCtx); err != ext.EndGroups {
		t.Fatalf("flood() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 4 {
		t.Fatalf("sendMessage calls = %d, want one reply per command", len(calls))
	}
}

func TestSetFloodModeTimedDurationControlsEnforcementAndDisplay(t *testing.T) {
	useEnglishLocale(t)
	for _, tc := range []struct {
		mode     string
		duration string
		seconds  int
		method   string
		label    string
	}{
		{"tmute", "10m", 600, "restrictChatMember", "muted for 10m"},
		{"tban", "1d", 86400, "banChatMember", "banned for 1d"},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			resetAntifloodState(t)
			f := newAdminChatFixture(t)
			if err := antiflood.SetFlood(f.chat.Id, 1); err != nil {
				t.Fatalf("SetFlood() error = %v", err)
			}
			command := "/setfloodmode " + tc.mode + " " + tc.duration
			if err := antifloodModule.setFloodMode(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, command)); err != ext.EndGroups {
				t.Fatalf("%s error = %v, want EndGroups", command, err)
			}
			if s := antiflood.GetFlood(f.chat.Id); s.Action != tc.mode || s.FloodActionTime != tc.seconds {
				t.Fatalf("flood settings = %+v, want %s %d", s, tc.mode, tc.seconds)
			}
			if got, want := lastReply(t, f.client), fmt.Sprintf(translated(t, "antiflood_mode_"+tc.mode), tc.duration); got != want {
				t.Fatalf("mode reply = %q, want %q", got, want)
			}
			if err := antifloodModule.flood(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, "/flood")); err != ext.EndGroups {
				t.Fatalf("flood() error = %v, want EndGroups", err)
			}
			if text := lastReply(t, f.client); !strings.Contains(text, tc.label) {
				t.Fatalf("/flood reply = %q, want %q", text, tc.label)
			}

			before := time.Now().Unix()
			for i, text := range []string{"one", "two"} {
				ctx := newModuleMessageContext(f.bot, f.chat, f.member, text)
				ctx.EffectiveMessage.MessageId = int64(500 + i)
				if err := antifloodModule.checkFlood(f.bot, ctx); err != ext.ContinueGroups {
					t.Fatalf("checkFlood(%s) error = %v, want ContinueGroups", text, err)
				}
			}
			after := time.Now().Unix()
			call := f.client.onlyCall(t, tc.method)
			assertParams(t, call, targetParams(f.chat.Id, f.member.Id))
			if until := untilDateParam(t, call); until < before+int64(tc.seconds) || until > after+int64(tc.seconds) {
				t.Fatalf("until_date = %d, want now+%d (%d..%d)", until, tc.seconds, before+int64(tc.seconds), after+int64(tc.seconds))
			}
			if text := lastReply(t, f.client); !strings.Contains(text, tc.label) {
				t.Fatalf("flood notice = %q, want %q", text, tc.label)
			}

			if err := antifloodModule.setFloodMode(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, "/setfloodmode ban")); err != ext.EndGroups {
				t.Fatalf("setFloodMode(ban) error = %v, want EndGroups", err)
			}
			if s := antiflood.GetFlood(f.chat.Id); s.Action != "ban" || s.FloodActionTime != 0 {
				t.Fatalf("flood settings = %+v, want ban with cleared duration", s)
			}
		})
	}
}

func TestSetFloodModeRejectsMissingInvalidAndOutOfRangeDurationsWithoutWrites(t *testing.T) {
	useEnglishLocale(t)
	for _, existing := range []bool{false, true} {
		for _, mode := range []string{"tban", "tmute"} {
			for _, tc := range []struct {
				duration string
				replyKey string
			}{
				{"", "extraction_no_time_specified"},
				{"0h", "extraction_invalid_time_amount"},
				{"-1m", "extraction_invalid_time_amount"},
				{"367d", "extraction_time_limit_exceeded"},
				{"1d extra", "extraction_invalid_time_format"},
			} {
				t.Run(fmt.Sprintf("existing=%t/%s/%s", existing, mode, tc.duration), func(t *testing.T) {
					f := newAdminChatFixture(t)
					if existing {
						if err := antiflood.SetFloodMode(f.chat.Id, "tmute", 600); err != nil {
							t.Fatalf("SetFloodMode() error = %v", err)
						}
					}
					before := *antiflood.GetFlood(f.chat.Id)
					rows := db.CountRows(&models.AntifloodSettings{}, "chat_id = ?", f.chat.Id)
					text := strings.TrimSpace("/setfloodmode " + mode + " " + tc.duration)
					if err := antifloodModule.setFloodMode(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, text)); err != ext.EndGroups {
						t.Fatalf("%s error = %v, want EndGroups", text, err)
					}
					if after := antiflood.GetFlood(f.chat.Id); *after != before {
						t.Fatalf("rejected command changed settings: before=%+v after=%+v", before, after)
					}
					if after := db.CountRows(&models.AntifloodSettings{}, "chat_id = ?", f.chat.Id); after != rows {
						t.Fatalf("rejected command changed row count from %d to %d", rows, after)
					}
					if got, want := lastReply(t, f.client), translated(t, tc.replyKey); got != want {
						t.Fatalf("duration refusal = %q, want %q", got, want)
					}
				})
			}
		}
	}
}

func TestAntifloodCommandsHandleDisabledAndValidationBranches(t *testing.T) {
	resetAntifloodState(t)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Flood Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	tests := []struct {
		name string
		text string
		run  func(*gotgbot.Bot, *ext.Context) error
	}{
		{name: "show disabled flood", text: "/flood", run: antifloodModule.flood},
		{name: "set missing limit", text: "/setflood", run: antifloodModule.setFlood},
		{name: "set non integer", text: "/setflood nope", run: antifloodModule.setFlood},
		{name: "set below range", text: "/setflood 2", run: antifloodModule.setFlood},
		{name: "disable flood", text: "/setflood off", run: antifloodModule.setFlood},
		{name: "mode missing", text: "/setfloodmode", run: antifloodModule.setFloodMode},
		{name: "mode invalid", text: "/setfloodmode warn", run: antifloodModule.setFloodMode},
		{name: "deleter current disabled", text: "/delflood", run: antifloodModule.setFloodDeleter},
		{name: "deleter invalid", text: "/delflood maybe", run: antifloodModule.setFloodDeleter},
		{name: "deleter off", text: "/delflood off", run: antifloodModule.setFloodDeleter},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newModuleMessageContext(bot, chat, admin, tt.text)
			if err := tt.run(bot, ctx); err != ext.EndGroups {
				t.Fatalf("%s error = %v, want EndGroups", tt.text, err)
			}
		})
	}
	settings := antiflood.GetFlood(chat.Id)
	if settings.Limit != 0 {
		t.Fatalf("flood limit = %d, want disabled", settings.Limit)
	}
	if settings.DeleteAntifloodMessage {
		t.Fatal("DeleteAntifloodMessage = true, want false after /delflood off")
	}
	if calls := client.callsFor("sendMessage"); len(calls) != len(tests) {
		t.Fatalf("sendMessage calls = %d, want one reply per validation branch", len(calls))
	}
}

func TestAntifloodUpdateFloodTracksLimitAndResetsAfterPunishment(t *testing.T) {
	resetAntifloodState(t)
	chatID := uniqueModuleChatID()
	if err := antiflood.SetFlood(chatID, 2); err != nil {
		t.Fatalf("SetFlood() error = %v", err)
	}

	flooded, state, settings := antifloodModule.updateFlood(chatID, 42, 100)
	if flooded {
		t.Fatal("first message flooded = true, want false")
	}
	if state.messageCount != 1 || len(state.messageIDs) != 1 {
		t.Fatalf("first state = %#v, want one tracked message", state)
	}
	if settings.Limit != 2 {
		t.Fatalf("settings limit = %d, want 2", settings.Limit)
	}

	flooded, state, _ = antifloodModule.updateFlood(chatID, 42, 101)
	if flooded {
		t.Fatal("second message flooded = true, want false at limit")
	}
	if state.messageCount != 2 {
		t.Fatalf("second message count = %d, want 2", state.messageCount)
	}

	flooded, _, _ = antifloodModule.updateFlood(chatID, 42, 102)
	if !flooded {
		t.Fatal("third message flooded = false, want true over limit")
	}
	stored, ok := antifloodModule.syncHelperMap.Load(floodKey{chatId: chatID, userId: 42})
	if !ok {
		t.Fatal("flood state was not stored after punishment")
	}
	got, ok := stored.(*floodControl)
	if !ok || got == nil {
		t.Fatalf("stored state type = %T, want *floodControl", stored)
	}
	if got.messageCount != 0 || got.userId != 0 {
		t.Fatalf("stored state after punishment = %#v, want reset state", got)
	}
}

func TestAntifloodWatcherMutesAndDeletesAfterLimit(t *testing.T) {
	resetAntifloodState(t)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Flood Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	if err := antiflood.SetFlood(chat.Id, 1); err != nil {
		t.Fatalf("SetFlood() error = %v", err)
	}
	if err := antiflood.SetFloodMode(chat.Id, "mute", 0); err != nil {
		t.Fatalf("SetFloodMode() error = %v", err)
	}

	firstCtx := newModuleMessageContext(bot, chat, member, "one")
	if err := antifloodModule.checkFlood(bot, firstCtx); err != ext.ContinueGroups {
		t.Fatalf("checkFlood first error = %v, want ContinueGroups", err)
	}
	if calls := client.callsFor("restrictChatMember"); len(calls) != 0 {
		t.Fatalf("restrictChatMember calls = %d, want none before limit", len(calls))
	}

	secondCtx := newModuleMessageContext(bot, chat, member, "two")
	secondCtx.EffectiveMessage.MessageId = 102
	if err := antifloodModule.checkFlood(bot, secondCtx); err != ext.ContinueGroups {
		t.Fatalf("checkFlood second error = %v, want ContinueGroups", err)
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != 1 {
		t.Fatalf("deleteMessage calls = %d, want triggering message deleted", len(calls))
	}
	if calls := client.callsFor("restrictChatMember"); len(calls) != 1 {
		t.Fatalf("restrictChatMember calls = %d, want mute action", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want antiflood notice", len(calls))
	}
}

func TestAntifloodWatcherIgnoresLinkedChannelPosts(t *testing.T) {
	for _, action := range []string{"mute", "kick", "ban"} {
		t.Run(action, func(t *testing.T) {
			resetAntifloodState(t)
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Flood Chat"}
			if err := antiflood.SetFlood(chat.Id, 1); err != nil {
				t.Fatalf("SetFlood() error = %v", err)
			}
			if err := antiflood.SetFloodMode(chat.Id, action, 0); err != nil {
				t.Fatalf("SetFloodMode() error = %v", err)
			}

			channel := gotgbot.Chat{Id: -1003267074951, Type: "channel", Title: "Linked Channel"}
			for i := range 3 {
				msg := &gotgbot.Message{
					MessageId:          int64(700 + i),
					Date:               1,
					Chat:               chat,
					From:               &gotgbot.User{Id: 777000, FirstName: "Telegram"},
					SenderChat:         &channel,
					IsAutomaticForward: true,
					Text:               "channel post",
				}
				ctx := ext.NewContext(bot, &gotgbot.Update{UpdateId: int64(700 + i), Message: msg}, nil)
				if err := antifloodModule.checkFlood(bot, ctx); err != ext.ContinueGroups {
					t.Fatalf("checkFlood(linked channel) error = %v, want ContinueGroups", err)
				}
			}
			for _, method := range []string{"restrictChatMember", "banChatMember", "banChatSenderChat", "deleteMessage"} {
				if calls := client.callsFor(method); len(calls) != 0 {
					t.Fatalf("%s calls = %d, want linked channel posts left alone", method, len(calls))
				}
			}
		})
	}
}

func TestAntifloodWatcherAppliesKickAndBanActions(t *testing.T) {
	for _, tt := range []struct {
		name    string
		action  string
		method  string
		userID  int64
		message string
	}{
		{name: "kick", action: "kick", method: "unbanChatMember", userID: 43, message: "kick me"},
		{name: "ban", action: "ban", method: "banChatMember", userID: 44, message: "ban me"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resetAntifloodState(t)
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Flood Chat"}
			member := gotgbot.User{Id: tt.userID, FirstName: "Flooder"}
			if err := antiflood.SetFlood(chat.Id, 1); err != nil {
				t.Fatalf("SetFlood() error = %v", err)
			}
			if err := antiflood.SetFloodMode(chat.Id, tt.action, 0); err != nil {
				t.Fatalf("SetFloodMode() error = %v", err)
			}

			firstCtx := newModuleMessageContext(bot, chat, member, "one")
			if err := antifloodModule.checkFlood(bot, firstCtx); err != ext.ContinueGroups {
				t.Fatalf("checkFlood first error = %v, want ContinueGroups", err)
			}
			secondCtx := newModuleMessageContext(bot, chat, member, tt.message)
			secondCtx.EffectiveMessage.MessageId = 202
			if err := antifloodModule.checkFlood(bot, secondCtx); err != ext.ContinueGroups {
				t.Fatalf("checkFlood second error = %v, want ContinueGroups", err)
			}

			if calls := client.callsFor(tt.method); len(calls) != 1 {
				t.Fatalf("%s calls = %d, want action request", tt.method, len(calls))
			}
			if calls := client.callsFor("sendMessage"); len(calls) != 1 {
				t.Fatalf("sendMessage calls = %d, want antiflood notice", len(calls))
			}
		})
	}
}

func TestAntifloodWatcherBulkDeletesTrackedMessages(t *testing.T) {
	resetAntifloodState(t)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Flood Chat"}
	member := gotgbot.User{Id: 45, FirstName: "BulkFlooder"}
	if err := antiflood.SetFlood(chat.Id, 4); err != nil {
		t.Fatalf("SetFlood() error = %v", err)
	}
	if err := antiflood.SetFloodMode(chat.Id, "ban", 0); err != nil {
		t.Fatalf("SetFloodMode() error = %v", err)
	}
	if err := antiflood.SetFloodMsgDel(chat.Id, true); err != nil {
		t.Fatalf("SetFloodMsgDel() error = %v", err)
	}

	for i := 0; i < 5; i++ {
		ctx := newModuleMessageContext(bot, chat, member, "bulk")
		ctx.EffectiveMessage.MessageId = int64(300 + i)
		if err := antifloodModule.checkFlood(bot, ctx); err != ext.ContinueGroups {
			t.Fatalf("checkFlood message %d error = %v, want ContinueGroups", i, err)
		}
	}

	if calls := client.callsFor("deleteMessage"); len(calls) != 5 {
		t.Fatalf("deleteMessage calls = %d, want all tracked flood messages deleted", len(calls))
	}
	if calls := client.callsFor("banChatMember"); len(calls) != 1 {
		t.Fatalf("banChatMember calls = %d, want ban action", len(calls))
	}
}

func TestAntifloodWatcherFailsOpenWhenAdminSemaphoreFull(t *testing.T) {
	resetAntifloodState(t)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Flood Chat"}
	member := gotgbot.User{Id: 46, FirstName: "Flooder"}
	if err := antiflood.SetFlood(chat.Id, 1); err != nil {
		t.Fatalf("SetFlood() error = %v", err)
	}

	for i := 0; i < maxConcurrentAdminChecks; i++ {
		antifloodModule.adminCheckSemaphore <- struct{}{}
	}
	t.Cleanup(func() {
		for i := 0; i < maxConcurrentAdminChecks; i++ {
			<-antifloodModule.adminCheckSemaphore
		}
	})

	// Two messages cross the limit of one, so the admin check runs.
	for i := 0; i < 2; i++ {
		ctx := newModuleMessageContext(bot, chat, member, "flood")
		ctx.EffectiveMessage.MessageId = int64(300 + i)
		if err := antifloodModule.checkFlood(bot, ctx); err != ext.ContinueGroups {
			t.Fatalf("checkFlood(semaphore full) message %d error = %v, want ContinueGroups", i, err)
		}
	}
	if calls := client.callsFor("banChatMember"); len(calls) != 0 {
		t.Fatalf("banChatMember calls = %d, want fail-open without action", len(calls))
	}
	if calls := client.callsFor("restrictChatMember"); len(calls) != 0 {
		t.Fatalf("restrictChatMember calls = %d, want fail-open without action", len(calls))
	}
}

func TestAntifloodWatcherSkipsAdminLookupBelowLimit(t *testing.T) {
	resetAntifloodState(t)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Flood Chat"}
	member := gotgbot.User{Id: 46, FirstName: "Talker"}
	if err := antiflood.SetFlood(chat.Id, 3); err != nil {
		t.Fatalf("SetFlood() error = %v", err)
	}

	for i := 0; i < 3; i++ {
		ctx := newModuleMessageContext(bot, chat, member, "hello")
		ctx.EffectiveMessage.MessageId = int64(400 + i)
		if err := antifloodModule.checkFlood(bot, ctx); err != ext.ContinueGroups {
			t.Fatalf("checkFlood message %d error = %v, want ContinueGroups", i, err)
		}
	}
	for _, method := range []string{"getChatAdministrators", "getChatMember"} {
		if calls := client.callsFor(method); len(calls) != 0 {
			t.Fatalf("%s calls = %d, want none while under the flood limit", method, len(calls))
		}
	}
}

func TestAntifloodWatcherTrustsCachedNonAdminWithoutSemaphore(t *testing.T) {
	resetAntifloodState(t)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Flood Chat"}
	member := gotgbot.User{Id: 46, FirstName: "Flooder"}
	seedCallbackAdmins(t, chat.Id, gotgbot.MergedChatMember{
		Status:             "administrator",
		User:               gotgbot.User{Id: 999, IsBot: true, FirstName: "Alita"},
		CanRestrictMembers: true,
	})
	if err := antiflood.SetFlood(chat.Id, 1); err != nil {
		t.Fatalf("SetFlood() error = %v", err)
	}
	if err := antiflood.SetFloodMode(chat.Id, "ban", 0); err != nil {
		t.Fatalf("SetFloodMode() error = %v", err)
	}

	for i := 0; i < maxConcurrentAdminChecks; i++ {
		antifloodModule.adminCheckSemaphore <- struct{}{}
	}
	t.Cleanup(func() {
		for i := 0; i < maxConcurrentAdminChecks; i++ {
			<-antifloodModule.adminCheckSemaphore
		}
	})

	firstCtx := newModuleMessageContext(bot, chat, member, "one")
	if err := antifloodModule.checkFlood(bot, firstCtx); err != ext.ContinueGroups {
		t.Fatalf("checkFlood first error = %v, want ContinueGroups", err)
	}
	secondCtx := newModuleMessageContext(bot, chat, member, "two")
	secondCtx.EffectiveMessage.MessageId = 202
	if err := antifloodModule.checkFlood(bot, secondCtx); err != ext.ContinueGroups {
		t.Fatalf("checkFlood second error = %v, want ContinueGroups", err)
	}
	if calls := client.callsFor("banChatMember"); len(calls) != 1 {
		t.Fatalf("banChatMember calls = %d, want punishment despite full semaphore", len(calls))
	}
}

func TestAntifloodWatcherSkipsUntargetableMessages(t *testing.T) {
	resetAntifloodState(t)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Flood Chat"}

	nilSenderCtx := newModuleMessageContext(bot, chat, gotgbot.User{Id: 42, FirstName: "Member"}, "system")
	nilSenderCtx.EffectiveMessage.From = nil
	if err := antifloodModule.checkFlood(bot, nilSenderCtx); err != ext.ContinueGroups {
		t.Fatalf("checkFlood(nil sender) error = %v, want ContinueGroups", err)
	}

	mediaGroupCtx := newModuleMessageContext(bot, chat, gotgbot.User{Id: 42, FirstName: "Member"}, "album")
	mediaGroupCtx.EffectiveMessage.MediaGroupId = "album-1"
	if err := antifloodModule.checkFlood(bot, mediaGroupCtx); err != ext.ContinueGroups {
		t.Fatalf("checkFlood(media group) error = %v, want ContinueGroups", err)
	}

	if calls := client.callsFor("deleteMessage"); len(calls) != 0 {
		t.Fatalf("deleteMessage calls = %d, want no action for skipped messages", len(calls))
	}
}

func TestAntifloodWatcherSkipsApprovedUsersAndBotRestrictFailures(t *testing.T) {
	for _, tt := range []struct {
		name   string
		userID int64
		setup  func(t *testing.T, chatID int64, client *moduleBotClient)
	}{
		{
			name:   "approved user",
			userID: 47,
			setup: func(t *testing.T, chatID int64, _ *moduleBotClient) {
				t.Helper()
				if err := approvals.AddApprovedUser(chatID, 47, 777000, "trusted"); err != nil {
					t.Fatalf("AddApprovedUser() error = %v", err)
				}
			},
		},
		{
			name:   "bot cannot restrict",
			userID: 48,
			setup: func(_ *testing.T, _ int64, client *moduleBotClient) {
				client.errors["getChatMember"] = fmt.Errorf("telegram admin lookup failed")
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resetAntifloodState(t)
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Flood Chat"}
			member := gotgbot.User{Id: tt.userID, FirstName: "Flooder"}
			if err := antiflood.SetFlood(chat.Id, 1); err != nil {
				t.Fatalf("SetFlood() error = %v", err)
			}
			if err := antiflood.SetFloodMode(chat.Id, "mute", 0); err != nil {
				t.Fatalf("SetFloodMode() error = %v", err)
			}
			tt.setup(t, chat.Id, client)

			for i := 0; i < 2; i++ {
				ctx := newModuleMessageContext(bot, chat, member, "flood")
				ctx.EffectiveMessage.MessageId = int64(500 + i)
				if err := antifloodModule.checkFlood(bot, ctx); err != ext.ContinueGroups {
					t.Fatalf("checkFlood message %d error = %v, want ContinueGroups", i, err)
				}
			}

			if calls := client.callsFor("restrictChatMember"); len(calls) != 0 {
				t.Fatalf("restrictChatMember calls = %d, want skipped mute action", len(calls))
			}
		})
	}
}

func TestAntifloodCleanupRemovesStaleEntries(t *testing.T) {
	resetAntifloodState(t)

	staleKey := floodKey{chatId: -9001, userId: 1001}
	freshKey := floodKey{chatId: -9001, userId: 1002}

	now := int64(1_000_000)
	staleActivity := now - 601
	freshActivity := now - 10

	antifloodModule.syncHelperMap.Store(staleKey, &floodControl{userId: 1001, lastActivity: staleActivity})
	antifloodModule.syncHelperMap.Store(freshKey, &floodControl{userId: 1002, lastActivity: freshActivity})

	antifloodModule.cleanupOnce(now)

	if _, ok := antifloodModule.syncHelperMap.Load(staleKey); ok {
		t.Error("stale key still present in syncHelperMap after cleanup")
	}

	if _, ok := antifloodModule.syncHelperMap.Load(freshKey); !ok {
		t.Error("fresh key was incorrectly removed from syncHelperMap")
	}

	antifloodModule.syncHelperMap.Delete(freshKey)
}

func TestAntifloodWatcherPunishesAfterFloodMessageDeleteErrors(t *testing.T) {
	resetAntifloodState(t)
	client := newModuleBotClient()
	client.errors["deleteMessage"] = fmt.Errorf("delete failed")
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Flood Chat"}
	member := gotgbot.User{Id: 49, FirstName: "Flooder"}
	if err := antiflood.SetFlood(chat.Id, 1); err != nil {
		t.Fatalf("SetFlood() error = %v", err)
	}
	if err := antiflood.SetFloodMode(chat.Id, "ban", 0); err != nil {
		t.Fatalf("SetFloodMode() error = %v", err)
	}
	if err := antiflood.SetFloodMsgDel(chat.Id, true); err != nil {
		t.Fatalf("SetFloodMsgDel() error = %v", err)
	}

	firstCtx := newModuleMessageContext(bot, chat, member, "one")
	if err := antifloodModule.checkFlood(bot, firstCtx); err != ext.ContinueGroups {
		t.Fatalf("checkFlood first error = %v, want ContinueGroups", err)
	}
	secondCtx := newModuleMessageContext(bot, chat, member, "two")
	secondCtx.EffectiveMessage.MessageId = 602
	if err := antifloodModule.checkFlood(bot, secondCtx); err != ext.ContinueGroups {
		t.Fatalf("checkFlood delete failure error = %v, want ContinueGroups", err)
	}
	if calls := client.callsFor("banChatMember"); len(calls) != 1 {
		t.Fatalf("banChatMember calls = %d, want punishment despite delete failure", len(calls))
	}
}

// Idle counters stay in syncHelperMap until the 10-minute cleaner. A message
// after the 60s burst window must replace that pointer and return. Treating
// the idle entry as absent makes LoadOrStore fail and the handler spin.
func TestUpdateFloodWithSettingsReplacesIdleCounter(t *testing.T) {
	mod := antifloodStruct{}
	key := floodKey{chatId: -4242, userId: 7}
	mod.syncHelperMap.Store(key, &floodControl{
		userId:       key.userId,
		messageCount: 4,
		messageIDs:   []int64{1, 2, 3, 4},
		lastActivity: time.Now().Unix() - 61,
	})
	settings := &db.AntifloodSettings{ChatId: key.chatId, Limit: 5}

	type result struct {
		punish bool
		state  floodControl
	}
	done := make(chan result, 1)
	go func() {
		punish, state := mod.updateFloodWithSettings(key.chatId, key.userId, 99, settings)
		done <- result{punish: punish, state: state}
	}()

	select {
	case got := <-done:
		if got.punish {
			t.Fatal("idle reset punished, want a fresh window")
		}
		if got.state.messageCount != 1 || len(got.state.messageIDs) != 1 || got.state.messageIDs[0] != 99 {
			t.Fatalf("idle reset state = %#v, want count 1 tracking message 99", got.state)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("updateFloodWithSettings hung: idle counter is still present, so LoadOrStore never commits")
	}

	punish, state := mod.updateFloodWithSettings(key.chatId, key.userId, 100, settings)
	if punish || state.messageCount != 2 || len(state.messageIDs) != 2 {
		t.Fatalf("follow-up = punish:%v state:%#v, want punish false count 2", punish, state)
	}
}

// The unknown-type reply is HTML, so the echoed argument must be escaped or
// Telegram rejects the reply and the admin gets no answer.
func TestSetFloodModeEscapesUnknownType(t *testing.T) {
	resetAntifloodState(t)
	f := newAdminChatFixture(t)
	if err := antifloodModule.setFloodMode(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, "/setfloodmode <x>")); err != ext.EndGroups {
		t.Fatalf("setFloodMode() error = %v, want EndGroups", err)
	}
	text := fmt.Sprint(f.client.onlyCall(t, "sendMessage").Params["text"])
	if strings.Contains(text, "<x>") || !strings.Contains(text, "&lt;x&gt;") {
		t.Fatalf("unknown-type reply does not escape the argument:\n%s", text)
	}
}

// Legacy zero and unusable stored durations retain the one-hour fallback.
func TestAntifloodWatcherAppliesTimedActionsForAnHour(t *testing.T) {
	useEnglishLocale(t)
	for _, tt := range []struct {
		action     string
		method     string
		wantLabel  string
		actionTime int
	}{
		{action: "tban", method: "banChatMember", wantLabel: "banned for 1h"},
		{action: "tmute", method: "restrictChatMember", wantLabel: "muted for 1h"},
		{action: "tban", method: "banChatMember", wantLabel: "banned for 1h", actionTime: 30},
		{action: "tmute", method: "restrictChatMember", wantLabel: "muted for 1h", actionTime: 366*86400 + 1},
	} {
		t.Run(fmt.Sprintf("%s/%d", tt.action, tt.actionTime), func(t *testing.T) {
			resetAntifloodState(t)
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Flood Chat"}
			member := gotgbot.User{Id: 45, FirstName: "Flooder"}
			if err := antiflood.SetFlood(chat.Id, 1); err != nil {
				t.Fatalf("SetFlood() error = %v", err)
			}
			if err := antiflood.SetFloodMode(chat.Id, tt.action, tt.actionTime); err != nil {
				t.Fatalf("SetFloodMode() error = %v", err)
			}

			firstCtx := newModuleMessageContext(bot, chat, member, "one")
			if err := antifloodModule.checkFlood(bot, firstCtx); err != ext.ContinueGroups {
				t.Fatalf("checkFlood first error = %v, want ContinueGroups", err)
			}
			secondCtx := newModuleMessageContext(bot, chat, member, "two")
			secondCtx.EffectiveMessage.MessageId = 302
			before := time.Now().Unix()
			if err := antifloodModule.checkFlood(bot, secondCtx); err != ext.ContinueGroups {
				t.Fatalf("checkFlood second error = %v, want ContinueGroups", err)
			}
			after := time.Now().Unix()

			calls := client.callsFor(tt.method)
			if len(calls) != 1 {
				t.Fatalf("%s calls = %d, want timed action", tt.method, len(calls))
			}
			if until := untilDateParam(t, calls[0]); until < before+3600 || until > after+3600 {
				t.Fatalf("until_date = %d, want now+3600 (%d..%d)", until, before+3600, after+3600)
			}
			notices := client.callsFor("sendMessage")
			if len(notices) != 1 || !strings.Contains(fmt.Sprint(notices[0].Params["text"]), tt.wantLabel) {
				t.Fatalf("antiflood notices = %#v, want %q", notices, tt.wantLabel)
			}

			showCtx := newModuleMessageContext(bot, chat, member, "/flood")
			if err := antifloodModule.flood(bot, showCtx); err != ext.EndGroups {
				t.Fatalf("flood() error = %v, want EndGroups", err)
			}
			replies := client.callsFor("sendMessage")
			if len(replies) != 2 || !strings.Contains(fmt.Sprint(replies[1].Params["text"]), tt.wantLabel) {
				t.Fatalf("/flood replies = %#v, want %q", replies, tt.wantLabel)
			}
		})
	}
}

func TestAntifloodWatcherDoesNotPunishChannelForTimedAction(t *testing.T) {
	for _, mode := range []string{"tban", "tmute"} {
		t.Run(mode, func(t *testing.T) {
			resetAntifloodState(t)
			f := newAdminChatFixture(t)
			if err := antiflood.SetFlood(f.chat.Id, 1); err != nil {
				t.Fatalf("SetFlood() error = %v", err)
			}
			if err := antiflood.SetFloodMode(f.chat.Id, mode, 600); err != nil {
				t.Fatalf("SetFloodMode() error = %v", err)
			}
			channel := gotgbot.Chat{Id: -1001234567892, Type: "channel", Title: "Flood Channel"}
			for i := range 2 {
				ctx := newModuleMessageContext(f.bot, f.chat, f.member, "channel flood")
				ctx.EffectiveMessage.MessageId = int64(600 + i)
				ctx.EffectiveMessage.From = nil
				ctx.EffectiveMessage.SenderChat = &channel
				ctx.EffectiveSender = &gotgbot.Sender{Chat: &channel, ChatId: f.chat.Id}
				if err := antifloodModule.checkFlood(f.bot, ctx); err != ext.ContinueGroups {
					t.Fatalf("checkFlood() error = %v, want ContinueGroups", err)
				}
			}
			assertDeleted(t, f.client, f.chat.Id, 601)
			for _, method := range []string{"banChatSenderChat", "banChatMember", "restrictChatMember", "sendMessage"} {
				if calls := f.client.callsFor(method); len(calls) != 0 {
					t.Fatalf("%s calls = %d, want none for a channel", method, len(calls))
				}
			}
		})
	}
}

// warn passes the action CHECK but has no antiflood handler: the bot must not
// announce a punishment it did not apply.
func TestAntifloodWatcherSkipsNoticeForUnhandledAction(t *testing.T) {
	resetAntifloodState(t)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Flood Chat"}
	member := gotgbot.User{Id: 46, FirstName: "Flooder"}
	if err := antiflood.SetFlood(chat.Id, 1); err != nil {
		t.Fatalf("SetFlood() error = %v", err)
	}
	if err := antiflood.SetFloodMode(chat.Id, "warn", 0); err != nil {
		t.Fatalf("SetFloodMode() error = %v", err)
	}

	for i, text := range []string{"one", "two"} {
		ctx := newModuleMessageContext(bot, chat, member, text)
		ctx.EffectiveMessage.MessageId = int64(400 + i)
		if err := antifloodModule.checkFlood(bot, ctx); err != ext.ContinueGroups {
			t.Fatalf("checkFlood(%s) error = %v, want ContinueGroups", text, err)
		}
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 0 {
		t.Fatalf("sendMessage calls = %#v, want no punishment notice", calls)
	}

	showCtx := newModuleMessageContext(bot, chat, member, "/flood")
	if err := antifloodModule.flood(bot, showCtx); err != ext.EndGroups {
		t.Fatalf("flood() error = %v, want EndGroups", err)
	}
	replies := client.callsFor("sendMessage")
	if len(replies) != 1 || !strings.Contains(fmt.Sprint(replies[0].Params["text"]), "warn") {
		t.Fatalf("/flood replies = %#v, want the stored action named", replies)
	}
}

// The flood notice and /flood interpolate the action into a translated
// template, so the action word must come from the same locale.
func TestFloodActionLabelFollowsTheChatLanguage(t *testing.T) {
	useRealLocales(t)

	en := i18n.MustNewTranslator("en")
	for _, langCode := range []string{"es", "fr", "hi", "id", "pt", "ru"} {
		tr := i18n.MustNewTranslator(langCode)
		for _, action := range []string{"mute", "ban", "kick", "tban", "tmute", ""} {
			settings := &models.AntifloodSettings{Action: action}
			got := floodActionLabel(tr, settings)
			if got == "" {
				t.Fatalf("floodActionLabel(%s, %q) is empty", langCode, action)
			}
			if got == floodActionLabel(en, settings) {
				t.Errorf("floodActionLabel(%s, %q) = %q, same as English", langCode, action, got)
			}
			if (action == "tban" || action == "tmute") && !strings.Contains(got, "1h") {
				t.Errorf("floodActionLabel(%s, %q) = %q, want the 1h duration", langCode, action, got)
			}
		}
	}
}
