//go:build testtools

package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/filters"
)

func TestFilterCommandsPreserveReplyThroughAdminProof(t *testing.T) {
	useRealLocales(t)
	for _, command := range []string{"/filter reply", "/addfilter reply", "/stop keep", "/rmfilter keep", "/removefilter keep"} {
		for _, limited := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/limited=%t", command, limited), func(t *testing.T) {
				f := newAdminChatFixture(t)
				if limited {
					key := fmt.Sprint(f.admin.Id)
					full := string(f.client.membersByUser[key])
					restricted := strings.ReplaceAll(full, `"can_change_info":true`, `"can_change_info":false`)
					f.client.membersByUser[key] = json.RawMessage(restricted)
					f.client.adminsByChat[fmt.Sprint(f.chat.Id)] = json.RawMessage(strings.ReplaceAll(string(f.client.adminsByChat[fmt.Sprint(f.chat.Id)]), full, restricted))
				}
				if err := filters.AddFilter(f.chat.Id, "keep", "original", "", nil, db.TEXT); err != nil {
					t.Fatal(err)
				}
				d := filtersDispatcher()
				LoadBotUpdates(d)
				ctx := newModuleMessageContext(f.bot, f.chat, f.admin, command)
				ctx.Message.ReplyToMessage = &gotgbot.Message{
					MessageId: 100, Chat: f.chat, From: &f.member,
					Caption: "replied content", Document: &gotgbot.Document{FileId: "reply-document"},
				}
				tap := anonymousAdminProof(t, d, f, ctx)
				if !filters.DoesFilterExists(f.chat.Id, "keep") || filters.DoesFilterExists(f.chat.Id, "reply") {
					t.Fatal("filter changed before proof")
				}
				before := len(f.client.callsFor("sendMessage"))
				if err := d.ProcessUpdate(f.bot, tap, nil); err != nil {
					t.Fatal(err)
				}
				replies := f.client.callsFor("sendMessage")[before:]
				if len(replies) != 1 {
					t.Fatalf("after proof replies = %v, want one result", replies)
				}
				if limited {
					assertParams(t, replies[0], map[string]any{"text": "You don't have permission to change info in this group!"})
					if !filters.DoesFilterExists(f.chat.Id, "keep") || filters.DoesFilterExists(f.chat.Id, "reply") {
						t.Fatal("limited tapper changed filters")
					}
				} else if strings.Contains(command, "filter reply") {
					rows, err := filters.ListFiltersContext(context.Background(), f.chat.Id, "reply", 0, 0)
					if err != nil || len(rows) != 1 || rows[0].FileID != "reply-document" || rows[0].FilterReply != "replied content" {
						t.Fatalf("saved reply = %+v, error = %v", rows, err)
					}
				} else if filters.DoesFilterExists(f.chat.Id, "keep") {
					t.Fatal("filter was not removed after proof")
				}
			})
		}
	}
}

func TestFilterPipelineKeepsRefusalAndEmptyOrder(t *testing.T) {
	useRealLocales(t)
	for _, command := range []string{"/filter key value", "/stop key"} {
		for _, tc := range []struct {
			role string
			want string
		}{
			{"member", "You need to be an admin to do this."},
			{"limited", "You don't have permission to change info in this group!"},
		} {
			for _, pm := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/PM=%t", command, tc.role, pm), func(t *testing.T) {
					f := newAdminChatFixture(t)
					actor := f.member
					if tc.role == "limited" {
						actor = f.admin
						key := fmt.Sprint(actor.Id)
						full := string(f.client.membersByUser[key])
						restricted := strings.ReplaceAll(full, `"can_change_info":true`, `"can_change_info":false`)
						f.client.membersByUser[key] = json.RawMessage(restricted)
						f.client.adminsByChat[fmt.Sprint(f.chat.Id)] = json.RawMessage(strings.ReplaceAll(string(f.client.adminsByChat[fmt.Sprint(f.chat.Id)]), full, restricted))
					}
					origin := f.chat
					if pm {
						origin = privateChatOf(actor)
						if err := connections.ConnectId(actor.Id, f.chat.Id); err != nil {
							t.Fatal(err)
						}
						t.Cleanup(func() { _ = connections.DisconnectId(actor.Id) })
					}
					if err := filtersDispatcher().ProcessUpdate(f.bot, newModuleMessageContext(f.bot, origin, actor, command).Update, nil); err != nil {
						t.Fatal(err)
					}
					assertParams(t, f.client.onlyCall(t, "sendMessage"), map[string]any{"text": tc.want, "chat_id": origin.Id})
					if filters.CountFilters(f.chat.Id) != 0 {
						t.Fatal("refused filter command changed state")
					}
				})
			}
		}
	}
	for _, populated := range []bool{false, true} {
		t.Run(fmt.Sprintf("stopall/populated=%t", populated), func(t *testing.T) {
			f := newAdminChatFixture(t)
			want := "There are no filters in this chat!"
			if populated {
				if err := filters.AddFilter(f.chat.Id, "keep", "original", "", nil, db.TEXT); err != nil {
					t.Fatal(err)
				}
				want = "Only group creator to do this!"
			}
			if err := filtersDispatcher().ProcessUpdate(f.bot, newModuleMessageContext(f.bot, f.chat, f.member, "/stopall").Update, nil); err != nil {
				t.Fatal(err)
			}
			assertParams(t, f.client.onlyCall(t, "sendMessage"), map[string]any{"text": want})
		})
	}
}
