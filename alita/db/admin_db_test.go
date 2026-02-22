package db

import (
	"testing"
	"time"
)

func TestGetAdminSettings_Defaults(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	t.Cleanup(func() {
		DB.Where("chat_id = ?", chatID).Delete(&AdminSettings{})
	})

	settings := GetAdminSettings(chatID)
	if settings == nil {
		t.Fatal("GetAdminSettings() returned nil")
	}
	if settings.ChatId != chatID {
		t.Fatalf("expected ChatId=%d, got %d", chatID, settings.ChatId)
	}
	if settings.AnonAdmin {
		t.Fatal("expected default AnonAdmin=false")
	}
}

func TestSetAnonAdmin(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano() + 1000

	t.Cleanup(func() {
		DB.Where("chat_id = ?", chatID).Delete(&AdminSettings{})
	})

	// Ensure settings exist first
	_ = GetAdminSettings(chatID)

	// Toggle to true
	if err := SetAnonAdminMode(chatID, true); err != nil {
		t.Fatalf("SetAnonAdminMode(true) error = %v", err)
	}
	settings := GetAdminSettings(chatID)
	if !settings.AnonAdmin {
		t.Fatal("expected AnonAdmin=true after SetAnonAdminMode(true)")
	}

	// Toggle to false -- zero-value boolean round-trip
	if err := SetAnonAdminMode(chatID, false); err != nil {
		t.Fatalf("SetAnonAdminMode(false) error = %v", err)
	}
	settings = GetAdminSettings(chatID)
	if settings.AnonAdmin {
		t.Fatal("expected AnonAdmin=false after SetAnonAdminMode(false)")
	}
}

func TestLoadAdminStats(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	// LoadAdminStats does not exist in admin_db.go; GetAdminSettings is tested above.
	// Verify GetAdminSettings creates records properly for multiple chats.
	base := time.Now().UnixNano() + 2000
	for i := 0; i < 3; i++ {
		chatID := base + int64(i)
		t.Cleanup(func() {
			DB.Where("chat_id = ?", chatID).Delete(&AdminSettings{})
		})
		s := GetAdminSettings(chatID)
		if s == nil {
			t.Fatalf("GetAdminSettings() returned nil for chatID=%d", chatID)
		}
	}
}

func TestSetAnonAdmin_Toggle(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano() + 3000

	t.Cleanup(func() {
		DB.Where("chat_id = ?", chatID).Delete(&AdminSettings{})
	})

	// New chat: default AnonAdmin=false
	settings := GetAdminSettings(chatID)
	if settings == nil {
		t.Fatal("GetAdminSettings() returned nil")
	}
	if settings.AnonAdmin {
		t.Fatal("expected default AnonAdmin=false for new chat")
	}

	// Enable anon admin
	if err := SetAnonAdminMode(chatID, true); err != nil {
		t.Fatalf("SetAnonAdminMode(true) error = %v", err)
	}
	settings = GetAdminSettings(chatID)
	if !settings.AnonAdmin {
		t.Fatal("expected AnonAdmin=true after SetAnonAdminMode(true)")
	}

	// Disable anon admin -- zero-value boolean must be persisted (UPSERT test)
	if err := SetAnonAdminMode(chatID, false); err != nil {
		t.Fatalf("SetAnonAdminMode(false) error = %v", err)
	}
	settings = GetAdminSettings(chatID)
	if settings.AnonAdmin {
		t.Fatal("expected AnonAdmin=false after SetAnonAdminMode(false) -- zero-value UPSERT must persist")
	}
}
