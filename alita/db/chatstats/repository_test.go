//go:build testtools

package chatstats

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db/models"
)

var testChatIDSeq atomic.Int64

// testChatID returns a chat ID no other test in this package uses, so rows
// from an earlier test cannot leak into this one.
func testChatID(t *testing.T) int64 {
	t.Helper()
	return time.Now().UnixNano() + testChatIDSeq.Add(1)
}

func mustAddDaily(t *testing.T, rows ...models.ChatDailyStat) {
	t.Helper()
	if err := AddDaily(rows); err != nil {
		t.Fatalf("AddDaily(%v) error = %v", rows, err)
	}
}

func mustGetRange(t *testing.T, chatID int64, from, to string) []models.ChatDailyStat {
	t.Helper()
	rows, err := GetRange(chatID, from, to)
	if err != nil {
		t.Fatalf("GetRange(%d, %q, %q) error = %v", chatID, from, to, err)
	}
	return rows
}

func TestAddDailySumsTwoReplicaFlushes(t *testing.T) {
	chatID := testChatID(t)

	// Each call stands in for one replica flushing the same day.
	mustAddDaily(t, models.ChatDailyStat{ChatID: chatID, Day: "2026-10-21", Messages: 2, Joins: 1})
	mustAddDaily(t, models.ChatDailyStat{ChatID: chatID, Day: "2026-10-21", Messages: 3, Leaves: 1})

	rows := mustGetRange(t, chatID, "2026-10-21", "2026-10-21")
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	got := rows[0]
	if got.Messages != 5 || got.Joins != 1 || got.Leaves != 1 {
		t.Fatalf("row = messages:%d joins:%d leaves:%d, want 5/1/1", got.Messages, got.Joins, got.Leaves)
	}
}

func TestGetRangeIsInclusiveAndScopedToTheChat(t *testing.T) {
	chatID := testChatID(t)
	otherChatID := chatID + 1000
	for _, day := range []string{"2026-10-10", "2026-10-11", "2026-10-12"} {
		mustAddDaily(t, models.ChatDailyStat{ChatID: chatID, Day: day, Messages: 1})
	}
	mustAddDaily(t, models.ChatDailyStat{ChatID: otherChatID, Day: "2026-10-11", Messages: 9})

	rows := mustGetRange(t, chatID, "2026-10-11", "2026-10-12")
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2 (bounds inclusive)", len(rows))
	}
	if rows[0].Day != "2026-10-11" || rows[1].Day != "2026-10-12" {
		t.Fatalf("days = %q, %q, want 2026-10-11, 2026-10-12", rows[0].Day, rows[1].Day)
	}

	single := mustGetRange(t, chatID, "2026-10-10", "2026-10-10")
	if len(single) != 1 || single[0].Messages != 1 {
		t.Fatalf("single-day rows = %v, want one row with messages 1", single)
	}
}

func TestDeleteBeforeKeepsRowsInsideRetention(t *testing.T) {
	chatID := testChatID(t)
	otherChatID := chatID + 1
	for _, row := range []models.ChatDailyStat{
		{ChatID: chatID, Day: "2025-01-01", Messages: 4},
		{ChatID: chatID, Day: "2026-10-20", Messages: 2},
		{ChatID: otherChatID, Day: "2025-01-01", Messages: 1},
	} {
		mustAddDaily(t, row)
	}

	deleted, err := DeleteBefore("2025-09-16")
	if err != nil {
		t.Fatalf("DeleteBefore() error = %v", err)
	}
	if deleted != 2 {
		t.Fatalf("deleted = %d, want 2", deleted)
	}

	if rows := mustGetRange(t, chatID, "2025-01-01", "2026-12-31"); len(rows) != 1 || rows[0].Day != "2026-10-20" {
		t.Fatalf("remaining chat rows = %v, want only 2026-10-20", rows)
	}
	if rows := mustGetRange(t, otherChatID, "2025-01-01", "2026-12-31"); len(rows) != 0 {
		t.Fatalf("remaining other chat rows = %v, want none", rows)
	}
}

func TestAddDailyEmptyTouchesNothing(t *testing.T) {
	chatID := testChatID(t)
	if err := AddDaily(nil); err != nil {
		t.Fatalf("AddDaily(nil) error = %v", err)
	}
	if rows := mustGetRange(t, chatID, "0000-01-01", "9999-12-31"); len(rows) != 0 {
		t.Fatalf("rows = %v, want none", rows)
	}
}
