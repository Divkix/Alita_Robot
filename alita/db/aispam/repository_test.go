//go:build testtools

package aispam

import (
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

func skipIfNoDb(t *testing.T) {
	if db.DB == nil {
		t.Fatal("test database was not initialized")
	}
}

func cleanupChat(t *testing.T, chatID int64) {
	t.Helper()
	t.Cleanup(func() {
		if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.AISpamSettings{}).Error; err != nil {
			t.Fatalf("cleanup failed: %v", err)
		}
	})
}

func TestAISpamEnabledRoundTrip(t *testing.T) {
	skipIfNoDb(t)
	cache.SetupTestMemoryMarshaler(t)

	chatID := time.Now().UnixNano()
	cleanupChat(t, chatID)

	if IsAISpamEnabled(chatID) {
		t.Fatal("missing row = enabled, want disabled")
	}

	if err := SetAISpamEnabled(chatID, true); err != nil {
		t.Fatalf("SetAISpamEnabled(true) error = %v", err)
	}
	if !IsAISpamEnabled(chatID) {
		t.Fatal("after enable = disabled, want enabled")
	}

	if err := SetAISpamEnabled(chatID, false); err != nil {
		t.Fatalf("SetAISpamEnabled(false) error = %v", err)
	}
	if IsAISpamEnabled(chatID) {
		t.Fatal("after disable = enabled, want disabled")
	}

	var count int64
	if err := db.DB.Model(&models.AISpamSettings{}).Where("chat_id = ?", chatID).Count(&count).Error; err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("rows for chat = %d, want 1 (toggle must upsert, not insert)", count)
	}
}

func TestAISpamEnabledWriteInvalidatesCache(t *testing.T) {
	skipIfNoDb(t)
	cache.SetupTestMemoryMarshaler(t)

	chatID := time.Now().UnixNano()
	cleanupChat(t, chatID)

	if err := SetAISpamEnabled(chatID, false); err != nil {
		t.Fatalf("SetAISpamEnabled(false) error = %v", err)
	}
	// Prime the cache with the disabled value.
	if IsAISpamEnabled(chatID) {
		t.Fatal("primed read = enabled, want disabled")
	}

	// A write through the repository must be visible on the next read even
	// though the previous value is still cached.
	if err := SetAISpamEnabled(chatID, true); err != nil {
		t.Fatalf("SetAISpamEnabled(true) error = %v", err)
	}
	if !IsAISpamEnabled(chatID) {
		t.Fatal("read after enable returned the stale cached value")
	}
}
