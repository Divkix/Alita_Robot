package modules

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/antiflood"
	"github.com/divkix/Alita_Robot/alita/db/antiraid"
	"github.com/divkix/Alita_Robot/alita/db/blacklists"
	"github.com/divkix/Alita_Robot/alita/db/captcha"
	"github.com/divkix/Alita_Robot/alita/db/locks"
	"github.com/divkix/Alita_Robot/alita/db/logchannels"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/warns"
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

func TestBlacklistLogsItsActionToTheLogChannel(t *testing.T) {
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
			tag:        "BLACKLIST",
			userID:     62,
			want:       append([]string{tt.action, "spam"}, tt.want...),
			failMethod: tt.failMethod,
			run: func(t *testing.T, _ *moduleBotClient, bot *gotgbot.Bot, chat gotgbot.Chat) {
				if err := blacklists.AddBlacklist(chat.Id, "spam"); err != nil {
					t.Fatalf("AddBlacklist() error = %v", err)
				}
				if err := blacklists.SetBlacklistAction(chat.Id, tt.action); err != nil {
					t.Fatalf("SetBlacklistAction() error = %v", err)
				}
				member := gotgbot.User{Id: 62, FirstName: "Spammer"}
				_ = blacklistsModule.blacklistWatcher(bot, newModuleMessageContext(bot, chat, member, "this has spam inside"))
			},
		})
	}
}

func TestBlacklistWarnAtTheLimitLogsTheWarnAction(t *testing.T) {
	checkAutomatedLog(t, automatedLogCase{
		name:       "ban",
		tag:        "WARNLIMIT",
		userID:     63,
		want:       []string{"ban", "1/1", "blacklist"},
		failMethod: "banChatMember",
		run: func(t *testing.T, client *moduleBotClient, bot *gotgbot.Bot, chat gotgbot.Chat) {
			client.membersByUser = map[string]json.RawMessage{
				"63": json.RawMessage(`{"status":"member","user":{"id":63,"is_bot":false,"first_name":"Spammer"}}`),
			}
			if err := blacklists.AddBlacklist(chat.Id, "spam"); err != nil {
				t.Fatalf("AddBlacklist() error = %v", err)
			}
			if err := blacklists.SetBlacklistAction(chat.Id, "warn"); err != nil {
				t.Fatalf("SetBlacklistAction() error = %v", err)
			}
			if err := warns.SetWarnMode(chat.Id, "ban", 0); err != nil {
				t.Fatalf("SetWarnMode() error = %v", err)
			}
			if err := warns.SetWarnLimit(chat.Id, 1); err != nil {
				t.Fatalf("SetWarnLimit() error = %v", err)
			}
			member := gotgbot.User{Id: 63, FirstName: "Spammer"}
			_ = blacklistsModule.blacklistWatcher(bot, newModuleMessageContext(bot, chat, member, "this has spam inside"))
		},
	})
}

// An admin's warn at the limit is a human action: it stays out of the
// automated category.
func TestAdminWarnAtTheLimitLogsNothingAutomated(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Automated Log Chat"}
	if err := logchannels.Set(chat.Id, chat.Title, automatedLogChannel); err != nil {
		t.Fatalf("logchannels.Set() error = %v", err)
	}
	if err := warns.SetWarnMode(chat.Id, "ban", 0); err != nil {
		t.Fatalf("SetWarnMode() error = %v", err)
	}
	if err := warns.SetWarnLimit(chat.Id, 1); err != nil {
		t.Fatalf("SetWarnLimit() error = %v", err)
	}
	admin := gotgbot.User{Id: 100, FirstName: "Admin"}
	ctx := newModuleMessageContext(bot, chat, admin, "/warn")
	_ = warnsModule.warnThisUser(bot, ctx, 64, "", "warn", models.WarnEntry{WarnedBy: admin.Id})

	if calls := client.callsFor("banChatMember"); len(calls) != 1 {
		t.Fatalf("banChatMember calls = %d, want the warn-limit ban", len(calls))
	}
	if entries := spamDBMessagesTo(client, automatedLogChannel); len(entries) != 0 {
		t.Fatalf("log channel entries = %d, want none: %v", len(entries), entries)
	}
}

func TestCaptchaFailureLogsToTheLogChannel(t *testing.T) {
	checkAutomatedLog(t, automatedLogCase{
		name:       "wrong answers",
		tag:        "CAPTCHA_FAILED",
		userID:     42,
		want:       []string{"ban", "wrong answers"},
		failMethod: "banChatMember",
		run: func(t *testing.T, _ *moduleBotClient, bot *gotgbot.Bot, chat gotgbot.Chat) {
			if err := captcha.SetCaptchaFailureAction(chat.Id, "ban"); err != nil {
				t.Fatalf("SetCaptchaFailureAction() error = %v", err)
			}
			if err := captcha.SetCaptchaMaxAttempts(chat.Id, 1); err != nil {
				t.Fatalf("SetCaptchaMaxAttempts() error = %v", err)
			}
			attempt, err := captcha.CreateCaptchaAttemptPreMessage(42, chat.Id, "7", 2)
			if err != nil {
				t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
			}
			member := gotgbot.User{Id: 42, FirstName: "Member"}
			ctx := newModuleCallbackContext(bot, chat, member, encodeCallbackData("captcha_verify", map[string]string{"a": fmt.Sprint(attempt.ID), "u": "42", "s": "8"}))
			_ = captchaModule.captchaVerifyCallback(bot, ctx)
		},
	})

	checkAutomatedLog(t, automatedLogCase{
		name:       "timeout",
		tag:        "CAPTCHA_FAILED",
		userID:     42,
		want:       []string{"kick", "timeout", "Automated Log Chat"},
		failMethod: "unbanChatMember",
		run: func(t *testing.T, _ *moduleBotClient, bot *gotgbot.Bot, chat gotgbot.Chat) {
			if err := captcha.SetCaptchaEnabled(chat.Id, true); err != nil {
				t.Fatalf("SetCaptchaEnabled() error = %v", err)
			}
			if err := captcha.SetCaptchaFailureAction(chat.Id, "kick"); err != nil {
				t.Fatalf("SetCaptchaFailureAction() error = %v", err)
			}
			attempt, err := captcha.CreateCaptchaAttemptPreMessage(42, chat.Id, "7", 2)
			if err != nil {
				t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
			}
			_, _ = expireCaptchaAttempt(bot, attempt)
		},
	})
}

// raidJoin posts one service message with members joining chat.
func raidJoin(t *testing.T, bot *gotgbot.Bot, chat gotgbot.Chat, members ...gotgbot.User) {
	t.Helper()
	msg := &gotgbot.Message{MessageId: 501, Date: 1, Chat: chat, From: &members[0], NewChatMembers: members}
	if err := antiRaidModule.onJoin(bot, ext.NewContext(bot, &gotgbot.Update{UpdateId: 501, Message: msg}, nil)); err != ext.ContinueGroups {
		t.Fatalf("onJoin() error = %v, want ContinueGroups", err)
	}
}

func TestAntiRaidAutoTriggerLogsOneEntry(t *testing.T) {
	members := []gotgbot.User{{Id: 5201, FirstName: "First"}, {Id: 5202, FirstName: "Second"}, {Id: 5203, FirstName: "Third"}}
	setup := func(t *testing.T, threshold int, categoryOn bool) (*moduleBotClient, *gotgbot.Bot, gotgbot.Chat) {
		t.Helper()
		withMiniredis(t)
		client := newModuleBotClient()
		bot := newModuleTestBot(client)
		chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Automated Log Chat"}
		if err := logchannels.Set(chat.Id, chat.Title, automatedLogChannel); err != nil {
			t.Fatalf("logchannels.Set() error = %v", err)
		}
		if !categoryOn {
			if err := logchannels.SetCategory(chat.Id, logchannels.CategoryAutomated, false); err != nil {
				t.Fatalf("SetCategory() error = %v", err)
			}
		}
		if err := antiraid.SetAutoAntiRaidThreshold(chat.Id, threshold); err != nil {
			t.Fatalf("SetAutoAntiRaidThreshold() error = %v", err)
		}
		t.Cleanup(func() {
			_, _ = antiRaidModule.disableRaid(chat.Id)
			clearJoinTracking(chat.Id)
		})
		return client, bot, chat
	}

	t.Run("logged once for the whole raid", func(t *testing.T) {
		client, bot, chat := setup(t, 2, true)
		raidJoin(t, bot, chat, members...)

		if calls := client.callsFor("banChatMember"); len(calls) != 2 {
			t.Fatalf("banChatMember calls = %d, want the joiners from the threshold on", len(calls))
		}
		entries := spamDBMessagesTo(client, automatedLogChannel)
		if len(entries) != 1 {
			t.Fatalf("log channel entries = %d, want 1: %v", len(entries), entries)
		}
		for _, want := range []string{"#ANTIRAID", "Joins: 2", "Threshold: 2"} {
			if !strings.Contains(entries[0], want) {
				t.Fatalf("log entry %q does not contain %q", entries[0], want)
			}
		}
		if strings.Contains(entries[0], "User:") {
			t.Fatalf("log entry %q names a user, want the raid alone", entries[0])
		}
	})

	t.Run("category off", func(t *testing.T) {
		client, bot, chat := setup(t, 2, false)
		raidJoin(t, bot, chat, members...)
		if entries := spamDBMessagesTo(client, automatedLogChannel); len(entries) != 0 {
			t.Fatalf("log channel entries = %d, want none: %v", len(entries), entries)
		}
	})

	t.Run("below the threshold", func(t *testing.T) {
		client, bot, chat := setup(t, 5, true)
		raidJoin(t, bot, chat, members...)
		if entries := spamDBMessagesTo(client, automatedLogChannel); len(entries) != 0 {
			t.Fatalf("log channel entries = %d, want none: %v", len(entries), entries)
		}
	})
}

func TestBotsLockLogsTheBannedBot(t *testing.T) {
	checkAutomatedLog(t, automatedLogCase{
		name:       "ban",
		tag:        "LOCK_BOTS",
		userID:     4243,
		want:       []string{"Added by", "42"},
		failMethod: "banChatMember",
		run: func(t *testing.T, _ *moduleBotClient, bot *gotgbot.Bot, chat gotgbot.Chat) {
			if err := locks.UpdateLock(chat.Id, "bots", true); err != nil {
				t.Fatalf("UpdateLock() error = %v", err)
			}
			adder := gotgbot.User{Id: 42, FirstName: "Member"}
			addedBot := gotgbot.User{Id: 4243, IsBot: true, FirstName: "Added Bot"}
			ctx := ext.NewContext(bot, &gotgbot.Update{UpdateId: 3, ChatMember: &gotgbot.ChatMemberUpdated{
				Chat:          chat,
				From:          adder,
				OldChatMember: gotgbot.ChatMemberLeft{User: addedBot},
				NewChatMember: gotgbot.ChatMemberMember{User: addedBot},
			}}, nil)
			_ = locksModule.botLockHandler(bot, ctx)
		},
	})
}
