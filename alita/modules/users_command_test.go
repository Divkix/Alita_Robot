package modules

import (
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	dbcache "github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/channels"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
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

func TestShouldTouchChatRateLimitsByChatAndBypassesOnRename(t *testing.T) {
	cache := &sync.Map{}
	const chatID = int64(-1001)

	if !shouldTouchChat(cache, chatID, "Chat", time.Hour) {
		t.Fatal("first touch should be allowed")
	}
	if shouldTouchChat(cache, chatID, "Chat", time.Hour) {
		t.Fatal("second touch of the same chat inside interval should be blocked")
	}
	if !shouldTouchChat(cache, chatID-1, "Chat", time.Hour) {
		t.Fatal("touch of a different chat should be allowed")
	}
	if !shouldTouchChat(cache, chatID, "Renamed Chat", time.Hour) {
		t.Fatal("chat title change inside interval should be allowed")
	}
	cache.Store(chatID, throttleEntry{at: time.Now().Add(-2 * time.Hour), state: [2]string{"Renamed Chat"}})
	if !shouldTouchChat(cache, chatID, "Renamed Chat", time.Hour) {
		t.Fatal("touch after interval should be allowed")
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
	oldChatMemberUpdateCache := chatMemberUpdateCache
	oldChannelUpdateCache := channelUpdateCache
	userUpdateCache = &sync.Map{}
	chatUpdateCache = &sync.Map{}
	chatMemberUpdateCache = &sync.Map{}
	channelUpdateCache = &sync.Map{}
	t.Cleanup(func() {
		userUpdateCache = oldUserUpdateCache
		chatUpdateCache = oldChatUpdateCache
		chatMemberUpdateCache = oldChatMemberUpdateCache
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

func TestLogUsersTouchesChatOncePerChatNotPerMember(t *testing.T) {
	isolateUsersThrottleCaches(t)

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Busy Chat"}
	first := gotgbot.User{Id: time.Now().UnixNano(), Username: "first", FirstName: "First"}
	second := gotgbot.User{Id: first.Id + 1, Username: "second", FirstName: "Second"}

	loadChat := func() models.Chat {
		t.Helper()
		var row models.Chat
		if err := db.DB.Where("chat_id = ?", chat.Id).First(&row).Error; err != nil {
			t.Fatalf("load chat: %v", err)
		}
		return row
	}

	if err := usersModule.logUsers(bot, newModuleMessageContext(bot, chat, first, "hello")); err != ext.ContinueGroups {
		t.Fatalf("logUsers error = %v, want ContinueGroups", err)
	}
	if got := loadChat().ChatName; got != chat.Title {
		t.Fatalf("chat name = %q, want %q", got, chat.Title)
	}

	// A stale last_activity would be refreshed by any touch that reaches the
	// database, so leaving it untouched proves the second member was throttled
	// by the per-chat key. SQLite compares these timestamps as text, so keep the
	// local representation the driver writes.
	stale := time.Now().Add(-2 * chatUpdateInterval).Truncate(time.Second)
	if err := db.DB.Model(&models.Chat{}).Where("chat_id = ?", chat.Id).
		Update("last_activity", stale).Error; err != nil {
		t.Fatalf("pin last_activity: %v", err)
	}
	if err := usersModule.logUsers(bot, newModuleMessageContext(bot, chat, second, "hi")); err != ext.ContinueGroups {
		t.Fatalf("logUsers error = %v, want ContinueGroups", err)
	}
	if got := loadChat().LastActivity; !got.Equal(stale) {
		t.Errorf("last_activity = %v, want %v left alone for another member of a freshly touched chat", got, stale)
	}
	// Membership appends use PostgreSQL JSONB operators.
	if db.DB.Name() == "postgres" {
		if users := loadChat().Users; !slices.Contains(users, first.Id) || !slices.Contains(users, second.Id) {
			t.Errorf("chat users = %v, want both %d and %d recorded per member", users, first.Id, second.Id)
		}
	}

	renamed := chat
	renamed.Title = "Busy Chat Renamed"
	if err := usersModule.logUsers(bot, newModuleMessageContext(bot, renamed, second, "renamed")); err != ext.ContinueGroups {
		t.Fatalf("logUsers error = %v, want ContinueGroups", err)
	}
	if got := loadChat().ChatName; got != renamed.Title {
		t.Errorf("chat name = %q, want rename %q to bypass the per-chat throttle", got, renamed.Title)
	}
}

// chatMemberIDs reads a chat's recorded members past the in-process cache.
func chatMemberIDs(chatID int64) []int64 {
	dbcache.ResetLocalForTest()
	return chats.GetChatSettings(chatID).Users
}

func TestLogUsersRecordsLinkedChannelAutoForwardAsChannel(t *testing.T) {
	isolateUsersThrottleCaches(t)

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Discussion Chat"}
	channel := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "channel", Title: "Linked Channel", Username: "linked_channel"}
	msg := &gotgbot.Message{
		MessageId:          102,
		Date:               1,
		Chat:               chat,
		From:               &gotgbot.User{Id: 777000, FirstName: "Telegram"},
		SenderChat:         &channel,
		IsAutomaticForward: true,
		Text:               "linked post",
	}
	ctx := ext.NewContext(bot, &gotgbot.Update{UpdateId: 5, Message: msg}, nil)

	if err := usersModule.logUsers(bot, ctx); err != ext.ContinueGroups {
		t.Fatalf("logUsers error = %v, want ContinueGroups", err)
	}
	waitForModuleCondition(t, func() bool {
		settings := channels.GetChannelSettings(channel.Id)
		return settings != nil && settings.ChannelId == channel.Id
	})
	DrainUsersAsyncWrites()
	if _, _, found := user.GetUserInfoById(channel.Id); found {
		t.Fatalf("linked channel %d was stored as a user", channel.Id)
	}
	if slices.Contains(chatMemberIDs(chat.Id), channel.Id) {
		t.Fatalf("linked channel %d was recorded as a member of chat %d", channel.Id, chat.Id)
	}
}

func TestLogUsersSkipsAnonymousAdminAsUser(t *testing.T) {
	isolateUsersThrottleCaches(t)

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Anon Admin Chat"}
	anonMsg := &gotgbot.Message{
		MessageId:  103,
		Date:       1,
		Chat:       chat,
		From:       &gotgbot.User{Id: 1087968824, IsBot: true, FirstName: "Group"},
		SenderChat: &chat,
		Text:       "anonymous admin post",
	}
	ctx := ext.NewContext(bot, &gotgbot.Update{UpdateId: 6, Message: anonMsg}, nil)

	if err := usersModule.logUsers(bot, ctx); err != ext.ContinueGroups {
		t.Fatalf("logUsers error = %v, want ContinueGroups", err)
	}
	waitForModuleCondition(t, func() bool { return db.ChatExists(chat.Id) })
	DrainUsersAsyncWrites()
	if _, _, found := user.GetUserInfoById(chat.Id); found {
		t.Fatalf("anonymous admin chat %d was stored as a user", chat.Id)
	}
	if slices.Contains(chatMemberIDs(chat.Id), chat.Id) {
		t.Fatalf("chat %d was recorded as its own member", chat.Id)
	}
}

func TestLogUsersRepliesToChatSenders(t *testing.T) {
	isolateUsersThrottleCaches(t)

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Reply Chat"}
	channel := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "channel", Title: "Replied Channel", Username: "replied_channel"}
	replier := gotgbot.User{Id: time.Now().UnixNano(), Username: "replier", FirstName: "Replier"}

	t.Run("reply to anonymous admin", func(t *testing.T) {
		ctx := newModuleMessageContext(bot, chat, replier, "answering the admin")
		ctx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
			MessageId:  104,
			Date:       1,
			Chat:       chat,
			From:       &gotgbot.User{Id: 1087968824, IsBot: true, FirstName: "Group"},
			SenderChat: &chat,
			Text:       "anonymous admin post",
		}
		if err := usersModule.logUsers(bot, ctx); err != ext.ContinueGroups {
			t.Fatalf("logUsers error = %v, want ContinueGroups", err)
		}
		waitForModuleCondition(t, func() bool {
			_, _, found := user.GetUserInfoById(replier.Id)
			return found
		})
		DrainUsersAsyncWrites()
		if _, _, found := user.GetUserInfoById(chat.Id); found {
			t.Fatalf("replied anonymous admin chat %d was stored as a user", chat.Id)
		}
	})

	t.Run("reply to automatic forward", func(t *testing.T) {
		ctx := newModuleMessageContext(bot, chat, replier, "commenting the post")
		ctx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
			MessageId:          105,
			Date:               1,
			Chat:               chat,
			From:               &gotgbot.User{Id: 777000, FirstName: "Telegram"},
			SenderChat:         &channel,
			IsAutomaticForward: true,
			Text:               "linked post",
		}
		if err := usersModule.logUsers(bot, ctx); err != ext.ContinueGroups {
			t.Fatalf("logUsers error = %v, want ContinueGroups", err)
		}
		waitForModuleCondition(t, func() bool {
			settings := channels.GetChannelSettings(channel.Id)
			return settings != nil && settings.ChannelId == channel.Id
		})
		DrainUsersAsyncWrites()
		if _, _, found := user.GetUserInfoById(channel.Id); found {
			t.Fatalf("replied linked channel %d was stored as a user", channel.Id)
		}
	})
}
