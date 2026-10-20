package modules

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

var reportCardChannel = gotgbot.Chat{Id: reportCardChannelID, Type: "channel", Title: "Log Channel"}

func reportCardBotMember() gotgbot.MergedChatMember {
	return gotgbot.MergedChatMember{
		Status:             "administrator",
		User:               gotgbot.User{Id: 999, IsBot: true},
		CanRestrictMembers: true,
		CanDeleteMessages:  true,
	}
}

// cardActions returns the rpt callback data of a card's buttons by action.
func cardActions(t *testing.T, markup any) map[string]string {
	t.Helper()
	return markupActions(t, markup, "rpt")
}

func cardActionData(action string, reportID int64) string {
	return encodeCallbackData("rpt", map[string]string{"a": action, "i": strconv.FormatInt(reportID, 10)})
}

func newCardCallbackContext(bot *gotgbot.Bot, chat gotgbot.Chat, messageID int64, from gotgbot.User, data string) *ext.Context {
	query := &gotgbot.CallbackQuery{
		Id:           "card-callback",
		From:         from,
		Message:      gotgbot.Message{MessageId: messageID, Date: 1, Chat: chat, Text: "#REPORT"},
		Data:         data,
		ChatInstance: "card-chat-instance",
	}
	return ext.NewContext(bot, &gotgbot.Update{UpdateId: 3, CallbackQuery: query}, nil)
}

// reportedCard reports message 505 and returns its card's callback data.
func reportedCard(t *testing.T, f reportCardFixture) map[string]string {
	t.Helper()
	f.report(t, f.reporter, "/report spam")
	cards := callsToChat(f.client, "sendMessage", reportCardChannelID)
	if len(cards) != 1 {
		t.Fatalf("log channel sendMessage calls = %d, want 1 card", len(cards))
	}
	actions := cardActions(t, cards[0].Params["reply_markup"])
	if len(actions) != 4 {
		t.Fatalf("card actions = %v, want ban, mute, delete and dismiss", actions)
	}
	return actions
}

func pressCard(t *testing.T, f reportCardFixture, chat gotgbot.Chat, messageID int64, from gotgbot.User, data string) error {
	t.Helper()
	return reportsModule.reportCardHandler(f.bot, newCardCallbackContext(f.bot, chat, messageID, from, data))
}

func requireReportStatus(t *testing.T, chatID int64, status string) []models.Report {
	t.Helper()
	rows := storedReports(t, chatID)
	if len(rows) == 0 {
		t.Fatal("no stored reports")
	}
	for _, row := range rows {
		if row.Status != status {
			t.Fatalf("report %d status = %q, want %q", row.ID, row.Status, status)
		}
	}
	return rows
}

func TestReportCardCallbacksFitTelegramLimit(t *testing.T) {
	f := newReportCardFixture(t, true)
	actions := reportedCard(t, f)
	rows := storedReports(t, f.chat.Id)
	for action, data := range actions {
		decoded, ok := decodeCallbackData(data, "rpt")
		if !ok {
			t.Fatalf("%s callback %q does not decode", action, data)
		}
		if id, _ := decoded.Field("i"); id != strconv.FormatInt(rows[0].ID, 10) {
			t.Fatalf("%s callback carries report %s, want %d", action, id, rows[0].ID)
		}
	}

	markup := reportCardMarkup(i18n.MustNewTranslator("en"), &f.chat, 505, math.MaxInt64)
	widest := cardActions(t, markup)
	if len(widest) != 4 {
		t.Fatalf("card actions for the largest report id = %v, want 4", widest)
	}
	for action, data := range widest {
		if data == "" || len(data) > 64 {
			t.Fatalf("%s callback %q is %d bytes, want 1..64", action, data, len(data))
		}
	}
}

func TestCardActionRejectsNonAdminOfSourceGroup(t *testing.T) {
	f := newReportCardFixture(t, true)
	channelAdmin := gotgbot.User{Id: reportCardAdminID, FirstName: "Channel Admin"}
	seedCallbackAdmins(t, f.chat.Id, reportCardBotMember())
	seedCallbackAdmins(t, reportCardChannelID, reportCardBotMember(), gotgbot.MergedChatMember{
		Status: "administrator", User: channelAdmin, CanRestrictMembers: true, CanDeleteMessages: true,
	})
	actions := reportedCard(t, f)

	for _, action := range []string{"ban", "mute", "delete", "dismiss"} {
		if err := pressCard(t, f, reportCardChannel, 9001, channelAdmin, actions[action]); err != ext.EndGroups {
			t.Fatalf("%s press error = %v, want EndGroups", action, err)
		}
	}
	for _, method := range []string{"banChatMember", "restrictChatMember", "deleteMessage"} {
		if got := len(f.client.callsFor(method)); got != 0 {
			t.Fatalf("%s calls = %d, want 0 for an admin of the log channel only", method, got)
		}
	}
	if got := len(callsToChat(f.client, "editMessageText", reportCardChannelID)); got != 0 {
		t.Fatalf("card edits = %d, want the card untouched", got)
	}
	if got := len(f.client.callsFor("answerCallbackQuery")); got != 4 {
		t.Fatalf("answerCallbackQuery calls = %d, want every press answered", got)
	}
	requireReportStatus(t, f.chat.Id, models.ReportStatusOpen)
}

func TestCardBanTargetsSourceGroup(t *testing.T) {
	f := newReportCardFixture(t, true)
	admin := gotgbot.User{Id: reportCardAdminID, FirstName: "Group Admin"}
	seedCallbackAdmins(t, f.chat.Id, reportCardBotMember(), gotgbot.MergedChatMember{
		Status: "administrator", User: admin, CanRestrictMembers: true,
	})
	actions := reportedCard(t, f)

	if err := pressCard(t, f, reportCardChannel, 9001, admin, actions["ban"]); err != ext.EndGroups {
		t.Fatalf("ban press error = %v, want EndGroups", err)
	}

	bans := f.client.callsFor("banChatMember")
	if len(bans) != 1 {
		t.Fatalf("banChatMember calls = %d, want 1", len(bans))
	}
	if fmt.Sprint(bans[0].Params["chat_id"]) != fmt.Sprint(f.chat.Id) || fmt.Sprint(bans[0].Params["user_id"]) != "43" {
		t.Fatalf("banChatMember params = %v, want user 43 banned in the reported group %d", bans[0].Params, f.chat.Id)
	}
	rows := requireReportStatus(t, f.chat.Id, models.ReportStatusBanned)
	if rows[0].ResolvedBy == nil || *rows[0].ResolvedBy != reportCardAdminID {
		t.Fatalf("resolved_by = %v, want %d", rows[0].ResolvedBy, reportCardAdminID)
	}
	edits := callsToChat(f.client, "editMessageText", reportCardChannelID)
	if len(edits) != 1 || fmt.Sprint(edits[0].Params["message_id"]) != "9001" {
		t.Fatalf("card edits = %v, want card 9001 marked", edits)
	}
	if left := cardActions(t, edits[0].Params["reply_markup"]); len(left) != 0 {
		t.Fatalf("closed card actions = %v, want none", left)
	}
	if got := len(f.client.callsFor("answerCallbackQuery")); got != 1 {
		t.Fatalf("answerCallbackQuery calls = %d, want 1", got)
	}
}

func TestCardMuteAndDismiss(t *testing.T) {
	t.Run("mute", func(t *testing.T) {
		f := newReportCardFixture(t, true)
		admin := gotgbot.User{Id: reportCardAdminID, FirstName: "Group Admin"}
		seedCallbackAdmins(t, f.chat.Id, reportCardBotMember(), gotgbot.MergedChatMember{
			Status: "administrator", User: admin, CanRestrictMembers: true,
		})
		actions := reportedCard(t, f)
		if err := pressCard(t, f, reportCardChannel, 9001, admin, actions["mute"]); err != ext.EndGroups {
			t.Fatalf("mute press error = %v, want EndGroups", err)
		}
		mutes := f.client.callsFor("restrictChatMember")
		if len(mutes) != 1 || fmt.Sprint(mutes[0].Params["chat_id"]) != fmt.Sprint(f.chat.Id) ||
			fmt.Sprint(mutes[0].Params["user_id"]) != "43" {
			t.Fatalf("restrictChatMember calls = %v, want user 43 muted in the reported group", mutes)
		}
		var perms gotgbot.ChatPermissions
		raw, _ := json.Marshal(mutes[0].Params["permissions"])
		if err := json.Unmarshal(raw, &perms); err != nil || perms.CanSendMessages {
			t.Fatalf("mute permissions = %s, want sending disabled", raw)
		}
		requireReportStatus(t, f.chat.Id, models.ReportStatusMuted)
	})

	t.Run("dismiss", func(t *testing.T) {
		f := newReportCardFixture(t, true)
		admin := gotgbot.User{Id: reportCardAdminID, FirstName: "Group Admin"}
		seedCallbackAdmins(t, f.chat.Id, reportCardBotMember(), gotgbot.MergedChatMember{
			Status: "administrator", User: admin,
		})
		actions := reportedCard(t, f)
		if err := pressCard(t, f, reportCardChannel, 9001, admin, actions["dismiss"]); err != ext.EndGroups {
			t.Fatalf("dismiss press error = %v, want EndGroups", err)
		}
		for _, method := range []string{"banChatMember", "restrictChatMember", "deleteMessage"} {
			if got := len(f.client.callsFor(method)); got != 0 {
				t.Fatalf("%s calls = %d, want no action on dismiss", method, got)
			}
		}
		requireReportStatus(t, f.chat.Id, models.ReportStatusDismissed)
		if got := len(callsToChat(f.client, "editMessageText", reportCardChannelID)); got != 1 {
			t.Fatalf("card edits = %d, want the card marked dismissed", got)
		}
	})
}

func TestCardDoublePressActsOnce(t *testing.T) {
	f := newReportCardFixture(t, true)
	first := gotgbot.User{Id: reportCardAdminID, FirstName: "First"}
	second := gotgbot.User{Id: reportCardAdminID + 1, FirstName: "Second"}
	seedCallbackAdmins(t, f.chat.Id, reportCardBotMember(),
		gotgbot.MergedChatMember{Status: "administrator", User: first, CanRestrictMembers: true},
		gotgbot.MergedChatMember{Status: "administrator", User: second, CanRestrictMembers: true},
	)
	actions := reportedCard(t, f)

	for _, admin := range []gotgbot.User{first, first, second} {
		if err := pressCard(t, f, reportCardChannel, 9001, admin, actions["ban"]); err != ext.EndGroups {
			t.Fatalf("ban press by %d error = %v, want EndGroups", admin.Id, err)
		}
	}
	if err := pressCard(t, f, reportCardChannel, 9001, second, actions["mute"]); err != ext.EndGroups {
		t.Fatalf("mute press error = %v, want EndGroups", err)
	}

	if got := len(f.client.callsFor("banChatMember")); got != 1 {
		t.Fatalf("banChatMember calls = %d, want exactly 1", got)
	}
	if got := len(f.client.callsFor("restrictChatMember")); got != 0 {
		t.Fatalf("restrictChatMember calls = %d, want none after the report was closed", got)
	}
	if got := len(callsToChat(f.client, "editMessageText", reportCardChannelID)); got != 1 {
		t.Fatalf("card edits = %d, want 1", got)
	}
	if got := len(f.client.callsFor("answerCallbackQuery")); got != 4 {
		t.Fatalf("answerCallbackQuery calls = %d, want every press answered", got)
	}
	rows := requireReportStatus(t, f.chat.Id, models.ReportStatusBanned)
	if *rows[0].ResolvedBy != first.Id {
		t.Fatalf("resolved_by = %d, want the first admin %d", *rows[0].ResolvedBy, first.Id)
	}
}

func TestCardForgedIdFromOtherCard(t *testing.T) {
	f := newReportCardFixture(t, true)
	admin := gotgbot.User{Id: reportCardAdminID, FirstName: "Group Admin"}
	seedCallbackAdmins(t, f.chat.Id, reportCardBotMember(), gotgbot.MergedChatMember{
		Status: "administrator", User: admin, CanRestrictMembers: true,
	})
	reportedCard(t, f) // message 505 → card 9001

	f.client.responses["sendMessage"] = json.RawMessage(
		`{"message_id":9002,"date":1,"chat":{"id":-1001,"type":"supergroup","title":"Test Chat"}}`,
	)
	other := gotgbot.User{Id: 46, FirstName: "Other Target"}
	ctx := newReportReplyContext(f.bot, f.chat, f.reporter, other, "/report")
	ctx.EffectiveMessage.ReplyToMessage.MessageId = 506
	if err := reportsModule.report(f.bot, ctx); err != ext.EndGroups {
		t.Fatalf("report error = %v, want EndGroups", err)
	}
	var otherRow models.Report
	for _, row := range storedReports(t, f.chat.Id) {
		if row.ReportedMessageId == 506 {
			otherRow = row
		}
	}
	if otherRow.AdminMessageId == nil || *otherRow.AdminMessageId != 9002 {
		t.Fatalf("second report card = %+v, want card 9002", otherRow)
	}
	forged := cardActionData("ban", otherRow.ID)

	// The second report's id pressed on the first card.
	if err := pressCard(t, f, reportCardChannel, 9001, admin, forged); err != ext.EndGroups {
		t.Fatalf("forged press error = %v, want EndGroups", err)
	}
	// The right card id from another chat.
	elsewhere := gotgbot.Chat{Id: reportCardChannelID - 1, Type: "channel", Title: "Other Channel"}
	if err := pressCard(t, f, elsewhere, 9002, admin, forged); err != ext.EndGroups {
		t.Fatalf("other chat press error = %v, want EndGroups", err)
	}
	if got := len(f.client.callsFor("banChatMember")); got != 0 {
		t.Fatalf("banChatMember calls = %d, want 0 for data not from its own card", got)
	}
	requireReportStatus(t, f.chat.Id, models.ReportStatusOpen)

	// The same data from its own card works.
	if err := pressCard(t, f, reportCardChannel, 9002, admin, forged); err != ext.EndGroups {
		t.Fatalf("own card press error = %v, want EndGroups", err)
	}
	bans := f.client.callsFor("banChatMember")
	if len(bans) != 1 || fmt.Sprint(bans[0].Params["user_id"]) != "46" {
		t.Fatalf("banChatMember calls = %v, want user 46 banned from its own card", bans)
	}
}

// A card left with buttons after its report was decided must not act on a
// newer report of the same message, which has a card of its own.
func TestStaleCardLeavesNewReportOpen(t *testing.T) {
	f := newReportCardFixture(t, true)
	admin := gotgbot.User{Id: reportCardAdminID, FirstName: "Group Admin"}
	seedCallbackAdmins(t, f.chat.Id, reportCardBotMember(), gotgbot.MergedChatMember{
		Status: "administrator", User: admin, CanRestrictMembers: true,
	})
	stale := reportedCard(t, f) // report A of message 505 → card 9001
	if err := pressCard(t, f, reportCardChannel, 9001, admin, stale["dismiss"]); err != ext.EndGroups {
		t.Fatalf("dismiss press error = %v, want EndGroups", err)
	}

	f.client.responses["sendMessage"] = json.RawMessage(
		`{"message_id":9002,"date":1,"chat":{"id":-1001,"type":"supergroup","title":"Test Chat"}}`,
	)
	f.report(t, gotgbot.User{Id: 44, FirstName: "Second"}, "/report") // report B → card 9002
	var newer models.Report
	for _, row := range storedReports(t, f.chat.Id) {
		if row.ReporterId == 44 {
			newer = row
		}
	}
	if newer.Status != models.ReportStatusOpen || newer.AdminMessageId == nil || *newer.AdminMessageId != 9002 {
		t.Fatalf("second report = %+v, want open with card 9002", newer)
	}

	// The buttons are still on card 9001, say its edit failed.
	if err := pressCard(t, f, reportCardChannel, 9001, admin, stale["ban"]); err != ext.EndGroups {
		t.Fatalf("stale ban press error = %v, want EndGroups", err)
	}
	if got := len(f.client.callsFor("banChatMember")); got != 0 {
		t.Fatalf("banChatMember calls = %d, want 0 from a card whose report is closed", got)
	}
	for _, row := range storedReports(t, f.chat.Id) {
		if row.ID == newer.ID && row.Status != models.ReportStatusOpen {
			t.Fatalf("second report status = %q, want it left open for card 9002", row.Status)
		}
	}
}

// The card sits in a channel the bot has no language for; its presses are
// answered in the reported group's language, like the card's buttons.
func TestCardAnswersInSourceGroupLanguage(t *testing.T) {
	locales := map[string]string{}
	for _, code := range []string{"en", "ru"} {
		raw, err := os.ReadFile("../../locales/" + code + ".yml")
		if err != nil {
			t.Fatalf("read %s.yml: %v", code, err)
		}
		locales[code] = string(raw)
	}
	restoreI18n, err := i18n.OverrideLocalesForTest(locales)
	if err != nil {
		t.Fatalf("OverrideLocalesForTest: %v", err)
	}
	t.Cleanup(restoreI18n)

	f := newReportCardFixture(t, true)
	admin := gotgbot.User{Id: reportCardAdminID, FirstName: "Group Admin"}
	outsider := gotgbot.User{Id: reportCardAdminID + 1, FirstName: "Outsider"}
	seedCallbackAdmins(t, f.chat.Id, reportCardBotMember(), gotgbot.MergedChatMember{
		Status: "administrator", User: admin, CanRestrictMembers: true,
	})
	actions := reportedCard(t, f)
	if err := lang.ChangeGroupLanguage(f.chat.Id, "ru"); err != nil {
		t.Fatalf("ChangeGroupLanguage() error = %v", err)
	}
	var channelRows int64
	if err := db.DB.Model(&models.Chat{}).Where("chat_id = ?", reportCardChannelID).Count(&channelRows).Error; err != nil || channelRows != 0 {
		t.Fatalf("log channel chats rows = %d (err %v), want none", channelRows, err)
	}

	ru := i18n.MustNewTranslator("ru")
	en := i18n.MustNewTranslator("en")
	for _, press := range []struct {
		from gotgbot.User
		key  string
	}{
		{outsider, "chat_status_user_admin_button_error"},
		{admin, "reports_success_ban"},
	} {
		before := len(f.client.callsFor("answerCallbackQuery"))
		if err := pressCard(t, f, reportCardChannel, 9001, press.from, actions["ban"]); err != ext.EndGroups {
			t.Fatalf("press by %d error = %v, want EndGroups", press.from.Id, err)
		}
		answers := f.client.callsFor("answerCallbackQuery")
		if len(answers) != before+1 {
			t.Fatalf("answerCallbackQuery calls = %d, want %d", len(answers), before+1)
		}
		want, _ := ru.GetString(press.key)
		english, _ := en.GetString(press.key)
		if got := fmt.Sprint(answers[len(answers)-1].Params["text"]); got != want || got == english {
			t.Fatalf("answer to %d = %q, want %s in Russian %q", press.from.Id, got, press.key, want)
		}
	}
}

func TestCardGranularPermissions(t *testing.T) {
	t.Run("admin without restrict rights", func(t *testing.T) {
		f := newReportCardFixture(t, true)
		admin := gotgbot.User{Id: reportCardAdminID, FirstName: "Delete Admin"}
		seedCallbackAdmins(t, f.chat.Id, reportCardBotMember(), gotgbot.MergedChatMember{
			Status: "administrator", User: admin, CanDeleteMessages: true,
		})
		actions := reportedCard(t, f)

		for _, action := range []string{"ban", "mute"} {
			if err := pressCard(t, f, reportCardChannel, 9001, admin, actions[action]); err != ext.EndGroups {
				t.Fatalf("%s press error = %v, want EndGroups", action, err)
			}
		}
		if got := len(f.client.callsFor("banChatMember")) + len(f.client.callsFor("restrictChatMember")); got != 0 {
			t.Fatalf("restrict calls = %d, want 0 for an admin without ban rights", got)
		}
		requireReportStatus(t, f.chat.Id, models.ReportStatusOpen)

		if err := pressCard(t, f, reportCardChannel, 9001, admin, actions["delete"]); err != ext.EndGroups {
			t.Fatalf("delete press error = %v, want EndGroups", err)
		}
		deletes := f.client.callsFor("deleteMessage")
		if len(deletes) != 1 || fmt.Sprint(deletes[0].Params["chat_id"]) != fmt.Sprint(f.chat.Id) ||
			fmt.Sprint(deletes[0].Params["message_id"]) != "505" {
			t.Fatalf("deleteMessage calls = %v, want message 505 deleted in the reported group", deletes)
		}
		requireReportStatus(t, f.chat.Id, models.ReportStatusDeleted)
	})

	t.Run("bot without restrict and delete rights", func(t *testing.T) {
		f := newReportCardFixture(t, true)
		admin := gotgbot.User{Id: reportCardAdminID, FirstName: "Group Admin"}
		seedCallbackAdmins(t, f.chat.Id,
			gotgbot.MergedChatMember{Status: "administrator", User: gotgbot.User{Id: 999, IsBot: true}},
			gotgbot.MergedChatMember{
				Status: "administrator", User: admin, CanRestrictMembers: true, CanDeleteMessages: true,
			},
		)
		actions := reportedCard(t, f)

		for _, action := range []string{"ban", "mute", "delete"} {
			if err := pressCard(t, f, reportCardChannel, 9001, admin, actions[action]); err != ext.EndGroups {
				t.Fatalf("%s press error = %v, want EndGroups", action, err)
			}
		}
		for _, method := range []string{"banChatMember", "restrictChatMember", "deleteMessage"} {
			if got := len(f.client.callsFor(method)); got != 0 {
				t.Fatalf("%s calls = %d, want 0 while the bot lacks the right", method, got)
			}
		}
		if got := len(f.client.callsFor("answerCallbackQuery")); got != 3 {
			t.Fatalf("answerCallbackQuery calls = %d, want every press refused", got)
		}
		requireReportStatus(t, f.chat.Id, models.ReportStatusOpen)
	})

	t.Run("admin without delete rights", func(t *testing.T) {
		f := newReportCardFixture(t, true)
		admin := gotgbot.User{Id: reportCardAdminID, FirstName: "Ban Admin"}
		seedCallbackAdmins(t, f.chat.Id, reportCardBotMember(), gotgbot.MergedChatMember{
			Status: "administrator", User: admin, CanRestrictMembers: true,
		})
		actions := reportedCard(t, f)

		if err := pressCard(t, f, reportCardChannel, 9001, admin, actions["delete"]); err != ext.EndGroups {
			t.Fatalf("delete press error = %v, want EndGroups", err)
		}
		if got := len(f.client.callsFor("deleteMessage")); got != 0 {
			t.Fatalf("deleteMessage calls = %d, want 0 for an admin without delete rights", got)
		}
		if got := len(f.client.callsFor("answerCallbackQuery")); got != 1 {
			t.Fatalf("answerCallbackQuery calls = %d, want the press refused", got)
		}
		requireReportStatus(t, f.chat.Id, models.ReportStatusOpen)
	})
}

func TestCardActionFailureReopensReports(t *testing.T) {
	f := newReportCardFixture(t, true)
	admin := gotgbot.User{Id: reportCardAdminID, FirstName: "Group Admin"}
	seedCallbackAdmins(t, f.chat.Id, reportCardBotMember(), gotgbot.MergedChatMember{
		Status: "administrator", User: admin, CanRestrictMembers: true,
	})
	actions := reportedCard(t, f)

	f.client.errors["banChatMember"] = errors.New("Bad Request: not enough rights")
	if err := pressCard(t, f, reportCardChannel, 9001, admin, actions["ban"]); err == nil || err == ext.EndGroups {
		t.Fatalf("ban press error = %v, want the ban failure", err)
	}
	rows := requireReportStatus(t, f.chat.Id, models.ReportStatusOpen)
	if rows[0].ResolvedBy != nil || rows[0].ResolvedAt != nil {
		t.Fatalf("reopened report = %+v, want no resolver", rows[0])
	}
	if got := len(callsToChat(f.client, "editMessageText", reportCardChannelID)); got != 0 {
		t.Fatalf("card edits = %d, want the card left open", got)
	}

	if got := len(f.client.callsFor("answerCallbackQuery")); got != 1 {
		t.Fatalf("answerCallbackQuery calls = %d, want the failed press answered", got)
	}

	delete(f.client.errors, "banChatMember")
	if err := pressCard(t, f, reportCardChannel, 9001, admin, actions["ban"]); err != ext.EndGroups {
		t.Fatalf("retry error = %v, want EndGroups", err)
	}
	requireReportStatus(t, f.chat.Id, models.ReportStatusBanned)
}

func TestCardStoreFailureAnswersPress(t *testing.T) {
	f := newReportCardFixture(t, true)
	admin := gotgbot.User{Id: reportCardAdminID, FirstName: "Group Admin"}
	seedCallbackAdmins(t, f.chat.Id, reportCardBotMember(), gotgbot.MergedChatMember{
		Status: "administrator", User: admin, CanRestrictMembers: true,
	})
	actions := reportedCard(t, f)
	// The report still loads, but closing it fails.
	if err := db.DB.Migrator().DropColumn(&models.Report{}, "resolved_at"); err != nil {
		t.Fatalf("drop resolved_at: %v", err)
	}
	t.Cleanup(func() {
		if err := db.DB.AutoMigrate(&models.Report{}); err != nil {
			t.Fatalf("restore reports table: %v", err)
		}
	})

	if err := pressCard(t, f, reportCardChannel, 9001, admin, actions["ban"]); err == nil || err == ext.EndGroups {
		t.Fatalf("ban press error = %v, want the store failure", err)
	}
	if got := len(f.client.callsFor("banChatMember")); got != 0 {
		t.Fatalf("banChatMember calls = %d, want none when the report cannot be closed", got)
	}
	if got := len(f.client.callsFor("answerCallbackQuery")); got != 1 {
		t.Fatalf("answerCallbackQuery calls = %d, want the press answered", got)
	}
}

func TestCardIgnoresReportClosedInGroup(t *testing.T) {
	groupAdmin := gotgbot.User{Id: reportCardAdminID, FirstName: "Group Admin"}
	for action, status := range map[string]string{
		"kick":     models.ReportStatusKicked,
		"ban":      models.ReportStatusBanned,
		"delete":   models.ReportStatusDeleted,
		"resolved": models.ReportStatusDismissed,
	} {
		t.Run(action, func(t *testing.T) {
			f := newReportCardFixture(t, true)
			seedCallbackAdmins(t, f.chat.Id, reportCardBotMember(), gotgbot.MergedChatMember{
				Status: "creator", User: groupAdmin,
			})
			actions := reportedCard(t, f)
			data := encodeCallbackData("report", map[string]string{"a": action, "u": "43", "m": "505"})
			if err := reportsModule.markResolvedButtonHandler(f.bot, newModuleCallbackContext(f.bot, f.chat, groupAdmin, data)); err != ext.EndGroups {
				t.Fatalf("legacy %s error = %v, want EndGroups", action, err)
			}
			rows := requireReportStatus(t, f.chat.Id, status)
			if rows[0].ResolvedBy == nil || *rows[0].ResolvedBy != groupAdmin.Id {
				t.Fatalf("resolved_by = %v, want %d", rows[0].ResolvedBy, groupAdmin.Id)
			}

			// The card no longer acts on a report closed in the group.
			bansBefore := len(f.client.callsFor("banChatMember"))
			if err := pressCard(t, f, reportCardChannel, 9001, groupAdmin, actions["ban"]); err != ext.EndGroups {
				t.Fatalf("card press error = %v, want EndGroups", err)
			}
			if got := len(f.client.callsFor("banChatMember")); got != bansBefore {
				t.Fatalf("banChatMember calls = %d, want %d: the report is already closed", got, bansBefore)
			}
		})
	}
}

// Guard: a decision on a message is final for its reporters.
func TestRepeatReportAfterDecisionIsIgnored(t *testing.T) {
	f := newReportCardFixture(t, true)
	admin := gotgbot.User{Id: reportCardAdminID, FirstName: "Group Admin"}
	seedCallbackAdmins(t, f.chat.Id, reportCardBotMember(), gotgbot.MergedChatMember{
		Status: "administrator", User: admin,
	})
	actions := reportedCard(t, f)
	if err := pressCard(t, f, reportCardChannel, 9001, admin, actions["dismiss"]); err != ext.EndGroups {
		t.Fatalf("dismiss press error = %v, want EndGroups", err)
	}

	f.report(t, f.reporter, "/report spam again")

	if rows := storedReports(t, f.chat.Id); len(rows) != 1 {
		t.Fatalf("stored reports = %d, want the closed report only", len(rows))
	}
	if got := len(callsToChat(f.client, "sendMessage", reportCardChannelID)); got != 1 {
		t.Fatalf("log channel sendMessage calls = %d, want no new card", got)
	}
}
