package modules

import (
	"errors"
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

func TestBanCommandRejectsMissingChannelAndProtectedTargets(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Ban Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	tests := []struct {
		name string
		text string
	}{
		{name: "missing target", text: "/ban"},
		{name: "channel id without reply", text: "/ban -1001234567890"},
		{name: "protected service admin", text: "/ban 777000"},
		{name: "bot itself", text: "/ban 999"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newModuleMessageContext(bot, chat, admin, tt.text)
			if err := bansModule.ban(bot, ctx); err != ext.EndGroups {
				t.Fatalf("ban(%s) error = %v, want EndGroups", tt.name, err)
			}
		})
	}
	if calls := client.callsFor("banChatMember"); len(calls) != 0 {
		t.Fatalf("banChatMember calls = %d, want none for rejected targets", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != len(tests) {
		t.Fatalf("sendMessage calls = %d, want one denial per rejected target", len(calls))
	}
}

func TestBanCommandBansAnonymousChannelFromReply(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Ban Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	channel := gotgbot.Chat{Id: -1001234567890, Type: "channel", Title: "Spam Channel"}
	ctx := newModuleMessageContext(bot, chat, admin, "/ban -1001234567890")
	ctx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
		MessageId: 304,
		Date:      1,
		Chat:      chat,
		SenderChat: &gotgbot.Chat{
			Id:    channel.Id,
			Type:  channel.Type,
			Title: channel.Title,
		},
		Text: "channel post",
	}

	if err := bansModule.ban(bot, ctx); err != ext.EndGroups {
		t.Fatalf("ban(anonymous channel) error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("banChatSenderChat"); len(calls) != 1 {
		t.Fatalf("banChatSenderChat calls = %d, want 1", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want anonymous ban confirmation", len(calls))
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

func TestSilentBanRejectsBotTarget(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Ban Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	ctx := newModuleMessageContext(bot, chat, admin, "/sban 999")
	if err := bansModule.sBan(bot, ctx); err != ext.EndGroups {
		t.Fatalf("sBan(bot itself) error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("banChatMember"); len(calls) != 0 {
		t.Fatalf("banChatMember calls = %d, want none for bot target", len(calls))
	}
}

func TestSilentAndDeleteBanRejectInvalidTargets(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Ban Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	for _, call := range []struct {
		name string
		text string
		run  func(*gotgbot.Bot, *ext.Context) error
	}{
		{name: "sban missing", text: "/sban", run: bansModule.sBan},
		{name: "sban channel", text: "/sban -1001234567890", run: bansModule.sBan},
		{name: "dban missing", text: "/dban", run: bansModule.dBan},
		{name: "dkick missing", text: "/dkick", run: bansModule.dkick},
	} {
		t.Run(call.name, func(t *testing.T) {
			ctx := newModuleMessageContext(bot, chat, admin, call.text)
			if err := call.run(bot, ctx); err != ext.EndGroups {
				t.Fatalf("%s error = %v, want EndGroups", call.name, err)
			}
		})
	}

	if calls := client.callsFor("banChatMember"); len(calls) != 0 {
		t.Fatalf("banChatMember calls = %d, want none for invalid target tests", len(calls))
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

func TestTemporaryBanRejectsInvalidDurationAndProtectedTarget(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Ban Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	target := gotgbot.User{Id: 42, FirstName: "Member"}

	badDurationCtx := newBanReplyContext(bot, chat, admin, target, "/tban nope invalid")
	if err := bansModule.tBan(bot, badDurationCtx); err != ext.EndGroups {
		t.Fatalf("tBan(invalid duration) error = %v, want EndGroups", err)
	}

	protectedCtx := newModuleMessageContext(bot, chat, admin, "/tban 777000 1h")
	if err := bansModule.tBan(bot, protectedCtx); err != ext.EndGroups {
		t.Fatalf("tBan(protected) error = %v, want EndGroups", err)
	}

	if calls := client.callsFor("banChatMember"); len(calls) != 0 {
		t.Fatalf("banChatMember calls = %d, want none for rejected temporary bans", len(calls))
	}
}

func TestTemporaryBanRejectsMissingChannelAndBotTargets(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Ban Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	for _, tt := range []struct {
		name string
		text string
	}{
		{name: "missing", text: "/tban"},
		{name: "channel id", text: "/tban -1001234567890 1h"},
		{name: "bot itself", text: "/tban 999 1h"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newModuleMessageContext(bot, chat, admin, tt.text)
			if err := bansModule.tBan(bot, ctx); err != ext.EndGroups {
				t.Fatalf("tBan(%s) error = %v, want EndGroups", tt.name, err)
			}
		})
	}
	if calls := client.callsFor("banChatMember"); len(calls) != 0 {
		t.Fatalf("banChatMember calls = %d, want none for invalid tban targets", len(calls))
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

func TestKickRejectsInvalidAndProtectedTargets(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Ban Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	for _, tt := range []struct {
		name string
		text string
	}{
		{name: "missing target", text: "/kick"},
		{name: "anonymous channel", text: "/kick -1001234567890"},
		{name: "telegram service admin", text: "/kick 777000"},
		{name: "bot itself", text: "/kick 999"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newModuleMessageContext(bot, chat, admin, tt.text)
			if err := bansModule.kick(bot, ctx); err != ext.EndGroups {
				t.Fatalf("kick(%s) error = %v, want EndGroups", tt.name, err)
			}
		})
	}

	if calls := client.callsFor("banChatMember"); len(calls) != 0 {
		t.Fatalf("banChatMember calls = %d, want none for rejected kick targets", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 4 {
		t.Fatalf("sendMessage calls = %d, want one denial per rejected target", len(calls))
	}
}

func TestKickRejectsTargetNotInChat(t *testing.T) {
	client := newModuleBotClient()
	client.responses["getChatMember"] = []byte(
		`{"status":"left","user":{"id":42,"is_bot":false,"first_name":"Gone"}}`,
	)
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Ban Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	ctx := newModuleMessageContext(bot, chat, admin, "/kick 42")
	if err := bansModule.kick(bot, ctx); err != ext.EndGroups {
		t.Fatalf("kick(user left) error = %v, want EndGroups", err)
	}

	if calls := client.callsFor("banChatMember"); len(calls) != 0 {
		t.Fatalf("banChatMember calls = %d, want none for target outside chat", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want user-not-in-chat denial", len(calls))
	}
}

func TestDeleteKickDeletesReplyBeforeKick(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Ban Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	target := gotgbot.User{Id: 42, FirstName: "Member"}

	ctx := newBanReplyContext(bot, chat, admin, target, "/dkick cleanup")
	if err := bansModule.dkick(bot, ctx); err != ext.EndGroups {
		t.Fatalf("dkick() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != 1 {
		t.Fatalf("deleteMessage calls = %d, want replied message deletion", len(calls))
	}
	if calls := client.callsFor("banChatMember"); len(calls) != 1 {
		t.Fatalf("banChatMember calls = %d, want 1", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
}

func TestDeleteKickDeletesReplyThenKicks(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Ban Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	target := gotgbot.User{Id: 42, FirstName: "Member"}

	ctx := newBanReplyContext(bot, chat, admin, target, "/dkick clean this")
	if err := bansModule.dkick(bot, ctx); err != ext.EndGroups {
		t.Fatalf("dkick() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != 1 {
		t.Fatalf("deleteMessage calls = %d, want replied message deletion", len(calls))
	}
	if calls := client.callsFor("banChatMember"); len(calls) != 1 {
		t.Fatalf("banChatMember calls = %d, want kick ban", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want kick confirmation", len(calls))
	}
}

func TestDeleteKickRejectsUnidentifiableReplySender(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Ban Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	ctx := newModuleMessageContext(bot, chat, admin, "/dkick")
	ctx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
		MessageId: 404,
		Date:      1,
		Chat:      chat,
		Text:      "senderless message",
	}

	if err := bansModule.dkick(bot, ctx); err != ext.EndGroups {
		t.Fatalf("dkick(senderless reply) error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != 0 {
		t.Fatalf("deleteMessage calls = %d, want none for unidentifiable sender", len(calls))
	}
	if calls := client.callsFor("banChatMember"); len(calls) != 0 {
		t.Fatalf("banChatMember calls = %d, want none for unidentifiable sender", len(calls))
	}
}

func TestDeleteKickRejectsInvalidReplyTargets(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Ban Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	tests := []struct {
		name   string
		target gotgbot.User
	}{
		{name: "anonymous channel", target: gotgbot.User{Id: -1001234567890, FirstName: "Channel"}},
		{name: "zero user id", target: gotgbot.User{Id: 0, FirstName: "Unknown"}},
		{name: "left user", target: gotgbot.User{Id: 13, FirstName: "Left"}},
		{name: "protected admin", target: gotgbot.User{Id: 777000, FirstName: "Telegram"}},
		{name: "bot itself", target: gotgbot.User{Id: bot.Id, FirstName: "Alita"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newBanReplyContext(bot, chat, admin, tt.target, "/dkick cleanup")
			if err := bansModule.dkick(bot, ctx); err != ext.EndGroups {
				t.Fatalf("dkick(%s) error = %v, want EndGroups", tt.name, err)
			}
		})
	}

	if calls := client.callsFor("banChatMember"); len(calls) != 0 {
		t.Fatalf("banChatMember calls = %d, want none for rejected dkick targets", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != len(tests) {
		t.Fatalf("sendMessage calls = %d, want one denial per rejected dkick target", len(calls))
	}
}

func TestDeleteBanDeletesReplyBeforeBan(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Ban Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	target := gotgbot.User{Id: 42, FirstName: "Member"}

	ctx := newBanReplyContext(bot, chat, admin, target, "/dban bad post")
	if err := bansModule.dBan(bot, ctx); err != ext.EndGroups {
		t.Fatalf("dBan() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != 1 {
		t.Fatalf("deleteMessage calls = %d, want replied message deletion", len(calls))
	}
	if calls := client.callsFor("banChatMember"); len(calls) != 1 {
		t.Fatalf("banChatMember calls = %d, want 1", len(calls))
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
	if calls[0].Params["reply_markup"] == nil {
		t.Fatal("dban reply did not include unban button markup")
	}
}

func TestDeleteBanRejectsValidTargetWithoutReplyAndProtectedTarget(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Ban Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	noReplyCtx := newModuleMessageContext(bot, chat, admin, "/dban 42")
	if err := bansModule.dBan(bot, noReplyCtx); err != ext.EndGroups {
		t.Fatalf("dBan(valid target without reply) error = %v, want EndGroups", err)
	}

	protectedCtx := newModuleMessageContext(bot, chat, admin, "/dban 777000")
	if err := bansModule.dBan(bot, protectedCtx); err != ext.EndGroups {
		t.Fatalf("dBan(protected target) error = %v, want EndGroups", err)
	}

	if calls := client.callsFor("deleteMessage"); len(calls) != 0 {
		t.Fatalf("deleteMessage calls = %d, want none for rejected dban", len(calls))
	}
	if calls := client.callsFor("banChatMember"); len(calls) != 0 {
		t.Fatalf("banChatMember calls = %d, want none for rejected dban", len(calls))
	}
}

func TestKickMeBansRequesterAndReplies(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Ban Chat"}
	target := gotgbot.User{Id: 42, FirstName: "Member"}

	ctx := newModuleMessageContext(bot, chat, target, "/kickme")
	if err := bansModule.kickme(bot, ctx); err != ext.EndGroups {
		t.Fatalf("kickme() error = %v, want EndGroups", err)
	}
	calls := client.callsFor("banChatMember")
	if len(calls) != 1 {
		t.Fatalf("banChatMember calls = %d, want 1", len(calls))
	}
	if got := calls[0].Params["user_id"]; got != int64(42) {
		t.Fatalf("banChatMember user_id = %v, want 42", got)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
}

func TestKickMePropagatesGotgbotRequestErrors(t *testing.T) {
	requestErr := errors.New("telegram request failed")
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Ban Chat"}
	target := gotgbot.User{Id: 42, FirstName: "Member"}

	for _, tt := range []struct {
		name   string
		method string
	}{
		{name: "kickme ban failure", method: "banChatMember"},
		{name: "kickme reply failure", method: "sendMessage"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			client.errors[tt.method] = requestErr
			ctx := newModuleMessageContext(bot, chat, target, "/kickme")

			err := bansModule.kickme(bot, ctx)
			if !errors.Is(err, requestErr) {
				t.Fatalf("kickme returned error %v, want request error", err)
			}
		})
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

func TestUnbanRejectsMissingAndBotTargets(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Ban Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	for _, tt := range []struct {
		name string
		text string
	}{
		{name: "missing", text: "/unban"},
		{name: "bot itself", text: "/unban 999"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newModuleMessageContext(bot, chat, admin, tt.text)
			if err := bansModule.unban(bot, ctx); err != ext.EndGroups {
				t.Fatalf("unban(%s) error = %v, want EndGroups", tt.name, err)
			}
		})
	}
	if calls := client.callsFor("unbanChatMember"); len(calls) != 0 {
		t.Fatalf("unbanChatMember calls = %d, want none for rejected unbans", len(calls))
	}
}

func TestUnbanAnonymousChannelBranches(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Ban Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	noReplyCtx := newModuleMessageContext(bot, chat, admin, "/unban -1001234567890")
	if err := bansModule.unban(bot, noReplyCtx); err != ext.EndGroups {
		t.Fatalf("unban anonymous without reply error = %v, want EndGroups", err)
	}

	replyCtx := newModuleMessageContext(bot, chat, admin, "/unban -1001234567890")
	replyCtx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
		MessageId: 304,
		Date:      1,
		Chat:      chat,
		SenderChat: &gotgbot.Chat{
			Id:    -1001234567890,
			Type:  "channel",
			Title: "Spam Channel",
		},
		Text: "channel post",
	}
	if err := bansModule.unban(bot, replyCtx); err != ext.EndGroups {
		t.Fatalf("unban anonymous reply error = %v, want EndGroups", err)
	}

	if calls := client.callsFor("unbanChatSenderChat"); len(calls) != 1 {
		t.Fatalf("unbanChatSenderChat calls = %d, want 1", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 2 {
		t.Fatalf("sendMessage calls = %d, want both anonymous branch replies", len(calls))
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

func TestRestrictCommandRejectsInvalidTargets(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Ban Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	for _, tt := range []struct {
		name string
		text string
	}{
		{name: "missing", text: "/restrict"},
		{name: "admin", text: "/restrict 777000"},
		{name: "bot itself", text: "/restrict 999"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newModuleMessageContext(bot, chat, admin, tt.text)
			if err := bansModule.restrict(bot, ctx); err != ext.EndGroups {
				t.Fatalf("restrict(%s) error = %v, want EndGroups", tt.name, err)
			}
		})
	}
	if calls := client.callsFor("restrictChatMember"); len(calls) != 0 {
		t.Fatalf("restrictChatMember calls = %d, want none from command validation", len(calls))
	}
}

func TestRestrictCallbacksApplyKickBanAndInvalidUser(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Ban Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	for _, action := range []string{"kick", "ban"} {
		data := encodeCallbackData("restrict", map[string]string{"a": action, "u": "42"}, "restrict."+action+".42")
		ctx := newModuleCallbackContext(bot, chat, admin, data)
		if err := bansModule.restrictButtonHandler(bot, ctx); err != ext.EndGroups {
			t.Fatalf("restrictButtonHandler(%s) error = %v, want EndGroups", action, err)
		}
	}

	invalidCtx := newModuleCallbackContext(bot, chat, admin, "restrict.mute.not-a-number")
	if err := bansModule.restrictButtonHandler(bot, invalidCtx); err != ext.EndGroups {
		t.Fatalf("restrictButtonHandler(invalid user) error = %v, want EndGroups", err)
	}

	if calls := client.callsFor("banChatMember"); len(calls) != 2 {
		t.Fatalf("banChatMember calls = %d, want kick and ban actions", len(calls))
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 3 {
		t.Fatalf("answerCallbackQuery calls = %d, want two successes and invalid user answer", len(calls))
	}
	if calls := client.callsFor("editMessageText"); len(calls) != 2 {
		t.Fatalf("editMessageText calls = %d, want two successful callback edits", len(calls))
	}
}

func TestRestrictCallbacksRejectMalformedAndNonAdminUsers(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Ban Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}

	malformedCtx := newModuleCallbackContext(bot, chat, admin, "restrict")
	if err := bansModule.restrictButtonHandler(bot, malformedCtx); err != ext.EndGroups {
		t.Fatalf("restrictButtonHandler(malformed) error = %v, want EndGroups", err)
	}

	nonAdminCtx := newModuleCallbackContext(
		bot,
		chat,
		member,
		encodeCallbackData("restrict", map[string]string{"a": "mute", "u": "42"}, "restrict.mute.42"),
	)
	if err := bansModule.restrictButtonHandler(bot, nonAdminCtx); err != ext.EndGroups {
		t.Fatalf("restrictButtonHandler(non-admin) error = %v, want EndGroups", err)
	}

	badUnrestrictCtx := newModuleCallbackContext(bot, chat, admin, "unrestrict")
	if err := bansModule.unrestrictButtonHandler(bot, badUnrestrictCtx); err != ext.EndGroups {
		t.Fatalf("unrestrictButtonHandler(malformed) error = %v, want EndGroups", err)
	}

	if calls := client.callsFor("restrictChatMember"); len(calls) != 0 {
		t.Fatalf("restrictChatMember calls = %d, want none for rejected callbacks", len(calls))
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 2 {
		t.Fatalf("answerCallbackQuery calls = %d, want malformed callback answers", len(calls))
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

func TestUnrestrictCommandRejectsInvalidTargets(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Ban Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	for _, tt := range []struct {
		name string
		text string
	}{
		{name: "missing", text: "/unrestrict"},
		{name: "admin", text: "/unrestrict 777000"},
		{name: "bot itself", text: "/unrestrict 999"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newModuleMessageContext(bot, chat, admin, tt.text)
			if err := bansModule.unrestrict(bot, ctx); err != ext.EndGroups {
				t.Fatalf("unrestrict(%s) error = %v, want EndGroups", tt.name, err)
			}
		})
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 3 {
		t.Fatalf("sendMessage calls = %d, want validation replies", len(calls))
	}
}

func TestUnrestrictCallbacksApplyUnmuteAndInvalidUser(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Ban Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	unmuteData := encodeCallbackData("unrestrict", map[string]string{"a": "unmute", "u": "42"}, "unrestrict.unmute.42")
	unmuteCtx := newModuleCallbackContext(bot, chat, admin, unmuteData)
	if err := bansModule.unrestrictButtonHandler(bot, unmuteCtx); err != ext.EndGroups {
		t.Fatalf("unrestrictButtonHandler(unmute) error = %v, want EndGroups", err)
	}

	invalidCtx := newModuleCallbackContext(bot, chat, admin, "unrestrict.unmute.not-a-number")
	if err := bansModule.unrestrictButtonHandler(bot, invalidCtx); err != ext.EndGroups {
		t.Fatalf("unrestrictButtonHandler(invalid user) error = %v, want EndGroups", err)
	}

	if calls := client.callsFor("restrictChatMember"); len(calls) != 1 {
		t.Fatalf("restrictChatMember calls = %d, want unmute action", len(calls))
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 2 {
		t.Fatalf("answerCallbackQuery calls = %d, want success and invalid user answer", len(calls))
	}
	if calls := client.callsFor("editMessageText"); len(calls) != 1 {
		t.Fatalf("editMessageText calls = %d, want unmute callback edit", len(calls))
	}
}

func TestKickMeRejectsAdminsAndLoadBansRegistersHelp(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Ban Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	ctx := newModuleMessageContext(bot, chat, admin, "/kickme")
	if err := bansModule.kickme(bot, ctx); err != ext.EndGroups {
		t.Fatalf("kickme(admin) error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("banChatMember"); len(calls) != 0 {
		t.Fatalf("banChatMember calls = %d, want none for admin kickme", len(calls))
	}

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadBans(dispatcher)
	if moduleName, enabled := DefaultHelpRegistry().AbleMap.Load(bansModule.moduleName); moduleName != bansModule.moduleName || !enabled {
		t.Fatalf("bans help registration = (%q, %v), want enabled", moduleName, enabled)
	}
}

func TestBanCommandsPropagateGotgbotRequestErrors(t *testing.T) {
	requestErr := errors.New("telegram request failed")
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Ban Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	target := gotgbot.User{Id: 42, FirstName: "Member"}
	channel := gotgbot.Chat{Id: -1001234567890, Type: "channel", Title: "Spam Channel"}
	channelReplyContext := func(bot *gotgbot.Bot, text string) *ext.Context {
		ctx := newModuleMessageContext(bot, chat, admin, text)
		ctx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
			MessageId: 304,
			Date:      1,
			Chat:      chat,
			SenderChat: &gotgbot.Chat{
				Id:    channel.Id,
				Type:  channel.Type,
				Title: channel.Title,
			},
			Text: "channel post",
		}
		return ctx
	}

	for _, tt := range []struct {
		name   string
		method string
		text   string
		ctx    func(*gotgbot.Bot) *ext.Context
		run    func(*gotgbot.Bot, *ext.Context) error
	}{
		{
			name:   "kick member failure",
			method: "banChatMember",
			text:   "/kick spam",
			ctx:    func(bot *gotgbot.Bot) *ext.Context { return newBanReplyContext(bot, chat, admin, target, "/kick spam") },
			run:    bansModule.kick,
		},
		{
			name:   "kick get chat failure",
			method: "getChat",
			text:   "/kick spam",
			ctx:    func(bot *gotgbot.Bot) *ext.Context { return newBanReplyContext(bot, chat, admin, target, "/kick spam") },
			run:    bansModule.kick,
		},
		{
			name:   "kick send failure",
			method: "sendMessage",
			text:   "/kick spam",
			ctx:    func(bot *gotgbot.Bot) *ext.Context { return newBanReplyContext(bot, chat, admin, target, "/kick spam") },
			run:    bansModule.kick,
		},
		{
			name:   "ban member failure",
			method: "banChatMember",
			text:   "/ban spam",
			ctx:    func(bot *gotgbot.Bot) *ext.Context { return newBanReplyContext(bot, chat, admin, target, "/ban spam") },
			run:    bansModule.ban,
		},
		{
			name:   "ban anonymous sender failure",
			method: "banChatSenderChat",
			text:   "/ban -1001234567890",
			ctx:    func(bot *gotgbot.Bot) *ext.Context { return channelReplyContext(bot, "/ban -1001234567890") },
			run:    bansModule.ban,
		},
		{
			name:   "ban send failure",
			method: "sendMessage",
			text:   "/ban spam",
			ctx:    func(bot *gotgbot.Bot) *ext.Context { return newBanReplyContext(bot, chat, admin, target, "/ban spam") },
			run:    bansModule.ban,
		},
		{
			name:   "temporary ban failure",
			method: "banChatMember",
			text:   "/tban 1h spam",
			ctx: func(bot *gotgbot.Bot) *ext.Context {
				return newBanReplyContext(bot, chat, admin, target, "/tban 1h spam")
			},
			run: bansModule.tBan,
		},
		{
			name:   "temporary ban get chat failure",
			method: "getChat",
			text:   "/tban 1h spam",
			ctx: func(bot *gotgbot.Bot) *ext.Context {
				return newBanReplyContext(bot, chat, admin, target, "/tban 1h spam")
			},
			run: bansModule.tBan,
		},
		{
			name:   "silent ban member failure",
			method: "banChatMember",
			text:   "/sban",
			ctx:    func(bot *gotgbot.Bot) *ext.Context { return newBanReplyContext(bot, chat, admin, target, "/sban") },
			run:    bansModule.sBan,
		},
		{
			name:   "silent ban delete failure",
			method: "deleteMessage",
			text:   "/sban",
			ctx:    func(bot *gotgbot.Bot) *ext.Context { return newBanReplyContext(bot, chat, admin, target, "/sban") },
			run:    bansModule.sBan,
		},
		{
			name:   "delete ban delete failure",
			method: "deleteMessage",
			text:   "/dban bad post",
			ctx: func(bot *gotgbot.Bot) *ext.Context {
				return newBanReplyContext(bot, chat, admin, target, "/dban bad post")
			},
			run: bansModule.dBan,
		},
		{
			name:   "delete ban member failure",
			method: "banChatMember",
			text:   "/dban bad post",
			ctx: func(bot *gotgbot.Bot) *ext.Context {
				return newBanReplyContext(bot, chat, admin, target, "/dban bad post")
			},
			run: bansModule.dBan,
		},
		{
			name:   "delete ban get chat failure",
			method: "getChat",
			text:   "/dban bad post",
			ctx: func(bot *gotgbot.Bot) *ext.Context {
				return newBanReplyContext(bot, chat, admin, target, "/dban bad post")
			},
			run: bansModule.dBan,
		},
		{
			name:   "delete kick member failure",
			method: "banChatMember",
			text:   "/dkick cleanup",
			ctx: func(bot *gotgbot.Bot) *ext.Context {
				return newBanReplyContext(bot, chat, admin, target, "/dkick cleanup")
			},
			run: bansModule.dkick,
		},
		{
			name:   "delete kick get chat failure",
			method: "getChat",
			text:   "/dkick cleanup",
			ctx: func(bot *gotgbot.Bot) *ext.Context {
				return newBanReplyContext(bot, chat, admin, target, "/dkick cleanup")
			},
			run: bansModule.dkick,
		},
		{
			name:   "unban member failure",
			method: "unbanChatMember",
			text:   "/unban 42",
			ctx:    func(bot *gotgbot.Bot) *ext.Context { return newModuleMessageContext(bot, chat, admin, "/unban 42") },
			run:    bansModule.unban,
		},
		{
			name:   "unban get chat failure",
			method: "getChat",
			text:   "/unban 42",
			ctx:    func(bot *gotgbot.Bot) *ext.Context { return newModuleMessageContext(bot, chat, admin, "/unban 42") },
			run:    bansModule.unban,
		},
		{
			name:   "unban anonymous sender failure",
			method: "unbanChatSenderChat",
			text:   "/unban -1001234567890",
			ctx:    func(bot *gotgbot.Bot) *ext.Context { return channelReplyContext(bot, "/unban -1001234567890") },
			run:    bansModule.unban,
		},
		{
			name:   "unban send failure",
			method: "sendMessage",
			text:   "/unban 42",
			ctx:    func(bot *gotgbot.Bot) *ext.Context { return newModuleMessageContext(bot, chat, admin, "/unban 42") },
			run:    bansModule.unban,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			client.errors[tt.method] = requestErr

			err := tt.run(bot, tt.ctx(bot))
			if !errors.Is(err, requestErr) {
				t.Fatalf("%s returned error %v, want request error", tt.text, err)
			}
		})
	}
}

func TestModerationHelpersValidateExtractionAndTargets(t *testing.T) {
	t.Run("extract from reply validation branches", func(t *testing.T) {
		client := newModuleBotClient()
		bot := newModuleTestBot(client)
		chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Ban Chat"}
		admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
		ctx := newModuleMessageContext(bot, chat, admin, "/dkick")
		mc, err := buildModerationCtx(&bansModule, bot, ctx)
		if err != nil {
			t.Fatalf("buildModerationCtx() error = %v", err)
		}

		if _, err := extractFromReply(mc); err == nil {
			t.Fatal("extractFromReply(no reply) error = nil, want validation error")
		}

		ctx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
			MessageId: 404,
			Date:      1,
			Chat:      chat,
			Text:      "senderless",
		}
		if _, err := extractFromReply(mc); err == nil {
			t.Fatal("extractFromReply(nil sender) error = nil, want validation error")
		}

		if calls := client.callsFor("sendMessage"); len(calls) != 2 {
			t.Fatalf("sendMessage calls = %d, want one reply per extraction failure", len(calls))
		}
	})

	t.Run("default target validation rejects missing and protected users", func(t *testing.T) {
		client := newModuleBotClient()
		bot := newModuleTestBot(client)
		chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Ban Chat"}
		admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
		ctx := newModuleMessageContext(bot, chat, admin, "/ban 42")
		mc, err := buildModerationCtx(&bansModule, bot, ctx)
		if err != nil {
			t.Fatalf("buildModerationCtx() error = %v", err)
		}

		client.responses["getChatMember"] = []byte(
			`{"status":"left","user":{"id":42,"is_bot":false,"first_name":"Gone"}}`,
		)
		if err := defaultTargetValidation(mc, &target{userID: 42}); err == nil {
			t.Fatal("defaultTargetValidation(left user) error = nil, want validation error")
		}

		protectedClient := newModuleBotClient()
		protectedClient.responses["getChatMember"] = []byte(
			`{"status":"administrator","user":{"id":4242,"is_bot":false,"first_name":"Admin"},"can_restrict_members":true}`,
		)
		protectedClient.responses["getChatAdministrators"] = []byte(
			`[
				{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"}},
				{"status":"administrator","user":{"id":4242,"is_bot":false,"first_name":"Admin"}}
			]`,
		)
		protectedBot := newModuleTestBot(protectedClient)
		protectedCtx := newModuleMessageContext(protectedBot, chat, admin, "/ban 4242")
		protectedMC, err := buildModerationCtx(&bansModule, protectedBot, protectedCtx)
		if err != nil {
			t.Fatalf("buildModerationCtx(protected) error = %v", err)
		}
		if err := defaultTargetValidation(protectedMC, &target{userID: 4242}); err == nil {
			t.Fatal("defaultTargetValidation(admin user) error = nil, want validation error")
		}

		if calls := client.callsFor("sendMessage"); len(calls) != 1 {
			t.Fatalf("left-user sendMessage calls = %d, want 1", len(calls))
		}
		if calls := protectedClient.callsFor("sendMessage"); len(calls) != 1 {
			t.Fatalf("protected-user sendMessage calls = %d, want 1", len(calls))
		}
	})
}

func TestRestrictCommandsAndCallbacksPropagateGotgbotRequestErrors(t *testing.T) {
	requestErr := errors.New("telegram request failed")
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Ban Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	restrictData := encodeCallbackData("restrict", map[string]string{"a": "mute", "u": "42"}, "restrict.mute.42")
	unrestrictData := encodeCallbackData("unrestrict", map[string]string{"a": "unmute", "u": "42"}, "unrestrict.unmute.42")
	unbanData := encodeCallbackData("unrestrict", map[string]string{"a": "unban", "u": "42"}, "unrestrict.unban.42")

	for _, tt := range []struct {
		name   string
		method string
		ctx    func(*gotgbot.Bot) *ext.Context
		run    func(*gotgbot.Bot, *ext.Context) error
	}{
		{
			name:   "restrict command send failure",
			method: "sendMessage",
			ctx:    func(bot *gotgbot.Bot) *ext.Context { return newModuleMessageContext(bot, chat, admin, "/restrict 42") },
			run:    bansModule.restrict,
		},
		{
			name:   "unrestrict command send failure",
			method: "sendMessage",
			ctx: func(bot *gotgbot.Bot) *ext.Context {
				return newModuleMessageContext(bot, chat, admin, "/unrestrict 42")
			},
			run: bansModule.unrestrict,
		},
		{
			name:   "restrict callback get chat failure",
			method: "getChat",
			ctx:    func(bot *gotgbot.Bot) *ext.Context { return newModuleCallbackContext(bot, chat, admin, restrictData) },
			run:    bansModule.restrictButtonHandler,
		},
		{
			name:   "restrict callback mute failure",
			method: "restrictChatMember",
			ctx:    func(bot *gotgbot.Bot) *ext.Context { return newModuleCallbackContext(bot, chat, admin, restrictData) },
			run:    bansModule.restrictButtonHandler,
		},
		{
			name:   "restrict callback ban failure",
			method: "banChatMember",
			ctx: func(bot *gotgbot.Bot) *ext.Context {
				data := encodeCallbackData("restrict", map[string]string{"a": "ban", "u": "42"}, "restrict.ban.42")
				return newModuleCallbackContext(bot, chat, admin, data)
			},
			run: bansModule.restrictButtonHandler,
		},
		{
			name:   "restrict callback edit failure",
			method: "editMessageText",
			ctx:    func(bot *gotgbot.Bot) *ext.Context { return newModuleCallbackContext(bot, chat, admin, restrictData) },
			run:    bansModule.restrictButtonHandler,
		},
		{
			name:   "restrict callback answer failure",
			method: "answerCallbackQuery",
			ctx:    func(bot *gotgbot.Bot) *ext.Context { return newModuleCallbackContext(bot, chat, admin, restrictData) },
			run:    bansModule.restrictButtonHandler,
		},
		{
			name:   "unrestrict callback get chat failure",
			method: "getChat",
			ctx:    func(bot *gotgbot.Bot) *ext.Context { return newModuleCallbackContext(bot, chat, admin, unrestrictData) },
			run:    bansModule.unrestrictButtonHandler,
		},
		{
			name:   "unrestrict callback unmute failure",
			method: "restrictChatMember",
			ctx:    func(bot *gotgbot.Bot) *ext.Context { return newModuleCallbackContext(bot, chat, admin, unrestrictData) },
			run:    bansModule.unrestrictButtonHandler,
		},
		{
			name:   "unrestrict callback unban failure",
			method: "unbanChatMember",
			ctx:    func(bot *gotgbot.Bot) *ext.Context { return newModuleCallbackContext(bot, chat, admin, unbanData) },
			run:    bansModule.unrestrictButtonHandler,
		},
		{
			name:   "unrestrict callback edit failure",
			method: "editMessageText",
			ctx:    func(bot *gotgbot.Bot) *ext.Context { return newModuleCallbackContext(bot, chat, admin, unbanData) },
			run:    bansModule.unrestrictButtonHandler,
		},
		{
			name:   "unrestrict callback answer failure",
			method: "answerCallbackQuery",
			ctx:    func(bot *gotgbot.Bot) *ext.Context { return newModuleCallbackContext(bot, chat, admin, unbanData) },
			run:    bansModule.unrestrictButtonHandler,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			client.errors[tt.method] = requestErr

			err := tt.run(bot, tt.ctx(bot))
			if !errors.Is(err, requestErr) {
				t.Fatalf("%s returned error %v, want request error", tt.name, err)
			}
		})
	}
}
