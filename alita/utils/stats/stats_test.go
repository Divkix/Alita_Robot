//go:build testtools

package stats

import (
	"os"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/chatstats"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/internal/testdb"
)

func TestMain(m *testing.M) {
	os.Exit(testdb.Run(m, &models.ChatDailyStat{}, &models.ChatHourlyStat{}))
}

var testChatIDSeq atomic.Int64

func testChatID(t *testing.T) int64 {
	t.Helper()
	return time.Now().UnixNano() + testChatIDSeq.Add(1)
}

// flushPending writes whatever an earlier test left behind, so assertions
// here only see this test's events.
func flushPending(t *testing.T) {
	t.Helper()
	if err := Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
}

func pendingTotal() int64 {
	var total int64
	counters.Range(func(_, value any) bool {
		total += value.(*atomic.Int64).Load()
		return true
	})
	return total
}

func rowsFor(t *testing.T, chatID int64, from, to string) []models.ChatDailyStat {
	t.Helper()
	rows, err := chatstats.GetRange(chatID, from, to)
	if err != nil {
		t.Fatalf("GetRange(%d, %q, %q) error = %v", chatID, from, to, err)
	}
	return rows
}

func TestRecordStaysInMemoryUntilFlush(t *testing.T) {
	flushPending(t)
	chatID := testChatID(t)
	day := Day(timeNow())

	RecordMessage(chatID)
	RecordMessage(chatID)
	RecordJoin(chatID)

	if pendingTotal() != 3 {
		t.Fatalf("pending events = %d, want 3", pendingTotal())
	}
	if rows := rowsFor(t, chatID, day, day); len(rows) != 0 {
		t.Fatalf("rows before flush = %v, want none: the hot path must not write", rows)
	}

	if err := Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	rows := rowsFor(t, chatID, day, day)
	if len(rows) != 1 {
		t.Fatalf("rows after flush = %d, want 1", len(rows))
	}
	if rows[0].Messages != 2 || rows[0].Joins != 1 || rows[0].Leaves != 0 {
		t.Fatalf("row = %d/%d/%d, want messages 2, joins 1, leaves 0",
			rows[0].Messages, rows[0].Joins, rows[0].Leaves)
	}
	if left := pendingTotal(); left != 0 {
		t.Fatalf("pending events after flush = %d, want 0 (drained)", left)
	}
}

func TestSecondFlushWithNoEventsWritesNothing(t *testing.T) {
	flushPending(t)
	chatID := testChatID(t)
	day := Day(timeNow())

	RecordLeave(chatID)
	if err := Flush(); err != nil {
		t.Fatalf("first Flush() error = %v", err)
	}

	// Remove the row; a flush with nothing pending must not recreate it.
	if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatDailyStat{}).Error; err != nil {
		t.Fatalf("delete row: %v", err)
	}
	if err := Flush(); err != nil {
		t.Fatalf("second Flush() error = %v", err)
	}
	if rows := rowsFor(t, chatID, day, day); len(rows) != 0 {
		t.Fatalf("rows after empty flush = %v, want none", rows)
	}
}

func TestDayKeyIsUTCAndFollowsTheClock(t *testing.T) {
	previous := timeNow
	t.Cleanup(func() { timeNow = previous })

	// 01:30 in a UTC+2 zone is 23:30 of the previous UTC day.
	timeNow = func() time.Time {
		return time.Date(2026, 10, 22, 1, 30, 0, 0, time.FixedZone("UTC+2", 2*60*60))
	}
	flushPending(t)
	chatID := testChatID(t)

	if got := Day(timeNow()); got != "2026-10-21" {
		t.Fatalf("Day() = %q, want 2026-10-21", got)
	}
	RecordMessage(chatID)
	if err := Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	rows := rowsFor(t, chatID, "2026-10-21", "2026-10-21")
	if len(rows) != 1 || rows[0].Messages != 1 {
		t.Fatalf("rows = %v, want one row for the UTC day 2026-10-21", rows)
	}
	if rows := rowsFor(t, chatID, "2026-10-22", "2026-10-22"); len(rows) != 0 {
		t.Fatalf("rows for the local day = %v, want none", rows)
	}
}

func TestStopFlusherFlushesPendingCountersAndIsIdempotent(t *testing.T) {
	flushPending(t)
	chatID := testChatID(t)
	day := Day(time.Now())

	RecordJoin(chatID)
	StartFlusher()
	if err := StopFlusher(); err != nil {
		t.Fatalf("StopFlusher() error = %v", err)
	}
	if err := StopFlusher(); err != nil {
		t.Fatalf("second StopFlusher() error = %v", err)
	}

	rows := rowsFor(t, chatID, day, day)
	if len(rows) != 1 || rows[0].Joins != 1 {
		t.Fatalf("rows = %v, want one join", rows)
	}
	if left := pendingTotal(); left != 0 {
		t.Fatalf("pending events = %d, want 0", left)
	}
}

func TestFlushRestoresCountersWhenTheWriteFails(t *testing.T) {
	flushPending(t)
	chatID := testChatID(t)
	day := Day(timeNow())

	original := db.DB
	broken, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open broken database: %v", err)
	}
	t.Cleanup(func() { db.DB = original })

	RecordMessage(chatID)
	db.DB = broken
	if err := Flush(); err == nil {
		t.Fatal("Flush() on a database without the table = nil, want an error")
	}
	if left := pendingTotal(); left != 1 {
		t.Fatalf("pending events after a failed flush = %d, want 1 (restored)", left)
	}

	db.DB = original
	if err := Flush(); err != nil {
		t.Fatalf("retry Flush() error = %v", err)
	}
	rows := rowsFor(t, chatID, day, day)
	if len(rows) != 1 || rows[0].Messages != 1 {
		t.Fatalf("rows after retry = %v, want one message", rows)
	}
}

func TestFlushWritesMessagesPerUTCHour(t *testing.T) {
	previous := timeNow
	t.Cleanup(func() { timeNow = previous })

	// 12:15 in a UTC+2 zone is 10:15 UTC.
	clock := time.Date(2026, 10, 22, 12, 15, 0, 0, time.FixedZone("UTC+2", 2*60*60))
	timeNow = func() time.Time { return clock }
	flushPending(t)
	chatID := testChatID(t)

	RecordMessage(chatID)
	RecordMessage(chatID)
	clock = clock.Add(time.Hour)
	RecordMessage(chatID)
	RecordJoin(chatID)
	if err := Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	hourly, err := chatstats.GetHourlyRange(chatID, "2026-10-22", "2026-10-22")
	if err != nil {
		t.Fatalf("GetHourlyRange() error = %v", err)
	}
	if len(hourly) != 2 ||
		hourly[0].Hour != 10 || hourly[0].Messages != 2 ||
		hourly[1].Hour != 11 || hourly[1].Messages != 1 {
		t.Fatalf("hourly rows = %+v, want 2 messages at 10 UTC and 1 at 11 UTC", hourly)
	}

	daily := rowsFor(t, chatID, "2026-10-22", "2026-10-22")
	if len(daily) != 1 || daily[0].Messages != 3 || daily[0].Joins != 1 {
		t.Fatalf("daily rows = %+v, want one row with 3 messages and 1 join", daily)
	}
}
