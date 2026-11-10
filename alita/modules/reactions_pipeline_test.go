//go:build testtools

package modules

import (
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/reactions"
)

func TestReactionCommandsApplyOnlyAfterAdminProof(t *testing.T) {
	useRealLocales(t)
	for _, tc := range []struct {
		command string
		want    map[string]string
	}{
		{"/addreaction hello 👍", map[string]string{"keep": "👀", "hello": "👍"}},
		{"/removereaction keep", nil},
		{"/resetreactions", nil},
	} {
		for _, limited := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/limited=%t", tc.command, limited), func(t *testing.T) {
				f := newAdminChatFixture(t)
				if limited {
					key := fmt.Sprint(f.admin.Id)
					full := string(f.client.membersByUser[key])
					restricted := strings.ReplaceAll(full, `"can_change_info":true`, `"can_change_info":false`)
					f.client.membersByUser[key] = json.RawMessage(restricted)
					f.client.adminsByChat[fmt.Sprint(f.chat.Id)] = json.RawMessage(strings.ReplaceAll(string(f.client.adminsByChat[fmt.Sprint(f.chat.Id)]), full, restricted))
				}
				if err := reactions.AddReaction(f.chat.Id, "keep", "👀"); err != nil {
					t.Fatal(err)
				}
				original := map[string]string{"keep": "👀"}
				d := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
				LoadReactions(d)
				LoadBotUpdates(d)
				tap := anonymousAdminProof(t, d, f, newModuleMessageContext(f.bot, f.chat, f.admin, tc.command))
				if got := reactions.GetReactions(f.chat.Id); !maps.Equal(got, original) {
					t.Fatalf("before proof reactions = %v, want unchanged %v", got, original)
				}
				before := len(f.client.callsFor("sendMessage"))
				if err := d.ProcessUpdate(f.bot, tap, nil); err != nil {
					t.Fatal(err)
				}
				want := tc.want
				if limited {
					want = original
				}
				if got := reactions.GetReactions(f.chat.Id); !maps.Equal(got, want) {
					t.Fatalf("after proof reactions = %v, want %v", got, want)
				}
				sent := f.client.callsFor("sendMessage")[before:]
				if len(sent) != 1 {
					t.Fatalf("after proof replies = %v, want one result", sent)
				}
				if limited {
					assertParams(t, sent[0], map[string]any{"text": "You don't have permission to change info in this group!"})
				}
			})
		}
	}
}

func TestReactionCommandsDoNotRequireBotAdmin(t *testing.T) {
	useRealLocales(t)
	for _, tc := range []struct {
		command string
		want    map[string]string
	}{
		{"/addreaction hello 👍", map[string]string{"keep": "👀", "hello": "👍"}},
		{"/removereaction keep", nil},
		{"/resetreactions", nil},
		{"/reactions", map[string]string{"keep": "👀"}},
	} {
		for _, pm := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/PM=%t", tc.command, pm), func(t *testing.T) {
				f := newAdminChatFixture(t)
				f.client.membersByUser["999"] = json.RawMessage(`{"status":"member","user":{"id":999,"is_bot":true,"first_name":"Alita"}}`)
				f.client.adminsByChat[fmt.Sprint(f.chat.Id)] = json.RawMessage("[" + string(f.client.membersByUser[fmt.Sprint(f.admin.Id)]) + "]")
				if err := reactions.AddReaction(f.chat.Id, "keep", "👀"); err != nil {
					t.Fatal(err)
				}
				actor := f.admin
				if tc.command == "/reactions" {
					actor = f.member
				}
				origin := f.chat
				if pm {
					origin = privateChatOf(actor)
					if err := connections.ConnectId(actor.Id, f.chat.Id); err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { _ = connections.DisconnectId(actor.Id) })
				}
				d := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
				LoadReactions(d)
				if err := d.ProcessUpdate(f.bot, newModuleMessageContext(f.bot, origin, actor, tc.command).Update, nil); err != nil {
					t.Fatal(err)
				}
				if got := reactions.GetReactions(f.chat.Id); !maps.Equal(got, tc.want) {
					t.Fatalf("reactions = %v, want %v", got, tc.want)
				}
				if got := sentTexts(f.client); len(got) != 1 || (tc.command == "/reactions" && !strings.Contains(got[0], "keep")) {
					t.Fatalf("replies = %q, want one successful result", got)
				}
			})
		}
	}
}
