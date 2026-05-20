package modules

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
)

func TestUnpinWithoutReplyUnpinsLatestMessage(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Pin Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	ctx := newModuleMessageContext(bot, chat, user, "/unpin")
	if err := pinsModule.unpin(bot, ctx); err != ext.EndGroups {
		t.Fatalf("unpin() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("unpinChatMessage"); len(calls) != 1 {
		t.Fatalf("unpinChatMessage calls = %d, want 1", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
}

func TestUnpinReplyWithoutPinnedMessageDoesNotCallTelegramUnpin(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Pin Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	ctx := newModuleMessageContext(bot, chat, user, "/unpin")
	ctx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
		MessageId: 77,
		Date:      1,
		Chat:      chat,
		Text:      "not pinned",
	}
	if err := pinsModule.unpin(bot, ctx); err != ext.EndGroups {
		t.Fatalf("unpin() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("unpinChatMessage"); len(calls) != 0 {
		t.Fatalf("unpinChatMessage calls = %d, want 0 for non-pinned reply", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
}

func TestUnpinAllSendsConfirmationButtons(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Pin Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	ctx := newModuleMessageContext(bot, chat, user, "/unpinall")
	if err := pinsModule.unpinAll(bot, ctx); err != ext.EndGroups {
		t.Fatalf("unpinAll() error = %v, want EndGroups", err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
	if calls[0].Params["reply_markup"] == nil {
		t.Fatal("unpinAll confirmation did not include reply_markup")
	}
}

func TestUnpinAllCallbackExecutesAndCancels(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Pin Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	yesData := encodeCallbackData("unpinallbtn", map[string]string{"a": "yes"}, "unpinallbtn(yes)")
	yesCtx := newModuleCallbackContext(bot, chat, user, yesData)
	if err := pinsModule.unpinallCallback(bot, yesCtx); err != ext.EndGroups {
		t.Fatalf("unpinallCallback yes error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("unpinAllChatMessages"); len(calls) != 1 {
		t.Fatalf("unpinAllChatMessages calls = %d, want 1", len(calls))
	}

	noCtx := newModuleCallbackContext(bot, chat, user, "unpinallbtn(no)")
	if err := pinsModule.unpinallCallback(bot, noCtx); err != ext.EndGroups {
		t.Fatalf("unpinallCallback no error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("editMessageText"); len(calls) != 2 {
		t.Fatalf("editMessageText calls = %d, want 2", len(calls))
	}
}

func TestPinReplyPinsMessageWithNotificationOption(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Pin Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	ctx := newModuleMessageContext(bot, chat, user, "/pin loud")
	ctx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
		MessageId: 88,
		Date:      1,
		Chat:      chat,
		Text:      "pin me",
	}
	if err := pinsModule.pin(bot, ctx); err != ext.EndGroups {
		t.Fatalf("pin() error = %v, want EndGroups", err)
	}
	calls := client.callsFor("pinChatMessage")
	if len(calls) != 1 {
		t.Fatalf("pinChatMessage calls = %d, want 1", len(calls))
	}
	if calls[0].Params["message_id"] != int64(88) {
		t.Fatalf("pinned message_id = %v, want 88", calls[0].Params["message_id"])
	}
}

func TestPinWithoutReplyAsksForReply(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Pin Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	ctx := newModuleMessageContext(bot, chat, user, "/pin")
	if err := pinsModule.pin(bot, ctx); err != ext.EndGroups {
		t.Fatalf("pin() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("pinChatMessage"); len(calls) != 0 {
		t.Fatalf("pinChatMessage calls = %d, want 0 without reply", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
}

func TestPermaPinSendsNewMessageThenPinsIt(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Pin Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	ctx := newModuleMessageContext(bot, chat, user, "/permapin keep this pinned")
	if err := pinsModule.permaPin(bot, ctx); err != ext.EndGroups {
		t.Fatalf("permaPin() error = %v, want EndGroups", err)
	}
	sendCalls := client.callsFor("sendMessage")
	if len(sendCalls) != 2 {
		t.Fatalf("sendMessage calls = %d, want pin message and confirmation", len(sendCalls))
	}
	pinCalls := client.callsFor("pinChatMessage")
	if len(pinCalls) != 1 {
		t.Fatalf("pinChatMessage calls = %d, want 1", len(pinCalls))
	}
	if got := pinCalls[0].Params["message_id"]; got != int64(9001) {
		t.Fatalf("pinned message_id = %v, want sent message 9001", got)
	}
}

func TestPinnedCommandLinksLatestPinnedMessage(t *testing.T) {
	client := newModuleBotClient()
	client.responses["getChat"] = []byte(
		`{"id":-1001,"type":"supergroup","title":"Pin Chat","pinned_message":{"message_id":77,"date":1,"chat":{"id":-1001,"type":"supergroup","title":"Pin Chat"},"text":"pinned"}}`,
	)
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Pin Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	ctx := newModuleMessageContext(bot, chat, user, "/pinned")
	if err := pinsModule.pinned(bot, ctx); err != ext.EndGroups {
		t.Fatalf("pinned() error = %v, want EndGroups", err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want pinned link reply", len(calls))
	}
	if calls[0].Params["reply_markup"] == nil {
		t.Fatal("pinned reply did not include link button markup")
	}
}

func TestPinnedCommandReportsMissingPinnedMessage(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Pin Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	ctx := newModuleMessageContext(bot, chat, user, "/pinned")
	if err := pinsModule.pinned(bot, ctx); err != nil {
		t.Fatalf("pinned() error = %v, want nil for missing pinned message reply", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want missing pinned message reply", len(calls))
	}
}

func TestAntiChannelPinAndCleanLinkedTogglePinPreferences(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chatID := uniqueModuleChatID()
	chat := gotgbot.Chat{Id: chatID, Type: "supergroup", Title: "Pin Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	antiOnCtx := newModuleMessageContext(bot, chat, user, "/antichannelpin on")
	if err := pinsModule.antichannelpin(bot, antiOnCtx); err != ext.EndGroups {
		t.Fatalf("antichannelpin on error = %v, want EndGroups", err)
	}
	if !db.GetPinData(chatID).AntiChannelPin {
		t.Fatal("AntiChannelPin was not enabled")
	}

	cleanOnCtx := newModuleMessageContext(bot, chat, user, "/cleanlinked on")
	if err := pinsModule.cleanlinked(bot, cleanOnCtx); err != ext.EndGroups {
		t.Fatalf("cleanlinked on error = %v, want EndGroups", err)
	}
	if !db.GetPinData(chatID).CleanLinked {
		t.Fatal("CleanLinked was not enabled")
	}

	antiInvalidCtx := newModuleMessageContext(bot, chat, user, "/antichannelpin maybe")
	if err := pinsModule.antichannelpin(bot, antiInvalidCtx); err != ext.EndGroups {
		t.Fatalf("antichannelpin invalid error = %v, want EndGroups", err)
	}

	cleanOffCtx := newModuleMessageContext(bot, chat, user, "/cleanlinked off")
	if err := pinsModule.cleanlinked(bot, cleanOffCtx); err != ext.EndGroups {
		t.Fatalf("cleanlinked off error = %v, want EndGroups", err)
	}
	if db.GetPinData(chatID).CleanLinked {
		t.Fatal("CleanLinked stayed enabled")
	}
}

func TestCheckPinnedDeletesOrUnpinsLinkedChannelMessages(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	cleanChat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Pin Chat"}
	user := gotgbot.User{Id: 42, FirstName: "Member"}
	if err := db.SetCleanLinked(cleanChat.Id, true); err != nil {
		t.Fatalf("SetCleanLinked() error = %v", err)
	}
	cleanCtx := newModuleMessageContext(bot, cleanChat, user, "linked message")
	if err := pinsModule.checkPinned(bot, cleanCtx); err != ext.ContinueGroups {
		t.Fatalf("checkPinned clean error = %v, want ContinueGroups", err)
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != 1 {
		t.Fatalf("deleteMessage calls = %d, want 1", len(calls))
	}

	unpinChat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Pin Chat"}
	if err := db.SetAntiChannelPin(unpinChat.Id, true); err != nil {
		t.Fatalf("SetAntiChannelPin() error = %v", err)
	}
	unpinCtx := newModuleMessageContext(bot, unpinChat, user, "linked message")
	if err := pinsModule.checkPinned(bot, unpinCtx); err != ext.ContinueGroups {
		t.Fatalf("checkPinned unpin error = %v, want ContinueGroups", err)
	}
	if calls := client.callsFor("unpinChatMessage"); len(calls) != 1 {
		t.Fatalf("unpinChatMessage calls = %d, want 1", len(calls))
	}
}
