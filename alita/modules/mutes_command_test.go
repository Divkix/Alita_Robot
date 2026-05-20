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

func TestMuteCommandRejectsMissingChannelAndProtectedTargets(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Mute Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	tests := []struct {
		name string
		text string
	}{
		{name: "missing target", text: "/mute"},
		{name: "channel id", text: "/mute -1001234567890"},
		{name: "protected service admin", text: "/mute 777000"},
		{name: "bot itself", text: "/mute 999"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newModuleMessageContext(bot, chat, admin, tt.text)
			if err := mutesModule.mute(bot, ctx); err != ext.EndGroups {
				t.Fatalf("mute(%s) error = %v, want EndGroups", tt.name, err)
			}
		})
	}
	if calls := client.callsFor("restrictChatMember"); len(calls) != 0 {
		t.Fatalf("restrictChatMember calls = %d, want none for rejected targets", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != len(tests) {
		t.Fatalf("sendMessage calls = %d, want one denial per rejected target", len(calls))
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

func TestTemporaryMuteRejectsInvalidDurationAndProtectedTarget(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Mute Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	target := gotgbot.User{Id: 42, FirstName: "Member"}

	badDurationCtx := newMuteReplyContext(bot, chat, admin, target, "/tmute nope invalid")
	if err := mutesModule.tMute(bot, badDurationCtx); err != ext.EndGroups {
		t.Fatalf("tMute(invalid duration) error = %v, want EndGroups", err)
	}

	protectedCtx := newModuleMessageContext(bot, chat, admin, "/tmute 777000 1h")
	if err := mutesModule.tMute(bot, protectedCtx); err != ext.EndGroups {
		t.Fatalf("tMute(protected) error = %v, want EndGroups", err)
	}

	if calls := client.callsFor("restrictChatMember"); len(calls) != 0 {
		t.Fatalf("restrictChatMember calls = %d, want none for rejected temporary mutes", len(calls))
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

func TestSilentAndDeleteMuteRejectInvalidTargets(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Mute Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	for _, call := range []struct {
		name string
		text string
		run  func(*gotgbot.Bot, *ext.Context) error
	}{
		{name: "smute missing", text: "/smute", run: mutesModule.sMute},
		{name: "smute channel", text: "/smute -1001234567890", run: mutesModule.sMute},
		{name: "dmute missing", text: "/dmute", run: mutesModule.dMute},
		{name: "dmute channel", text: "/dmute -1001234567890", run: mutesModule.dMute},
	} {
		t.Run(call.name, func(t *testing.T) {
			ctx := newModuleMessageContext(bot, chat, admin, call.text)
			if err := call.run(bot, ctx); err != ext.EndGroups {
				t.Fatalf("%s error = %v, want EndGroups", call.name, err)
			}
		})
	}

	if calls := client.callsFor("restrictChatMember"); len(calls) != 0 {
		t.Fatalf("restrictChatMember calls = %d, want none for invalid target tests", len(calls))
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != 0 {
		t.Fatalf("deleteMessage calls = %d, want no deletion for invalid target tests", len(calls))
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

func TestDeleteMuteRejectsValidTargetWithoutReply(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Mute Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	ctx := newModuleMessageContext(bot, chat, admin, "/dmute 42")
	if err := mutesModule.dMute(bot, ctx); err != ext.EndGroups {
		t.Fatalf("dMute(valid target without reply) error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("restrictChatMember"); len(calls) != 0 {
		t.Fatalf("restrictChatMember calls = %d, want none without replied message", len(calls))
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != 0 {
		t.Fatalf("deleteMessage calls = %d, want no deletion without replied message", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want reply-required message", len(calls))
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

func TestUnmuteRejectsMissingChannelAndSelfTargets(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Mute Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	for _, tt := range []struct {
		name string
		text string
	}{
		{name: "missing target", text: "/unmute"},
		{name: "channel id", text: "/unmute -1001234567890"},
		{name: "bot itself", text: "/unmute 999"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newModuleMessageContext(bot, chat, admin, tt.text)
			if err := mutesModule.unmute(bot, ctx); err != ext.EndGroups {
				t.Fatalf("unmute(%s) error = %v, want EndGroups", tt.name, err)
			}
		})
	}

	if calls := client.callsFor("restrictChatMember"); len(calls) != 0 {
		t.Fatalf("restrictChatMember calls = %d, want none for rejected unmute targets", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 3 {
		t.Fatalf("sendMessage calls = %d, want one denial per rejected target", len(calls))
	}
}
