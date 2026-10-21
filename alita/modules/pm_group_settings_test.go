//go:build testtools

package modules

import (
	"encoding/json"
	"fmt"
	"html"
	"maps"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"unsafe"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"

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
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
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

// One settings command per module, sent from PM without a group and without
// a connection, answers with its help, its example and the user's groups. An
// alias answers with the help of the command it stands for.
func TestSettingsCommandsInPMWithoutGroupReplyHelp(t *testing.T) {
	f := titleFixture(t, "My Group")
	dispatcher := pmSettingsDispatcher()
	pick := translated(t, "connections_pm_group_pick")
	for _, tc := range []struct{ sent, help string }{
		{"lock", "lock"},
		{"setflood", "setflood"},
		{"disabled", "disabled"},
		{"addreaction", "addreaction"},
		{"antiraid", "antiraid"},
		{"cleanlinked", "cleanlinked"},
		{"allowconnect", "allowconnect"},
		{"clearrules", "resetrules"},
		{"blocklist", "addblacklist"},
		{"approve", "approve"},
	} {
		t.Run(tc.sent, func(t *testing.T) {
			before := len(f.client.callsFor("sendMessage"))

			processPM(t, dispatcher, f, f.admin.Id, "/"+tc.sent)

			replies := f.client.callsFor("sendMessage")[before:]
			if len(replies) != 1 || !strings.Contains(fmt.Sprint(replies[0].Params["text"]), pick) {
				t.Fatalf("/%s from PM replied %+v, want one reply asking for the group", tc.sent, replies)
			}
			assertPMGroupHelp(t, fmt.Sprint(replies[0].Params["text"]), tc.help, f.chat)
		})
	}
}

// dispatcherCommands returns the commands registered in dispatcher, which
// gotgbot keeps unexported.
func dispatcherCommands(t *testing.T, dispatcher *ext.Dispatcher) []handlers.Command {
	t.Helper()
	field := reflect.ValueOf(dispatcher).Elem().FieldByName("handlers").FieldByName("handlers")
	if field.Kind() != reflect.Map {
		t.Fatal("ext.Dispatcher no longer keeps its handlers in handlers.handlers")
	}
	groups := reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Interface().(map[int][]ext.Handler)
	var cmds []handlers.Command
	for _, group := range groups {
		for _, h := range group {
			if cmd, ok := h.(handlers.Command); ok {
				cmds = append(cmds, cmd)
			}
		}
	}
	return cmds
}

// funcName returns the name of function f. The closures a function returns are
// named after it with a suffix, inlined copies too.
func funcName(f any) string {
	return runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
}

// Every command that takes its group from PM answers a call without a group
// with the help entry and example of a command in commandHelpModules, so a
// new command or alias without an entry fails here. Every withPMGroupArg
// command must ask for the group; a WrapCommand command asks only when its
// descriptor sets PMGroupArg.
func TestEveryPMGroupCommandHasHelpEntry(t *testing.T) {
	f := titleFixture(t, "My Group")
	legacy, pipeline := funcName(withPMGroupArg)+".", funcName(helpers.WrapCommand)+"."
	all := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadAllModules(all)
	pick := translated(t, "connections_pm_group_pick")
	var asked []string
	for _, cmd := range dispatcherCommands(t, all) {
		name := funcName(cmd.Response)
		isLegacy := strings.HasPrefix(name, legacy)
		if !isLegacy && !strings.HasPrefix(name, pipeline) {
			continue
		}
		dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
		dispatcher.AddHandler(cmd)
		before := len(f.client.callsFor("sendMessage"))

		processPM(t, dispatcher, f, f.admin.Id, "/"+cmd.Command)

		replies := f.client.callsFor("sendMessage")[before:]
		if len(replies) != 1 || !strings.Contains(fmt.Sprint(replies[0].Params["text"]), pick) {
			if isLegacy {
				t.Errorf("/%s from PM replied %+v, want one reply asking for the group", cmd.Command, replies)
			}
			continue
		}
		asked = append(asked, cmd.Command)
		text := fmt.Sprint(replies[0].Params["text"])
		if !slices.ContainsFunc(slices.Collect(maps.Keys(commandHelpModules)), func(help string) bool {
			example := commandExample(help)
			return example != "" && strings.Contains(text, html.EscapeString(example))
		}) {
			t.Errorf("/%s from PM answers without a help entry; add its command to commandHelpModules", cmd.Command)
		}
	}
	for _, want := range []string{"lock", "setwelcome"} {
		if !slices.Contains(asked, want) {
			t.Fatalf("/%s is not among the %d commands that asked for the group: %v", want, len(asked), asked)
		}
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

// A member who is not an admin cannot change the group named from PM, though
// the group lets members connect, and the refusal keeps their connection.
// /setflood checks the admin only through resolveAdminChat; /lock checks it
// again in its handler.
func TestSettingsCommandsInPMWithGroupRefuseNonAdmin(t *testing.T) {
	for _, tc := range []struct {
		text    string
		changed func(chatID int64) bool
	}{
		{"/lock @fixturegroup sticker", func(id int64) bool { return locks.IsPermLocked(id, "sticker") }},
		{"/setflood @fixturegroup 7", func(id int64) bool { return antiflood.GetFlood(id).Limit == 7 }},
	} {
		t.Run(tc.text, func(t *testing.T) {
			f, connected := pmSettingsFixture(t)
			if err := connections.ToggleAllowConnect(f.chat.Id, true); err != nil {
				t.Fatalf("ToggleAllowConnect() error = %v", err)
			}
			if err := connections.ConnectId(f.member.Id, connected); err != nil {
				t.Fatalf("ConnectId() error = %v", err)
			}
			t.Cleanup(func() { _ = connections.DisconnectId(f.member.Id) })

			processPM(t, pmSettingsDispatcher(), f, f.member.Id, tc.text)

			if tc.changed(f.chat.Id) {
				t.Fatalf("%s: a member who is not an admin changed the named group; sent:\n%s", tc.text, sentText(f.client))
			}
			if conn := connections.Connection(f.member.Id); !conn.Connected || conn.ChatId != connected {
				t.Fatalf("member connection = %+v, want it kept on %d", conn, connected)
			}
		})
	}
}
