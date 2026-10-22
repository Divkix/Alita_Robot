//go:build testtools

package modules

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/bans"
	"github.com/divkix/Alita_Robot/alita/db/logchannels"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

const banLogChannelID int64 = -1009955000001

var banLogChannel = gotgbot.Chat{Id: banLogChannelID, Type: "channel", Title: "Ban Log"}

// withBanLogChannel sets the fixture group's log channel to banLogChannel.
func withBanLogChannel(t *testing.T, f adminChatFixture) {
	t.Helper()
	if err := logchannels.Set(f.chat.Id, f.chat.Title, banLogChannelID); err != nil {
		t.Fatalf("logchannels.Set() error = %v", err)
	}
	t.Cleanup(func() { _ = logchannels.Unset(f.chat.Id) })
}

// logUnbanButtons returns the callback data of the Unban buttons on each
// entry sent to the log channel, in order.
func logUnbanButtons(t *testing.T, f adminChatFixture) [][]string {
	t.Helper()
	var entries [][]string
	for _, call := range callsToChat(f.client, "sendMessage", banLogChannelID) {
		var found []string
		if markup, ok := call.Params["reply_markup"]; ok && markup != nil {
			raw, err := json.Marshal(markup)
			if err != nil {
				t.Fatalf("marshal reply_markup: %v", err)
			}
			var keyboard gotgbot.InlineKeyboardMarkup
			if err := json.Unmarshal(raw, &keyboard); err != nil {
				t.Fatalf("decode reply_markup %s: %v", raw, err)
			}
			for _, row := range keyboard.InlineKeyboard {
				for _, button := range row {
					if strings.HasPrefix(button.CallbackData, banLogUnbanNamespace+"|") {
						found = append(found, button.CallbackData)
					}
				}
			}
		}
		entries = append(entries, found)
	}
	return entries
}

func seedOpenBan(t *testing.T, chatID, userID int64) models.BanRecord {
	t.Helper()
	seedBanRecord(t, models.BanRecord{ChatId: chatID, UserId: userID, Source: models.BanSourceSpamDB})
	return singleBanRecord(t, chatID)
}

func unbanLogData(recordID int64) string {
	return encodeCallbackData(banLogUnbanNamespace, map[string]string{"r": strconv.FormatInt(recordID, 10)})
}

// pressLogUnban presses data as from on a log entry in chat that carries it
// as its button.
func pressLogUnban(t *testing.T, f adminChatFixture, in gotgbot.Chat, from gotgbot.User, data string) {
	t.Helper()
	pressLogUnbanOn(t, f, from, data, gotgbot.Message{
		MessageId: 7001, Date: 1, Chat: in, Text: "Group (-100)\n#SPAMDB",
		ReplyMarkup: &gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{{Text: "Unban", CallbackData: data}}}},
	})
}

// pressLogUnbanOn presses data as from on message, which need not carry it.
func pressLogUnbanOn(t *testing.T, f adminChatFixture, from gotgbot.User, data string, message gotgbot.MaybeInaccessibleMessage) {
	t.Helper()
	query := &gotgbot.CallbackQuery{Id: "log-unban", From: from, Message: message, Data: data, ChatInstance: "log-unban"}
	ctx := ext.NewContext(f.bot, &gotgbot.Update{UpdateId: 3, CallbackQuery: query}, nil)
	if err := bansModule.banLogUnbanCallback(f.bot, ctx); err != ext.EndGroups {
		t.Fatalf("banLogUnbanCallback() error = %v, want EndGroups", err)
	}
}

func answered(t *testing.T, f adminChatFixture) string {
	t.Helper()
	text, _ := lastAnswer(t, f.client)
	return text
}

func TestBanLogEntryCarriesTheUnbanButtonOfItsRecord(t *testing.T) {
	useEnglishLocale(t)
	f := newAdminChatFixture(t)
	withBanLogChannel(t, f)

	ctx := newMuteReplyContext(f.bot, f.chat, f.admin, f.member, "/ban spam")
	if err := bansModule.ban(f.bot, ctx); err != ext.EndGroups {
		t.Fatalf("ban() error = %v, want EndGroups", err)
	}
	record := singleBanRecord(t, f.chat.Id)

	entries := logUnbanButtons(t, f)
	if len(entries) != 1 || len(entries[0]) != 1 || entries[0][0] != unbanLogData(record.ID) {
		t.Fatalf("log entries' Unban buttons = %v, want one entry with the button of record %d", entries, record.ID)
	}
}

func TestCaptchaFailureEntryHasTheButtonOnlyForABan(t *testing.T) {
	useEnglishLocale(t)
	f := newAdminChatFixture(t)
	withBanLogChannel(t, f)

	reportCaptchaFailure(f.bot, f.chat.Id, f.member.Id, "Member", "kick", "timeout", 0, unbanLogMarkup(f.chat.Id, 0))
	reportCaptchaFailure(f.bot, f.chat.Id, f.member.Id, "Member", "ban", "timeout", 0, unbanLogMarkup(f.chat.Id, 77))

	entries := logUnbanButtons(t, f)
	if len(entries) != 2 || len(entries[0]) != 0 || len(entries[1]) != 1 || entries[1][0] != unbanLogData(77) {
		t.Fatalf("captcha entries' Unban buttons = %v, want none on the kick and record 77's on the ban", entries)
	}
}

func TestLogUnbanButtonLiftsTheBanOnce(t *testing.T) {
	useEnglishLocale(t)
	f := newAdminChatFixture(t)
	withBanLogChannel(t, f)
	second := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Second"}
	f.addAdmins(fmt.Sprintf(`{"status":"administrator","user":{"id":%d,"is_bot":false,"first_name":"Second"},"can_restrict_members":true}`, second.Id))
	record := seedOpenBan(t, f.chat.Id, f.member.Id)

	pressLogUnban(t, f, banLogChannel, f.admin, unbanLogData(record.ID))
	if got, want := answered(t, f), translated(t, "bans_log_unban_done"); got != want {
		t.Fatalf("first press answered %q, want %q", got, want)
	}
	pressLogUnban(t, f, banLogChannel, second, unbanLogData(record.ID))
	if got, want := answered(t, f), translated(t, "bans_log_unban_already"); got != want {
		t.Fatalf("second press answered %q, want %q", got, want)
	}

	unbans := f.client.callsFor("unbanChatMember")
	if len(unbans) != 1 || fmt.Sprint(unbans[0].Params["chat_id"]) != fmt.Sprint(f.chat.Id) ||
		fmt.Sprint(unbans[0].Params["user_id"]) != fmt.Sprint(f.member.Id) {
		t.Fatalf("unbanChatMember calls = %v, want the member unbanned once in the group", unbans)
	}
	got, err := bans.GetByID(record.ID)
	if err != nil || got == nil || got.LiftedBy == nil || *got.LiftedBy != f.admin.Id {
		t.Fatalf("record after the presses = %+v, %v; want it lifted by the first admin", got, err)
	}
	if event := singleModerationEvent(t, f.chat.Id); event.Action != models.ModerationActionUnban || event.ActorId != f.admin.Id {
		t.Fatalf("moderation event = %+v, want the first admin's unban", event)
	}
	edits := callsToChat(f.client, "editMessageText", banLogChannelID)
	if len(edits) != 1 || !strings.Contains(fmt.Sprint(edits[0].Params["text"]), fmt.Sprintf("Unbanned by Admin (%d)", f.admin.Id)) {
		t.Fatalf("entry edits = %v, want one appending who unbanned", edits)
	}
	// The #unban entry /unban posts, without a button of its own.
	if entries := logUnbanButtons(t, f); len(entries) != 1 || len(entries[0]) != 0 {
		t.Fatalf("log entries after the unban = %v, want one #unban entry without buttons", entries)
	}
}

// Like the report card, the button acts only for an admin of the group the
// ban is in who can restrict members.
func TestLogUnbanButtonRefusesWhoCannotRestrictInTheGroup(t *testing.T) {
	useEnglishLocale(t)
	for name, presser := range map[string]func(f adminChatFixture) gotgbot.User{
		"member": func(f adminChatFixture) gotgbot.User { return f.member },
		"admin without restrict": func(f adminChatFixture) gotgbot.User {
			limited := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Limited"}
			f.addAdmins(fmt.Sprintf(`{"status":"administrator","user":{"id":%d,"is_bot":false,"first_name":"Limited"},"can_delete_messages":true}`, limited.Id))
			return limited
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newAdminChatFixture(t)
			withBanLogChannel(t, f)
			record := seedOpenBan(t, f.chat.Id, 43)

			pressLogUnban(t, f, banLogChannel, presser(f), unbanLogData(record.ID))

			if got := len(f.client.callsFor("unbanChatMember")); got != 0 {
				t.Fatalf("unbanChatMember calls = %d, want none", got)
			}
			if got, err := bans.GetByID(record.ID); err != nil || got == nil || !got.IsOpen(time.Now()) {
				t.Fatalf("record = %+v, %v; want it still open", got, err)
			}
		})
	}
}

// Callback data can be forged: a press outside the group's log channel does
// nothing.
func TestLogUnbanButtonOutsideTheLogChannelDoesNothing(t *testing.T) {
	useEnglishLocale(t)
	f := newAdminChatFixture(t)
	withBanLogChannel(t, f)
	record := seedOpenBan(t, f.chat.Id, 43)

	other := gotgbot.Chat{Id: banLogChannelID - 1, Type: "channel", Title: "Elsewhere"}
	pressLogUnban(t, f, other, f.admin, unbanLogData(record.ID))

	if got := len(f.client.callsFor("unbanChatMember")); got != 0 {
		t.Fatalf("unbanChatMember calls = %d, want none", got)
	}
	if got, err := bans.GetByID(record.ID); err != nil || got == nil || !got.IsOpen(time.Now()) {
		t.Fatalf("record = %+v, %v; want it still open", got, err)
	}
}

// Forged Unban data pressed on another message of the channel (a report
// card) neither unbans nor touches that message.
func TestLogUnbanDataOnAnotherMessageDoesNothing(t *testing.T) {
	useEnglishLocale(t)
	f := newAdminChatFixture(t)
	withBanLogChannel(t, f)
	record := seedOpenBan(t, f.chat.Id, 43)
	card := gotgbot.Message{
		MessageId: 7002, Date: 1, Chat: banLogChannel, Text: "#REPORT",
		ReplyMarkup: &gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{{Text: "Ban", CallbackData: cardActionData("ban", 1)}}}},
	}

	pressLogUnbanOn(t, f, f.admin, unbanLogData(record.ID), card)

	if got := len(f.client.callsFor("unbanChatMember")); got != 0 {
		t.Fatalf("unbanChatMember calls = %d, want none", got)
	}
	for _, method := range []string{"editMessageText", "editMessageReplyMarkup"} {
		if got := len(callsToChat(f.client, method, banLogChannelID)); got != 0 {
			t.Fatalf("%s calls = %d, want the card untouched", method, got)
		}
	}
	if got, err := bans.GetByID(record.ID); err != nil || got == nil || !got.IsOpen(time.Now()) {
		t.Fatalf("record = %+v, %v; want it still open", got, err)
	}
}

// An entry too old to read cannot be checked for its button: the press
// still unbans, but leaves the entry as it is.
func TestLogUnbanButtonOnAnOldEntryUnbansWithoutEditing(t *testing.T) {
	useEnglishLocale(t)
	f := newAdminChatFixture(t)
	withBanLogChannel(t, f)
	record := seedOpenBan(t, f.chat.Id, f.member.Id)

	pressLogUnbanOn(t, f, f.admin, unbanLogData(record.ID), gotgbot.InaccessibleMessage{Chat: banLogChannel, MessageId: 7003})

	if got := len(f.client.callsFor("unbanChatMember")); got != 1 {
		t.Fatalf("unbanChatMember calls = %d, want 1", got)
	}
	for _, method := range []string{"editMessageText", "editMessageReplyMarkup"} {
		if got := len(callsToChat(f.client, method, banLogChannelID)); got != 0 {
			t.Fatalf("%s calls = %d, want the old entry untouched", method, got)
		}
	}
}

func TestLogUnbanButtonAfterUnbanSaysSo(t *testing.T) {
	useEnglishLocale(t)
	f := newAdminChatFixture(t)
	withBanLogChannel(t, f)
	record := seedOpenBan(t, f.chat.Id, f.member.Id)
	if err := bansModule.unban(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, fmt.Sprintf("/unban %d", f.member.Id))); err != ext.EndGroups {
		t.Fatalf("unban() error = %v, want EndGroups", err)
	}
	before := len(f.client.callsFor("unbanChatMember"))

	pressLogUnban(t, f, banLogChannel, f.admin, unbanLogData(record.ID))

	if got, want := answered(t, f), translated(t, "bans_log_unban_already"); got != want {
		t.Fatalf("press answered %q, want %q", got, want)
	}
	if got := len(f.client.callsFor("unbanChatMember")); got != before {
		t.Fatalf("unbanChatMember calls = %d, want %d: no second unban", got, before)
	}
	if got := len(callsToChat(f.client, "editMessageReplyMarkup", banLogChannelID)); got != 1 {
		t.Fatalf("editMessageReplyMarkup calls = %d, want the stale button dropped", got)
	}
}

func TestLogUnbanButtonFailureKeepsTheBanOpen(t *testing.T) {
	useEnglishLocale(t)
	f := newAdminChatFixture(t)
	withBanLogChannel(t, f)
	record := seedOpenBan(t, f.chat.Id, f.member.Id)
	f.client.errors["unbanChatMember"] = errors.New("Bad Request: not enough rights")

	pressLogUnban(t, f, banLogChannel, f.admin, unbanLogData(record.ID))

	if got, want := answered(t, f), translated(t, "bans_log_unban_failed"); got != want {
		t.Fatalf("press answered %q, want %q", got, want)
	}
	if got, err := bans.GetByID(record.ID); err != nil || got == nil || !got.IsOpen(time.Now()) {
		t.Fatalf("record = %+v, %v; want it open again", got, err)
	}
	if got := len(callsToChat(f.client, "editMessageText", banLogChannelID)); got != 0 {
		t.Fatalf("entry edits = %d, want the entry untouched", got)
	}
	if events := chatModerationEvents(t, f.chat.Id); len(events) != 0 {
		t.Fatalf("moderation events = %+v, want none for a failed unban", events)
	}
}

func TestLogUnbanButtonFitsTelegramLimit(t *testing.T) {
	useEnglishLocale(t)
	if markup := unbanLogMarkup(-1001234567890, 1<<62); markup == nil || len(markup.InlineKeyboard[0][0].CallbackData) > 64 {
		t.Fatalf("unbanLogMarkup() for the largest record = %+v, want a button within 64 bytes", markup)
	}
	if markup := unbanLogMarkup(-1001234567890, 0); markup != nil {
		t.Fatalf("unbanLogMarkup() without a record = %+v, want nil", markup)
	}
}
