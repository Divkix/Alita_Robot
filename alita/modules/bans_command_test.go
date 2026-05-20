package modules

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func newBanReplyContext(
	bot *gotgbot.Bot,
	chat gotgbot.Chat,
	admin gotgbot.User,
	target gotgbot.User,
	text string,
) *ext.Context {
	ctx := newModuleMessageContext(bot, chat, admin, text)
	ctx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
		MessageId: 303,
		Date:      1,
		Chat:      chat,
		From:      &target,
		Text:      "message being moderated",
	}
	return ctx
}

func TestBanReplyBansUserAndSendsUnbanButton(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Ban Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	target := gotgbot.User{Id: 42, FirstName: "Member"}

	ctx := newBanReplyContext(bot, chat, admin, target, "/ban spam")
	if err := bansModule.ban(bot, ctx); err != ext.EndGroups {
		t.Fatalf("ban() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("banChatMember"); len(calls) != 1 {
		t.Fatalf("banChatMember calls = %d, want 1", len(calls))
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
	if calls[0].Params["reply_markup"] == nil {
		t.Fatal("ban reply did not include unban button markup")
	}
}

func TestSilentBanDeletesCommandMessage(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Ban Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	target := gotgbot.User{Id: 42, FirstName: "Member"}

	ctx := newBanReplyContext(bot, chat, admin, target, "/sban")
	if err := bansModule.sBan(bot, ctx); err != ext.EndGroups {
		t.Fatalf("sBan() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("banChatMember"); len(calls) != 1 {
		t.Fatalf("banChatMember calls = %d, want 1", len(calls))
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != 1 {
		t.Fatalf("deleteMessage calls = %d, want 1", len(calls))
	}
}

func TestTemporaryBanUsesUntilDate(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Ban Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	target := gotgbot.User{Id: 42, FirstName: "Member"}

	ctx := newBanReplyContext(bot, chat, admin, target, "/tban 1h temp reason")
	if err := bansModule.tBan(bot, ctx); err != ext.EndGroups {
		t.Fatalf("tBan() error = %v, want EndGroups", err)
	}
	calls := client.callsFor("banChatMember")
	if len(calls) != 1 {
		t.Fatalf("banChatMember calls = %d, want 1", len(calls))
	}
	if calls[0].Params["until_date"] == nil {
		t.Fatalf("banChatMember params = %#v, want until_date", calls[0].Params)
	}
}

func TestKickUsesBanThenReply(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Ban Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	target := gotgbot.User{Id: 42, FirstName: "Member"}

	ctx := newBanReplyContext(bot, chat, admin, target, "/kick too much")
	if err := bansModule.kick(bot, ctx); err != ext.EndGroups {
		t.Fatalf("kick() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("banChatMember"); len(calls) != 1 {
		t.Fatalf("banChatMember calls = %d, want 1", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
}

func TestUnbanCommandUnbansUser(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Ban Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	ctx := newModuleMessageContext(bot, chat, admin, "/unban 42")
	if err := bansModule.unban(bot, ctx); err != ext.EndGroups {
		t.Fatalf("unban() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("unbanChatMember"); len(calls) != 1 {
		t.Fatalf("unbanChatMember calls = %d, want 1", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
}

func TestRestrictCommandAndMuteCallback(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Ban Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	ctx := newModuleMessageContext(bot, chat, admin, "/restrict 42")
	if err := bansModule.restrict(bot, ctx); err != ext.EndGroups {
		t.Fatalf("restrict() error = %v, want EndGroups", err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want restriction menu", len(calls))
	}
	if calls[0].Params["reply_markup"] == nil {
		t.Fatal("restrict menu did not include reply_markup")
	}

	data := encodeCallbackData("restrict", map[string]string{"a": "mute", "u": "42"}, "restrict.mute.42")
	callbackCtx := newModuleCallbackContext(bot, chat, admin, data)
	if err := bansModule.restrictButtonHandler(bot, callbackCtx); err != ext.EndGroups {
		t.Fatalf("restrictButtonHandler() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("restrictChatMember"); len(calls) != 1 {
		t.Fatalf("restrictChatMember calls = %d, want 1", len(calls))
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 1 {
		t.Fatalf("answerCallbackQuery calls = %d, want 1", len(calls))
	}
}

func TestUnrestrictCommandAndUnbanCallback(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Ban Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	ctx := newModuleMessageContext(bot, chat, admin, "/unrestrict 42")
	if err := bansModule.unrestrict(bot, ctx); err != ext.EndGroups {
		t.Fatalf("unrestrict() error = %v, want EndGroups", err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want unrestriction menu", len(calls))
	}
	if calls[0].Params["reply_markup"] == nil {
		t.Fatal("unrestrict menu did not include reply_markup")
	}

	data := encodeCallbackData("unrestrict", map[string]string{"a": "unban", "u": "42"}, "unrestrict.unban.42")
	callbackCtx := newModuleCallbackContext(bot, chat, admin, data)
	if err := bansModule.unrestrictButtonHandler(bot, callbackCtx); err != ext.EndGroups {
		t.Fatalf("unrestrictButtonHandler() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("unbanChatMember"); len(calls) != 1 {
		t.Fatalf("unbanChatMember calls = %d, want 1", len(calls))
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 1 {
		t.Fatalf("answerCallbackQuery calls = %d, want 1", len(calls))
	}
}
