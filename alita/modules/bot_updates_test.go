package modules

import (
	"errors"
	"fmt"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

func TestBotJoinedGroupIgnoresPrivateChats(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: 42, Type: "private", FirstName: "Private"}
	user := gotgbot.User{Id: 42, FirstName: "Private"}
	ctx := newModuleMessageContext(bot, chat, user, "bot joined")

	if err := botJoinedGroup(bot, ctx); err != ext.EndGroups {
		t.Fatalf("botJoinedGroup() error = %v, want EndGroups for private chat", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 0 {
		t.Fatalf("sendMessage calls = %d, want none for private chat", len(calls))
	}
	if calls := client.callsFor("leaveChat"); len(calls) != 0 {
		t.Fatalf("leaveChat calls = %d, want none for private chat", len(calls))
	}
}

func TestBotJoinedGroupLeavesBasicGroupAfterMigrationNotice(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "group", Title: "Basic Group"}
	user := gotgbot.User{Id: 42, FirstName: "Member"}
	ctx := newModuleMessageContext(bot, chat, user, "bot joined")

	if err := botJoinedGroup(bot, ctx); err != ext.EndGroups {
		t.Fatalf("botJoinedGroup() error = %v, want EndGroups for basic group", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want migration notice", len(calls))
	}
	if calls := client.callsFor("leaveChat"); len(calls) != 1 {
		t.Fatalf("leaveChat calls = %d, want bot to leave basic group", len(calls))
	}
}

func TestBotJoinedGroupLeavesChannelWithoutMigrationNotice(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "channel", Title: "Broadcast"}
	user := gotgbot.User{Id: 42, FirstName: "Member"}
	ctx := newModuleMessageContext(bot, chat, user, "bot joined")

	if err := botJoinedGroup(bot, ctx); err != ext.EndGroups {
		t.Fatalf("botJoinedGroup() error = %v, want EndGroups for channel", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 0 {
		t.Fatalf("sendMessage calls = %d, want no migration notice for channel", len(calls))
	}
	if calls := client.callsFor("leaveChat"); len(calls) != 1 {
		t.Fatalf("leaveChat calls = %d, want bot to leave channel", len(calls))
	}
}

func TestBotJoinedSupergroupSendsWelcome(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Super Group"}
	user := gotgbot.User{Id: 42, FirstName: "Member"}
	ctx := newModuleMessageContext(bot, chat, user, "bot joined")

	if err := botJoinedGroup(bot, ctx); err != ext.ContinueGroups {
		t.Fatalf("botJoinedGroup() error = %v, want ContinueGroups for supergroup", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want welcome message", len(calls))
	}
	if calls := client.callsFor("leaveChat"); len(calls) != 0 {
		t.Fatalf("leaveChat calls = %d, want none for supergroup", len(calls))
	}
}

func TestBotJoinedGroupPropagatesGotgbotRequestErrors(t *testing.T) {
	requestErr := errors.New("telegram request failed")
	user := gotgbot.User{Id: 42, FirstName: "Member"}

	for _, tt := range []struct {
		name   string
		chat   gotgbot.Chat
		method string
	}{
		{
			name:   "basic group migration notice",
			chat:   gotgbot.Chat{Id: uniqueModuleChatID(), Type: "group", Title: "Basic Group"},
			method: "sendMessage",
		},
		{
			name:   "basic group leave",
			chat:   gotgbot.Chat{Id: uniqueModuleChatID(), Type: "group", Title: "Basic Group"},
			method: "leaveChat",
		},
		{
			name:   "channel leave",
			chat:   gotgbot.Chat{Id: uniqueModuleChatID(), Type: "channel", Title: "Broadcast"},
			method: "leaveChat",
		},
		{
			name:   "supergroup welcome",
			chat:   gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Super Group"},
			method: "sendMessage",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			client.errors[tt.method] = requestErr
			ctx := newModuleMessageContext(bot, tt.chat, user, "bot joined")

			err := botJoinedGroup(bot, ctx)
			if !errors.Is(err, requestErr) {
				t.Fatalf("botJoinedGroup() error = %v, want request error", err)
			}
		})
	}
}

func TestAdminCacheAutoUpdateSkipsMissingEffectiveChat(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	ctx := ext.NewContext(bot, &gotgbot.Update{UpdateId: 99}, nil)

	if err := adminCacheAutoUpdate(bot, ctx); err != ext.ContinueGroups {
		t.Fatalf("adminCacheAutoUpdate() error = %v, want ContinueGroups", err)
	}
	if calls := client.callsFor("getChatAdministrators"); len(calls) != 0 {
		t.Fatalf("getChatAdministrators calls = %d, want none for missing chat", len(calls))
	}
}

func TestAdminCacheAutoUpdateReloadsAdminList(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Admin Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	ctx := newModuleMessageContext(bot, chat, admin, "admin changed")

	if err := adminCacheAutoUpdate(bot, ctx); err != ext.ContinueGroups {
		t.Fatalf("adminCacheAutoUpdate() error = %v, want ContinueGroups", err)
	}
	if calls := client.callsFor("getChatAdministrators"); len(calls) != 1 {
		t.Fatalf("getChatAdministrators calls = %d, want cache reload", len(calls))
	}
}

func TestGetAnonAdminCacheReportsMissingCache(t *testing.T) {
	withNilCacheMarshal(t)

	if msg, err := getAnonAdminCache(-100123, 99); err == nil || msg != nil {
		t.Fatalf("getAnonAdminCache() = (%#v, %v), want nil message and error", msg, err)
	}
}

func TestVerifyAnonymousAdminRejectsLegacyCallbackWithBadChatID(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Anon Admin Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	ctx := newModuleCallbackContext(bot, chat, admin, "alita:anonAdmin:not-a-chat:101")

	if err := verifyAnonymousAdmin(bot, ctx); err != ext.EndGroups {
		t.Fatalf("verifyAnonymousAdmin() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 1 {
		t.Fatalf("answerCallbackQuery calls = %d, want invalid-request answer", len(calls))
	}
	if calls := client.callsFor("getChatMember"); len(calls) != 0 {
		t.Fatalf("getChatMember calls = %d, want none before valid chat ID", len(calls))
	}
}

func TestVerifyAnonymousAdminRejectsDottedCallbackWithBadMessageID(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Anon Admin Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	ctx := newModuleCallbackContext(bot, chat, admin, fmt.Sprintf("legacy.%d.not-a-message", chat.Id))

	if err := verifyAnonymousAdmin(bot, ctx); err != ext.EndGroups {
		t.Fatalf("verifyAnonymousAdmin() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 1 {
		t.Fatalf("answerCallbackQuery calls = %d, want invalid-request answer", len(calls))
	}
	if calls := client.callsFor("getChatMember"); len(calls) != 0 {
		t.Fatalf("getChatMember calls = %d, want none before valid message ID", len(calls))
	}
}

func TestVerifyAnonymousAdminRejectsMalformedCallbackData(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Anon Admin Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	ctx := newModuleCallbackContext(bot, chat, admin, "anon_admin|v1|broken")

	if err := verifyAnonymousAdmin(bot, ctx); err != ext.EndGroups {
		t.Fatalf("verifyAnonymousAdmin() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 1 {
		t.Fatalf("answerCallbackQuery calls = %d, want invalid-request answer", len(calls))
	}
}

func TestVerifyAnonymousAdminSkipsMissingCallbackQuery(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Anon Admin Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	ctx := newModuleMessageContext(bot, chat, admin, "/ban 42")

	if err := verifyAnonymousAdmin(bot, ctx); err != ext.EndGroups {
		t.Fatalf("verifyAnonymousAdmin() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 0 {
		t.Fatalf("answerCallbackQuery calls = %d, want none without callback query", len(calls))
	}
}

func TestVerifyAnonymousAdminRejectsNonAdmin(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Anon Admin Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	data := encodeCallbackData("anon_admin", map[string]string{
		"c": fmt.Sprint(chat.Id),
		"m": "101",
	}, "")
	ctx := newModuleCallbackContext(bot, chat, member, data)

	if err := verifyAnonymousAdmin(bot, ctx); err != ext.EndGroups {
		t.Fatalf("verifyAnonymousAdmin() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("getChatMember"); len(calls) != 1 {
		t.Fatalf("getChatMember calls = %d, want admin check", len(calls))
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 1 {
		t.Fatalf("answerCallbackQuery calls = %d, want non-admin answer", len(calls))
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != 0 {
		t.Fatalf("deleteMessage calls = %d, want none for rejected callback", len(calls))
	}
}

func TestVerifyAnonymousAdminEditsExpiredButton(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Anon Admin Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	data := encodeCallbackData("anon_admin", map[string]string{
		"c": fmt.Sprint(chat.Id),
		"m": "202",
	}, "")
	ctx := newModuleCallbackContext(bot, chat, admin, data)

	if err := verifyAnonymousAdmin(bot, ctx); err != ext.EndGroups {
		t.Fatalf("verifyAnonymousAdmin() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("editMessageText"); len(calls) != 1 {
		t.Fatalf("editMessageText calls = %d, want expired-button edit", len(calls))
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != 0 {
		t.Fatalf("deleteMessage calls = %d, want none when cached command is missing", len(calls))
	}
}

func TestVerifyAnonymousAdminPropagatesEditAndDeleteErrors(t *testing.T) {
	m := cache.GetMarshal()
	if m == nil {
		t.Skip("requires initialized cache marshaler")
	}

	requestErr := errors.New("telegram request failed")
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Anon Admin Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	t.Run("expired button edit failure", func(t *testing.T) {
		client := newModuleBotClient()
		bot := newModuleTestBot(client)
		client.errors["editMessageText"] = requestErr
		data := encodeCallbackData("anon_admin", map[string]string{
			"c": fmt.Sprint(chat.Id),
			"m": "202",
		}, "")
		ctx := newModuleCallbackContext(bot, chat, admin, data)

		err := verifyAnonymousAdmin(bot, ctx)
		if !errors.Is(err, requestErr) {
			t.Fatalf("verifyAnonymousAdmin() error = %v, want edit request error", err)
		}
	})

	t.Run("cached command delete failure", func(t *testing.T) {
		client := newModuleBotClient()
		bot := newModuleTestBot(client)
		client.errors["deleteMessage"] = requestErr
		cached := &gotgbot.Message{
			MessageId: 404,
			Date:      1,
			Chat:      chat,
			Text:      "/unknown",
		}
		key := fmt.Sprintf("alita:anonAdmin:%d:%d", chat.Id, cached.MessageId)
		if err := m.Set(cache.Context, key, cached); err != nil {
			t.Fatalf("cache set: %v", err)
		}
		t.Cleanup(func() {
			_ = m.Delete(cache.Context, key)
		})
		data := encodeCallbackData("anon_admin", map[string]string{
			"c": fmt.Sprint(chat.Id),
			"m": fmt.Sprint(cached.MessageId),
		}, "")
		ctx := newModuleCallbackContext(bot, chat, admin, data)

		err := verifyAnonymousAdmin(bot, ctx)
		if !errors.Is(err, requestErr) {
			t.Fatalf("verifyAnonymousAdmin() error = %v, want delete request error", err)
		}
	})
}

func TestVerifyAnonymousAdminRestoresCachedMessageAndDeletesButton(t *testing.T) {
	m := cache.GetMarshal()
	if m == nil {
		t.Skip("requires initialized cache marshaler")
	}

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Anon Admin Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	cached := &gotgbot.Message{
		MessageId: 303,
		Date:      1,
		Chat:      chat,
		SenderChat: &gotgbot.Chat{
			Id:    chat.Id,
			Type:  "supergroup",
			Title: chat.Title,
		},
		Text: "/unknown",
	}
	key := fmt.Sprintf("alita:anonAdmin:%d:%d", chat.Id, cached.MessageId)
	if err := m.Set(cache.Context, key, cached); err != nil {
		t.Fatalf("cache set: %v", err)
	}
	t.Cleanup(func() {
		_ = m.Delete(cache.Context, key)
	})
	data := encodeCallbackData("anon_admin", map[string]string{
		"c": fmt.Sprint(chat.Id),
		"m": fmt.Sprint(cached.MessageId),
	}, "")
	ctx := newModuleCallbackContext(bot, chat, admin, data)

	if err := verifyAnonymousAdmin(bot, ctx); err != ext.EndGroups {
		t.Fatalf("verifyAnonymousAdmin() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != 1 {
		t.Fatalf("deleteMessage calls = %d, want callback message deleted", len(calls))
	}
	if ctx.CallbackQuery != nil {
		t.Fatal("CallbackQuery was not cleared after restoring cached command")
	}
	if ctx.EffectiveMessage == nil ||
		ctx.EffectiveMessage.MessageId != cached.MessageId ||
		ctx.EffectiveMessage.Text != cached.Text {
		t.Fatalf("EffectiveMessage = %#v, want cached command message", ctx.EffectiveMessage)
	}
	if ctx.EffectiveMessage.SenderChat != nil {
		t.Fatal("SenderChat was not cleared before command replay")
	}
}

func TestBotUpdatesLoadersRegisterExpectedHandlers(t *testing.T) {
	moduleDispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	botUpdatesModule{}.Load(moduleDispatcher)
	if removed := moduleDispatcher.RemoveGroup(-1); !removed {
		t.Fatal("botUpdatesModule.Load did not register join handler in group -1")
	}
	if removed := moduleDispatcher.RemoveGroup(0); !removed {
		t.Fatal("botUpdatesModule.Load did not register standard handlers in group 0")
	}
}
