package modules

import (
	"sync"
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db/channels"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/user"
)

func TestAsyncUserUpdateWrappersPersistRecords(t *testing.T) {
	userID := time.Now().UnixNano()
	chatID := uniqueModuleChatID()
	channelID := -1000000000000 - userID%1000000

	asyncUpdateUser(userID, "user_name", "User Name")
	waitForModuleCondition(t, func() bool {
		username, name, found := user.GetUserInfoById(userID)
		return found && username == "user_name" && name == "User Name"
	})

	asyncUpdateChat(chatID, "Users Chat", userID)
	waitForModuleCondition(t, func() bool {
		chat := chats.GetChatSettings(chatID)
		return chat.ChatId == chatID && chat.ChatName == "Users Chat"
	})

	asyncUpdateChannel(channelID, "Updates", "updates")
	waitForModuleCondition(t, func() bool {
		channelUsername, channelName, found := channels.GetChannelInfoById(channelID)
		return found && channelUsername == "updates" && channelName == "Updates"
	})
}

func TestShouldUpdateKeyStillThrottlesWithoutTimers(t *testing.T) {
	m := &sync.Map{}
	key := int64(42)
	interval := time.Minute

	if !shouldUpdateKey(m, key, interval) {
		t.Fatal("first call should allow an update")
	}
	if shouldUpdateKey(m, key, interval) {
		t.Fatal("immediate second call should be throttled")
	}

	m.Store(key, time.Now().Add(-2*interval))
	if !shouldUpdateKey(m, key, interval) {
		t.Fatal("call after interval elapsed should allow an update")
	}
}

func TestSweepUpdateCacheDropsOnlyExpiredEntries(t *testing.T) {
	m := &sync.Map{}
	now := time.Now()
	m.Store(1, now.Add(-10*time.Minute))
	m.Store(2, now)

	sweepUpdateCache(m, now, 5*time.Minute)

	if _, ok := m.Load(1); ok {
		t.Fatal("expired entry should have been swept")
	}
	if _, ok := m.Load(2); !ok {
		t.Fatal("fresh entry should have been kept")
	}
}
