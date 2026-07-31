package modules

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/channels"
	"github.com/divkix/Alita_Robot/alita/db/user"
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

func TestShouldUpdateRateLimitsChatMembershipByChatAndUser(t *testing.T) {
	cache := &sync.Map{}
	const chatID = int64(-1001)

	if !shouldUpdateChatMember(cache, chatID, 10, time.Hour) {
		t.Fatal("first user update should be allowed")
	}
	if !shouldUpdateChatMember(cache, chatID, 11, time.Hour) {
		t.Fatal("different user in the same chat should be allowed")
	}
	if shouldUpdateChatMember(cache, chatID, 10, time.Hour) {
		t.Fatal("same chat/user update inside interval should be blocked")
	}
}

func TestShouldUpdateAllowsOneConcurrentCaller(t *testing.T) {
	cache := &sync.Map{}
	start := make(chan struct{})
	var allowed atomic.Int32
	var workers sync.WaitGroup

	for range 32 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			if shouldUpdate(cache, 10, time.Hour) {
				allowed.Add(1)
			}
		}()
	}
	close(start)
	workers.Wait()

	if got := allowed.Load(); got != 1 {
		t.Fatalf("concurrent allowed calls = %d, want 1", got)
	}
}

func TestShouldUpdateExpiresInactiveKeys(t *testing.T) {
	cache := &sync.Map{}
	const key = int64(10)

	if !shouldUpdate(cache, key, time.Millisecond) {
		t.Fatal("first update should be allowed")
	}
	waitForModuleCondition(t, func() bool {
		_, loaded := cache.Load(key)
		return !loaded
	})
}

func TestLogUsersPersistsSenderChatAndReplyUsers(t *testing.T) {
	oldUserUpdateCache := userUpdateCache
	oldChatUpdateCache := chatUpdateCache
	oldChannelUpdateCache := channelUpdateCache
	userUpdateCache = &sync.Map{}
	chatUpdateCache = &sync.Map{}
	channelUpdateCache = &sync.Map{}
	t.Cleanup(func() {
		userUpdateCache = oldUserUpdateCache
		chatUpdateCache = oldChatUpdateCache
		channelUpdateCache = oldChannelUpdateCache
	})

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
		_, _, senderFound := user.GetUserInfoById(sender.Id)
		_, _, replyFound := user.GetUserInfoById(replyUser.Id)
		return senderFound && replyFound && db.ChatExists(chat.Id)
	})
}

func TestLogUsersPersistsAnonymousChannelSender(t *testing.T) {
	oldUserUpdateCache := userUpdateCache
	oldChatUpdateCache := chatUpdateCache
	oldChannelUpdateCache := channelUpdateCache
	userUpdateCache = &sync.Map{}
	chatUpdateCache = &sync.Map{}
	channelUpdateCache = &sync.Map{}
	t.Cleanup(func() {
		userUpdateCache = oldUserUpdateCache
		chatUpdateCache = oldChatUpdateCache
		channelUpdateCache = oldChannelUpdateCache
	})

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
		MessageId:  101,
		Date:       1,
		Chat:       chat,
		SenderChat: &channel,
		Text:       "channel post",
	}
	ctx := ext.NewContext(bot, &gotgbot.Update{UpdateId: 4, Message: msg}, nil)

	if err := usersModule.logUsers(bot, ctx); err != ext.ContinueGroups {
		t.Fatalf("logUsers error = %v, want ContinueGroups", err)
	}
	waitForModuleCondition(t, func() bool {
		channelSettings := channels.GetChannelSettings(channel.Id)
		return channelSettings != nil &&
			channelSettings.ChannelId == channel.Id &&
			channelSettings.ChannelName == channel.Title &&
			channelSettings.Username == channel.Username
	})
}
