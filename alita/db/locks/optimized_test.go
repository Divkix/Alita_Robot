//go:build testtools

package locks

import (
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

func TestGetChatLocksOptimized(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	lockTypes := []string{"text", "photo", "video"}
	t.Cleanup(func() {
		for _, lt := range lockTypes {
			_ = db.DB.Where("chat_id = ? AND lock_type = ?", chatID, lt).Delete(&models.LockSettings{}).Error
		}
	})

	// Insert 3 locks
	for _, lt := range lockTypes {
		if err := db.DB.Create(&models.LockSettings{ChatId: chatID, LockType: lt, Locked: true}).Error; err != nil {
			t.Fatalf("DB.Create() lock %q error = %v", lt, err)
		}
	}

	// Get all locks for chatID -> map with 3 entries
	result, err := GetChatLocksOptimized(chatID)
	if err != nil {
		t.Fatalf("GetChatLocksOptimized() error = %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("GetChatLocksOptimized() len = %d, want 3", len(result))
	}
	for _, lt := range lockTypes {
		if !result[lt] {
			t.Fatalf("GetChatLocksOptimized() lock %q = false, want true", lt)
		}
	}

	// Different chatID -> empty map
	differentChatID := chatID + 1
	emptyResult, err := GetChatLocksOptimized(differentChatID)
	if err != nil {
		t.Fatalf("GetChatLocksOptimized() different chat error = %v", err)
	}
	if len(emptyResult) != 0 {
		t.Fatalf("GetChatLocksOptimized() different chat len = %d, want 0", len(emptyResult))
	}
}
