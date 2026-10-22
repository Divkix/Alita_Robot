//go:build testtools

package modules

import (
	"encoding/json"
	"fmt"
	"html"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/db"
	dbcache "github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/disabling"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/warns"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

// pmWarnsStatusFixture is pmWarnsFixture with @victim holding two warns in
// the fixture's group and one in the connected group, so a reply shows which
// group it read.
func pmWarnsStatusFixture(t *testing.T) (f adminChatFixture, connected int64) {
	t.Helper()
	f, connected = pmWarnsFixture(t, 0)
	for chatID, num := range map[int64]int{f.chat.Id: 2, connected: 1} {
		for range num {
			if _, _, err := warns.WarnUser(victimID, chatID, models.WarnEntry{Reason: "seed"}); err != nil {
				t.Fatalf("WarnUser() error = %v", err)
			}
		}
	}
	return f, connected
}

// warnsCountText is the head of a /warns reply listing num warns under the
// default limit.
func warnsCountText(t *testing.T, num int) string {
	t.Helper()
	return fmt.Sprintf(translated(t, "warns_user_warnings_list"), num, 3)
}

// From PM, /warns with a group shows the user's warns in that group, never in
// the connected one. A member of the group who is no admin sees them as they
// would in the group.
func TestWarnsInPMWithGroupShowsWarnsInThatGroup(t *testing.T) {
	for _, tc := range []struct {
		name   string
		member bool
	}{
		{"admin", false},
		{"member", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, connected := pmWarnsStatusFixture(t)
			caller := f.admin
			if tc.member {
				caller = f.member
			}

			processPM(t, warnsPMDispatcher(), f, caller.Id, "/warns @fixturegroup @victim")

			if !repliedIn(f, caller.Id, warnsCountText(t, 2)) || repliedIn(f, caller.Id, warnsCountText(t, 1)) {
				t.Fatalf("/warns from PM by the %s: want the 2 warns of the named group, not the connected one's 1; sent:\n%s", tc.name, sentText(f.client))
			}
			if want := groupLine(t, chatTitleOf(f, f.chat.Id), f.chat.Id); !repliedIn(f, caller.Id, want) {
				t.Fatalf("/warns from PM by the %s did not name the group %q; sent:\n%s", tc.name, want, sentText(f.client))
			}
			requireNothingSentTo(t, f, f.chat.Id)
			requireConnectedTo(t, f.admin.Id, connected)
		})
	}
}

// From PM, /warns names the group also when the user has no warns there, and
// when their warns there have no reasons.
func TestWarnsInPMWithGroupNamesGroupWithoutWarnsOrReasons(t *testing.T) {
	for _, tc := range []struct {
		name string
		seed int
		want func(t *testing.T) string
	}{
		{"no warns", 0, func(t *testing.T) string { return translated(t, "warns_user_no_warnings") }},
		{"no reasons", 2, func(t *testing.T) string { return fmt.Sprintf(translated(t, "warns_user_warnings_no_reasons"), 2, 3) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, _ := pmWarnsFixture(t, 0)
			if tc.seed > 0 {
				// A row from before warn reasons were kept: warns, no reasons.
				if err := db.DB.Create(&models.Warns{UserId: victimID, ChatId: f.chat.Id, NumWarns: tc.seed}).Error; err != nil {
					t.Fatalf("create warns without reasons: %v", err)
				}
				dbcache.ResetLocalForTest()
			}

			processPM(t, warnsPMDispatcher(), f, f.admin.Id, "/warns @fixturegroup @victim")

			if !repliedIn(f, f.admin.Id, tc.want(t)) {
				t.Fatalf("/warns from PM with %s: want %q; sent:\n%s", tc.name, tc.want(t), sentText(f.client))
			}
			if want := groupLine(t, chatTitleOf(f, f.chat.Id), f.chat.Id); !repliedIn(f, f.admin.Id, want) {
				t.Fatalf("/warns from PM with %s did not name the group %q; sent:\n%s", tc.name, want, sentText(f.client))
			}
		})
	}
}

// From PM, /warns with a group and no user answers as it does in the group
// without one, with its help showing a call with a group. In the group it
// answers as before, with the no-user error alone.
func TestWarnsInPMWithGroupWithoutUserRepliesHelp(t *testing.T) {
	f, _ := pmWarnsStatusFixture(t)
	useRealLocales(t)

	processPM(t, warnsPMDispatcher(), f, f.admin.Id, "/warns @fixturegroup")

	pmExample, _ := i18n.MustNewTranslator("config").GetString("warns_pm_example")
	calls := f.client.callsFor("sendMessage")
	if len(calls) != 1 || fmt.Sprint(calls[0].Params["chat_id"]) != fmt.Sprint(f.admin.Id) {
		t.Fatalf("/warns @fixturegroup from PM: want one reply in the PM; sent:\n%s", sentText(f.client))
	}
	text := fmt.Sprint(calls[0].Params["text"])
	if pmExample == "" || !strings.Contains(text, translated(t, "common_no_user_specified")) || !strings.Contains(text, html.EscapeString(pmExample)) {
		t.Fatalf("/warns @fixturegroup from PM replied %q, want common_no_user_specified with the example %q", text, pmExample)
	}

	update := newModuleMessageContext(f.bot, f.chat, f.admin, "/warns").Update
	if err := warnsPMDispatcher().ProcessUpdate(f.bot, update, nil); err != nil {
		t.Fatalf("ProcessUpdate(/warns) error = %v", err)
	}
	inGroup := f.client.callsFor("sendMessage")[1:]
	if len(inGroup) != 1 || !isNoUserGroupReply(t, f, inGroup[0]) {
		t.Fatalf("/warns in the group without a user: want only an HTML reply %q; sent %+v", translated(t, "common_no_user_specified"), inGroup)
	}
}

// With a connection and no group named, /warns shows warns in the connected
// group and names it.
func TestWarnsInPMOnConnectionShowsConnectedGroup(t *testing.T) {
	f, connected := pmWarnsStatusFixture(t)

	processPM(t, warnsPMDispatcher(), f, f.admin.Id, "/warns @victim")

	if !repliedIn(f, f.admin.Id, warnsCountText(t, 1)) || repliedIn(f, f.admin.Id, warnsCountText(t, 2)) {
		t.Fatalf("/warns from PM on a connection: want the connected group's 1 warn; sent:\n%s", sentText(f.client))
	}
	if want := groupLine(t, chatTitleOf(f, connected), connected); !repliedIn(f, f.admin.Id, want) {
		t.Fatalf("/warns from PM on a connection did not name the connected group %q; sent:\n%s", want, sentText(f.client))
	}
}

// From PM, a user who is no member of the named group sees no warns there.
func TestWarnsInPMWithGroupRefusesNonMember(t *testing.T) {
	f, _ := pmWarnsStatusFixture(t)
	f.setMemberStatus(f.chat.Id, f.member.Id, "left")

	processPM(t, warnsPMDispatcher(), f, f.member.Id, "/warns @fixturegroup @victim")

	if repliedIn(f, f.member.Id, warnsCountText(t, 2)) {
		t.Fatalf("/warns from PM showed a non-member the warns of the named group")
	}
	if !repliedIn(f, f.member.Id, translated(t, "connections_connect_not_member")) {
		t.Fatalf("/warns from PM by a non-member: want connections_connect_not_member; sent:\n%s", sentText(f.client))
	}
}

// A member connected to a group sees its warns from PM on the connection;
// once they have left the group they see none, and the connection is dropped
// as stale.
func TestWarnsInPMOnConnectionToLeftGroupShowsNoWarns(t *testing.T) {
	for _, tc := range []struct {
		status string
		shown  bool
	}{
		{"member", true},
		{"left", false},
	} {
		t.Run(tc.status, func(t *testing.T) {
			f, _ := pmWarnsStatusFixture(t)
			if err := connections.ConnectId(f.member.Id, f.chat.Id); err != nil {
				t.Fatalf("ConnectId() error = %v", err)
			}
			t.Cleanup(func() { _ = connections.DisconnectId(f.member.Id) })
			f.setMemberStatus(f.chat.Id, f.member.Id, tc.status)

			processPM(t, warnsPMDispatcher(), f, f.member.Id, "/warns @victim")

			if shown := repliedIn(f, f.member.Id, warnsCountText(t, 2)); shown != tc.shown {
				t.Fatalf("/warns from PM on a connection by a user with status %q showed the group's warns: %v, want %v; sent:\n%s", tc.status, shown, tc.shown, sentText(f.client))
			}
			if tc.shown {
				return
			}
			if !repliedIn(f, f.member.Id, translated(t, "connections_stale_connection")) {
				t.Fatalf("/warns from PM on a connection to a group left: want connections_stale_connection; sent:\n%s", sentText(f.client))
			}
			if conn := connections.Connection(f.member.Id); conn.Connected {
				t.Fatalf("connection of a user who left the group = %+v, want it dropped", conn)
			}
		})
	}
}

// A reply in PM names no user: /warns with only a group answers as it does
// without one, not with the warns of the user replied to.
func TestWarnsInPMWithGroupIgnoresReply(t *testing.T) {
	f, _ := pmWarnsStatusFixture(t)
	bot := gotgbot.User{Id: 999, IsBot: true, FirstName: "Alita"}

	processPMReply(t, warnsPMDispatcher(), f, f.admin, bot, "/warns @fixturegroup")

	if !repliedIn(f, f.admin.Id, translated(t, "common_no_user_specified")) {
		t.Fatalf("/warns from PM in reply to the bot: want common_no_user_specified; sent:\n%s", sentText(f.client))
	}
}

// /warns disabled in the named group answers a member from PM as it does in
// the group, with nothing; an admin of the group still sees the warns.
func TestWarnsInPMWithGroupDisabledThereRefusesMember(t *testing.T) {
	f, _ := pmWarnsStatusFixture(t)
	if err := disabling.DisableCMD(f.chat.Id, "warns"); err != nil {
		t.Fatalf("DisableCMD() error = %v", err)
	}
	dispatcher := warnsPMDispatcher()

	processPM(t, dispatcher, f, f.member.Id, "/warns @fixturegroup @victim")

	if calledFor(f, "sendMessage", f.member.Id) || len(f.client.callsFor("deleteMessage")) != 0 {
		t.Fatalf("/warns disabled in the named group answered a member from PM; sent:\n%s", sentText(f.client))
	}

	processPM(t, dispatcher, f, f.admin.Id, "/warns @fixturegroup @victim")

	if !repliedIn(f, f.admin.Id, warnsCountText(t, 2)) {
		t.Fatalf("/warns disabled in the named group refused its admin from PM; sent:\n%s", sentText(f.client))
	}
}

// From PM, /warnings with a group shows that group's settings in the PM,
// naming it in HTML; on a connection, the connected group's.
func TestWarningsInPMShowsNamedOrConnectedGroup(t *testing.T) {
	f, connected := pmWarnsFixture(t, 0)
	for chatID, limit := range map[int64]int{f.chat.Id: 5, connected: 7} {
		if err := warns.SetWarnLimit(chatID, limit); err != nil {
			t.Fatalf("SetWarnLimit() error = %v", err)
		}
	}
	dispatcher := warnsPMDispatcher()
	for _, tc := range []struct {
		text         string
		chatID       int64
		limit, other int
	}{
		{"/warnings @fixturegroup", f.chat.Id, 5, 7},
		{"/warnings", connected, 7, 5},
	} {
		before := len(f.client.callsFor("sendMessage"))

		processPM(t, dispatcher, f, f.admin.Id, tc.text)

		replies := f.client.callsFor("sendMessage")[before:]
		text := ""
		if len(replies) == 1 && fmt.Sprint(replies[0].Params["chat_id"]) == fmt.Sprint(f.admin.Id) &&
			fmt.Sprint(replies[0].Params["parse_mode"]) == gotgbot.ParseModeHTML {
			text = fmt.Sprint(replies[0].Params["text"])
		}
		limit := fmt.Sprintf("<code>%d</code>", tc.limit)
		other := fmt.Sprintf("<code>%d</code>", tc.other)
		if !strings.Contains(text, limit) || strings.Contains(text, other) ||
			!strings.Contains(text, groupLine(t, chatTitleOf(f, tc.chatID), tc.chatID)) {
			t.Fatalf("%q from PM: want one HTML reply in the PM with limit %d naming group %d; sent:\n%s", tc.text, tc.limit, tc.chatID, sentText(f.client))
		}
	}
	requireNothingSentTo(t, f, f.chat.Id)
	requireConnectedTo(t, f.admin.Id, connected)
}

// From PM, a member of the named group who is no admin there is refused
// /warnings.
func TestWarningsInPMWithGroupRefusesMember(t *testing.T) {
	f, _ := pmWarnsFixture(t, 0)
	if err := warns.SetWarnLimit(f.chat.Id, 5); err != nil {
		t.Fatalf("SetWarnLimit() error = %v", err)
	}

	processPM(t, warnsPMDispatcher(), f, f.member.Id, "/warnings @fixturegroup")

	if repliedIn(f, f.member.Id, "<code>5</code>") {
		t.Fatalf("/warnings from PM showed a member who is no admin the group's settings")
	}
	if !calledFor(f, "sendMessage", f.member.Id) {
		t.Fatalf("/warnings from PM by a member who is no admin got no answer")
	}
}

// From PM, /warnings for a group where the bot is no admin gets the answer it
// gets in that group, in the PM.
func TestWarningsInPMWithGroupAnswersAsGroupWhenBotIsNoAdmin(t *testing.T) {
	f, _ := pmWarnsFixture(t, 0)
	setup := func() {
		seedAdminCache(t, f.chat.Id, adminMember(f.admin.Id, false))
		f.client.membersByChat[fmt.Sprintf("%d|999", f.chat.Id)] = json.RawMessage(
			`{"status":"member","user":{"id":999,"is_bot":true,"first_name":"Alita"}}`)
	}
	dispatcher := warnsPMDispatcher()

	setup()
	update := newModuleMessageContext(f.bot, f.chat, f.admin, "/warnings").Update
	if err := dispatcher.ProcessUpdate(f.bot, update, nil); err != nil {
		t.Fatalf("ProcessUpdate(/warnings) error = %v", err)
	}
	inGroup := f.client.callsFor("sendMessage")
	if len(inGroup) != 1 {
		t.Fatalf("/warnings in the group with the bot no admin: want one refusal; sent:\n%s", sentText(f.client))
	}

	setup()
	processPM(t, dispatcher, f, f.admin.Id, "/warnings @fixturegroup")

	inPM := f.client.callsFor("sendMessage")[1:]
	if len(inPM) != 1 || fmt.Sprint(inPM[0].Params["text"]) != fmt.Sprint(inGroup[0].Params["text"]) ||
		fmt.Sprint(inPM[0].Params["chat_id"]) != fmt.Sprint(f.admin.Id) {
		t.Fatalf("/warnings from PM answered differently from the group, or not in the PM; sent:\n%s", sentText(f.client))
	}
}
