package db

import (
	"testing"
	"time"
)

func TestAddBlacklistTrigger(t *testing.T) {
	skipIfNoDb(t)
	t.Parallel()

	chatID := -time.Now().UnixNano()

	t.Cleanup(func() {
		_ = RemoveAllBlacklist(chatID)
	})

	if err := AddBlacklist(chatID, "badword"); err != nil {
		t.Fatalf("AddBlacklist() error = %v", err)
	}

	settings := GetBlacklistSettings(chatID)
	if len(settings) != 1 {
		t.Fatalf("expected 1 blacklist entry, got %d", len(settings))
	}
	if settings[0].Word != "badword" {
		t.Fatalf("expected Word=%q, got %q", "badword", settings[0].Word)
	}
	if settings[0].Action != "warn" {
		t.Fatalf("expected default Action='warn', got %q", settings[0].Action)
	}
}

func TestRemoveBlacklistTrigger(t *testing.T) {
	skipIfNoDb(t)
	t.Parallel()

	chatID := -time.Now().UnixNano()

	t.Cleanup(func() {
		_ = RemoveAllBlacklist(chatID)
	})

	if err := AddBlacklist(chatID, "remove-me"); err != nil {
		t.Fatalf("AddBlacklist() error = %v", err)
	}
	if err := AddBlacklist(chatID, "keep-me"); err != nil {
		t.Fatalf("AddBlacklist() error = %v", err)
	}

	if err := RemoveBlacklist(chatID, "remove-me"); err != nil {
		t.Fatalf("RemoveBlacklist() error = %v", err)
	}

	settings := GetBlacklistSettings(chatID)
	for _, s := range settings {
		if s.Word == "remove-me" {
			t.Fatalf("expected 'remove-me' to be deleted from blacklist")
		}
	}

	found := false
	for _, s := range settings {
		if s.Word == "keep-me" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected 'keep-me' to still be in blacklist")
	}
}

func TestGetBlacklistSettings(t *testing.T) {
	skipIfNoDb(t)
	t.Parallel()

	chatID := -time.Now().UnixNano()

	t.Cleanup(func() {
		_ = RemoveAllBlacklist(chatID)
	})

	// Empty chat should return empty slice, not nil
	settings := GetBlacklistSettings(chatID)
	if settings == nil {
		t.Fatalf("GetBlacklistSettings() returned nil, expected empty slice")
	}
	if len(settings) != 0 {
		t.Fatalf("expected 0 blacklist entries for new chat, got %d", len(settings))
	}
}

func TestSetBlacklistAction(t *testing.T) {
	skipIfNoDb(t)
	t.Parallel()

	chatID := -time.Now().UnixNano()

	t.Cleanup(func() {
		_ = RemoveAllBlacklist(chatID)
	})

	if err := AddBlacklist(chatID, "word1"); err != nil {
		t.Fatalf("AddBlacklist() error = %v", err)
	}
	if err := AddBlacklist(chatID, "word2"); err != nil {
		t.Fatalf("AddBlacklist() error = %v", err)
	}

	err := SetBlacklistAction(chatID, "ban")
	if err != nil {
		t.Fatalf("SetBlacklistAction() error = %v", err)
	}

	settings := GetBlacklistSettings(chatID)
	for _, s := range settings {
		if s.Action != "ban" {
			t.Fatalf("expected Action='ban' for all entries, got %q for word=%q", s.Action, s.Word)
		}
	}
}

func TestGetAllBlacklists(t *testing.T) {
	skipIfNoDb(t)
	t.Parallel()

	chatID := -time.Now().UnixNano()

	t.Cleanup(func() {
		_ = RemoveAllBlacklist(chatID)
	})

	words := []string{"alpha", "beta", "gamma"}
	for _, w := range words {
		if err := AddBlacklist(chatID, w); err != nil {
			t.Fatalf("AddBlacklist() error = %v", err)
		}
	}

	settings := GetBlacklistSettings(chatID)
	if len(settings) < 3 {
		t.Fatalf("expected at least 3 blacklist entries, got %d", len(settings))
	}

	found := map[string]bool{}
	for _, s := range settings {
		found[s.Word] = true
	}
	for _, w := range words {
		if !found[w] {
			t.Fatalf("expected word %q in blacklist, not found", w)
		}
	}
}

func TestLoadBlacklistStats(t *testing.T) {
	skipIfNoDb(t)
	t.Parallel()

	triggers, chats := LoadBlacklistsStats()
	if triggers < 0 {
		t.Errorf("LoadBlacklistsStats triggers = %d, want >= 0", triggers)
	}
	if chats < 0 {
		t.Errorf("LoadBlacklistsStats chats = %d, want >= 0", chats)
	}
}

func TestBlacklistTriggerLowercased(t *testing.T) {
	skipIfNoDb(t)
	t.Parallel()

	chatID := -time.Now().UnixNano()

	t.Cleanup(func() {
		_ = RemoveAllBlacklist(chatID)
	})

	if err := AddBlacklist(chatID, "BadWord"); err != nil {
		t.Fatalf("AddBlacklist() error = %v", err)
	}

	settings := GetBlacklistSettings(chatID)
	if len(settings) != 1 {
		t.Fatalf("expected 1 blacklist entry, got %d", len(settings))
	}
	if settings[0].Word != "badword" {
		t.Fatalf("expected trigger to be lowercased to 'badword', got %q", settings[0].Word)
	}
}
