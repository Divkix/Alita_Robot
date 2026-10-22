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
// chatID: restricted them, offered the restriction menu for them, or changed
// their warns. A command needs the right to restrict members, from the caller
// and the bot, unless adminOnly: then both need only be admins of the group.
// protectKey is the refusal for a target protected in the group, "" for a
// command that protects none (/unban). prepare, when set, readies @victim in
// the fixture's group and the connected one before the command. Called in the
// group without a user, a command answers with its help, or, when
// noHelpInGroup, with the no-user error alone, as before PM support.
type pmModerationCase struct {
	cmd, args     string
	dispatcher    func() *ext.Dispatcher
	acted         func(t *testing.T, f adminChatFixture, chatID, userID int64) bool
	protectKey    string
	adminOnly     bool
	noHelpInGroup bool
	prepare       func(t *testing.T, f adminChatFixture, connected int64)
}

var pmModerationCases = []pmModerationCase{
	{cmd: "ban", args: " spam", dispatcher: bansPMDispatcher, acted: calledOn("banChatMember"), protectKey: "bans_ban_is_admin"},
	{cmd: "sban", dispatcher: bansPMDispatcher, acted: calledOn("banChatMember"), protectKey: "bans_ban_is_admin"},
	{cmd: "tban", args: " 2h", dispatcher: bansPMDispatcher, acted: calledOn("banChatMember"), protectKey: "bans_ban_is_admin"},
	{cmd: "unban", dispatcher: bansPMDispatcher, acted: calledOn("unbanChatMember")},
	{cmd: "kick", dispatcher: bansPMDispatcher, acted: calledOn("unbanChatMember"), protectKey: "bans_kick_cannot_kick_admin"},
	{cmd: "skick", dispatcher: bansPMDispatcher, acted: calledOn("unbanChatMember"), protectKey: "bans_kick_cannot_kick_admin"},
	{cmd: "restrict", dispatcher: bansPMDispatcher, acted: menuFor("restrict"), protectKey: "bans_restrict_admin_error"},
	{cmd: "unrestrict", dispatcher: bansPMDispatcher, acted: menuFor("unrestrict"), protectKey: "bans_unrestrict_admin_error"},
	{cmd: "mute", args: " spam", dispatcher: mutesPMDispatcher, acted: calledOn("restrictChatMember"), protectKey: "mutes_mute_admin_error"},
	{cmd: "smute", dispatcher: mutesPMDispatcher, acted: calledOn("restrictChatMember"), protectKey: "mutes_mute_admin_error"},
	{cmd: "tmute", args: " 2h", dispatcher: mutesPMDispatcher, acted: calledOn("restrictChatMember"), protectKey: "mutes_mute_admin_error"},
	{cmd: "unmute", dispatcher: mutesPMDispatcher, acted: calledOn("restrictChatMember")},
	{cmd: "warn", args: " spam", dispatcher: warnsPMDispatcher, acted: warnedNow, protectKey: "warns_admin_warning_error", noHelpInGroup: true, prepare: victimWarns(0)},
	{cmd: "swarn", dispatcher: warnsPMDispatcher, acted: warnedNow, protectKey: "warns_admin_warning_error", noHelpInGroup: true, prepare: victimWarns(0)},
	{cmd: "rmwarn", dispatcher: warnsPMDispatcher, acted: seedWarnsLeft(pmSeedWarns - 1), adminOnly: true, noHelpInGroup: true, prepare: victimWarns(pmSeedWarns)},
	{cmd: "resetwarns", dispatcher: warnsPMDispatcher, acted: seedWarnsLeft(0), adminOnly: true, noHelpInGroup: true, prepare: victimWarns(pmSeedWarns)},
}

// fixture is pmModerationFixture readied for the command.
func (tc pmModerationCase) fixture(t *testing.T) (f adminChatFixture, connected int64) {
	t.Helper()
	f, connected = pmModerationFixture(t)
	if tc.prepare != nil {
		tc.prepare(t, f, connected)
	}
	return f, connected
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

// restrictedAnyone reports whether anyone was banned, unbanned or restricted.
func restrictedAnyone(f adminChatFixture) bool {
	return len(f.client.callsFor("banChatMember"))+len(f.client.callsFor("unbanChatMember"))+len(f.client.callsFor("restrictChatMember")) != 0
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
			f, connected := tc.fixture(t)

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

// Sent from PM, the reply of each command names the group it acted on, as
// HTML: the one given first, or the connected one. Sent in the group, it does
// not.
func TestPMModerationNamesGroupInReply(t *testing.T) {
	for _, tc := range pmModerationCases {
		t.Run(tc.cmd, func(t *testing.T) {
			f, connected := tc.fixture(t)
			// A title with HTML characters: the reply is HTML, so it must be escaped.
			named := `Tom & <Jerry>`
			for _, key := range []string{fmt.Sprint(f.chat.Id), "@fixturegroup"} {
				f.client.chatsByID[key] = json.RawMessage(fmt.Sprintf(`{"id":%d,"type":"supergroup","title":%q}`, f.chat.Id, named))
			}
			dispatcher := tc.dispatcher()

			processPM(t, dispatcher, f, f.admin.Id, "/"+tc.cmd+" @fixturegroup @victim"+tc.args)
			if want := groupLine(t, named, f.chat.Id); !repliedInHTML(f, f.admin.Id, want) {
				t.Fatalf("/%s @fixturegroup from PM did not name the group %q as HTML; sent:\n%s", tc.cmd, want, sentText(f.client))
			}

			processPM(t, dispatcher, f, f.admin.Id, "/"+tc.cmd+" @victim"+tc.args)
			if want := groupLine(t, chatTitleOf(f, connected), connected); !repliedInHTML(f, f.admin.Id, want) {
				t.Fatalf("/%s from PM on a connection did not name the connected group %q; sent:\n%s", tc.cmd, want, sentText(f.client))
			}

			before := len(f.client.callsFor("sendMessage"))
			update := newModuleMessageContext(f.bot, f.chat, f.admin, "/"+tc.cmd+" @victim"+tc.args).Update
			if err := dispatcher.ProcessUpdate(f.bot, update, nil); err != nil {
				t.Fatalf("ProcessUpdate(/%s) error = %v", tc.cmd, err)
			}
			prefix := strings.SplitN(translated(t, "connections_pm_group_confirm_target"), "{", 2)[0]
			for _, call := range f.client.callsFor("sendMessage")[before:] {
				if strings.Contains(fmt.Sprint(call.Params["text"]), prefix) {
					t.Fatalf("/%s in the group named a group: %q", tc.cmd, call.Params["text"])
				}
			}
		})
	}
}

// repliedInHTML reports whether a message containing text was sent to chatID
// as HTML.
func repliedInHTML(f adminChatFixture, chatID int64, text string) bool {
	for _, call := range f.client.callsFor("sendMessage") {
		if fmt.Sprint(call.Params["chat_id"]) == fmt.Sprint(chatID) && strings.Contains(fmt.Sprint(call.Params["text"]), text) &&
			fmt.Sprint(call.Params["parse_mode"]) == gotgbot.ParseModeHTML {
			return true
		}
	}
	return false
}

// With a connection and no group named, each command acts on @victim in the
// connected group, which before took no command from PM.
func TestPMModerationOnConnection(t *testing.T) {
	for _, tc := range pmModerationCases {
		t.Run(tc.cmd, func(t *testing.T) {
			f, connected := tc.fixture(t)

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
			f, connected := tc.fixture(t)
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
		if tc.adminOnly {
			// No right to lack: an admin of the group may run it.
			continue
		}
		t.Run(tc.cmd, func(t *testing.T) {
			f, connected := tc.fixture(t)
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

// From PM, a command the bot cannot carry out in the named group, where it is
// no admin or, for a command that needs the right to restrict, an admin
// without it, gets the answer it gets in that group, in the PM.
func TestPMModerationRefusesWhenBotLacksRight(t *testing.T) {
	for _, tc := range pmModerationCases {
		for _, botRole := range []struct {
			name  string
			admin bool
		}{{"no admin", false}, {"admin without the right", true}} {
			if botRole.admin && tc.adminOnly {
				continue
			}
			t.Run(tc.cmd+"/"+botRole.name, func(t *testing.T) {
				f, connected := tc.fixture(t)
				setup := func() {
					admins := []gotgbot.MergedChatMember{adminMember(f.admin.Id, false)}
					member := `{"status":"member","user":{"id":999,"is_bot":true,"first_name":"Alita"}}`
					if botRole.admin {
						bot := adminMember(999, true)
						bot.CanRestrictMembers = false
						admins = append(admins, bot)
						member = `{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"},"can_delete_messages":true}`
					}
					seedAdminCache(t, f.chat.Id, admins...)
					f.client.membersByChat[fmt.Sprintf("%d|999", f.chat.Id)] = json.RawMessage(member)
				}
				dispatcher := tc.dispatcher()

				setup()
				update := newModuleMessageContext(f.bot, f.chat, f.admin, "/"+tc.cmd+" @victim"+tc.args).Update
				if err := dispatcher.ProcessUpdate(f.bot, update, nil); err != nil {
					t.Fatalf("ProcessUpdate(/%s) error = %v", tc.cmd, err)
				}
				inGroup := f.client.callsFor("sendMessage")
				if len(inGroup) != 1 || tc.acted(t, f, f.chat.Id, victimID) {
					t.Fatalf("/%s in the group with the bot %s: want one refusal; sent:\n%s", tc.cmd, botRole.name, sentText(f.client))
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
					t.Fatalf("/%s from PM acted though the bot is %s of the named group", tc.cmd, botRole.name)
				}
			})
		}
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
				f, connected := tc.fixture(t)
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
			f, _ := tc.fixture(t)

			processPMReply(t, tc.dispatcher(), f, f.admin, bot, "/"+tc.cmd+" @fixturegroup")

			if tc.acted(t, f, f.chat.Id, 999) || restrictedAnyone(f) {
				t.Fatalf("/%s from PM in reply to the bot acted", tc.cmd)
			}
			if !repliedIn(f, f.admin.Id, translated(t, "common_no_user_specified")) {
				t.Fatalf("/%s from PM in reply to the bot: want common_no_user_specified; sent:\n%s", tc.cmd, sentText(f.client))
			}
		})
		t.Run(tc.cmd+"/to own message", func(t *testing.T) {
			f, _ := tc.fixture(t)

			processPMReply(t, tc.dispatcher(), f, f.admin, f.admin, "/"+tc.cmd+" @fixturegroup")

			if tc.acted(t, f, f.chat.Id, f.admin.Id) || restrictedAnyone(f) {
				t.Fatalf("/%s from PM in reply to the caller's own message acted", tc.cmd)
			}
			if !repliedIn(f, f.admin.Id, translated(t, "common_no_user_specified")) {
				t.Fatalf("/%s from PM in reply to own message: want common_no_user_specified; sent:\n%s", tc.cmd, sentText(f.client))
			}
		})
		t.Run(tc.cmd+"/with a user", func(t *testing.T) {
			f, _ := tc.fixture(t)

			processPMReply(t, tc.dispatcher(), f, f.admin, bot, "/"+tc.cmd+" @fixturegroup @victim"+tc.args)

			if !tc.acted(t, f, f.chat.Id, victimID) {
				t.Fatalf("/%s from PM in reply to the bot did not act on the user named; sent:\n%s", tc.cmd, sentText(f.client))
			}
		})
	}
}

// The help a command gives from PM shows a call with a group, both without a
// group and with the group named but no user: the group's call would act on
// the connected group, or get the help again without one. In the group a call
// without a user gets what it got before: the help with the group's call, or
// the no-user error alone.
func TestPMModerationHelpShowsPMExample(t *testing.T) {
	for _, tc := range pmModerationCases {
		t.Run(tc.cmd, func(t *testing.T) {
			f, _ := tc.fixture(t)
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
			if tc.noHelpInGroup {
				if want := translated(t, "common_no_user_specified"); inGroup != want {
					t.Fatalf("/%s in the group without a user replied %q, want %q alone", tc.cmd, inGroup, want)
				}
				return
			}
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
