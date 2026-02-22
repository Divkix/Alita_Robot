package db

import (
	"fmt"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// EnsureChatInDb
// ---------------------------------------------------------------------------

func TestEnsureChatInDb(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	chatName := fmt.Sprintf("test-chat-%d", chatID)

	err := EnsureChatInDb(chatID, chatName)
	if err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() { DB.Where("chat_id = ?", chatID).Delete(&Chat{}) })

	// Verify chat was created
	var chat Chat
	if err := DB.Where("chat_id = ?", chatID).First(&chat).Error; err != nil {
		t.Fatalf("expected chat %d to exist, got error: %v", chatID, err)
	}
	if chat.ChatName != chatName {
		t.Errorf("chat name = %q, want %q", chat.ChatName, chatName)
	}
}

func TestEnsureChatInDb_Idempotent(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	chatName := fmt.Sprintf("test-chat-%d", chatID)

	t.Cleanup(func() { DB.Where("chat_id = ?", chatID).Delete(&Chat{}) })

	// Call twice -- must not error
	if err := EnsureChatInDb(chatID, chatName); err != nil {
		t.Fatalf("first EnsureChatInDb() error = %v", err)
	}
	if err := EnsureChatInDb(chatID, chatName); err != nil {
		t.Fatalf("second EnsureChatInDb() error = %v", err)
	}

	// Only one record should exist
	var count int64
	DB.Model(&Chat{}).Where("chat_id = ?", chatID).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 chat record, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// UpdateChat
// ---------------------------------------------------------------------------

func TestUpdateChat(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	userID := chatID + 1
	chatName := fmt.Sprintf("test-chat-%d", chatID)
	updatedName := chatName + "-updated"

	t.Cleanup(func() { DB.Where("chat_id = ?", chatID).Delete(&Chat{}) })

	// Create initial
	if err := UpdateChat(chatID, chatName, userID); err != nil {
		t.Fatalf("initial UpdateChat() error = %v", err)
	}

	// Update name
	if err := UpdateChat(chatID, updatedName, userID); err != nil {
		t.Fatalf("UpdateChat() with new name error = %v", err)
	}

	var chat Chat
	if err := DB.Where("chat_id = ?", chatID).First(&chat).Error; err != nil {
		t.Fatalf("expected chat to exist: %v", err)
	}
	if chat.ChatName != updatedName {
		t.Errorf("chat name = %q, want %q", chat.ChatName, updatedName)
	}
}

// ---------------------------------------------------------------------------
// GetAllChats
// ---------------------------------------------------------------------------

func TestGetAllChats(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	id1 := base
	id2 := base + 1

	t.Cleanup(func() {
		DB.Where("chat_id IN ?", []int64{id1, id2}).Delete(&Chat{})
	})

	if err := EnsureChatInDb(id1, "chat-one"); err != nil {
		t.Fatalf("EnsureChatInDb(id1) error = %v", err)
	}
	if err := EnsureChatInDb(id2, "chat-two"); err != nil {
		t.Fatalf("EnsureChatInDb(id2) error = %v", err)
	}

	allChats := GetAllChats()

	if _, ok := allChats[id1]; !ok {
		t.Errorf("GetAllChats() missing chat with id %d", id1)
	}
	if _, ok := allChats[id2]; !ok {
		t.Errorf("GetAllChats() missing chat with id %d", id2)
	}
}

// ---------------------------------------------------------------------------
// ChatExists
// ---------------------------------------------------------------------------

func TestChatExists(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	t.Run("returns false for non-existent chat", func(t *testing.T) {
		t.Parallel()
		if ChatExists(chatID) {
			t.Errorf("ChatExists(%d) = true, want false for non-existent chat", chatID)
		}
	})

	t.Run("returns true after creation", func(t *testing.T) {
		existingID := chatID + 1000
		if err := EnsureChatInDb(existingID, "existing-chat"); err != nil {
			t.Fatalf("EnsureChatInDb() error = %v", err)
		}
		t.Cleanup(func() { DB.Where("chat_id = ?", existingID).Delete(&Chat{}) })

		if !ChatExists(existingID) {
			t.Errorf("ChatExists(%d) = false, want true after creation", existingID)
		}
	})
}

// ---------------------------------------------------------------------------
// LoadChatStats
// ---------------------------------------------------------------------------

func TestLoadChatStats(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	activeChats, inactiveChats := LoadChatStats()

	// Both values must be non-negative
	if activeChats < 0 {
		t.Errorf("LoadChatStats() activeChats = %d, want >= 0", activeChats)
	}
	if inactiveChats < 0 {
		t.Errorf("LoadChatStats() inactiveChats = %d, want >= 0", inactiveChats)
	}
}
