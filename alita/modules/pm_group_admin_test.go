//go:build testtools

package modules

import (
	"encoding/json"
	"fmt"
	"html"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/chattz"
	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

func adminPMDispatcher() *ext.Dispatcher {
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadAdmin(dispatcher)
	return dispatcher
}

// staleAdminID is in every admin list seeded by pmAdminFixture and in none
// Telegram returns, so a reloaded list lacks it.
const staleAdminID = 4242

// inviteLinkOf is the invite link getChat returns for chatID.
func inviteLinkOf(chatID int64) string {
	return fmt.Sprintf("https://t.me/+invite%d", -chatID)
}

// adminMember is an admin of a chat with every right.
func adminMember(userID int64, isBot bool) gotgbot.MergedChatMember {
	return gotgbot.MergedChatMember{
		Status: "administrator", User: gotgbot.User{Id: userID, IsBot: isBot, FirstName: "Admin"},
		CanManageChat: true, CanDeleteMessages: true, CanRestrictMembers: true, CanPromoteMembers: true,
		CanChangeInfo: true, CanInviteUsers: true, CanPinMessages: true,
	}
}

// seedAdminCache stores members as chatID's cached admin list.
func seedAdminCache(t *testing.T, chatID int64, members ...gotgbot.MergedChatMember) {
	t.Helper()
	if err := cache.GetMarshal().Set(cache.Context, fmt.Sprintf("alita:cache:adminCache:%d", chatID),
		cache.AdminCache{ChatId: chatID, UserInfo: members, Cached: true}); err != nil {
		t.Fatalf("seed admin cache of %d: %v", chatID, err)
	}
}

// pmAdminFixture is pmSettingsFixture's group (@fixturegroup, private, with
// an invite link) and the group its admin is connected to, "Group B". The
// admin lists of both groups and of the admin's PM are cached with
// staleAdminID in them; in both groups the bot and the admin hold every
// right.
func pmAdminFixture(t *testing.T) (f adminChatFixture, connected int64) {
	t.Helper()
	f, connected = pmSettingsFixture(t)
	for id, title := range map[int64]string{f.chat.Id: f.chat.Title, connected: "Group B"} {
		f.client.chatsByID[fmt.Sprint(id)] = json.RawMessage(fmt.Sprintf(
			`{"id":%d,"type":"supergroup","title":%q,"invite_link":%q}`, id, title, inviteLinkOf(id)))
	}
	f.client.chatsByID["@fixturegroup"] = f.client.chatsByID[fmt.Sprint(f.chat.Id)]
	f.client.adminsByChat[fmt.Sprint(connected)] = adminListJSON(f.admin.Id)
	if f.client.membersByChat == nil {
		f.client.membersByChat = map[string]json.RawMessage{}
	}
	stale := gotgbot.MergedChatMember{Status: "administrator", User: gotgbot.User{Id: staleAdminID, FirstName: "Stale"}}
	for _, id := range []int64{f.chat.Id, connected} {
		seedAdminCache(t, id, adminMember(999, true), adminMember(f.admin.Id, false), stale)
	}
	seedAdminCache(t, f.admin.Id, stale)
	t.Cleanup(func() {
		cache.InvalidateAdminCache(connected)
		cache.InvalidateAdminCache(f.admin.Id)
	})
	return f, connected
}

// chatTitleOf returns the title getChat returns for chatID.
func chatTitleOf(f adminChatFixture, chatID int64) string {
	var chat gotgbot.Chat
	_ = json.Unmarshal(f.client.chatsByID[fmt.Sprint(chatID)], &chat)
	return chat.Title
}

// repliedIn reports whether a message containing want was sent to chatID.
func repliedIn(f adminChatFixture, chatID int64, want string) bool {
	for _, call := range f.client.callsFor("sendMessage") {
		if fmt.Sprint(call.Params["chat_id"]) == fmt.Sprint(chatID) && strings.Contains(fmt.Sprint(call.Params["text"]), want) {
			return true
		}
	}
	return false
}

// calledFor reports whether method was called with chatID.
func calledFor(f adminChatFixture, method string, chatID int64) bool {
	for _, call := range f.client.callsFor(method) {
		if fmt.Sprint(call.Params["chat_id"]) == fmt.Sprint(chatID) {
			return true
		}
	}
	return false
}

// pmAdminCase is an admin command that acts on one group. acted reports
// whether it acted on chatID: changed it, sent its admins or invite link to
// replyTo, or reloaded or cleared its cached admin list.
type pmAdminCase struct {
	cmd, args string
	acted     func(t *testing.T, f adminChatFixture, chatID, replyTo int64) bool
}

var pmAdminCases = []pmAdminCase{
	{"adminlist", "", func(t *testing.T, f adminChatFixture, id, replyTo int64) bool {
		return repliedIn(f, replyTo, fmt.Sprintf(translated(t, "admin_adminlist"), html.EscapeString(chatTitleOf(f, id))))
	}},
	{"invitelink", "", func(_ *testing.T, f adminChatFixture, id, replyTo int64) bool {
		return repliedIn(f, replyTo, inviteLinkOf(id))
	}},
	{"setgtitle", " New Title", func(_ *testing.T, f adminChatFixture, id, _ int64) bool {
		return calledFor(f, "setChatTitle", id)
	}},
	{"setgdesc", " New description", func(_ *testing.T, f adminChatFixture, id, _ int64) bool {
		return calledFor(f, "setChatDescription", id)
	}},
	{"timezone", " Asia/Kolkata", func(_ *testing.T, _ adminChatFixture, id, _ int64) bool {
		name, err := chattz.Name(id)
		return err == nil && name == "Asia/Kolkata"
	}},
	{"admincache", "", func(_ *testing.T, _ adminChatFixture, id, _ int64) bool {
		found, admins := cache.GetAdminCacheList(id)
		if !found {
			return false
		}
		for _, admin := range admins.UserInfo {
			if admin.User.Id == staleAdminID {
				return false
			}
		}
		return true
	}},
	{"clearadmincache", "", func(_ *testing.T, _ adminChatFixture, id, _ int64) bool {
		found, _ := cache.GetAdminCacheList(id)
		return !found
	}},
}

// From PM with @fixturegroup while connected to another group, each admin
// command acts on the named group and answers in the PM; the connected group,
// the PM's own admin cache and the connection stay as they were.
func TestAdminCommandsInPMWithGroupActOnThatGroup(t *testing.T) {
	for _, tc := range pmAdminCases {
		t.Run(tc.cmd, func(t *testing.T) {
			f, connected := pmAdminFixture(t)

			processPM(t, adminPMDispatcher(), f, f.admin.Id, "/"+tc.cmd+" @fixturegroup"+tc.args)

			if !tc.acted(t, f, f.chat.Id, f.admin.Id) {
				t.Fatalf("/%s from PM did not act on the named group; sent:\n%s", tc.cmd, sentText(f.client))
			}
			if tc.acted(t, f, connected, f.admin.Id) {
				t.Fatalf("/%s from PM acted on the connected group", tc.cmd)
			}
			if tc.acted(t, f, f.admin.Id, f.admin.Id) {
				t.Fatalf("/%s from PM acted on the PM", tc.cmd)
			}
			if !calledFor(f, "sendMessage", f.admin.Id) || calledFor(f, "sendMessage", f.chat.Id) {
				t.Fatalf("/%s from PM answered elsewhere than in the PM; sent:\n%s", tc.cmd, sentText(f.client))
			}
			requireConnectedTo(t, f.admin.Id, connected)
		})
	}
}

// From PM, a member of the named group who is no admin there gets a refusal
// and the group stays as it was; so does their connection. /admincache checks
// admin rights itself, live, and refuses with its own message.
func TestAdminCommandsInPMWithGroupRefuseMember(t *testing.T) {
	refusals := map[string]string{"admincache": "admin_need_admin"}
	for _, tc := range pmAdminCases {
		t.Run(tc.cmd, func(t *testing.T) {
			f, connected := pmAdminFixture(t)
			if err := connections.ToggleAllowConnect(f.chat.Id, true); err != nil {
				t.Fatalf("ToggleAllowConnect() error = %v", err)
			}
			if err := connections.ConnectId(f.member.Id, connected); err != nil {
				t.Fatalf("ConnectId() error = %v", err)
			}
			t.Cleanup(func() { _ = connections.DisconnectId(f.member.Id) })

			processPM(t, adminPMDispatcher(), f, f.member.Id, "/"+tc.cmd+" @fixturegroup"+tc.args)

			if tc.acted(t, f, f.chat.Id, f.member.Id) {
				t.Fatalf("/%s from a member who is no admin acted on the group", tc.cmd)
			}
			refusal, ok := refusals[tc.cmd]
			if !ok {
				refusal = "connections_is_user_connected_user_not_admin"
			}
			if !repliedIn(f, f.member.Id, translated(t, refusal)) {
				t.Fatalf("/%s from a member who is no admin did not get %s; sent:\n%s", tc.cmd, refusal, sentText(f.client))
			}
			requireConnectedTo(t, f.member.Id, connected)
		})
	}
}

// From PM, an admin of the named group who lacks the right the command needs
// there gets the refusal the group would give, though they hold that right in
// the group they are connected to.
func TestAdminCommandsInPMWithGroupRefuseAdminWithoutRight(t *testing.T) {
	for _, tc := range []struct {
		cmd, args, refusal string
		strip              func(*gotgbot.MergedChatMember)
	}{
		{"setgtitle", " New Title", "chat_status_change_info_cmd_error", func(m *gotgbot.MergedChatMember) { m.CanChangeInfo = false }},
		{"setgdesc", " New description", "chat_status_change_info_cmd_error", func(m *gotgbot.MergedChatMember) { m.CanChangeInfo = false }},
		{"timezone", " Asia/Kolkata", "chat_status_change_info_cmd_error", func(m *gotgbot.MergedChatMember) { m.CanChangeInfo = false }},
		{"invitelink", "", "chat_status_invite_link_user_error", func(m *gotgbot.MergedChatMember) { m.CanInviteUsers = false }},
	} {
		t.Run(tc.cmd, func(t *testing.T) {
			f, connected := pmAdminFixture(t)
			admin := adminMember(f.admin.Id, false)
			tc.strip(&admin)
			seedAdminCache(t, f.chat.Id, adminMember(999, true), admin)

			processPM(t, adminPMDispatcher(), f, f.admin.Id, "/"+tc.cmd+" @fixturegroup"+tc.args)

			timezone, err := chattz.Name(f.chat.Id)
			if err != nil {
				t.Fatalf("chattz.Name() error = %v", err)
			}
			if calledFor(f, "setChatTitle", f.chat.Id) || calledFor(f, "setChatDescription", f.chat.Id) ||
				repliedIn(f, f.admin.Id, inviteLinkOf(f.chat.Id)) || timezone != chattz.DefaultName {
				t.Fatalf("/%s from an admin without the right acted on the group", tc.cmd)
			}
			if !repliedIn(f, f.admin.Id, translated(t, tc.refusal)) {
				t.Fatalf("/%s from an admin without the right: want %q; sent:\n%s", tc.cmd, tc.refusal, sentText(f.client))
			}
			requireConnectedTo(t, f.admin.Id, connected)
		})
	}
}

// From PM, a command the bot cannot carry out in the named group gets the same
// answer it gets in that group.
func TestAdminCommandsInPMWithGroupAnswerAsGroupWhenBotLacksRight(t *testing.T) {
	notAdmin := func(f adminChatFixture) {
		f.client.membersByChat[fmt.Sprintf("%d|999", f.chat.Id)] = json.RawMessage(
			`{"status":"member","user":{"id":999,"is_bot":true,"first_name":"Alita"}}`)
	}
	for _, tc := range []struct {
		cmd, args string
		// bot is the bot's entry in the group's admin list; nil drops it.
		bot   func() *gotgbot.MergedChatMember
		live  func(f adminChatFixture)
		check string
	}{
		{"setgtitle", " New Title", func() *gotgbot.MergedChatMember {
			m := adminMember(999, true)
			m.CanChangeInfo = false
			return &m
		}, func(adminChatFixture) {}, "chat_status_bot_change_info_error"},
		{"setgdesc", " New description", func() *gotgbot.MergedChatMember {
			m := adminMember(999, true)
			m.CanChangeInfo = false
			return &m
		}, func(adminChatFixture) {}, "chat_status_bot_change_info_error"},
		{"invitelink", "", func() *gotgbot.MergedChatMember {
			m := adminMember(999, true)
			m.CanInviteUsers = false
			return &m
		}, func(f adminChatFixture) {
			f.client.membersByChat[fmt.Sprintf("%d|999", f.chat.Id)] = json.RawMessage(
				`{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"},"can_change_info":true}`)
		}, "chat_status_invite_link_bot_error"},
		{"adminlist", "", func() *gotgbot.MergedChatMember { return nil }, notAdmin, "chat_status_bot_not_admin"},
		{"admincache", "", func() *gotgbot.MergedChatMember { return nil }, notAdmin, "chat_status_bot_not_admin"},
		{"clearadmincache", "", func() *gotgbot.MergedChatMember { return nil }, notAdmin, "chat_status_bot_not_admin"},
	} {
		t.Run(tc.cmd, func(t *testing.T) {
			f, connected := pmAdminFixture(t)
			setup := func() {
				members := []gotgbot.MergedChatMember{adminMember(f.admin.Id, false)}
				if bot := tc.bot(); bot != nil {
					members = append(members, *bot)
				}
				seedAdminCache(t, f.chat.Id, members...)
				tc.live(f)
			}
			dispatcher := adminPMDispatcher()

			setup()
			update := newModuleMessageContext(f.bot, f.chat, f.admin, "/"+tc.cmd+tc.args).Update
			if err := dispatcher.ProcessUpdate(f.bot, update, nil); err != nil {
				t.Fatalf("ProcessUpdate(/%s) error = %v", tc.cmd, err)
			}
			inGroup := f.client.callsFor("sendMessage")
			if len(inGroup) != 1 || !strings.Contains(fmt.Sprint(inGroup[0].Params["text"]), translated(t, tc.check)) {
				t.Fatalf("/%s in the group: want one %q reply; sent:\n%s", tc.cmd, tc.check, sentText(f.client))
			}

			setup()
			processPM(t, dispatcher, f, f.admin.Id, "/"+tc.cmd+" @fixturegroup"+tc.args)

			inPM := f.client.callsFor("sendMessage")[1:]
			if len(inPM) != 1 || fmt.Sprint(inPM[0].Params["text"]) != fmt.Sprint(inGroup[0].Params["text"]) {
				t.Fatalf("/%s from PM answered differently from the group; sent:\n%s", tc.cmd, sentText(f.client))
			}
			if fmt.Sprint(inPM[0].Params["chat_id"]) != fmt.Sprint(f.admin.Id) {
				t.Fatalf("/%s from PM answered in chat %v, want the PM", tc.cmd, inPM[0].Params["chat_id"])
			}
			if calledFor(f, "setChatTitle", f.chat.Id) || calledFor(f, "setChatDescription", f.chat.Id) {
				t.Fatalf("/%s from PM changed the group the bot has no right in", tc.cmd)
			}
			requireConnectedTo(t, f.admin.Id, connected)
		})
	}
}

// From PM, /admincache checks the caller live in the named group, not in the
// PM: an admin the group's cache still lists, but Telegram no longer does,
// gets the group's refusal and the cache stays as it was.
func TestAdminCacheInPMWithGroupRefusesRemovedAdmin(t *testing.T) {
	f, _ := pmAdminFixture(t)
	f.setMemberStatus(f.chat.Id, f.admin.Id, "member")

	processPM(t, adminPMDispatcher(), f, f.admin.Id, "/admincache @fixturegroup")

	if !repliedIn(f, f.admin.Id, translated(t, "admin_need_admin")) {
		t.Fatalf("/admincache from a removed admin: want admin_need_admin; sent:\n%s", sentText(f.client))
	}
	if calledFor(f, "getChatAdministrators", f.chat.Id) {
		t.Fatal("/admincache from a removed admin reloaded the group's admin cache")
	}
}

// From PM, /admincache lets in an admin promoted after the group's admin list
// was cached, as it does in the group: the live check decides, not the cache
// the command is there to reload.
func TestAdminCacheInPMWithGroupAcceptsNewlyPromotedAdmin(t *testing.T) {
	f, _ := pmAdminFixture(t)
	seedAdminCache(t, f.chat.Id, adminMember(999, true))
	f.setMemberStatus(f.chat.Id, f.admin.Id, "administrator")

	processPM(t, adminPMDispatcher(), f, f.admin.Id, "/admincache @fixturegroup")

	if !repliedIn(f, f.admin.Id, translated(t, "commonstrings_admin_cache_cache_reloaded")) {
		t.Fatalf("/admincache from a newly promoted admin: want commonstrings_admin_cache_cache_reloaded; sent:\n%s", sentText(f.client))
	}
	if !calledFor(f, "getChatAdministrators", f.chat.Id) {
		t.Fatal("/admincache from a newly promoted admin did not reload the group's admin cache")
	}
}

// Every admin command sent from PM without a group and without a connection
// answers with its help, its example and the user's groups.
func TestAdminCommandsInPMWithoutGroupReplyHelp(t *testing.T) {
	f := titleFixture(t, "My Group")
	dispatcher := adminPMDispatcher()
	pick := translated(t, "connections_pm_group_pick")
	for _, tc := range pmAdminCases {
		t.Run(tc.cmd, func(t *testing.T) {
			before := len(f.client.callsFor("sendMessage"))

			processPM(t, dispatcher, f, f.admin.Id, "/"+tc.cmd)

			replies := f.client.callsFor("sendMessage")[before:]
			if len(replies) != 1 || !strings.Contains(fmt.Sprint(replies[0].Params["text"]), pick) {
				t.Fatalf("/%s from PM replied %+v, want one reply asking for the group", tc.cmd, replies)
			}
			assertPMGroupHelp(t, fmt.Sprint(replies[0].Params["text"]), tc.cmd, f.chat)
		})
	}
}

// In the group, each admin command acts on that group and answers there.
func TestAdminCommandsInGroupActOnGroup(t *testing.T) {
	for _, tc := range pmAdminCases {
		t.Run(tc.cmd, func(t *testing.T) {
			f, _ := pmAdminFixture(t)

			update := newModuleMessageContext(f.bot, f.chat, f.admin, "/"+tc.cmd+tc.args).Update
			if err := adminPMDispatcher().ProcessUpdate(f.bot, update, nil); err != nil {
				t.Fatalf("ProcessUpdate(/%s) error = %v", tc.cmd, err)
			}

			if !tc.acted(t, f, f.chat.Id, f.chat.Id) {
				t.Fatalf("/%s in the group did not act on it; sent:\n%s", tc.cmd, sentText(f.client))
			}
		})
	}
}
