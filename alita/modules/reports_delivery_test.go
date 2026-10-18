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
	"github.com/divkix/Alita_Robot/alita/db/reports"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

const (
	reportCardChannelID int64 = -1009876543210
	reportCardAdminID   int64 = 50
)

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

// legacyReportActions lists the actions of the in-group report buttons.
func legacyReportActions(t *testing.T, markup any) map[string]string {
	t.Helper()
	return markupActions(t, markup, "report")
}

// markupActions returns the callback data of a keyboard's buttons in
// namespace, by their "a" field.
func markupActions(t *testing.T, markup any, namespace string) map[string]string {
	t.Helper()
	raw, err := json.Marshal(markup)
	if err != nil {
		t.Fatalf("marshal reply_markup: %v", err)
	}
	var keyboard gotgbot.InlineKeyboardMarkup
	if err := json.Unmarshal(raw, &keyboard); err != nil {
		t.Fatalf("decode reply_markup %s: %v", raw, err)
	}
	actions := map[string]string{}
	for _, row := range keyboard.InlineKeyboard {
		for _, button := range row {
			decoded, ok := decodeCallbackData(button.CallbackData, namespace)
			if !ok {
				continue
			}
			action, _ := decoded.Field("a")
			actions[action] = button.CallbackData
		}
	}
	return actions
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

	assertShortGroupReply(t, f)
}

// assertShortGroupReply checks that every in-group reply is the short notice
// that the card reached the admins, with no admin mentions or buttons.
func assertShortGroupReply(t *testing.T, f reportCardFixture) {
	t.Helper()
	want, _ := i18n.MustNewTranslator("en").GetString("reports_sent_to_admins")
	group := callsToChat(f.client, "sendMessage", f.chat.Id)
	if len(group) == 0 {
		t.Fatal("group sendMessage calls = 0, want the short in-group reply")
	}
	for _, call := range group {
		if call.Params["reply_markup"] != nil || fmt.Sprint(call.Params["text"]) != want {
			t.Fatalf("group reply = %v, want only %q without buttons", call.Params, want)
		}
	}
}

// assertFullGroupReply checks that the in-group reply is the report with its
// action buttons, as without a log channel.
func assertFullGroupReply(t *testing.T, f reportCardFixture) {
	t.Helper()
	group := callsToChat(f.client, "sendMessage", f.chat.Id)
	if len(group) != 1 {
		t.Fatalf("group sendMessage calls = %d, want the full in-group report", len(group))
	}
	if actions := legacyReportActions(t, group[0].Params["reply_markup"]); len(actions) == 0 {
		t.Fatalf("group reply = %v, want the in-group action buttons", group[0].Params)
	}
}

func TestReportFallsBackWhenCardNotSent(t *testing.T) {
	f := newReportCardFixture(t, true)
	f.client.errorsByChat = map[string]error{
		"sendMessage|" + fmt.Sprint(reportCardChannelID): &gotgbot.TelegramError{
			Method: "sendMessage", Code: 403, Description: "Forbidden: bot is not a member of the channel chat",
		},
	}
	f.report(t, f.reporter, "/report spam")

	assertFullGroupReply(t, f)
	if rows := storedReports(t, f.chat.Id); len(rows) != 1 || rows[0].AdminMessageId != nil {
		t.Fatalf("stored report = %+v, want a row without a card", rows)
	}
}

func TestRepeatReportWithoutCardRetriesCard(t *testing.T) {
	f := newReportCardFixture(t, true)
	f.client.errorsByChat = map[string]error{
		"sendMessage|" + fmt.Sprint(reportCardChannelID): errors.New("context deadline exceeded"),
	}
	f.report(t, f.reporter, "/report spam")
	f.client.errorsByChat = nil
	f.client.calls = nil

	f.report(t, f.reporter, "/report spam again")

	if got := len(callsToChat(f.client, "sendMessage", reportCardChannelID)); got != 1 {
		t.Fatalf("log channel sendMessage calls = %d, want the card the first report could not post", got)
	}
	assertShortGroupReply(t, f)
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
	assertShortGroupReply(t, f)
}

func TestRepeatReportAfterCloseKeepsShortReply(t *testing.T) {
	f := newReportCardFixture(t, true)
	f.report(t, f.reporter, "/report spam")
	if _, err := reports.Resolve(f.chat.Id, 505, models.ReportStatusDismissed, reportCardAdminID); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	f.client.calls = nil

	f.report(t, f.reporter, "/report still spam")

	// The admins already decided this member's report: no new card, and no
	// full report pinging them in the group.
	if got := len(callsToChat(f.client, "sendMessage", reportCardChannelID)); got != 0 {
		t.Fatalf("log channel sendMessage calls = %d, want no card for a decided report", got)
	}
	assertShortGroupReply(t, f)
	if rows := storedReports(t, f.chat.Id); len(rows) != 1 || rows[0].Status != models.ReportStatusDismissed {
		t.Fatalf("stored reports = %+v, want the decided report left closed", rows)
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
	// The card is not linked to a stored report, so the group keeps the
	// full report with its buttons.
	assertFullGroupReply(t, f)
}

func TestReportClosedBeforeDeliverySendsNoCard(t *testing.T) {
	f := newReportCardFixture(t, true)
	reported := &gotgbot.Message{MessageId: 505, Date: 1, Chat: f.chat, From: &f.target, Text: "spam"}
	if _, err := reports.Create(&models.Report{
		ChatId: f.chat.Id, ReportedUserId: f.target.Id, ReportedMessageId: 505, ReporterId: f.reporter.Id,
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	// An admin closes it with the in-group button before the card goes out.
	if _, err := reports.Resolve(f.chat.Id, 505, models.ReportStatusDismissed, reportCardAdminID); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if deliverReport(f.bot, i18n.MustNewTranslator("en"), &f.chat, &f.reporter, reported, "spam", true, true) {
		t.Fatal("deliverReport() = true, want false without a card")
	}

	if got := len(f.client.callsFor("copyMessage")); got != 0 {
		t.Fatalf("copyMessage calls = %d, want 0 for a closed report", got)
	}
	if got := len(callsToChat(f.client, "sendMessage", reportCardChannelID)); got != 0 {
		t.Fatalf("log channel sendMessage calls = %d, want no card for a closed report", got)
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
	assertFullGroupReply(t, f)
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
	assertFullGroupReply(t, f)
}
