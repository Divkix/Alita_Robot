package modules

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/logchannels"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
)

func TestAdminPermissionUpdatesRefreshAuthorization(t *testing.T) {
	for _, botUpdate := range []bool{false, true} {
		t.Run(fmt.Sprintf("bot=%t", botUpdate), func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup"}
			user := gotgbot.User{Id: 42}
			if botUpdate {
				user = bot.User
			}
			oldMember := gotgbot.ChatMemberAdministrator{User: user, CanRestrictMembers: true}
			newMember := gotgbot.ChatMemberAdministrator{User: user, CanRestrictMembers: false}
			key := fmt.Sprintf("alita:cache:adminCache:%d", chat.Id)
			if err := cache.GetMarshal().Set(cache.Context, key, cache.AdminCache{
				ChatId: chat.Id, Cached: true,
				UserMap: map[int64]gotgbot.MergedChatMember{user.Id: oldMember.MergeChatMember()},
			}); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { cache.InvalidateAdminCache(chat.Id) })
			response, err := json.Marshal([]gotgbot.ChatMemberAdministrator{newMember})
			if err != nil {
				t.Fatal(err)
			}
			client.responses["getChatAdministrators"] = response
			event := &gotgbot.ChatMemberUpdated{Chat: chat, OldChatMember: oldMember, NewChatMember: newMember}
			update := &gotgbot.Update{ChatMember: event}
			if botUpdate {
				update = &gotgbot.Update{MyChatMember: event}
			}
			dispatcher := ext.NewDispatcher(nil)
			LoadBotUpdates(dispatcher)
			if err := dispatcher.ProcessUpdate(bot, update, nil); err != nil {
				t.Fatal(err)
			}
			ctx := newModuleMessageContext(bot, chat, user, "/ban 123")
			if chat_status.CanUserRestrict(bot, ctx, &chat, user.Id) {
				t.Fatal("revoked restriction permission is still authorized")
			}
		})
	}
}

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

func TestVerifyAnonymousAdminRejectsNonAdmin(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Anon Admin Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	data := encodeCallbackData("anon_admin", map[string]string{
		"c": fmt.Sprint(chat.Id),
		"m": "101",
	})
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
	})
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
		t.Fatal("cache marshaler is not initialised; TestMain must set it")
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
		})
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
		})
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
		t.Fatal("cache marshaler is not initialised; TestMain must set it")
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
	})
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
	LoadBotUpdates(moduleDispatcher)
	if removed := moduleDispatcher.RemoveGroup(-1); !removed {
		t.Fatal("LoadBotUpdates did not register join handler in group -1")
	}
	if removed := moduleDispatcher.RemoveGroup(0); !removed {
		t.Fatal("LoadBotUpdates did not register standard handlers in group 0")
	}
}

type anonAdminBanFlow struct {
	client *moduleBotClient
	chat   gotgbot.Chat
	target gotgbot.User
	admin  gotgbot.User
}

func newAnonAdminBanChat() gotgbot.Chat {
	return gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Anon Ban Chat"}
}

// runAnonymousAdminBan sends /ban from an anonymous admin in chat through a real dispatcher, then
// presses the confirmation button the bot sent. promptChat is the chat the button message is
// reported in; pass nil to use chat.
func runAnonymousAdminBan(t *testing.T, chat gotgbot.Chat, promptChat *gotgbot.Chat) anonAdminBanFlow {
	t.Helper()
	if cache.GetMarshal() == nil {
		t.Fatal("cache marshaler is not initialised; TestMain must set it")
	}

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadBans(dispatcher)
	LoadBotUpdates(dispatcher)

	flow := anonAdminBanFlow{
		client: client,
		chat:   chat,
		target: gotgbot.User{Id: 42, FirstName: "Member"},
		admin:  gotgbot.User{Id: 777000, FirstName: "Telegram"},
	}
	anonBot := gotgbot.User{Id: 1087968824, IsBot: true, FirstName: "Group", Username: "GroupAnonymousBot"}
	senderChat := chat
	command := &gotgbot.Message{
		MessageId:  505,
		Date:       1,
		Chat:       chat,
		From:       &anonBot,
		SenderChat: &senderChat,
		Text:       "/ban spam",
		Entities:   []gotgbot.MessageEntity{{Type: "bot_command", Offset: 0, Length: 4}},
		ReplyToMessage: &gotgbot.Message{
			MessageId: 504,
			Date:      1,
			Chat:      chat,
			From:      &flow.target,
			Text:      "message being moderated",
		},
	}
	if err := dispatcher.ProcessUpdate(bot, &gotgbot.Update{UpdateId: 10, Message: command}, nil); err != nil {
		t.Fatalf("ProcessUpdate(command) error = %v", err)
	}
	if calls := client.callsFor("banChatMember"); len(calls) != 0 {
		t.Fatalf("banChatMember calls = %d before confirmation, want 0", len(calls))
	}

	var callbackData string
	for _, call := range client.callsFor("sendMessage") {
		markup, ok := call.Params["reply_markup"].(gotgbot.InlineKeyboardMarkup)
		if !ok || len(markup.InlineKeyboard) == 0 || len(markup.InlineKeyboard[0]) == 0 {
			continue
		}
		callbackData = markup.InlineKeyboard[0][0].CallbackData
	}
	if callbackData == "" {
		t.Fatal("anonymous admin confirmation button was not sent")
	}
	t.Cleanup(func() {
		_ = cache.GetMarshal().Delete(cache.Context, fmt.Sprintf("alita:anonAdmin:%d:%d", chat.Id, command.MessageId))
	})

	if promptChat == nil {
		promptChat = &chat
	}
	prompt := gotgbot.Message{MessageId: 506, Date: 1, Chat: *promptChat, From: &bot.User, Text: "prove you are admin"}
	press := &gotgbot.CallbackQuery{
		Id:           "anon-ban-confirm",
		From:         flow.admin,
		Message:      prompt,
		Data:         callbackData,
		ChatInstance: "test-chat-instance",
	}
	if err := dispatcher.ProcessUpdate(bot, &gotgbot.Update{UpdateId: 11, CallbackQuery: press}, nil); err != nil {
		t.Fatalf("ProcessUpdate(callback) error = %v", err)
	}
	return flow
}

func TestAnonymousAdminBanRunsInGroupAfterButtonConfirmation(t *testing.T) {
	flow := runAnonymousAdminBan(t, newAnonAdminBanChat(), nil)

	bans := flow.client.callsFor("banChatMember")
	if len(bans) != 1 {
		t.Fatalf("banChatMember calls = %d after confirmation, want 1", len(bans))
	}
	if got := fmt.Sprint(bans[0].Params["chat_id"]); got != fmt.Sprint(flow.chat.Id) {
		t.Fatalf("banChatMember chat_id = %s, want %d", got, flow.chat.Id)
	}
	if got := fmt.Sprint(bans[0].Params["user_id"]); got != fmt.Sprint(flow.target.Id) {
		t.Fatalf("banChatMember user_id = %s, want %d", got, flow.target.Id)
	}
}

func TestAnonymousAdminConfirmationAnswersCallback(t *testing.T) {
	flow := runAnonymousAdminBan(t, newAnonAdminBanChat(), nil)

	answers := flow.client.callsFor("answerCallbackQuery")
	if len(answers) != 1 {
		t.Fatalf("answerCallbackQuery calls = %d, want 1 so the button stops spinning", len(answers))
	}
	if got := fmt.Sprint(answers[0].Params["callback_query_id"]); got != "anon-ban-confirm" {
		t.Fatalf("answerCallbackQuery callback_query_id = %s, want anon-ban-confirm", got)
	}
}

func TestAnonymousAdminBanIsLoggedAsConfirmingAdmin(t *testing.T) {
	const logChannelID int64 = -100777
	chat := newAnonAdminBanChat()
	if err := logchannels.Set(chat.Id, chat.Title, logChannelID); err != nil {
		t.Fatalf("logchannels.Set: %v", err)
	}
	t.Cleanup(func() { _ = logchannels.Unset(chat.Id) })
	flow := runAnonymousAdminBan(t, chat, nil)

	var logText string
	for _, call := range flow.client.callsFor("sendMessage") {
		if fmt.Sprint(call.Params["chat_id"]) == fmt.Sprint(logChannelID) {
			logText = fmt.Sprint(call.Params["text"])
		}
	}
	if logText == "" {
		t.Fatal("ban was not written to the log channel")
	}
	if want := fmt.Sprintf("Admin: %s", formatting.MentionHtml(flow.admin.Id, flow.admin.FirstName)); !strings.Contains(logText, want) {
		t.Fatalf("log channel text = %q, want actor %q", logText, want)
	}
}

func TestAnonymousAdminConfirmationFromAnotherChatDoesNotRunCommand(t *testing.T) {
	otherChat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Other Chat"}
	flow := runAnonymousAdminBan(t, newAnonAdminBanChat(), &otherChat)

	if calls := flow.client.callsFor("banChatMember"); len(calls) != 0 {
		t.Fatalf("banChatMember calls = %d for a button reported in another chat, want 0", len(calls))
	}
	if calls := flow.client.callsFor("deleteMessage"); len(calls) != 0 {
		t.Fatalf("deleteMessage calls = %d for a button reported in another chat, want 0", len(calls))
	}
	if calls := flow.client.callsFor("answerCallbackQuery"); len(calls) != 1 {
		t.Fatalf("answerCallbackQuery calls = %d, want the invalid-request answer", len(calls))
	}
}
