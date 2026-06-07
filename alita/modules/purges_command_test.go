package modules

import (
	"fmt"
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

func TestPurgeMsgsConcurrentHandlesDeleteBoundariesAndErrors(t *testing.T) {
	tests := []struct {
		name       string
		pFrom      bool
		msgID      int64
		deleteTo   int64
		deleteErr  error
		wantOK     bool
		wantSends  int
		wantDelete int
	}{
		{
			name:       "empty range skips deletion when purging from marker",
			pFrom:      true,
			msgID:      10,
			deleteTo:   9,
			wantOK:     true,
			wantDelete: 0,
		},
		{
			name:       "old starting message sends explanation and continues",
			msgID:      10,
			deleteTo:   10,
			deleteErr:  fmt.Errorf("Bad Request: message can't be deleted"),
			wantOK:     true,
			wantSends:  1,
			wantDelete: 1,
		},
		{
			name:       "missing starting message is ignored",
			msgID:      10,
			deleteTo:   10,
			deleteErr:  fmt.Errorf("Bad Request: message to delete not found"),
			wantOK:     true,
			wantDelete: 1,
		},
		{
			name:       "unexpected starting delete error fails purge",
			msgID:      10,
			deleteTo:   10,
			deleteErr:  fmt.Errorf("Internal Server Error"),
			wantOK:     false,
			wantDelete: 1,
		},
		{
			name:       "large ranges use worker deletion path",
			pFrom:      true,
			msgID:      1,
			deleteTo:   12,
			wantOK:     true,
			wantDelete: 12,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Purge Chat"}
			if tt.deleteErr != nil {
				client.errors["deleteMessage"] = tt.deleteErr
			}

			got := purgesModule.purgeMsgsConcurrent(bot, &chat, tt.pFrom, tt.msgID, tt.deleteTo)
			if got != tt.wantOK {
				t.Fatalf("purgeMsgsConcurrent() = %v, want %v", got, tt.wantOK)
			}
			if calls := client.callsFor("deleteMessage"); len(calls) != tt.wantDelete {
				t.Fatalf("deleteMessage calls = %d, want %d", len(calls), tt.wantDelete)
			}
			if calls := client.callsFor("sendMessage"); len(calls) != tt.wantSends {
				t.Fatalf("sendMessage calls = %d, want %d", len(calls), tt.wantSends)
			}
		})
	}
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

func TestPurgeRejectsOverLimitRange(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Purge Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	ctx := newPurgeReplyContext(bot, chat, admin, "/purge", 2005, 1)
	if err := purgesModule.purge(bot, ctx); err != ext.EndGroups {
		t.Fatalf("purge over limit error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != 0 {
		t.Fatalf("deleteMessage calls = %d, want none for over-limit purge", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want limit message", len(calls))
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

func TestPurgeFromToValidationBranches(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Purge Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	delMsgs.Delete(chat.Id)

	noFromReplyCtx := newModuleMessageContext(bot, chat, admin, "/purgefrom")
	if err := purgesModule.purgeFrom(bot, noFromReplyCtx); err != ext.EndGroups {
		t.Fatalf("purgeFrom no reply error = %v, want EndGroups", err)
	}

	noToReplyCtx := newModuleMessageContext(bot, chat, admin, "/purgeto")
	if err := purgesModule.purgeTo(bot, noToReplyCtx); err != ext.EndGroups {
		t.Fatalf("purgeTo no reply error = %v, want EndGroups", err)
	}

	delMsgs.Store(chat.Id, int64(300))
	sameMessageCtx := newPurgeReplyContext(bot, chat, admin, "/purgeto", 310, 300)
	if err := purgesModule.purgeTo(bot, sameMessageCtx); err != ext.EndGroups {
		t.Fatalf("purgeTo same message error = %v, want EndGroups", err)
	}

	delMsgs.Store(chat.Id, int64(1))
	overLimitCtx := newPurgeReplyContext(bot, chat, admin, "/purgeto", 2100, 1002)
	if err := purgesModule.purgeTo(bot, overLimitCtx); err != ext.EndGroups {
		t.Fatalf("purgeTo over limit error = %v, want EndGroups", err)
	}
	if _, ok := delMsgs.Load(chat.Id); !ok {
		t.Fatal("purge marker was deleted for rejected over-limit purgeto")
	}
	delMsgs.Delete(chat.Id)

	if calls := client.callsFor("sendMessage"); len(calls) != 4 {
		t.Fatalf("sendMessage calls = %d, want one validation reply per branch", len(calls))
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != 0 {
		t.Fatalf("deleteMessage calls = %d, want no deletion for validation branches", len(calls))
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
