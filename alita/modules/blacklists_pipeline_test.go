//go:build testtools

package modules

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/blacklists"
	"github.com/divkix/Alita_Robot/alita/db/connections"
)

func TestBlacklistAliasesApplyOnlyAfterAdminProof(t *testing.T) {
	useRealLocales(t)
	for _, tc := range []struct {
		command string
		want    string
	}{
		{"/blocklist spam", "[keep spam] ban"},
		{"/unblacklist keep", "[] warn"},
		{"/blacklistaction mute", "[keep] mute"},
	} {
		for _, limited := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/limited=%t", tc.command, limited), func(t *testing.T) {
				f := newAdminChatFixture(t)
				if limited {
					key := fmt.Sprint(f.admin.Id)
					full := string(f.client.membersByUser[key])
					restricted := strings.ReplaceAll(full, `"can_restrict_members":true`, `"can_restrict_members":false`)
					f.client.membersByUser[key] = json.RawMessage(restricted)
					f.client.adminsByChat[fmt.Sprint(f.chat.Id)] = json.RawMessage(strings.ReplaceAll(string(f.client.adminsByChat[fmt.Sprint(f.chat.Id)]), full, restricted))
				}
				if err := blacklists.AddBlacklist(f.chat.Id, "keep"); err != nil {
					t.Fatal(err)
				}
				if err := blacklists.SetBlacklistAction(f.chat.Id, "ban", 0); err != nil {
					t.Fatal(err)
				}
				state := func() string {
					settings := blacklists.GetBlacklistSettings(f.chat.Id)
					words := settings.Triggers()
					slices.Sort(words)
					return fmt.Sprint(words, " ", settings.Action())
				}
				d := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
				LoadBlacklists(d)
				LoadBotUpdates(d)
				tap := anonymousAdminProof(t, d, f, newModuleMessageContext(f.bot, f.chat, f.admin, tc.command))
				if got := state(); got != "[keep] ban" {
					t.Fatalf("before proof state = %q, want unchanged blacklist", got)
				}
				before := len(f.client.callsFor("sendMessage"))
				if err := d.ProcessUpdate(f.bot, tap, nil); err != nil {
					t.Fatal(err)
				}
				want := tc.want
				if limited {
					want = "[keep] ban"
				}
				if got := state(); got != want {
					t.Fatalf("after proof state = %q, want %q", got, want)
				}
				sent := f.client.callsFor("sendMessage")[before:]
				if len(sent) != 1 {
					t.Fatalf("after proof replies = %v, want one result", sent)
				}
				if limited {
					assertParams(t, sent[0], map[string]any{"text": "You don't have permission to restrict users in this group!"})
				}
			})
		}
	}
}

func TestBlacklistPipelineKeepsRefusalOrder(t *testing.T) {
	useRealLocales(t)
	for _, command := range []string{"/addblacklist spam", "/rmblacklist keep", "/blaction mute"} {
		for _, tc := range []struct {
			user string
			bot  string
			want string
		}{
			{"member", "admin", "You need to be an admin to do this."},
			{"member", "member", "I'm not an admin in this chat!"},
			{"limited", "limited", "You don't have permission to restrict users in this group!"},
			{"admin", "limited", "I can't restrict people here! Make sure I'm admin and can restrict other members."},
		} {
			for _, pm := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/%s/PM=%t", command, tc.user, tc.bot, pm), func(t *testing.T) {
					f := newAdminChatFixture(t)
					if tc.user == "limited" {
						key := fmt.Sprint(f.admin.Id)
						full := string(f.client.membersByUser[key])
						restricted := strings.ReplaceAll(full, `"can_restrict_members":true`, `"can_restrict_members":false`)
						f.client.membersByUser[key] = json.RawMessage(restricted)
						f.client.adminsByChat[fmt.Sprint(f.chat.Id)] = json.RawMessage(strings.ReplaceAll(string(f.client.adminsByChat[fmt.Sprint(f.chat.Id)]), full, restricted))
					}
					botJSON := `{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"},` + fixtureAdminRights + `}`
					switch tc.bot {
					case "member":
						f.client.membersByUser["999"] = json.RawMessage(`{"status":"member","user":{"id":999,"is_bot":true,"first_name":"Alita"}}`)
						f.client.adminsByChat[fmt.Sprint(f.chat.Id)] = json.RawMessage("[" + string(f.client.membersByUser[fmt.Sprint(f.admin.Id)]) + "]")
					case "limited":
						restricted := strings.ReplaceAll(botJSON, `"can_restrict_members":true`, `"can_restrict_members":false`)
						f.client.membersByUser["999"] = json.RawMessage(restricted)
						f.client.adminsByChat[fmt.Sprint(f.chat.Id)] = json.RawMessage(strings.ReplaceAll(string(f.client.adminsByChat[fmt.Sprint(f.chat.Id)]), botJSON, restricted))
					}
					actor := f.admin
					if tc.user == "member" {
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
					LoadBlacklists(d)
					if err := d.ProcessUpdate(f.bot, newModuleMessageContext(f.bot, origin, actor, command).Update, nil); err != nil {
						t.Fatal(err)
					}
					if got := sentTexts(f.client); len(got) != 1 || got[0] != tc.want {
						t.Fatalf("replies = %q, want only %q", got, tc.want)
					}
					if got := blacklists.GetBlacklistSettings(f.chat.Id).Triggers(); len(got) != 0 {
						t.Fatalf("refused command stored blacklist words: %q", got)
					}
				})
			}
		}
	}
}
