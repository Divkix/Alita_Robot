//go:build testtools

package modules

import (
	"encoding/json"
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
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/filters"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/notes"
	"github.com/divkix/Alita_Robot/alita/db/warns"
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
			if err := db.DB.Model(&models.Chat{}).Where("chat_id = ?", id).
				Update("users", models.Int64Array{43}).Error; err != nil {
				t.Fatalf("seed users of %d: %v", id, err)
			}
			cache.DeleteCache(cache.CacheKey("chat_users", id))
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
	var yes []string
	for _, call := range f.client.callsFor("sendMessage")[from:] {
		markup, ok := call.Params["reply_markup"].(gotgbot.InlineKeyboardMarkup)
		if !ok {
			continue
		}
		for _, row := range markup.InlineKeyboard {
			for _, button := range row {
				if decoded, ok := decodeCallbackData(button.CallbackData); ok {
					if action, _ := decoded.Field("a"); action == "yes" {
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
