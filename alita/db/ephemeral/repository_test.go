//go:build testtools

package ephemeral

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

var testChatSeq atomic.Int64

// newEphemeralChat returns a chat ID no other test in this package uses.
func newEphemeralChat(t *testing.T) int64 {
	t.Helper()
	chatID := -(time.Now().UnixNano() + testChatSeq.Add(1))
	t.Cleanup(func() {
		if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.EphemeralSettings{}).Error; err != nil {
			t.Errorf("cleanup ephemeral settings of chat %d: %v", chatID, err)
		}
	})
	return chatID
}

func enabled(t *testing.T, chatID int64) bool {
	t.Helper()
	row, err := GetContext(t.Context(), chatID)
	if err != nil {
		t.Fatalf("GetContext(%d) error = %v", chatID, err)
	}
	return row.Enabled
}

func countRows(t *testing.T, chatID int64) int64 {
	t.Helper()
	var n int64
	if err := db.DB.Model(&models.EphemeralSettings{}).Where("chat_id = ?", chatID).Count(&n).Error; err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return n
}

func TestChatWithoutRowIsOffAndReadingInsertsNothing(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	chatID := newEphemeralChat(t)

	if enabled(t, chatID) {
		t.Fatal("missing row = on, want off")
	}
	if n := countRows(t, chatID); n != 0 {
		t.Fatalf("rows = %d, want 0: reading must not insert", n)
	}
}

func TestSetEnabledRoundTripsAndInvalidatesTheCache(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	chatID := newEphemeralChat(t)

	// Warm the cache with the default first, so a missed invalidation shows.
	if enabled(t, chatID) {
		t.Fatal("before SetEnabled = on, want off")
	}
	if err := SetEnabled(chatID, true); err != nil {
		t.Fatalf("SetEnabled(true) error = %v", err)
	}
	if !enabled(t, chatID) {
		t.Fatal("after SetEnabled(true) = off, want on")
	}
	// false must reach the row rather than the column default.
	if err := SetEnabled(chatID, false); err != nil {
		t.Fatalf("SetEnabled(false) error = %v", err)
	}
	if enabled(t, chatID) {
		t.Fatal("after SetEnabled(false) = on, want off")
	}
	if n := countRows(t, chatID); n != 1 {
		t.Fatalf("rows = %d, want 1 (writes must upsert)", n)
	}
}
