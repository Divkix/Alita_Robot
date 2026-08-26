package modules

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/stretchr/testify/require"

	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/logchannels"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

func TestLogChannelSetAndUnset(t *testing.T) {
	chatID := uniqueModuleChatID()
	channelID := uniqueModuleChatID()
	require.NoError(t, chats.EnsureChatInDb(chatID, "logged"))
	require.NoError(t, chats.EnsureChatInDb(channelID, "logchan"))

	require.NoError(t, logchannels.Set(chatID, "logged", channelID))
	got := logchannels.Get(chatID)
	require.NotNil(t, got)
	require.Equal(t, channelID, got.LogChannelID)
	require.True(t, got.CatAdmin)

	require.NoError(t, logchannels.SetCategory(chatID, "admin", false))
	got = logchannels.Get(chatID)
	require.False(t, got.CatAdmin)

	require.NoError(t, logchannels.Unset(chatID))
	require.Nil(t, logchannels.Get(chatID))
}

func TestLogChannelUnknownCategory(t *testing.T) {
	chatID := uniqueModuleChatID()
	channelID := uniqueModuleChatID()
	require.NoError(t, chats.EnsureChatInDb(chatID, "logged2"))
	require.NoError(t, chats.EnsureChatInDb(channelID, "logchan2"))
	require.NoError(t, logchannels.Set(chatID, "logged2", channelID))
	err := logchannels.SetCategory(chatID, "not-a-cat", false)
	require.Error(t, err)
}

func TestLogChannelModelTableName(t *testing.T) {
	require.Equal(t, "log_channels", models.LogChannel{}.TableName())
}

func TestLogChannelCommandReportsUnset(t *testing.T) {
	bot := newModuleTestBot(newModuleBotClient())
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Logged Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	ctx := newModuleMessageContext(bot, chat, admin, "/logchannel")
	if err := logChannelsModule.logChannel(bot, ctx); err != ext.EndGroups {
		t.Fatalf("logChannel() error = %v, want EndGroups", err)
	}
}

func TestNologTogglesCategory(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chatID := uniqueModuleChatID()
	channelID := uniqueModuleChatID()
	require.NoError(t, chats.EnsureChatInDb(chatID, "toggle logs"))
	require.NoError(t, logchannels.Set(chatID, "toggle logs", channelID))

	chat := gotgbot.Chat{Id: chatID, Type: "supergroup", Title: "Logged Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	ctx := newModuleMessageContext(bot, chat, admin, "/nolog admin")
	if err := logChannelsModule.disableLog(bot, ctx); err != ext.EndGroups {
		t.Fatalf("disableLog() error = %v, want EndGroups", err)
	}
	got := logchannels.Get(chatID)
	require.NotNil(t, got)
	require.False(t, got.CatAdmin)
}

func TestLogChannelCommandsAndForwardBind(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chatID := uniqueModuleChatID()
	channelID := uniqueModuleChatID()
	require.NoError(t, chats.EnsureChatInDb(chatID, "logged group"))
	require.NoError(t, chats.EnsureChatInDb(channelID, "log channel"))

	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	channel := gotgbot.Chat{Id: channelID, Type: "channel", Title: "Log Channel"}
	group := gotgbot.Chat{Id: chatID, Type: "supergroup", Title: "Logged Group"}

	if err := logChannelsModule.setLog(bot, newModuleMessageContext(bot, group, admin, "/setlog")); err != ext.EndGroups {
		t.Fatalf("setLog group: %v", err)
	}
	setCtx := newModuleMessageContext(bot, channel, admin, "/setlog")
	if err := logChannelsModule.setLog(bot, setCtx); err != ext.EndGroups {
		t.Fatalf("setLog channel: %v", err)
	}

	fwd := newModuleMessageContext(bot, group, admin, "forwarded")
	fwd.EffectiveMessage.ForwardOrigin = gotgbot.MessageOriginChannel{
		Date:      1,
		Chat:      channel,
		MessageId: setCtx.EffectiveMessage.MessageId,
	}
	if err := logChannelsModule.captureSetLogForward(bot, fwd); err != ext.ContinueGroups {
		t.Fatalf("captureSetLogForward: %v", err)
	}
	got := logchannels.Get(chatID)
	require.NotNil(t, got)
	require.Equal(t, channelID, got.LogChannelID)

	if err := logChannelsModule.logChannel(bot, newModuleMessageContext(bot, group, admin, "/logchannel")); err != ext.EndGroups {
		t.Fatalf("logChannel: %v", err)
	}
	if err := logChannelsModule.logCategories(bot, newModuleMessageContext(bot, group, admin, "/logcategories")); err != ext.EndGroups {
		t.Fatalf("logCategories: %v", err)
	}
	if err := logChannelsModule.enableLog(bot, newModuleMessageContext(bot, group, admin, "/log reports")); err != ext.EndGroups {
		t.Fatalf("enableLog: %v", err)
	}
	if err := logChannelsModule.disableLog(bot, newModuleMessageContext(bot, group, admin, "/nolog nope")); err != ext.EndGroups {
		t.Fatalf("disableLog unknown: %v", err)
	}
	if err := logChannelsModule.unsetLog(bot, newModuleMessageContext(bot, group, admin, "/unsetlog")); err != ext.EndGroups {
		t.Fatalf("unsetLog: %v", err)
	}
	require.Nil(t, logchannels.Get(chatID))
}

func TestSetLogForwardRequiresExactMessageAndFailsClosedWithoutCache(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chatID := uniqueModuleChatID()
	channelID := uniqueModuleChatID()
	require.NoError(t, chats.EnsureChatInDb(chatID, "logged group exact"))
	require.NoError(t, chats.EnsureChatInDb(channelID, "log channel exact"))

	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	channel := gotgbot.Chat{Id: channelID, Type: "channel", Title: "Log Channel"}
	group := gotgbot.Chat{Id: chatID, Type: "supergroup", Title: "Logged Group"}

	setCtx := newModuleMessageContext(bot, channel, admin, "/setlog")
	if err := logChannelsModule.setLog(bot, setCtx); err != ext.EndGroups {
		t.Fatalf("setLog channel: %v", err)
	}

	wrongFwd := newModuleMessageContext(bot, group, admin, "unrelated forward")
	wrongFwd.EffectiveMessage.ForwardOrigin = gotgbot.MessageOriginChannel{
		Date:      1,
		Chat:      channel,
		MessageId: setCtx.EffectiveMessage.MessageId + 99,
	}
	if err := logChannelsModule.captureSetLogForward(bot, wrongFwd); err != ext.ContinueGroups {
		t.Fatalf("captureSetLogForward(wrong message): %v", err)
	}
	require.Nil(t, logchannels.Get(chatID), "non-/setlog forward must not bind a log channel")

	withNilCacheMarshal(t)
	nilClient := newModuleBotClient()
	nilBot := newModuleTestBot(nilClient)
	nilSet := newModuleMessageContext(nilBot, channel, admin, "/setlog")
	if err := logChannelsModule.setLog(nilBot, nilSet); err != ext.EndGroups {
		t.Fatalf("setLog with nil marshaler: %v", err)
	}
	calls := nilClient.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want failure reply", len(calls))
	}
	text, _ := calls[0].Params["text"].(string)
	if text == "" {
		t.Fatal("nil-marshaler setLog replied with empty text")
	}
	if got := logchannels.Get(chatID); got != nil {
		t.Fatal("nil-marshaler setLog bound a log channel")
	}
}
