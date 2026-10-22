//go:build testtools

package modules

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/warns"
)

func warnsPMDispatcher() *ext.Dispatcher {
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadWarns(dispatcher)
	return dispatcher
}

// pmSeedWarns is how many warns @victim holds in every group before a
// command that removes warns.
const pmSeedWarns = 2

// victimWarns readies @victim, a member of the fixture's group and of the
// connected one, holding seed warns in both.
func victimWarns(seed int) func(t *testing.T, f adminChatFixture, connected int64) {
	return func(t *testing.T, f adminChatFixture, connected int64) {
		t.Helper()
		for _, chatID := range []int64{f.chat.Id, connected} {
			f.setMemberStatus(chatID, victimID, "member")
			ensureWarnParents(t, chatID, victimID)
			for range seed {
				if _, _, err := warns.WarnUser(victimID, chatID, models.WarnEntry{Reason: "seed"}); err != nil {
					t.Fatalf("WarnUser() error = %v", err)
				}
			}
		}
	}
}

// pmWarnsFixture is pmModerationFixture with @victim holding seed warns in
// the fixture's group and in the connected one.
func pmWarnsFixture(t *testing.T, seed int) (f adminChatFixture, connected int64) {
	t.Helper()
	f, connected = pmModerationFixture(t)
	victimWarns(seed)(t, f, connected)
	return f, connected
}

// warnedNow reports whether userID, who held no warns, has one in chatID.
func warnedNow(_ *testing.T, _ adminChatFixture, chatID, userID int64) bool {
	return warnsOf(userID, chatID) > 0
}

// seedWarnsLeft reports whether @victim, the only user holding seed warns,
// has left of the pmSeedWarns in chatID.
func seedWarnsLeft(left int) func(*testing.T, adminChatFixture, int64, int64) bool {
	return func(_ *testing.T, _ adminChatFixture, chatID, userID int64) bool {
		return userID == victimID && warnsOf(userID, chatID) == left
	}
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
	for _, tc := range []struct{ cmd, args string }{{"warn", " spam"}, {"swarn", ""}} {
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
