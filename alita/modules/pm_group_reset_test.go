//go:build testtools

package modules

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"strconv"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/approvals"
	"github.com/divkix/Alita_Robot/alita/db/blacklists"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/filters"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/notes"
	"github.com/divkix/Alita_Robot/alita/db/warns"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

func pmResetDispatcher() *ext.Dispatcher {
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadNotes(dispatcher)
	LoadFilters(dispatcher)
	LoadBlacklists(dispatcher)
	LoadApprovals(dispatcher)
	LoadWarns(dispatcher)
	LoadBans(dispatcher)
	return dispatcher
}

// pmResetCase is a group-only command that resets a group's data. ns is the
// callback namespace of its confirmation; seed puts data in a group; intact
// reports whether it is still there.
type pmResetCase struct {
	cmd, ns string
	args    string
	seed    func(t *testing.T, f adminChatFixture, chatID int64)
	intact  func(f adminChatFixture, chatID int64) bool
}

var pmResetCases = []pmResetCase{
	{"clearall", "rmAllNotes", "",
		func(t *testing.T, _ adminChatFixture, id int64) {
			if err := notes.AddNote(id, "rules", "Be kind", "", nil, db.TEXT, false, false, false, true, false, false); err != nil {
				t.Fatalf("AddNote(%d) error = %v", id, err)
			}
		},
		func(_ adminChatFixture, id int64) bool { return len(notes.GetNotesList(id, true)) > 0 }},
	{"stopall", "rmAllFilters", "",
		func(t *testing.T, _ adminChatFixture, id int64) {
			if err := filters.AddFilter(id, "hello", "Hi there", "", nil, db.TEXT); err != nil {
				t.Fatalf("AddFilter(%d) error = %v", id, err)
			}
		},
		func(_ adminChatFixture, id int64) bool { return len(filters.GetFiltersList(id)) > 0 }},
	{"remallbl", "rmAllBlacklist", "",
		func(t *testing.T, _ adminChatFixture, id int64) {
			if err := blacklists.AddBlacklist(id, "spam"); err != nil {
				t.Fatalf("AddBlacklist(%d) error = %v", id, err)
			}
		},
		func(_ adminChatFixture, id int64) bool {
			return len(blacklists.GetBlacklistSettings(id).Triggers()) > 0
		}},
	{"unapproveall", "rmAllApprovals", "",
		func(t *testing.T, _ adminChatFixture, id int64) {
			if err := approvals.AddApprovedUser(id, 43, 777000, "trusted"); err != nil {
				t.Fatalf("AddApprovedUser(%d) error = %v", id, err)
			}
		},
		func(_ adminChatFixture, id int64) bool { return approvals.IsUserApproved(id, 43) }},
	{"resetallwarns", "rmAllChatWarns", "",
		func(t *testing.T, _ adminChatFixture, id int64) {
			if _, _, err := warns.WarnUser(43, id, models.WarnEntry{Reason: "spam"}); err != nil {
				t.Fatalf("WarnUser(%d) error = %v", id, err)
			}
		},
		func(_ adminChatFixture, id int64) bool { return warns.GetAllChatWarns(id) > 0 }},
	{"unbanall", "unbanall", "",
		func(t *testing.T, _ adminChatFixture, id int64) {
			if err := chats.AddChatMember(id, 43); err != nil {
				t.Fatalf("seed member 43 of %d: %v", id, err)
			}
		},
		func(f adminChatFixture, id int64) bool {
			for _, call := range f.client.callsFor("unbanChatMember") {
				if fmt.Sprint(call.Params["chat_id"]) == fmt.Sprint(id) {
					return false
				}
			}
			return true
		}},
	{"privnote", "", " on", nil,
		func(_ adminChatFixture, id int64) bool { return !notes.GetNotes(id).PrivateNotesEnabled() }},
}

// pmResetFixture is pmSettingsFixture with data seeded in both groups.
func pmResetFixture(t *testing.T, tc pmResetCase) (f adminChatFixture, connected int64) {
	t.Helper()
	f, connected = pmSettingsFixture(t)
	f.client.chatsByID[fmt.Sprint(connected)] = json.RawMessage(fmt.Sprintf(`{"id":%d,"type":"supergroup","title":"Group B"}`, connected))
	f.client.adminsByChat[fmt.Sprint(connected)] = adminListJSON(f.admin.Id)
	if f.client.membersByChat == nil {
		f.client.membersByChat = map[string]json.RawMessage{}
	}
	for _, id := range []int64{f.chat.Id, connected} {
		if tc.seed != nil {
			tc.seed(t, f, id)
		}
	}
	return f, connected
}

// setMemberStatus answers getChatMember for userID in chatID with status.
func (f adminChatFixture) setMemberStatus(chatID, userID int64, status string) {
	f.client.membersByChat[fmt.Sprintf("%d|%d", chatID, userID)] = json.RawMessage(
		fmt.Sprintf(`{"status":%q,"user":{"id":%d,"is_bot":false,"first_name":"User"}}`, status, userID))
}

// confirmButtons returns the data of the "yes" buttons in the messages sent
// since call number from.
func confirmButtons(f adminChatFixture, from int) []string {
	return buttonsWithAction(f, from, "yes")
}

// buttonsWithAction returns the data of the buttons whose "a" field is action
// in the messages sent since call number from.
func buttonsWithAction(f adminChatFixture, from int, action string) []string {
	var yes []string
	for _, call := range f.client.callsFor("sendMessage")[from:] {
		markup, ok := call.Params["reply_markup"].(gotgbot.InlineKeyboardMarkup)
		if !ok {
			continue
		}
		for _, row := range markup.InlineKeyboard {
			for _, button := range row {
				if decoded, ok := decodeCallbackData(button.CallbackData); ok {
					if a, _ := decoded.Field("a"); a == action {
						yes = append(yes, button.CallbackData)
					}
				}
			}
		}
	}
	return yes
}

// pressConfirmInPM runs a button press with data by user in their PM.
func pressConfirmInPM(t *testing.T, dispatcher *ext.Dispatcher, f adminChatFixture, user gotgbot.User, data string) {
	t.Helper()
	update := newModuleCallbackContext(f.bot, privateChatOf(user), user, data).Update
	if err := dispatcher.ProcessUpdate(f.bot, update, nil); err != nil {
		t.Fatalf("ProcessUpdate(callback %q) error = %v", data, err)
	}
}

// runResetInPM sends tc from PM as user with the group named, then presses
// every confirmation it got.
func runResetInPM(t *testing.T, dispatcher *ext.Dispatcher, f adminChatFixture, user gotgbot.User, tc pmResetCase) (buttons []string) {
	t.Helper()
	before := len(f.client.callsFor("sendMessage"))
	processPM(t, dispatcher, f, user.Id, "/"+tc.cmd+" @fixturegroup"+tc.args)
	buttons = confirmButtons(f, before)
	for _, data := range buttons {
		pressConfirmInPM(t, dispatcher, f, user, data)
	}
	return buttons
}

func requireConnectedTo(t *testing.T, userID, connected int64) {
	t.Helper()
	if conn := connections.Connection(userID); !conn.Connected || conn.ChatId != connected {
		t.Fatalf("connection of %d = %+v, want it kept on %d", userID, conn, connected)
	}
}

// From PM with @fixturegroup while connected to another group, the owner's
// command, once confirmed, resets the named group and leaves the connected
// one alone.
func TestResetCommandsInPMWithGroupResetThatGroup(t *testing.T) {
	for _, tc := range pmResetCases {
		t.Run(tc.cmd, func(t *testing.T) {
			f, connected := pmResetFixture(t, tc)
			f.makeOwner()
			dispatcher := pmResetDispatcher()

			buttons := runResetInPM(t, dispatcher, f, f.admin, tc)

			if tc.cmd != "privnote" && len(buttons) != 1 {
				t.Fatalf("/%s from PM sent %d confirmations, want one; sent:\n%s", tc.cmd, len(buttons), sentText(f.client))
			}
			if tc.intact(f, f.chat.Id) {
				t.Fatalf("/%s from PM left the named group as it was; sent:\n%s", tc.cmd, sentText(f.client))
			}
			if !tc.intact(f, connected) {
				t.Fatalf("/%s from PM changed the connected group", tc.cmd)
			}
			requireConnectedTo(t, f.admin.Id, connected)
		})
	}
}

// The cancel button of a confirmation sent from PM names the group too, and
// pressing it there leaves both groups as they were.
func TestResetCommandsInPMCancelLeavesGroups(t *testing.T) {
	for _, tc := range pmResetCases {
		if tc.cmd == "privnote" {
			continue
		}
		t.Run(tc.cmd, func(t *testing.T) {
			f, connected := pmResetFixture(t, tc)
			f.makeOwner()
			dispatcher := pmResetDispatcher()

			processPM(t, dispatcher, f, f.admin.Id, "/"+tc.cmd+" @fixturegroup"+tc.args)
			cancels := buttonsWithAction(f, 0, "no")
			if len(cancels) != 1 {
				t.Fatalf("/%s from PM sent %d cancel buttons, want one; sent:\n%s", tc.cmd, len(cancels), sentText(f.client))
			}
			decoded, _ := decodeCallbackData(cancels[0])
			if c, _ := decoded.Field("c"); c != fmt.Sprint(f.chat.Id) {
				t.Fatalf("/%s cancel button names group %q, want %d", tc.cmd, c, f.chat.Id)
			}

			pressConfirmInPM(t, dispatcher, f, f.admin, cancels[0])

			if !tc.intact(f, f.chat.Id) || !tc.intact(f, connected) {
				t.Fatalf("/%s cancel from PM changed a group", tc.cmd)
			}
			if len(f.client.callsFor("editMessageText")) == 0 && len(f.client.callsFor("answerCallbackQuery")) == 0 {
				t.Fatalf("/%s cancel from PM got no answer", tc.cmd)
			}
		})
	}
}

// From PM, an admin who does not own the named group gets no confirmation
// for an owner-only reset, though Telegram calls them the owner of every
// other chat (the connected group, the PM), and a member who is no admin
// changes nothing either. /privnote is for any admin, so only the member
// case applies to it.
func TestResetCommandsInPMWithGroupRefuseWithoutRights(t *testing.T) {
	for _, tc := range pmResetCases {
		if tc.cmd != "privnote" {
			t.Run(tc.cmd+"/admin", func(t *testing.T) {
				f, connected := pmResetFixture(t, tc)
				f.makeOwner()
				f.setMemberStatus(f.chat.Id, f.admin.Id, "administrator")

				if buttons := runResetInPM(t, pmResetDispatcher(), f, f.admin, tc); len(buttons) != 0 {
					t.Fatalf("an admin who does not own the group got the /%s confirmation", tc.cmd)
				}
				if !tc.intact(f, f.chat.Id) || !tc.intact(f, connected) {
					t.Fatalf("/%s from an admin who does not own the group changed a group; sent:\n%s", tc.cmd, sentText(f.client))
				}
				requireConnectedTo(t, f.admin.Id, connected)
			})
		}
		t.Run(tc.cmd+"/member", func(t *testing.T) {
			f, connected := pmResetFixture(t, tc)
			if err := connections.ToggleAllowConnect(f.chat.Id, true); err != nil {
				t.Fatalf("ToggleAllowConnect() error = %v", err)
			}
			if err := connections.ConnectId(f.member.Id, connected); err != nil {
				t.Fatalf("ConnectId() error = %v", err)
			}
			t.Cleanup(func() { _ = connections.DisconnectId(f.member.Id) })

			if buttons := runResetInPM(t, pmResetDispatcher(), f, f.member, tc); len(buttons) != 0 {
				t.Fatalf("a member who is not an admin got the /%s confirmation", tc.cmd)
			}
			if !tc.intact(f, f.chat.Id) {
				t.Fatalf("/%s from a member changed the named group; sent:\n%s", tc.cmd, sentText(f.client))
			}
			requireConnectedTo(t, f.member.Id, connected)
		})
	}
}

// A confirmation sent in the owner's PM resets nothing when pressed by
// someone else, or when its data names a group the presser does not own.
func TestResetConfirmationFromPMChecksPresser(t *testing.T) {
	for _, tc := range pmResetCases {
		if tc.cmd == "privnote" {
			continue
		}
		t.Run(tc.cmd+"/other user", func(t *testing.T) {
			f, _ := pmResetFixture(t, tc)
			f.makeOwner()
			dispatcher := pmResetDispatcher()
			before := len(f.client.callsFor("sendMessage"))
			processPM(t, dispatcher, f, f.admin.Id, "/"+tc.cmd+" @fixturegroup")
			buttons := confirmButtons(f, before)
			if len(buttons) != 1 {
				t.Fatalf("/%s from PM sent %d confirmations, want one", tc.cmd, len(buttons))
			}

			pressConfirmInPM(t, dispatcher, f, f.member, buttons[0])

			if !tc.intact(f, f.chat.Id) {
				t.Fatalf("a member's press of the /%s confirmation reset the group", tc.cmd)
			}
		})
		t.Run(tc.cmd+"/other group", func(t *testing.T) {
			f, other := pmResetFixture(t, tc)
			f.makeOwner()
			f.setMemberStatus(other, f.admin.Id, "member")
			data := encodeCallbackData(tc.ns, map[string]string{"a": "yes", "c": strconv.FormatInt(other, 10)})

			pressConfirmInPM(t, pmResetDispatcher(), f, f.admin, data)

			if !tc.intact(f, other) {
				t.Fatalf("a /%s confirmation naming a group the presser does not own reset it", tc.cmd)
			}
		})
		// Pressed in a group the presser owns, a button naming another group
		// is checked against the group it names, not the one it is in.
		t.Run(tc.cmd+"/other group pressed in own group", func(t *testing.T) {
			f, other := pmResetFixture(t, tc)
			f.makeOwner()
			f.setMemberStatus(other, f.admin.Id, "member")
			data := encodeCallbackData(tc.ns, map[string]string{"a": "yes", "c": strconv.FormatInt(other, 10)})

			update := newModuleCallbackContext(f.bot, f.chat, f.admin, data).Update
			if err := pmResetDispatcher().ProcessUpdate(f.bot, update, nil); err != nil {
				t.Fatalf("ProcessUpdate(callback %q) error = %v", data, err)
			}

			if !tc.intact(f, other) {
				t.Fatalf("a /%s confirmation pressed in the presser's own group reset another group it names", tc.cmd)
			}
			if !tc.intact(f, f.chat.Id) {
				t.Fatalf("a /%s confirmation naming another group reset the group it was pressed in", tc.cmd)
			}
		})
	}
}

// Every reset command sent from PM without a group and without a connection
// answers with its help, its example and the user's groups.
func TestResetCommandsInPMWithoutGroupReplyHelp(t *testing.T) {
	f := titleFixture(t, "My Group")
	dispatcher := pmResetDispatcher()
	pick := translated(t, "connections_pm_group_pick")
	for _, tc := range pmResetCases {
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

// confirmationText returns the text of the one confirmation sent since call
// number from.
func confirmationText(t *testing.T, f adminChatFixture, from int) string {
	t.Helper()
	var texts []string
	for _, call := range f.client.callsFor("sendMessage")[from:] {
		if _, ok := call.Params["reply_markup"].(gotgbot.InlineKeyboardMarkup); ok {
			texts = append(texts, fmt.Sprint(call.Params["text"]))
		}
	}
	if len(texts) != 1 {
		t.Fatalf("sent %d confirmations, want one; sent:\n%s", len(texts), sentText(f.client))
	}
	return texts[0]
}

// confirmQuestionKeys are the locale keys of the reset confirmations.
var confirmQuestionKeys = map[string]string{
	"clearall":      "notes_clear_all_confirm",
	"stopall":       "filters_clear_all_confirm",
	"remallbl":      "blacklists_rm_all_bl_ask",
	"unapproveall":  "approvals_unapproveall_ask",
	"resetallwarns": "warns_reset_all_confirm",
	"unbanall":      "bans_unbanall_ask",
}

// From PM, the group line names a title with HTML characters escaped exactly
// once in an HTML reply (/ban) and as typed in a plain-text confirmation
// (/stopall), so both show it as typed.
func TestPMGroupLineEscapesOnlyHTMLReplies(t *testing.T) {
	f, _ := pmModerationFixture(t)
	f.makeOwner()
	named := `Tom & <Jerry>`
	for _, key := range []string{fmt.Sprint(f.chat.Id), "@fixturegroup"} {
		f.client.chatsByID[key] = json.RawMessage(fmt.Sprintf(`{"id":%d,"type":"supergroup","title":%q}`, f.chat.Id, named))
	}
	if err := filters.AddFilter(f.chat.Id, "hello", "Hi there", "", nil, db.TEXT); err != nil {
		t.Fatalf("AddFilter() error = %v", err)
	}
	dispatcher := pmResetDispatcher()
	line := strings.NewReplacer("{id}", fmt.Sprint(f.chat.Id)).Replace(translated(t, "connections_pm_group_confirm_target"))

	for _, tc := range []struct {
		text, parseMode, title string
	}{
		{"/ban @fixturegroup @victim", gotgbot.ParseModeHTML, html.EscapeString(named)},
		{"/stopall @fixturegroup", "", named},
	} {
		before := len(f.client.callsFor("sendMessage"))
		processPM(t, dispatcher, f, f.admin.Id, tc.text)

		replies := f.client.callsFor("sendMessage")[before:]
		if len(replies) != 1 {
			t.Fatalf("%s from PM sent %d replies, want one; sent:\n%s", tc.text, len(replies), sentText(f.client))
		}
		text, mode := fmt.Sprint(replies[0].Params["text"]), replies[0].Params["parse_mode"]
		if mode == nil {
			mode = ""
		}
		if want := strings.ReplaceAll(line, "{title}", tc.title); !strings.Contains(text, want) || fmt.Sprint(mode) != tc.parseMode {
			t.Fatalf("%s from PM replied %q with parse_mode %q, want %q with parse_mode %q", tc.text, text, mode, want, tc.parseMode)
		}
	}
}

// A bans button or reset confirmation without a "c" field, as sent before PM
// support or in a group, acts on the chat it is pressed in.
func TestModerationButtonWithoutGroupFieldActsOnItsChat(t *testing.T) {
	for _, tc := range []struct {
		name, data string
		acted      func(f adminChatFixture) bool
	}{
		{"restrict", encodeCallbackData("restrict", map[string]string{"a": "ban", "u": fmt.Sprint(victimID)}),
			func(f adminChatFixture) bool { return targetedIn(f, "banChatMember", f.chat.Id, victimID) }},
		{"unrestrict", encodeCallbackData("unrestrict", map[string]string{"a": "unban", "u": fmt.Sprint(victimID)}),
			func(f adminChatFixture) bool { return targetedIn(f, "unbanChatMember", f.chat.Id, victimID) }},
		{"reset", encodeCallbackData("rmAllFilters", map[string]string{"a": "yes"}),
			func(f adminChatFixture) bool { return len(filters.GetFiltersList(f.chat.Id)) == 0 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, _ := pmModerationFixture(t)
			f.makeOwner()
			if err := filters.AddFilter(f.chat.Id, "hello", "Hi there", "", nil, db.TEXT); err != nil {
				t.Fatalf("AddFilter() error = %v", err)
			}

			update := newModuleCallbackContext(f.bot, f.chat, f.admin, tc.data).Update
			if err := pmResetDispatcher().ProcessUpdate(f.bot, update, nil); err != nil {
				t.Fatalf("ProcessUpdate(callback %q) error = %v", tc.data, err)
			}

			if !tc.acted(f) {
				t.Fatalf("the %s button without a group did not act on its chat; calls:\n%s", tc.name, sentText(f.client))
			}
		})
	}
}

// A bans button or reset confirmation whose "c" field is no chat ID is
// answered as invalid, in PM or in the group, and acts nowhere.
func TestModerationButtonWithMalformedGroupFieldIsRejected(t *testing.T) {
	for _, tc := range []struct {
		name, data, answerKey string
	}{
		{"restrict", encodeCallbackData("restrict", map[string]string{"a": "ban", "c": "x", "u": fmt.Sprint(victimID)}), "bans_invalid_callback_data"},
		{"unrestrict", encodeCallbackData("unrestrict", map[string]string{"a": "unban", "c": "x", "u": fmt.Sprint(victimID)}), "bans_invalid_callback_data"},
		{"reset", encodeCallbackData("rmAllFilters", map[string]string{"a": "yes", "c": "x"}), "common_callback_invalid_request"},
	} {
		for _, where := range []string{"pm", "group"} {
			t.Run(tc.name+"/"+where, func(t *testing.T) {
				f, _ := pmModerationFixture(t)
				f.makeOwner()
				if err := filters.AddFilter(f.chat.Id, "hello", "Hi there", "", nil, db.TEXT); err != nil {
					t.Fatalf("AddFilter() error = %v", err)
				}
				chat := f.chat
				if where == "pm" {
					chat = privateChatOf(f.admin)
				}

				update := newModuleCallbackContext(f.bot, chat, f.admin, tc.data).Update
				if err := pmResetDispatcher().ProcessUpdate(f.bot, update, nil); err != nil {
					t.Fatalf("ProcessUpdate(callback %q) error = %v", tc.data, err)
				}

				for _, method := range []string{"banChatMember", "unbanChatMember"} {
					if len(f.client.callsFor(method)) != 0 {
						t.Fatalf("the %s button with a malformed group called %s", tc.name, method)
					}
				}
				if len(filters.GetFiltersList(f.chat.Id)) == 0 {
					t.Fatalf("the %s button with a malformed group cleared the filters", tc.name)
				}
				answers := f.client.callsFor("answerCallbackQuery")
				if len(answers) != 1 || fmt.Sprint(answers[0].Params["text"]) != translated(t, tc.answerKey) {
					t.Fatalf("the %s button with a malformed group answered %+v, want one %s", tc.name, answers, tc.answerKey)
				}
			})
		}
	}
}

// A confirmation asked in PM names the group it resets, whether the group was
// named or connected; asked in the group, it does not.
func TestResetConfirmationInPMNamesGroup(t *testing.T) {
	for _, tc := range pmResetCases {
		if tc.cmd == "privnote" {
			continue
		}
		t.Run(tc.cmd, func(t *testing.T) {
			f, _ := pmResetFixture(t, tc)
			f.makeOwner()
			dispatcher := pmResetDispatcher()
			target := translated(t, "connections_pm_group_confirm_target")
			target = strings.NewReplacer("{title}", f.chat.Title, "{id}", fmt.Sprint(f.chat.Id)).Replace(target)

			for _, text := range []string{"/" + tc.cmd + " @fixturegroup", "connected:/" + tc.cmd} {
				if cmd, ok := strings.CutPrefix(text, "connected:"); ok {
					f.connectAdmin(t)
					text = cmd
				}
				before := len(f.client.callsFor("sendMessage"))
				processPM(t, dispatcher, f, f.admin.Id, text)
				if got := confirmationText(t, f, before); !strings.Contains(got, target) {
					t.Fatalf("%s from PM asked %q, want it to name the group: %q", text, got, target)
				}
			}

			before := len(f.client.callsFor("sendMessage"))
			update := newModuleMessageContext(f.bot, f.chat, f.admin, "/"+tc.cmd).Update
			if err := dispatcher.ProcessUpdate(f.bot, update, nil); err != nil {
				t.Fatalf("ProcessUpdate(/%s) error = %v", tc.cmd, err)
			}
			if got, want := confirmationText(t, f, before), translated(t, confirmQuestionKeys[tc.cmd]); got != want {
				t.Fatalf("/%s in the group asked %q, want the question as it was: %q", tc.cmd, got, want)
			}
		})
	}
}

// registerOwnerConfirm owns the whole reset-confirmation lifecycle; this
// exercises it once through callback queries against the fake BotClient.
func TestOwnerConfirmSkeletonLifecycle(t *testing.T) {
	useEnglishLocale(t)
	f := newAdminChatFixture(t)
	f.makeOwner()
	client, bot, chat, owner := f.client, f.bot, f.chat, f.admin

	commits := 0
	fail := false
	message, callback := registerOwnerConfirm(ownerConfirmSpec{
		Namespace: "testconfirm",
		AskKey:    "notes_clear_all_confirm",
		Ask: func(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, tr *i18n.Translator) (bool, error) {
			return true, nil
		},
		Commit: func(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, tr *i18n.Translator, actor *gotgbot.User) (string, i18n.TranslationParams, error) {
			commits++
			if fail {
				return "", nil, errors.New("commit failed")
			}
			return "", nil, nil
		},
		Keys: ownerConfirmKeys{
			Confirmed: "notes_clear_all_success",
			Cancelled: "notes_clear_all_cancelled",
			Failed:    "error_generic",
		},
	})

	// The command sends the question with a yes/no keyboard naming the chat.
	if err := message(bot, newModuleMessageContext(bot, chat, owner, "/testconfirm")); err != ext.EndGroups {
		t.Fatalf("message handler error = %v, want EndGroups", err)
	}
	prompt := client.onlyCall(t, "sendMessage")
	markup, ok := prompt.Params["reply_markup"].(gotgbot.InlineKeyboardMarkup)
	if !ok || len(markup.InlineKeyboard) != 1 || len(markup.InlineKeyboard[0]) != 2 {
		t.Fatalf("prompt markup = %#v, want one row of two buttons", prompt.Params["reply_markup"])
	}
	buttons := map[string]string{}
	for _, button := range markup.InlineKeyboard[0] {
		decoded, ok := decodeCallbackData(button.CallbackData, "testconfirm")
		if !ok {
			t.Fatalf("button data %q does not decode in the namespace", button.CallbackData)
		}
		if c, _ := decoded.Field("c"); c != fmt.Sprint(chat.Id) {
			t.Fatalf("button names group %q, want %d", c, chat.Id)
		}
		if a, _ := decoded.Field("a"); a == "yes" || a == "no" {
			buttons[a] = button.CallbackData
		}
	}
	if len(buttons) != 2 {
		t.Fatalf("prompt buttons = %v, want yes and no", buttons)
	}

	lastAnswer := func() string {
		calls := client.callsFor("answerCallbackQuery")
		if len(calls) == 0 {
			t.Fatal("no answerCallbackQuery sent")
		}
		return fmt.Sprint(calls[len(calls)-1].Params["text"])
	}

	// yes commits and edits the prompt to the confirmed text.
	if err := callback(bot, newModuleCallbackContext(bot, chat, owner, buttons["yes"])); err != ext.EndGroups {
		t.Fatalf("callback(yes) error = %v, want EndGroups", err)
	}
	if commits != 1 {
		t.Fatalf("commits = %d, want 1", commits)
	}
	if got, want := lastAnswer(), translated(t, "notes_clear_all_success"); got != want {
		t.Fatalf("yes answer = %q, want %q", got, want)
	}
	if edits := client.callsFor("editMessageText"); len(edits) != 1 {
		t.Fatalf("editMessageText calls = %d, want 1", len(edits))
	}

	// no cancels without committing.
	if err := callback(bot, newModuleCallbackContext(bot, chat, owner, buttons["no"])); err != ext.EndGroups {
		t.Fatalf("callback(no) error = %v, want EndGroups", err)
	}
	if commits != 1 {
		t.Fatalf("commits after no = %d, want still 1", commits)
	}
	if got, want := lastAnswer(), translated(t, "notes_clear_all_cancelled"); got != want {
		t.Fatalf("no answer = %q, want %q", got, want)
	}

	// A failed commit answers with the failed text.
	fail = true
	if err := callback(bot, newModuleCallbackContext(bot, chat, owner, buttons["yes"])); err != ext.EndGroups {
		t.Fatalf("callback(yes, failing commit) error = %v, want EndGroups", err)
	}
	if commits != 2 {
		t.Fatalf("commits after failing yes = %d, want 2", commits)
	}
	if got, want := lastAnswer(), translated(t, "error_generic"); got != want {
		t.Fatalf("failed answer = %q, want %q", got, want)
	}

	// A non-owner's press is refused and commits nothing.
	if err := callback(bot, newModuleCallbackContext(bot, chat, f.member, buttons["yes"])); err != ext.EndGroups {
		t.Fatalf("callback(member yes) error = %v, want EndGroups", err)
	}
	if commits != 2 {
		t.Fatalf("commits after member press = %d, want still 2", commits)
	}

	// A press whose confirmation message is gone is answered, never committed.
	gone := newModuleCallbackContext(bot, chat, owner, buttons["yes"])
	gone.CallbackQuery.Message = nil
	if err := callback(bot, gone); err != ext.EndGroups {
		t.Fatalf("callback(gone message) error = %v, want EndGroups", err)
	}
	if commits != 2 {
		t.Fatalf("commits after gone-message press = %d, want still 2", commits)
	}
	if got, want := lastAnswer(), translated(t, "common_callback_message_unavailable"); got != want {
		t.Fatalf("gone-message answer = %q, want %q", got, want)
	}
}
