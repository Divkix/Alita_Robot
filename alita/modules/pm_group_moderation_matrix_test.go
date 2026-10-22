//go:build testtools

package modules

import (
	"encoding/json"
	"fmt"
	"html"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

// The tests in this file run every moderation command that acts on one user
// of one group from PM, with the group named first or connected, over the
// same checks. A command joins them with a row in pmModerationCases; tests of
// what only one command does stay in its module's test file.

// victimID is the user the moderation tests act on: @victim, a member of
// every group and an admin of none.
const victimID = 4300

// pmModerationFixture is pmAdminFixture with @victim known to getChat.
func pmModerationFixture(t *testing.T) (f adminChatFixture, connected int64) {
	t.Helper()
	f, connected = pmAdminFixture(t)
	victim := json.RawMessage(fmt.Sprintf(`{"id":%d,"type":"private","first_name":"Victim","username":"victim"}`, victimID))
	f.client.chatsByID["@victim"] = victim
	f.client.chatsByID[fmt.Sprint(victimID)] = victim
	return f, connected
}

// pmModerationCase is a moderation command that acts on one user of one
// group: "/<cmd> <user><args>". acted reports whether it acted on userID in
// chatID: restricted them, or offered the restriction menu for them. Every
// command needs the right to restrict members, from the caller and the bot.
// protectKey is the refusal for a target protected in the group, "" for a
// command that protects none (/unban).
type pmModerationCase struct {
	cmd, args  string
	dispatcher func() *ext.Dispatcher
	acted      func(t *testing.T, f adminChatFixture, chatID, userID int64) bool
	protectKey string
}

var pmModerationCases = []pmModerationCase{
	{"ban", " spam", bansPMDispatcher, calledOn("banChatMember"), "bans_ban_is_admin"},
	{"sban", "", bansPMDispatcher, calledOn("banChatMember"), "bans_ban_is_admin"},
	{"tban", " 2h", bansPMDispatcher, calledOn("banChatMember"), "bans_ban_is_admin"},
	{"unban", "", bansPMDispatcher, calledOn("unbanChatMember"), ""},
	{"kick", "", bansPMDispatcher, calledOn("unbanChatMember"), "bans_kick_cannot_kick_admin"},
	{"skick", "", bansPMDispatcher, calledOn("unbanChatMember"), "bans_kick_cannot_kick_admin"},
	{"restrict", "", bansPMDispatcher, menuFor("restrict"), "bans_restrict_admin_error"},
	{"unrestrict", "", bansPMDispatcher, menuFor("unrestrict"), "bans_unrestrict_admin_error"},
}

// targetedIn reports whether method was called on userID in chatID.
func targetedIn(f adminChatFixture, method string, chatID, userID int64) bool {
	for _, call := range f.client.callsFor(method) {
		if fmt.Sprint(call.Params["chat_id"]) == fmt.Sprint(chatID) && fmt.Sprint(call.Params["user_id"]) == fmt.Sprint(userID) {
			return true
		}
	}
	return false
}

func calledOn(method string) func(*testing.T, adminChatFixture, int64, int64) bool {
	return func(_ *testing.T, f adminChatFixture, chatID, userID int64) bool {
		return targetedIn(f, method, chatID, userID)
	}
}

// menuFor reports whether a menu of namespace for userID, acting on chatID,
// was sent: from PM its buttons name the group, in a group they need not.
func menuFor(namespace string) func(*testing.T, adminChatFixture, int64, int64) bool {
	return func(_ *testing.T, f adminChatFixture, chatID, userID int64) bool {
		for _, data := range moderationButtons(f, 0, namespace) {
			decoded, _ := decodeCallbackData(data)
			u, _ := decoded.Field("u")
			c, hasChat := decoded.Field("c")
			if u == fmt.Sprint(userID) && (c == fmt.Sprint(chatID) || !hasChat && chatID < 0) {
				return true
			}
		}
		return false
	}
}

// moderationButtons returns the callback data of the namespace's buttons in
// the messages sent since call number from.
func moderationButtons(f adminChatFixture, from int, namespace string) []string {
	var buttons []string
	for _, call := range f.client.callsFor("sendMessage")[from:] {
		markup, ok := call.Params["reply_markup"].(gotgbot.InlineKeyboardMarkup)
		if !ok {
			continue
		}
		for _, row := range markup.InlineKeyboard {
			for _, button := range row {
				if decoded, ok := decodeCallbackData(button.CallbackData, namespace); ok && decoded != nil {
					buttons = append(buttons, button.CallbackData)
				}
			}
		}
	}
	return buttons
}

// buttonFor returns the data of the namespace button with action a sent
// since call number from.
func buttonFor(t *testing.T, f adminChatFixture, from int, namespace, action string) string {
	t.Helper()
	for _, data := range moderationButtons(f, from, namespace) {
		decoded, _ := decodeCallbackData(data)
		if a, _ := decoded.Field("a"); a == action {
			return data
		}
	}
	t.Fatalf("no %s button %q was sent; sent:\n%s", namespace, action, sentText(f.client))
	return ""
}

// requireNothingSentTo fails if any message was sent to chatID.
func requireNothingSentTo(t *testing.T, f adminChatFixture, chatID int64) {
	t.Helper()
	if calledFor(f, "sendMessage", chatID) {
		t.Fatalf("a message was sent to chat %d; sent:\n%s", chatID, sentText(f.client))
	}
}

// processPMReply runs text through the dispatcher as a message user sent in
// PM in reply to a PM message from replyTo.
func processPMReply(t *testing.T, dispatcher *ext.Dispatcher, f adminChatFixture, user, replyTo gotgbot.User, text string) {
	t.Helper()
	update := newModuleMessageContext(f.bot, privateChatOf(user), user, text).Update
	update.Message.ReplyToMessage = &gotgbot.Message{
		MessageId: 55, Date: 1, Chat: privateChatOf(user), From: &replyTo, Text: "an earlier message",
	}
	if err := dispatcher.ProcessUpdate(f.bot, update, nil); err != nil {
		t.Fatalf("ProcessUpdate(%q) error = %v", text, err)
	}
}

// groupLine is the line a moderation reply sent in PM adds to name the group
// it acted on, with the title as HTML.
func groupLine(t *testing.T, title string, chatID int64) string {
	t.Helper()
	line := translated(t, "connections_pm_group_confirm_target")
	return strings.NewReplacer("{title}", html.EscapeString(title), "{id}", fmt.Sprint(chatID)).Replace(line)
}

// From PM with @fixturegroup while connected to another group, each command
// acts on @victim in the named group and answers in the PM; the connected
// group and the connection stay as they were, and nothing is sent to the
// named group.
func TestPMModerationActsOnNamedGroup(t *testing.T) {
	for _, tc := range pmModerationCases {
		t.Run(tc.cmd, func(t *testing.T) {
			f, connected := pmModerationFixture(t)

			processPM(t, tc.dispatcher(), f, f.admin.Id, "/"+tc.cmd+" @fixturegroup @victim"+tc.args)

			if !tc.acted(t, f, f.chat.Id, victimID) {
				t.Fatalf("/%s from PM did not act on @victim in the named group; sent:\n%s", tc.cmd, sentText(f.client))
			}
			if tc.acted(t, f, connected, victimID) {
				t.Fatalf("/%s from PM acted on the connected group", tc.cmd)
			}
			if !calledFor(f, "sendMessage", f.admin.Id) {
				t.Fatalf("/%s from PM did not answer in the PM; sent:\n%s", tc.cmd, sentText(f.client))
			}
			requireNothingSentTo(t, f, f.chat.Id)
			requireConnectedTo(t, f.admin.Id, connected)
		})
	}
}

// With a connection and no group named, each command acts on @victim in the
// connected group, which before took no command from PM.
func TestPMModerationOnConnection(t *testing.T) {
	for _, tc := range pmModerationCases {
		t.Run(tc.cmd, func(t *testing.T) {
			f, connected := pmModerationFixture(t)

			processPM(t, tc.dispatcher(), f, f.admin.Id, "/"+tc.cmd+" @victim"+tc.args)

			if !tc.acted(t, f, connected, victimID) {
				t.Fatalf("/%s from PM on a connection did not act on the connected group; sent:\n%s", tc.cmd, sentText(f.client))
			}
			if tc.acted(t, f, f.chat.Id, victimID) {
				t.Fatalf("/%s from PM on a connection acted on another group", tc.cmd)
			}
		})
	}
}

// From PM, a member of the named group who is no admin there gets a refusal,
// and the group and their connection stay as they were.
func TestPMModerationRefusesMember(t *testing.T) {
	for _, tc := range pmModerationCases {
		t.Run(tc.cmd, func(t *testing.T) {
			f, connected := pmModerationFixture(t)
			if err := connections.ToggleAllowConnect(f.chat.Id, true); err != nil {
				t.Fatalf("ToggleAllowConnect() error = %v", err)
			}
			if err := connections.ConnectId(f.member.Id, connected); err != nil {
				t.Fatalf("ConnectId() error = %v", err)
			}
			t.Cleanup(func() { _ = connections.DisconnectId(f.member.Id) })

			processPM(t, tc.dispatcher(), f, f.member.Id, "/"+tc.cmd+" @fixturegroup @victim"+tc.args)

			if tc.acted(t, f, f.chat.Id, victimID) {
				t.Fatalf("/%s from a member who is no admin acted on the group", tc.cmd)
			}
			if !calledFor(f, "sendMessage", f.member.Id) {
				t.Fatalf("/%s from a member who is no admin got no answer", tc.cmd)
			}
			requireConnectedTo(t, f.member.Id, connected)
		})
	}
}

// From PM, an admin of the named group without the right to restrict there
// gets the group's refusal, though they hold that right in the group they
// are connected to.
func TestPMModerationRefusesCallerWithoutRight(t *testing.T) {
	for _, tc := range pmModerationCases {
		t.Run(tc.cmd, func(t *testing.T) {
			f, connected := pmModerationFixture(t)
			admin := adminMember(f.admin.Id, false)
			admin.CanRestrictMembers = false
			seedAdminCache(t, f.chat.Id, adminMember(999, true), admin)

			processPM(t, tc.dispatcher(), f, f.admin.Id, "/"+tc.cmd+" @fixturegroup @victim"+tc.args)

			if tc.acted(t, f, f.chat.Id, victimID) || tc.acted(t, f, connected, victimID) {
				t.Fatalf("/%s from an admin without the right acted on a group", tc.cmd)
			}
			if !repliedIn(f, f.admin.Id, translated(t, "chat_status_restrict_cmd_error")) {
				t.Fatalf("/%s from an admin without the right: want chat_status_restrict_cmd_error; sent:\n%s", tc.cmd, sentText(f.client))
			}
			requireConnectedTo(t, f.admin.Id, connected)
		})
	}
}

// From PM, a command the bot cannot carry out in the named group gets the
// answer it gets in that group, in the PM.
func TestPMModerationRefusesWhenBotLacksRight(t *testing.T) {
	for _, tc := range pmModerationCases {
		t.Run(tc.cmd, func(t *testing.T) {
			f, connected := pmModerationFixture(t)
			setup := func() {
				bot := adminMember(999, true)
				bot.CanRestrictMembers = false
				seedAdminCache(t, f.chat.Id, adminMember(f.admin.Id, false), bot)
				f.client.membersByChat[fmt.Sprintf("%d|999", f.chat.Id)] = json.RawMessage(
					`{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"},"can_delete_messages":true}`)
			}
			dispatcher := tc.dispatcher()

			setup()
			update := newModuleMessageContext(f.bot, f.chat, f.admin, "/"+tc.cmd+" @victim"+tc.args).Update
			if err := dispatcher.ProcessUpdate(f.bot, update, nil); err != nil {
				t.Fatalf("ProcessUpdate(/%s) error = %v", tc.cmd, err)
			}
			inGroup := f.client.callsFor("sendMessage")
			if len(inGroup) != 1 || tc.acted(t, f, f.chat.Id, victimID) {
				t.Fatalf("/%s in the group without the bot's right: want one refusal; sent:\n%s", tc.cmd, sentText(f.client))
			}

			setup()
			processPM(t, dispatcher, f, f.admin.Id, "/"+tc.cmd+" @fixturegroup @victim"+tc.args)

			inPM := f.client.callsFor("sendMessage")[1:]
			if len(inPM) != 1 || fmt.Sprint(inPM[0].Params["text"]) != fmt.Sprint(inGroup[0].Params["text"]) {
				t.Fatalf("/%s from PM answered differently from the group; sent:\n%s", tc.cmd, sentText(f.client))
			}
			if fmt.Sprint(inPM[0].Params["chat_id"]) != fmt.Sprint(f.admin.Id) {
				t.Fatalf("/%s from PM answered in chat %v, want the PM", tc.cmd, inPM[0].Params["chat_id"])
			}
			if tc.acted(t, f, f.chat.Id, victimID) || tc.acted(t, f, connected, victimID) {
				t.Fatalf("/%s from PM acted though the bot has no right in the named group", tc.cmd)
			}
		})
	}
}

// From PM, a target protected in the named group is refused there, though it
// is no admin of the connected group; so are the bot and the caller, as in
// the group.
func TestPMModerationKeepsProtectedTargets(t *testing.T) {
	const groupAdminID = 4343
	for _, tc := range pmModerationCases {
		if tc.protectKey == "" {
			continue
		}
		for _, target := range []struct {
			name string
			id   func(f adminChatFixture) int64
		}{
			{"admin of the named group", func(adminChatFixture) int64 { return groupAdminID }},
			{"caller", func(f adminChatFixture) int64 { return f.admin.Id }},
			{"bot", func(adminChatFixture) int64 { return 999 }},
		} {
			t.Run(tc.cmd+"/"+target.name, func(t *testing.T) {
				f, connected := pmModerationFixture(t)
				seedAdminCache(t, f.chat.Id, adminMember(999, true), adminMember(f.admin.Id, false), adminMember(groupAdminID, false))
				id := target.id(f)

				processPM(t, tc.dispatcher(), f, f.admin.Id, fmt.Sprintf("/%s @fixturegroup %d%s", tc.cmd, id, tc.args))

				if tc.acted(t, f, f.chat.Id, id) || tc.acted(t, f, connected, id) {
					t.Fatalf("/%s from PM acted on the %s", tc.cmd, target.name)
				}
				// The bot is an admin of the group, so it is refused as one.
				if !repliedIn(f, f.admin.Id, translated(t, tc.protectKey)) {
					t.Fatalf("/%s from PM on the %s: want %s; sent:\n%s", tc.cmd, target.name, tc.protectKey, sentText(f.client))
				}
			})
		}
	}
}

// A reply in PM names no target: the command answers as it does in the group
// without one, and a user named after the group wins over the reply.
func TestPMModerationIgnoresReplyInPM(t *testing.T) {
	bot := gotgbot.User{Id: 999, IsBot: true, FirstName: "Alita"}
	for _, tc := range pmModerationCases {
		t.Run(tc.cmd+"/to the bot", func(t *testing.T) {
			f, _ := pmModerationFixture(t)

			processPMReply(t, tc.dispatcher(), f, f.admin, bot, "/"+tc.cmd+" @fixturegroup")

			if tc.acted(t, f, f.chat.Id, 999) || len(f.client.callsFor("banChatMember")) != 0 || len(f.client.callsFor("unbanChatMember")) != 0 {
				t.Fatalf("/%s from PM in reply to the bot acted", tc.cmd)
			}
			if !repliedIn(f, f.admin.Id, translated(t, "common_no_user_specified")) {
				t.Fatalf("/%s from PM in reply to the bot: want common_no_user_specified; sent:\n%s", tc.cmd, sentText(f.client))
			}
		})
		t.Run(tc.cmd+"/to own message", func(t *testing.T) {
			f, _ := pmModerationFixture(t)

			processPMReply(t, tc.dispatcher(), f, f.admin, f.admin, "/"+tc.cmd+" @fixturegroup")

			if tc.acted(t, f, f.chat.Id, f.admin.Id) {
				t.Fatalf("/%s from PM in reply to the caller's own message acted on them", tc.cmd)
			}
			if !repliedIn(f, f.admin.Id, translated(t, "common_no_user_specified")) {
				t.Fatalf("/%s from PM in reply to own message: want common_no_user_specified; sent:\n%s", tc.cmd, sentText(f.client))
			}
		})
		t.Run(tc.cmd+"/with a user", func(t *testing.T) {
			f, _ := pmModerationFixture(t)

			processPMReply(t, tc.dispatcher(), f, f.admin, bot, "/"+tc.cmd+" @fixturegroup @victim"+tc.args)

			if !tc.acted(t, f, f.chat.Id, victimID) {
				t.Fatalf("/%s from PM in reply to the bot did not act on the user named; sent:\n%s", tc.cmd, sentText(f.client))
			}
		})
	}
}

// The help a command gives from PM shows a call with a group, both without a
// group and with the group named but no user: the group's call would act on
// the connected group, or get the help again without one. The help it gives
// in the group for a wrong call shows the group's call, as before.
func TestPMModerationHelpShowsPMExample(t *testing.T) {
	for _, tc := range pmModerationCases {
		t.Run(tc.cmd, func(t *testing.T) {
			f, _ := pmModerationFixture(t)
			useRealLocales(t)
			dispatcher := tc.dispatcher()
			pmExample, _ := i18n.MustNewTranslator("config").GetString(tc.cmd + "_pm_example")
			groupExample := commandExample(tc.cmd)
			if !strings.HasPrefix(pmExample, "/"+tc.cmd+" ") || pmExample == groupExample {
				t.Fatalf("%s_pm_example = %q, want a call of /%s with a group", tc.cmd, pmExample, tc.cmd)
			}
			for _, call := range []struct{ name, text, mustHave string }{
				{"without a user", "/" + tc.cmd + " @fixturegroup", translated(t, "common_no_user_specified")},
				{"without a group", "/" + tc.cmd, translated(t, "connections_pm_group_pick")},
			} {
				if call.name == "without a group" {
					// Connected, a call without a group acts on the connected group.
					if err := connections.DisconnectId(f.admin.Id); err != nil {
						t.Fatalf("DisconnectId() error = %v", err)
					}
				}
				before := len(f.client.callsFor("sendMessage"))
				processPM(t, dispatcher, f, f.admin.Id, call.text)
				replies := f.client.callsFor("sendMessage")[before:]
				if len(replies) != 1 {
					t.Fatalf("/%s from PM %s sent %d replies, want one; sent:\n%s", tc.cmd, call.name, len(replies), sentText(f.client))
				}
				inPM := fmt.Sprint(replies[0].Params["text"])
				if !strings.Contains(inPM, call.mustHave) || !strings.Contains(inPM, html.EscapeString(pmExample)) || strings.Contains(inPM, html.EscapeString(groupExample)) {
					t.Fatalf("/%s help from PM %s %q, want the example %q only", tc.cmd, call.name, inPM, pmExample)
				}
			}

			before := len(f.client.callsFor("sendMessage"))
			update := newModuleMessageContext(f.bot, f.chat, f.admin, "/"+tc.cmd).Update
			if err := dispatcher.ProcessUpdate(f.bot, update, nil); err != nil {
				t.Fatalf("ProcessUpdate(/%s) error = %v", tc.cmd, err)
			}
			inGroup := fmt.Sprint(f.client.callsFor("sendMessage")[before].Params["text"])
			if !strings.Contains(inGroup, html.EscapeString(groupExample)) || strings.Contains(inGroup, html.EscapeString(pmExample)) {
				t.Fatalf("/%s help in the group %q, want the example %q only", tc.cmd, inGroup, groupExample)
			}
		})
	}
}

// Every command sent from PM without a group and without a connection
// answers with its help, its example and the user's groups.
func TestPMModerationWithoutGroupRepliesHelp(t *testing.T) {
	f := titleFixture(t, "My Group")
	pick := translated(t, "connections_pm_group_pick")
	for _, tc := range pmModerationCases {
		t.Run(tc.cmd, func(t *testing.T) {
			before := len(f.client.callsFor("sendMessage"))

			processPM(t, tc.dispatcher(), f, f.admin.Id, "/"+tc.cmd)

			replies := f.client.callsFor("sendMessage")[before:]
			if len(replies) != 1 || !strings.Contains(fmt.Sprint(replies[0].Params["text"]), pick) {
				t.Fatalf("/%s from PM replied %+v, want one reply asking for the group", tc.cmd, replies)
			}
			assertPMGroupHelp(t, fmt.Sprint(replies[0].Params["text"]), tc.cmd, f.chat)
		})
	}
}
