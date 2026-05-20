package modules

import (
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func TestPingSendsAndEditsLatencyMessage(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Misc Chat"}
	user := gotgbot.User{Id: 42, FirstName: "Member"}
	ctx := newModuleMessageContext(bot, chat, user, "/ping")

	if err := miscModule.ping(bot, ctx); err != ext.EndGroups {
		t.Fatalf("ping() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
	if calls := client.callsFor("getMe"); len(calls) != 1 {
		t.Fatalf("getMe calls = %d, want 1", len(calls))
	}
	if calls := client.callsFor("editMessageText"); len(calls) != 1 {
		t.Fatalf("editMessageText calls = %d, want 1", len(calls))
	}
}

func TestStatRepliesWithGroupMessageCount(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Misc Chat"}
	user := gotgbot.User{Id: 42, FirstName: "Member"}
	ctx := newModuleMessageContext(bot, chat, user, "/stat")
	ctx.EffectiveMessage.MessageId = 123

	if err := miscModule.stat(bot, ctx); err != ext.EndGroups {
		t.Fatalf("stat() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
}

func TestRemoveBotKeyboardSendsKeyboardRemoval(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Misc Chat"}
	user := gotgbot.User{Id: 42, FirstName: "Member"}
	ctx := newModuleMessageContext(bot, chat, user, "/removebotkeyboard")

	if err := miscModule.removeBotKeyboard(bot, ctx); err != ext.EndGroups {
		t.Fatalf("removeBotKeyboard() error = %v, want EndGroups", err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
	if _, ok := calls[0].Params["reply_markup"].(*gotgbot.ReplyKeyboardRemove); !ok {
		t.Fatalf("reply_markup = %#v, want ReplyKeyboardRemove", calls[0].Params["reply_markup"])
	}

	time.Sleep(1100 * time.Millisecond)
	if calls := client.callsFor("deleteMessage"); len(calls) != 1 {
		t.Fatalf("deleteMessage calls after timer = %d, want 1", len(calls))
	}
}

func TestEchoMessageRequiresReplyAndContent(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Misc Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	noReplyCtx := newModuleMessageContext(bot, chat, user, "/tell hi")
	if err := miscModule.echomsg(bot, noReplyCtx); err != ext.EndGroups {
		t.Fatalf("echomsg no-reply error = %v, want EndGroups", err)
	}

	noContentCtx := newModuleMessageContext(bot, chat, user, "/tell")
	noContentCtx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
		MessageId: 55,
		Date:      1,
		Chat:      chat,
		From:      &gotgbot.User{Id: 88, FirstName: "Target"},
		Text:      "target",
	}
	if err := miscModule.echomsg(bot, noContentCtx); err != ext.EndGroups {
		t.Fatalf("echomsg no-content error = %v, want EndGroups", err)
	}

	echoCtx := newModuleMessageContext(bot, chat, user, "/tell hello there")
	echoCtx.EffectiveMessage.ReplyToMessage = noContentCtx.EffectiveMessage.ReplyToMessage
	if err := miscModule.echomsg(bot, echoCtx); err != ext.EndGroups {
		t.Fatalf("echomsg echo error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 3 {
		t.Fatalf("sendMessage calls = %d, want 3", len(calls))
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != 1 {
		t.Fatalf("deleteMessage calls = %d, want 1 for successful echo", len(calls))
	}
}

func TestTranslateMissingInputReplies(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Misc Chat"}
	user := gotgbot.User{Id: 42, FirstName: "Member"}
	ctx := newModuleMessageContext(bot, chat, user, "/tr")

	if err := miscModule.translate(bot, ctx); err != ext.EndGroups {
		t.Fatalf("translate() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
}

func TestGetIdRepliesForCurrentGroupUserAndReply(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Misc Chat"}
	user := gotgbot.User{Id: 42, FirstName: "Member"}

	groupCtx := newModuleMessageContext(bot, chat, user, "/id")
	if err := miscModule.getId(bot, groupCtx); err != ext.EndGroups {
		t.Fatalf("getId group error = %v, want EndGroups", err)
	}

	replyCtx := newModuleMessageContext(bot, chat, user, "/id")
	replyCtx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
		MessageId: 55,
		Date:      1,
		Chat:      chat,
		From:      &gotgbot.User{Id: 88, FirstName: "Target"},
		Text:      "target",
		Sticker:   &gotgbot.Sticker{FileId: "sticker-file-id"},
	}
	if err := miscModule.getId(bot, replyCtx); err != ext.EndGroups {
		t.Fatalf("getId reply error = %v, want EndGroups", err)
	}

	if calls := client.callsFor("sendMessage"); len(calls) != 2 {
		t.Fatalf("sendMessage calls = %d, want 2", len(calls))
	}
}

func TestInfoRepliesForUnknownNumericUser(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Misc Chat"}
	user := gotgbot.User{Id: 42, FirstName: "Member"}
	ctx := newModuleMessageContext(bot, chat, user, "/info 123456789")

	if err := miscModule.info(bot, ctx); err != ext.EndGroups {
		t.Fatalf("info() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
}
