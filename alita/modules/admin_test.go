package modules

import (
	"errors"
	"fmt"
	"maps"
	"strconv"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/admin"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestDemoteErrorHandling(t *testing.T) {
	// returning values that staticcheck cannot statically resolve.
	simulateGetMemberResult := func(wantErr bool) (gotgbot.ChatMember, error) {
		if wantErr {
			return nil, &testError{msg: "API error"}
		}
		return nil, nil
	}

	t.Run("error takes precedence over nil member", func(t *testing.T) {
		userMember, err := simulateGetMemberResult(true)

		if err != nil {
			t.Logf("Error handled first: %v", err)
			return
		}

		if userMember == nil {
			t.Fatal("Should have returned on error, not reached nil check")
		}
	})

	t.Run("nil error with nil member", func(t *testing.T) {
		userMember, err := simulateGetMemberResult(false)

		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if userMember == nil {
			t.Log("Nil member properly detected after nil error check")
			return
		}

		t.Log("Non-nil member received")
	})
}

func TestAdminListLoadsAndRepliesWithVisibleAdmins(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, user := f.client, f.bot, f.chat, f.admin
	f.addAdmins(
		`{"status":"administrator","user":{"id":4242,"is_bot":false,"first_name":"Visible","username":"visibleadmin"}}`,
		`{"status":"administrator","user":{"id":4343,"is_bot":true,"first_name":"Helper","username":"helperbot"}}`,
		`{"status":"administrator","user":{"id":4444,"is_bot":false,"first_name":"Hidden","username":"hiddenadmin"},"is_anonymous":true}`,
	)
	ctx := newModuleMessageContext(bot, chat, user, "/adminlist")

	cmdCtx, err := helpers.BuildCommandContext(bot, ctx)
	if err != nil {
		t.Fatalf("BuildCommandContext failed: %v", err)
	}
	if err := adminModule.adminlist(cmdCtx); err != ext.EndGroups {
		t.Fatalf("adminlist() error = %v, want EndGroups", err)
	}
	assertParams(t, client.onlyCall(t, "getChatAdministrators"), map[string]any{"chat_id": chat.Id})
	text := fmt.Sprint(client.onlyCall(t, "sendMessage").Params["text"])
	for _, want := range []string{"@visibleadmin", fmt.Sprintf("tg://user?id=%d", user.Id)} {
		if !strings.Contains(text, want) {
			t.Errorf("adminlist reply lacks visible admin %q:\n%s", want, text)
		}
	}
	for _, hidden := range []string{"tg://user?id=999", "@helperbot", "@hiddenadmin"} {
		if strings.Contains(text, hidden) {
			t.Errorf("adminlist reply lists bot or anonymous admin %q:\n%s", hidden, text)
		}
	}
}

func TestAdminListReportsWhenOnlyBotsAreVisible(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, user := f.client, f.bot, f.chat, f.member
	client.adminsByChat[fmt.Sprint(chat.Id)] = []byte(
		`[` +
			`{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"}},` +
			`{"status":"administrator","user":{"id":1087968824,"is_bot":false,"first_name":"Group Anonymous Bot"},"is_anonymous":true}` +
			`]`,
	)
	ctx := newModuleMessageContext(bot, chat, user, "/adminlist")

	cmdCtx, err := helpers.BuildCommandContext(bot, ctx)
	if err != nil {
		t.Fatalf("BuildCommandContext failed: %v", err)
	}
	if err := adminModule.adminlist(cmdCtx); err != ext.EndGroups {
		t.Fatalf("adminlist(no visible admins) error = %v, want EndGroups", err)
	}
	if text := fmt.Sprint(client.onlyCall(t, "sendMessage").Params["text"]); strings.Contains(text, "tg://user?id=") {
		t.Fatalf("adminlist reply mentions a bot or anonymous admin:\n%s", text)
	}
}

// grantedRights lists the can_* rights a promoteChatMember call turns on;
// gotgbot omits false booleans, so absent rights are not granted.
func grantedRights(call moduleBotCall) map[string]bool {
	granted := map[string]bool{}
	for key, value := range call.Params {
		if !strings.HasPrefix(key, "can_") {
			continue
		}
		switch v := value.(type) {
		case bool:
			if v {
				granted[key] = true
			}
		case *bool:
			if v != nil && *v {
				granted[key] = true
			}
		}
	}
	return granted
}

func TestPromoteReplyPromotesTargetAndSetsTitle(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, admin := f.client, f.bot, f.chat, f.admin
	target := gotgbot.User{Id: 42, FirstName: "Member"}
	ctx := newBanReplyContext(bot, chat, admin, target, "/promote VeryLongCustomAdminTitle")

	cmdCtx, err := helpers.BuildCommandContext(bot, ctx)
	if err != nil {
		t.Fatalf("BuildCommandContext failed: %v", err)
	}
	if err := adminModule.promote(cmdCtx); err != ext.EndGroups {
		t.Fatalf("promote() error = %v, want EndGroups", err)
	}
	promote := client.onlyCall(t, "promoteChatMember")
	assertParams(t, promote, targetParams(chat.Id, target.Id))
	// The bot and the promoter both hold exactly these rights.
	wantRights := map[string]bool{
		"can_manage_chat": true, "can_delete_messages": true, "can_restrict_members": true,
		"can_change_info": true, "can_invite_users": true, "can_pin_messages": true,
	}
	if got := grantedRights(promote); !maps.Equal(got, wantRights) {
		t.Errorf("granted rights = %v, want %v", got, wantRights)
	}
	title := client.onlyCall(t, "setChatAdministratorCustomTitle")
	assertParams(t, title, map[string]any{"chat_id": chat.Id, "user_id": target.Id, "custom_title": "VeryLongCustomAd"})
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
}

func TestPromoteGrantsOnlyRightsThePromoterHolds(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, admin := f.client, f.bot, f.chat, f.admin
	// A non-owner promoter without can_change_info or can_pin_messages.
	client.membersByUser[fmt.Sprint(admin.Id)] = []byte(fmt.Sprintf(
		`{"status":"administrator","user":{"id":%d,"is_bot":false,"first_name":"Admin"},`+
			`"can_manage_chat":true,"can_delete_messages":true,"can_restrict_members":true,"can_promote_members":true,"can_invite_users":true}`,
		admin.Id,
	))
	target := gotgbot.User{Id: 42, FirstName: "Member"}
	cmdCtx, err := helpers.BuildCommandContext(bot, newBanReplyContext(bot, chat, admin, target, "/promote"))
	if err != nil {
		t.Fatalf("BuildCommandContext failed: %v", err)
	}
	if err := adminModule.promote(cmdCtx); err != ext.EndGroups {
		t.Fatalf("promote() error = %v, want EndGroups", err)
	}

	promote := client.onlyCall(t, "promoteChatMember")
	assertParams(t, promote, targetParams(chat.Id, target.Id))
	wantRights := map[string]bool{
		"can_manage_chat": true, "can_delete_messages": true, "can_restrict_members": true, "can_invite_users": true,
	}
	if got := grantedRights(promote); !maps.Equal(got, wantRights) {
		t.Errorf("granted rights = %v, want only the promoter's %v", got, wantRights)
	}
	if calls := client.callsFor("setChatAdministratorCustomTitle"); len(calls) != 0 {
		t.Fatalf("setChatAdministratorCustomTitle calls = %d, want none without a title", len(calls))
	}
}

func TestPromoteRejectsInvalidAndProtectedTargets(t *testing.T) {
	f := newAdminChatFixture(t)
	f.makeOwner()
	client, bot, chat, admin := f.client, f.bot, f.chat, f.admin

	tests := []struct {
		name string
		text string
	}{
		{name: "missing target", text: "/promote"},
		{name: "channel id", text: "/promote -1001234567890"},
		{name: "owner", text: "/promote " + fmtInt(admin.Id)},
		{name: "bot itself", text: "/promote 999"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newModuleMessageContext(bot, chat, admin, tt.text)
			cmdCtx, err := helpers.BuildCommandContext(bot, ctx)
			if err != nil {
				t.Fatalf("BuildCommandContext failed: %v", err)
			}
			if err := adminModule.promote(cmdCtx); err != ext.EndGroups {
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
		f := newAdminChatFixture(t)
		client, bot, chat, admin := f.client, f.bot, f.chat, f.admin
		client.responses["getChatMember"] = []byte(
			`{"status":"administrator","user":{"id":42,"is_bot":false,"first_name":"Member"},"can_promote_members":true}`,
		)
		f.addAdmins(`{"status":"administrator","user":{"id":42,"is_bot":false,"first_name":"Member"},"can_promote_members":true}`)
		ctx := newModuleMessageContext(bot, chat, admin, "/promote 42")
		cmdCtx, err := helpers.BuildCommandContext(bot, ctx)
		if err != nil {
			t.Fatalf("BuildCommandContext failed: %v", err)
		}

		if err := adminModule.promote(cmdCtx); err != ext.EndGroups {
			t.Fatalf("promote(existing admin) error = %v, want EndGroups", err)
		}
		if calls := client.callsFor("promoteChatMember"); len(calls) != 0 {
			t.Fatalf("promoteChatMember calls = %d, want none for existing admin", len(calls))
		}
	})

	t.Run("empty admin cache", func(t *testing.T) {
		f := newAdminChatFixture(t)
		client, bot, chat, admin := f.client, f.bot, f.chat, f.admin
		client.adminsByChat[fmt.Sprint(chat.Id)] = []byte(`[]`)
		ctx := newModuleMessageContext(bot, chat, admin, "/promote 42")
		cmdCtx, err := helpers.BuildCommandContext(bot, ctx)
		if err != nil {
			t.Fatalf("BuildCommandContext failed: %v", err)
		}

		if err := adminModule.promote(cmdCtx); err != ext.EndGroups {
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
	f := newAdminChatFixture(t)
	client, bot, chat, admin := f.client, f.bot, f.chat, f.admin
	client.responses["getChatMember"] = []byte(
		`{"status":"administrator","user":{"id":42,"is_bot":false,"first_name":"Member"},"can_promote_members":true}`,
	)
	f.addAdmins(`{"status":"administrator","user":{"id":42,"is_bot":false,"first_name":"Member"},"can_promote_members":true}`)
	target := gotgbot.User{Id: 42, FirstName: "Member"}
	ctx := newBanReplyContext(bot, chat, admin, target, "/demote")
	cmdCtx, err := helpers.BuildCommandContext(bot, ctx)
	if err != nil {
		t.Fatalf("BuildCommandContext failed: %v", err)
	}

	if err := adminModule.demote(cmdCtx); err != ext.EndGroups {
		t.Fatalf("demote() error = %v, want EndGroups", err)
	}
	demote := client.onlyCall(t, "promoteChatMember")
	assertParams(t, demote, targetParams(chat.Id, target.Id))
	if got := grantedRights(demote); len(got) != 0 {
		t.Errorf("demote kept rights %v, want none", got)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
}

func TestDemoteValidationBranches(t *testing.T) {
	f := newAdminChatFixture(t)
	f.makeOwner()
	f.addAdmins(`{"status":"administrator","user":{"id":42,"is_bot":false,"first_name":"Member"},"can_promote_members":true}`)
	client, bot, chat, admin := f.client, f.bot, f.chat, f.admin

	tests := []struct {
		name string
		text string
	}{
		{name: "missing target", text: "/demote"},
		{name: "channel id", text: "/demote -1001234567890"},
		{name: "owner", text: "/demote " + fmtInt(admin.Id)},
		{name: "bot itself", text: "/demote 999"},
		{name: "not an admin", text: "/demote 4545"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newModuleMessageContext(bot, chat, admin, tt.text)
			cmdCtx, err := helpers.BuildCommandContext(bot, ctx)
			if err != nil {
				t.Fatalf("BuildCommandContext failed: %v", err)
			}
			if err := adminModule.demote(cmdCtx); err != ext.EndGroups {
				t.Fatalf("demote(%s) error = %v, want EndGroups", tt.name, err)
			}
		})
	}

	if calls := client.callsFor("promoteChatMember"); len(calls) != 0 {
		t.Fatalf("promoteChatMember calls = %d, want none for rejected demotions", len(calls))
	}
}

func TestDemoteRejectsMissingAdminCache(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, admin := f.client, f.bot, f.chat, f.admin
	client.adminsByChat[fmt.Sprint(chat.Id)] = []byte(`[]`)
	ctx := newModuleMessageContext(bot, chat, admin, "/demote 42")
	cmdCtx, err := helpers.BuildCommandContext(bot, ctx)
	if err != nil {
		t.Fatalf("BuildCommandContext failed: %v", err)
	}

	if err := adminModule.demote(cmdCtx); err != ext.EndGroups {
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
	f := newAdminChatFixture(t)
	client := f.client
	client.responses["getChatMember"] = []byte(
		`{"status":"administrator","user":{"id":42,"is_bot":false,"first_name":"Member"},"custom_title":"Captain","can_promote_members":true}`,
	)
	f.addAdmins(`{"status":"administrator","user":{"id":42,"is_bot":false,"first_name":"Member"},"custom_title":"Captain","can_promote_members":true}`)
	bot, chat, admin := f.bot, f.chat, f.admin
	target := gotgbot.User{Id: 42, FirstName: "Member"}
	ctx := newBanReplyContext(bot, chat, admin, target, "/title Captain")
	cmdCtx, err := helpers.BuildCommandContext(bot, ctx)
	if err != nil {
		t.Fatalf("BuildCommandContext failed: %v", err)
	}

	if err := adminModule.setTitle(cmdCtx); err != ext.EndGroups {
		t.Fatalf("setTitle() error = %v, want EndGroups", err)
	}
	assertParams(t, client.onlyCall(t, "setChatAdministratorCustomTitle"),
		map[string]any{"chat_id": chat.Id, "user_id": target.Id, "custom_title": "Captain"})
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
}

// The success reply is HTML, so a name or title with markup characters must be
// escaped or Telegram rejects the reply after the title was already set.
func TestSetTitleReplyEscapesNameAndTitle(t *testing.T) {
	f := newAdminChatFixture(t)
	client := f.client
	targetJSON := `{"status":"administrator","user":{"id":42,"is_bot":false,"first_name":"<x & y>"},"custom_title":"<x & y>","can_promote_members":true}`
	client.responses["getChatMember"] = []byte(targetJSON)
	f.addAdmins(targetJSON)
	bot, chat, admin := f.bot, f.chat, f.admin
	target := gotgbot.User{Id: 42, FirstName: "<x & y>"}
	ctx := newBanReplyContext(bot, chat, admin, target, "/title Captain")
	cmdCtx, err := helpers.BuildCommandContext(bot, ctx)
	if err != nil {
		t.Fatalf("BuildCommandContext failed: %v", err)
	}

	if err := adminModule.setTitle(cmdCtx); err != ext.EndGroups {
		t.Fatalf("setTitle() error = %v, want EndGroups", err)
	}
	text := fmt.Sprint(client.onlyCall(t, "sendMessage").Params["text"])
	if strings.Contains(text, "<x & y>") || !strings.Contains(text, "&lt;x &amp; y&gt;") {
		t.Fatalf("title reply does not escape the name and title:\n%s", text)
	}
}

func TestSetTitleValidationAndTruncation(t *testing.T) {
	f := newAdminChatFixture(t)
	f.makeOwner()
	f.addAdmins(`{"status":"administrator","user":{"id":42,"is_bot":false,"first_name":"Member"},"can_promote_members":true}`)
	client, bot, chat, admin := f.client, f.bot, f.chat, f.admin
	client.responses["getChatMember"] = []byte(
		`{"status":"administrator","user":{"id":42,"is_bot":false,"first_name":"Member"},"can_promote_members":true}`,
	)

	for _, tt := range []struct {
		name string
		text string
	}{
		{name: "missing target", text: "/title"},
		{name: "owner", text: "/title " + fmtInt(admin.Id) + " Boss"},
		{name: "not an admin", text: "/title 4545 Boss"},
		{name: "bot itself", text: "/title 999 Boss"},
		{name: "empty title", text: "/title 42"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newModuleMessageContext(bot, chat, admin, tt.text)
			cmdCtx, err := helpers.BuildCommandContext(bot, ctx)
			if err != nil {
				t.Fatalf("BuildCommandContext failed: %v", err)
			}
			if err := adminModule.setTitle(cmdCtx); err != ext.EndGroups {
				t.Fatalf("setTitle(%s) error = %v, want EndGroups", tt.name, err)
			}
		})
	}

	longTitleCtx := newModuleMessageContext(bot, chat, admin, "/title 42 VeryLongCustomAdminTitle")
	cmdCtx, err := helpers.BuildCommandContext(bot, longTitleCtx)
	if err != nil {
		t.Fatalf("BuildCommandContext failed: %v", err)
	}
	if err := adminModule.setTitle(cmdCtx); err != ext.EndGroups {
		t.Fatalf("setTitle(long title) error = %v, want EndGroups", err)
	}

	calls := client.callsFor("setChatAdministratorCustomTitle")
	if len(calls) != 1 {
		t.Fatalf("setChatAdministratorCustomTitle calls = %d, want only long-title success", len(calls))
	}
	assertParams(t, calls[0], map[string]any{"chat_id": chat.Id, "user_id": 42, "custom_title": "VeryLongCustomAd"})
}

func TestGetInviteLinkUsesPublicUsernameWithoutFetchingChat(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, user := f.client, f.bot, f.chat, f.admin
	chat.Username = "public_chat"
	ctx := newModuleMessageContext(bot, chat, user, "/invitelink")
	cmdCtx, err := helpers.BuildCommandContext(bot, ctx)
	if err != nil {
		t.Fatalf("BuildCommandContext failed: %v", err)
	}

	if err := adminModule.getinvitelink(cmdCtx); err != ext.EndGroups {
		t.Fatalf("getinvitelink() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("getChat"); len(calls) != 0 {
		t.Fatalf("getChat calls = %d, want 0 for public username", len(calls))
	}
	if text := fmt.Sprint(client.onlyCall(t, "sendMessage").Params["text"]); !strings.Contains(text, chat.Username) {
		t.Fatalf("invite reply lacks the public username %q:\n%s", chat.Username, text)
	}
}

func TestGetInviteLinkFetchesPrivateInviteLink(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, user := f.client, f.bot, f.chat, f.admin
	client.chatsByID[fmt.Sprint(chat.Id)] = []byte(fmt.Sprintf(
		`{"id":%d,"type":"supergroup","title":"Admin Chat","invite_link":"https://t.me/+invite"}`, chat.Id,
	))
	ctx := newModuleMessageContext(bot, chat, user, "/invitelink")
	cmdCtx, err := helpers.BuildCommandContext(bot, ctx)
	if err != nil {
		t.Fatalf("BuildCommandContext failed: %v", err)
	}

	if err := adminModule.getinvitelink(cmdCtx); err != ext.EndGroups {
		t.Fatalf("getinvitelink(private) error = %v, want EndGroups", err)
	}
	assertParams(t, client.onlyCall(t, "getChat"), map[string]any{"chat_id": chat.Id})
	if text := fmt.Sprint(client.onlyCall(t, "sendMessage").Params["text"]); !strings.Contains(text, "https://t.me/+invite") {
		t.Fatalf("invite reply lacks the fetched invite link:\n%s", text)
	}
}

func TestGetInviteLinkHandlesPrivateLookupFailure(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, user := f.client, f.bot, f.chat, f.admin
	client.errors["getChat"] = errors.New("telegram unavailable")
	ctx := newModuleMessageContext(bot, chat, user, "/invitelink")
	cmdCtx, err := helpers.BuildCommandContext(bot, ctx)
	if err != nil {
		t.Fatalf("BuildCommandContext failed: %v", err)
	}

	if err := adminModule.getinvitelink(cmdCtx); err != ext.EndGroups {
		t.Fatalf("getinvitelink(lookup failure) error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want lookup-error reply", len(calls))
	}
}

func TestAnonAdminOwnerTogglesSetting(t *testing.T) {
	f := newAdminChatFixture(t)
	f.makeOwner()
	bot, chat, user := f.bot, f.chat, f.admin

	onCtx := newModuleMessageContext(bot, chat, user, "/anonadmin on")
	cmdCtx, err := helpers.BuildCommandContext(bot, onCtx)
	if err != nil {
		t.Fatalf("BuildCommandContext failed: %v", err)
	}
	if err := adminModule.anonAdmin(cmdCtx); err != ext.EndGroups {
		t.Fatalf("anonAdmin(on) error = %v, want EndGroups", err)
	}
	if !admin.GetAdminSettings(chat.Id).AnonAdmin {
		t.Fatal("AnonAdmin was not enabled")
	}

	statusCtx := newModuleMessageContext(bot, chat, user, "/anonadmin")
	cmdCtx, err = helpers.BuildCommandContext(bot, statusCtx)
	if err != nil {
		t.Fatalf("BuildCommandContext failed: %v", err)
	}
	if err := adminModule.anonAdmin(cmdCtx); err != ext.EndGroups {
		t.Fatalf("anonAdmin(status) error = %v, want EndGroups", err)
	}

	offCtx := newModuleMessageContext(bot, chat, user, "/anonadmin false")
	cmdCtx, err = helpers.BuildCommandContext(bot, offCtx)
	if err != nil {
		t.Fatalf("BuildCommandContext failed: %v", err)
	}
	if err := adminModule.anonAdmin(cmdCtx); err != ext.EndGroups {
		t.Fatalf("anonAdmin(off) error = %v, want EndGroups", err)
	}
	if admin.GetAdminSettings(chat.Id).AnonAdmin {
		t.Fatal("AnonAdmin stayed enabled after false")
	}
}

func TestAnonAdminRefusesNonOwnerAdmin(t *testing.T) {
	f := newAdminChatFixture(t)
	bot, chat, user := f.bot, f.chat, f.admin

	cmdCtx, err := helpers.BuildCommandContext(bot, newModuleMessageContext(bot, chat, user, "/anonadmin on"))
	if err != nil {
		t.Fatalf("BuildCommandContext failed: %v", err)
	}
	if err := adminModule.anonAdmin(cmdCtx); err != ext.EndGroups {
		t.Fatalf("anonAdmin(on) error = %v, want EndGroups", err)
	}
	if admin.GetAdminSettings(chat.Id).AnonAdmin {
		t.Fatal("a non-owner admin enabled AnonAdmin")
	}
}

func TestAnonAdminHandlesNoopAndInvalidOptions(t *testing.T) {
	f := newAdminChatFixture(t)
	f.makeOwner()
	client, bot, chat, user := f.client, f.bot, f.chat, f.admin
	t.Cleanup(func() {
		if err := admin.SetAnonAdminMode(chat.Id, false); err != nil {
			t.Fatalf("cleanup SetAnonAdminMode(false) error = %v", err)
		}
	})

	if err := admin.SetAnonAdminMode(chat.Id, true); err != nil {
		t.Fatalf("SetAnonAdminMode(true) error = %v", err)
	}
	onAgainCtx := newModuleMessageContext(bot, chat, user, "/anonadmin yes")
	cmdCtx, err := helpers.BuildCommandContext(bot, onAgainCtx)
	if err != nil {
		t.Fatalf("BuildCommandContext failed: %v", err)
	}
	if err := adminModule.anonAdmin(cmdCtx); err != ext.EndGroups {
		t.Fatalf("anonAdmin(already on) error = %v, want EndGroups", err)
	}

	if err := admin.SetAnonAdminMode(chat.Id, false); err != nil {
		t.Fatalf("SetAnonAdminMode(false) error = %v", err)
	}
	offAgainCtx := newModuleMessageContext(bot, chat, user, "/anonadmin off")
	cmdCtx, err = helpers.BuildCommandContext(bot, offAgainCtx)
	if err != nil {
		t.Fatalf("BuildCommandContext failed: %v", err)
	}
	if err := adminModule.anonAdmin(cmdCtx); err != ext.EndGroups {
		t.Fatalf("anonAdmin(already off) error = %v, want EndGroups", err)
	}

	invalidCtx := newModuleMessageContext(bot, chat, user, "/anonadmin maybe")
	cmdCtx, err = helpers.BuildCommandContext(bot, invalidCtx)
	if err != nil {
		t.Fatalf("BuildCommandContext failed: %v", err)
	}
	if err := adminModule.anonAdmin(cmdCtx); err != ext.EndGroups {
		t.Fatalf("anonAdmin(invalid) error = %v, want EndGroups", err)
	}

	if calls := client.callsFor("sendMessage"); len(calls) != 3 {
		t.Fatalf("sendMessage calls = %d, want one response per anonadmin branch", len(calls))
	}
}

func TestAdminCacheCommandsRefreshAndClearCache(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, user := f.client, f.bot, f.chat, f.admin

	refreshCtx := newModuleMessageContext(bot, chat, user, "/admincache")
	cmdCtx, err := helpers.BuildCommandContext(bot, refreshCtx)
	if err != nil {
		t.Fatalf("BuildCommandContext failed: %v", err)
	}
	if err := adminModule.adminCache(cmdCtx); err != ext.EndGroups {
		t.Fatalf("adminCache() error = %v, want EndGroups", err)
	}
	assertParams(t, client.onlyCall(t, "getChatAdministrators"), map[string]any{"chat_id": chat.Id})
	if found, admins := cache.GetAdminCacheList(chat.Id); !found || len(admins.UserInfo) != 2 {
		t.Fatalf("admin cache after /admincache = (%v, %d entries), want the bot and the admin", found, len(admins.UserInfo))
	}

	clearChat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Admin Chat"}
	m := cache.GetMarshal()
	if m == nil {
		t.Fatal("cache marshaler is not initialised; TestMain must set it")
	}
	if err := m.Set(cache.Context, "alita:cache:adminCache:"+fmtInt(clearChat.Id), cache.AdminCache{
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
	cmdCtx, err = helpers.BuildCommandContext(bot, clearCtx)
	if err != nil {
		t.Fatalf("BuildCommandContext failed: %v", err)
	}
	if err := adminModule.clearAdminCache(cmdCtx); err != ext.EndGroups {
		t.Fatalf("clearAdminCache() error = %v, want EndGroups", err)
	}
	if found, _ := cache.GetAdminCacheList(clearChat.Id); found {
		t.Fatal("admin cache entry remained after /clearadmincache")
	}
}

func TestClearAdminCacheNilMarshal(t *testing.T) {
	withNilCacheMarshal(t)

	f := newAdminChatFixture(t)
	client, bot, chat, user := f.client, f.bot, f.chat, f.admin
	ctx := newModuleMessageContext(bot, chat, user, "/clearadmincache")
	cmdCtx, err := helpers.BuildCommandContext(bot, ctx)
	if err != nil {
		t.Fatalf("BuildCommandContext failed: %v", err)
	}

	if err := adminModule.clearAdminCache(cmdCtx); err != ext.EndGroups {
		t.Fatalf("clearAdminCache(nil marshal) error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 0 {
		t.Fatalf("sendMessage calls = %d, want none when cache marshal is nil", len(calls))
	}
}

func TestAdminCacheHandlesMemberAndLookupFailures(t *testing.T) {
	memberClient := newModuleBotClient()
	memberBot := newModuleTestBot(memberClient)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Admin Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	memberCtx := newModuleMessageContext(memberBot, chat, member, "/admincache")
	cmdCtx, err := helpers.BuildCommandContext(memberBot, memberCtx)
	if err != nil {
		t.Fatalf("BuildCommandContext failed: %v", err)
	}
	if err := adminModule.adminCache(cmdCtx); err != ext.EndGroups {
		t.Fatalf("adminCache(member) error = %v, want EndGroups", err)
	}
	if calls := memberClient.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want non-admin denial", len(calls))
	}

	f := newAdminChatFixture(t)
	errorClient, errorBot, admin := f.client, f.bot, f.admin
	errorClient.errors["getChatMember"] = errors.New("telegram unavailable")
	errorCtx := newModuleMessageContext(errorBot, f.chat, admin, "/admincache")
	cmdCtx, err = helpers.BuildCommandContext(errorBot, errorCtx)
	if err != nil {
		t.Fatalf("BuildCommandContext failed: %v", err)
	}
	if err := adminModule.adminCache(cmdCtx); err != ext.EndGroups {
		t.Fatalf("adminCache(lookup failure) error = %v, want EndGroups", err)
	}
	if calls := errorClient.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want lookup-failure denial", len(calls))
	}
	if calls := errorClient.callsFor("getChatAdministrators"); len(calls) != 0 {
		t.Fatalf("getChatAdministrators calls = %d, want no reload after the lookup failed", len(calls))
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
		run       func(*helpers.CommandContext) error
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

			cmdCtx, err := helpers.BuildCommandContext(bot, ctx)
			if err != nil {
				t.Fatalf("BuildCommandContext failed: %v", err)
			}
			err = tt.run(cmdCtx)
			if !errors.Is(err, requestErr) {
				t.Fatalf("%s returned error %v, want request error", tt.text, err)
			}
		})
	}
}

func TestSetGroupTitlePicAndDescription(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, admin := f.client, f.bot, f.chat, f.admin

	missingTitle, err := helpers.BuildCommandContext(bot, newModuleMessageContext(bot, chat, admin, "/setgtitle"))
	if err != nil {
		t.Fatalf("BuildCommandContext: %v", err)
	}
	if err := adminModule.setGTitle(missingTitle); err != ext.EndGroups {
		t.Fatalf("setGTitle missing: %v", err)
	}

	okTitle, err := helpers.BuildCommandContext(bot, newModuleMessageContext(bot, chat, admin, "/setgtitle New Title"))
	if err != nil {
		t.Fatalf("BuildCommandContext: %v", err)
	}
	if err := adminModule.setGTitle(okTitle); err != ext.EndGroups {
		t.Fatalf("setGTitle: %v", err)
	}
	assertParams(t, client.onlyCall(t, "setChatTitle"), map[string]any{"chat_id": chat.Id, "title": "New Title"})

	okDesc, err := helpers.BuildCommandContext(bot, newModuleMessageContext(bot, chat, admin, "/setgdesc Hello group"))
	if err != nil {
		t.Fatalf("BuildCommandContext: %v", err)
	}
	if err := adminModule.setGDesc(okDesc); err != ext.EndGroups {
		t.Fatalf("setGDesc: %v", err)
	}
	assertParams(t, client.onlyCall(t, "setChatDescription"), map[string]any{"chat_id": chat.Id, "description": "Hello group"})

	needPhoto, err := helpers.BuildCommandContext(bot, newModuleMessageContext(bot, chat, admin, "/setgpic"))
	if err != nil {
		t.Fatalf("BuildCommandContext: %v", err)
	}
	if err := adminModule.setGPic(needPhoto); err != ext.EndGroups {
		t.Fatalf("setGPic missing: %v", err)
	}

	picCtx := newModuleMessageContext(bot, chat, admin, "/setgpic")
	picCtx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
		MessageId: 55,
		Date:      1,
		Chat:      chat,
		Photo:     []gotgbot.PhotoSize{{FileId: "photo-1", FileUniqueId: "u1", Width: 64, Height: 64}},
	}
	picCmd, err := helpers.BuildCommandContext(bot, picCtx)
	if err != nil {
		t.Fatalf("BuildCommandContext: %v", err)
	}
	if err := adminModule.setGPic(picCmd); err != ext.EndGroups {
		t.Fatalf("setGPic: %v", err)
	}
}

func fmtInt(value int64) string {
	return strconv.FormatInt(value, 10)
}
