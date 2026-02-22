package db

import (
	"sync"
	"testing"
	"time"
)

func TestUpdateLockCreatesNewRecord(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	perm := "sticker"

	t.Cleanup(func() {
		_ = DB.Where("chat_id = ? AND lock_type = ?", chatID, perm).Delete(&LockSettings{}).Error
	})

	// First-time lock creation — this was the bug: silently did nothing
	err := UpdateLock(chatID, perm, true)
	if err != nil {
		t.Fatalf("UpdateLock() error = %v", err)
	}

	// Verify the record was actually created
	var lock LockSettings
	err = DB.Where("chat_id = ? AND lock_type = ?", chatID, perm).First(&lock).Error
	if err != nil {
		t.Fatalf("expected lock record to exist, got error: %v", err)
	}

	if !lock.Locked {
		t.Fatalf("expected Locked=true, got false")
	}
}

func TestUpdateLockHandlesZeroValueBoolean(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	perm := "url"

	t.Cleanup(func() {
		_ = DB.Where("chat_id = ? AND lock_type = ?", chatID, perm).Delete(&LockSettings{}).Error
	})

	// Create with Locked=true
	if err := UpdateLock(chatID, perm, true); err != nil {
		t.Fatalf("UpdateLock(true) error = %v", err)
	}

	var lock LockSettings
	if err := DB.Where("chat_id = ? AND lock_type = ?", chatID, perm).First(&lock).Error; err != nil {
		t.Fatalf("First lock query error: %v", err)
	}
	if !lock.Locked {
		t.Fatalf("expected Locked=true after first call")
	}

	// Update to Locked=false — zero value must be persisted
	if err := UpdateLock(chatID, perm, false); err != nil {
		t.Fatalf("UpdateLock(false) error = %v", err)
	}

	if err := DB.Where("chat_id = ? AND lock_type = ?", chatID, perm).First(&lock).Error; err != nil {
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
		_ = DB.Where("chat_id = ? AND lock_type = ?", chatID, perm).Delete(&LockSettings{}).Error
	})

	// Call 3 times with same value
	for i := range 3 {
		if err := UpdateLock(chatID, perm, true); err != nil {
			t.Fatalf("UpdateLock() call %d error = %v", i+1, err)
		}
	}

	// Should produce exactly 1 record
	var count int64
	DB.Model(&LockSettings{}).Where("chat_id = ? AND lock_type = ?", chatID, perm).Count(&count)
	if count != 1 {
		t.Fatalf("expected exactly 1 lock record, got %d", count)
	}
}

func TestUpdateLockConcurrentCreation(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	perm := "photo"

	t.Cleanup(func() {
		_ = DB.Where("chat_id = ? AND lock_type = ?", chatID, perm).Delete(&LockSettings{}).Error
	})

	const workers = 10
	var wg sync.WaitGroup
	wg.Add(workers)

	errs := make(chan error, workers)

	for range workers {
		go func() {
			defer wg.Done()
			if err := UpdateLock(chatID, perm, true); err != nil {
				errs <- err
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("UpdateLock() concurrent error: %v", err)
	}

	// All goroutines should converge to exactly 1 record
	var count int64
	DB.Model(&LockSettings{}).Where("chat_id = ? AND lock_type = ?", chatID, perm).Count(&count)
	if count != 1 {
		t.Fatalf("expected exactly 1 lock record after concurrent writes, got %d", count)
	}

	var lock LockSettings
	if err := DB.Where("chat_id = ? AND lock_type = ?", chatID, perm).First(&lock).Error; err != nil {
		t.Fatalf("query error: %v", err)
	}
	if !lock.Locked {
		t.Fatalf("expected Locked=true, got false")
	}
}
