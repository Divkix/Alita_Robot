package filters

import (
	"fmt"
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

func skipIfNoDb(t *testing.T) {
	if db.DB == nil {
		t.Skip("DB not initialized")
	}
}

func TestAddAndGetFiltersList(t *testing.T) {
	skipIfNoDb(t)

	chatID := -time.Now().UnixNano()

	t.Cleanup(func() {
		if err := RemoveAllFilters(chatID); err != nil {
			t.Errorf("RemoveAllFilters failed: %v", err)
		}
	})

	// Initially empty
	list := GetFiltersList(chatID)
	if len(list) != 0 {
		t.Fatalf("expected empty filter list for new chat, got %d items", len(list))
	}

	// Add two filters
	if err := AddFilter(chatID, "spam", "spam reply", "", nil, 1); err != nil {
		t.Fatalf("AddFilter failed: %v", err)
	}
	if err := AddFilter(chatID, "flood", "flood reply", "", nil, 1); err != nil {
		t.Fatalf("AddFilter failed: %v", err)
	}

	list = GetFiltersList(chatID)
	if len(list) != 2 {
		t.Fatalf("expected 2 filters after adding, got %d", len(list))
	}

	// Adding same keyword again should not duplicate
	if err := AddFilter(chatID, "spam", "different reply", "", nil, 2); err != nil {
		t.Fatalf("AddFilter failed: %v", err)
	}
	list = GetFiltersList(chatID)
	if len(list) != 2 {
		t.Fatalf("expected 2 filters (no duplicate), got %d", len(list))
	}
}

func TestDoesFilterExists(t *testing.T) {
	skipIfNoDb(t)

	chatID := -time.Now().UnixNano()

	t.Cleanup(func() {
		if err := RemoveAllFilters(chatID); err != nil {
			t.Errorf("RemoveAllFilters failed: %v", err)
		}
	})

	if DoesFilterExists(chatID, "nonexistent") {
		t.Fatal("expected DoesFilterExists=false for non-existent filter")
	}

	if err := AddFilter(chatID, "hello", "hello reply", "", nil, 1); err != nil {
		t.Fatalf("AddFilter failed: %v", err)
	}

	if !DoesFilterExists(chatID, "hello") {
		t.Fatal("expected DoesFilterExists=true after adding filter")
	}

	// Case-insensitive check
	if !DoesFilterExists(chatID, "HELLO") {
		t.Fatal("expected DoesFilterExists=true for uppercase variant (case-insensitive)")
	}
}

func TestRemoveFilter(t *testing.T) {
	skipIfNoDb(t)

	chatID := -time.Now().UnixNano()

	t.Cleanup(func() {
		if err := RemoveAllFilters(chatID); err != nil {
			t.Errorf("RemoveAllFilters failed: %v", err)
		}
	})

	if err := AddFilter(chatID, "remove_me", "reply", "", nil, 1); err != nil {
		t.Fatalf("AddFilter failed: %v", err)
	}
	if err := AddFilter(chatID, "keep_me", "reply", "", nil, 1); err != nil {
		t.Fatalf("AddFilter failed: %v", err)
	}

	if err := RemoveFilter(chatID, "remove_me"); err != nil {
		t.Fatalf("RemoveFilter failed: %v", err)
	}

	if DoesFilterExists(chatID, "remove_me") {
		t.Fatal("expected filter to be removed")
	}
	if !DoesFilterExists(chatID, "keep_me") {
		t.Fatal("expected keep_me filter to still exist")
	}

	// Removing non-existent filter should not error
	if err := RemoveFilter(chatID, "does_not_exist"); err != nil {
		t.Fatalf("RemoveFilter(nonexistent) failed: %v", err)
	}
}

func TestRemoveAllFilters(t *testing.T) {
	skipIfNoDb(t)

	chatID := -time.Now().UnixNano()

	if err := AddFilter(chatID, "a", "a", "", nil, 1); err != nil {
		t.Fatalf("AddFilter failed: %v", err)
	}
	if err := AddFilter(chatID, "b", "b", "", nil, 1); err != nil {
		t.Fatalf("AddFilter failed: %v", err)
	}
	if err := AddFilter(chatID, "c", "c", "", nil, 1); err != nil {
		t.Fatalf("AddFilter failed: %v", err)
	}

	if err := RemoveAllFilters(chatID); err != nil {
		t.Fatalf("RemoveAllFilters failed: %v", err)
	}

	list := GetFiltersList(chatID)
	if len(list) != 0 {
		t.Fatalf("expected empty list after RemoveAllFilters, got %d", len(list))
	}
}

func TestCountFilters(t *testing.T) {
	skipIfNoDb(t)

	chatID := -time.Now().UnixNano()

	t.Cleanup(func() {
		if err := RemoveAllFilters(chatID); err != nil {
			t.Errorf("RemoveAllFilters failed: %v", err)
		}
	})

	if CountFilters(chatID) != 0 {
		t.Fatal("expected count=0 for new chat")
	}

	for i := 0; i < 3; i++ {
		if err := AddFilter(chatID, fmt.Sprintf("word%d", i), "reply", "", nil, 1); err != nil {
			t.Fatalf("AddFilter failed: %v", err)
		}
	}

	if CountFilters(chatID) != 3 {
		t.Fatalf("expected count=3, got %d", CountFilters(chatID))
	}
}

func TestLoadFilterStats(t *testing.T) {
	skipIfNoDb(t)

	// Just verify it returns non-negative values without panicking
	total, chats := LoadFilterStats()
	if total < 0 {
		t.Errorf("LoadFilterStats total = %d, want >= 0", total)
	}
	if chats < 0 {
		t.Errorf("LoadFilterStats chats = %d, want >= 0", chats)
	}
}

func TestLoadFilterStatsErrorBranch(t *testing.T) {
	skipIfNoDb(t)

	if err := db.DB.Migrator().DropTable(&models.ChatFilters{}); err != nil {
		t.Fatalf("DropTable failed: %v", err)
	}
	t.Cleanup(func() {
		if err := db.DB.AutoMigrate(&models.ChatFilters{}); err != nil {
			t.Errorf("AutoMigrate failed: %v", err)
		}
	})

	total, chats := LoadFilterStats()
	if total != 0 || chats != 0 {
		t.Fatalf("LoadFilterStats() = (%d, %d), want (0, 0) on error", total, chats)
	}
}

func TestAddFilterWithButtons(t *testing.T) {
	skipIfNoDb(t)

	chatID := -time.Now().UnixNano()

	t.Cleanup(func() {
		if err := RemoveAllFilters(chatID); err != nil {
			t.Errorf("RemoveAllFilters failed: %v", err)
		}
	})

	buttons := []models.Button{
		{Name: "Click me", Url: "https://example.com", SameLine: false},
	}

	if err := AddFilter(chatID, "btn_filter", "Filter with button", "", buttons, 1); err != nil {
		t.Fatalf("AddFilter failed: %v", err)
	}

	if !DoesFilterExists(chatID, "btn_filter") {
		t.Fatal("expected filter with buttons to exist")
	}
}
