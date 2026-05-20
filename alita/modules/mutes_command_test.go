package modules

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func newMuteReplyContext(
	bot *gotgbot.Bot,
	chat gotgbot.Chat,
	admin gotgbot.User,
	target gotgbot.User,
	text string,
) *ext.Context {
	ctx := newModuleMessageContext(bot, chat, admin, text)
	ctx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
		MessageId: 404,
		Date:      1,
		Chat:      chat,
		From:      &target,
		Text:      "message being muted",
	}
	return ctx
}

func TestMuteReplyRestrictsUserAndSendsUnmuteButton(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Mute Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	target := gotgbot.User{Id: 42, FirstName: "Member"}

	ctx := newMuteReplyContext(bot, chat, admin, target, "/mute noisy")
	if err := mutesModule.mute(bot, ctx); err != ext.EndGroups {
		t.Fatalf("mute error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("restrictChatMember"); len(calls) != 1 {
		t.Fatalf("restrictChatMember calls = %d, want 1", len(calls))
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want mute confirmation", len(calls))
	}
	if calls[0].Params["reply_markup"] == nil {
		t.Fatal("mute confirmation did not include unmute button")
	}
}

func TestTemporaryMuteSetsUntilDate(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Mute Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	target := gotgbot.User{Id: 42, FirstName: "Member"}

	ctx := newMuteReplyContext(bot, chat, admin, target, "/tmute 1h temporary")
	if err := mutesModule.tMute(bot, ctx); err != ext.EndGroups {
		t.Fatalf("tMute error = %v, want EndGroups", err)
	}
	calls := client.callsFor("restrictChatMember")
	if len(calls) != 1 {
		t.Fatalf("restrictChatMember calls = %d, want 1", len(calls))
	}
	if calls[0].Params["until_date"] == nil {
		t.Fatalf("restrictChatMember params = %#v, want until_date", calls[0].Params)
	}
}

func TestSilentMuteDeletesCommandMessage(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Mute Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	target := gotgbot.User{Id: 42, FirstName: "Member"}

	ctx := newMuteReplyContext(bot, chat, admin, target, "/smute")
	if err := mutesModule.sMute(bot, ctx); err != ext.EndGroups {
		t.Fatalf("sMute error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("restrictChatMember"); len(calls) != 1 {
		t.Fatalf("restrictChatMember calls = %d, want 1", len(calls))
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != 1 {
		t.Fatalf("deleteMessage calls = %d, want command deletion", len(calls))
	}
}

func TestDeleteMuteDeletesReplyThenRestricts(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Mute Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	target := gotgbot.User{Id: 42, FirstName: "Member"}

	noReplyCtx := newModuleMessageContext(bot, chat, admin, "/dmute 42")
	if err := mutesModule.dMute(bot, noReplyCtx); err != ext.EndGroups {
		t.Fatalf("dMute no reply error = %v, want EndGroups", err)
	}

	ctx := newMuteReplyContext(bot, chat, admin, target, "/dmute delete this")
	if err := mutesModule.dMute(bot, ctx); err != ext.EndGroups {
		t.Fatalf("dMute reply error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != 1 {
		t.Fatalf("deleteMessage calls = %d, want replied message deletion", len(calls))
	}
	if calls := client.callsFor("restrictChatMember"); len(calls) != 1 {
		t.Fatalf("restrictChatMember calls = %d, want mute restriction", len(calls))
	}
}

func TestUnmuteCommandRestoresPermissions(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Mute Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	ctx := newModuleMessageContext(bot, chat, admin, "/unmute 42")
	if err := mutesModule.unmute(bot, ctx); err != ext.EndGroups {
		t.Fatalf("unmute error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("restrictChatMember"); len(calls) != 1 {
		t.Fatalf("restrictChatMember calls = %d, want unmute restriction", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want unmute confirmation", len(calls))
	}
}
