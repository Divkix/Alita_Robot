package modules

import (
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/blacklists"
	"github.com/divkix/Alita_Robot/alita/db/logchannels"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/warns"
)

const warnsLogChannel = int64(-1009876543212)

// newWarnsLogFixture is an admin chat with a log channel; warnsOn turns the
// opt-in warns category on, which a new log channel has off.
func newWarnsLogFixture(t *testing.T, warnsOn bool) adminChatFixture {
	t.Helper()
	f := newAdminChatFixture(t)
	if err := logchannels.Set(f.chat.Id, f.chat.Title, warnsLogChannel); err != nil {
		t.Fatalf("logchannels.Set() error = %v", err)
	}
	if warnsOn {
		if err := logchannels.SetCategory(f.chat.Id, logchannels.CategoryWarns, true); err != nil {
			t.Fatalf("SetCategory(warns) error = %v", err)
		}
	}
	return f
}

// processWarnCommand sends text from the fixture's admin, as a reply to the
// fixture's member, through the warns handlers.
func processWarnCommand(t *testing.T, f adminChatFixture, text string) {
	t.Helper()
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadWarns(dispatcher)
	ctx := newWarnReplyContext(f.bot, f.chat, f.admin, f.member, text)
	if err := dispatcher.ProcessUpdate(f.bot, &gotgbot.Update{UpdateId: 1, Message: ctx.EffectiveMessage}, nil); err != nil {
		t.Fatalf("ProcessUpdate(%s) error = %v", text, err)
	}
}

func requireOneLogEntry(t *testing.T, f adminChatFixture, want ...string) {
	t.Helper()
	entries := spamDBMessagesTo(f.client, warnsLogChannel)
	if len(entries) != 1 {
		t.Fatalf("log channel entries = %d, want 1: %v", len(entries), entries)
	}
	for _, w := range want {
		if !strings.Contains(entries[0], w) {
			t.Fatalf("log entry %q does not contain %q", entries[0], w)
		}
	}
}

func requireNoLogEntry(t *testing.T, f adminChatFixture) {
	t.Helper()
	if entries := spamDBMessagesTo(f.client, warnsLogChannel); len(entries) != 0 {
		t.Fatalf("log channel entries = %d, want none: %v", len(entries), entries)
	}
}

func seedWarns(t *testing.T, chatID, userID int64, n int) {
	t.Helper()
	for i := range n {
		if _, _, err := warns.WarnUser(userID, chatID, models.WarnEntry{Reason: fmt.Sprintf("seed %d", i)}); err != nil {
			t.Fatalf("WarnUser() error = %v", err)
		}
	}
}

// Each warn action an admin takes logs one entry under the warns category
// when it is on, and nothing while it is off, as it is by default.
func TestAdminWarnActionsLogUnderTheWarnsCategory(t *testing.T) {
	useEnglishLocale(t)
	for _, tc := range []struct {
		name string
		tag  string
		want []string
		run  func(t *testing.T, f adminChatFixture)
	}{
		{name: "warn", tag: "WARN", want: []string{"Warns: 1/3", "Reason: spam"},
			run: func(t *testing.T, f adminChatFixture) { processWarnCommand(t, f, "/warn spam") }},
		{name: "warn at the limit", tag: "WARNLIMIT", want: []string{"Warns: 1/1", "Action: ban", "Reason: spam"},
			run: func(t *testing.T, f adminChatFixture) {
				if err := warns.SetWarnLimit(f.chat.Id, 1); err != nil {
					t.Fatalf("SetWarnLimit() error = %v", err)
				}
				if err := warns.SetWarnMode(f.chat.Id, "ban", 0); err != nil {
					t.Fatalf("SetWarnMode() error = %v", err)
				}
				processWarnCommand(t, f, "/warn spam")
				if calls := f.client.callsFor("banChatMember"); len(calls) != 1 {
					t.Fatalf("banChatMember calls = %d, want the warn-limit ban", len(calls))
				}
			}},
		{name: "rmwarn", tag: "RMWARN",
			run: func(t *testing.T, f adminChatFixture) {
				seedWarns(t, f.chat.Id, f.member.Id, 2)
				_ = warnsModule.removeWarn(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, "/rmwarn 42"))
			}},
		{name: "rmwarn button", tag: "RMWARN",
			run: func(t *testing.T, f adminChatFixture) {
				seedWarns(t, f.chat.Id, f.member.Id, 1)
				data := encodeCallbackData("rmWarn", map[string]string{"u": "42"})
				_ = warnsModule.rmWarnButton(f.bot, newModuleCallbackContext(f.bot, f.chat, f.admin, data))
			}},
		{name: "resetwarns", tag: "RESETWARNS",
			run: func(t *testing.T, f adminChatFixture) {
				seedWarns(t, f.chat.Id, f.member.Id, 2)
				_ = warnsModule.resetWarns(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, "/resetwarns 42"))
			}},
		{name: "resetallwarns", tag: "RESETALLWARNS",
			run: func(t *testing.T, f adminChatFixture) {
				seedWarns(t, f.chat.Id, f.member.Id, 1)
				data := confirmData("rmAllChatWarns", "yes", &f.chat)
				_ = warnsModule.warnsButtonHandler(f.bot, newModuleCallbackContext(f.bot, f.chat, f.admin, data))
				if warns.GetAllChatWarns(f.chat.Id) != 0 {
					t.Fatal("chat warns remain after the reset")
				}
			}},
	} {
		t.Run(tc.name+"/logged", func(t *testing.T) {
			f := newWarnsLogFixture(t, true)
			f.makeOwner()
			tc.run(t, f)
			want := append([]string{"#" + tc.tag + "\n", "Admin Fixture Chat", "Admin: ", fmt.Sprint(f.admin.Id)}, tc.want...)
			if tc.tag != "RESETALLWARNS" {
				want = append(want, "<code>42</code>")
			}
			requireOneLogEntry(t, f, want...)
		})
		t.Run(tc.name+"/category off by default", func(t *testing.T) {
			f := newWarnsLogFixture(t, false)
			f.makeOwner()
			tc.run(t, f)
			requireNoLogEntry(t, f)
		})
	}
}

// An action that did not happen logs nothing.
func TestFailedWarnActionsLogNothing(t *testing.T) {
	useEnglishLocale(t)

	t.Run("warning an admin is refused", func(t *testing.T) {
		f := newWarnsLogFixture(t, true)
		f.addAdmins(`{"status":"administrator","user":{"id":42,"is_bot":false,"first_name":"Member"}}`)
		processWarnCommand(t, f, "/warn spam")
		if numWarns, _ := warns.GetWarns(f.member.Id, f.chat.Id); numWarns != 0 {
			t.Fatalf("numWarns = %d, want the warn refused", numWarns)
		}
		requireNoLogEntry(t, f)
	})

	t.Run("no warn to remove", func(t *testing.T) {
		f := newWarnsLogFixture(t, true)
		_ = warnsModule.removeWarn(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, "/rmwarn 42"))
		_ = warnsModule.resetWarns(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, "/resetwarns 42"))
		requireNoLogEntry(t, f)
	})

	t.Run("reset all cancelled", func(t *testing.T) {
		f := newWarnsLogFixture(t, true)
		f.makeOwner()
		seedWarns(t, f.chat.Id, f.member.Id, 1)
		data := confirmData("rmAllChatWarns", "no", &f.chat)
		_ = warnsModule.warnsButtonHandler(f.bot, newModuleCallbackContext(f.bot, f.chat, f.admin, data))
		requireNoLogEntry(t, f)
	})

	// The warn is stored though the ban at the limit failed: it logs as a
	// warn, never as a limit reached.
	t.Run("ban at the limit failed", func(t *testing.T) {
		f := newWarnsLogFixture(t, true)
		f.client.errors["banChatMember"] = fmt.Errorf("Bad Request: not enough rights")
		if err := warns.SetWarnLimit(f.chat.Id, 1); err != nil {
			t.Fatalf("SetWarnLimit() error = %v", err)
		}
		if err := warns.SetWarnMode(f.chat.Id, "ban", 0); err != nil {
			t.Fatalf("SetWarnMode() error = %v", err)
		}
		processWarnCommand(t, f, "/warn spam")
		requireOneLogEntry(t, f, "#WARN\n", "Warns: 1/1")
	})
}

// A blacklist warn is the bot's, not an admin's: it stays out of the warns
// category even when that is on.
func TestBlacklistWarnLogsNothingUnderWarns(t *testing.T) {
	f := newWarnsLogFixture(t, true)
	if err := logchannels.SetCategory(f.chat.Id, logchannels.CategoryAutomated, false); err != nil {
		t.Fatalf("SetCategory(automated) error = %v", err)
	}
	if err := blacklists.AddBlacklist(f.chat.Id, "spam"); err != nil {
		t.Fatalf("AddBlacklist() error = %v", err)
	}
	if err := blacklists.SetBlacklistAction(f.chat.Id, "warn"); err != nil {
		t.Fatalf("SetBlacklistAction() error = %v", err)
	}
	_ = blacklistsModule.blacklistWatcher(f.bot, newModuleMessageContext(f.bot, f.chat, f.member, "this has spam inside"))

	if numWarns, _ := warns.GetWarns(f.member.Id, f.chat.Id); numWarns != 1 {
		t.Fatalf("numWarns = %d, want the blacklist warn", numWarns)
	}
	requireNoLogEntry(t, f)
}
