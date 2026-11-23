//go:build testtools

package chat_status

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/approvals"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/updatememo"
)

// protectedSenderCtx builds a context whose effective sender is sender in a
// supergroup chat, the shape message watchers hand the predicate.
func protectedSenderCtx(sender *gotgbot.Sender, withMemo bool) *ext.Context {
	ctx := &ext.Context{
		EffectiveChat:   &gotgbot.Chat{Id: -1001, Type: "supergroup", Title: "Protected Chat"},
		EffectiveSender: sender,
	}
	if withMemo {
		ctx.Data = map[string]any{updatememo.DataKey: updatememo.New()}
	}
	return ctx
}

func userSender(user gotgbot.User) *gotgbot.Sender {
	return &gotgbot.Sender{User: &user, ChatId: -1001}
}

// withoutCacheMarshal clears the cache marshaler for the duration of a test,
// so every uncached lookup reaches the fake bot client.
func withoutCacheMarshal(t *testing.T) {
	t.Helper()
	previous := cache.GetMarshal()
	cache.SetMarshal(nil)
	t.Cleanup(func() { cache.SetMarshal(previous) })
}

// TestIsProtectedForUpdateMatrix asserts the exemption matrix once, through
// the predicate: system IDs, bots, anonymous and channel senders, admins and
// approved users are protected; a plain member is not.
func TestIsProtectedForUpdateMatrix(t *testing.T) {
	withoutCacheMarshal(t)
	bot := newChatStatusBot(999)
	chatID := int64(-1001)

	t.Run("system and non-positive IDs", func(t *testing.T) {
		for _, id := range []int64{0, -5, TelegramUserID, GroupAnonymousBotID, SendAsChannelBotID} {
			if !IsProtectedForUpdate(bot, protectedSenderCtx(userSender(gotgbot.User{Id: 42}), false), chatID, id) {
				t.Fatalf("IsProtectedForUpdate(userID=%d) = false, want true", id)
			}
		}
	})

	t.Run("bot sender", func(t *testing.T) {
		sender := userSender(gotgbot.User{Id: 555, IsBot: true, FirstName: "Other Bot"})
		if !IsProtectedForUpdate(bot, protectedSenderCtx(sender, false), chatID, 555) {
			t.Fatal("IsProtectedForUpdate(bot sender) = false, want true")
		}
	})

	t.Run("anonymous and channel senders", func(t *testing.T) {
		group := gotgbot.Chat{Id: chatID, Type: "supergroup"}
		channel := gotgbot.Chat{Id: -100222, Type: "channel", Title: "Chan"}
		cases := map[string]*gotgbot.Sender{
			"anonymous admin":   {Chat: &group, ChatId: chatID},
			"anonymous channel": {Chat: &channel, ChatId: chatID},
			"linked channel":    {Chat: &channel, ChatId: chatID, IsAutomaticForward: true},
		}
		for name, sender := range cases {
			if !IsProtectedForUpdate(bot, protectedSenderCtx(sender, false), chatID, sender.Id()) {
				t.Fatalf("IsProtectedForUpdate(%s sender) = false, want true", name)
			}
		}
	})

	t.Run("sender steps skip other users", func(t *testing.T) {
		// An anonymous admin's update must not exempt the member it added:
		// the sender shape applies only when userID is the sender.
		group := gotgbot.Chat{Id: chatID, Type: "supergroup"}
		sender := &gotgbot.Sender{Chat: &group, ChatId: chatID}
		ctx := protectedSenderCtx(sender, false)
		if IsProtectedForUpdate(bot, ctx, chatID, 42) {
			t.Fatal("IsProtectedForUpdate(member 42 under anonymous sender) = true, want false")
		}
	})

	t.Run("admin and creator", func(t *testing.T) {
		for _, id := range []int64{10, 12} {
			sender := userSender(gotgbot.User{Id: id, FirstName: "Admin"})
			if !IsProtectedForUpdate(bot, protectedSenderCtx(sender, false), chatID, id) {
				t.Fatalf("IsProtectedForUpdate(admin %d) = false, want true", id)
			}
		}
	})

	t.Run("approved user", func(t *testing.T) {
		if err := approvals.AddApprovedUser(chatID, 43, 1, "test"); err != nil {
			t.Fatalf("AddApprovedUser() error = %v", err)
		}
		t.Cleanup(func() { _ = approvals.RemoveAllApprovedUsers(chatID) })
		sender := userSender(gotgbot.User{Id: 43, FirstName: "Approved"})
		if !IsProtectedForUpdate(bot, protectedSenderCtx(sender, false), chatID, 43) {
			t.Fatal("IsProtectedForUpdate(approved user) = false, want true")
		}
	})

	t.Run("plain member", func(t *testing.T) {
		sender := userSender(gotgbot.User{Id: 44, FirstName: "Member"})
		if IsProtectedForUpdate(bot, protectedSenderCtx(sender, false), chatID, 44) {
			t.Fatal("IsProtectedForUpdate(plain member) = true, want false")
		}
	})

	t.Run("nil context and nil sender", func(t *testing.T) {
		if !IsProtectedForUpdate(bot, nil, chatID, TelegramUserID) {
			t.Fatal("IsProtectedForUpdate(nil ctx, system user) = false, want true")
		}
		if IsProtectedForUpdate(bot, protectedSenderCtx(nil, false), chatID, 44) {
			t.Fatal("IsProtectedForUpdate(nil sender, member) = true, want false")
		}
	})
}

func TestIsProtectedForUpdateReusesLookupWithinUpdate(t *testing.T) {
	withoutCacheMarshal(t)

	client := &recordingChatStatusClient{}
	bot := &gotgbot.Bot{
		Token:     "999:test",
		BotClient: client,
		User:      gotgbot.User{Id: 999, IsBot: true, FirstName: "Bot"},
	}
	chatID := int64(-1001)
	userID := int64(10) // administrator in chatStatusBotClient's admin list
	sender := userSender(gotgbot.User{Id: userID, FirstName: "Admin"})

	memoized := protectedSenderCtx(sender, true)
	for range 2 {
		if !IsProtectedForUpdate(bot, memoized, chatID, userID) {
			t.Fatal("IsProtectedForUpdate() = false, want true")
		}
	}
	if calls := client.callsFor("getChatAdministrators"); calls != 1 {
		t.Fatalf("getChatAdministrators calls with memo = %d, want 1", calls)
	}

	unmemoized := protectedSenderCtx(sender, false)
	for range 2 {
		if !IsProtectedForUpdate(bot, unmemoized, chatID, userID) {
			t.Fatal("IsProtectedForUpdate() = false, want true")
		}
	}
	if calls := client.callsFor("getChatAdministrators"); calls != 3 {
		t.Fatalf("getChatAdministrators calls total = %d, want 3", calls)
	}
}
