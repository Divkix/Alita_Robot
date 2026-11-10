//go:build testtools

package modules

import (
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/notes"
)

// A member's admin-only note refusal in PM must stop later handler groups
// after the pipeline resolves the connected group.
func TestGetAdminOnlyNoteFromPMRefusalEndsGroups(t *testing.T) {
	client, bot, groupA, _, member := connectedPMFixture(t)
	if err := notes.AddNote(groupA, "secret", "admins only", "", nil, db.TEXT, false, false, true, false, false, false); err != nil {
		t.Fatalf("AddNote() error = %v", err)
	}

	ctx := newModuleMessageContext(bot, privateChatOf(member), member, "/get secret")
	dispatcher := notesDispatcher()
	reached := false
	dispatcher.AddHandlerToGroup(handlers.NewMessage(func(*gotgbot.Message) bool { return true }, func(*gotgbot.Bot, *ext.Context) error {
		reached = true
		return ext.ContinueGroups
	}), 2)
	if err := dispatcher.ProcessUpdate(bot, ctx.Update, nil); err != nil {
		t.Fatal(err)
	}
	if reached {
		t.Fatal("later handler group ran on connected PM context")
	}
	refused := false
	for _, call := range client.callsFor("sendMessage") {
		if fmt.Sprint(call.Params["chat_id"]) == fmt.Sprint(member.Id) && !strings.Contains(fmt.Sprint(call.Params["text"]), "admins only") {
			refused = true
		}
	}
	if !refused {
		t.Fatal("member got no refusal in PM")
	}
	assertNothingSentTo(t, client, groupA)
}
