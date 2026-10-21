//go:build testtools

package modules

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/antiflood"
	"github.com/divkix/Alita_Robot/alita/db/antiraid"
	"github.com/divkix/Alita_Robot/alita/db/approvals"
	"github.com/divkix/Alita_Robot/alita/db/blacklists"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/disabling"
	"github.com/divkix/Alita_Robot/alita/db/locks"
	"github.com/divkix/Alita_Robot/alita/db/pins"
	"github.com/divkix/Alita_Robot/alita/db/reactions"
	"github.com/divkix/Alita_Robot/alita/db/rules"
)

// pmSettingsDispatcher loads every module whose settings commands take a
// group from PM.
func pmSettingsDispatcher() *ext.Dispatcher {
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadLocks(dispatcher)
	LoadAntiflood(dispatcher)
	LoadDisabling(dispatcher)
	LoadReactions(dispatcher)
	LoadAntiRaid(dispatcher)
	LoadPin(dispatcher)
	LoadConnections(dispatcher)
	LoadRules(dispatcher)
	LoadBlacklists(dispatcher)
	LoadApprovals(dispatcher)
	return dispatcher
}

// pmSettingsFixture is oneOffFixture's group (@fixturegroup) with its admin
// connected to another group.
func pmSettingsFixture(t *testing.T) (f adminChatFixture, connected int64) {
	t.Helper()
	withMiniredis(t)
	f = oneOffFixture(t)
	connected = uniqueModuleChatID()
	for _, id := range []int64{f.chat.Id, connected} {
		if err := chats.EnsureChatInDb(id, fmt.Sprintf("Group %d", id)); err != nil {
			t.Fatalf("EnsureChatInDb(%d) error = %v", id, err)
		}
	}
	if err := connections.ConnectId(f.admin.Id, connected); err != nil {
		t.Fatalf("ConnectId() error = %v", err)
	}
	t.Cleanup(func() { _ = connections.DisconnectId(f.admin.Id) })
	return f, connected
}

// processPM runs text through the dispatcher as a message user sent in PM.
func processPM(t *testing.T, dispatcher *ext.Dispatcher, f adminChatFixture, userID int64, text string) {
	t.Helper()
	user := f.admin
	if userID != f.admin.Id {
		user = f.member
	}
	update := newModuleMessageContext(f.bot, privateChatOf(user), user, text).Update
	if err := dispatcher.ProcessUpdate(f.bot, update, nil); err != nil {
		t.Fatalf("ProcessUpdate(%q) error = %v", text, err)
	}
}

// One command per module, sent from PM with @fixturegroup while connected to
// another group, changes the named group and leaves the connected one alone.
func TestSettingsCommandsInPMWithGroupChangeThatGroup(t *testing.T) {
	for _, tc := range []struct {
		module, text string
		state        func(chatID int64) any
		want         any
	}{
		{"locks", "/lock @fixturegroup sticker", func(id int64) any { return locks.IsPermLocked(id, "sticker") }, true},
		{"antiflood", "/setflood @fixturegroup 7", func(id int64) any { return antiflood.GetFlood(id).Limit }, 7},
		{"disabling", "/disable @fixturegroup locks", func(id int64) any { return disabling.IsCommandDisabled(id, "locks") }, true},
		{"reactions", "/addreaction @fixturegroup hello 👍", func(id int64) any { return reactions.GetReactions(id)["hello"] }, "👍"},
		{"antiraid", "/raidtime @fixturegroup 2h", func(id int64) any { return antiraid.GetAntiRaidSettings(id).RaidTime }, 7200},
		{"pins", "/antichannelpin @fixturegroup on", func(id int64) any { return pins.GetPinData(id).AntiChannelPin }, true},
		{"connections", "/allowconnect @fixturegroup off", func(id int64) any { return connections.GetChatConnectionSetting(id).AllowConnect }, false},
		{"rules", "/privaterules @fixturegroup on", func(id int64) any { return rules.GetChatRulesInfo(id).Private }, true},
		{"blacklists", "/addblacklist @fixturegroup badword", func(id int64) any { return fmt.Sprint(blacklists.GetBlacklistSettings(id).Triggers()) }, "[badword]"},
		{"approvals", "/approve @fixturegroup 4242", func(id int64) any { return approvals.IsUserApproved(id, 4242) }, true},
	} {
		t.Run(tc.module, func(t *testing.T) {
			f, connected := pmSettingsFixture(t)
			before := tc.state(connected)

			processPM(t, pmSettingsDispatcher(), f, f.admin.Id, tc.text)

			if got := tc.state(f.chat.Id); got != tc.want {
				t.Fatalf("%s: named group state = %v, want %v; sent:\n%s", tc.text, got, tc.want, sentText(f.client))
			}
			if got := tc.state(connected); got != before {
				t.Fatalf("%s: connected group state = %v, want it unchanged (%v)", tc.text, got, before)
			}
			if conn := connections.Connection(f.admin.Id); !conn.Connected || conn.ChatId != connected {
				t.Fatalf("connection = %+v, want it kept on %d", conn, connected)
			}
		})
	}
}

// Every settings command sent from PM without a group and without a
// connection answers with its help, its example and the user's groups. An
// alias answers with the help of the command it stands for.
func TestSettingsCommandsInPMWithoutGroupReplyHelp(t *testing.T) {
	f := titleFixture(t, "My Group")
	dispatcher := pmSettingsDispatcher()
	pick := translated(t, "connections_pm_group_pick")
	for sent, help := range map[string]string{
		"locks": "locks", "locktypes": "locktypes", "lock": "lock", "unlock": "unlock",
		"flood": "flood", "setflood": "setflood", "setfloodmode": "setfloodmode",
		"delflood": "delflood", "clearflood": "delflood",
		"disabled": "disabled", "disable": "disable", "enable": "enable", "disabledel": "disabledel",
		"reactions": "reactions", "addreaction": "addreaction", "removereaction": "removereaction",
		"resetreactions": "resetreactions",
		"antiraid":       "antiraid", "raidtime": "raidtime", "raidactiontime": "raidactiontime",
		"autoantiraid":   "autoantiraid",
		"antichannelpin": "antichannelpin", "cleanlinked": "cleanlinked",
		"allowconnect": "allowconnect",
		"privaterules": "privaterules", "setrules": "setrules",
		"resetrules": "resetrules", "clearrules": "resetrules",
		"rulesbtn": "rulesbtn", "rulesbutton": "rulesbtn",
		"resetrulesbutton": "resetrulesbutton", "resetrulesbtn": "resetrulesbutton",
		"clearrulesbutton": "resetrulesbutton", "clearrulesbtn": "resetrulesbutton",
		"blacklists": "blacklists", "blocklists": "blacklists",
		"addblacklist": "addblacklist", "blacklist": "addblacklist", "addblocklist": "addblacklist",
		"blocklist":   "addblacklist",
		"rmblacklist": "rmblacklist", "rmblocklist": "rmblacklist", "unblacklist": "rmblacklist",
		"blaction": "blaction", "blacklistaction": "blaction",
		"approved": "approved", "approve": "approve", "unapprove": "unapprove", "approval": "approval",
	} {
		t.Run(sent, func(t *testing.T) {
			before := len(f.client.callsFor("sendMessage"))

			processPM(t, dispatcher, f, f.admin.Id, "/"+sent)

			replies := f.client.callsFor("sendMessage")[before:]
			if len(replies) != 1 || !strings.Contains(fmt.Sprint(replies[0].Params["text"]), pick) {
				t.Fatalf("/%s from PM replied %+v, want one reply asking for the group", sent, replies)
			}
			assertPMGroupHelp(t, fmt.Sprint(replies[0].Params["text"]), help, f.chat)
		})
	}
}

// With a connection, a user's @username or ID after /approve names the user
// to approve in the connected group, not a group.
func TestApproveInPMOnConnectionKeepsUserArgument(t *testing.T) {
	for _, tc := range []struct {
		text   string
		userID int64
	}{
		{"/approve @someuser", 5},
		{"/approve 4242", 4242},
	} {
		t.Run(tc.text, func(t *testing.T) {
			f := oneOffFixture(t)
			f.client.chatsByID["@someuser"] = json.RawMessage(`{"id":5,"type":"private","first_name":"Some"}`)
			f.connectAdmin(t)

			processPM(t, pmSettingsDispatcher(), f, f.admin.Id, tc.text)

			if !approvals.IsUserApproved(f.chat.Id, tc.userID) {
				t.Fatalf("%s did not approve %d in the connected group; sent:\n%s", tc.text, tc.userID, sentText(f.client))
			}
		})
	}
}

// With a connection a bare word after /lock is a lock type, as before.
func TestLockInPMOnConnectionKeepsBareWord(t *testing.T) {
	f := oneOffFixture(t)
	f.connectAdmin(t)

	processPM(t, pmSettingsDispatcher(), f, f.admin.Id, "/lock all")

	if !locks.IsPermLocked(f.chat.Id, "all") {
		t.Fatalf("/lock all did not lock the connected group; sent:\n%s", sentText(f.client))
	}
}

// A member who is not an admin cannot lock the group named from PM, and the
// refusal keeps their connection.
func TestLockInPMWithGroupRefusesNonAdmin(t *testing.T) {
	f, connected := pmSettingsFixture(t)
	if err := connections.ConnectId(f.member.Id, connected); err != nil {
		t.Fatalf("ConnectId() error = %v", err)
	}
	t.Cleanup(func() { _ = connections.DisconnectId(f.member.Id) })

	processPM(t, pmSettingsDispatcher(), f, f.member.Id, "/lock @fixturegroup sticker")

	if locks.IsPermLocked(f.chat.Id, "sticker") {
		t.Fatal("a member who is not an admin locked the named group")
	}
	if conn := connections.Connection(f.member.Id); !conn.Connected || conn.ChatId != connected {
		t.Fatalf("member connection = %+v, want it kept on %d", conn, connected)
	}
}

// On a connection to another group, a command with arguments of its own
// called with the title of the user's group, as the help examples once
// showed, changes neither group.
func TestSettingsCommandsInPMOnConnectionWithGroupTitleChangeNothing(t *testing.T) {
	for _, tc := range []struct {
		text  string
		state func(id int64) any
	}{
		{`/setrules "My Group" Be kind to each other.`, func(id int64) any { return rules.GetChatRulesInfo(id).Rules }},
		{`/addblacklist "My Group" spamword`, func(id int64) any { return fmt.Sprint(blacklists.GetBlacklistSettings(id).Triggers()) }},
		{`/rmblacklist "My Group" spamword`, func(id int64) any { return fmt.Sprint(blacklists.GetBlacklistSettings(id).Triggers()) }},
		{`/disable "My Group" rules`, func(id int64) any { return disabling.IsCommandDisabled(id, "rules") }},
		{`/lock "My Group" sticker`, func(id int64) any { return locks.IsPermLocked(id, "sticker") }},
		{`/rulesbtn "My Group" Read the rules`, func(id int64) any { return rules.GetChatRulesInfo(id).RulesBtn }},
		{`/addreaction "My Group" hello 👍`, func(id int64) any { return fmt.Sprint(reactions.GetReactions(id)) }},
	} {
		t.Run(tc.text, func(t *testing.T) {
			f := titleFixture(t, "My Group")
			other := uniqueModuleChatID()
			f.addGroup(t, other, "Other", true)
			for _, id := range []int64{f.chat.Id, other} {
				if err := blacklists.AddBlacklist(id, "spamword"); err != nil {
					t.Fatalf("AddBlacklist() error = %v", err)
				}
			}
			if err := rules.SetChatRules(other, "B rules"); err != nil {
				t.Fatalf("SetChatRules() error = %v", err)
			}
			if err := connections.ConnectId(f.admin.Id, other); err != nil {
				t.Fatalf("ConnectId() error = %v", err)
			}
			t.Cleanup(func() { _ = connections.DisconnectId(f.admin.Id) })
			beforeA, beforeB := tc.state(f.chat.Id), tc.state(other)

			processPM(t, pmSettingsDispatcher(), f, f.admin.Id, tc.text)

			if afterA, afterB := tc.state(f.chat.Id), tc.state(other); afterA != beforeA || afterB != beforeB {
				t.Fatalf("%s: named group %v -> %v, connected group %v -> %v, want both unchanged; sent:\n%s",
					tc.text, beforeA, afterA, beforeB, afterB, sentText(f.client))
			}
			requireTitleRefused(t, f, "My Group", other)
		})
	}
}
