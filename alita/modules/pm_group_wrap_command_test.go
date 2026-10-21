//go:build testtools

package modules

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/admin"
	"github.com/divkix/Alita_Robot/alita/db/aispam"
	"github.com/divkix/Alita_Robot/alita/db/captcha"
	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/greetings"
	"github.com/divkix/Alita_Robot/alita/db/spamdbsettings"
	"github.com/divkix/Alita_Robot/alita/db/warns"
)

// pmWrapCommandDispatcher loads every module whose WrapCommand settings
// commands take a group from PM.
func pmWrapCommandDispatcher() *ext.Dispatcher {
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadGreetings(dispatcher)
	LoadCaptcha(dispatcher)
	LoadWarns(dispatcher)
	LoadAISpam(dispatcher)
	LoadSpamDB(dispatcher)
	LoadAdmin(dispatcher)
	return dispatcher
}

// One command per module, sent from PM with @fixturegroup while connected to
// another group, changes the named group and leaves the connected one alone.
func TestWrapCommandSettingsInPMWithGroupChangeThatGroup(t *testing.T) {
	for _, tc := range []struct {
		module, text string
		setup        func(t *testing.T, f adminChatFixture, connected int64)
		state        func(chatID int64) any
		want         any
	}{
		{"greetings", "/cleanservice @fixturegroup on", nil,
			func(id int64) any { return greetings.GetGreetingSettings(id).ShouldCleanService }, true},
		{"captcha", "/captchamode @fixturegroup text", nil,
			func(id int64) any {
				settings, _ := captcha.GetCaptchaSettings(id)
				return settings.CaptchaMode
			}, "text"},
		{"warns", "/setwarnlimit @fixturegroup 5", nil, warnLimit, 5},
		{"aispam", "/aispam @fixturegroup off",
			func(t *testing.T, f adminChatFixture, connected int64) {
				for _, id := range []int64{f.chat.Id, connected} {
					if err := aispam.SetAISpamEnabled(id, true); err != nil {
						t.Fatalf("SetAISpamEnabled(%d) error = %v", id, err)
					}
				}
			},
			func(id int64) any { return aispam.IsAISpamEnabled(id) }, false},
		{"spamdb", "/spamdb @fixturegroup action mute", nil,
			func(id int64) any {
				settings, _ := spamdbsettings.GetSpamDBSettingsContext(t.Context(), id)
				return settings.Action
			}, "mute"},
		{"admin", "/anonadmin @fixturegroup on",
			func(t *testing.T, f adminChatFixture, _ int64) { f.makeOwner() },
			func(id int64) any { return admin.GetAdminSettings(id).AnonAdmin }, true},
	} {
		t.Run(tc.module, func(t *testing.T) {
			f, connected := pmSettingsFixture(t)
			if tc.setup != nil {
				tc.setup(t, f, connected)
			}
			before := tc.state(connected)

			processPM(t, pmWrapCommandDispatcher(), f, f.admin.Id, tc.text)

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

// Every WrapCommand settings command sent from PM without a group and without
// a connection answers with its help, its example and the user's groups.
func TestWrapCommandSettingsInPMWithoutGroupReplyHelp(t *testing.T) {
	f := titleFixture(t, "My Group")
	dispatcher := pmWrapCommandDispatcher()
	pick := translated(t, "connections_pm_group_pick")
	for _, cmd := range []string{
		"welcome", "setwelcome", "resetwelcome", "goodbye", "setgoodbye", "resetgoodbye",
		"cleanwelcome", "cleangoodbye", "cleanservice", "autoapprove",
		"captcha", "captchamode", "captchatime", "captchaaction", "captchamaxattempts",
		"captchapending", "captchaclear",
		"setwarnlimit", "setwarnmode", "setwarntime",
		"aispam", "spamdb", "anonadmin",
	} {
		t.Run(cmd, func(t *testing.T) {
			before := len(f.client.callsFor("sendMessage"))

			processPM(t, dispatcher, f, f.admin.Id, "/"+cmd)

			replies := f.client.callsFor("sendMessage")[before:]
			if len(replies) != 1 || !strings.Contains(fmt.Sprint(replies[0].Params["text"]), pick) {
				t.Fatalf("/%s from PM replied %+v, want one reply asking for the group", cmd, replies)
			}
			assertPMGroupHelp(t, fmt.Sprint(replies[0].Params["text"]), cmd, f.chat)
		})
	}
}

// A member who is not an admin cannot change the group named from PM, though
// the group lets members connect, and the refusal keeps their connection.
func TestSetWarnLimitInPMWithGroupRefusesNonAdmin(t *testing.T) {
	f, connected := pmSettingsFixture(t)
	if err := connections.ToggleAllowConnect(f.chat.Id, true); err != nil {
		t.Fatalf("ToggleAllowConnect() error = %v", err)
	}
	if err := connections.ConnectId(f.member.Id, connected); err != nil {
		t.Fatalf("ConnectId() error = %v", err)
	}
	t.Cleanup(func() { _ = connections.DisconnectId(f.member.Id) })
	before := warnLimit(f.chat.Id)

	processPM(t, pmWrapCommandDispatcher(), f, f.member.Id, "/setwarnlimit @fixturegroup 5")

	if got := warnLimit(f.chat.Id); got != before {
		t.Fatalf("a member who is not an admin set the named group's warn limit to %v; sent:\n%s", got, sentText(f.client))
	}
	if conn := connections.Connection(f.member.Id); !conn.Connected || conn.ChatId != connected {
		t.Fatalf("member connection = %+v, want it kept on %d", conn, connected)
	}
}

// With a connection, a command with arguments of its own gets an argument
// that names no group, as before.
func TestWrapCommandSettingsInPMOnConnectionKeepOwnArguments(t *testing.T) {
	for _, tc := range []struct {
		text  string
		state func(chatID int64) any
		want  any
	}{
		{"/setwelcome Hi {first}", func(id int64) any {
			return strings.HasPrefix(greetings.GetGreetingSettings(id).WelcomeSettings.WelcomeText, "Hi ")
		}, true},
		{"/setgoodbye Bye {first}", func(id int64) any {
			return strings.HasPrefix(greetings.GetGreetingSettings(id).GoodbyeSettings.GoodbyeText, "Bye ")
		}, true},
		{"/cleanservice on", func(id int64) any { return greetings.GetGreetingSettings(id).ShouldCleanService }, true},
		{"/captchatime 5", func(id int64) any {
			settings, _ := captcha.GetCaptchaSettings(id)
			return settings.Timeout
		}, 5},
		{"/setwarnmode mute", func(id int64) any { return warns.GetWarnSetting(id).WarnMode }, "mute"},
		{"/setwarntime 30d", func(id int64) any { return warns.GetWarnSetting(id).WarnExpiry > 0 }, true},
		{"/spamdb action mute", func(id int64) any {
			settings, _ := spamdbsettings.GetSpamDBSettingsContext(context.Background(), id)
			return settings.Action
		}, "mute"},
	} {
		t.Run(tc.text, func(t *testing.T) {
			f := oneOffFixture(t)
			f.connectAdmin(t)

			processPM(t, pmWrapCommandDispatcher(), f, f.admin.Id, tc.text)

			if got := tc.state(f.chat.Id); got != tc.want {
				t.Fatalf("%s: connected group state = %v, want %v; sent:\n%s", tc.text, got, tc.want, sentText(f.client))
			}
		})
	}
}

// With a connection a number after /setwarnlimit is the limit, as before.
func TestSetWarnLimitInPMOnConnectionKeepsNumber(t *testing.T) {
	f := oneOffFixture(t)
	f.connectAdmin(t)

	processPM(t, pmWrapCommandDispatcher(), f, f.admin.Id, "/setwarnlimit 5")

	if got := warnLimit(f.chat.Id); got != 5 {
		t.Fatalf("/setwarnlimit 5 set the connected group's limit to %v, want 5; sent:\n%s", got, sentText(f.client))
	}
}

// /captchapending and /captchaclear take the pending user as their argument,
// so on a connection a user ID after them names that user in the connected
// group, not a group.
func TestCaptchaClearInPMOnConnectionKeepsUserArgument(t *testing.T) {
	f := oneOffFixture(t)
	f.connectAdmin(t)
	const userID = 4243
	attempt, err := captcha.CreateCaptchaAttemptPreMessage(userID, f.chat.Id, "7", 5)
	if err != nil {
		t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
	}
	if err := captcha.StoreMessageForCaptcha(userID, f.chat.Id, attempt.ID, db.TEXT, "pending", "", ""); err != nil {
		t.Fatalf("StoreMessageForCaptcha() error = %v", err)
	}

	processPM(t, pmWrapCommandDispatcher(), f, f.admin.Id, fmt.Sprintf("/captchaclear %d", userID))

	if messages, err := captcha.GetStoredMessagesForUser(userID, f.chat.Id); err != nil || len(messages) != 0 {
		t.Fatalf("stored messages after /captchaclear %d on a connection = %d, %v; want none; sent:\n%s",
			userID, len(messages), err, sentText(f.client))
	}
}
