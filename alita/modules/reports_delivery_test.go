package modules

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/logchannels"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

const reportCardChannelID int64 = -1009876543210

func callsToChat(client *moduleBotClient, method string, chatID int64) []moduleBotCall {
	var out []moduleBotCall
	for _, call := range client.callsFor(method) {
		if fmt.Sprint(call.Params["chat_id"]) == fmt.Sprint(chatID) {
			out = append(out, call)
		}
	}
	return out
}

func replyTargetID(t *testing.T, call moduleBotCall) string {
	t.Helper()
	raw, err := json.Marshal(call.Params["reply_parameters"])
	if err != nil {
		t.Fatalf("marshal reply_parameters: %v", err)
	}
	var params struct {
		MessageId json.Number `json:"message_id"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		t.Fatalf("decode reply_parameters %s: %v", raw, err)
	}
	return params.MessageId.String()
}

type reportCardFixture struct {
	client   *moduleBotClient
	bot      *gotgbot.Bot
	chat     gotgbot.Chat
	reporter gotgbot.User
	target   gotgbot.User
}

func newReportCardFixture(t *testing.T, withLogChannel bool) reportCardFixture {
	t.Helper()
	client := newModuleBotClient()
	client.responses["copyMessage"] = json.RawMessage(`{"message_id":900}`)
	f := reportCardFixture{
		client:   client,
		bot:      newModuleTestBot(client),
		chat:     gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Report <Chat>"},
		reporter: gotgbot.User{Id: 42, FirstName: "Reporter"},
		target:   gotgbot.User{Id: 43, FirstName: "Target"},
	}
	if withLogChannel {
		if err := logchannels.Set(f.chat.Id, f.chat.Title, reportCardChannelID); err != nil {
			t.Fatalf("logchannels.Set() error = %v", err)
		}
		t.Cleanup(func() { _ = logchannels.Unset(f.chat.Id) })
	}
	return f
}

func (f reportCardFixture) report(t *testing.T, reporter gotgbot.User, text string) {
	t.Helper()
	ctx := newReportReplyContext(f.bot, f.chat, reporter, f.target, text)
	if err := reportsModule.report(f.bot, ctx); err != ext.EndGroups {
		t.Fatalf("report error = %v, want EndGroups", err)
	}
}

func TestReportCardSentToLogChannel(t *testing.T) {
	f := newReportCardFixture(t, true)
	f.report(t, f.reporter, "/report spam <b>now</b> & later")

	copies := f.client.callsFor("copyMessage")
	if len(copies) != 1 {
		t.Fatalf("copyMessage calls = %d, want 1", len(copies))
	}
	copyParams := copies[0].Params
	if fmt.Sprint(copyParams["chat_id"]) != fmt.Sprint(reportCardChannelID) ||
		fmt.Sprint(copyParams["from_chat_id"]) != fmt.Sprint(f.chat.Id) ||
		fmt.Sprint(copyParams["message_id"]) != "505" {
		t.Fatalf("copyMessage params = %v, want reported message 505 copied from the group to the log channel", copyParams)
	}

	cards := callsToChat(f.client, "sendMessage", reportCardChannelID)
	if len(cards) != 1 {
		t.Fatalf("log channel sendMessage calls = %d, want only the report card", len(cards))
	}
	if got := replyTargetID(t, cards[0]); got != "900" {
		t.Fatalf("card replies to message %s, want the copy 900", got)
	}
	if !strings.Contains(fmt.Sprint(cards[0].Params["text"]), "Report &lt;Chat&gt;") {
		t.Fatalf("card text = %q, want the escaped group title", cards[0].Params["text"])
	}
	if !strings.Contains(fmt.Sprint(cards[0].Params["text"]), "Reason: spam &lt;b&gt;now&lt;/b&gt; &amp; later") {
		t.Fatalf("card text = %q, want the escaped reason", cards[0].Params["text"])
	}
	if cards[0].Params["reply_markup"] == nil {
		t.Fatal("card has no link to the reported message")
	}

	rows := storedReports(t, f.chat.Id)
	if len(rows) != 1 || rows[0].AdminChatId == nil || rows[0].AdminMessageId == nil {
		t.Fatalf("stored report card = %+v, want admin chat and message recorded", rows)
	}
	if *rows[0].AdminChatId != reportCardChannelID || *rows[0].AdminMessageId != 9001 {
		t.Fatalf("stored card = (%d, %d), want (%d, 9001)", *rows[0].AdminChatId, *rows[0].AdminMessageId, reportCardChannelID)
	}

	if group := callsToChat(f.client, "sendMessage", f.chat.Id); len(group) != 1 || group[0].Params["reply_markup"] == nil {
		t.Fatalf("group sendMessage calls = %d, want the unchanged in-group report", len(group))
	}
}

func TestReportCardFallbackWhenCopyFails(t *testing.T) {
	f := newReportCardFixture(t, true)
	f.client.errors["copyMessage"] = errors.New("Bad Request: message can't be copied")
	ctx := newReportReplyContext(f.bot, f.chat, f.reporter, f.target, "/report")
	ctx.EffectiveMessage.ReplyToMessage.Text = "buy <b>cheap</b> & now"
	if err := reportsModule.report(f.bot, ctx); err != ext.EndGroups {
		t.Fatalf("report error = %v, want EndGroups", err)
	}

	sent := callsToChat(f.client, "sendMessage", reportCardChannelID)
	if len(sent) != 2 {
		t.Fatalf("log channel sendMessage calls = %d, want excerpt and card", len(sent))
	}
	excerpt := fmt.Sprint(sent[0].Params["text"])
	if !strings.Contains(excerpt, "buy &lt;b&gt;cheap&lt;/b&gt; &amp; now") {
		t.Fatalf("excerpt text = %q, want the escaped reported text", excerpt)
	}
	if got := replyTargetID(t, sent[1]); got != "9001" {
		t.Fatalf("card replies to message %s, want the excerpt 9001", got)
	}
	if rows := storedReports(t, f.chat.Id); len(rows) != 1 || rows[0].AdminMessageId == nil {
		t.Fatalf("stored report = %+v, want the card recorded", rows)
	}
}

func TestSecondReporterEditsCard(t *testing.T) {
	f := newReportCardFixture(t, true)
	f.report(t, f.reporter, "/report spam")
	second := gotgbot.User{Id: 44, FirstName: "Second"}
	f.report(t, second, "/report scam")

	if got := len(f.client.callsFor("copyMessage")); got != 1 {
		t.Fatalf("copyMessage calls = %d, want 1 for one reported message", got)
	}
	if got := len(callsToChat(f.client, "sendMessage", reportCardChannelID)); got != 1 {
		t.Fatalf("log channel sendMessage calls = %d, want 1 card", got)
	}
	edits := callsToChat(f.client, "editMessageText", reportCardChannelID)
	if len(edits) != 1 || fmt.Sprint(edits[0].Params["message_id"]) != "9001" {
		t.Fatalf("card edits = %v, want one edit of card 9001", edits)
	}
	if edits[0].Params["reply_markup"] == nil {
		t.Fatal("edited card lost its link button")
	}
	rows := storedReports(t, f.chat.Id)
	if len(rows) != 2 {
		t.Fatalf("stored reports = %d, want 2", len(rows))
	}
	for _, row := range rows {
		if row.AdminMessageId == nil || *row.AdminMessageId != 9001 {
			t.Fatalf("row %d card = %v, want 9001 on every open report", row.ReporterId, row.AdminMessageId)
		}
	}
}

func TestRepeatReportDoesNotTouchCard(t *testing.T) {
	f := newReportCardFixture(t, true)
	f.report(t, f.reporter, "/report spam")
	f.report(t, f.reporter, "/report spam again")

	if got := len(f.client.callsFor("copyMessage")); got != 1 {
		t.Fatalf("copyMessage calls = %d, want 1", got)
	}
	if got := len(callsToChat(f.client, "sendMessage", reportCardChannelID)); got != 1 {
		t.Fatalf("log channel sendMessage calls = %d, want 1", got)
	}
	if got := len(callsToChat(f.client, "editMessageText", reportCardChannelID)); got != 0 {
		t.Fatalf("card edits = %d, want 0 for a repeated report", got)
	}
}

func editMessageTextError(description string) error {
	return &gotgbot.TelegramError{Method: "editMessageText", Code: 400, Description: description}
}

func TestCardRecreatedWhenEditFails(t *testing.T) {
	f := newReportCardFixture(t, true)
	f.report(t, f.reporter, "/report spam")
	f.client.errors["editMessageText"] = editMessageTextError("Bad Request: message to edit not found")
	f.report(t, gotgbot.User{Id: 44, FirstName: "Second"}, "/report")

	if got := len(f.client.callsFor("copyMessage")); got != 2 {
		t.Fatalf("copyMessage calls = %d, want a fresh copy for the new card", got)
	}
	if got := len(callsToChat(f.client, "sendMessage", reportCardChannelID)); got != 2 {
		t.Fatalf("log channel sendMessage calls = %d, want a replacement card", got)
	}
}

func TestCardKeptWhenEditFailsTransiently(t *testing.T) {
	transient := map[string]error{
		"flood wait": &gotgbot.TelegramError{
			Method: "editMessageText", Code: 429, Description: "Too Many Requests: retry after 5",
		},
		"network": errors.New("Post \"https://api.telegram.org/bot/editMessageText\": context deadline exceeded"),
	}
	for name, editErr := range transient {
		t.Run(name, func(t *testing.T) {
			f := newReportCardFixture(t, true)
			f.report(t, f.reporter, "/report spam")
			f.client.errors["editMessageText"] = editErr
			f.report(t, gotgbot.User{Id: 44, FirstName: "Second"}, "/report")

			if got := len(f.client.callsFor("copyMessage")); got != 1 {
				t.Fatalf("copyMessage calls = %d, want no second copy while the card still exists", got)
			}
			if got := len(callsToChat(f.client, "sendMessage", reportCardChannelID)); got != 1 {
				t.Fatalf("log channel sendMessage calls = %d, want no duplicate card", got)
			}
			rows := storedReports(t, f.chat.Id)
			if len(rows) != 2 {
				t.Fatalf("stored reports = %d, want 2", len(rows))
			}
			for _, row := range rows {
				if row.AdminMessageId == nil || *row.AdminMessageId != 9001 {
					t.Fatalf("row %d card = %v, want the existing card 9001", row.ReporterId, row.AdminMessageId)
				}
			}
		})
	}
}

func TestReportCardSentWhenStoreFails(t *testing.T) {
	f := newReportCardFixture(t, true)
	if err := db.DB.Migrator().DropTable(&models.Report{}); err != nil {
		t.Fatalf("drop reports table: %v", err)
	}
	t.Cleanup(func() {
		if err := db.DB.AutoMigrate(&models.Report{}); err != nil {
			t.Fatalf("restore reports table: %v", err)
		}
	})

	f.report(t, f.reporter, "/report spam")

	if got := len(f.client.callsFor("copyMessage")); got != 1 {
		t.Fatalf("copyMessage calls = %d, want the reported message copied", got)
	}
	cards := callsToChat(f.client, "sendMessage", reportCardChannelID)
	if len(cards) != 1 || !strings.Contains(fmt.Sprint(cards[0].Params["text"]), "#REPORT") {
		t.Fatalf("log channel sendMessage calls = %v, want the report card despite the store failure", cards)
	}
}

func TestNoLogChannelKeepsGroupBehaviour(t *testing.T) {
	f := newReportCardFixture(t, false)
	f.report(t, f.reporter, "/report spam")

	if got := len(f.client.callsFor("copyMessage")); got != 0 {
		t.Fatalf("copyMessage calls = %d, want 0 without a log channel", got)
	}
	sent := f.client.callsFor("sendMessage")
	if len(sent) != 1 || fmt.Sprint(sent[0].Params["chat_id"]) != fmt.Sprint(f.chat.Id) {
		t.Fatalf("sendMessage calls = %v, want only the in-group report", sent)
	}
	if rows := storedReports(t, f.chat.Id); len(rows) != 1 || rows[0].AdminMessageId != nil {
		t.Fatalf("stored report = %+v, want a row without a card", rows)
	}
}

func TestReportsCategoryDisabledSkipsCard(t *testing.T) {
	f := newReportCardFixture(t, true)
	if err := logchannels.SetCategory(f.chat.Id, logchannels.CategoryReports, false); err != nil {
		t.Fatalf("SetCategory() error = %v", err)
	}
	f.report(t, f.reporter, "/report spam")

	if got := len(f.client.callsFor("copyMessage")); got != 0 {
		t.Fatalf("copyMessage calls = %d, want 0 with the reports category off", got)
	}
	if got := len(callsToChat(f.client, "sendMessage", reportCardChannelID)); got != 0 {
		t.Fatalf("log channel sendMessage calls = %d, want 0", got)
	}
}
