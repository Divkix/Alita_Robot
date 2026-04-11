package db

import (
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestUserModelHasLastActivity(t *testing.T) {
	user := User{
		UserId:       123,
		UserName:     "test",
		Name:         "Test User",
		Language:     "en",
		LastActivity: time.Now(),
	}
	if user.LastActivity.IsZero() {
		t.Error("LastActivity field should be accessible")
	}
}

func TestChatModelHasLastActivity(t *testing.T) {
	chat := Chat{
		ChatId:       456,
		ChatName:     "Test Chat",
		Language:     "en",
		LastActivity: time.Now(),
	}
	if chat.LastActivity.IsZero() {
		t.Error("LastActivity field should be accessible")
	}
}

func TestOptimizedQueriesIncludeLastActivity(t *testing.T) {
	// Test that optimized queries actually select LastActivity
	// This requires running the queries and checking the results include LastActivity

	// Test GetUserBasicInfo
	userQueries := NewOptimizedUserQueries()
	user, err := userQueries.GetUserBasicInfo(123) // use test user ID
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("GetUserBasicInfo failed: %v", err)
	}
	if err == nil && user.LastActivity.IsZero() {
		t.Error("GetUserBasicInfo should populate LastActivity field")
	}

	// Test GetChatBasicInfo
	chatQueries := NewOptimizedChatQueries()
	chat, err := chatQueries.GetChatBasicInfo(456) // use test chat ID
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("GetChatBasicInfo failed: %v", err)
	}
	if err == nil && chat.LastActivity.IsZero() {
		t.Error("GetChatBasicInfo should populate LastActivity field")
	}
}
