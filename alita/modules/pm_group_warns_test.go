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

	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/warns"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

func warnsPMDispatcher() *ext.Dispatcher {
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadWarns(dispatcher)
	return dispatcher
}

// pmWarnCase is a warns command that acts on one user of one group: @victim
// has seed warns in every group before it and want in the group it acts on
// after it.
type pmWarnCase struct {
	cmd, args  string
	seed, want int
}

var pmWarnCases = []pmWarnCase{
	{"warn", " spam", 0, 1},
	{"swarn", "", 0, 1},
	{"rmwarn", "", 2, 1},
	{"resetwarns", "", 2, 0},
}

// pmWarnsFixture is pmBansFixture with @victim holding seed warns in the
// fixture's group and in the connected one.
func pmWarnsFixture(t *testing.T, seed int) (f adminChatFixture, connected int64) {
	t.Helper()
	f, connected = pmModerationFixture(t)
	for _, chatID := range []int64{f.chat.Id, connected} {
		f.setMemberStatus(chatID, victimID, "member")
		ensureWarnParents(t, chatID, victimID)
		for range seed {
			if _, _, err := warns.WarnUser(victimID, chatID, models.WarnEntry{Reason: "seed"}); err != nil {
				t.Fatalf("WarnUser() error = %v", err)
			}
		}
	}
	return f, connected
}

// rmWarnButtons returns the data of the rmWarn buttons in the messages sent
// since call number from. A warn reply carries its keyboard by pointer.
func rmWarnButtons(f adminChatFixture, from int) []string {
	var buttons []string
	for _, call := range f.client.callsFor("sendMessage")[from:] {
		markup, ok := call.Params["reply_markup"].(*gotgbot.InlineKeyboardMarkup)
		if !ok || markup == nil {
			continue
		}
		for _, row := range markup.InlineKeyboard {
			for _, button := range row {
				if decoded, ok := decodeCallbackData(button.CallbackData, "rmWarn"); ok && decoded != nil {
					buttons = append(buttons, button.CallbackData)
				}
			}
		}
	}
	return buttons
}

// warnsOf returns the warns userID has in chatID.
func warnsOf(userID, chatID int64) int {
	num, _ := warns.GetWarns(userID, chatID)
	return num
}

// From PM with @fixturegroup while connected to another group, each warns
// command changes @victim's warns in the named group and answers in the PM,
// naming that group in HTML; the connected group and the connection stay as
// they were, and nothing is sent to the named group.
func TestWarnCommandsInPMWithGroupActOnThatGroup(t *testing.T) {
	for _, tc := range pmWarnCases {
		t.Run(tc.cmd, func(t *testing.T) {
			f, connected := pmWarnsFixture(t, tc.seed)

			processPM(t, warnsPMDispatcher(), f, f.admin.Id, "/"+tc.cmd+" @fixturegroup @victim"+tc.args)

			if got := warnsOf(victimID, f.chat.Id); got != tc.want {
				t.Fatalf("/%s from PM: @victim has %d warns in the named group, want %d; sent:\n%s", tc.cmd, got, tc.want, sentText(f.client))
			}
			if got := warnsOf(victimID, connected); got != tc.seed {
				t.Fatalf("/%s from PM changed the connected group's warns to %d", tc.cmd, got)
			}
			want := groupLine(t, chatTitleOf(f, f.chat.Id), f.chat.Id)
			found := false
			for _, call := range f.client.callsFor("sendMessage") {
				if fmt.Sprint(call.Params["chat_id"]) == fmt.Sprint(f.admin.Id) && strings.Contains(fmt.Sprint(call.Params["text"]), want) {
					found = found || fmt.Sprint(call.Params["parse_mode"]) == gotgbot.ParseModeHTML
				}
			}
			if !found {
				t.Fatalf("/%s from PM did not answer in the PM in HTML naming the group %q; sent:\n%s", tc.cmd, want, sentText(f.client))
			}
			requireNothingSentTo(t, f, f.chat.Id)
			requireConnectedTo(t, f.admin.Id, connected)
		})
	}
}

// With a connection and no group named, a warns command acts on @victim in
// the connected group and names it in the reply.
func TestWarnCommandsInPMOnConnectionActOnConnectedGroup(t *testing.T) {
	for _, tc := range pmWarnCases {
		t.Run(tc.cmd, func(t *testing.T) {
			f, connected := pmWarnsFixture(t, tc.seed)

			processPM(t, warnsPMDispatcher(), f, f.admin.Id, "/"+tc.cmd+" @victim"+tc.args)

			if got := warnsOf(victimID, connected); got != tc.want {
				t.Fatalf("/%s from PM on a connection: @victim has %d warns in the connected group, want %d; sent:\n%s", tc.cmd, got, tc.want, sentText(f.client))
			}
			if got := warnsOf(victimID, f.chat.Id); got != tc.seed {
				t.Fatalf("/%s from PM on a connection changed another group's warns to %d", tc.cmd, got)
			}
			if want := groupLine(t, chatTitleOf(f, connected), connected); !repliedIn(f, f.admin.Id, want) {
				t.Fatalf("/%s from PM on a connection did not name the connected group %q; sent:\n%s", tc.cmd, want, sentText(f.client))
			}
		})
	}
}

// From PM, a member of the named group who is no admin there gets a refusal,
// and the group and their connection stay as they were.
func TestWarnCommandsInPMWithGroupRefuseMember(t *testing.T) {
	for _, tc := range pmWarnCases {
		t.Run(tc.cmd, func(t *testing.T) {
			f, connected := pmWarnsFixture(t, tc.seed)
			if err := connections.ToggleAllowConnect(f.chat.Id, true); err != nil {
				t.Fatalf("ToggleAllowConnect() error = %v", err)
			}
			if err := connections.ConnectId(f.member.Id, connected); err != nil {
				t.Fatalf("ConnectId() error = %v", err)
			}
			t.Cleanup(func() { _ = connections.DisconnectId(f.member.Id) })

			processPM(t, warnsPMDispatcher(), f, f.member.Id, "/"+tc.cmd+" @fixturegroup @victim"+tc.args)

			if got := warnsOf(victimID, f.chat.Id); got != tc.seed {
				t.Fatalf("/%s from a member who is no admin changed the group's warns to %d", tc.cmd, got)
			}
			if !calledFor(f, "sendMessage", f.member.Id) {
				t.Fatalf("/%s from a member who is no admin got no answer", tc.cmd)
			}
			requireConnectedTo(t, f.member.Id, connected)
		})
	}
}

// From PM, an admin of the named group without the right to restrict there
// cannot warn, though they hold that right in the group they are connected
// to.
func TestWarnCommandsInPMWithGroupRefuseAdminWithoutRestrictRight(t *testing.T) {
	for _, tc := range pmWarnCases[:2] {
		t.Run(tc.cmd, func(t *testing.T) {
			f, connected := pmWarnsFixture(t, tc.seed)
			admin := adminMember(f.admin.Id, false)
			admin.CanRestrictMembers = false
			seedAdminCache(t, f.chat.Id, adminMember(999, true), admin)

			processPM(t, warnsPMDispatcher(), f, f.admin.Id, "/"+tc.cmd+" @fixturegroup @victim"+tc.args)

			if warnsOf(victimID, f.chat.Id) != tc.seed || warnsOf(victimID, connected) != tc.seed {
				t.Fatalf("/%s from an admin without the right changed a group's warns", tc.cmd)
			}
			if !repliedIn(f, f.admin.Id, translated(t, "chat_status_restrict_cmd_error")) {
				t.Fatalf("/%s from an admin without the right: want chat_status_restrict_cmd_error; sent:\n%s", tc.cmd, sentText(f.client))
			}
		})
	}
}

// From PM, a command the bot cannot carry out in the named group, where it is
// no admin, gets the answer it gets in that group, in the PM.
func TestWarnCommandsInPMWithGroupAnswerAsGroupWhenBotIsNoAdmin(t *testing.T) {
	for _, tc := range pmWarnCases {
		t.Run(tc.cmd, func(t *testing.T) {
			f, connected := pmWarnsFixture(t, tc.seed)
			setup := func() {
				seedAdminCache(t, f.chat.Id, adminMember(f.admin.Id, false))
				f.client.membersByChat[fmt.Sprintf("%d|999", f.chat.Id)] = json.RawMessage(
					`{"status":"member","user":{"id":999,"is_bot":true,"first_name":"Alita"}}`)
			}
			dispatcher := warnsPMDispatcher()

			setup()
			update := newModuleMessageContext(f.bot, f.chat, f.admin, "/"+tc.cmd+" @victim"+tc.args).Update
			if err := dispatcher.ProcessUpdate(f.bot, update, nil); err != nil {
				t.Fatalf("ProcessUpdate(/%s) error = %v", tc.cmd, err)
			}
			inGroup := f.client.callsFor("sendMessage")
			if len(inGroup) != 1 || warnsOf(victimID, f.chat.Id) != tc.seed {
				t.Fatalf("/%s in the group with the bot no admin: want one refusal; sent:\n%s", tc.cmd, sentText(f.client))
			}

			setup()
			processPM(t, dispatcher, f, f.admin.Id, "/"+tc.cmd+" @fixturegroup @victim"+tc.args)

			inPM := f.client.callsFor("sendMessage")[1:]
			if len(inPM) != 1 || fmt.Sprint(inPM[0].Params["text"]) != fmt.Sprint(inGroup[0].Params["text"]) ||
				fmt.Sprint(inPM[0].Params["chat_id"]) != fmt.Sprint(f.admin.Id) {
				t.Fatalf("/%s from PM answered differently from the group, or not in the PM; sent:\n%s", tc.cmd, sentText(f.client))
			}
			if warnsOf(victimID, f.chat.Id) != tc.seed || warnsOf(victimID, connected) != tc.seed {
				t.Fatalf("/%s from PM changed a group's warns though the bot is no admin of the named group", tc.cmd)
			}
		})
	}
}

// From PM, an admin of the named group who is no admin of the connected one
// cannot be warned; neither can the caller, an admin of the group.
func TestWarnCommandsInPMWithGroupProtectAdminsOfThatGroup(t *testing.T) {
	const groupAdminID = 4343
	for _, tc := range pmWarnCases[:2] {
		for _, target := range []struct {
			name string
			id   func(f adminChatFixture) int64
		}{
			{"admin of the named group", func(adminChatFixture) int64 { return groupAdminID }},
			{"caller", func(f adminChatFixture) int64 { return f.admin.Id }},
		} {
			t.Run(tc.cmd+"/"+target.name, func(t *testing.T) {
				f, connected := pmWarnsFixture(t, 0)
				seedAdminCache(t, f.chat.Id, adminMember(999, true), adminMember(f.admin.Id, false), adminMember(groupAdminID, false))
				id := target.id(f)
				ensureWarnParents(t, f.chat.Id, id)

				processPM(t, warnsPMDispatcher(), f, f.admin.Id, fmt.Sprintf("/%s @fixturegroup %d%s", tc.cmd, id, tc.args))

				if warnsOf(id, f.chat.Id) != 0 || warnsOf(id, connected) != 0 {
					t.Fatalf("/%s from PM warned the %s", tc.cmd, target.name)
				}
				if !repliedIn(f, f.admin.Id, translated(t, "warns_admin_warning_error")) {
					t.Fatalf("/%s from PM on the %s: want warns_admin_warning_error; sent:\n%s", tc.cmd, target.name, sentText(f.client))
				}
			})
		}
	}
}

// A reply in PM names no target: the command answers as it does in the group
// without one, and a user named after the group wins over the reply.
func TestWarnCommandsInPMWithGroupIgnoreReply(t *testing.T) {
	bot := gotgbot.User{Id: 999, IsBot: true, FirstName: "Alita"}
	for _, tc := range pmWarnCases {
		t.Run(tc.cmd+"/to the bot", func(t *testing.T) {
			f, _ := pmWarnsFixture(t, tc.seed)

			processPMReply(t, warnsPMDispatcher(), f, f.admin, bot, "/"+tc.cmd+" @fixturegroup")

			if warnsOf(victimID, f.chat.Id) != tc.seed || warnsOf(bot.Id, f.chat.Id) != 0 {
				t.Fatalf("/%s from PM in reply to the bot changed warns", tc.cmd)
			}
			if !repliedIn(f, f.admin.Id, translated(t, "common_no_user_specified")) {
				t.Fatalf("/%s from PM in reply to the bot: want common_no_user_specified; sent:\n%s", tc.cmd, sentText(f.client))
			}
		})
		t.Run(tc.cmd+"/to own message", func(t *testing.T) {
			f, _ := pmWarnsFixture(t, tc.seed)

			processPMReply(t, warnsPMDispatcher(), f, f.admin, f.admin, "/"+tc.cmd+" @fixturegroup")

			if !repliedIn(f, f.admin.Id, translated(t, "common_no_user_specified")) {
				t.Fatalf("/%s from PM in reply to own message: want common_no_user_specified; sent:\n%s", tc.cmd, sentText(f.client))
			}
		})
		t.Run(tc.cmd+"/with a user", func(t *testing.T) {
			f, _ := pmWarnsFixture(t, tc.seed)

			processPMReply(t, warnsPMDispatcher(), f, f.admin, bot, "/"+tc.cmd+" @fixturegroup @victim"+tc.args)

			if got := warnsOf(victimID, f.chat.Id); got != tc.want {
				t.Fatalf("/%s from PM in reply to the bot: @victim has %d warns, want %d; sent:\n%s", tc.cmd, got, tc.want, sentText(f.client))
			}
		})
	}
}

// /dwarn needs a reply to a group message, so it stays group-only: from PM it
// warns no one and answers as before.
func TestDwarnInPMStaysGroupOnly(t *testing.T) {
	f, connected := pmWarnsFixture(t, 0)

	requireGroupOnlyInPM(t, warnsPMDispatcher, f, "dwarn")

	if warnsOf(victimID, f.chat.Id) != 0 || warnsOf(victimID, connected) != 0 {
		t.Fatalf("/dwarn from PM warned @victim: %d in the named group, %d in the connected one", warnsOf(victimID, f.chat.Id), warnsOf(victimID, connected))
	}
}

// From PM, /swarn answers like /warn: there is no command message in the
// group to delete.
func TestSwarnInPMWithGroupAnswersLikeWarn(t *testing.T) {
	f, _ := pmWarnsFixture(t, 0)

	processPM(t, warnsPMDispatcher(), f, f.admin.Id, "/swarn @fixturegroup @victim")

	if len(f.client.callsFor("deleteMessage")) != 0 {
		t.Fatalf("/swarn from PM deleted a message")
	}
	if !calledFor(f, "sendMessage", f.admin.Id) || len(rmWarnButtons(f, 0)) != 1 {
		t.Fatalf("/swarn from PM did not answer in the PM like /warn; sent:\n%s", sentText(f.client))
	}
}

// From PM, the warn that reaches the named group's limit punishes @victim in
// that group and answers in the PM, naming the group.
func TestWarnInPMWithGroupPunishesAtLimitInThatGroup(t *testing.T) {
	f, connected := pmWarnsFixture(t, 0)
	if err := warns.SetWarnLimit(f.chat.Id, 1); err != nil {
		t.Fatalf("SetWarnLimit() error = %v", err)
	}
	if err := warns.SetWarnMode(f.chat.Id, "ban", 0); err != nil {
		t.Fatalf("SetWarnMode() error = %v", err)
	}

	processPM(t, warnsPMDispatcher(), f, f.admin.Id, "/warn @fixturegroup @victim spam")

	if !targetedIn(f, "banChatMember", f.chat.Id, victimID) || targetedIn(f, "banChatMember", connected, victimID) {
		t.Fatalf("/warn from PM at the limit: banChatMember %+v, want @victim banned in the named group only", f.client.callsFor("banChatMember"))
	}
	if warnsOf(victimID, f.chat.Id) != 0 {
		t.Fatalf("/warn from PM at the limit kept the warns")
	}
	if want := groupLine(t, chatTitleOf(f, f.chat.Id), f.chat.Id); !repliedIn(f, f.admin.Id, want) {
		t.Fatalf("/warn from PM at the limit did not answer in the PM naming the group; sent:\n%s", sentText(f.client))
	}
	requireNothingSentTo(t, f, f.chat.Id)
}

// The remove-warn button /warn sends from PM names the group and removes the
// warn there when an admin presses it; one sent in the group names none.
func TestRmWarnButtonFromPMActsOnNamedGroup(t *testing.T) {
	f, connected := pmWarnsFixture(t, 1)
	dispatcher := warnsPMDispatcher()

	processPM(t, dispatcher, f, f.admin.Id, "/warn @fixturegroup @victim")

	buttons := rmWarnButtons(f, 0)
	if len(buttons) != 1 {
		t.Fatalf("/warn from PM sent rmWarn buttons %v, want one; sent:\n%s", buttons, sentText(f.client))
	}
	data := buttons[0]
	decoded, _ := decodeCallbackData(data)
	if c, _ := decoded.Field("c"); c != fmt.Sprint(f.chat.Id) || len(data) > 64 {
		t.Fatalf("rmWarn button from PM %q, want it to name group %d within 64 bytes", data, f.chat.Id)
	}

	pressConfirmInPM(t, dispatcher, f, f.admin, data)

	if warnsOf(victimID, f.chat.Id) != 1 || warnsOf(victimID, connected) != 1 {
		t.Fatalf("the rmWarn button from PM left warns %d in the named group and %d in the connected one, want 1 and 1",
			warnsOf(victimID, f.chat.Id), warnsOf(victimID, connected))
	}

	before := len(f.client.callsFor("sendMessage"))
	update := newModuleMessageContext(f.bot, f.chat, f.admin, "/warn @victim").Update
	if err := dispatcher.ProcessUpdate(f.bot, update, nil); err != nil {
		t.Fatalf("ProcessUpdate(/warn) error = %v", err)
	}
	inGroup := rmWarnButtons(f, before)
	if len(inGroup) != 1 {
		t.Fatalf("/warn in the group sent rmWarn buttons %v, want one", inGroup)
	}
	decoded, _ = decodeCallbackData(inGroup[0])
	if c, named := decoded.Field("c"); named {
		t.Fatalf("/warn in the group sent a button naming group %q: %s", c, inGroup[0])
	}
}

// The rmWarn button checks the presser against the group it names: a member
// there, or an admin pressing a button forged to name a group where they are
// no admin, is refused.
func TestRmWarnButtonFromPMChecksPresserInNamedGroup(t *testing.T) {
	t.Run("member", func(t *testing.T) {
		f, _ := pmWarnsFixture(t, 1)
		data := encodeCallbackData("rmWarn", map[string]string{"c": fmt.Sprint(f.chat.Id), "u": fmt.Sprint(victimID)})

		pressConfirmInPM(t, warnsPMDispatcher(), f, f.member, data)

		if warnsOf(victimID, f.chat.Id) != 1 {
			t.Fatalf("a member's press of the rmWarn button removed a warn")
		}
		if len(f.client.callsFor("answerCallbackQuery")) == 0 {
			t.Fatalf("a member's press of the rmWarn button got no answer")
		}
	})
	t.Run("forged group", func(t *testing.T) {
		f, _ := pmWarnsFixture(t, 0)
		other := uniqueModuleChatID()
		f.setMemberStatus(other, f.admin.Id, "member")
		ensureWarnParents(t, other, victimID)
		if _, _, err := warns.WarnUser(victimID, other, models.WarnEntry{Reason: "seed"}); err != nil {
			t.Fatalf("WarnUser() error = %v", err)
		}
		data := encodeCallbackData("rmWarn", map[string]string{"c": fmt.Sprint(other), "u": fmt.Sprint(victimID)})

		pressConfirmInPM(t, warnsPMDispatcher(), f, f.admin, data)

		if warnsOf(victimID, other) != 1 {
			t.Fatalf("an rmWarn button naming a group the presser is no admin of removed a warn")
		}
	})
}

// From PM, a user who is no member of the named group is not warned there,
// though they are a member of the connected group and of the PM.
func TestWarnCommandsInPMWithGroupIgnoreNonMember(t *testing.T) {
	for _, tc := range pmWarnCases[:2] {
		t.Run(tc.cmd, func(t *testing.T) {
			f, connected := pmWarnsFixture(t, 0)
			f.setMemberStatus(f.chat.Id, victimID, "left")
			f.setMemberStatus(f.admin.Id, victimID, "member")

			processPM(t, warnsPMDispatcher(), f, f.admin.Id, "/"+tc.cmd+" @fixturegroup @victim"+tc.args)

			if warnsOf(victimID, f.chat.Id) != 0 || warnsOf(victimID, connected) != 0 {
				t.Fatalf("/%s from PM warned a user who is no member of the named group: %d in it, %d in the connected one",
					tc.cmd, warnsOf(victimID, f.chat.Id), warnsOf(victimID, connected))
			}
		})
	}
}

// The rmWarn button from PM needs the bot to be an admin of the group it
// names, not of the PM it is pressed in.
func TestRmWarnButtonFromPMRefusedWhenBotIsNoAdminOfNamedGroup(t *testing.T) {
	f, _ := pmWarnsFixture(t, 1)
	seedAdminCache(t, f.chat.Id, adminMember(f.admin.Id, false))
	f.client.membersByChat[fmt.Sprintf("%d|999", f.chat.Id)] = json.RawMessage(
		`{"status":"member","user":{"id":999,"is_bot":true,"first_name":"Alita"}}`)
	data := encodeCallbackData("rmWarn", map[string]string{"c": fmt.Sprint(f.chat.Id), "u": fmt.Sprint(victimID)})

	pressConfirmInPM(t, warnsPMDispatcher(), f, f.admin, data)

	if warnsOf(victimID, f.chat.Id) != 1 {
		t.Fatalf("the rmWarn button from PM removed a warn in a group where the bot is no admin")
	}
	if !repliedIn(f, f.admin.Id, translated(t, "chat_status_bot_not_admin")) {
		t.Fatalf("the rmWarn button from PM with the bot no admin: want chat_status_bot_not_admin; sent:\n%s", sentText(f.client))
	}
}

// An rmWarn button whose group is not a chat ID is answered as invalid and
// removes no warn, neither in a group nor in the chat it is pressed in: the
// PM, or a group where the presser is an admin.
func TestRmWarnButtonWithMalformedGroupIsInvalid(t *testing.T) {
	for _, where := range []struct {
		name string
		chat func(f adminChatFixture) gotgbot.Chat
	}{
		{"in the PM", func(f adminChatFixture) gotgbot.Chat { return privateChatOf(f.admin) }},
		{"in a group", func(f adminChatFixture) gotgbot.Chat { return f.chat }},
	} {
		t.Run(where.name, func(t *testing.T) {
			f, connected := pmWarnsFixture(t, 1)
			ensureWarnParents(t, f.admin.Id, victimID)
			if _, _, err := warns.WarnUser(victimID, f.admin.Id, models.WarnEntry{Reason: "seed"}); err != nil {
				t.Fatalf("WarnUser() error = %v", err)
			}
			data := encodeCallbackData("rmWarn", map[string]string{"c": "abc", "u": fmt.Sprint(victimID)})

			update := newModuleCallbackContext(f.bot, where.chat(f), f.admin, data).Update
			if err := warnsPMDispatcher().ProcessUpdate(f.bot, update, nil); err != nil {
				t.Fatalf("ProcessUpdate(callback %q) error = %v", data, err)
			}

			for _, chatID := range []int64{f.chat.Id, connected, f.admin.Id} {
				if got := warnsOf(victimID, chatID); got != 1 {
					t.Fatalf("an rmWarn button with a malformed group pressed %s left %d warns in chat %d, want 1", where.name, got, chatID)
				}
			}
			want := translated(t, "common_callback_invalid_request")
			answered := false
			for _, call := range f.client.callsFor("answerCallbackQuery") {
				answered = answered || fmt.Sprint(call.Params["text"]) == want
			}
			if !answered {
				t.Fatalf("an rmWarn button with a malformed group pressed %s was not answered %q", where.name, want)
			}
		})
	}
}

// The group line in a warns reply from PM is HTML-escaped; sent in the
// group, the reply names no group.
func TestWarnCommandsInPMNameGroupInReply(t *testing.T) {
	for _, tc := range pmWarnCases {
		t.Run(tc.cmd, func(t *testing.T) {
			f, _ := pmWarnsFixture(t, tc.seed+1)
			named := `Tom & <Jerry>`
			for _, key := range []string{fmt.Sprint(f.chat.Id), "@fixturegroup"} {
				f.client.chatsByID[key] = json.RawMessage(fmt.Sprintf(`{"id":%d,"type":"supergroup","title":%q}`, f.chat.Id, named))
			}
			dispatcher := warnsPMDispatcher()

			processPM(t, dispatcher, f, f.admin.Id, "/"+tc.cmd+" @fixturegroup @victim"+tc.args)
			if want := groupLine(t, named, f.chat.Id); !repliedIn(f, f.admin.Id, want) {
				t.Fatalf("/%s @fixturegroup from PM did not name the group %q; sent:\n%s", tc.cmd, want, sentText(f.client))
			}

			before := len(f.client.callsFor("sendMessage"))
			update := newModuleMessageContext(f.bot, f.chat, f.admin, "/"+tc.cmd+" @victim"+tc.args).Update
			if err := dispatcher.ProcessUpdate(f.bot, update, nil); err != nil {
				t.Fatalf("ProcessUpdate(/%s) error = %v", tc.cmd, err)
			}
			prefix := strings.SplitN(translated(t, "connections_pm_group_confirm_target"), "{", 2)[0]
			for _, call := range f.client.callsFor("sendMessage")[before:] {
				if strings.Contains(fmt.Sprint(call.Params["text"]), prefix) {
					t.Fatalf("/%s in the group named a group: %q", tc.cmd, call.Params["text"])
				}
			}
		})
	}
}

// Without a user, a warns command answers from PM with its help, showing a
// call with a group.
func TestWarnCommandsHelpShowsExampleWithGroupInPMOnly(t *testing.T) {
	for _, tc := range pmWarnCases {
		t.Run(tc.cmd, func(t *testing.T) {
			f, _ := pmWarnsFixture(t, 0)
			useRealLocales(t)
			dispatcher := warnsPMDispatcher()
			pmExample, _ := i18n.MustNewTranslator("config").GetString(tc.cmd + "_pm_example")
			groupExample := commandExample(tc.cmd)
			if !strings.HasPrefix(pmExample, "/"+tc.cmd+" ") || !strings.HasPrefix(groupExample, "/"+tc.cmd+" ") || pmExample == groupExample {
				t.Fatalf("/%s examples %q (PM) and %q (group), want two calls of /%s, the PM one with a group", tc.cmd, pmExample, groupExample, tc.cmd)
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
		})
	}
}

// isNoUserGroupReply reports whether call answers the command message (101) in
// the group as before PM support: an HTML reply with the no-user error alone.
func isNoUserGroupReply(t *testing.T, f adminChatFixture, call moduleBotCall) bool {
	t.Helper()
	reply, _ := call.Params["reply_parameters"].(*gotgbot.ReplyParameters)
	return fmt.Sprint(call.Params["chat_id"]) == fmt.Sprint(f.chat.Id) &&
		fmt.Sprint(call.Params["text"]) == translated(t, "common_no_user_specified") &&
		fmt.Sprint(call.Params["parse_mode"]) == gotgbot.ParseModeHTML &&
		reply != nil && reply.MessageId == 101
}

// In the group, a warn command without a user answers as before PM support:
// with the no-user error alone, no help or example.
func TestWarnCommandsWithoutUserInGroupAnswerAsBefore(t *testing.T) {
	for _, cmd := range []string{"warn", "swarn", "dwarn", "rmwarn", "unwarn", "resetwarns", "resetwarn"} {
		t.Run(cmd, func(t *testing.T) {
			f, _ := pmWarnsFixture(t, 0)
			useRealLocales(t)

			update := newModuleMessageContext(f.bot, f.chat, f.admin, "/"+cmd).Update
			if err := warnsPMDispatcher().ProcessUpdate(f.bot, update, nil); err != nil {
				t.Fatalf("ProcessUpdate(/%s) error = %v", cmd, err)
			}

			replies := f.client.callsFor("sendMessage")
			if len(replies) != 1 || !isNoUserGroupReply(t, f, replies[0]) {
				t.Fatalf("/%s in the group without a user: want only an HTML reply %q; sent %+v", cmd, translated(t, "common_no_user_specified"), replies)
			}
		})
	}
}

// Every warns command sent from PM without a group and without a connection
// answers with its help, its PM example and the user's groups.
func TestWarnCommandsInPMWithoutGroupReplyHelp(t *testing.T) {
	f := titleFixture(t, "My Group")
	dispatcher := warnsPMDispatcher()
	pick := translated(t, "connections_pm_group_pick")
	for _, tc := range pmWarnCases {
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
