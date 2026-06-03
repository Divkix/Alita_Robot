//go:build testtools

package locks

import (
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

func TestGetLockStatus_NilDB(t *testing.T) {
	originalDB := db.DB
	db.DB = nil
	t.Cleanup(func() {
		db.DB = originalDB
	})

	locked, err := GetLockStatus(123, "text")
	if err == nil {
		t.Fatal("GetLockStatus() with nil db expected error, got nil")
	}
	if err.Error() != "database not initialized" {
		t.Fatalf("GetLockStatus() error = %q, want %q", err.Error(), "database not initialized")
	}
	if locked {
		t.Fatal("GetLockStatus() with nil db expected false, got true")
	}
}

func TestGetLockStatus(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	lockType := "text"

	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ? AND lock_type = ?", chatID, lockType).Delete(&models.LockSettings{}).Error
	})

	// No record -> default unlocked (false)
	locked, err := GetLockStatus(chatID, lockType)
	if err != nil {
		t.Fatalf("GetLockStatus() no record error = %v", err)
	}
	if locked {
		t.Fatalf("GetLockStatus() no record = %v, want false", locked)
	}

	// Insert a locked record
	if err := db.DB.Create(&models.LockSettings{ChatId: chatID, LockType: lockType, Locked: true}).Error; err != nil {
		t.Fatalf("DB.Create() lock error = %v", err)
	}

	locked, err = GetLockStatus(chatID, lockType)
	if err != nil {
		t.Fatalf("GetLockStatus() after insert error = %v", err)
	}
	if !locked {
		t.Fatalf("GetLockStatus() after insert = %v, want true", locked)
	}
}

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

func TestGetLockStatusCached(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	lockType := "text"

	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ? AND lock_type = ?", chatID, lockType).Delete(&models.LockSettings{}).Error
	})

	// Insert a locked record
	if err := db.DB.Create(&models.LockSettings{ChatId: chatID, LockType: lockType, Locked: true}).Error; err != nil {
		t.Fatalf("DB.Create() lock error = %v", err)
	}

	// First call should cache
	locked, err := GetLockStatusCached(chatID, lockType)
	if err != nil {
		t.Fatalf("GetLockStatusCached() error = %v", err)
	}
	if !locked {
		t.Fatalf("GetLockStatusCached() = %v, want true", locked)
	}

	// Second call should use cache
	locked, err = GetLockStatusCached(chatID, lockType)
	if err != nil {
		t.Fatalf("GetLockStatusCached() cached error = %v", err)
	}
	if !locked {
		t.Fatalf("GetLockStatusCached() cached = %v, want true", locked)
	}
}

func TestGetLockStatusCached_RecordNotFound(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	lockType := "text"

	// No record -> default unlocked (false)
	locked, err := GetLockStatusCached(chatID, lockType)
	if err != nil {
		t.Fatalf("GetLockStatusCached() no record error = %v", err)
	}
	if locked {
		t.Fatalf("GetLockStatusCached() no record = %v, want false", locked)
	}
}
