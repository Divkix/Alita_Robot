//go:build testtools

package modules

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/disabling"
	"github.com/divkix/Alita_Robot/alita/db/notes"
)

func TestNoteCommandsPreserveReplyThroughAdminProof(t *testing.T) {
	useRealLocales(t)
	for _, command := range []string{"/save reply", "/addnote reply", "/clear keep", "/rmnote keep"} {
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
				addTestNote(t, f.chat.Id, "keep")
				d := notesDispatcher()
				LoadBotUpdates(d)
				ctx := newModuleMessageContext(f.bot, f.chat, f.admin, command)
				ctx.Message.ReplyToMessage = &gotgbot.Message{
					MessageId: 100, Chat: f.chat, From: &f.member,
					Caption: "replied content", Document: &gotgbot.Document{FileId: "reply-document"},
				}
				tap := anonymousAdminProof(t, d, f, ctx)
				if !notes.DoesNoteExists(f.chat.Id, "keep") || notes.DoesNoteExists(f.chat.Id, "reply") {
					t.Fatal("notes changed before proof")
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
					if !notes.DoesNoteExists(f.chat.Id, "keep") || notes.DoesNoteExists(f.chat.Id, "reply") {
						t.Fatal("limited tapper changed notes")
					}
				} else if strings.Contains(command, "note reply") || strings.HasPrefix(command, "/save") {
					row := notes.GetNote(f.chat.Id, "reply")
					if row == nil || row.FileID != "reply-document" || row.NoteContent != "replied content" || row.MsgType != db.DOCUMENT {
						t.Fatalf("saved reply = %+v", row)
					}
				} else if notes.DoesNoteExists(f.chat.Id, "keep") {
					t.Fatal("note was not removed after proof")
				}
			})
		}
	}
}

func TestNotesPipelineKeepsRefusalOrder(t *testing.T) {
	useRealLocales(t)
	for _, command := range []string{"/save key value", "/clear key", "/clear"} {
		for _, tc := range []struct {
			role string
			bot  bool
			want string
		}{
			{"member", true, "You need to be an admin to do this."},
			{"member", false, "I'm not an admin in this chat!"},
			{"limited", true, "You don't have permission to change info in this group!"},
			{"limited", false, "I'm not an admin in this chat!"},
		} {
			for _, pm := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/bot=%t/PM=%t", command, tc.role, tc.bot, pm), func(t *testing.T) {
					f := newAdminChatFixture(t)
					actor := f.member
					want := tc.want
					if tc.role == "limited" {
						actor = f.admin
						key := fmt.Sprint(actor.Id)
						full := string(f.client.membersByUser[key])
						restricted := strings.ReplaceAll(full, `"can_change_info":true`, `"can_change_info":false`)
						f.client.membersByUser[key] = json.RawMessage(restricted)
						f.client.adminsByChat[fmt.Sprint(f.chat.Id)] = json.RawMessage(strings.ReplaceAll(string(f.client.adminsByChat[fmt.Sprint(f.chat.Id)]), full, restricted))
						if tc.bot && command == "/clear" {
							want = "Please give a keyword to remove!"
						}
					}
					if !tc.bot {
						f.client.adminsByChat[fmt.Sprint(f.chat.Id)] = json.RawMessage("[" + string(f.client.membersByUser[fmt.Sprint(f.admin.Id)]) + "]")
						f.client.membersByChat = map[string]json.RawMessage{
							fmt.Sprint(f.chat.Id) + "|999": json.RawMessage(`{"status":"member","user":{"id":999,"is_bot":true,"first_name":"Alita"}}`),
						}
					}
					origin := f.chat
					if pm {
						origin = privateChatOf(actor)
						if err := connections.ConnectId(actor.Id, f.chat.Id); err != nil {
							t.Fatal(err)
						}
						t.Cleanup(func() { _ = connections.DisconnectId(actor.Id) })
					}
					if err := notesDispatcher().ProcessUpdate(f.bot, newModuleMessageContext(f.bot, origin, actor, command).Update, nil); err != nil {
						t.Fatal(err)
					}
					assertParams(t, f.client.onlyCall(t, "sendMessage"), map[string]any{"text": want, "chat_id": origin.Id})
					if notes.DoesNoteExists(f.chat.Id, "key") {
						t.Fatal("refused command saved a note")
					}
				})
			}
		}
	}
}

// Every way of naming a note to /clear reaches it, and every form that can
// delete is refused to an admin without the change-info right.
func TestClearNoteNormalisesTheNameAndChecksTheRightInEveryForm(t *testing.T) {
	useRealLocales(t)
	for _, command := range []string{"/clear  rules", `/clear "rules"`, "/clear #rules", "/clear RULES", "/clear rules please", "/clear\nrules"} {
		for _, limited := range []bool{false, true} {
			t.Run(fmt.Sprintf("%q/limited=%t", command, limited), func(t *testing.T) {
				f := newAdminChatFixture(t)
				addTestNote(t, f.chat.Id, "rules")
				if limited {
					key := fmt.Sprint(f.admin.Id)
					full := string(f.client.membersByUser[key])
					restricted := strings.ReplaceAll(full, `"can_change_info":true`, `"can_change_info":false`)
					f.client.membersByUser[key] = json.RawMessage(restricted)
					f.client.adminsByChat[fmt.Sprint(f.chat.Id)] = json.RawMessage(strings.ReplaceAll(string(f.client.adminsByChat[fmt.Sprint(f.chat.Id)]), full, restricted))
				}
				if err := notesDispatcher().ProcessUpdate(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, command).Update, nil); err != nil {
					t.Fatal(err)
				}
				// onlyCall: exactly one reply was sent, the refusal or the confirmation.
				text := fmt.Sprint(f.client.onlyCall(t, "sendMessage").Params["text"])
				gone := notes.GetNote(f.chat.Id, "rules") == nil
				if limited {
					if gone {
						t.Fatalf("limited admin: note removed after %q; reply %q, want the note kept", command, text)
					}
					return
				}
				if !gone {
					t.Fatalf("note still stored after %q; reply %q", command, text)
				}
			})
		}
	}
}

func TestNotesPipelineKeepsVisibilityAndRawAccess(t *testing.T) {
	useRealLocales(t)
	for _, command := range []string{"/get public", "/get public noformat", "/get secret", "/get secret noformat", "/get absent noformat", "/get private", "/notes", "/saved"} {
		for _, admin := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/admin=%t", command, admin), func(t *testing.T) {
				f := newAdminChatFixture(t)
				for _, note := range []struct {
					name string
					priv bool
					adm  bool
				}{{"public", false, false}, {"secret", false, true}, {"private", true, false}} {
					if err := notes.AddNote(f.chat.Id, note.name, "<b>"+note.name+" body</b>", "", nil, db.TEXT, note.priv, false, note.adm, false, false, false); err != nil {
						t.Fatal(err)
					}
				}
				actor := f.member
				if admin {
					actor = f.admin
				}
				if err := notesDispatcher().ProcessUpdate(f.bot, newModuleMessageContext(f.bot, f.chat, actor, command).Update, nil); err != nil {
					t.Fatal(err)
				}
				call := f.client.onlyCall(t, "sendMessage")
				text := fmt.Sprint(call.Params["text"])
				switch {
				case strings.Contains(command, "absent"):
					assertParams(t, call, map[string]any{"text": "Note doesn't exists!"})
				case !admin && strings.Contains(command, "secret"):
					assertParams(t, call, map[string]any{"text": "This note can only be accessed by a admin!"})
				case !admin && strings.Contains(command, "noformat"):
					assertParams(t, call, map[string]any{"text": "Only admins can execute this command!"})
				case command == "/notes" || command == "/saved":
					if !strings.Contains(text, "#public") || strings.Contains(text, "#secret") != admin {
						t.Fatalf("note visibility = %q, admin=%t", text, admin)
					}
				case command == "/get private":
					if call.Params["reply_markup"] == nil || strings.Contains(text, "private body") {
						t.Fatalf("private note did not redirect: %+v", call.Params)
					}
				default:
					if !strings.Contains(text, "body") {
						t.Fatalf("note was not sent: %q", text)
					}
				}
			})
		}
	}
}

func TestNotesAliasIsDisableableAndSharesEveryRow(t *testing.T) {
	useRealLocales(t)
	withDisableCommands(t)
	f := newAdminChatFixture(t)
	d := notesDispatcher()
	LoadDisabling(d)
	addTestNote(t, f.chat.Id, "public")
	if err := d.ProcessUpdate(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, "/disable saved").Update, nil); err != nil {
		t.Fatal(err)
	}
	if !disabling.IsCommandDisabled(f.chat.Id, "saved") {
		t.Fatal("/disable refused the saved alias")
	}
	for _, command := range []string{"/notes", "/saved"} {
		before := len(f.client.callsFor("sendMessage"))
		if err := d.ProcessUpdate(f.bot, newModuleMessageContext(f.bot, f.chat, f.member, command).Update, nil); err != nil {
			t.Fatal(err)
		}
		if len(f.client.callsFor("sendMessage")) != before {
			t.Fatalf("disabled member command %s replied", command)
		}
		if err := d.ProcessUpdate(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, command).Update, nil); err != nil {
			t.Fatal(err)
		}
		if len(f.client.callsFor("sendMessage")) != before+1 {
			t.Fatalf("disabled admin command %s was silenced", command)
		}
	}
}

func TestClearAllNotesKeepsEmptyResultBeforeOwnerRefusal(t *testing.T) {
	useRealLocales(t)
	for _, populated := range []bool{false, true} {
		t.Run(fmt.Sprint(populated), func(t *testing.T) {
			f := newAdminChatFixture(t)
			want := "There are no notes in this chat!"
			if populated {
				addTestNote(t, f.chat.Id, "keep")
				want = "Only Chat Creator can use this command."
			}
			if err := notesDispatcher().ProcessUpdate(f.bot, newModuleMessageContext(f.bot, f.chat, f.member, "/clearall").Update, nil); err != nil {
				t.Fatal(err)
			}
			call := f.client.onlyCall(t, "sendMessage")
			assertParams(t, call, map[string]any{"text": want})
			if call.Params["reply_markup"] != nil {
				t.Fatal("non-owner got a clear-all confirmation")
			}
		})
	}
}
