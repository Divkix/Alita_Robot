package db

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestOptimizedLockQueries_NilDB(t *testing.T) {
	t.Parallel()

	q := &OptimizedLockQueries{db: nil}
	_, err := q.GetLockStatus(123, "text")
	if err == nil {
		t.Fatal("GetLockStatus() with nil db expected error, got nil")
	}
	if err.Error() != "database not initialized" {
		t.Fatalf("GetLockStatus() error = %q, want %q", err.Error(), "database not initialized")
	}
}

func TestOptimizedLockQueries_GetLockStatus(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	lockType := "text"

	t.Cleanup(func() {
		_ = DB.Where("chat_id = ? AND lock_type = ?", chatID, lockType).Delete(&LockSettings{}).Error
	})

	q := NewOptimizedLockQueries()

	// No record -> default unlocked (false)
	locked, err := q.GetLockStatus(chatID, lockType)
	if err != nil {
		t.Fatalf("GetLockStatus() no record error = %v", err)
	}
	if locked {
		t.Fatalf("GetLockStatus() no record = %v, want false", locked)
	}

	// Insert a locked record
	if err := DB.Create(&LockSettings{ChatId: chatID, LockType: lockType, Locked: true}).Error; err != nil {
		t.Fatalf("DB.Create() lock error = %v", err)
	}

	locked, err = q.GetLockStatus(chatID, lockType)
	if err != nil {
		t.Fatalf("GetLockStatus() after insert error = %v", err)
	}
	if !locked {
		t.Fatalf("GetLockStatus() after insert = %v, want true", locked)
	}
}

func TestOptimizedLockQueries_GetChatLocksOptimized(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	lockTypes := []string{"text", "photo", "video"}
	t.Cleanup(func() {
		for _, lt := range lockTypes {
			_ = DB.Where("chat_id = ? AND lock_type = ?", chatID, lt).Delete(&LockSettings{}).Error
		}
	})

	// Insert 3 locks
	for _, lt := range lockTypes {
		if err := DB.Create(&LockSettings{ChatId: chatID, LockType: lt, Locked: true}).Error; err != nil {
			t.Fatalf("DB.Create() lock %q error = %v", lt, err)
		}
	}

	q := NewOptimizedLockQueries()

	// Get all locks for chatID -> map with 3 entries
	result, err := q.GetChatLocksOptimized(chatID)
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
	emptyResult, err := q.GetChatLocksOptimized(differentChatID)
	if err != nil {
		t.Fatalf("GetChatLocksOptimized() different chat error = %v", err)
	}
	if len(emptyResult) != 0 {
		t.Fatalf("GetChatLocksOptimized() different chat len = %d, want 0", len(emptyResult))
	}
}

func TestOptimizedUserQueries_NilDB(t *testing.T) {
	t.Parallel()

	q := &OptimizedUserQueries{db: nil}
	_, err := q.GetUserBasicInfo(123)
	if err == nil {
		t.Fatal("GetUserBasicInfo() with nil db expected error, got nil")
	}
	if err.Error() != "database not initialized" {
		t.Fatalf("GetUserBasicInfo() error = %q, want %q", err.Error(), "database not initialized")
	}
}

func TestOptimizedUserQueries_GetUserBasicInfo(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	userID := time.Now().UnixNano()
	username := fmt.Sprintf("testuser_%d", userID)
	name := "Test User"

	t.Cleanup(func() {
		_ = DB.Where("user_id = ?", userID).Delete(&User{}).Error
	})

	// Insert a user
	if err := EnsureUserInDb(userID, username, name); err != nil {
		t.Fatalf("EnsureUserInDb() error = %v", err)
	}

	q := NewOptimizedUserQueries()

	// Get user by ID
	user, err := q.GetUserBasicInfo(userID)
	if err != nil {
		t.Fatalf("GetUserBasicInfo() error = %v", err)
	}
	if user.UserId != userID {
		t.Fatalf("GetUserBasicInfo() UserId = %d, want %d", user.UserId, userID)
	}

	// Non-existent user -> ErrRecordNotFound
	nonExistentID := userID + 999999
	_, err = q.GetUserBasicInfo(nonExistentID)
	if err == nil {
		t.Fatal("GetUserBasicInfo() nonexistent expected error, got nil")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("GetUserBasicInfo() nonexistent error = %v, want gorm.ErrRecordNotFound", err)
	}
}

func TestOptimizedChatQueries_NilDB(t *testing.T) {
	t.Parallel()

	q := &OptimizedChatQueries{db: nil}
	_, err := q.GetChatBasicInfo(123)
	if err == nil {
		t.Fatal("GetChatBasicInfo() with nil db expected error, got nil")
	}
	if err.Error() != "database not initialized" {
		t.Fatalf("GetChatBasicInfo() error = %q, want %q", err.Error(), "database not initialized")
	}
}

func TestGetOptimizedQueries_Singleton(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	// Call twice and verify same pointer is returned
	first := GetOptimizedQueries()
	second := GetOptimizedQueries()

	if first == nil {
		t.Fatal("GetOptimizedQueries() returned nil on first call")
	}
	if second == nil {
		t.Fatal("GetOptimizedQueries() returned nil on second call")
	}
	if first != second {
		t.Fatalf("GetOptimizedQueries() returned different pointers: %p != %p", first, second)
	}
}

func TestCacheKeyFormats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		fn       func() string
		expected string
	}{
		{
			name:     "lockCacheKey",
			fn:       func() string { return lockCacheKey(123, "text") },
			expected: "alita:lock:123:text",
		},
		{
			name:     "userCacheKey",
			fn:       func() string { return userCacheKey(456) },
			expected: "alita:user:456",
		},
		{
			name:     "chatCacheKey",
			fn:       func() string { return chatCacheKey(789) },
			expected: "alita:chat:789",
		},
		{
			name:     "optimizedAntifloodCacheKey",
			fn:       func() string { return optimizedAntifloodCacheKey(101) },
			expected: "alita:antiflood:101",
		},
		{
			name:     "channelCacheKey",
			fn:       func() string { return channelCacheKey(202) },
			expected: "alita:channel:202",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.fn()
			if got != tc.expected {
				t.Fatalf("%s() = %q, want %q", tc.name, got, tc.expected)
			}
		})
	}
}
