//go:build testtools

package modules

import (
	"fmt"
	"html"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/i18n"
)

// textsTo returns the texts of the messages sent to chatID.
func (f logChannelPMFixture) textsTo(chatID int64) []string {
	var texts []string
	for _, call := range f.client.callsFor("sendMessage") {
		if fmt.Sprint(call.Params["chat_id"]) == fmt.Sprint(chatID) {
			texts = append(texts, fmt.Sprint(call.Params["text"]))
		}
	}
	return texts
}

// sendInGroup sends text to the fixture's group as user, through the dispatcher.
func (f logChannelPMFixture) sendInGroup(t *testing.T, user gotgbot.User, text string) {
	t.Helper()
	cmd := strings.Fields(text)[0]
	update := &gotgbot.Update{UpdateId: 1, Message: &gotgbot.Message{
		MessageId: 101, Date: 1, Chat: f.chat, From: &user, Text: text,
		Entities: []gotgbot.MessageEntity{{Type: "bot_command", Offset: 0, Length: int64(len(cmd))}},
	}}
	if err := logChannelsDispatcher().ProcessUpdate(f.bot, update, nil); err != nil {
		t.Fatalf("ProcessUpdate(%q) error = %v", text, err)
	}
}

// requireOnlyReply fails unless exactly one message went to chatID and it is want.
func (f logChannelPMFixture) requireOnlyReply(t *testing.T, chatID int64, want string) {
	t.Helper()
	got := f.textsTo(chatID)
	if len(got) != 1 || got[0] != want {
		t.Fatalf("messages to %d = %q, want exactly %q", chatID, got, want)
	}
}

// pmGroupLineText is the line a PM reply ends with to name the fixture's group.
func pmGroupLineText(t *testing.T, f logChannelPMFixture) string {
	t.Helper()
	line, _ := i18n.MustNewTranslator("en").GetString("connections_pm_group_confirm_target", i18n.TranslationParams{
		"title": html.EscapeString(f.chat.Title),
		"id":    f.chat.Id,
	})
	if line == "" {
		t.Fatal("locale key connections_pm_group_confirm_target is missing")
	}
	return "\n\n" + line
}

type logReplyCase struct {
	name   string
	bound  bool
	text   string // sent as typed, with the group (if any) already in it
	key    string
	params func(f logChannelPMFixture) i18n.TranslationParams
}

func logReplyCases() []logReplyCase {
	chanID := func(f logChannelPMFixture) i18n.TranslationParams {
		return i18n.TranslationParams{"id": fmt.Sprint(f.channel.Id)}
	}
	cats := func(f logChannelPMFixture) i18n.TranslationParams {
		return i18n.TranslationParams{"cats": "admin"}
	}
	return []logReplyCase{
		{"unsetlog", true, "/unsetlog%s", "logs_unset_ok", nil},
		{"logchannel", true, "/logchannel%s", "logs_current", chanID},
		{"log", true, "/log%s admin", "logs_enabled", cats},
		{"nolog", true, "/nolog%s admin", "logs_disabled", cats},
		{"unsetlog not set", false, "/unsetlog%s", "logs_not_set", nil},
		{"logchannel not set", false, "/logchannel%s", "logs_not_set", nil},
		{"log not set", false, "/log%s admin", "logs_not_set", nil},
		{"nolog not set", false, "/nolog%s admin", "logs_not_set", nil},
		{"log usage", true, "/log%s", "logs_usage", nil},
		{"nolog usage", true, "/nolog%s", "logs_usage", nil},
		{"log unknown category", true, "/log%s nonsense", "logs_unknown_category", nil},
		{"nolog unknown category", true, "/nolog%s nonsense", "logs_unknown_category", nil},
	}
}

func (c logReplyCase) want(t *testing.T, f logChannelPMFixture) string {
	if c.params == nil {
		return translated(t, c.key)
	}
	return translatedWith(t, c.key, c.params(f))
}

// A reply from PM names the group the command acted on, however it was named:
// by @username, by ID, or by the connection.
func TestLogChannelRepliesInPMNameTheGroup(t *testing.T) {
	for _, tc := range logReplyCases() {
		for _, via := range []string{"@fixturegroup", "id", "connection"} {
			t.Run(tc.name+"/"+via, func(t *testing.T) {
				f := newLogChannelPMFixture(t)
				if tc.bound {
					f.bindChannel(t)
				}
				group := ""
				switch via {
				case "@fixturegroup":
					group = " @fixturegroup"
				case "id":
					group = fmt.Sprintf(" %d", f.chat.Id)
				default:
					f.connectAdmin(t)
				}

				f.sendPM(t, f.admin, fmt.Sprintf(tc.text, group))

				f.requireOnlyReply(t, f.admin.Id, tc.want(t, f)+pmGroupLineText(t, f))
			})
		}
	}
}

// With a connection a quoted title is not looked up, so it is read as the
// command's own arguments and the command acts on the connected group: the
// reply has to say which group that is.
func TestNologInPMOnConnectionWithQuotedTitleNamesConnectedGroup(t *testing.T) {
	f := newLogChannelPMFixture(t)
	f.bindChannel(t)
	f.connectAdmin(t)

	f.sendPM(t, f.admin, `/nolog "Group B" admin`)

	f.requireOnlyReply(t, f.admin.Id, translatedWith(t, "logs_disabled", i18n.TranslationParams{"cats": "admin"})+pmGroupLineText(t, f))
}

// In the group the replies stay as they were: no group line.
func TestLogChannelRepliesInGroupHaveNoGroupLine(t *testing.T) {
	for _, tc := range logReplyCases() {
		t.Run(tc.name, func(t *testing.T) {
			f := newLogChannelPMFixture(t)
			if tc.bound {
				f.bindChannel(t)
			}

			f.sendInGroup(t, f.admin, fmt.Sprintf(tc.text, ""))

			f.requireOnlyReply(t, f.chat.Id, tc.want(t, f))
		})
	}
}
