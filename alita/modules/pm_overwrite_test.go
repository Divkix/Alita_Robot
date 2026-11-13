//go:build testtools

package modules

import (
	"fmt"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/filters"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/notes"
)

// overwriteCase is /save or /filter on a name the group already has. The
// group's stored reply for the name starts as "old"; the command asks to set
// it to "new".
type overwriteCase struct {
	name       string
	dispatcher func() *ext.Dispatcher
	ns         string // callback namespace of the buttons
	cmd        string
	no         string // action of the No button
	success    string
	cancelled  string
	other      string // what an admin other than the one who started it is told
	seed       func(t *testing.T, chatID int64)
	reply      func(chatID int64) string
	store      func(t *testing.T, token string, chatID, userID int64)
}

var overwriteCases = []overwriteCase{
	{
		name: "notes", dispatcher: notesDispatcher, ns: "notes.overwrite", cmd: "/save rules new", no: "no",
		success: "notes_overwrite_success", cancelled: "notes_overwrite_cancelled", other: "notes_overwrite_cancelled",
		seed: func(t *testing.T, chatID int64) {
			if err := notes.AddNote(chatID, "rules", "old", "", nil, db.TEXT, false, false, false, false, false, false); err != nil {
				t.Fatalf("AddNote() error = %v", err)
			}
		},
		reply: func(chatID int64) string { return notes.GetNote(chatID, "rules").NoteContent },
		store: func(t *testing.T, token string, chatID, userID int64) {
			if err := setNoteOverwriteCache(token, overwriteNote{overwriteBase: overwriteBase{
				ChatID: chatID, UserID: userID, ItemName: "rules", Text: "new", DataType: db.TEXT,
			}}); err != nil {
				t.Fatalf("setNoteOverwriteCache() error = %v", err)
			}
		},
	},
	{
		name: "filters", dispatcher: filtersDispatcher, ns: "filters_overwrite", cmd: "/filter hello new", no: "cancel",
		success: "filters_overwrite_success", cancelled: "filters_overwrite_cancelled", other: "filters_overwrite_expired",
		seed: func(t *testing.T, chatID int64) {
			if err := filters.AddFilter(chatID, "hello", "old", "", nil, db.TEXT); err != nil {
				t.Fatalf("AddFilter() error = %v", err)
			}
		},
		reply: func(chatID int64) string {
			var row models.ChatFilters
			db.DB.Where("chat_id = ? AND keyword = ?", chatID, "hello").Take(&row)
			return row.FilterReply
		},
		store: func(t *testing.T, token string, chatID, userID int64) {
			if err := setFilterOverwriteCache(token, overwriteFilter{overwriteBase: overwriteBase{
				ChatID: chatID, UserID: userID, ItemName: "hello", Text: "new", DataType: db.TEXT,
			}}); err != nil {
				t.Fatalf("setFilterOverwriteCache() error = %v", err)
			}
		},
	},
}

// overwriteFixture is oneOffFixture's group holding tc's "old" reply, with
// its admin connected to it from PM.
func overwriteFixture(t *testing.T, tc overwriteCase) adminChatFixture {
	t.Helper()
	f := oneOffFixture(t)
	setPMGroupCandidates(t, models.Chat{ChatId: f.chat.Id, ChatName: f.chat.Title})
	f.connectAdmin(t)
	tc.seed(t, f.chat.Id)
	return f
}

// askOverwriteInPM sends tc's command from the admin's PM and returns the
// data of the button with action.
func askOverwriteInPM(t *testing.T, f adminChatFixture, d *ext.Dispatcher, tc overwriteCase, action string) string {
	t.Helper()
	before := len(f.client.callsFor("sendMessage"))
	processPM(t, d, f, f.admin.Id, tc.cmd)
	buttons := buttonsWithAction(f, before, action)
	if len(buttons) != 1 {
		t.Fatalf("%s from PM sent %d %q buttons, want one; sent:\n%s", tc.cmd, len(buttons), action, sentText(f.client))
	}
	return buttons[0]
}

func lastEditText(t *testing.T, f adminChatFixture) string {
	t.Helper()
	calls := f.client.callsFor("editMessageText")
	if len(calls) == 0 {
		t.Fatalf("no message was edited; answers: %+v", f.client.callsFor("answerCallbackQuery"))
	}
	return fmt.Sprint(calls[len(calls)-1].Params["text"])
}

// Yes and No pressed in PM act on the connected group, and the edited
// message names it.
func TestOverwriteButtonsInPMActOnTheConnectedGroup(t *testing.T) {
	for _, tc := range overwriteCases {
		t.Run(tc.name+"/yes", func(t *testing.T) {
			f := overwriteFixture(t, tc)
			d := tc.dispatcher()

			pressConfirmInPM(t, d, f, f.admin, askOverwriteInPM(t, f, d, tc, "yes"))

			if got := tc.reply(f.chat.Id); got != "new" {
				t.Fatalf("reply after Yes in PM = %q, want new; edits: %+v", got, f.client.callsFor("editMessageText"))
			}
			want := translated(t, tc.success) + "\n\n" + groupLine(t, f.chat.Title, f.chat.Id)
			if got := lastEditText(t, f); got != want {
				t.Fatalf("PM answer = %q, want %q", got, want)
			}
		})
		t.Run(tc.name+"/no", func(t *testing.T) {
			f := overwriteFixture(t, tc)
			d := tc.dispatcher()

			pressConfirmInPM(t, d, f, f.admin, askOverwriteInPM(t, f, d, tc, tc.no))

			if got := tc.reply(f.chat.Id); got != "old" {
				t.Fatalf("reply after No in PM = %q, want old", got)
			}
			want := translated(t, tc.cancelled) + "\n\n" + groupLine(t, f.chat.Title, f.chat.Id)
			if got := lastEditText(t, f); got != want {
				t.Fatalf("PM answer = %q, want %q", got, want)
			}
		})
	}
}

// Only an admin of the token's group passes, and only the admin who started
// the overwrite changes anything.
func TestOverwriteButtonsInPMRefuseOtherPressers(t *testing.T) {
	for _, tc := range overwriteCases {
		t.Run(tc.name+"/non-admin", func(t *testing.T) {
			f := overwriteFixture(t, tc)
			tc.store(t, "non-admin-token", f.chat.Id, f.member.Id)

			pressConfirmInPM(t, tc.dispatcher(), f, f.member,
				encodeCallbackData(tc.ns, map[string]string{"a": "yes", "t": "non-admin-token"}))

			if got := tc.reply(f.chat.Id); got != "old" {
				t.Fatalf("reply after a non-admin's Yes = %q, want old", got)
			}
			answers := f.client.callsFor("answerCallbackQuery")
			if len(answers) != 1 || answers[0].Params["text"] != translated(t, "chat_status_user_admin_button_error") {
				t.Fatalf("answers = %+v, want the admin refusal", answers)
			}
		})
		t.Run(tc.name+"/other-admin", func(t *testing.T) {
			f := overwriteFixture(t, tc)
			other := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Other"}
			f.addAdmins(fmt.Sprintf(`{"status":"administrator","user":{"id":%d,"is_bot":false,"first_name":"Other"},%s}`, other.Id, fixtureAdminRights))
			d := tc.dispatcher()

			pressConfirmInPM(t, d, f, other, askOverwriteInPM(t, f, d, tc, "yes"))

			if got := tc.reply(f.chat.Id); got != "old" {
				t.Fatalf("reply after another admin's Yes = %q, want old", got)
			}
			want := translated(t, tc.other) + "\n\n" + groupLine(t, f.chat.Title, f.chat.Id)
			if got := lastEditText(t, f); got != want {
				t.Fatalf("answer to another admin = %q, want %q", got, want)
			}
		})
	}
}

// Pressed in the group, Yes overwrites and the answer names no group.
func TestOverwriteButtonsInGroupCarryNoGroupLine(t *testing.T) {
	for _, tc := range overwriteCases {
		t.Run(tc.name, func(t *testing.T) {
			f := overwriteFixture(t, tc)
			tc.store(t, "group-token", f.chat.Id, f.admin.Id)
			data := encodeCallbackData(tc.ns, map[string]string{"a": "yes", "t": "group-token"})
			if err := tc.dispatcher().ProcessUpdate(f.bot, newModuleCallbackContext(f.bot, f.chat, f.admin, data).Update, nil); err != nil {
				t.Fatalf("ProcessUpdate() error = %v", err)
			}

			if got := tc.reply(f.chat.Id); got != "new" {
				t.Fatalf("reply after Yes in group = %q, want new", got)
			}
			if got, want := lastEditText(t, f), translated(t, tc.success); got != want {
				t.Fatalf("group answer = %q, want %q", got, want)
			}
			// Group use is unchanged: the answer is sent as plain text.
			edits := f.client.callsFor("editMessageText")
			if mode, ok := edits[len(edits)-1].Params["parse_mode"]; ok && mode != "" {
				t.Fatalf("group answer parse_mode = %v, want plain text", mode)
			}
		})
	}
}
