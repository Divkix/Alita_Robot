//go:build testtools

package modules

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/rules"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
)

func TestStripPMGroupTokenShiftsEntities(t *testing.T) {
	msg := &gotgbot.Message{
		Text: "/save @grp note bold",
		Entities: []gotgbot.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: 5},
			{Type: "mention", Offset: 6, Length: 4},
			{Type: "bold", Offset: 16, Length: 4},
		},
	}
	tok, ok := parsePMGroupToken(msg.GetText())
	if !ok || tok.Raw != "@grp" {
		t.Fatalf("parsePMGroupToken() = %+v, %v; want @grp", tok, ok)
	}

	got := stripPMGroupToken(msg, tok)

	if got.Text != "/save note bold" {
		t.Fatalf("text = %q, want %q", got.Text, "/save note bold")
	}
	want := []gotgbot.MessageEntity{
		{Type: "bot_command", Offset: 0, Length: 5},
		{Type: "bold", Offset: 11, Length: 4},
	}
	if fmt.Sprint(got.Entities) != fmt.Sprint(want) {
		t.Fatalf("entities = %+v, want %+v", got.Entities, want)
	}
	if msg.Text != "/save @grp note bold" || len(msg.Entities) != 3 {
		t.Fatalf("original message changed: %q %+v", msg.Text, msg.Entities)
	}
}

// Offsets are UTF-16 code units: the emoji in the quoted title counts as two.
func TestStripPMGroupTokenShiftsEntitiesPastQuotedEmojiTitle(t *testing.T) {
	msg := &gotgbot.Message{
		Text: "/save \"👍 grp\" note bold",
		Entities: []gotgbot.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: 5},
			{Type: "bold", Offset: 20, Length: 4},
		},
	}
	tok, ok := parsePMGroupToken(msg.GetText())
	if !ok || tok.Raw != "👍 grp" || !tok.Quoted {
		t.Fatalf("parsePMGroupToken() = %+v, %v; want quoted 👍 grp", tok, ok)
	}

	got := stripPMGroupToken(msg, tok)

	if got.Text != "/save note bold" {
		t.Fatalf("text = %q, want %q", got.Text, "/save note bold")
	}
	if len(got.Entities) != 2 || got.Entities[1].Offset != 11 || got.Entities[1].Length != 4 {
		t.Fatalf("entities = %+v, want bold at 11 length 4", got.Entities)
	}
}

func TestStripPMGroupTokenCutsCaption(t *testing.T) {
	msg := &gotgbot.Message{
		Caption: "/save -1001 note bold",
		CaptionEntities: []gotgbot.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: 5},
			{Type: "bold", Offset: 17, Length: 4},
		},
	}
	tok, ok := parsePMGroupToken(msg.GetText())
	if !ok {
		t.Fatal("parsePMGroupToken() found no token in the caption")
	}

	got := stripPMGroupToken(msg, tok)

	if got.Caption != "/save note bold" || got.Text != "" {
		t.Fatalf("caption = %q text = %q, want the caption cut", got.Caption, got.Text)
	}
	if len(got.CaptionEntities) != 2 || got.CaptionEntities[1].Offset != 11 {
		t.Fatalf("caption entities = %+v, want bold at 11", got.CaptionEntities)
	}
}

// The line break after the group stays, so text that starts on the next line
// keeps its layout.
func TestStripPMGroupTokenKeepsLineBreakAfterGroup(t *testing.T) {
	msg := &gotgbot.Message{Text: "/setrules @grp\nBe kind"}
	tok, _ := parsePMGroupToken(msg.GetText())

	if got := stripPMGroupToken(msg, tok).Text; got != "/setrules\nBe kind" {
		t.Fatalf("text = %q, want %q", got, "/setrules\nBe kind")
	}
}

func TestParsePMGroupTokenWithoutArgument(t *testing.T) {
	if tok, ok := parsePMGroupToken("/rules"); ok {
		t.Fatalf("parsePMGroupToken(/rules) = %+v, want none", tok)
	}
}

// oneOffFixture is adminChatFixture's group reachable by @username, with rules.
func oneOffFixture(t *testing.T) adminChatFixture {
	t.Helper()
	useEnglishLocale(t)
	f := newAdminChatFixture(t)
	f.client.chatsByID["@fixturegroup"] = f.client.chatsByID[fmt.Sprint(f.chat.Id)]
	if err := rules.SetChatRules(f.chat.Id, "Be kind to each other"); err != nil {
		t.Fatalf("SetChatRules() error = %v", err)
	}
	return f
}

func rulesDispatcher() *ext.Dispatcher {
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadRules(dispatcher)
	return dispatcher
}

func sentText(client *moduleBotClient) string {
	return strings.Join(sentTexts(client), "\n---\n")
}

func TestRulesInPMWithGroupID(t *testing.T) {
	f := oneOffFixture(t)

	sendPMCommand(t, rulesDispatcher(), f.bot, f.client, f.admin, fmt.Sprintf("/rules %d", f.chat.Id), "Be kind to each other")
}

func TestRulesInPMWithGroupWinsOverConnection(t *testing.T) {
	f := oneOffFixture(t)
	other := uniqueModuleChatID()
	if err := rules.SetChatRules(other, "Rules of the connected group"); err != nil {
		t.Fatalf("SetChatRules() error = %v", err)
	}
	if err := connections.ConnectId(f.admin.Id, other); err != nil {
		t.Fatalf("ConnectId() error = %v", err)
	}
	t.Cleanup(func() { _ = connections.DisconnectId(f.admin.Id) })

	sendPMCommand(t, rulesDispatcher(), f.bot, f.client, f.admin, "/rules @fixturegroup", "Be kind to each other")

	if strings.Contains(sentText(f.client), "Rules of the connected group") {
		t.Fatalf("sent the connected group's rules: %s", sentText(f.client))
	}
	if conn := connections.Connection(f.admin.Id); !conn.Connected || conn.ChatId != other {
		t.Fatalf("connection = %+v, want it kept on %d", conn, other)
	}
}

// A user's @username is not a group: with no connection the first argument
// has to name one, so the bot says it found no group instead of guessing.
func TestRulesInPMWithUserMentionIsNotGroup(t *testing.T) {
	f := oneOffFixture(t)
	f.client.chatsByID["@someuser"] = json.RawMessage(`{"id":5,"type":"private","first_name":"Some"}`)

	notFound := fmt.Sprintf(translated(t, "connections_pm_group_not_found"), "@someuser")
	sendPMCommand(t, rulesDispatcher(), f.bot, f.client, f.admin, "/rules @someuser", strings.TrimSpace(notFound))

	if strings.Contains(sentText(f.client), "Be kind to each other") {
		t.Fatalf("sent rules for a user mention: %s", sentText(f.client))
	}
}

// /rules takes no arguments, so with a connection an argument that names no
// group is a mistake, not a request for the connected group's rules.
func TestRulesInPMUnknownTokenWithConnection(t *testing.T) {
	f := oneOffFixture(t)
	f.connectAdmin(t)

	notFound := fmt.Sprintf(translated(t, "connections_pm_group_not_found"), "typo")
	sendPMCommand(t, rulesDispatcher(), f.bot, f.client, f.admin, "/rules typo", strings.TrimSpace(notFound))

	if strings.Contains(sentText(f.client), "Be kind to each other") {
		t.Fatalf("sent the connected group's rules for an unknown group: %s", sentText(f.client))
	}
}

// A member who is not an admin may use a group from PM only while
// /allowconnect is on, the same as /connect.
func TestRulesInPMAllowConnectOffRefusesMember(t *testing.T) {
	f := oneOffFixture(t)
	if err := connections.ToggleAllowConnect(f.chat.Id, false); err != nil {
		t.Fatalf("ToggleAllowConnect() error = %v", err)
	}
	refusal := translated(t, "connections_connect_connection_disabled")

	sendPMCommand(t, rulesDispatcher(), f.bot, f.client, f.member, fmt.Sprintf("/rules %d", f.chat.Id), strings.TrimSpace(refusal))

	if strings.Contains(sentText(f.client), "Be kind to each other") {
		t.Fatalf("sent rules to a member while /allowconnect is off: %s", sentText(f.client))
	}
}

// Without a connection a PM command has no group to act on, so the bot asks
// for one as the first argument.
func TestRulesInPMWithoutGroupOrConnectionAsksForGroup(t *testing.T) {
	f := oneOffFixture(t)
	need := fmt.Sprintf(translated(t, "connections_pm_group_need"), "rules")

	sendPMCommand(t, rulesDispatcher(), f.bot, f.client, f.admin, "/rules", strings.TrimSpace(need))
}

// A rich message carries its text in blocks, not in Text and entities, so
// there is nothing to cut: the command runs on the message as it came.
func TestRulesInPMRichMessageIsLeftAlone(t *testing.T) {
	f := oneOffFixture(t)
	update := &gotgbot.Update{UpdateId: 1, Message: &gotgbot.Message{
		MessageId: 101, Date: 1, Chat: privateChatOf(f.admin), From: &f.admin,
		RichMessage: &gotgbot.RichMessage{Blocks: []gotgbot.RichBlock{
			gotgbot.RichBlockParagraph{Text: gotgbot.RichTextArray{
				gotgbot.RichTextBotCommand{Text: gotgbot.RichTextString("/rules"), BotCommand: "rules"},
				gotgbot.RichTextString(fmt.Sprintf(" %d", f.chat.Id)),
			}},
		}},
	}}

	if err := rulesDispatcher().ProcessUpdate(f.bot, update, nil); err != nil {
		t.Fatalf("ProcessUpdate() error = %v", err)
	}

	needGroup := strings.TrimSpace(translated(t, "connections_is_user_connected_need_group"))
	if got := sentText(f.client); !strings.Contains(got, needGroup) || strings.Contains(got, "Be kind to each other") {
		t.Fatalf("rich /rules from PM sent %q, want today's %q", got, needGroup)
	}
}

// In the group the argument is ignored, as before.
func TestRulesInGroupIgnoresArgument(t *testing.T) {
	f := oneOffFixture(t)
	update := &gotgbot.Update{UpdateId: 1, Message: &gotgbot.Message{
		MessageId: 101, Date: 1, Chat: f.chat, From: &f.member, Text: "/rules @someothergroup",
	}}

	if err := rulesDispatcher().ProcessUpdate(f.bot, update, nil); err != nil {
		t.Fatalf("ProcessUpdate() error = %v", err)
	}

	if !strings.Contains(sentText(f.client), "Be kind to each other") {
		t.Fatalf("/rules in the group did not show its rules: %s", sentText(f.client))
	}
}

func TestIsUserConnectedPrefersOneOffGroup(t *testing.T) {
	f := newAdminChatFixture(t)
	other := uniqueModuleChatID()
	if err := connections.ConnectId(f.member.Id, other); err != nil {
		t.Fatalf("ConnectId() error = %v", err)
	}
	t.Cleanup(func() { _ = connections.DisconnectId(f.member.Id) })
	ctx := newModuleMessageContext(f.bot, privateChatOf(f.member), f.member, "/rules")
	chat_status.SetOneOffGroup(ctx, &f.chat)

	got := chat_status.IsUserConnected(f.bot, ctx, false, false)

	if got == nil || got.Id != f.chat.Id {
		t.Fatalf("IsUserConnected() = %+v, want the one-off group %d", got, f.chat.Id)
	}
}

// Failing a check on the one-off group leaves the saved connection alone:
// the user asked about another group, not about the connected one.
func TestOneOffGroupRefusalKeepsConnection(t *testing.T) {
	f := newAdminChatFixture(t)
	left := gotgbot.User{Id: 13, FirstName: "Left User"}
	other := uniqueModuleChatID()
	if err := connections.ConnectId(left.Id, other); err != nil {
		t.Fatalf("ConnectId() error = %v", err)
	}
	t.Cleanup(func() { _ = connections.DisconnectId(left.Id) })
	ctx := newModuleMessageContext(f.bot, privateChatOf(left), left, "/rules")
	chat_status.SetOneOffGroup(ctx, &f.chat)

	if got := chat_status.IsUserConnected(f.bot, ctx, false, false); got != nil {
		t.Fatalf("IsUserConnected() = %+v, want a refusal for a user who left", got)
	}
	if conn := connections.Connection(left.Id); !conn.Connected || conn.ChatId != other {
		t.Fatalf("connection = %+v, want it kept on %d", conn, other)
	}
}
