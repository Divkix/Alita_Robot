package modules

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/db/antiflood"
	"github.com/divkix/Alita_Robot/alita/db/logchannels"
)

const automatedLogChannel = int64(-1009876543211)

// automatedLogCase is one action the bot takes on its own. run sets it up in
// chat and fires it; failMethod is the Bot API call whose failure means the
// action did not happen.
type automatedLogCase struct {
	name       string
	tag        string
	userID     int64
	want       []string
	failMethod string
	run        func(t *testing.T, client *moduleBotClient, bot *gotgbot.Bot, chat gotgbot.Chat)
}

// checkAutomatedLog asserts the action logs one entry under the automated
// category, none with the category off, and none when Telegram refuses it.
func checkAutomatedLog(t *testing.T, tc automatedLogCase) {
	t.Helper()
	newChat := func(t *testing.T) gotgbot.Chat {
		t.Helper()
		chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Automated Log Chat"}
		if err := logchannels.Set(chat.Id, chat.Title, automatedLogChannel); err != nil {
			t.Fatalf("logchannels.Set() error = %v", err)
		}
		return chat
	}

	t.Run(tc.name+"/logged", func(t *testing.T) {
		client := newModuleBotClient()
		bot := newModuleTestBot(client)
		chat := newChat(t)
		tc.run(t, client, bot, chat)

		entries := spamDBMessagesTo(client, automatedLogChannel)
		if len(entries) != 1 {
			t.Fatalf("log channel entries = %d, want 1: %v", len(entries), entries)
		}
		for _, want := range append([]string{"#" + tc.tag, fmt.Sprint(tc.userID)}, tc.want...) {
			if !strings.Contains(entries[0], want) {
				t.Fatalf("log entry %q does not contain %q", entries[0], want)
			}
		}
	})

	t.Run(tc.name+"/category off", func(t *testing.T) {
		client := newModuleBotClient()
		bot := newModuleTestBot(client)
		chat := newChat(t)
		if err := logchannels.SetCategory(chat.Id, logchannels.CategoryAutomated, false); err != nil {
			t.Fatalf("SetCategory() error = %v", err)
		}
		tc.run(t, client, bot, chat)

		if entries := spamDBMessagesTo(client, automatedLogChannel); len(entries) != 0 {
			t.Fatalf("log channel entries = %d, want none: %v", len(entries), entries)
		}
	})

	t.Run(tc.name+"/action failed", func(t *testing.T) {
		client := newModuleBotClient()
		client.errors[tc.failMethod] = errors.New("Bad Request: not enough rights")
		bot := newModuleTestBot(client)
		chat := newChat(t)
		tc.run(t, client, bot, chat)

		if entries := spamDBMessagesTo(client, automatedLogChannel); len(entries) != 0 {
			t.Fatalf("log channel entries = %d, want none: %v", len(entries), entries)
		}
	})
}

func TestAntifloodLogsItsActionToTheLogChannel(t *testing.T) {
	for _, tt := range []struct {
		action     string
		failMethod string
		want       []string
	}{
		{action: "mute", failMethod: "restrictChatMember"},
		{action: "kick", failMethod: "unbanChatMember"},
		{action: "ban", failMethod: "banChatMember"},
		{action: "tban", failMethod: "banChatMember", want: []string{"1h"}},
		{action: "tmute", failMethod: "restrictChatMember", want: []string{"1h"}},
	} {
		checkAutomatedLog(t, automatedLogCase{
			name:       tt.action,
			tag:        "ANTIFLOOD",
			userID:     61,
			want:       append([]string{tt.action}, tt.want...),
			failMethod: tt.failMethod,
			run: func(t *testing.T, _ *moduleBotClient, bot *gotgbot.Bot, chat gotgbot.Chat) {
				resetAntifloodState(t)
				if err := antiflood.SetFlood(chat.Id, 1); err != nil {
					t.Fatalf("SetFlood() error = %v", err)
				}
				if err := antiflood.SetFloodMode(chat.Id, tt.action); err != nil {
					t.Fatalf("SetFloodMode() error = %v", err)
				}
				member := gotgbot.User{Id: 61, FirstName: "Flooder"}
				_ = antifloodModule.checkFlood(bot, newModuleMessageContext(bot, chat, member, "one"))
				second := newModuleMessageContext(bot, chat, member, "two")
				second.EffectiveMessage.MessageId = 202
				_ = antifloodModule.checkFlood(bot, second)
			},
		})
	}
}
