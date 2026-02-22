package db

import (
	"fmt"
	"testing"
	"time"
)

func TestAddAndGetFiltersList(t *testing.T) {
	skipIfNoDb(t)
	t.Parallel()

	chatID := -time.Now().UnixNano()

	t.Cleanup(func() {
		RemoveAllFilters(chatID)
	})

	// Initially empty
	list := GetFiltersList(chatID)
	if len(list) != 0 {
		t.Fatalf("expected empty filter list for new chat, got %d items", len(list))
	}

	// Add two filters
	_ = AddFilter(chatID, "spam", "spam reply", "", nil, 1)
	_ = AddFilter(chatID, "flood", "flood reply", "", nil, 1)

	list = GetFiltersList(chatID)
	if len(list) != 2 {
		t.Fatalf("expected 2 filters after adding, got %d", len(list))
	}

	// Adding same keyword again should not duplicate
	_ = AddFilter(chatID, "spam", "different reply", "", nil, 2)
	list = GetFiltersList(chatID)
	if len(list) != 2 {
		t.Fatalf("expected 2 filters (no duplicate), got %d", len(list))
	}
}

func TestDoesFilterExists(t *testing.T) {
	skipIfNoDb(t)
	t.Parallel()

	chatID := -time.Now().UnixNano()

	t.Cleanup(func() {
		RemoveAllFilters(chatID)
	})

	if DoesFilterExists(chatID, "nonexistent") {
		t.Fatal("expected DoesFilterExists=false for non-existent filter")
	}

	_ = AddFilter(chatID, "hello", "hello reply", "", nil, 1)

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
	t.Parallel()

	chatID := -time.Now().UnixNano()

	t.Cleanup(func() {
		RemoveAllFilters(chatID)
	})

	_ = AddFilter(chatID, "remove_me", "reply", "", nil, 1)
	_ = AddFilter(chatID, "keep_me", "reply", "", nil, 1)

	_ = RemoveFilter(chatID, "remove_me")

	if DoesFilterExists(chatID, "remove_me") {
		t.Fatal("expected filter to be removed")
	}
	if !DoesFilterExists(chatID, "keep_me") {
		t.Fatal("expected keep_me filter to still exist")
	}

	// Removing non-existent filter should not error
	_ = RemoveFilter(chatID, "does_not_exist")
}

func TestRemoveAllFilters(t *testing.T) {
	skipIfNoDb(t)
	t.Parallel()

	chatID := -time.Now().UnixNano()

	_ = AddFilter(chatID, "a", "a", "", nil, 1)
	_ = AddFilter(chatID, "b", "b", "", nil, 1)
	_ = AddFilter(chatID, "c", "c", "", nil, 1)

	RemoveAllFilters(chatID)

	list := GetFiltersList(chatID)
	if len(list) != 0 {
		t.Fatalf("expected empty list after RemoveAllFilters, got %d", len(list))
	}
}

func TestCountFilters(t *testing.T) {
	skipIfNoDb(t)
	t.Parallel()

	chatID := -time.Now().UnixNano()

	t.Cleanup(func() {
		RemoveAllFilters(chatID)
	})

	if CountFilters(chatID) != 0 {
		t.Fatal("expected count=0 for new chat")
	}

	for i := 0; i < 3; i++ {
		_ = AddFilter(chatID, fmt.Sprintf("word%d", i), "reply", "", nil, 1)
	}

	if CountFilters(chatID) != 3 {
		t.Fatalf("expected count=3, got %d", CountFilters(chatID))
	}
}

func TestLoadFilterStats(t *testing.T) {
	skipIfNoDb(t)
	t.Parallel()

	// Just verify it returns non-negative values without panicking
	total, chats := LoadFilterStats()
	if total < 0 {
		t.Errorf("LoadFilterStats total = %d, want >= 0", total)
	}
	if chats < 0 {
		t.Errorf("LoadFilterStats chats = %d, want >= 0", chats)
	}
}

func TestAddFilterWithButtons(t *testing.T) {
	skipIfNoDb(t)
	t.Parallel()

	chatID := -time.Now().UnixNano()

	t.Cleanup(func() {
		RemoveAllFilters(chatID)
	})

	buttons := []Button{
		{Name: "Click me", Url: "https://example.com", SameLine: false},
	}

	_ = AddFilter(chatID, "btn_filter", "Filter with button", "", buttons, 1)

	if !DoesFilterExists(chatID, "btn_filter") {
		t.Fatal("expected filter with buttons to exist")
	}
}
