package moderation

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

var lastEventChat atomic.Int64

// newEventChat returns a chat id no other test uses, so one test's rows never
// reach another's assertions. Back-to-back clock reads can repeat, so the id
// comes from a counter seeded once from the clock.
func newEventChat() int64 {
	lastEventChat.CompareAndSwap(0, time.Now().UnixNano())
	return lastEventChat.Add(1)
}

// deleteEventsFor removes a test chat's rows when the test ends.
func deleteEventsFor(t *testing.T, chatID int64) {
	t.Helper()
	t.Cleanup(func() {
		if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.ModerationEvent{}).Error; err != nil {
			t.Errorf("delete events of chat %d: %v", chatID, err)
		}
	})
}

func recordEvent(t *testing.T, ev models.ModerationEvent) {
	t.Helper()
	if err := Record(ev); err != nil {
		t.Fatalf("Record(%+v) error = %v", ev, err)
	}
}

func eventsForChat(t *testing.T, chatID int64) []models.ModerationEvent {
	t.Helper()
	var rows []models.ModerationEvent
	if err := db.DB.Where("chat_id = ?", chatID).Order("id").Find(&rows).Error; err != nil {
		t.Fatalf("load events of chat %d: %v", chatID, err)
	}
	return rows
}

// assertUTCOffset proves Record wrote a UTC instant: SQLite stores the offset
// it was given, so the value read back carries it. PostgreSQL normalises a
// timestamptz on storage and returns it in the session zone, which says
// nothing about what Record wrote, so the check applies to SQLite alone.
func assertUTCOffset(t *testing.T, got time.Time) {
	t.Helper()
	if db.DB.Name() != "sqlite" {
		return
	}
	if _, offset := got.Zone(); offset != 0 {
		t.Fatalf("timestamp %v has UTC offset %d, want 0", got, offset)
	}
}

func TestRecordRoundTripsTheEvent(t *testing.T) {
	chatID := newEventChat()
	deleteEventsFor(t, chatID)
	want := models.ModerationEvent{
		ChatId:       chatID,
		ActorId:      77,
		TargetUserId: 42,
		Action:       models.ModerationActionBan,
		Reason:       "spam",
		CreatedAt:    time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC),
	}

	recordEvent(t, want)

	rows := eventsForChat(t, chatID)
	if len(rows) != 1 {
		t.Fatalf("stored events = %d, want 1", len(rows))
	}
	got := rows[0]
	if got.ID == 0 {
		t.Fatal("stored event has no id")
	}
	if got.ChatId != want.ChatId || got.ActorId != want.ActorId || got.TargetUserId != want.TargetUserId ||
		got.Action != want.Action || got.Reason != want.Reason {
		t.Fatalf("stored event = %+v, want %+v", got, want)
	}
	if !got.CreatedAt.Equal(want.CreatedAt) {
		t.Fatalf("created_at = %v, want %v", got.CreatedAt, want.CreatedAt)
	}
	assertUTCOffset(t, got.CreatedAt)
}

func TestRecordStampsAMissingTimestampInUTC(t *testing.T) {
	chatID := newEventChat()
	deleteEventsFor(t, chatID)
	before := time.Now().UTC()

	recordEvent(t, models.ModerationEvent{
		ChatId:       chatID,
		ActorId:      77,
		TargetUserId: 42,
		Action:       models.ModerationActionKick,
	})

	rows := eventsForChat(t, chatID)
	if len(rows) != 1 {
		t.Fatalf("stored events = %d, want 1", len(rows))
	}
	got := rows[0].CreatedAt
	if got.Before(before.Add(-time.Second)) || got.After(before.Add(time.Minute)) {
		t.Fatalf("created_at = %v, want the record time around %v", got, before)
	}
	assertUTCOffset(t, got)
}

func TestRecordBoundsTheReasonAtARuneBoundary(t *testing.T) {
	chatID := newEventChat()
	deleteEventsFor(t, chatID)
	recordEvent(t, models.ModerationEvent{
		ChatId:       chatID,
		ActorId:      77,
		TargetUserId: 42,
		Action:       models.ModerationActionMute,
		Reason:       strings.Repeat("é", models.MaxModerationReasonRunes+1),
	})

	rows := eventsForChat(t, chatID)
	if len(rows) != 1 {
		t.Fatalf("stored events = %d, want 1", len(rows))
	}
	if want := strings.Repeat("é", models.MaxModerationReasonRunes); rows[0].Reason != want {
		t.Fatalf("stored reason is %d runes, want the first %d whole runes",
			utf8.RuneCountInString(rows[0].Reason), models.MaxModerationReasonRunes)
	}
	if !utf8.ValidString(rows[0].Reason) {
		t.Fatal("stored reason is not valid UTF-8")
	}
}

func TestListForChatReturnsNewestFirstInsideTheCutoff(t *testing.T) {
	chatID := newEventChat()
	otherChatID := newEventChat()
	deleteEventsFor(t, chatID)
	deleteEventsFor(t, otherChatID)
	now := time.Now().UTC()
	for _, ev := range []models.ModerationEvent{
		{ChatId: chatID, ActorId: 1, TargetUserId: 42, Action: models.ModerationActionBan, CreatedAt: now.Add(-48 * time.Hour)},
		{ChatId: chatID, ActorId: 1, TargetUserId: 42, Action: models.ModerationActionKick, CreatedAt: now.Add(-2 * time.Hour)},
		{ChatId: chatID, ActorId: 1, TargetUserId: 42, Action: models.ModerationActionMute, CreatedAt: now.Add(-time.Hour)},
		{ChatId: otherChatID, ActorId: 1, TargetUserId: 42, Action: models.ModerationActionBan, CreatedAt: now.Add(-time.Hour)},
	} {
		recordEvent(t, ev)
	}

	rows, err := ListForChat(chatID, now.Add(-24*time.Hour), 0)
	if err != nil {
		t.Fatalf("ListForChat() error = %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("ListForChat() returned %d events, want the 2 inside the cutoff", len(rows))
	}
	if rows[0].Action != models.ModerationActionMute || rows[1].Action != models.ModerationActionKick {
		t.Fatalf("ListForChat() order = %q, %q, want newest first", rows[0].Action, rows[1].Action)
	}
	for _, row := range rows {
		if row.ChatId != chatID {
			t.Fatalf("ListForChat() returned an event of chat %d, want only %d", row.ChatId, chatID)
		}
	}

	limited, err := ListForChat(chatID, now.Add(-24*time.Hour), 1)
	if err != nil {
		t.Fatalf("ListForChat(limit 1) error = %v", err)
	}
	if len(limited) != 1 || limited[0].Action != models.ModerationActionMute {
		t.Fatalf("ListForChat(limit 1) = %+v, want only the newest event", limited)
	}
}

func TestListForTargetReturnsNewestFirstInsideTheCutoff(t *testing.T) {
	target := int64(424242)
	otherTarget := int64(434343)
	firstChatID := newEventChat()
	secondChatID := newEventChat()
	deleteEventsFor(t, firstChatID)
	deleteEventsFor(t, secondChatID)
	now := time.Now().UTC()
	for _, ev := range []models.ModerationEvent{
		{ChatId: firstChatID, ActorId: 1, TargetUserId: target, Action: models.ModerationActionBan, CreatedAt: now.Add(-48 * time.Hour)},
		{ChatId: secondChatID, ActorId: 1, TargetUserId: target, Action: models.ModerationActionKick, CreatedAt: now.Add(-2 * time.Hour)},
		{ChatId: firstChatID, ActorId: 1, TargetUserId: target, Action: models.ModerationActionMute, CreatedAt: now.Add(-time.Hour)},
		{ChatId: secondChatID, ActorId: 1, TargetUserId: otherTarget, Action: models.ModerationActionBan, CreatedAt: now.Add(-time.Hour)},
	} {
		recordEvent(t, ev)
	}

	rows, err := ListForTarget(target, now.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("ListForTarget() error = %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("ListForTarget() returned %d events, want the 2 inside the cutoff", len(rows))
	}
	if rows[0].ChatId != firstChatID || rows[0].Action != models.ModerationActionMute {
		t.Fatalf("ListForTarget() first row = %+v, want the newest event of the first chat", rows[0])
	}
	if rows[1].ChatId != secondChatID || rows[1].Action != models.ModerationActionKick {
		t.Fatalf("ListForTarget() second row = %+v, want the older event of the second chat", rows[1])
	}
	for _, row := range rows {
		if row.TargetUserId != target {
			t.Fatalf("ListForTarget() returned target %d, want only %d", row.TargetUserId, target)
		}
	}
}

func TestDeleteExpiredBeforeRemovesOnlyOldEvents(t *testing.T) {
	oldChatID := newEventChat()
	freshChatID := newEventChat()
	deleteEventsFor(t, oldChatID)
	deleteEventsFor(t, freshChatID)
	now := time.Now().UTC()
	recordEvent(t, models.ModerationEvent{
		ChatId: oldChatID, ActorId: 1, TargetUserId: 42,
		Action: models.ModerationActionBan, CreatedAt: now.Add(-181 * 24 * time.Hour),
	})
	recordEvent(t, models.ModerationEvent{
		ChatId: freshChatID, ActorId: 1, TargetUserId: 42,
		Action: models.ModerationActionBan, CreatedAt: now.Add(-179 * 24 * time.Hour),
	})

	deleted, err := DeleteExpiredBefore(now.Add(-180 * 24 * time.Hour))
	if err != nil {
		t.Fatalf("DeleteExpiredBefore() error = %v", err)
	}
	// Other packages may share a PostgreSQL test database, so only our own
	// rows are checked strictly; the old event must be among the deleted.
	if deleted < 1 {
		t.Fatalf("DeleteExpiredBefore() deleted = %d, want the old event removed", deleted)
	}
	if rows := eventsForChat(t, oldChatID); len(rows) != 0 {
		t.Fatalf("old chat still has %d events, want none", len(rows))
	}
	if rows := eventsForChat(t, freshChatID); len(rows) != 1 {
		t.Fatalf("fresh chat has %d events, want the one inside retention", len(rows))
	}
}
