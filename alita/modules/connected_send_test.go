//go:build testtools

package modules

import (
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/greetings"
	"github.com/divkix/Alita_Robot/alita/db/notes"
)

// sendPMCommand runs text through the dispatcher as a message the user sent
// in PM and returns the sendMessage call that carries want.
func sendPMCommand(t *testing.T, dispatcher *ext.Dispatcher, bot *gotgbot.Bot, client *moduleBotClient, user gotgbot.User, text, want string) moduleBotCall {
	t.Helper()
	update := &gotgbot.Update{UpdateId: 1, Message: &gotgbot.Message{MessageId: 101, Date: 1, Chat: privateChatOf(user), From: &user, Text: text}}
	if err := dispatcher.ProcessUpdate(bot, update, nil); err != nil {
		t.Fatalf("ProcessUpdate(%q) error = %v", text, err)
	}
	for _, call := range client.callsFor("sendMessage") {
		if strings.Contains(fmt.Sprint(call.Params["text"]), want) {
			return call
		}
	}
	t.Fatalf("%s from PM sent nothing containing %q", text, want)
	return moduleBotCall{}
}

// /get from PM on a connected group sends the note to the user, replying to
// their message, and nothing to the group.
func TestGetNoteFromPMIsSentToThePM(t *testing.T) {
	client, bot, groupA, _, member := connectedPMFixture(t)
	if err := notes.AddNote(groupA, "rules", "be kind", "", nil, db.TEXT, false, false, false, false, false, false); err != nil {
		t.Fatalf("AddNote() error = %v", err)
	}
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadNotes(dispatcher)

	call := sendPMCommand(t, dispatcher, bot, client, member, "/get rules", "be kind")

	if got := fmt.Sprint(call.Params["chat_id"]); got != fmt.Sprint(member.Id) {
		t.Errorf("note sent to chat %s, want the PM %d", got, member.Id)
	}
	if rp, _ := call.Params["reply_parameters"].(*gotgbot.ReplyParameters); rp == nil || rp.MessageId != 101 {
		t.Errorf("note reply_parameters = %+v, want a reply to the PM message 101", call.Params["reply_parameters"])
	}
	assertNothingSentTo(t, client, groupA)
}

// /welcome from PM on a connected group shows the settings and the greeting
// preview to the admin, and sends nothing to the group.
func TestWelcomePreviewFromPMIsSentToThePM(t *testing.T) {
	client, bot, groupA, admin, _ := connectedPMFixture(t)
	if err := greetings.SetWelcomeText(groupA, "hello there", "", nil, db.TEXT); err != nil {
		t.Fatalf("SetWelcomeText() error = %v", err)
	}
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadGreetings(dispatcher)

	for _, text := range []string{"/welcome", "/welcome noformat"} {
		resetCalls(client)
		call := sendPMCommand(t, dispatcher, bot, client, admin, text, "hello there")
		if got := fmt.Sprint(call.Params["chat_id"]); got != fmt.Sprint(admin.Id) {
			t.Errorf("%s preview sent to chat %s, want the PM %d", text, got, admin.Id)
		}
		assertNothingSentTo(t, client, groupA)
	}
}
