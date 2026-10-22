//go:build testtools

package modules

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/logchannels"
)

// mutesPMDispatcher also loads the bans module, which answers the unmute
// button the mute commands send.
func mutesPMDispatcher() *ext.Dispatcher {
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadMutes(dispatcher)
	LoadBans(dispatcher)
	return dispatcher
}

// From PM, /tmute reads the time after the user as in the group.
func TestTmuteInPMWithGroupReadsTimeAfterUser(t *testing.T) {
	f, _ := pmModerationFixture(t)
	before := time.Now().Unix()

	processPM(t, mutesPMDispatcher(), f, f.admin.Id, "/tmute @fixturegroup @victim 2h flooding")

	calls := f.client.callsFor("restrictChatMember")
	if len(calls) != 1 || fmt.Sprint(calls[0].Params["chat_id"]) != fmt.Sprint(f.chat.Id) {
		t.Fatalf("/tmute from PM: restrictChatMember calls %+v, want one in the named group", calls)
	}
	until, err := strconv.ParseInt(fmt.Sprint(calls[0].Params["until_date"]), 10, 64)
	if err != nil || until < before+7200 || until > time.Now().Unix()+7200 {
		t.Fatalf("/tmute from PM: until_date = %v, want two hours from now", calls[0].Params["until_date"])
	}
	if !repliedIn(f, f.admin.Id, "flooding") {
		t.Fatalf("/tmute from PM lost the reason; sent:\n%s", sentText(f.client))
	}
}

// From PM, /smute answers like /mute, unmute button included: there is no
// command message in the group to delete.
func TestSmuteInPMWithGroupAnswersLikeMute(t *testing.T) {
	f, _ := pmModerationFixture(t)

	processPM(t, mutesPMDispatcher(), f, f.admin.Id, "/smute @fixturegroup @victim")

	if calls := f.client.callsFor("deleteMessage"); len(calls) != 0 {
		t.Fatalf("/smute from PM deleted a message: %+v", calls)
	}
	if !repliedIn(f, f.admin.Id, "Victim") {
		t.Fatalf("/smute from PM did not answer in the PM; sent:\n%s", sentText(f.client))
	}
	buttonFor(t, f, 0, "unrestrict", "unmute")
}

// /dmute needs a reply to a group message, so it stays group-only: from PM it
// acts on no one and answers as before.
func TestDmuteInPMStaysGroupOnly(t *testing.T) {
	f, _ := pmModerationFixture(t)
	requireGroupOnlyInPM(t, mutesPMDispatcher, f, "dmute")
}

// The unmute button a mute command sends from PM names the named group and
// acts on it; the presser is checked against that group, so a member, or an
// admin pressing a button forged to name a group where they have no right,
// is refused. Sent in the group, the button names no group.
func TestMuteButtonsFromPMActOnNamedGroup(t *testing.T) {
	for _, cmd := range []string{"mute", "smute"} {
		t.Run(cmd, func(t *testing.T) {
			f, connected := pmModerationFixture(t)
			dispatcher := mutesPMDispatcher()
			processPM(t, dispatcher, f, f.admin.Id, "/"+cmd+" @fixturegroup @victim")
			data := buttonFor(t, f, 0, "unrestrict", "unmute")
			decoded, _ := decodeCallbackData(data)
			if decoded == nil {
				t.Fatalf("the unmute button from PM has data %q that does not decode", data)
			}
			if c, _ := decoded.Field("c"); c != fmt.Sprint(f.chat.Id) {
				t.Fatalf("the unmute button from PM names group %q, want %d", c, f.chat.Id)
			}
			calls := len(f.client.callsFor("restrictChatMember"))

			pressConfirmInPM(t, dispatcher, f, f.admin, data)

			pressed := f.client.callsFor("restrictChatMember")[calls:]
			if len(pressed) != 1 || fmt.Sprint(pressed[0].Params["chat_id"]) != fmt.Sprint(f.chat.Id) {
				t.Fatalf("the unmute button from PM: restrictChatMember calls %+v, want one in the named group", pressed)
			}
			if targetedIn(f, "restrictChatMember", connected, victimID) {
				t.Fatalf("the unmute button from PM acted on the connected group")
			}
		})
	}
	t.Run("member", func(t *testing.T) {
		f, _ := pmModerationFixture(t)
		data := encodeCallbackData("unrestrict", map[string]string{"a": "unmute", "c": fmt.Sprint(f.chat.Id), "u": fmt.Sprint(victimID)})

		pressConfirmInPM(t, mutesPMDispatcher(), f, f.member, data)

		if len(f.client.callsFor("restrictChatMember")) != 0 {
			t.Fatalf("a member's press of the unmute button acted")
		}
	})
	t.Run("forged group", func(t *testing.T) {
		f, _ := pmModerationFixture(t)
		other := uniqueModuleChatID()
		f.setMemberStatus(other, f.admin.Id, "member")
		data := encodeCallbackData("unrestrict", map[string]string{"a": "unmute", "c": fmt.Sprint(other), "u": fmt.Sprint(victimID)})

		pressConfirmInPM(t, mutesPMDispatcher(), f, f.admin, data)

		if len(f.client.callsFor("restrictChatMember")) != 0 {
			t.Fatalf("an unmute button naming a group the presser has no right in acted")
		}
	})
	t.Run("in the group", func(t *testing.T) {
		for _, cmd := range []string{"mute", "dmute"} {
			t.Run(cmd, func(t *testing.T) {
				f, _ := pmModerationFixture(t)

				// /dmute needs a reply to a group message; the others take @victim.
				update := newModuleMessageContext(f.bot, f.chat, f.admin, "/mute @victim").Update
				if cmd == "dmute" {
					update = newBanReplyContext(f.bot, f.chat, f.admin, gotgbot.User{Id: victimID, FirstName: "Victim"}, "/dmute").Update
				}
				if err := mutesPMDispatcher().ProcessUpdate(f.bot, update, nil); err != nil {
					t.Fatalf("ProcessUpdate(/%s) error = %v", cmd, err)
				}

				decoded, _ := decodeCallbackData(buttonFor(t, f, 0, "unrestrict", "unmute"))
				if c, named := decoded.Field("c"); named {
					t.Fatalf("/%s in the group sent an unmute button naming group %q", cmd, c)
				}
			})
		}
	})
}

// A mute command sent from PM logs to the log channel of the named group, as
// the same command does in that group, and not to the one of the connected
// group.
func TestMuteCommandsInPMWithGroupLogToNamedGroup(t *testing.T) {
	const namedLog, connectedLog int64 = -100771, -100772
	for _, tc := range []struct{ cmd, args string }{
		{"mute", " spam"}, {"smute", ""}, {"tmute", " 2h"}, {"unmute", ""},
	} {
		t.Run(tc.cmd, func(t *testing.T) {
			f, connected := pmModerationFixture(t)
			if err := logchannels.Set(f.chat.Id, f.chat.Title, namedLog); err != nil {
				t.Fatalf("logchannels.Set(named) error = %v", err)
			}
			if err := logchannels.Set(connected, "Group B", connectedLog); err != nil {
				t.Fatalf("logchannels.Set(connected) error = %v", err)
			}
			t.Cleanup(func() {
				_ = logchannels.Unset(f.chat.Id)
				_ = logchannels.Unset(connected)
			})

			processPM(t, mutesPMDispatcher(), f, f.admin.Id, "/"+tc.cmd+" @fixturegroup @victim"+tc.args)

			if !targetedIn(f, "restrictChatMember", f.chat.Id, victimID) {
				t.Fatalf("/%s from PM did not act on @victim in the named group; sent:\n%s", tc.cmd, sentText(f.client))
			}
			if n := logChannelMessages(f.client, namedLog); n != 1 {
				t.Fatalf("/%s from PM: the named group's log channel got %d messages, want 1; sent:\n%s", tc.cmd, n, sentText(f.client))
			}
			if n := logChannelMessages(f.client, connectedLog); n != 0 {
				t.Fatalf("/%s from PM: the connected group's log channel got %d messages, want none", tc.cmd, n)
			}
		})
	}
}
