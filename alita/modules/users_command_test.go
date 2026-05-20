package modules

import (
	"sync"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
)

func TestShouldUpdateRateLimitsByID(t *testing.T) {
	cache := &sync.Map{}
	if !shouldUpdate(cache, 10, time.Hour) {
		t.Fatal("first update should be allowed")
	}
	if shouldUpdate(cache, 10, time.Hour) {
		t.Fatal("second update inside interval should be blocked")
	}
	cache.Store(int64(10), time.Now().Add(-2*time.Hour))
	if !shouldUpdate(cache, 10, time.Hour) {
		t.Fatal("update after interval should be allowed")
	}
}

func TestLogUsersPersistsSenderChatAndReplyUsers(t *testing.T) {
	userUpdateCache = &sync.Map{}
	chatUpdateCache = &sync.Map{}
	channelUpdateCache = &sync.Map{}

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Users Chat"}
	sender := gotgbot.User{Id: 4242, Username: "sender", FirstName: "Send", LastName: "Er"}
	replyUser := gotgbot.User{Id: 5252, Username: "reply", FirstName: "Re", LastName: "Ply"}
	ctx := newModuleMessageContext(bot, chat, sender, "hello")
	ctx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
		MessageId: 99,
		Date:      1,
		Chat:      chat,
		From:      &replyUser,
		Text:      "reply",
	}

	if err := usersModule.logUsers(bot, ctx); err != ext.ContinueGroups {
		t.Fatalf("logUsers error = %v, want ContinueGroups", err)
	}
	waitForModuleCondition(t, func() bool {
		_, _, senderFound := db.GetUserInfoById(sender.Id)
		_, _, replyFound := db.GetUserInfoById(replyUser.Id)
		return senderFound && replyFound && db.ChatExists(chat.Id)
	})
}

func TestLogUsersPersistsAnonymousChannelSender(t *testing.T) {
	userUpdateCache = &sync.Map{}
	chatUpdateCache = &sync.Map{}
	channelUpdateCache = &sync.Map{}

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Users Chat"}
	channel := gotgbot.Chat{
		Id:       -1009876543210,
		Type:     "channel",
		Title:    "News Channel",
		Username: "news_channel",
	}
	msg := &gotgbot.Message{
		MessageId: 101,
		Date:      1,
		Chat:      chat,
		SenderChat: &channel,
		Text:      "channel post",
	}
	ctx := ext.NewContext(bot, &gotgbot.Update{UpdateId: 4, Message: msg}, nil)

	if err := usersModule.logUsers(bot, ctx); err != ext.ContinueGroups {
		t.Fatalf("logUsers error = %v, want ContinueGroups", err)
	}
	waitForModuleCondition(t, func() bool {
		channelSettings := db.GetChannelSettings(channel.Id)
		return channelSettings != nil &&
			channelSettings.ChannelId == channel.Id &&
			channelSettings.ChannelName == channel.Title &&
			channelSettings.Username == channel.Username
	})
}
