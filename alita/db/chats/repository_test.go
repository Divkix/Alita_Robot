package chats

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

func skipIfNoDb(t *testing.T) {
	t.Helper()
	if db.DB == nil {
		t.Skip("requires database connection")
	}
}

// ---------------------------------------------------------------------------
// EnsureChatInDb
// ---------------------------------------------------------------------------

func TestEnsureChatInDb(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	chatName := fmt.Sprintf("test-chat-%d", chatID)

	err := EnsureChatInDb(chatID, chatName)
	if err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() { db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}) })

	// Verify chat was created
	var chat models.Chat
	if err := db.DB.Where("chat_id = ?", chatID).First(&chat).Error; err != nil {
		t.Fatalf("expected chat %d to exist, got error: %v", chatID, err)
	}
	if chat.ChatName != chatName {
		t.Errorf("chat name = %q, want %q", chat.ChatName, chatName)
	}
}

func TestEnsureChatInDb_Idempotent(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	chatName := fmt.Sprintf("test-chat-%d", chatID)

	t.Cleanup(func() { db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}) })

	// Call twice -- must not error
	if err := EnsureChatInDb(chatID, chatName); err != nil {
		t.Fatalf("first EnsureChatInDb() error = %v", err)
	}
	if err := EnsureChatInDb(chatID, chatName); err != nil {
		t.Fatalf("second EnsureChatInDb() error = %v", err)
	}

	// Only one record should exist
	var count int64
	db.DB.Model(&models.Chat{}).Where("chat_id = ?", chatID).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 chat record, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// GetChatSettings
// ---------------------------------------------------------------------------

func TestGetChatSettings(t *testing.T) {
	skipIfNoDb(t)

	unknownChatID := time.Now().UnixNano()

	// Unknown chat must return an empty struct
	settings := GetChatSettings(unknownChatID)
	if settings == nil {
		t.Fatal("GetChatSettings() returned nil for unknown chat, want empty struct")
	}
	if settings.ChatId != 0 {
		t.Errorf("GetChatSettings() ChatId = %d, want 0 for unknown chat", settings.ChatId)
	}

	// Known chat must return correct struct
	chatID := unknownChatID + 1
	chatName := fmt.Sprintf("settings-chat-%d", chatID)
	userID := chatID + 100

	t.Cleanup(func() { db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}) })

	if err := UpdateChat(chatID, chatName, userID); err != nil {
		t.Fatalf("UpdateChat() error = %v", err)
	}

	settings = GetChatSettings(chatID)
	if settings == nil {
		t.Fatal("GetChatSettings() returned nil for known chat")
	}
	if settings.ChatId != chatID {
		t.Errorf("GetChatSettings() ChatId = %d, want %d", settings.ChatId, chatID)
	}
	if settings.ChatName != chatName {
		t.Errorf("GetChatSettings() ChatName = %q, want %q", settings.ChatName, chatName)
	}
	if !slices.Contains(settings.Users, userID) {
		t.Errorf("GetChatSettings() Users = %v, want to contain %d", settings.Users, userID)
	}
}

// ---------------------------------------------------------------------------
// UpdateChat
// ---------------------------------------------------------------------------

func TestUpdateChat(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	userID := chatID + 1
	userID2 := chatID + 2
	chatName := fmt.Sprintf("test-chat-%d", chatID)
	updatedName := chatName + "-updated"

	t.Cleanup(func() { db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}) })

	// Create new chat
	if err := UpdateChat(chatID, chatName, userID); err != nil {
		t.Fatalf("initial UpdateChat() error = %v", err)
	}

	var chat models.Chat
	if err := db.DB.Where("chat_id = ?", chatID).First(&chat).Error; err != nil {
		t.Fatalf("expected chat to exist after creation: %v", err)
	}
	if chat.ChatName != chatName {
		t.Errorf("chat name = %q, want %q", chat.ChatName, chatName)
	}
	if !slices.Contains(chat.Users, userID) {
		t.Errorf("chat users = %v, want to contain %d", chat.Users, userID)
	}

	// Update existing chat name
	if err := UpdateChat(chatID, updatedName, userID); err != nil {
		t.Fatalf("UpdateChat() with new name error = %v", err)
	}

	if err := db.DB.Where("chat_id = ?", chatID).First(&chat).Error; err != nil {
		t.Fatalf("expected chat to exist after update: %v", err)
	}
	if chat.ChatName != updatedName {
		t.Errorf("chat name after update = %q, want %q", chat.ChatName, updatedName)
	}

	// Add another user
	if err := UpdateChat(chatID, updatedName, userID2); err != nil {
		t.Fatalf("UpdateChat() adding user2 error = %v", err)
	}

	if err := db.DB.Where("chat_id = ?", chatID).First(&chat).Error; err != nil {
		t.Fatalf("expected chat to exist after adding user: %v", err)
	}
	if !slices.Contains(chat.Users, userID) {
		t.Errorf("chat users missing original user %d: %v", userID, chat.Users)
	}
	if !slices.Contains(chat.Users, userID2) {
		t.Errorf("chat users missing new user %d: %v", userID2, chat.Users)
	}
}

// ---------------------------------------------------------------------------
// GetAllChats
// ---------------------------------------------------------------------------

func TestGetAllChats(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	id1 := base
	id2 := base + 1

	t.Cleanup(func() {
		db.DB.Where("chat_id IN ?", []int64{id1, id2}).Delete(&models.Chat{})
	})

	// Should be empty before creating chats
	before := GetAllChats()
	if _, exists := before[id1]; exists {
		t.Errorf("GetAllChats() should not contain id %d before creation", id1)
	}
	if _, exists := before[id2]; exists {
		t.Errorf("GetAllChats() should not contain id %d before creation", id2)
	}

	if err := EnsureChatInDb(id1, "chat-one"); err != nil {
		t.Fatalf("EnsureChatInDb(id1) error = %v", err)
	}
	if err := EnsureChatInDb(id2, "chat-two"); err != nil {
		t.Fatalf("EnsureChatInDb(id2) error = %v", err)
	}

	allChats := GetAllChats()

	c1, ok := allChats[id1]
	if !ok {
		t.Errorf("GetAllChats() missing chat with id %d", id1)
	} else if c1.ChatName != "chat-one" {
		t.Errorf("chat-one name = %q, want %q", c1.ChatName, "chat-one")
	}

	c2, ok := allChats[id2]
	if !ok {
		t.Errorf("GetAllChats() missing chat with id %d", id2)
	} else if c2.ChatName != "chat-two" {
		t.Errorf("chat-two name = %q, want %q", c2.ChatName, "chat-two")
	}
}

// ---------------------------------------------------------------------------
// ChatExists
// ---------------------------------------------------------------------------

func TestChatExists(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	t.Run("returns false for non-existent chat", func(t *testing.T) {
		if db.ChatExists(chatID) {
			t.Errorf("ChatExists(%d) = true, want false for non-existent chat", chatID)
		}
	})

	t.Run("returns true after creation", func(t *testing.T) {
		existingID := chatID + 1000
		if err := EnsureChatInDb(existingID, "existing-chat"); err != nil {
			t.Fatalf("EnsureChatInDb() error = %v", err)
		}
		t.Cleanup(func() { db.DB.Where("chat_id = ?", existingID).Delete(&models.Chat{}) })

		if !db.ChatExists(existingID) {
			t.Errorf("ChatExists(%d) = false, want true after creation", existingID)
		}
	})
}

// ---------------------------------------------------------------------------
// LoadChatStats
// ---------------------------------------------------------------------------

func TestLoadChatStats(t *testing.T) {
	skipIfNoDb(t)

	// Record baseline
	baseActive, baseInactive := LoadChatStats()

	chatIDActive := time.Now().UnixNano()
	chatIDInactive := chatIDActive + 1

	t.Cleanup(func() {
		db.DB.Where("chat_id IN ?", []int64{chatIDActive, chatIDInactive}).Delete(&models.Chat{})
	})

	// Create an active chat
	if err := EnsureChatInDb(chatIDActive, "active-chat"); err != nil {
		t.Fatalf("EnsureChatInDb(active) error = %v", err)
	}
	// Ensure it is active (default is_inactive=false)
	if err := db.DB.Model(&models.Chat{}).Where("chat_id = ?", chatIDActive).Update("is_inactive", false).Error; err != nil {
		t.Fatalf("failed to mark chat active: %v", err)
	}

	// Create an inactive chat
	if err := EnsureChatInDb(chatIDInactive, "inactive-chat"); err != nil {
		t.Fatalf("EnsureChatInDb(inactive) error = %v", err)
	}
	if err := db.DB.Model(&models.Chat{}).Where("chat_id = ?", chatIDInactive).Update("is_inactive", true).Error; err != nil {
		t.Fatalf("failed to mark chat inactive: %v", err)
	}

	activeChats, inactiveChats := LoadChatStats()

	if activeChats != baseActive+1 {
		t.Errorf("LoadChatStats() activeChats = %d, want %d", activeChats, baseActive+1)
	}
	if inactiveChats != baseInactive+1 {
		t.Errorf("LoadChatStats() inactiveChats = %d, want %d", inactiveChats, baseInactive+1)
	}
}

// ---------------------------------------------------------------------------
// LoadActivityStats
// ---------------------------------------------------------------------------

func TestLoadActivityStats(t *testing.T) {
	skipIfNoDb(t)

	// Record baseline
	baseDag, baseWag, baseMag := LoadActivityStats()

	now := time.Now()
	chatIDDaily := now.UnixNano()
	chatIDWeekly := chatIDDaily + 1
	chatIDMonthly := chatIDDaily + 2
	chatIDOld := chatIDDaily + 3

	t.Cleanup(func() {
		db.DB.Where("chat_id IN ?", []int64{chatIDDaily, chatIDWeekly, chatIDMonthly, chatIDOld}).Delete(&models.Chat{})
	})

	// Chat active within last 24 hours
	if err := EnsureChatInDb(chatIDDaily, "daily-chat"); err != nil {
		t.Fatalf("EnsureChatInDb(daily) error = %v", err)
	}
	if err := db.DB.Model(&models.Chat{}).Where("chat_id = ?", chatIDDaily).Updates(map[string]any{
		"last_activity": now.Add(-1 * time.Hour),
		"is_inactive":   false,
	}).Error; err != nil {
		t.Fatalf("failed to set daily chat activity: %v", err)
	}

	// Chat active within last week but not last day
	if err := EnsureChatInDb(chatIDWeekly, "weekly-chat"); err != nil {
		t.Fatalf("EnsureChatInDb(weekly) error = %v", err)
	}
	if err := db.DB.Model(&models.Chat{}).Where("chat_id = ?", chatIDWeekly).Updates(map[string]any{
		"last_activity": now.Add(-3 * 24 * time.Hour),
		"is_inactive":   false,
	}).Error; err != nil {
		t.Fatalf("failed to set weekly chat activity: %v", err)
	}

	// Chat active within last month but not last week
	if err := EnsureChatInDb(chatIDMonthly, "monthly-chat"); err != nil {
		t.Fatalf("EnsureChatInDb(monthly) error = %v", err)
	}
	if err := db.DB.Model(&models.Chat{}).Where("chat_id = ?", chatIDMonthly).Updates(map[string]any{
		"last_activity": now.Add(-15 * 24 * time.Hour),
		"is_inactive":   false,
	}).Error; err != nil {
		t.Fatalf("failed to set monthly chat activity: %v", err)
	}

	// Chat older than a month (should not count in any bucket)
	if err := EnsureChatInDb(chatIDOld, "old-chat"); err != nil {
		t.Fatalf("EnsureChatInDb(old) error = %v", err)
	}
	if err := db.DB.Model(&models.Chat{}).Where("chat_id = ?", chatIDOld).Updates(map[string]any{
		"last_activity": now.Add(-40 * 24 * time.Hour),
		"is_inactive":   false,
	}).Error; err != nil {
		t.Fatalf("failed to set old chat activity: %v", err)
	}

	dag, wag, mag := LoadActivityStats()

	// DAG counts only daily
	if dag != baseDag+1 {
		t.Errorf("LoadActivityStats() dag = %d, want %d", dag, baseDag+1)
	}
	// WAG counts daily + weekly
	if wag != baseWag+2 {
		t.Errorf("LoadActivityStats() wag = %d, want %d", wag, baseWag+2)
	}
	// MAG counts daily + weekly + monthly
	if mag != baseMag+3 {
		t.Errorf("LoadActivityStats() mag = %d, want %d", mag, baseMag+3)
	}
}
