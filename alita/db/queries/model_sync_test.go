package queries

import (
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

func TestUserModelHasLastActivity(t *testing.T) {
	user := models.User{
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
	chat := models.Chat{
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
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	userID := base + 100
	chatID := base + 101
	userLastActivity := time.Now().UTC().Truncate(time.Second)
	chatLastActivity := userLastActivity.Add(30 * time.Second)

	userRecord := &models.User{
		UserId:       userID,
		UserName:     "sync_test_user",
		Name:         "Model Sync Test User",
		Language:     "en",
		LastActivity: userLastActivity,
	}
	if err := db.CreateRecord(userRecord); err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	chatRecord := &models.Chat{
		ChatId:       chatID,
		ChatName:     "Model Sync Test Chat",
		Language:     "en",
		Users:        models.Int64Array{userID},
		LastActivity: chatLastActivity,
	}
	if err := db.CreateRecord(chatRecord); err != nil {
		t.Fatalf("failed to create test chat: %v", err)
	}

	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
		_ = db.DB.Where("user_id = ?", userID).Delete(&models.User{}).Error
	})

	// Test GetUserBasicInfo query projection.
	userQueries := NewOptimizedUserQueries()
	user, err := userQueries.GetUserBasicInfo(userID)
	if err != nil {
		t.Fatalf("GetUserBasicInfo failed: %v", err)
	}
	if user.LastActivity.IsZero() {
		t.Error("GetUserBasicInfo should populate LastActivity field")
	}
	if got := user.LastActivity.Sub(userLastActivity); got < -time.Second || got > time.Second {
		t.Fatalf("GetUserBasicInfo LastActivity mismatch: got %v want around %v", user.LastActivity, userLastActivity)
	}

	// Test GetChatBasicInfo query projection.
	chatQueries := NewOptimizedChatQueries()
	chat, err := chatQueries.GetChatBasicInfo(chatID)
	if err != nil {
		t.Fatalf("GetChatBasicInfo failed: %v", err)
	}
	if chat.LastActivity.IsZero() {
		t.Error("GetChatBasicInfo should populate LastActivity field")
	}
	if got := chat.LastActivity.Sub(chatLastActivity); got < -time.Second || got > time.Second {
		t.Fatalf("GetChatBasicInfo LastActivity mismatch: got %v want around %v", chat.LastActivity, chatLastActivity)
	}
}
