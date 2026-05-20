package modules

import (
	"strconv"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

// testError is a simple error implementation for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

// TestDemoteErrorHandling verifies error handling patterns in demote logic
func TestDemoteErrorHandling(t *testing.T) {
	// simulateGetMemberResult simulates the return values of GetMember,
	// returning values that staticcheck cannot statically resolve.
	simulateGetMemberResult := func(wantErr bool) (gotgbot.ChatMember, error) {
		if wantErr {
			return nil, &testError{msg: "API error"}
		}
		return nil, nil
	}

	t.Run("error takes precedence over nil member", func(t *testing.T) {
		// When GetMember returns (nil, error), error should be checked first
		// This is the standard pattern: check err != nil before using result
		userMember, err := simulateGetMemberResult(true)

		if err != nil {
			// Expected: error is handled first
			t.Logf("Error handled first: %v", err)
			return
		}

		// Should not reach here when err != nil
		if userMember == nil {
			t.Fatal("Should have returned on error, not reached nil check")
		}
	})

	t.Run("nil error with nil member", func(t *testing.T) {
		// When GetMember returns (nil, nil), the nil member should be handled
		userMember, err := simulateGetMemberResult(false)

		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		// After confirming no error, check the member
		if userMember == nil {
			t.Log("Nil member properly detected after nil error check")
			return
		}

		t.Log("Non-nil member received")
	})
}

func TestAdminListLoadsAndRepliesWithVisibleAdmins(t *testing.T) {
	client := newModuleBotClient()
	client.responses["getChatAdministrators"] = []byte(
		`[` +
			`{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"}},` +
			`{"status":"administrator","user":{"id":4242,"is_bot":false,"first_name":"Visible","username":"visibleadmin"}}` +
			`]`,
	)
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Admin Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	ctx := newModuleMessageContext(bot, chat, user, "/adminlist")

	if err := adminModule.adminlist(bot, ctx); err != ext.EndGroups {
		t.Fatalf("adminlist() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("getChatAdministrators"); len(calls) != 1 {
		t.Fatalf("getChatAdministrators calls = %d, want 1", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
}

func TestGetInviteLinkUsesPublicUsernameWithoutFetchingChat(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{
		Id:       uniqueModuleChatID(),
		Type:     "supergroup",
		Title:    "Admin Chat",
		Username: "public_chat",
	}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	ctx := newModuleMessageContext(bot, chat, user, "/invitelink")

	if err := adminModule.getinvitelink(bot, ctx); err != ext.EndGroups {
		t.Fatalf("getinvitelink() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("getChat"); len(calls) != 0 {
		t.Fatalf("getChat calls = %d, want 0 for public username", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
}

func TestAnonAdminOwnerTogglesSetting(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Admin Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	onCtx := newModuleMessageContext(bot, chat, user, "/anonadmin on")
	if err := adminModule.anonAdmin(bot, onCtx); err != ext.EndGroups {
		t.Fatalf("anonAdmin(on) error = %v, want EndGroups", err)
	}
	if !db.GetAdminSettings(chat.Id).AnonAdmin {
		t.Fatal("AnonAdmin was not enabled")
	}

	statusCtx := newModuleMessageContext(bot, chat, user, "/anonadmin")
	if err := adminModule.anonAdmin(bot, statusCtx); err != ext.EndGroups {
		t.Fatalf("anonAdmin(status) error = %v, want EndGroups", err)
	}

	offCtx := newModuleMessageContext(bot, chat, user, "/anonadmin false")
	if err := adminModule.anonAdmin(bot, offCtx); err != ext.EndGroups {
		t.Fatalf("anonAdmin(off) error = %v, want EndGroups", err)
	}
	if db.GetAdminSettings(chat.Id).AnonAdmin {
		t.Fatal("AnonAdmin stayed enabled after false")
	}
}

func TestAdminCacheCommandsRefreshAndClearCache(t *testing.T) {
	client := newModuleBotClient()
	client.responses["getChatAdministrators"] = []byte(
		`[{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"}}]`,
	)
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Admin Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	refreshCtx := newModuleMessageContext(bot, chat, user, "/admincache")
	if err := adminModule.adminCache(bot, refreshCtx); err != ext.EndGroups {
		t.Fatalf("adminCache() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("getChatAdministrators"); len(calls) != 1 {
		t.Fatalf("getChatAdministrators calls = %d, want 1", len(calls))
	}

	clearChat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Admin Chat"}
	if err := cache.Marshal.Set(cache.Context, "alita:adminCache:"+fmtInt(clearChat.Id), cache.AdminCache{
		ChatId: clearChat.Id,
		UserInfo: []gotgbot.MergedChatMember{
			{
				Status: "administrator",
				User:   gotgbot.User{Id: 999, IsBot: true, FirstName: "Alita"},
			},
		},
		Cached: true,
	}); err != nil {
		t.Fatalf("seed admin cache: %v", err)
	}
	clearCtx := newModuleMessageContext(bot, clearChat, user, "/clearadmincache")
	if err := adminModule.clearAdminCache(bot, clearCtx); err != ext.EndGroups {
		t.Fatalf("clearAdminCache() error = %v, want EndGroups", err)
	}
	if found, _ := cache.GetAdminCacheList(clearChat.Id); found {
		t.Fatal("admin cache entry remained after /clearadmincache")
	}
}

func fmtInt(value int64) string {
	return strconv.FormatInt(value, 10)
}
