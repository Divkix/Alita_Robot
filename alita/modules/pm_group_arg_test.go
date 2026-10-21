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
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
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

// iOS Smart Punctuation turns typed quotes into curly ones.
func TestParsePMGroupTokenAcceptsCurlyQuotes(t *testing.T) {
	tok, ok := parsePMGroupToken("/notes “My Group” x")
	if !ok || tok.Raw != "My Group" || !tok.Quoted {
		t.Fatalf("parsePMGroupToken() = %+v, %v; want quoted My Group", tok, ok)
	}
}

// “ and ” are three bytes in UTF-8 but one UTF-16 unit each.
func TestStripPMGroupTokenShiftsEntitiesPastCurlyQuotedTitle(t *testing.T) {
	msg := &gotgbot.Message{
		Text: "/save “My Group” note bold",
		Entities: []gotgbot.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: 5},
			{Type: "bold", Offset: 22, Length: 4},
		},
	}
	tok, ok := parsePMGroupToken(msg.GetText())
	if !ok || tok.Raw != "My Group" || !tok.Quoted {
		t.Fatalf("parsePMGroupToken() = %+v, %v; want quoted My Group", tok, ok)
	}

	got := stripPMGroupToken(msg, tok)

	if got.Text != "/save note bold" {
		t.Fatalf("text = %q, want %q", got.Text, "/save note bold")
	}
	if len(got.Entities) != 2 || extraction.ExtractEntityText(got.Text, got.Entities[1].Offset, got.Entities[1].Length) != "bold" {
		t.Fatalf("entities = %+v, want bold to keep its text", got.Entities)
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

// The /connect checks that admit a member to the named group already looked
// the member up, so the command does not ask Telegram again.
func TestPMGroupArgChecksMembershipOnce(t *testing.T) {
	f := oneOffFixture(t)
	f.client.membersByUser[fmt.Sprint(f.member.Id)] = json.RawMessage(fmt.Sprintf(
		`{"status":"member","user":{"id":%d,"is_bot":false,"first_name":"Member"}}`, f.member.Id))

	sendPMCommand(t, rulesDispatcher(), f.bot, f.client, f.member, "/rules @fixturegroup", "Be kind to each other")

	var lookups int
	for _, call := range callsToChat(f.client, "getChatMember", f.chat.Id) {
		if fmt.Sprint(call.Params["user_id"]) == fmt.Sprint(f.member.Id) {
			lookups++
		}
	}
	if lookups != 1 {
		t.Fatalf("getChatMember calls for the member in the group = %d, want 1", lookups)
	}
}

// sendRichPMRules sends /rules followed by args as a rich message in the
// admin's PM.
func sendRichPMRules(t *testing.T, f adminChatFixture, args string) {
	t.Helper()
	text := gotgbot.RichTextArray{gotgbot.RichTextBotCommand{Text: gotgbot.RichTextString("/rules"), BotCommand: "rules"}}
	if args != "" {
		text = append(text, gotgbot.RichTextString(" "+args))
	}
	update := &gotgbot.Update{UpdateId: 1, Message: &gotgbot.Message{
		MessageId: 101, Date: 1, Chat: privateChatOf(f.admin), From: &f.admin,
		RichMessage: &gotgbot.RichMessage{Blocks: []gotgbot.RichBlock{gotgbot.RichBlockParagraph{Text: text}}},
	}}
	if err := rulesDispatcher().ProcessUpdate(f.bot, update, nil); err != nil {
		t.Fatalf("ProcessUpdate() error = %v", err)
	}
}

// A rich message carries its text in blocks, not in Text and entities, so a
// group in it cannot be cut out. Rather than ignore the group and answer for
// the connected one, the bot asks for the group in a plain message.
func TestRulesInPMRichMessageWithArgumentAsksForGroup(t *testing.T) {
	f := oneOffFixture(t)
	f.connectAdmin(t)

	sendRichPMRules(t, f, fmt.Sprint(uniqueModuleChatID()))

	need := strings.TrimSpace(fmt.Sprintf(translated(t, "connections_pm_group_need"), "rules"))
	if got := sentText(f.client); !strings.Contains(got, need) || strings.Contains(got, "Be kind to each other") {
		t.Fatalf("rich /rules <group> from PM sent %q, want %q", got, need)
	}
}

// A rich message without arguments runs the command as it came.
func TestRulesInPMRichMessageWithoutArgumentRunsCommand(t *testing.T) {
	f := oneOffFixture(t)
	f.connectAdmin(t)

	sendRichPMRules(t, f, "")

	if got := sentText(f.client); !strings.Contains(got, "Be kind to each other") {
		t.Fatalf("rich /rules from PM sent %q, want the connected group's rules", got)
	}
}

// Entities that cross the cut keep their part outside it.
func TestStripPMGroupTokenClipsEntityAcrossCut(t *testing.T) {
	msg := &gotgbot.Message{
		Text: "/setrules -1001 Be kind",
		Entities: []gotgbot.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: 9},
			{Type: "bold", Offset: 10, Length: 13},
		},
	}
	tok, _ := parsePMGroupToken(msg.GetText())

	got := stripPMGroupToken(msg, tok)

	want := []gotgbot.MessageEntity{
		{Type: "bot_command", Offset: 0, Length: 9},
		{Type: "bold", Offset: 9, Length: 8},
	}
	if got.Text != "/setrules Be kind" || fmt.Sprint(got.Entities) != fmt.Sprint(want) {
		t.Fatalf("got %q %+v, want %q %+v", got.Text, got.Entities, "/setrules Be kind", want)
	}
}

// A quote ends a quoted group only before whitespace or the end of the text.
func TestParsePMGroupTokenQuoteInsideWord(t *testing.T) {
	msg := &gotgbot.Message{Text: `/x "a"b c`}
	tok, ok := parsePMGroupToken(msg.GetText())
	if !ok || tok.Quoted || tok.Raw != `"a"b` {
		t.Fatalf("parsePMGroupToken() = %+v, %v; want the bare token \"a\"b", tok, ok)
	}

	if got := stripPMGroupToken(msg, tok).Text; got != "/x c" {
		t.Fatalf("text = %q, want %q", got, "/x c")
	}
}

// The wrapped handler reads its own arguments, without the group.
func TestPMGroupArgHandlerSeesArgumentsWithoutGroup(t *testing.T) {
	f := oneOffFixture(t)
	ctx := newModuleMessageContext(f.bot, privateChatOf(f.admin), f.admin, fmt.Sprintf("/cmd %d on now", f.chat.Id))
	var args []string
	var text string
	handler := withPMGroupArg("cmd", helpers.PMGroupArgOwn, func(_ *gotgbot.Bot, ctx *ext.Context) error {
		args, text = ctx.Args(), ctx.EffectiveMessage.Text
		return ext.EndGroups
	})

	if err := handler(f.bot, ctx); err != ext.EndGroups {
		t.Fatalf("handler error = %v, want ext.EndGroups", err)
	}

	if fmt.Sprint(args) != "[/cmd on now]" || text != "/cmd on now" {
		t.Fatalf("handler saw args %q text %q, want [/cmd on now] and %q", args, text, "/cmd on now")
	}
}

// With chatAdmin the one-off group needs the user to be its admin, not only
// a member.
func TestIsUserConnectedOneOffGroupChatAdminRefusesMember(t *testing.T) {
	useEnglishLocale(t)
	f := newAdminChatFixture(t)
	ctx := newModuleMessageContext(f.bot, privateChatOf(f.member), f.member, "/setrules Be kind")
	chat_status.SetOneOffGroup(ctx, &f.chat)

	if got := chat_status.IsUserConnected(f.bot, ctx, true, false); got != nil {
		t.Fatalf("IsUserConnected(chatAdmin) = %+v, want a refusal for a member", got)
	}
	if notAdmin := translated(t, "connections_is_user_connected_user_not_admin"); !strings.Contains(sentText(f.client), notAdmin) {
		t.Fatalf("sent %q, want %q", sentText(f.client), notAdmin)
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
	f := oneOffFixture(t)
	left := gotgbot.User{Id: 13, FirstName: "Left User"}
	other := uniqueModuleChatID()
	if err := connections.ConnectId(left.Id, other); err != nil {
		t.Fatalf("ConnectId() error = %v", err)
	}
	t.Cleanup(func() { _ = connections.DisconnectId(left.Id) })

	refusal := translated(t, "connections_connect_not_member")
	sendPMCommand(t, rulesDispatcher(), f.bot, f.client, left, "/rules @fixturegroup", strings.TrimSpace(refusal))

	if strings.Contains(sentText(f.client), "Be kind to each other") {
		t.Fatalf("sent rules to a user who left: %s", sentText(f.client))
	}
	if conn := connections.Connection(left.Id); !conn.Connected || conn.ChatId != other {
		t.Fatalf("connection = %+v, want it kept on %d", conn, other)
	}
}
