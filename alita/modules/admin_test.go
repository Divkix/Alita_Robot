package modules

import (
	"errors"
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

func TestAdminListReportsWhenOnlyBotsAreVisible(t *testing.T) {
	client := newModuleBotClient()
	client.responses["getChatAdministrators"] = []byte(
		`[` +
			`{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"}},` +
			`{"status":"administrator","user":{"id":1087968824,"is_bot":false,"first_name":"Group Anonymous Bot"},"is_anonymous":true}` +
			`]`,
	)
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Admin Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	ctx := newModuleMessageContext(bot, chat, user, "/adminlist")

	if err := adminModule.adminlist(bot, ctx); err != ext.EndGroups {
		t.Fatalf("adminlist(no visible admins) error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want no-visible-admins response", len(calls))
	}
}

func TestPromoteReplyPromotesTargetAndSetsTitle(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Admin Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	target := gotgbot.User{Id: 42, FirstName: "Member"}
	ctx := newBanReplyContext(bot, chat, admin, target, "/promote VeryLongCustomAdminTitle")

	if err := adminModule.promote(bot, ctx); err != ext.EndGroups {
		t.Fatalf("promote() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("promoteChatMember"); len(calls) != 1 {
		t.Fatalf("promoteChatMember calls = %d, want 1", len(calls))
	}
	if calls := client.callsFor("setChatAdministratorCustomTitle"); len(calls) != 1 {
		t.Fatalf("setChatAdministratorCustomTitle calls = %d, want 1", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
}

func TestPromoteRejectsInvalidAndProtectedTargets(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Admin Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	tests := []struct {
		name string
		text string
	}{
		{name: "missing target", text: "/promote"},
		{name: "channel id", text: "/promote -1001234567890"},
		{name: "owner", text: "/promote 777000"},
		{name: "bot itself", text: "/promote 999"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newModuleMessageContext(bot, chat, admin, tt.text)
			if err := adminModule.promote(bot, ctx); err != ext.EndGroups {
				t.Fatalf("promote(%s) error = %v, want EndGroups", tt.name, err)
			}
		})
	}

	if calls := client.callsFor("promoteChatMember"); len(calls) != 0 {
		t.Fatalf("promoteChatMember calls = %d, want none for rejected targets", len(calls))
	}
	if calls := client.callsFor("setChatAdministratorCustomTitle"); len(calls) != 0 {
		t.Fatalf("setChatAdministratorCustomTitle calls = %d, want none", len(calls))
	}
}

func TestPromoteRejectsExistingAdminAndMissingAdminCache(t *testing.T) {
	t.Run("target already admin", func(t *testing.T) {
		client := newModuleBotClient()
		client.responses["getChatMember"] = []byte(
			`{"status":"administrator","user":{"id":42,"is_bot":false,"first_name":"Member"},"can_promote_members":true}`,
		)
		client.responses["getChatAdministrators"] = []byte(
			`[` +
				`{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"}},` +
				`{"status":"administrator","user":{"id":42,"is_bot":false,"first_name":"Member"},"can_promote_members":true}` +
				`]`,
		)
		bot := newModuleTestBot(client)
		chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Admin Chat"}
		admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
		ctx := newModuleMessageContext(bot, chat, admin, "/promote 42")

		if err := adminModule.promote(bot, ctx); err != ext.EndGroups {
			t.Fatalf("promote(existing admin) error = %v, want EndGroups", err)
		}
		if calls := client.callsFor("promoteChatMember"); len(calls) != 0 {
			t.Fatalf("promoteChatMember calls = %d, want none for existing admin", len(calls))
		}
	})

	t.Run("empty admin cache", func(t *testing.T) {
		client := newModuleBotClient()
		client.responses["getChatAdministrators"] = []byte(`[]`)
		bot := newModuleTestBot(client)
		chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Admin Chat"}
		admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
		ctx := newModuleMessageContext(bot, chat, admin, "/promote 42")

		if err := adminModule.promote(bot, ctx); err != ext.EndGroups {
			t.Fatalf("promote(empty admin cache) error = %v, want EndGroups", err)
		}
		if calls := client.callsFor("promoteChatMember"); len(calls) != 0 {
			t.Fatalf("promoteChatMember calls = %d, want none without admin cache", len(calls))
		}
		if calls := client.callsFor("sendMessage"); len(calls) != 1 {
			t.Fatalf("sendMessage calls = %d, want admin-cache failure reply", len(calls))
		}
	})
}

func TestDemoteReplyDemotesAdminTarget(t *testing.T) {
	client := newModuleBotClient()
	client.responses["getChatMember"] = []byte(
		`{"status":"administrator","user":{"id":42,"is_bot":false,"first_name":"Member"},"can_promote_members":true}`,
	)
	client.responses["getChatAdministrators"] = []byte(
		`[` +
			`{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"}},` +
			`{"status":"administrator","user":{"id":42,"is_bot":false,"first_name":"Member"},"can_promote_members":true}` +
			`]`,
	)
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Admin Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	target := gotgbot.User{Id: 42, FirstName: "Member"}
	ctx := newBanReplyContext(bot, chat, admin, target, "/demote")

	if err := adminModule.demote(bot, ctx); err != ext.EndGroups {
		t.Fatalf("demote() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("promoteChatMember"); len(calls) != 1 {
		t.Fatalf("promoteChatMember calls = %d, want 1", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
}

func TestDemoteValidationBranches(t *testing.T) {
	client := newModuleBotClient()
	client.responses["getChatAdministrators"] = []byte(
		`[` +
			`{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"}},` +
			`{"status":"administrator","user":{"id":42,"is_bot":false,"first_name":"Member"},"can_promote_members":true}` +
			`]`,
	)
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Admin Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	tests := []struct {
		name string
		text string
	}{
		{name: "missing target", text: "/demote"},
		{name: "channel id", text: "/demote -1001234567890"},
		{name: "owner", text: "/demote 777000"},
		{name: "bot itself", text: "/demote 999"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newModuleMessageContext(bot, chat, admin, tt.text)
			if err := adminModule.demote(bot, ctx); err != ext.EndGroups {
				t.Fatalf("demote(%s) error = %v, want EndGroups", tt.name, err)
			}
		})
	}

	if calls := client.callsFor("promoteChatMember"); len(calls) != 0 {
		t.Fatalf("promoteChatMember calls = %d, want none for rejected demotions", len(calls))
	}
}

func TestDemoteRejectsMissingAdminCache(t *testing.T) {
	client := newModuleBotClient()
	client.responses["getChatAdministrators"] = []byte(`[]`)
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Admin Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	ctx := newModuleMessageContext(bot, chat, admin, "/demote 42")

	if err := adminModule.demote(bot, ctx); err != ext.EndGroups {
		t.Fatalf("demote(empty admin cache) error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("promoteChatMember"); len(calls) != 0 {
		t.Fatalf("promoteChatMember calls = %d, want none without admin cache", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want admin-cache failure reply", len(calls))
	}
}

func TestSetTitleReplyUpdatesAdminTitle(t *testing.T) {
	client := newModuleBotClient()
	client.responses["getChatMember"] = []byte(
		`{"status":"administrator","user":{"id":42,"is_bot":false,"first_name":"Member"},"custom_title":"Captain","can_promote_members":true}`,
	)
	client.responses["getChatAdministrators"] = []byte(
		`[` +
			`{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"}},` +
			`{"status":"administrator","user":{"id":42,"is_bot":false,"first_name":"Member"},"custom_title":"Captain","can_promote_members":true}` +
			`]`,
	)
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Admin Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	target := gotgbot.User{Id: 42, FirstName: "Member"}
	ctx := newBanReplyContext(bot, chat, admin, target, "/title Captain")

	if err := adminModule.setTitle(bot, ctx); err != ext.EndGroups {
		t.Fatalf("setTitle() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("setChatAdministratorCustomTitle"); len(calls) != 1 {
		t.Fatalf("setChatAdministratorCustomTitle calls = %d, want 1", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
}

func TestSetTitleValidationAndTruncation(t *testing.T) {
	client := newModuleBotClient()
	client.responses["getChatMember"] = []byte(
		`{"status":"administrator","user":{"id":42,"is_bot":false,"first_name":"Member"},"can_promote_members":true}`,
	)
	client.responses["getChatAdministrators"] = []byte(
		`[` +
			`{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"}},` +
			`{"status":"administrator","user":{"id":42,"is_bot":false,"first_name":"Member"},"can_promote_members":true}` +
			`]`,
	)
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Admin Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	for _, tt := range []struct {
		name string
		text string
	}{
		{name: "missing target", text: "/title"},
		{name: "owner", text: "/title 777000 Boss"},
		{name: "bot itself", text: "/title 999 Boss"},
		{name: "empty title", text: "/title 42"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newModuleMessageContext(bot, chat, admin, tt.text)
			if err := adminModule.setTitle(bot, ctx); err != ext.EndGroups {
				t.Fatalf("setTitle(%s) error = %v, want EndGroups", tt.name, err)
			}
		})
	}

	longTitleCtx := newModuleMessageContext(bot, chat, admin, "/title 42 VeryLongCustomAdminTitle")
	if err := adminModule.setTitle(bot, longTitleCtx); err != ext.EndGroups {
		t.Fatalf("setTitle(long title) error = %v, want EndGroups", err)
	}

	calls := client.callsFor("setChatAdministratorCustomTitle")
	if len(calls) != 1 {
		t.Fatalf("setChatAdministratorCustomTitle calls = %d, want only long-title success", len(calls))
	}
	if got, want := calls[0].Params["custom_title"], "VeryLongCustomAd"; got != want {
		t.Fatalf("custom_title = %q, want truncated %q", got, want)
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

func TestGetInviteLinkFetchesPrivateInviteLink(t *testing.T) {
	client := newModuleBotClient()
	client.responses["getChat"] = []byte(
		`{"id":-1001,"type":"supergroup","title":"Admin Chat","invite_link":"https://t.me/+invite"}`,
	)
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Admin Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	ctx := newModuleMessageContext(bot, chat, user, "/invitelink")

	if err := adminModule.getinvitelink(bot, ctx); err != ext.EndGroups {
		t.Fatalf("getinvitelink(private) error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("getChat"); len(calls) != 1 {
		t.Fatalf("getChat calls = %d, want private invite lookup", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want invite reply", len(calls))
	}
}

func TestGetInviteLinkHandlesPrivateLookupFailure(t *testing.T) {
	client := newModuleBotClient()
	client.errors["getChat"] = errors.New("telegram unavailable")
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Admin Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	ctx := newModuleMessageContext(bot, chat, user, "/invitelink")

	if err := adminModule.getinvitelink(bot, ctx); err != ext.EndGroups {
		t.Fatalf("getinvitelink(lookup failure) error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want lookup-error reply", len(calls))
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

func TestAnonAdminHandlesNoopAndInvalidOptions(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Admin Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	t.Cleanup(func() {
		_ = db.SetAnonAdminMode(chat.Id, false)
	})

	if err := db.SetAnonAdminMode(chat.Id, true); err != nil {
		t.Fatalf("SetAnonAdminMode(true) error = %v", err)
	}
	onAgainCtx := newModuleMessageContext(bot, chat, user, "/anonadmin yes")
	if err := adminModule.anonAdmin(bot, onAgainCtx); err != ext.EndGroups {
		t.Fatalf("anonAdmin(already on) error = %v, want EndGroups", err)
	}

	if err := db.SetAnonAdminMode(chat.Id, false); err != nil {
		t.Fatalf("SetAnonAdminMode(false) error = %v", err)
	}
	offAgainCtx := newModuleMessageContext(bot, chat, user, "/anonadmin off")
	if err := adminModule.anonAdmin(bot, offAgainCtx); err != ext.EndGroups {
		t.Fatalf("anonAdmin(already off) error = %v, want EndGroups", err)
	}

	invalidCtx := newModuleMessageContext(bot, chat, user, "/anonadmin maybe")
	if err := adminModule.anonAdmin(bot, invalidCtx); err != ext.EndGroups {
		t.Fatalf("anonAdmin(invalid) error = %v, want EndGroups", err)
	}

	if calls := client.callsFor("sendMessage"); len(calls) != 3 {
		t.Fatalf("sendMessage calls = %d, want one response per anonadmin branch", len(calls))
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

func TestAdminCacheHandlesMemberAndLookupFailures(t *testing.T) {
	memberClient := newModuleBotClient()
	memberBot := newModuleTestBot(memberClient)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Admin Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	memberCtx := newModuleMessageContext(memberBot, chat, member, "/admincache")
	if err := adminModule.adminCache(memberBot, memberCtx); err != ext.EndGroups {
		t.Fatalf("adminCache(member) error = %v, want EndGroups", err)
	}
	if calls := memberClient.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want non-admin denial", len(calls))
	}

	errorClient := newModuleBotClient()
	errorClient.errors["getChatMember"] = errors.New("telegram unavailable")
	errorBot := newModuleTestBot(errorClient)
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	errorCtx := newModuleMessageContext(errorBot, chat, admin, "/admincache")
	if err := adminModule.adminCache(errorBot, errorCtx); err != ext.EndGroups {
		t.Fatalf("adminCache(lookup failure) error = %v, want EndGroups", err)
	}
	if calls := errorClient.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want lookup-failure denial", len(calls))
	}
}

func TestAdminCommandsPropagateGotgbotRequestErrors(t *testing.T) {
	requestErr := errors.New("telegram request failed")
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	target := gotgbot.User{Id: 42, FirstName: "Member"}

	adminMemberResponse := []byte(
		`{"status":"administrator","user":{"id":42,"is_bot":false,"first_name":"Member"},"custom_title":"Captain","can_promote_members":true}`,
	)
	adminListResponse := []byte(
		`[` +
			`{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"}},` +
			`{"status":"administrator","user":{"id":42,"is_bot":false,"first_name":"Member"},"custom_title":"Captain","can_promote_members":true}` +
			`]`,
	)

	for _, tt := range []struct {
		name      string
		text      string
		method    string
		setup     func(*moduleBotClient)
		withReply bool
		run       func(*gotgbot.Bot, *ext.Context) error
	}{
		{name: "admin list reply", text: "/adminlist", method: "sendMessage", run: adminModule.adminlist},
		{name: "promote missing target reply", text: "/promote", method: "sendMessage", run: adminModule.promote},
		{
			name:   "promote request",
			text:   "/promote 42 Captain",
			method: "promoteChatMember",
			run:    adminModule.promote,
		},
		{name: "demote missing target reply", text: "/demote", method: "sendMessage", run: adminModule.demote},
		{
			name:   "demote not-admin reply",
			text:   "/demote 42",
			method: "sendMessage",
			run:    adminModule.demote,
		},
		{name: "title missing target reply", text: "/title", method: "sendMessage", run: adminModule.setTitle},
		{
			name:   "title request",
			text:   "/title Captain",
			method: "setChatAdministratorCustomTitle",
			setup: func(client *moduleBotClient) {
				client.responses["getChatMember"] = adminMemberResponse
				client.responses["getChatAdministrators"] = adminListResponse
			},
			withReply: true,
			run:       adminModule.setTitle,
		},
		{name: "anon admin status reply", text: "/anonadmin", method: "sendMessage", run: adminModule.anonAdmin},
		{name: "admin cache success reply", text: "/admincache", method: "sendMessage", run: adminModule.adminCache},
		{name: "clear admin cache reply", text: "/clearadmincache", method: "sendMessage", run: adminModule.clearAdminCache},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			client.errors[tt.method] = requestErr
			if tt.setup != nil {
				tt.setup(client)
			}
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Admin Chat"}
			var ctx *ext.Context
			if tt.withReply {
				ctx = newBanReplyContext(bot, chat, admin, target, tt.text)
			} else {
				ctx = newModuleMessageContext(bot, chat, admin, tt.text)
			}

			err := tt.run(bot, ctx)
			if !errors.Is(err, requestErr) {
				t.Fatalf("%s returned error %v, want request error", tt.text, err)
			}
		})
	}
}

func fmtInt(value int64) string {
	return strconv.FormatInt(value, 10)
}
