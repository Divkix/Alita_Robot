package modules

import (
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
)

func resetAntiSpamMapForTest(t *testing.T) {
	t.Helper()
	for i := range antiSpamShards {
		antiSpamShards[i].mu.Lock()
		antiSpamShards[i].m = make(map[spamKey]*antiSpamInfo)
		antiSpamShards[i].mu.Unlock()
	}
	t.Cleanup(func() {
		for i := range antiSpamShards {
			antiSpamShards[i].mu.Lock()
			antiSpamShards[i].m = make(map[spamKey]*antiSpamInfo)
			antiSpamShards[i].mu.Unlock()
		}
	})
}

func setAntiSpamInfoForTest(key spamKey, info *antiSpamInfo) {
	shard := shardFor(key)
	shard.mu.Lock()
	shard.m[key] = info
	shard.mu.Unlock()
}

func getAntiSpamInfoForTest(key spamKey) (*antiSpamInfo, bool) {
	shard := shardFor(key)
	shard.mu.Lock()
	v, ok := shard.m[key]
	shard.mu.Unlock()
	return v, ok
}

func TestSpamCheckResetsExpiredWindow(t *testing.T) {
	resetAntiSpamMapForTest(t)

	key := spamKey{chatId: -1002, userId: 43}
	setAntiSpamInfoForTest(key, &antiSpamInfo{
		Count:       antiSpamLimit,
		WindowStart: time.Now().Add(-2 * antiSpamWindow),
	})

	if spamCheck(key) {
		t.Fatal("expired spam window should not stay spammed after reset")
	}

	if got, _ := getAntiSpamInfoForTest(key); got.Count != 1 {
		t.Fatalf("reset Count = %d, want 1 after current message", got.Count)
	}
}

func TestCleanupExpiredAntiSpamDeletesNilAndExpiredEntries(t *testing.T) {
	resetAntiSpamMapForTest(t)

	now := time.Now()
	nilKey := spamKey{chatId: -1003, userId: 44}
	expiredKey := spamKey{chatId: -1003, userId: 45}
	activeKey := spamKey{chatId: -1003, userId: 46}

	setAntiSpamInfoForTest(nilKey, nil)
	setAntiSpamInfoForTest(expiredKey, &antiSpamInfo{WindowStart: now.Add(-3 * antiSpamWindow)})
	setAntiSpamInfoForTest(activeKey, &antiSpamInfo{WindowStart: now.Add(-antiSpamWindow)})

	cleanupExpiredAntiSpam(now)

	if _, ok := getAntiSpamInfoForTest(nilKey); ok {
		t.Fatal("nil anti-spam entry was not deleted")
	}
	if _, ok := getAntiSpamInfoForTest(expiredKey); ok {
		t.Fatal("expired anti-spam entry was not deleted")
	}
	if _, ok := getAntiSpamInfoForTest(activeKey); !ok {
		t.Fatal("active anti-spam entry was deleted")
	}
}

func TestSpamCheckUsesDefaultThreshold(t *testing.T) {
	resetAntiSpamMapForTest(t)

	key := spamKey{chatId: -1004, userId: 47}
	for i := 0; i < 17; i++ {
		if spamCheck(key) {
			t.Fatalf("spamCheck() marked message %d as spam before default threshold", i+1)
		}
	}
	if !spamCheck(key) {
		t.Fatal("spamCheck() did not mark eighteenth message as spam")
	}
}

func TestLoadAntispamRegisteredHandlerAllowsDownstreamModeration(t *testing.T) {
	resetAntiSpamMapForTest(t)

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadAntispam(dispatcher)
	processed := 0
	dispatcher.AddHandler(handlers.NewMessage(message.All, func(*gotgbot.Bot, *ext.Context) error {
		processed++
		return ext.ContinueGroups
	}))
	bot := newModuleTestBot(newModuleBotClient())
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Spam Chat"}

	channelPost := &gotgbot.Update{
		UpdateId: 1,
		Message: &gotgbot.Message{
			MessageId: 1,
			Date:      1,
			Chat:      chat,
			Text:      "channel post",
		},
	}
	if err := dispatcher.ProcessUpdate(bot, channelPost, nil); err != nil {
		t.Fatalf("ProcessUpdate(channel post) error = %v", err)
	}
	processed = 0

	user := &gotgbot.User{Id: 42, FirstName: "Member"}
	for i := 0; i < 18; i++ {
		update := &gotgbot.Update{
			UpdateId: int64(i + 2),
			Message: &gotgbot.Message{
				MessageId: int64(i + 2),
				Date:      1,
				Chat:      chat,
				From:      user,
				Text:      "spam",
			},
		}
		if err := dispatcher.ProcessUpdate(bot, update, nil); err != nil {
			t.Fatalf("ProcessUpdate(user spam #%d) error = %v", i+1, err)
		}
	}

	key := spamKey{chatId: chat.Id, userId: user.Id}
	info, _ := getAntiSpamInfoForTest(key)
	if info == nil || info.Count != antiSpamLimit {
		t.Fatalf("antiSpam shard[%+v] = %#v, want count %d", key, info, antiSpamLimit)
	}
	if processed != antiSpamLimit {
		t.Fatalf("downstream messages = %d, want all %d messages", processed, antiSpamLimit)
	}
	if !spamCheck(key) {
		t.Fatal("antispam dispatcher handler did not mark user as spammed at threshold")
	}
}
