//go:build testtools

package chatstats

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
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

// dailyRows builds one row for each chat in base..base+count-1 on day.
func dailyRows(base int64, count int, day string) []models.ChatDailyStat {
	rows := make([]models.ChatDailyStat, 0, count)
	for i := range count {
		rows = append(rows, models.ChatDailyStat{
			ChatID:   base + int64(i),
			Day:      day,
			Messages: int64(i + 1),
			Joins:    int64(i % 7),
			Leaves:   int64(i % 3),
		})
	}
	return rows
}

// assertDailyRows checks the stored rows for base..base+count-1 against the
// counters dailyRows writes, multiplied by flushes.
func assertDailyRows(t *testing.T, base int64, count, flushes int, day string) {
	t.Helper()
	var rows []models.ChatDailyStat
	err := db.DB.
		Where("chat_id >= ? AND chat_id < ? AND day = ?", base, base+int64(count), day).
		Find(&rows).Error
	if err != nil {
		t.Fatalf("load daily rows: %v", err)
	}
	if len(rows) != count {
		t.Fatalf("rows = %d, want %d", len(rows), count)
	}
	for _, row := range rows {
		i := int(row.ChatID - base)
		wantMessages := int64(flushes * (i + 1))
		wantJoins := int64(flushes * (i % 7))
		wantLeaves := int64(flushes * (i % 3))
		if row.Messages != wantMessages || row.Joins != wantJoins || row.Leaves != wantLeaves {
			t.Fatalf("chat %d = messages:%d joins:%d leaves:%d, want %d/%d/%d",
				row.ChatID, row.Messages, row.Joins, row.Leaves, wantMessages, wantJoins, wantLeaves)
		}
	}
}

// TestAddDailyBatchesLargeFlush flushes more rows than one batch holds. Every
// row must land with its own counters, and a second flush of the same rows
// must add to them rather than replace them.
func TestAddDailyBatchesLargeFlush(t *testing.T) {
	const chats = 1201 // spans three batches of 500
	base := testChatID(t)
	day := "2026-10-21"
	rows := dailyRows(base, chats, day)

	if err := AddDaily(rows); err != nil {
		t.Fatalf("first AddDaily(%d rows) error = %v", len(rows), err)
	}
	assertDailyRows(t, base, chats, 1, day)

	if err := AddDaily(rows); err != nil {
		t.Fatalf("second AddDaily(%d rows) error = %v", len(rows), err)
	}
	assertDailyRows(t, base, chats, 2, day)
}

// TestAddDailyFlushPastBindParameterLimit is the regression guard for the
// single-statement insert: 7000 rows are 35000 bind parameters, more than
// SQLite accepts in one statement, so the flush must be split into batches.
func TestAddDailyFlushPastBindParameterLimit(t *testing.T) {
	const chats = 7000
	base := testChatID(t)
	if err := AddDaily(dailyRows(base, chats, "2026-10-22")); err != nil {
		t.Fatalf("AddDaily(%d rows) error = %v", chats, err)
	}
	assertDailyRows(t, base, chats, 1, "2026-10-22")
}
