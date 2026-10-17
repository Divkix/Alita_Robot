//go:build testtools

package modules

import (
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/notes"
)

// A member who asks for an admin-only note from PM gets the refusal and must
// end handler iteration: getNotes has already pointed ctx.EffectiveChat at the
// connected group, so letting later groups run would treat the PM message as
// one sent in that group.
func TestGetAdminOnlyNoteFromPMRefusalEndsGroups(t *testing.T) {
	client, bot, groupA, _, member := connectedPMFixture(t)
	if err := notes.AddNote(groupA, "secret", "admins only", "", nil, db.TEXT, false, false, true, false, false, false); err != nil {
		t.Fatalf("AddNote() error = %v", err)
	}

	ctx := newModuleMessageContext(bot, privateChatOf(member), member, "/get secret")
	err := notesModule.getNotes(bot, ctx)

	if err != ext.EndGroups {
		t.Fatalf("getNotes() error = %v, want EndGroups", err)
	}
	if ctx.EffectiveChat.Id != groupA {
		t.Fatalf("ctx.EffectiveChat = %d, want connected group %d (precondition)", ctx.EffectiveChat.Id, groupA)
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
