package db

import (
	"testing"
	"time"
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
