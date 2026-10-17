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
	if !shouldUpdateUser(cache, 10, "user", "User", time.Hour) {
		t.Fatal("first update should be allowed")
	}
	if shouldUpdateUser(cache, 10, "user", "User", time.Hour) {
		t.Fatal("second update inside interval should be blocked")
	}
	cache.Store(int64(10), throttleEntry{at: time.Now().Add(-2 * time.Hour), state: [2]string{"user", "User"}})
	if !shouldUpdateUser(cache, 10, "user", "User", time.Hour) {
		t.Fatal("update after interval should be allowed")
	}
}

func TestShouldUpdateUserBypassesThrottleOnRename(t *testing.T) {
	cache := &sync.Map{}
	if !shouldUpdateUser(cache, 10, "user", "User", time.Hour) {
		t.Fatal("first update should be allowed")
	}
	if !shouldUpdateUser(cache, 10, "renamed", "User", time.Hour) {
		t.Fatal("username change inside interval should be allowed")
	}
	if !shouldUpdateUser(cache, 10, "renamed", "Renamed", time.Hour) {
		t.Fatal("display name change inside interval should be allowed")
	}
	if shouldUpdateUser(cache, 10, "renamed", "Renamed", time.Hour) {
		t.Fatal("unchanged user inside interval should be blocked")
	}
}

func TestShouldUpdateChannelBypassesThrottleOnRename(t *testing.T) {
	cache := &sync.Map{}
	const channelID = int64(-1009876543210)
	if !shouldUpdateChannel(cache, channelID, "News", "news", time.Hour) {
		t.Fatal("first update should be allowed")
	}
	if shouldUpdateChannel(cache, channelID, "News", "news", time.Hour) {
		t.Fatal("unchanged channel inside interval should be blocked")
	}
	if !shouldUpdateChannel(cache, channelID, "Daily News", "news", time.Hour) {
		t.Fatal("channel title change inside interval should be allowed")
	}
	if !shouldUpdateChannel(cache, channelID, "Daily News", "daily_news", time.Hour) {
		t.Fatal("channel username change inside interval should be allowed")
	}
}

func TestShouldUpdateRateLimitsChatMembershipByChatAndUser(t *testing.T) {
	cache := &sync.Map{}
	const chatID = int64(-1001)

	if !shouldUpdateChatMember(cache, chatID, "Chat", 10, time.Hour) {
		t.Fatal("first user update should be allowed")
	}
	if !shouldUpdateChatMember(cache, chatID, "Chat", 11, time.Hour) {
		t.Fatal("different user in the same chat should be allowed")
	}
	if shouldUpdateChatMember(cache, chatID, "Chat", 10, time.Hour) {
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
			if shouldUpdateUser(cache, 10, "user", "User", time.Hour) {
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

	if !shouldUpdateUser(cache, key, "user", "User", time.Minute) {
		t.Fatal("first update should be allowed")
	}
	sweepUpdateCache(cache, time.Now(), time.Minute)
	if _, loaded := cache.Load(key); !loaded {
		t.Fatal("active key should survive the sweep")
	}
	sweepUpdateCache(cache, time.Now().Add(2*time.Minute), time.Minute)
	if _, loaded := cache.Load(key); loaded {
		t.Fatal("inactive key should be swept")
	}
}

// isolateUsersThrottleCaches gives the test empty throttle caches so entries
// left by other tests cannot suppress the writes it asserts on.
func isolateUsersThrottleCaches(t *testing.T) {
	t.Helper()
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
}

func TestLogUsersPersistsSenderChatAndReplyUsers(t *testing.T) {
	isolateUsersThrottleCaches(t)

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
	isolateUsersThrottleCaches(t)

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

func TestLogUsersPersistsRenameInsideThrottleWindow(t *testing.T) {
	isolateUsersThrottleCaches(t)

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Users Chat"}
	sender := gotgbot.User{Id: time.Now().UnixNano(), Username: "before", FirstName: "Before"}

	if err := usersModule.logUsers(bot, newModuleMessageContext(bot, chat, sender, "hello")); err != ext.ContinueGroups {
		t.Fatalf("logUsers error = %v, want ContinueGroups", err)
	}
	waitForModuleCondition(t, func() bool {
		username, name, found := user.GetUserInfoById(sender.Id)
		return found && username == "before" && name == "Before"
	})

	renamed := sender
	renamed.Username = "after"
	renamed.FirstName = "After"
	if err := usersModule.logUsers(bot, newModuleMessageContext(bot, chat, renamed, "hello again")); err != ext.ContinueGroups {
		t.Fatalf("logUsers error = %v, want ContinueGroups", err)
	}
	waitForModuleCondition(t, func() bool {
		username, name, found := user.GetUserInfoById(sender.Id)
		return found && username == "after" && name == "After"
	})
}
