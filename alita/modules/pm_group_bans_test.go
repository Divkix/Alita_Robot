//go:build testtools

package modules

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/connections"
)

func bansPMDispatcher() *ext.Dispatcher {
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadBans(dispatcher)
	return dispatcher
}

// /dban and /dkick need a reply to a group message, so they stay group-only:
// from PM they act on no one and answer as before.
func TestDeleteBanCommandsInPMStayGroupOnly(t *testing.T) {
	for _, cmd := range []string{"dban", "dkick"} {
		t.Run(cmd, func(t *testing.T) {
			f, _ := pmModerationFixture(t)
			requireGroupOnlyInPM(t, bansPMDispatcher, f, cmd)
		})
	}
}

// requireGroupOnlyInPM sends cmd from PM with the group named, on the
// connection, in reply to the bot and, disconnected, without a group, and
// wants each call answered in the PM with chat_status_group_only_error alone,
// and nothing restricted or deleted.
func requireGroupOnlyInPM(t *testing.T, dispatcher func() *ext.Dispatcher, f adminChatFixture, cmd string) {
	t.Helper()
	bot := gotgbot.User{Id: 999, IsBot: true, FirstName: "Alita"}
	want := translated(t, "chat_status_group_only_error")
	for _, call := range []struct {
		name string
		send func()
	}{
		{"with the group named", func() { processPM(t, dispatcher(), f, f.admin.Id, "/"+cmd+" @fixturegroup @victim") }},
		{"on the connection", func() { processPM(t, dispatcher(), f, f.admin.Id, "/"+cmd+" @victim") }},
		{"in reply to the bot", func() { processPMReply(t, dispatcher(), f, f.admin, bot, "/"+cmd+" @fixturegroup @victim") }},
		{"without a group", func() {
			if err := connections.DisconnectId(f.admin.Id); err != nil {
				t.Fatalf("DisconnectId() error = %v", err)
			}
			processPM(t, dispatcher(), f, f.admin.Id, "/"+cmd+" @victim")
		}},
	} {
		before := len(f.client.callsFor("sendMessage"))
		call.send()
		replies := f.client.callsFor("sendMessage")[before:]
		if len(replies) != 1 || fmt.Sprint(replies[0].Params["text"]) != want || fmt.Sprint(replies[0].Params["chat_id"]) != fmt.Sprint(f.admin.Id) {
			t.Fatalf("/%s from PM %s: want only %q in the PM; sent:\n%s", cmd, call.name, want, sentText(f.client))
		}
	}
	for _, method := range []string{"banChatMember", "unbanChatMember", "restrictChatMember", "deleteMessage"} {
		if calls := f.client.callsFor(method); len(calls) != 0 {
			t.Fatalf("/%s from PM called %s: %+v", cmd, method, calls)
		}
	}
}

// From PM the user can be named by ID or by a text mention after the group.
func TestBanCommandsInPMWithGroupTakeUserByIDAndMention(t *testing.T) {
	t.Run("id", func(t *testing.T) {
		f, _ := pmModerationFixture(t)

		processPM(t, bansPMDispatcher(), f, f.admin.Id, fmt.Sprintf("/kick @fixturegroup %d", victimID))

		if !targetedIn(f, "unbanChatMember", f.chat.Id, victimID) {
			t.Fatalf("/kick from PM with the user's ID did not kick them; sent:\n%s", sentText(f.client))
		}
	})
	t.Run("text mention", func(t *testing.T) {
		f, _ := pmModerationFixture(t)
		update := newModuleMessageContext(f.bot, privateChatOf(f.admin), f.admin, "/ban @fixturegroup Victim spam").Update
		update.Message.Entities = []gotgbot.MessageEntity{
			{Type: "text_mention", Offset: 19, Length: 6, User: &gotgbot.User{Id: victimID, FirstName: "Victim"}},
		}

		if err := bansPMDispatcher().ProcessUpdate(f.bot, update, nil); err != nil {
			t.Fatalf("ProcessUpdate() error = %v", err)
		}

		if !targetedIn(f, "banChatMember", f.chat.Id, victimID) {
			t.Fatalf("/ban from PM with a text mention did not ban the user; sent:\n%s", sentText(f.client))
		}
		if !repliedIn(f, f.admin.Id, "spam") {
			t.Fatalf("/ban from PM with a text mention lost the reason; sent:\n%s", sentText(f.client))
		}
	})
}

// From PM, /tban reads the time after the user as in the group.
func TestTbanInPMWithGroupReadsTimeAfterUser(t *testing.T) {
	f, _ := pmModerationFixture(t)
	before := time.Now().Unix()

	processPM(t, bansPMDispatcher(), f, f.admin.Id, "/tban @fixturegroup @victim 2h flooding")

	calls := f.client.callsFor("banChatMember")
	if len(calls) != 1 || fmt.Sprint(calls[0].Params["chat_id"]) != fmt.Sprint(f.chat.Id) {
		t.Fatalf("/tban from PM: banChatMember calls %+v, want one in the named group", calls)
	}
	until, err := strconv.ParseInt(fmt.Sprint(calls[0].Params["until_date"]), 10, 64)
	if err != nil || until < before+7200 || until > time.Now().Unix()+7200 {
		t.Fatalf("/tban from PM: until_date = %v, want two hours from now", calls[0].Params["until_date"])
	}
	if !repliedIn(f, f.admin.Id, "flooding") {
		t.Fatalf("/tban from PM lost the reason; sent:\n%s", sentText(f.client))
	}
}

// From PM the silent commands answer like the plain ones: there is no command
// message in the group to delete.
func TestSilentBanCommandsInPMWithGroupAnswerInPM(t *testing.T) {
	for _, cmd := range []string{"sban", "skick"} {
		t.Run(cmd, func(t *testing.T) {
			f, _ := pmModerationFixture(t)

			processPM(t, bansPMDispatcher(), f, f.admin.Id, "/"+cmd+" @fixturegroup @victim")

			if len(f.client.callsFor("deleteMessage")) != 0 {
				t.Fatalf("/%s from PM deleted a message", cmd)
			}
			if !repliedIn(f, f.admin.Id, "Victim") {
				t.Fatalf("/%s from PM did not answer in the PM; sent:\n%s", cmd, sentText(f.client))
			}
		})
	}
}

// The buttons a bans command sends from PM act on the named group.
func TestBanButtonsFromPMActOnNamedGroup(t *testing.T) {
	for _, tc := range []struct {
		cmd, ns, action, method string
	}{
		{"ban", "unrestrict", "unban", "unbanChatMember"},
		{"restrict", "restrict", "ban", "banChatMember"},
		{"restrict", "restrict", "kick", "unbanChatMember"},
		{"restrict", "restrict", "mute", "restrictChatMember"},
		{"unrestrict", "unrestrict", "unban", "unbanChatMember"},
		{"unrestrict", "unrestrict", "unmute", "restrictChatMember"},
	} {
		t.Run(tc.cmd+"/"+tc.action, func(t *testing.T) {
			f, connected := pmModerationFixture(t)
			dispatcher := bansPMDispatcher()
			processPM(t, dispatcher, f, f.admin.Id, "/"+tc.cmd+" @fixturegroup @victim")
			data := buttonFor(t, f, 0, tc.ns, tc.action)
			if len(data) > 64 {
				t.Fatalf("button data %q is %d bytes, over Telegram's 64", data, len(data))
			}
			calls := len(f.client.callsFor(tc.method))

			pressConfirmInPM(t, dispatcher, f, f.admin, data)

			if len(f.client.callsFor(tc.method)) == calls || !targetedIn(f, tc.method, f.chat.Id, victimID) {
				t.Fatalf("the %s button from PM did not act on the named group; calls:\n%+v", tc.action, f.client.callsFor(tc.method))
			}
			if targetedIn(f, tc.method, connected, victimID) {
				t.Fatalf("the %s button from PM acted on the connected group", tc.action)
			}
		})
	}
}

// A button from PM checks the presser against the group it names: a member
// without the right there, or an admin pressing a button forged to name a
// group where they have none, is refused.
func TestBanButtonsFromPMCheckPresserInNamedGroup(t *testing.T) {
	for _, tc := range []struct {
		ns, action, method string
	}{
		{"restrict", "ban", "banChatMember"},
		{"unrestrict", "unban", "unbanChatMember"},
	} {
		t.Run(tc.ns+"/member", func(t *testing.T) {
			f, _ := pmModerationFixture(t)
			data := encodeCallbackData(tc.ns, map[string]string{"a": tc.action, "c": fmt.Sprint(f.chat.Id), "u": fmt.Sprint(victimID)})

			pressConfirmInPM(t, bansPMDispatcher(), f, f.member, data)

			if len(f.client.callsFor(tc.method)) != 0 {
				t.Fatalf("a member's press of the %s button acted", tc.action)
			}
			if len(f.client.callsFor("answerCallbackQuery")) == 0 {
				t.Fatalf("a member's press of the %s button got no answer", tc.action)
			}
		})
		t.Run(tc.ns+"/forged group", func(t *testing.T) {
			f, _ := pmModerationFixture(t)
			other := uniqueModuleChatID()
			f.setMemberStatus(other, f.admin.Id, "member")
			data := encodeCallbackData(tc.ns, map[string]string{"a": tc.action, "c": fmt.Sprint(other), "u": fmt.Sprint(victimID)})

			pressConfirmInPM(t, bansPMDispatcher(), f, f.admin, data)

			if len(f.client.callsFor(tc.method)) != 0 {
				t.Fatalf("a %s button naming a group the presser has no right in acted", tc.action)
			}
			if len(f.client.callsFor("answerCallbackQuery")) == 0 {
				t.Fatalf("a %s button naming a group the presser has no right in got no answer", tc.action)
			}
		})
	}
}

// Buttons a bans command sends in the group carry no group, as before PM
// support: the chat they are pressed in is the group.
func TestBanButtonsInGroupNameNoGroup(t *testing.T) {
	for _, tc := range []struct {
		cmd, ns string
	}{
		{"ban", "unrestrict"},
		{"restrict", "restrict"},
		{"unrestrict", "unrestrict"},
	} {
		t.Run(tc.cmd, func(t *testing.T) {
			f, _ := pmModerationFixture(t)

			update := newModuleMessageContext(f.bot, f.chat, f.admin, "/"+tc.cmd+" @victim").Update
			if err := bansPMDispatcher().ProcessUpdate(f.bot, update, nil); err != nil {
				t.Fatalf("ProcessUpdate(/%s) error = %v", tc.cmd, err)
			}

			buttons := moderationButtons(f, 0, tc.ns)
			if len(buttons) == 0 {
				t.Fatalf("/%s in the group sent no %s button; sent:\n%s", tc.cmd, tc.ns, sentText(f.client))
			}
			for _, data := range buttons {
				decoded, _ := decodeCallbackData(data)
				if c, named := decoded.Field("c"); named {
					t.Fatalf("/%s in the group sent a button naming group %q: %s", tc.cmd, c, data)
				}
			}
		})
	}
}

// renderedText is the text Telegram shows for an HTML message, which is what
// a button press brings back as the message's text: tags dropped, entities
// decoded.
func renderedText(htmlText string) string {
	return html.UnescapeString(regexp.MustCompile(`<[^>]*>`).ReplaceAllString(htmlText, ""))
}

// The unban button of a /ban and the unmute button of a /mute sent from PM
// for a group whose title has HTML characters strike the reply through with
// the title escaped exactly once, so the edit goes through.
func TestUnrestrictButtonInPMWithMarkupInGroupTitleEscapes(t *testing.T) {
	for _, tc := range []struct {
		cmd, action string
		dispatcher  func() *ext.Dispatcher
	}{
		{"ban", "unban", bansPMDispatcher},
		{"mute", "unmute", mutesPMDispatcher},
	} {
		t.Run(tc.cmd, func(t *testing.T) {
			f, _ := pmModerationFixture(t)
			named := `Tom & <Jerry>`
			for _, key := range []string{fmt.Sprint(f.chat.Id), "@fixturegroup"} {
				f.client.chatsByID[key] = json.RawMessage(fmt.Sprintf(`{"id":%d,"type":"supergroup","title":%q}`, f.chat.Id, named))
			}
			dispatcher := tc.dispatcher()
			processPM(t, dispatcher, f, f.admin.Id, "/"+tc.cmd+" @fixturegroup @victim")
			data := buttonFor(t, f, 0, "unrestrict", tc.action)
			reply := renderedText(fmt.Sprint(f.client.callsFor("sendMessage")[0].Params["text"]))
			if !strings.Contains(reply, named) {
				t.Fatalf("/%s from PM shows %q, want the title %q", tc.cmd, reply, named)
			}

			update := newModuleCallbackContext(f.bot, privateChatOf(f.admin), f.admin, data).Update
			shown := update.CallbackQuery.Message.(gotgbot.Message)
			shown.Text = reply
			update.CallbackQuery.Message = shown
			if err := dispatcher.ProcessUpdate(f.bot, update, nil); err != nil {
				t.Fatalf("ProcessUpdate(callback %q) error = %v", data, err)
			}

			edit := f.client.onlyCall(t, "editMessageText")
			text := fmt.Sprint(edit.Params["text"])
			if !strings.Contains(text, html.EscapeString(named)) || strings.Contains(text, named) || strings.Contains(text, "&amp;amp;") {
				t.Fatalf("edited text = %q, want the title escaped once as %q", text, html.EscapeString(named))
			}
			if struck := "<s>" + html.EscapeString(reply) + "</s>"; !strings.HasPrefix(text, struck) {
				t.Fatalf("edited text = %q, want it to start with the reply struck through: %q", text, struck)
			}
		})
	}
}

// The group line is HTML-escaped, so the menus of /restrict and /unrestrict
// sent from PM are HTML too: the title shows as typed, not as entities.
func TestRestrictMenusInPMAreSentAsHTML(t *testing.T) {
	for _, cmd := range []string{"restrict", "unrestrict"} {
		t.Run(cmd, func(t *testing.T) {
			f, _ := pmModerationFixture(t)
			named := `Tom & <Jerry>`
			for _, key := range []string{fmt.Sprint(f.chat.Id), "@fixturegroup"} {
				f.client.chatsByID[key] = json.RawMessage(fmt.Sprintf(`{"id":%d,"type":"supergroup","title":%q}`, f.chat.Id, named))
			}

			processPM(t, bansPMDispatcher(), f, f.admin.Id, "/"+cmd+" @fixturegroup @victim")

			want := groupLine(t, named, f.chat.Id)
			for _, call := range f.client.callsFor("sendMessage") {
				if strings.Contains(fmt.Sprint(call.Params["text"]), want) {
					if got := fmt.Sprint(call.Params["parse_mode"]); got != gotgbot.ParseModeHTML {
						t.Fatalf("/%s menu from PM has parse_mode %q, want %q; text %q", cmd, got, gotgbot.ParseModeHTML, call.Params["text"])
					}
					return
				}
			}
			t.Fatalf("/%s from PM sent no menu naming the group %q; sent:\n%s", cmd, want, sentText(f.client))
		})
	}
}

// A /restrict menu button replaces the menu with what it did. Pressed in PM,
// the new text still names the group, its title escaped once; pressed in the
// group, it names none.
func TestRestrictButtonInPMNamesGroupInEdit(t *testing.T) {
	for _, action := range []string{"ban", "kick", "mute"} {
		t.Run(action, func(t *testing.T) {
			f, _ := pmModerationFixture(t)
			named := `Tom & <Jerry>`
			for _, key := range []string{fmt.Sprint(f.chat.Id), "@fixturegroup"} {
				f.client.chatsByID[key] = json.RawMessage(fmt.Sprintf(`{"id":%d,"type":"supergroup","title":%q}`, f.chat.Id, named))
			}
			dispatcher := bansPMDispatcher()

			processPM(t, dispatcher, f, f.admin.Id, "/restrict @fixturegroup @victim")
			pressConfirmInPM(t, dispatcher, f, f.admin, buttonFor(t, f, 0, "restrict", action))

			edit := f.client.onlyCall(t, "editMessageText")
			text := fmt.Sprint(edit.Params["text"])
			if want := groupLine(t, named, f.chat.Id); strings.Count(text, want) != 1 || strings.Contains(text, named) {
				t.Fatalf("%s button from PM edited the menu to %q, want the group line %q once", action, text, want)
			}

			before := len(f.client.callsFor("sendMessage"))
			update := newModuleMessageContext(f.bot, f.chat, f.admin, "/restrict @victim").Update
			if err := dispatcher.ProcessUpdate(f.bot, update, nil); err != nil {
				t.Fatalf("ProcessUpdate(/restrict) error = %v", err)
			}
			update = newModuleCallbackContext(f.bot, f.chat, f.admin, buttonFor(t, f, before, "restrict", action)).Update
			if err := dispatcher.ProcessUpdate(f.bot, update, nil); err != nil {
				t.Fatalf("ProcessUpdate(%s button) error = %v", action, err)
			}
			edits := f.client.callsFor("editMessageText")
			prefix := strings.SplitN(translated(t, "connections_pm_group_confirm_target"), "{", 2)[0]
			if text := fmt.Sprint(edits[len(edits)-1].Params["text"]); len(edits) != 2 || strings.Contains(text, prefix) {
				t.Fatalf("%s button in the group edited the menu to %q (%d edits), want no group line", action, text, len(edits))
			}
		})
	}
}

// The group line is only text: when the group's title cannot be looked up, a
// /restrict menu button pressed in PM still acts, replaces the menu without
// the line, and answers the press.
func TestRestrictButtonInPMActsWhenGroupLookupFails(t *testing.T) {
	for _, tc := range []struct{ action, method string }{
		{"ban", "banChatMember"},
		{"kick", "unbanChatMember"},
		{"mute", "restrictChatMember"},
	} {
		t.Run(tc.action, func(t *testing.T) {
			f, _ := pmModerationFixture(t)
			dispatcher := bansPMDispatcher()
			processPM(t, dispatcher, f, f.admin.Id, "/restrict @fixturegroup @victim")
			data := buttonFor(t, f, 0, "restrict", tc.action)
			f.client.errorsByChat = map[string]error{"getChat|" + fmt.Sprint(f.chat.Id): errors.New("getChat failed")}

			update := newModuleCallbackContext(f.bot, privateChatOf(f.admin), f.admin, data).Update
			_ = dispatcher.ProcessUpdate(f.bot, update, nil)

			if !targetedIn(f, tc.method, f.chat.Id, victimID) {
				t.Fatalf("%s button from PM did not act when getChat of the group failed; calls:\n%+v", tc.action, f.client.callsFor(tc.method))
			}
			edits := f.client.callsFor("editMessageText")
			prefix := strings.SplitN(translated(t, "connections_pm_group_confirm_target"), "{", 2)[0]
			if len(edits) != 1 || strings.Contains(fmt.Sprint(edits[0].Params["text"]), prefix) {
				t.Fatalf("%s button from PM edited the menu %d times, want once without the group line; edits: %+v", tc.action, len(edits), edits)
			}
			if len(f.client.callsFor("answerCallbackQuery")) == 0 {
				t.Fatalf("%s button from PM left the press unanswered", tc.action)
			}
		})
	}
}
