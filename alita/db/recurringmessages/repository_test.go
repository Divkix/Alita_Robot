//go:build testtools

package recurringmessages

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

var testChatSeq atomic.Int64

// newRecurringChat returns a chat ID no other test in this package uses.
func newRecurringChat(t *testing.T) int64 {
	t.Helper()
	chatID := -(time.Now().UnixNano() + testChatSeq.Add(1))
	t.Cleanup(func() {
		if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.RecurringMessage{}).Error; err != nil {
			t.Errorf("cleanup recurring messages of chat %d: %v", chatID, err)
		}
	})
	return chatID
}

func newMessage(chatID int64, next time.Time) *models.RecurringMessage {
	return &models.RecurringMessage{
		ChatID:       chatID,
		Text:         "Read the rules",
		MsgType:      1,
		IntervalSecs: int64(time.Hour / time.Second),
		NextRunAt:    next,
		CreatedBy:    42,
	}
}

func mustAdd(t *testing.T, msg *models.RecurringMessage) {
	t.Helper()
	if err := Add(msg); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
}

func mustList(t *testing.T, chatID int64) []models.RecurringMessage {
	t.Helper()
	rows, err := List(chatID)
	if err != nil {
		t.Fatalf("List(%d) error = %v", chatID, err)
	}
	return rows
}

func mustGetRow(t *testing.T, id int64) models.RecurringMessage {
	t.Helper()
	var row models.RecurringMessage
	if err := db.DB.Where("id = ?", id).Take(&row).Error; err != nil {
		t.Fatalf("load recurring message %d: %v", id, err)
	}
	return row
}

func dueRow(t *testing.T, id int64, now time.Time) *models.RecurringMessage {
	t.Helper()
	rows, err := ListDue(now, 1000)
	if err != nil {
		t.Fatalf("ListDue() error = %v", err)
	}
	for i := range rows {
		if rows[i].ID == id {
			return &rows[i]
		}
	}
	return nil
}

func TestAddStoresTheMessageAndListShowsIt(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	chatID := newRecurringChat(t)

	if got := mustList(t, chatID); len(got) != 0 {
		t.Fatalf("List() before Add = %d rows, want 0", len(got))
	}

	msg := newMessage(chatID, time.Now().Add(time.Hour))
	msg.ThreadID = 7
	msg.DeletePrevious = true
	mustAdd(t, msg)
	if msg.ID == 0 {
		t.Fatal("Add() left the ID unset")
	}

	rows := mustList(t, chatID)
	if len(rows) != 1 {
		t.Fatalf("List() after Add = %d rows, want 1 (the cached empty list must be dropped)", len(rows))
	}
	got := rows[0]
	if got.ID != msg.ID || got.Text != "Read the rules" || got.ThreadID != 7 || !got.DeletePrevious || !got.Enabled {
		t.Fatalf("listed row = %+v, want the added message enabled", got)
	}
	if got.Interval() != time.Hour {
		t.Fatalf("interval = %v, want 1h", got.Interval())
	}
}

func TestAddRefusesPastTheChatLimit(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	chatID := newRecurringChat(t)

	for i := 0; i < MaxPerChat; i++ {
		mustAdd(t, newMessage(chatID, time.Now().Add(time.Hour)))
	}
	err := Add(newMessage(chatID, time.Now().Add(time.Hour)))
	if !errors.Is(err, ErrLimitReached) {
		t.Fatalf("Add() past the limit error = %v, want ErrLimitReached", err)
	}
	if got := mustList(t, chatID); len(got) != MaxPerChat {
		t.Fatalf("chat has %d messages, want %d", len(got), MaxPerChat)
	}

	// The limit is per chat.
	mustAdd(t, newMessage(newRecurringChat(t), time.Now().Add(time.Hour)))
}

func TestAddHoldsTheLimitUnderConcurrentAdds(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	chatID := newRecurringChat(t)

	var wg sync.WaitGroup
	var added, refused atomic.Int64
	for i := 0; i < MaxPerChat+3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := Add(newMessage(chatID, time.Now().Add(time.Hour)))
			switch {
			case err == nil:
				added.Add(1)
			case errors.Is(err, ErrLimitReached):
				refused.Add(1)
			default:
				t.Errorf("Add() error = %v", err)
			}
		}()
	}
	wg.Wait()

	if added.Load() != MaxPerChat || refused.Load() != 3 {
		t.Fatalf("added %d, refused %d; want %d and 3", added.Load(), refused.Load(), MaxPerChat)
	}
}

func TestAddRefusesAnIntervalUnderAnHour(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	chatID := newRecurringChat(t)

	msg := newMessage(chatID, time.Now().Add(time.Hour))
	msg.IntervalSecs = int64((59 * time.Minute) / time.Second)
	if err := Add(msg); !errors.Is(err, ErrIntervalTooShort) {
		t.Fatalf("Add() error = %v, want ErrIntervalTooShort", err)
	}
	if got := mustList(t, chatID); len(got) != 0 {
		t.Fatalf("chat has %d messages, want none", len(got))
	}
}

func TestRemoveDeletesOnlyTheChatsOwnMessage(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	chatID := newRecurringChat(t)
	otherChat := newRecurringChat(t)

	msg := newMessage(chatID, time.Now().Add(time.Hour))
	mustAdd(t, msg)
	other := newMessage(otherChat, time.Now().Add(time.Hour))
	mustAdd(t, other)
	_ = mustList(t, chatID) // fill the cache

	removed, err := Remove(chatID, other.ID)
	if err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if removed {
		t.Fatal("Remove() deleted another chat's message")
	}

	removed, err = Remove(chatID, msg.ID)
	if err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if !removed {
		t.Fatal("Remove() reported the chat's own message missing")
	}
	if got := mustList(t, chatID); len(got) != 0 {
		t.Fatalf("List() after Remove = %d rows, want 0", len(got))
	}
	if got := mustList(t, otherChat); len(got) != 1 {
		t.Fatalf("other chat has %d messages, want 1", len(got))
	}
}

func TestListDueReturnsOnlyEnabledDueMessages(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	chatID := newRecurringChat(t)
	now := time.Now().UTC().Truncate(time.Second)

	due := newMessage(chatID, now.Add(-time.Minute))
	mustAdd(t, due)
	notYet := newMessage(chatID, now.Add(time.Minute))
	mustAdd(t, notYet)
	stopped := newMessage(chatID, now.Add(-time.Minute))
	mustAdd(t, stopped)
	if err := Disable(chatID, stopped.ID); err != nil {
		t.Fatalf("Disable() error = %v", err)
	}

	if dueRow(t, due.ID, now) == nil {
		t.Fatal("ListDue() missed the due message")
	}
	if dueRow(t, notYet.ID, now) != nil {
		t.Fatal("ListDue() returned a message that is not due")
	}
	if dueRow(t, stopped.ID, now) != nil {
		t.Fatal("ListDue() returned a disabled message")
	}
}

func TestClaimLetsOnlyOneReplicaWin(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	chatID := newRecurringChat(t)
	now := time.Now().UTC()

	msg := newMessage(chatID, now.Add(-time.Minute))
	mustAdd(t, msg)

	// Two replicas read the same due row.
	first := dueRow(t, msg.ID, now)
	second := dueRow(t, msg.ID, now)
	if first == nil || second == nil {
		t.Fatal("message not due")
	}

	next := now.Add(time.Hour + 123*time.Millisecond)
	won, err := Claim(first, next, false)
	if err != nil || !won {
		t.Fatalf("first Claim() = %v, %v; want true", won, err)
	}
	won, err = Claim(second, next, false)
	if err != nil || won {
		t.Fatalf("second Claim() = %v, %v; want false", won, err)
	}

	if dueRow(t, msg.ID, now) != nil {
		t.Fatal("claimed message is still due")
	}
	row := mustGetRow(t, msg.ID)
	if !row.NextRunAt.Equal(next.Truncate(time.Second)) || !row.Enabled {
		t.Fatalf("after claim next_run_at = %v enabled = %v, want %v and enabled", row.NextRunAt, row.Enabled, next.Truncate(time.Second))
	}
}

func TestClaimOfTheLastOccurrenceStopsTheMessage(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	chatID := newRecurringChat(t)
	now := time.Now().UTC()

	msg := newMessage(chatID, now.Add(-time.Minute))
	mustAdd(t, msg)
	row := dueRow(t, msg.ID, now)

	won, err := Claim(row, now.Add(time.Hour), true)
	if err != nil || !won {
		t.Fatalf("Claim() = %v, %v; want true", won, err)
	}
	if got := mustList(t, chatID); len(got) != 1 || got[0].Enabled {
		t.Fatalf("List() after the last claim = %+v, want the message kept and stopped", got)
	}
	if dueRow(t, msg.ID, now.Add(2*time.Hour)) != nil {
		t.Fatal("a stopped message came due again")
	}
}

func TestRestoreMakesTheOccurrenceDueAgain(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	chatID := newRecurringChat(t)
	now := time.Now().UTC()

	msg := newMessage(chatID, now.Add(-time.Minute))
	mustAdd(t, msg)
	row := dueRow(t, msg.ID, now)

	next := now.Add(time.Hour)
	if won, err := Claim(row, next, true); err != nil || !won {
		t.Fatalf("Claim() = %v, %v; want true", won, err)
	}
	if err := Restore(row, next); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	again := dueRow(t, msg.ID, now)
	if again == nil {
		t.Fatal("restored message is not due")
	}
	if !again.Enabled {
		t.Fatal("restored message stayed stopped")
	}

	// A restore after another claim moved the row on does nothing.
	later := now.Add(2 * time.Hour)
	if won, err := Claim(again, later, false); err != nil || !won {
		t.Fatalf("second Claim() = %v, %v; want true", won, err)
	}
	if err := Restore(row, next); err != nil {
		t.Fatalf("stale Restore() error = %v", err)
	}
	if dueRow(t, msg.ID, now) != nil {
		t.Fatal("a stale restore undid a later claim")
	}
}

func TestSetLastMessageIDAndDisableShowInTheList(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	chatID := newRecurringChat(t)

	msg := newMessage(chatID, time.Now().Add(time.Hour))
	mustAdd(t, msg)
	_ = mustList(t, chatID) // fill the cache

	if err := SetLastMessageID(chatID, msg.ID, 555); err != nil {
		t.Fatalf("SetLastMessageID() error = %v", err)
	}
	if got := mustList(t, chatID); got[0].LastMessageID != 555 {
		t.Fatalf("last_message_id = %d, want 555", got[0].LastMessageID)
	}

	if err := Disable(chatID, msg.ID); err != nil {
		t.Fatalf("Disable() error = %v", err)
	}
	if got := mustList(t, chatID); got[0].Enabled {
		t.Fatal("disabled message still listed as enabled")
	}
}
