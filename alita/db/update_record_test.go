package db

import (
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestUpdateRecordReturnsErrorOnNoMatch(t *testing.T) {
	if err := DB.AutoMigrate(&LockSettings{}); err != nil {
		t.Fatalf("failed to migrate locks: %v", err)
	}

	// Use a chat ID that doesn't exist in the database
	nonExistentChatID := time.Now().UnixNano()

	err := UpdateRecord(
		&LockSettings{},
		LockSettings{ChatId: nonExistentChatID, LockType: "sticker"},
		map[string]any{"locked": true},
	)
	if err == nil {
		t.Fatalf("expected error for non-existent record, got nil")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected gorm.ErrRecordNotFound, got: %v", err)
	}
}

func TestUpdateRecordWithZeroValuesReturnsErrorOnNoMatch(t *testing.T) {
	if err := DB.AutoMigrate(&LockSettings{}); err != nil {
		t.Fatalf("failed to migrate locks: %v", err)
	}

	nonExistentChatID := time.Now().UnixNano()

	err := UpdateRecordWithZeroValues(
		&LockSettings{},
		LockSettings{ChatId: nonExistentChatID, LockType: "url"},
		map[string]any{"locked": false},
	)
	if err == nil {
		t.Fatalf("expected error for non-existent record, got nil")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected gorm.ErrRecordNotFound, got: %v", err)
	}
}

func TestUpdateRecordWithZeroValuesUpdatesZeroValues(t *testing.T) {
	if err := DB.AutoMigrate(&LockSettings{}); err != nil {
		t.Fatalf("failed to migrate locks: %v", err)
	}

	chatID := time.Now().UnixNano()
	perm := "photo"

	t.Cleanup(func() {
		_ = DB.Where("chat_id = ? AND lock_type = ?", chatID, perm).Delete(&LockSettings{}).Error
	})

	// Create a record with Locked=true
	if err := DB.Create(&LockSettings{ChatId: chatID, LockType: perm, Locked: true}).Error; err != nil {
		t.Fatalf("failed to create test record: %v", err)
	}

	// Update to Locked=false using UpdateRecordWithZeroValues
	err := UpdateRecordWithZeroValues(
		&LockSettings{},
		LockSettings{ChatId: chatID, LockType: perm},
		map[string]any{"locked": false},
	)
	if err != nil {
		t.Fatalf("UpdateRecordWithZeroValues() error = %v", err)
	}

	var lock LockSettings
	if err := DB.Where("chat_id = ? AND lock_type = ?", chatID, perm).First(&lock).Error; err != nil {
		t.Fatalf("query error: %v", err)
	}
	if lock.Locked {
		t.Fatalf("expected Locked=false after zero-value update, got true")
	}
}

func TestUpdateRecordSucceedsWhenRowsAffected(t *testing.T) {
	if err := DB.AutoMigrate(&LockSettings{}); err != nil {
		t.Fatalf("failed to migrate locks: %v", err)
	}

	chatID := time.Now().UnixNano()
	perm := "forward"

	t.Cleanup(func() {
		_ = DB.Where("chat_id = ? AND lock_type = ?", chatID, perm).Delete(&LockSettings{}).Error
	})

	// Create a record
	if err := DB.Create(&LockSettings{ChatId: chatID, LockType: perm, Locked: false}).Error; err != nil {
		t.Fatalf("failed to create test record: %v", err)
	}

	// Update it — should succeed (rows affected > 0)
	err := UpdateRecord(
		&LockSettings{},
		LockSettings{ChatId: chatID, LockType: perm},
		map[string]any{"locked": true},
	)
	if err != nil {
		t.Fatalf("UpdateRecord() error = %v, expected nil", err)
	}

	var lock LockSettings
	if err := DB.Where("chat_id = ? AND lock_type = ?", chatID, perm).First(&lock).Error; err != nil {
		t.Fatalf("query error: %v", err)
	}
	if !lock.Locked {
		t.Fatalf("expected Locked=true after update, got false")
	}
}
