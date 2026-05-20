package modules

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func newPurgeReplyContext(
	bot *gotgbot.Bot,
	chat gotgbot.Chat,
	admin gotgbot.User,
	text string,
	messageID int64,
	replyID int64,
) *ext.Context {
	ctx := newModuleMessageContext(bot, chat, admin, text)
	ctx.EffectiveMessage.MessageId = messageID
	ctx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
		MessageId: replyID,
		Date:      1,
		Chat:      chat,
		From:      &gotgbot.User{Id: 42, FirstName: "Member"},
		Text:      "message to purge",
	}
	return ctx
}

func TestPurgeRequiresReplyAndDeletesRange(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Purge Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	noReplyCtx := newModuleMessageContext(bot, chat, admin, "/purge")
	if err := purgesModule.purge(bot, noReplyCtx); err != ext.EndGroups {
		t.Fatalf("purge no reply error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want reply-required message", len(calls))
	}

	replyCtx := newPurgeReplyContext(bot, chat, admin, "/purge cleanup", 105, 101)
	if err := purgesModule.purge(bot, replyCtx); err != ext.EndGroups {
		t.Fatalf("purge reply error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("deleteMessage"); len(calls) < 5 {
		t.Fatalf("deleteMessage calls = %d, want range and command deletions", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) < 2 {
		t.Fatalf("sendMessage calls = %d, want purge notification", len(calls))
	}
}

func TestDelCommandDeletesReplyAndCommandMessage(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Purge Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	noReplyCtx := newModuleMessageContext(bot, chat, admin, "/del")
	if err := purgesModule.delCmd(bot, noReplyCtx); err != ext.EndGroups {
		t.Fatalf("del no reply error = %v, want EndGroups", err)
	}

	replyCtx := newPurgeReplyContext(bot, chat, admin, "/del", 120, 119)
	if err := purgesModule.delCmd(bot, replyCtx); err != ext.EndGroups {
		t.Fatalf("del reply error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != 2 {
		t.Fatalf("deleteMessage calls = %d, want reply and command deletions", len(calls))
	}
}

func TestPurgeFromToDeletesMarkedRange(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Purge Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	delMsgs.Delete(chat.Id)

	missingFromCtx := newPurgeReplyContext(bot, chat, admin, "/purgeto", 210, 208)
	if err := purgesModule.purgeTo(bot, missingFromCtx); err != ext.EndGroups {
		t.Fatalf("purgeTo without marker error = %v, want EndGroups", err)
	}

	fromCtx := newPurgeReplyContext(bot, chat, admin, "/purgefrom", 220, 215)
	if err := purgesModule.purgeFrom(bot, fromCtx); err != ext.EndGroups {
		t.Fatalf("purgeFrom error = %v, want EndGroups", err)
	}
	if stored, ok := delMsgs.Load(chat.Id); !ok || stored.(int64) != 215 {
		t.Fatalf("stored purge marker = %v/%v, want 215", stored, ok)
	}

	toCtx := newPurgeReplyContext(bot, chat, admin, "/purgeto reason", 230, 218)
	if err := purgesModule.purgeTo(bot, toCtx); err != ext.EndGroups {
		t.Fatalf("purgeTo error = %v, want EndGroups", err)
	}
	if _, ok := delMsgs.Load(chat.Id); ok {
		t.Fatal("purge marker was not cleared after purgeTo")
	}
	if calls := client.callsFor("deleteMessage"); len(calls) < 6 {
		t.Fatalf("deleteMessage calls = %d, want marker, range, and command deletions", len(calls))
	}
}

func TestDeleteButtonHandlerDeletesTargetMessage(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Purge Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	validData := encodeCallbackData("deleteMsg", map[string]string{"m": "345"}, "deleteMsg.345")
	ctx := newModuleCallbackContext(bot, chat, admin, validData)
	if err := purgesModule.deleteButtonHandler(bot, ctx); err != ext.EndGroups {
		t.Fatalf("deleteButtonHandler error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != 1 {
		t.Fatalf("deleteMessage calls = %d, want callback target deletion", len(calls))
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 1 {
		t.Fatalf("answerCallbackQuery calls = %d, want callback answer", len(calls))
	}
}

func TestDeleteButtonHandlerRejectsInvalidData(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Purge Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	ctx := newModuleCallbackContext(bot, chat, admin, "deleteMsg.bad")
	if err := purgesModule.deleteButtonHandler(bot, ctx); err != ext.EndGroups {
		t.Fatalf("deleteButtonHandler invalid data error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != 0 {
		t.Fatalf("deleteMessage calls = %d, want no deletion", len(calls))
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 1 {
		t.Fatalf("answerCallbackQuery calls = %d, want error answer", len(calls))
	}
}
