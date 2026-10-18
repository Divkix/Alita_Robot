package modules

import (
	"fmt"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/stretchr/testify/require"

	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/logchannels"
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
	f := newAdminChatFixture(t)
	bot := f.bot
	chatID := f.chat.Id
	channelID := uniqueModuleChatID()
	require.NoError(t, chats.EnsureChatInDb(chatID, f.chat.Title))
	require.NoError(t, chats.EnsureChatInDb(channelID, "log channel"))

	admin := f.admin
	channel := gotgbot.Chat{Id: channelID, Type: "channel", Title: "Log Channel"}
	group := f.chat

	if err := logChannelsModule.setLog(bot, newModuleMessageContext(bot, group, admin, "/setlog")); err != ext.EndGroups {
		t.Fatalf("setLog group: %v", err)
	}
	// A channel post has no user sender.
	setCtx := ext.NewContext(bot, &gotgbot.Update{UpdateId: 1, ChannelPost: &gotgbot.Message{
		MessageId:  101,
		Date:       1,
		Chat:       channel,
		SenderChat: &channel,
		Text:       "/setlog",
	}}, nil)
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
	f := newAdminChatFixture(t)
	bot := f.bot
	chatID := f.chat.Id
	channelID := uniqueModuleChatID()
	require.NoError(t, chats.EnsureChatInDb(chatID, f.chat.Title))
	require.NoError(t, chats.EnsureChatInDb(channelID, "log channel exact"))

	admin := f.admin
	channel := gotgbot.Chat{Id: channelID, Type: "channel", Title: "Log Channel"}
	group := f.chat

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
	nilBot := f.bot
	nilSet := newModuleMessageContext(nilBot, channel, admin, "/setlog")
	if err := logChannelsModule.setLog(nilBot, nilSet); err != ext.EndGroups {
		t.Fatalf("setLog with nil marshaler: %v", err)
	}
	nilFwd := newModuleMessageContext(nilBot, group, admin, "forwarded")
	nilFwd.EffectiveMessage.ForwardOrigin = gotgbot.MessageOriginChannel{
		Date:      1,
		Chat:      channel,
		MessageId: nilSet.EffectiveMessage.MessageId,
	}
	if err := logChannelsModule.captureSetLogForward(nilBot, nilFwd); err != ext.ContinueGroups {
		t.Fatalf("captureSetLogForward(nil marshaler): %v", err)
	}
	if got := logchannels.Get(chatID); got != nil {
		t.Fatal("nil-marshaler setLog bound a log channel")
	}
}

// /setlog posted in the channel and then forwarded to the group binds the
// channel through the dispatcher, as users run it.
func TestSetLogFlowBindsThroughDispatcher(t *testing.T) {
	useRealLocales(t)
	f := newAdminChatFixture(t)
	require.NoError(t, chats.EnsureChatInDb(f.chat.Id, f.chat.Title))
	channel := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "channel", Title: "Log Channel"}
	require.NoError(t, chats.EnsureChatInDb(channel.Id, channel.Title))
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadLogChannels(dispatcher)
	command := []gotgbot.MessageEntity{{Type: "bot_command", Offset: 0, Length: 7}}

	const postID = 4242
	post := &gotgbot.Update{UpdateId: 1, ChannelPost: &gotgbot.Message{
		MessageId:  postID,
		Date:       1,
		Chat:       channel,
		SenderChat: &channel,
		Text:       "/setlog",
		Entities:   command,
	}}
	require.NoError(t, dispatcher.ProcessUpdate(f.bot, post, nil))
	var repliedInChannel bool
	for _, call := range f.client.callsFor("sendMessage") {
		repliedInChannel = repliedInChannel || fmt.Sprint(call.Params["chat_id"]) == fmt.Sprint(channel.Id)
	}
	require.True(t, repliedInChannel, "/setlog in the channel got no reply there")

	forward := &gotgbot.Update{UpdateId: 2, Message: &gotgbot.Message{
		MessageId:     101,
		Date:          1,
		Chat:          f.chat,
		From:          &f.admin,
		Text:          "/setlog",
		Entities:      command,
		ForwardOrigin: gotgbot.MessageOriginChannel{Date: 1, Chat: channel, MessageId: postID},
	}}
	require.NoError(t, dispatcher.ProcessUpdate(f.bot, forward, nil))
	got := logchannels.Get(f.chat.Id)
	require.NotNil(t, got, "forwarded /setlog did not bind a log channel")
	require.Equal(t, channel.Id, got.LogChannelID)
}

// A /setlog posted in a channel linked to the group comes back as Telegram's
// automatic forward (sender 777000, which the admin checks count as an admin).
// That copy must not bind the channel: no group admin took part.
func TestSetLogAutomaticForwardFromLinkedChannelDoesNotBind(t *testing.T) {
	useRealLocales(t)
	f := newAdminChatFixture(t)
	require.NoError(t, chats.EnsureChatInDb(f.chat.Id, f.chat.Title))
	channel := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "channel", Title: "Linked Channel"}
	require.NoError(t, chats.EnsureChatInDb(channel.Id, channel.Title))
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadLogChannels(dispatcher)
	command := []gotgbot.MessageEntity{{Type: "bot_command", Offset: 0, Length: 7}}

	const postID = 5151
	post := &gotgbot.Update{UpdateId: 1, ChannelPost: &gotgbot.Message{
		MessageId:  postID,
		Date:       1,
		Chat:       channel,
		SenderChat: &channel,
		Text:       "/setlog",
		Entities:   command,
	}}
	require.NoError(t, dispatcher.ProcessUpdate(f.bot, post, nil))

	telegram := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	copyOf := func(updateID, messageID int64, automatic bool) *gotgbot.Update {
		return &gotgbot.Update{UpdateId: updateID, Message: &gotgbot.Message{
			MessageId:          messageID,
			Date:               1,
			Chat:               f.chat,
			From:               &telegram,
			SenderChat:         &channel,
			IsAutomaticForward: automatic,
			Text:               "/setlog",
			Entities:           command,
			ForwardOrigin:      gotgbot.MessageOriginChannel{Date: 1, Chat: channel, MessageId: postID},
		}}
	}
	require.NoError(t, dispatcher.ProcessUpdate(f.bot, copyOf(2, 201, true), nil))
	require.Nil(t, logchannels.Get(f.chat.Id), "automatic forward bound a log channel")
	require.NoError(t, dispatcher.ProcessUpdate(f.bot, copyOf(3, 202, false), nil))
	require.Nil(t, logchannels.Get(f.chat.Id), "a forward sent by 777000 bound a log channel")

	// The pending /setlog is still there for a group admin to forward.
	forward := &gotgbot.Update{UpdateId: 4, Message: &gotgbot.Message{
		MessageId:     203,
		Date:          1,
		Chat:          f.chat,
		From:          &f.admin,
		Text:          "/setlog",
		Entities:      command,
		ForwardOrigin: gotgbot.MessageOriginChannel{Date: 1, Chat: channel, MessageId: postID},
	}}
	require.NoError(t, dispatcher.ProcessUpdate(f.bot, forward, nil))
	got := logchannels.Get(f.chat.Id)
	require.NotNil(t, got, "admin forward after the automatic copy did not bind")
	require.Equal(t, channel.Id, got.LogChannelID)
}

// /unsetlog records the change in the log channel it just removed.
func TestUnsetLogLogsToTheRemovedChannel(t *testing.T) {
	f := newAdminChatFixture(t)
	channelID := uniqueModuleChatID()
	require.NoError(t, chats.EnsureChatInDb(f.chat.Id, f.chat.Title))
	require.NoError(t, chats.EnsureChatInDb(channelID, "old log channel"))
	require.NoError(t, logchannels.Set(f.chat.Id, f.chat.Title, channelID))
	settings := logchannels.Get(f.chat.Id)
	require.NotNil(t, settings)
	require.True(t, settings.CatSettings)

	if err := logChannelsModule.unsetLog(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, "/unsetlog")); err != ext.EndGroups {
		t.Fatalf("unsetLog: %v", err)
	}
	require.Nil(t, logchannels.Get(f.chat.Id))
	var logged bool
	for _, call := range f.client.callsFor("sendMessage") {
		logged = logged || fmt.Sprint(call.Params["chat_id"]) == fmt.Sprint(channelID)
	}
	require.True(t, logged, "/unsetlog sent nothing to the old log channel")
}
