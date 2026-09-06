//go:build testtools

package locks

import (
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

func skipIfNoDb(t *testing.T) {
	if db.DB == nil {
		t.Skip("DB not initialized")
	}
}

func TestUpdateLockHandlesZeroValueBoolean(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	perm := "url"

	t.Cleanup(func() {
		if err := db.DB.Where("chat_id = ? AND lock_type = ?", chatID, perm).Delete(&models.LockSettings{}).Error; err != nil {
			t.Fatalf("cleanup Delete error: %v", err)
		}
	})

	if err := UpdateLock(chatID, perm, true); err != nil {
		t.Fatalf("UpdateLock(true) error = %v", err)
	}

	var lock models.LockSettings
	if err := db.DB.Where("chat_id = ? AND lock_type = ?", chatID, perm).First(&lock).Error; err != nil {
		t.Fatalf("First lock query error: %v", err)
	}
	if !lock.Locked {
		t.Fatalf("expected Locked=true after first call")
	}

	if err := UpdateLock(chatID, perm, false); err != nil {
		t.Fatalf("UpdateLock(false) error = %v", err)
	}

	if err := db.DB.Where("chat_id = ? AND lock_type = ?", chatID, perm).First(&lock).Error; err != nil {
		t.Fatalf("Second lock query error: %v", err)
	}
	if lock.Locked {
		t.Fatalf("expected Locked=false after update, got true")
	}
}

func TestUpdateLockIdempotent(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	perm := "forward"

	t.Cleanup(func() {
		if err := db.DB.Where("chat_id = ? AND lock_type = ?", chatID, perm).Delete(&models.LockSettings{}).Error; err != nil {
			t.Fatalf("cleanup Delete error: %v", err)
		}
	})

	for i := range 3 {
		if err := UpdateLock(chatID, perm, true); err != nil {
			t.Fatalf("UpdateLock() call %d error = %v", i+1, err)
		}
	}

	var count int64
	db.DB.Model(&models.LockSettings{}).Where("chat_id = ? AND lock_type = ?", chatID, perm).Count(&count)
	if count != 1 {
		t.Fatalf("expected exactly 1 lock record, got %d", count)
	}
}

func TestIsPermLocked(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	perm := "sticker"

	t.Cleanup(func() {
		if err := db.DB.Where("chat_id = ? AND lock_type = ?", chatID, perm).Delete(&models.LockSettings{}).Error; err != nil {
			t.Fatalf("cleanup Delete error: %v", err)
		}
	})

	if IsPermLocked(chatID, perm) {
		t.Fatal("IsPermLocked() = true for non-existent record, want false")
	}

	if err := UpdateLock(chatID, perm, true); err != nil {
		t.Fatalf("UpdateLock(true) error = %v", err)
	}

	if !IsPermLocked(chatID, perm) {
		t.Fatal("IsPermLocked() = false after locking, want true")
	}

	if err := UpdateLock(chatID, perm, false); err != nil {
		t.Fatalf("UpdateLock(false) error = %v", err)
	}

	if IsPermLocked(chatID, perm) {
		t.Fatal("IsPermLocked() = true after unlocking, want false")
	}
}

func TestGetChatLocks(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	lockTypes := []string{"text", "photo", "url"}

	t.Cleanup(func() {
		for _, lt := range lockTypes {
			if err := db.DB.Where("chat_id = ? AND lock_type = ?", chatID, lt).Delete(&models.LockSettings{}).Error; err != nil {
				t.Fatalf("cleanup Delete error: %v", err)
			}
		}
	})

	locks := GetChatLocks(chatID)
	if len(locks) != 0 {
		t.Fatalf("GetChatLocks() empty chat len = %d, want 0", len(locks))
	}

	for _, lt := range lockTypes {
		if err := UpdateLock(chatID, lt, true); err != nil {
			t.Fatalf("UpdateLock(%q, true) error = %v", lt, err)
		}
	}

	locks = GetChatLocks(chatID)
	if len(locks) != len(lockTypes) {
		t.Fatalf("GetChatLocks() len = %d, want %d", len(locks), len(lockTypes))
	}

	for _, lt := range lockTypes {
		if !locks[lt] {
			t.Fatalf("GetChatLocks()[%q] = false, want true", lt)
		}
	}
}

func TestGetChatLocksUsesMemoryCache(t *testing.T) {
	skipIfNoDb(t)
	cache.SetupTestMemoryMarshaler(t)

	chatID := time.Now().UnixNano()
	perm := "video"

	t.Cleanup(func() {
		if err := db.DB.Where("chat_id = ? AND lock_type = ?", chatID, perm).Delete(&models.LockSettings{}).Error; err != nil {
			t.Fatalf("cleanup Delete error: %v", err)
		}
	})

	if err := UpdateLock(chatID, perm, true); err != nil {
		t.Fatalf("UpdateLock(true) error = %v", err)
	}

	locks := GetChatLocks(chatID)
	if !locks[perm] {
		t.Fatalf("GetChatLocks() = %v, want locked video", locks)
	}

	if err := UpdateLock(chatID, perm, false); err != nil {
		t.Fatalf("UpdateLock(false) error = %v", err)
	}

	locks = GetChatLocks(chatID)
	if locks[perm] {
		t.Fatalf("GetChatLocks() after unlock = %v, want unlocked video", locks)
	}
}

func TestGetChatLocksCacheInvalidation(t *testing.T) {
	skipIfNoDb(t)
	cache.SetupTestMemoryMarshaler(t)

	chatID := time.Now().UnixNano()
	perm := "game"

	t.Cleanup(func() {
		if err := db.DB.Where("chat_id = ? AND lock_type = ?", chatID, perm).Delete(&models.LockSettings{}).Error; err != nil {
			t.Fatalf("cleanup Delete error: %v", err)
		}
	})

	if err := UpdateLock(chatID, perm, true); err != nil {
		t.Fatalf("UpdateLock(true) error = %v", err)
	}
	locks := GetChatLocks(chatID)
	if !locks[perm] {
		t.Fatalf("GetChatLocks() after first UpdateLock = %v, want locked %q", locks, perm)
	}

	if err := UpdateLock(chatID, perm, false); err != nil {
		t.Fatalf("UpdateLock(false) error = %v", err)
	}

	locks = GetChatLocks(chatID)
	if locks[perm] {
		t.Fatalf("GetChatLocks() after second UpdateLock = %v, want unlocked %q (cache invalidation failed)", locks, perm)
	}
}
