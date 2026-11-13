//go:build testtools

package modules

import (
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/notes"
)

// privateNotesChat is an admin fixture whose notes open in PM, holding the
// note "rules".
func privateNotesChat(t *testing.T) adminChatFixture {
	t.Helper()
	useRealLocales(t)
	f := newAdminChatFixture(t)
	if err := notes.AddNote(f.chat.Id, "rules", "be kind", "", nil, db.TEXT, false, false, false, false, false, false); err != nil {
		t.Fatalf("AddNote() error = %v", err)
	}
	if err := notes.TooglePrivateNote(f.chat.Id, true); err != nil {
		t.Fatalf("TooglePrivateNote() error = %v", err)
	}
	cleanupEphemeral(t, f.chat.Id)
	return f
}

// assertNotePMButton fails unless call is today's public answer: the button
// that opens the note in PM.
func assertNotePMButton(t *testing.T, call moduleBotCall) {
	t.Helper()
	kb, _ := call.Params["reply_markup"].(gotgbot.InlineKeyboardMarkup)
	if len(kb.InlineKeyboard) == 0 || !strings.Contains(kb.InlineKeyboard[0][0].Url, "?start=note_") {
		t.Fatalf("reply_markup = %+v, want the PM note button", call.Params["reply_markup"])
	}
	if ephemeralReceiverOf(call) != 0 {
		t.Fatal("the public answer was sent ephemerally")
	}
}

// assertEphemeralNote fails unless call shows the note body to the asker only.
func assertEphemeralNote(t *testing.T, f adminChatFixture, call moduleBotCall, body string) {
	t.Helper()
	if got := ephemeralReceiverOf(call); got != f.member.Id {
		t.Fatalf("%s receiver = %d, want the asker %d", call.Method, got, f.member.Id)
	}
	assertParams(t, call, map[string]any{"chat_id": f.chat.Id})
	if _, ok := call.Params["reply_parameters"]; ok {
		t.Fatalf("%s: an ephemeral send carried reply_parameters", call.Method)
	}
	text := fmt.Sprint(call.Params["text"]) + fmt.Sprint(call.Params["caption"])
	if !strings.Contains(text, body) {
		t.Fatalf("%s text = %q, want the note %q", call.Method, text, body)
	}
}

func TestPrivateNoteWithEphemeralOffSendsThePMButton(t *testing.T) {
	for _, text := range []string{"/get rules", "#rules"} {
		t.Run(text, func(t *testing.T) {
			f := privateNotesChat(t)
			aispamProcessUpdate(t, notesDispatcher(), f.bot, f.chat, f.member, 1301, text)
			assertNotePMButton(t, f.client.onlyCall(t, "sendMessage"))
		})
	}
}

func TestPrivateNoteWithEphemeralOnShowsTheNoteOnlyToTheAsker(t *testing.T) {
	for _, text := range []string{"/get rules", "#rules"} {
		t.Run(text, func(t *testing.T) {
			f := privateNotesChat(t)
			enableEphemeral(t, f.chat.Id)
			aispamProcessUpdate(t, notesDispatcher(), f.bot, f.chat, f.member, 1302, text)
			assertEphemeralNote(t, f, f.client.onlyCall(t, "sendMessage"), "be kind")
		})
	}
}

// A note marked private in a chat that keeps notes public is redirected too;
// its media goes out ephemerally through the same saved-message sender.
func TestPrivateOnlyMediaNoteWithEphemeralOnShowsOnlyToTheAsker(t *testing.T) {
	useRealLocales(t)
	f := newAdminChatFixture(t)
	if err := notes.AddNote(f.chat.Id, "pic", "look here", "photo-1", nil, db.PHOTO, true, false, false, false, false, false); err != nil {
		t.Fatalf("AddNote() error = %v", err)
	}
	enableEphemeral(t, f.chat.Id)
	aispamProcessUpdate(t, notesDispatcher(), f.bot, f.chat, f.member, 1303, "/get pic")

	assertEphemeralNote(t, f, f.client.onlyCall(t, "sendPhoto"), "look here")
	if n := len(f.client.callsFor("sendMessage")); n != 0 {
		t.Fatalf("sendMessage calls = %d, want no public answer", n)
	}
}

func TestPrivateNoteFallsBackToThePMButtonWhenTheEphemeralSendFails(t *testing.T) {
	f := privateNotesChat(t)
	enableEphemeral(t, f.chat.Id)
	f.bot.BotClient = ephemeralRefusingClient{f.client}
	aispamProcessUpdate(t, notesDispatcher(), f.bot, f.chat, f.member, 1304, "#rules")

	calls := f.client.callsFor("sendMessage")
	if len(calls) != 2 || ephemeralReceiverOf(calls[0]) != f.member.Id {
		t.Fatalf("sendMessage calls = %+v, want the refused ephemeral send then the public answer", calls)
	}
	assertNotePMButton(t, calls[1])
}

func TestPrivateNoteSentAsAReplyStaysPublic(t *testing.T) {
	f := privateNotesChat(t)
	enableEphemeral(t, f.chat.Id)
	update := aispamMessageUpdate(f.chat, f.admin, 1305, "/get rules")
	update.Message.ReplyToMessage = &gotgbot.Message{MessageId: 50, Date: 1, Chat: f.chat, From: &f.member, Text: "hi"}
	aispamProcessRawUpdate(t, notesDispatcher(), f.bot, update)

	assertNotePMButton(t, f.client.onlyCall(t, "sendMessage"))
}

func TestPrivateNotesListWithEphemeralOnShowsTheListOnlyToTheAsker(t *testing.T) {
	f := privateNotesChat(t)
	aispamProcessUpdate(t, notesDispatcher(), f.bot, f.chat, f.member, 1306, "/notes")
	public := f.client.onlyCall(t, "sendMessage")
	if ephemeralReceiverOf(public) != 0 || fmt.Sprint(public.Params["text"]) != translated(t, "notes_check_button") {
		t.Fatalf("/notes with /ephemeral off = %+v, want the public PM button", public.Params)
	}

	enableEphemeral(t, f.chat.Id)
	aispamProcessUpdate(t, notesDispatcher(), f.bot, f.chat, f.member, 1307, "/notes")
	calls := f.client.callsFor("sendMessage")
	if len(calls) != 2 || ephemeralReceiverOf(calls[1]) != f.member.Id {
		t.Fatalf("sendMessage calls = %+v, want one ephemeral list after the public one", calls)
	}
	if text := fmt.Sprint(calls[1].Params["text"]); !strings.Contains(text, "?start=note_") || !strings.Contains(text, "rules") {
		t.Fatalf("ephemeral list = %q, want the PM note links", text)
	}
}

// From PM, /get for the connected group answers in PM; nothing is ephemeral.
func TestNoteFromPMWithEphemeralOnStaysInPM(t *testing.T) {
	f := privateNotesChat(t)
	enableEphemeral(t, f.chat.Id)
	f.connectAdmin(t)
	processPM(t, notesDispatcher(), f, f.admin.Id, "/get rules")

	if sends := ephemeralSends(f.client); len(sends) != 0 {
		t.Fatalf("ephemeral sends = %+v, want none from PM", sends)
	}
	if !calledFor(f, "sendMessage", f.admin.Id) || calledFor(f, "sendMessage", f.chat.Id) {
		t.Fatalf("/get from PM did not answer in PM only; sent:\n%s", sentText(f.client))
	}
}

// A connected member who sends /get in PM is already where a private note
// opens: the note arrives there, with no button pointing back at the PM.
func TestGetPrivateNoteInPMSendsTheNote(t *testing.T) {
	for _, byChat := range []bool{true, false} {
		t.Run(fmt.Sprintf("privateByChat=%t", byChat), func(t *testing.T) {
			useRealLocales(t)
			f := newAdminChatFixture(t)
			if err := notes.AddNote(f.chat.Id, "rules", "be kind", "", nil, db.TEXT, !byChat, false, false, false, false, false); err != nil {
				t.Fatalf("AddNote() error = %v", err)
			}
			if err := notes.TooglePrivateNote(f.chat.Id, byChat); err != nil {
				t.Fatalf("TooglePrivateNote() error = %v", err)
			}
			if err := connections.ToggleAllowConnect(f.chat.Id, true); err != nil {
				t.Fatal(err)
			}
			if err := connections.ConnectId(f.member.Id, f.chat.Id); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = connections.DisconnectId(f.member.Id) })

			processPM(t, notesDispatcher(), f, f.member.Id, "/get rules")

			call := f.client.onlyCall(t, "sendMessage")
			assertParams(t, call, map[string]any{"chat_id": f.member.Id})
			if text := fmt.Sprint(call.Params["text"]); !strings.Contains(text, "be kind") {
				t.Fatalf("PM text = %q, want the note", text)
			}
			if kb, _ := call.Params["reply_markup"].(gotgbot.InlineKeyboardMarkup); len(kb.InlineKeyboard) != 0 {
				t.Fatalf("reply_markup = %+v, want no deep-link button", kb)
			}
		})
	}
}
