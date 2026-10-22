//go:build testtools

package modules

import (
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/chatstats"
	"github.com/divkix/Alita_Robot/alita/utils/stats"
)

func TestUsersWatcherCountsMemberMessagesForRealUsersOnly(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Member Counting Chat"}
	member := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Member"}

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadUsers(dispatcher)
	t.Cleanup(DrainUsersAsyncWrites)

	aispamProcessUpdate(t, dispatcher, bot, chat, member, 930, "hello")
	aispamProcessUpdate(t, dispatcher, bot, chat, member, 931, "again")
	dispatchDigestUpdate(t, dispatcher, bot, &gotgbot.Update{
		UpdateId: 932,
		EditedMessage: &gotgbot.Message{
			MessageId: 930, Date: 1, Chat: chat, From: &member, Text: "hello, edited",
		},
	})
	dispatchDigestUpdate(t, dispatcher, bot, &gotgbot.Update{
		UpdateId: 933,
		Message: &gotgbot.Message{
			MessageId: 933, Date: 1, Chat: chat, Text: "from an anonymous admin",
			From:       &gotgbot.User{Id: 1087968824, FirstName: "Group", IsBot: true},
			SenderChat: &chat,
		},
	})
	dispatchDigestUpdate(t, dispatcher, bot, &gotgbot.Update{
		UpdateId: 934,
		Message: &gotgbot.Message{
			MessageId: 934, Date: 1, Chat: chat, Text: "from a channel",
			SenderChat: &gotgbot.Chat{Id: uniqueModuleChatID(), Type: "channel", Title: "Linked"},
		},
	})

	if err := stats.Flush(); err != nil {
		t.Fatalf("stats.Flush() error = %v", err)
	}
	now := time.Now().UTC()
	top, err := chatstats.TopMembers(chat.Id, stats.Day(now.AddDate(0, 0, -1)), stats.Day(now), 10)
	if err != nil {
		t.Fatalf("TopMembers() error = %v", err)
	}
	if len(top) != 1 || top[0].UserID != member.Id || top[0].Messages != 2 {
		t.Fatalf("top members = %+v, want only the member with 2 messages", top)
	}
}
