//go:build testtools

package stats

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/chatstats"
)

func topFor(t *testing.T, chatID int64) map[int64]int64 {
	t.Helper()
	day := Day(timeNow())
	top, err := chatstats.TopMembers(chatID, day, day, 100)
	if err != nil {
		t.Fatalf("TopMembers(%d) error = %v", chatID, err)
	}
	byUser := make(map[int64]int64, len(top))
	for _, row := range top {
		byUser[row.UserID] = row.Messages
	}
	return byUser
}

func TestMemberMessagesStayInMemoryUntilFlush(t *testing.T) {
	flushPending(t)
	chatID := testChatID(t)

	RecordMemberMessage(chatID, 11)
	RecordMemberMessage(chatID, 11)
	RecordMemberMessage(chatID, 12)
	RecordMemberMessage(chatID, 0) // no user: never counted

	if got := topFor(t, chatID); len(got) != 0 {
		t.Fatalf("rows before flush = %v, want none: the hot path must not write", got)
	}
	if err := Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	got := topFor(t, chatID)
	if len(got) != 2 || got[11] != 2 || got[12] != 1 {
		t.Fatalf("per-member counts = %v, want user 11: 2, user 12: 1", got)
	}

	// A second flush with nothing new must not add again.
	if err := Flush(); err != nil {
		t.Fatalf("second Flush() error = %v", err)
	}
	if again := topFor(t, chatID); again[11] != 2 || again[12] != 1 {
		t.Fatalf("per-member counts after an empty flush = %v, want unchanged", again)
	}
}

func TestMemberCounterCapSkipsNewUsersUntilTheNextFlush(t *testing.T) {
	flushPending(t)
	previous := memberCounterLimit
	t.Cleanup(func() { memberCounterLimit = previous })
	memberCounterLimit = 2
	chatID := testChatID(t)

	RecordMemberMessage(chatID, 21)
	RecordMemberMessage(chatID, 22)
	RecordMemberMessage(chatID, 23) // over the cap: not counted
	RecordMemberMessage(chatID, 21) // already counted: still adds
	if err := Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	got := topFor(t, chatID)
	if len(got) != 2 || got[21] != 2 || got[22] != 1 {
		t.Fatalf("per-member counts = %v, want users 21 (2) and 22 (1) only", got)
	}

	// The flush emptied the map, so the user dropped before counts now.
	RecordMemberMessage(chatID, 23)
	if err := Flush(); err != nil {
		t.Fatalf("second Flush() error = %v", err)
	}
	if got := topFor(t, chatID); got[23] != 1 {
		t.Fatalf("per-member counts after the next flush = %v, want user 23 counted once", got)
	}
}

func TestFlushRestoresMemberCountersWhenTheWriteFails(t *testing.T) {
	flushPending(t)
	chatID := testChatID(t)

	original := db.DB
	broken, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open broken database: %v", err)
	}
	t.Cleanup(func() { db.DB = original })

	RecordMemberMessage(chatID, 31)
	db.DB = broken
	if err := Flush(); err == nil {
		t.Fatal("Flush() on a database without the tables = nil, want an error")
	}

	db.DB = original
	RecordMemberMessage(chatID, 31)
	if err := Flush(); err != nil {
		t.Fatalf("retry Flush() error = %v", err)
	}
	if got := topFor(t, chatID); got[31] != 2 {
		t.Fatalf("per-member counts after retry = %v, want user 31 with 2 (restored + new)", got)
	}
}
