//go:build testtools

package modules

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/recurringmessages"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

var recurringTestNow = time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)

// recurringFixture is an admin chat with /recurring loaded, the clock fixed at
// recurringTestNow, and the chat's recurring rows removed after the test.
func recurringFixture(t *testing.T) (adminChatFixture, *ext.Dispatcher) {
	t.Helper()
	useRealLocales(t)

	previous := recurringNow
	recurringNow = func() time.Time { return recurringTestNow }
	t.Cleanup(func() { recurringNow = previous })

	f := newAdminChatFixture(t)
	t.Cleanup(func() {
		if err := db.DB.Where("chat_id = ?", f.chat.Id).Delete(&models.RecurringMessage{}).Error; err != nil {
			t.Errorf("cleanup recurring messages: %v", err)
		}
	})
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadRecurring(dispatcher)
	return f, dispatcher
}

func recurringRowsFor(t *testing.T, chatID int64) []models.RecurringMessage {
	t.Helper()
	rows, err := recurringmessages.List(chatID)
	if err != nil {
		t.Fatalf("recurringmessages.List(%d) error = %v", chatID, err)
	}
	return rows
}

func seedRecurring(t *testing.T, chatID int64, text string) *models.RecurringMessage {
	t.Helper()
	row := &models.RecurringMessage{
		ChatID:       chatID,
		Text:         text,
		MsgType:      db.TEXT,
		IntervalSecs: int64(time.Hour / time.Second),
		NextRunAt:    recurringTestNow.Add(time.Hour),
		Enabled:      true,
	}
	if err := recurringmessages.Add(row); err != nil {
		t.Fatalf("recurringmessages.Add() error = %v", err)
	}
	return row
}

func TestRecurringAddStoresTheMessageAndReplies(t *testing.T) {
	f, dispatcher := recurringFixture(t)

	aispamProcessUpdate(t, dispatcher, f.bot, f.chat, f.admin, 1001, "/recurring add 6h Read the <b>rules</b>, {chatname}!")

	rows := recurringRowsFor(t, f.chat.Id)
	if len(rows) != 1 {
		t.Fatalf("stored %d rows, want 1", len(rows))
	}
	row := rows[0]
	if row.IntervalSecs != 6*3600 || !row.Enabled || row.MsgType != db.TEXT {
		t.Fatalf("row = %+v, want a 6h enabled text message", row)
	}
	if !strings.Contains(row.Text, "{chatname}") || !strings.Contains(row.Text, "rules") || strings.Contains(row.Text, "6h") {
		t.Fatalf("stored text %q, want the content without the interval and with the filling kept", row.Text)
	}
	if want := recurringTestNow.Add(6 * time.Hour); !row.NextRunAt.Equal(want) {
		t.Fatalf("NextRunAt = %v, want %v", row.NextRunAt, want)
	}
	if row.ThreadID != 0 || row.DeletePrevious || row.EndsAt != nil || row.CreatedBy != f.admin.Id {
		t.Fatalf("row = %+v, want default options created by the admin", row)
	}

	reply := lastMessageTo(t, f.client, f.chat.Id)
	if want := translatedWith(t, "recurring_added", i18n.TranslationParams{"id": row.ID}); !strings.Contains(reply, want) {
		t.Fatalf("reply %q does not confirm %q", reply, want)
	}
}

func TestRecurringAddReadsOptionsBeforeTheContent(t *testing.T) {
	f, dispatcher := recurringFixture(t)
	f.chat.IsForum = true

	aispamProcessUpdate(t, dispatcher, f.bot, f.chat, f.admin, 1002,
		"/recurring add 1d start=09:30 end=2026-12-31 delete=on topic=12 Good morning")

	rows := recurringRowsFor(t, f.chat.Id)
	if len(rows) != 1 {
		t.Fatalf("stored %d rows, want 1", len(rows))
	}
	row := rows[0]
	// 09:30 has passed on the 21st at 12:00 UTC, so the first run is the 22nd.
	if want := time.Date(2026, 10, 22, 9, 30, 0, 0, time.UTC); !row.NextRunAt.Equal(want) {
		t.Fatalf("NextRunAt = %v, want %v", row.NextRunAt, want)
	}
	if row.EndsAt == nil || !row.EndsAt.Equal(time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)) {
		t.Fatalf("EndsAt = %v, want the end of 2026-12-31", row.EndsAt)
	}
	if !row.DeletePrevious || row.ThreadID != 12 {
		t.Fatalf("row = %+v, want delete-previous in topic 12", row)
	}
	if strings.Contains(row.Text, "=") || !strings.Contains(row.Text, "Good morning") {
		t.Fatalf("stored text %q, want only the content", row.Text)
	}
}

func TestRecurringAddPostsToTheTopicItIsSentIn(t *testing.T) {
	f, dispatcher := recurringFixture(t)

	update := aispamMessageUpdate(f.chat, f.admin, 1003, "/recurring add 2h Topic reminder")
	update.Message.IsTopicMessage = true
	update.Message.MessageThreadId = 7
	aispamProcessRawUpdate(t, dispatcher, f.bot, update)

	rows := recurringRowsFor(t, f.chat.Id)
	if len(rows) != 1 || rows[0].ThreadID != 7 {
		t.Fatalf("rows = %+v, want one message in topic 7", rows)
	}
}

func TestRecurringAddTakesTheRepliedMedia(t *testing.T) {
	f, dispatcher := recurringFixture(t)

	update := aispamMessageUpdate(f.chat, f.admin, 1004, "/recurring add 12h")
	update.Message.ReplyToMessage = &gotgbot.Message{
		MessageId: 1000,
		Chat:      f.chat,
		Photo:     []gotgbot.PhotoSize{{FileId: "recurring-photo", FileUniqueId: "u1", Width: 10, Height: 10}},
		Caption:   "Weekly photo",
	}
	aispamProcessRawUpdate(t, dispatcher, f.bot, update)

	rows := recurringRowsFor(t, f.chat.Id)
	if len(rows) != 1 {
		t.Fatalf("stored %d rows, want 1", len(rows))
	}
	if rows[0].MsgType != db.PHOTO || rows[0].FileID != "recurring-photo" || !strings.Contains(rows[0].Text, "Weekly photo") {
		t.Fatalf("row = %+v, want the replied photo with its caption", rows[0])
	}
}

func TestRecurringAddRefusesBadInput(t *testing.T) {
	cases := []struct {
		name    string
		text    string
		wantKey string
		params  i18n.TranslationParams
	}{
		{"interval under an hour", "/recurring add 30m Too often", "recurring_interval_too_short", nil},
		{"bad option value", "/recurring add 6h start=25:00 Hello", "recurring_invalid_option", i18n.TranslationParams{"option": "start=25:00"}},
		{"topic outside a forum", "/recurring add 6h topic=12 Hello", "recurring_invalid_option", i18n.TranslationParams{"option": "topic=12"}},
		{"no content", "/recurring add 6h", "recurring_need_content", nil},
		{"only options", "/recurring add 6h delete=on", "recurring_need_content", nil},
		{"end before the first run", "/recurring add 1d end=2026-10-21T18:00 Hello", "recurring_end_before_start", nil},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, dispatcher := recurringFixture(t)

			aispamProcessUpdate(t, dispatcher, f.bot, f.chat, f.admin, int64(1010+i), tc.text)

			if rows := recurringRowsFor(t, f.chat.Id); len(rows) != 0 {
				t.Fatalf("stored %+v, want nothing", rows)
			}
			reply := lastMessageTo(t, f.client, f.chat.Id)
			if want := translatedWith(t, tc.wantKey, tc.params); reply != want {
				t.Fatalf("reply = %q, want %q", reply, want)
			}
		})
	}
}

func TestRecurringAddStopsAtTheLimit(t *testing.T) {
	f, dispatcher := recurringFixture(t)
	for i := 0; i < recurringmessages.MaxPerChat; i++ {
		seedRecurring(t, f.chat.Id, fmt.Sprintf("seed %d", i))
	}

	aispamProcessUpdate(t, dispatcher, f.bot, f.chat, f.admin, 1020, "/recurring add 6h One too many")

	if rows := recurringRowsFor(t, f.chat.Id); len(rows) != recurringmessages.MaxPerChat {
		t.Fatalf("stored %d rows, want %d", len(rows), recurringmessages.MaxPerChat)
	}
	reply := lastMessageTo(t, f.client, f.chat.Id)
	if want := translatedWith(t, "recurring_limit_reached", i18n.TranslationParams{"max": recurringmessages.MaxPerChat}); reply != want {
		t.Fatalf("reply = %q, want %q", reply, want)
	}
}

func TestRecurringRefusesNonAdmin(t *testing.T) {
	f, dispatcher := recurringFixture(t)
	seedRecurring(t, f.chat.Id, "Secret schedule")

	aispamProcessUpdate(t, dispatcher, f.bot, f.chat, f.member, 1030, "/recurring add 6h Member spam")
	aispamProcessUpdate(t, dispatcher, f.bot, f.chat, f.member, 1031, "/recurring list")
	rows := recurringRowsFor(t, f.chat.Id)
	aispamProcessUpdate(t, dispatcher, f.bot, f.chat, f.member, 1032, fmt.Sprintf("/recurring rm %d", rows[0].ID))

	if rows := recurringRowsFor(t, f.chat.Id); len(rows) != 1 || rows[0].Text != "Secret schedule" {
		t.Fatalf("rows = %+v, want only the seeded message", rows)
	}
	for _, text := range spamDBMessagesTo(f.client, f.chat.Id) {
		if strings.Contains(text, "Secret schedule") {
			t.Fatalf("a member saw the list: %q", text)
		}
	}
}

func TestRecurringListShowsTheChatsMessages(t *testing.T) {
	f, dispatcher := recurringFixture(t)

	aispamProcessUpdate(t, dispatcher, f.bot, f.chat, f.admin, 1040, "/recurring")
	if reply, want := lastMessageTo(t, f.client, f.chat.Id), translatedWith(t, "recurring_list_empty", nil); reply != want {
		t.Fatalf("empty list reply = %q, want %q", reply, want)
	}

	first := seedRecurring(t, f.chat.Id, "First <b>reminder</b>")
	second := seedRecurring(t, f.chat.Id, "Second reminder")
	other := newAdminChatFixture(t)
	t.Cleanup(func() { db.DB.Where("chat_id = ?", other.chat.Id).Delete(&models.RecurringMessage{}) })
	seedRecurring(t, other.chat.Id, "Another chat's reminder")

	aispamProcessUpdate(t, dispatcher, f.bot, f.chat, f.admin, 1041, "/recurring list")

	reply := lastMessageTo(t, f.client, f.chat.Id)
	wantParts := []string{
		translatedWith(t, "recurring_list_title", i18n.TranslationParams{"count": 2, "max": recurringmessages.MaxPerChat}),
		fmt.Sprintf("#%d", first.ID),
		fmt.Sprintf("#%d", second.ID),
		"First reminder",
		"Second reminder",
	}
	for _, want := range wantParts {
		if !strings.Contains(reply, want) {
			t.Fatalf("list %q does not contain %q", reply, want)
		}
	}
	if strings.Contains(reply, "Another chat") {
		t.Fatalf("list %q shows another chat's message", reply)
	}
}

func TestRecurringRemoveDeletesOnlyThisChatsMessage(t *testing.T) {
	f, dispatcher := recurringFixture(t)
	mine := seedRecurring(t, f.chat.Id, "Mine")
	other := newAdminChatFixture(t)
	t.Cleanup(func() { db.DB.Where("chat_id = ?", other.chat.Id).Delete(&models.RecurringMessage{}) })
	theirs := seedRecurring(t, other.chat.Id, "Theirs")

	aispamProcessUpdate(t, dispatcher, f.bot, f.chat, f.admin, 1050, fmt.Sprintf("/recurring rm %d", theirs.ID))
	if reply, want := lastMessageTo(t, f.client, f.chat.Id), translatedWith(t, "recurring_not_found", i18n.TranslationParams{"id": theirs.ID}); reply != want {
		t.Fatalf("reply = %q, want %q", reply, want)
	}
	if rows := recurringRowsFor(t, other.chat.Id); len(rows) != 1 {
		t.Fatalf("the other chat has %d rows, want 1", len(rows))
	}

	aispamProcessUpdate(t, dispatcher, f.bot, f.chat, f.admin, 1051, fmt.Sprintf("/recurring rm #%d", mine.ID))
	if reply, want := lastMessageTo(t, f.client, f.chat.Id), translatedWith(t, "recurring_removed", i18n.TranslationParams{"id": mine.ID}); reply != want {
		t.Fatalf("reply = %q, want %q", reply, want)
	}
	if rows := recurringRowsFor(t, f.chat.Id); len(rows) != 0 {
		t.Fatalf("rows = %+v after removal, want none", rows)
	}
}
