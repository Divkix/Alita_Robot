//go:build testtools

package modules

import (
	"encoding/json"
	"fmt"
	"html"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

// mutesPMDispatcher also loads the bans module, which answers the unmute
// button the mute commands send.
func mutesPMDispatcher() *ext.Dispatcher {
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadMutes(dispatcher)
	LoadBans(dispatcher)
	return dispatcher
}

var pmMuteCases = []pmBanCase{
	{"mute", " spam", calledOn("restrictChatMember")},
	{"smute", "", calledOn("restrictChatMember")},
	{"tmute", " 2h", calledOn("restrictChatMember")},
	{"unmute", "", calledOn("restrictChatMember")},
}

// From PM with @fixturegroup while connected to another group, each mute
// command acts on @victim in the named group and answers in the PM, naming
// that group in HTML; the connected group and the connection stay as they
// were, and nothing is sent to the named group.
func TestMuteCommandsInPMWithGroupActOnThatGroup(t *testing.T) {
	for _, tc := range pmMuteCases {
		t.Run(tc.cmd, func(t *testing.T) {
			f, connected := pmBansFixture(t)
			named := `Tom & <Jerry>`
			for _, key := range []string{fmt.Sprint(f.chat.Id), "@fixturegroup"} {
				f.client.chatsByID[key] = json.RawMessage(fmt.Sprintf(`{"id":%d,"type":"supergroup","title":%q}`, f.chat.Id, named))
			}

			processPM(t, mutesPMDispatcher(), f, f.admin.Id, "/"+tc.cmd+" @fixturegroup @victim"+tc.args)

			if !tc.acted(t, f, f.chat.Id, victimID) {
				t.Fatalf("/%s from PM did not act on @victim in the named group; sent:\n%s", tc.cmd, sentText(f.client))
			}
			if tc.acted(t, f, connected, victimID) {
				t.Fatalf("/%s from PM acted on the connected group", tc.cmd)
			}
			want := groupLine(t, named, f.chat.Id)
			replied := false
			for _, call := range f.client.callsFor("sendMessage") {
				if fmt.Sprint(call.Params["chat_id"]) == fmt.Sprint(f.admin.Id) && strings.Contains(fmt.Sprint(call.Params["text"]), want) {
					replied = fmt.Sprint(call.Params["parse_mode"]) == gotgbot.ParseModeHTML
				}
			}
			if !replied {
				t.Fatalf("/%s from PM did not answer in the PM naming the group %q as HTML; sent:\n%s", tc.cmd, want, sentText(f.client))
			}
			requireNothingSentTo(t, f, f.chat.Id)
			requireConnectedTo(t, f.admin.Id, connected)
		})
	}
}

// With a connection and no group named, a mute command acts on @victim in
// the connected group and names it in the reply.
func TestMuteCommandsInPMOnConnectionActOnConnectedGroup(t *testing.T) {
	for _, tc := range pmMuteCases {
		t.Run(tc.cmd, func(t *testing.T) {
			f, connected := pmBansFixture(t)

			processPM(t, mutesPMDispatcher(), f, f.admin.Id, "/"+tc.cmd+" @victim"+tc.args)

			if !tc.acted(t, f, connected, victimID) {
				t.Fatalf("/%s from PM on a connection did not act on the connected group; sent:\n%s", tc.cmd, sentText(f.client))
			}
			if tc.acted(t, f, f.chat.Id, victimID) {
				t.Fatalf("/%s from PM on a connection acted on another group", tc.cmd)
			}
			if want := groupLine(t, chatTitleOf(f, connected), connected); !repliedIn(f, f.admin.Id, want) {
				t.Fatalf("/%s from PM on a connection did not name the connected group %q; sent:\n%s", tc.cmd, want, sentText(f.client))
			}
		})
	}
}

// From PM, a member of the named group who is no admin there, and an admin
// of it without the right to restrict there, are refused, though the caller
// holds that right in the group they are connected to.
func TestMuteCommandsInPMWithGroupRefuseCallerWithoutRight(t *testing.T) {
	for _, tc := range pmMuteCases {
		t.Run(tc.cmd+"/member", func(t *testing.T) {
			f, connected := pmBansFixture(t)
			if err := connections.ToggleAllowConnect(f.chat.Id, true); err != nil {
				t.Fatalf("ToggleAllowConnect() error = %v", err)
			}
			if err := connections.ConnectId(f.member.Id, connected); err != nil {
				t.Fatalf("ConnectId() error = %v", err)
			}
			t.Cleanup(func() { _ = connections.DisconnectId(f.member.Id) })

			processPM(t, mutesPMDispatcher(), f, f.member.Id, "/"+tc.cmd+" @fixturegroup @victim"+tc.args)

			if tc.acted(t, f, f.chat.Id, victimID) {
				t.Fatalf("/%s from a member who is no admin acted on the group", tc.cmd)
			}
			if !calledFor(f, "sendMessage", f.member.Id) {
				t.Fatalf("/%s from a member who is no admin got no answer", tc.cmd)
			}
			requireConnectedTo(t, f.member.Id, connected)
		})
		t.Run(tc.cmd+"/admin without the right", func(t *testing.T) {
			f, connected := pmBansFixture(t)
			admin := adminMember(f.admin.Id, false)
			admin.CanRestrictMembers = false
			seedAdminCache(t, f.chat.Id, adminMember(999, true), admin)

			processPM(t, mutesPMDispatcher(), f, f.admin.Id, "/"+tc.cmd+" @fixturegroup @victim"+tc.args)

			if tc.acted(t, f, f.chat.Id, victimID) || tc.acted(t, f, connected, victimID) {
				t.Fatalf("/%s from an admin without the right acted on a group", tc.cmd)
			}
			if !repliedIn(f, f.admin.Id, translated(t, "chat_status_restrict_cmd_error")) {
				t.Fatalf("/%s from an admin without the right: want chat_status_restrict_cmd_error; sent:\n%s", tc.cmd, sentText(f.client))
			}
			requireConnectedTo(t, f.admin.Id, connected)
		})
	}
}

// From PM, a mute command the bot cannot carry out in the named group gets
// the answer it gets in that group, in the PM.
func TestMuteCommandsInPMWithGroupAnswerAsGroupWhenBotLacksRight(t *testing.T) {
	for _, tc := range pmMuteCases {
		t.Run(tc.cmd, func(t *testing.T) {
			f, connected := pmBansFixture(t)
			setup := func() {
				bot := adminMember(999, true)
				bot.CanRestrictMembers = false
				seedAdminCache(t, f.chat.Id, adminMember(f.admin.Id, false), bot)
				f.client.membersByChat[fmt.Sprintf("%d|999", f.chat.Id)] = json.RawMessage(
					`{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"},"can_delete_messages":true}`)
			}
			dispatcher := mutesPMDispatcher()

			setup()
			update := newModuleMessageContext(f.bot, f.chat, f.admin, "/"+tc.cmd+" @victim"+tc.args).Update
			if err := dispatcher.ProcessUpdate(f.bot, update, nil); err != nil {
				t.Fatalf("ProcessUpdate(/%s) error = %v", tc.cmd, err)
			}
			inGroup := f.client.callsFor("sendMessage")
			if len(inGroup) != 1 || tc.acted(t, f, f.chat.Id, victimID) {
				t.Fatalf("/%s in the group without the bot's right: want one refusal; sent:\n%s", tc.cmd, sentText(f.client))
			}

			setup()
			processPM(t, dispatcher, f, f.admin.Id, "/"+tc.cmd+" @fixturegroup @victim"+tc.args)

			inPM := f.client.callsFor("sendMessage")[1:]
			if len(inPM) != 1 || fmt.Sprint(inPM[0].Params["text"]) != fmt.Sprint(inGroup[0].Params["text"]) ||
				fmt.Sprint(inPM[0].Params["chat_id"]) != fmt.Sprint(f.admin.Id) {
				t.Fatalf("/%s from PM answered differently from the group, or not in the PM; sent:\n%s", tc.cmd, sentText(f.client))
			}
			if tc.acted(t, f, f.chat.Id, victimID) || tc.acted(t, f, connected, victimID) {
				t.Fatalf("/%s from PM acted though the bot has no right in the named group", tc.cmd)
			}
		})
	}
}

// From PM, an admin of the named group who is no admin of the connected one
// cannot be muted; neither can the caller or the bot, admins of the group.
func TestMuteCommandsInPMWithGroupProtectAdminsOfThatGroup(t *testing.T) {
	const groupAdminID = 4343
	for _, tc := range pmMuteCases {
		if tc.cmd == "unmute" {
			continue
		}
		for _, target := range []struct {
			name string
			id   func(f adminChatFixture) int64
		}{
			{"admin of the named group", func(adminChatFixture) int64 { return groupAdminID }},
			{"caller", func(f adminChatFixture) int64 { return f.admin.Id }},
			{"bot", func(adminChatFixture) int64 { return 999 }},
		} {
			t.Run(tc.cmd+"/"+target.name, func(t *testing.T) {
				f, connected := pmBansFixture(t)
				seedAdminCache(t, f.chat.Id, adminMember(999, true), adminMember(f.admin.Id, false), adminMember(groupAdminID, false))
				id := target.id(f)

				processPM(t, mutesPMDispatcher(), f, f.admin.Id, fmt.Sprintf("/%s @fixturegroup %d%s", tc.cmd, id, tc.args))

				if tc.acted(t, f, f.chat.Id, id) || tc.acted(t, f, connected, id) {
					t.Fatalf("/%s from PM muted the %s", tc.cmd, target.name)
				}
				if !repliedIn(f, f.admin.Id, translated(t, "mutes_mute_admin_error")) {
					t.Fatalf("/%s from PM on the %s: want mutes_mute_admin_error; sent:\n%s", tc.cmd, target.name, sentText(f.client))
				}
			})
		}
	}
}

// A reply in PM names no target: the command answers as it does in the group
// without one, and a user named after the group wins over the reply.
func TestMuteCommandsInPMWithGroupIgnoreReply(t *testing.T) {
	bot := gotgbot.User{Id: 999, IsBot: true, FirstName: "Alita"}
	for _, tc := range pmMuteCases {
		for _, replyTo := range []struct {
			name string
			user func(f adminChatFixture) gotgbot.User
		}{
			{"to the bot", func(adminChatFixture) gotgbot.User { return bot }},
			{"to own message", func(f adminChatFixture) gotgbot.User { return f.admin }},
		} {
			t.Run(tc.cmd+"/"+replyTo.name, func(t *testing.T) {
				f, _ := pmBansFixture(t)

				processPMReply(t, mutesPMDispatcher(), f, f.admin, replyTo.user(f), "/"+tc.cmd+" @fixturegroup")

				if len(f.client.callsFor("restrictChatMember")) != 0 {
					t.Fatalf("/%s from PM in reply %s restricted someone", tc.cmd, replyTo.name)
				}
				if !repliedIn(f, f.admin.Id, translated(t, "common_no_user_specified")) {
					t.Fatalf("/%s from PM in reply %s: want common_no_user_specified; sent:\n%s", tc.cmd, replyTo.name, sentText(f.client))
				}
			})
		}
		t.Run(tc.cmd+"/with a user", func(t *testing.T) {
			f, _ := pmBansFixture(t)

			processPMReply(t, mutesPMDispatcher(), f, f.admin, bot, "/"+tc.cmd+" @fixturegroup @victim"+tc.args)

			if !tc.acted(t, f, f.chat.Id, victimID) {
				t.Fatalf("/%s from PM in reply to the bot did not act on the user named; sent:\n%s", tc.cmd, sentText(f.client))
			}
		})
	}
}

// From PM, /tmute reads the time after the user as in the group.
func TestTmuteInPMWithGroupReadsTimeAfterUser(t *testing.T) {
	f, _ := pmBansFixture(t)
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
	f, _ := pmBansFixture(t)

	processPM(t, mutesPMDispatcher(), f, f.admin.Id, "/smute @fixturegroup @victim")

	if calls := f.client.callsFor("deleteMessage"); len(calls) != 0 {
		t.Fatalf("/smute from PM deleted a message: %+v", calls)
	}
	if !repliedIn(f, f.admin.Id, "Victim") {
		t.Fatalf("/smute from PM did not answer in the PM; sent:\n%s", sentText(f.client))
	}
	buttonFor(t, f, 0, "unrestrict", "unmute")
}

// The unmute button a mute command sends from PM names the named group and
// acts on it; the presser is checked against that group, so a member, or an
// admin pressing a button forged to name a group where they have no right,
// is refused. Sent in the group, the button names no group.
func TestMuteButtonsFromPMActOnNamedGroup(t *testing.T) {
	for _, cmd := range []string{"mute", "smute"} {
		t.Run(cmd, func(t *testing.T) {
			f, connected := pmBansFixture(t)
			dispatcher := mutesPMDispatcher()
			processPM(t, dispatcher, f, f.admin.Id, "/"+cmd+" @fixturegroup @victim")
			data := buttonFor(t, f, 0, "unrestrict", "unmute")
			if decoded, _ := decodeCallbackData(data); decoded != nil {
				if c, _ := decoded.Field("c"); c != fmt.Sprint(f.chat.Id) {
					t.Fatalf("the unmute button from PM names group %q, want %d", c, f.chat.Id)
				}
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
		f, _ := pmBansFixture(t)
		data := encodeCallbackData("unrestrict", map[string]string{"a": "unmute", "c": fmt.Sprint(f.chat.Id), "u": fmt.Sprint(victimID)})

		pressConfirmInPM(t, mutesPMDispatcher(), f, f.member, data)

		if len(f.client.callsFor("restrictChatMember")) != 0 {
			t.Fatalf("a member's press of the unmute button acted")
		}
	})
	t.Run("forged group", func(t *testing.T) {
		f, _ := pmBansFixture(t)
		other := uniqueModuleChatID()
		f.setMemberStatus(other, f.admin.Id, "member")
		data := encodeCallbackData("unrestrict", map[string]string{"a": "unmute", "c": fmt.Sprint(other), "u": fmt.Sprint(victimID)})

		pressConfirmInPM(t, mutesPMDispatcher(), f, f.admin, data)

		if len(f.client.callsFor("restrictChatMember")) != 0 {
			t.Fatalf("an unmute button naming a group the presser has no right in acted")
		}
	})
	t.Run("in the group", func(t *testing.T) {
		f, _ := pmBansFixture(t)

		update := newModuleMessageContext(f.bot, f.chat, f.admin, "/mute @victim").Update
		if err := mutesPMDispatcher().ProcessUpdate(f.bot, update, nil); err != nil {
			t.Fatalf("ProcessUpdate(/mute) error = %v", err)
		}

		decoded, _ := decodeCallbackData(buttonFor(t, f, 0, "unrestrict", "unmute"))
		if c, named := decoded.Field("c"); named {
			t.Fatalf("/mute in the group sent an unmute button naming group %q", c)
		}
	})
}

// The help a mute command gives from PM shows a call with a group, both
// without a group and with a group but no user; in the group it shows the
// group's call, as before.
func TestMuteCommandsHelpShowsExampleWithGroupInPMOnly(t *testing.T) {
	for _, tc := range pmMuteCases {
		t.Run(tc.cmd, func(t *testing.T) {
			f, _ := pmBansFixture(t)
			useRealLocales(t)
			dispatcher := mutesPMDispatcher()
			pmExample, _ := i18n.MustNewTranslator("config").GetString(tc.cmd + "_pm_example")
			groupExample := commandExample(tc.cmd)
			if !strings.HasPrefix(pmExample, "/"+tc.cmd+" ") || pmExample == groupExample {
				t.Fatalf("%s_pm_example = %q, want a call of /%s with a group", tc.cmd, pmExample, tc.cmd)
			}
			lastReply := func() string {
				calls := f.client.callsFor("sendMessage")
				return fmt.Sprint(calls[len(calls)-1].Params["text"])
			}

			processPM(t, dispatcher, f, f.admin.Id, "/"+tc.cmd+" @fixturegroup")
			if text := lastReply(); !strings.Contains(text, translated(t, "common_no_user_specified")) ||
				!strings.Contains(text, html.EscapeString(pmExample)) || strings.Contains(text, html.EscapeString(groupExample)) {
				t.Fatalf("/%s from PM without a user replied %q, want no-user help with the example %q only", tc.cmd, text, pmExample)
			}

			update := newModuleMessageContext(f.bot, f.chat, f.admin, "/"+tc.cmd).Update
			if err := dispatcher.ProcessUpdate(f.bot, update, nil); err != nil {
				t.Fatalf("ProcessUpdate(/%s) error = %v", tc.cmd, err)
			}
			if text := lastReply(); !strings.Contains(text, html.EscapeString(groupExample)) || strings.Contains(text, html.EscapeString(pmExample)) {
				t.Fatalf("/%s help in the group %q, want the example %q only", tc.cmd, text, groupExample)
			}
		})
	}
}

// Every mute command sent from PM without a group and without a connection
// answers with its help, its PM example and the user's groups.
func TestMuteCommandsInPMWithoutGroupReplyHelp(t *testing.T) {
	f := titleFixture(t, "My Group")
	dispatcher := mutesPMDispatcher()
	pick := translated(t, "connections_pm_group_pick")
	for _, tc := range pmMuteCases {
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
