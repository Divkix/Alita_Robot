package modules

import (
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
	previousMarshal := cache.Marshal
	cache.Marshal = nil
	t.Cleanup(func() {
		cache.Marshal = previousMarshal
	})

	if msg, err := getAnonAdminCache(-100123, 99); err == nil || msg != nil {
		t.Fatalf("getAnonAdminCache() = (%#v, %v), want nil message and error", msg, err)
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

func TestVerifyAnonymousAdminRestoresCachedMessageAndDeletesButton(t *testing.T) {
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
	if err := cache.Marshal.Set(cache.Context, key, cached); err != nil {
		t.Fatalf("cache set: %v", err)
	}
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
