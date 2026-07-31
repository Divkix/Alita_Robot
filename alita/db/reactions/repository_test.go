package reactions

import (
	"os"
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

func TestMain(m *testing.M) {
	if db.DB != nil && db.DB.Name() == "sqlite" {
		_ = db.DB.AutoMigrate(&models.Reactions{})
	}
	os.Exit(m.Run())
}

func skipIfNoDb(t *testing.T) {
	t.Helper()
	if db.DB == nil {
		t.Skip("DB not initialized")
	}
}

func TestReactionsRoundTrip(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	t.Cleanup(func() {
		_ = ResetReactions(chatID)
		cache.DeleteCache(cache.CacheKey("reactions", chatID))
	})

	if got := GetReactions(chatID); len(got) != 0 {
		t.Fatalf("GetReactions on fresh chat = %v, want empty", got)
	}

	if err := AddReaction(chatID, "hello", "👋"); err != nil {
		t.Fatalf("AddReaction() error = %v", err)
	}
	if err := AddReaction(chatID, "bye", "🚀"); err != nil {
		t.Fatalf("AddReaction(second) error = %v", err)
	}
	if got := GetReactions(chatID); got["hello"] != "👋" || got["bye"] != "🚀" {
		t.Fatalf("GetReactions = %v, want hello=👋 bye=🚀", got)
	}

	// Upsert: re-adding the same keyword updates the emoji, not a duplicate.
	if err := AddReaction(chatID, "hello", "😀"); err != nil {
		t.Fatalf("AddReaction(upsert) error = %v", err)
	}
	if got := GetReactions(chatID); got["hello"] != "😀" || len(got) != 2 {
		t.Fatalf("GetReactions after upsert = %v, want hello=😀 and 2 entries", got)
	}

	if err := RemoveReaction(chatID, "bye"); err != nil {
		t.Fatalf("RemoveReaction() error = %v", err)
	}
	got := GetReactions(chatID)
	if _, ok := got["bye"]; ok || len(got) != 1 {
		t.Fatalf("GetReactions after remove = %v, want only hello", got)
	}

	if err := ResetReactions(chatID); err != nil {
		t.Fatalf("ResetReactions() error = %v", err)
	}
	if got := GetReactions(chatID); len(got) != 0 {
		t.Fatalf("GetReactions after reset = %v, want empty", got)
	}
}
