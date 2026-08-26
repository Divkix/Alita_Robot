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
