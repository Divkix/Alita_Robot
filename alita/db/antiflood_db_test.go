package db

import (
	"testing"
	"time"
)

func TestSetFloodMsgDelZeroValueBoolean(t *testing.T) {
	if err := DB.AutoMigrate(&AntifloodSettings{}); err != nil {
		t.Fatalf("failed to migrate antiflood: %v", err)
	}

	chatID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&AntifloodSettings{}).Error
	})

	// Set to true first
	SetFloodMsgDel(chatID, true)

	var settings AntifloodSettings
	if err := DB.Where("chat_id = ?", chatID).First(&settings).Error; err != nil {
		t.Fatalf("expected record to exist after SetFloodMsgDel(true), got error: %v", err)
	}
	if !settings.DeleteAntifloodMessage {
		t.Fatalf("expected DeleteAntifloodMessage=true, got false")
	}

	// Now set to false — this was the bug: zero value was silently skipped
	SetFloodMsgDel(chatID, false)

	if err := DB.Where("chat_id = ?", chatID).First(&settings).Error; err != nil {
		t.Fatalf("query error after SetFloodMsgDel(false): %v", err)
	}
	if settings.DeleteAntifloodMessage {
		t.Fatalf("expected DeleteAntifloodMessage=false after update, got true")
	}
}

func TestSetFloodZeroValueLimit(t *testing.T) {
	if err := DB.AutoMigrate(&AntifloodSettings{}); err != nil {
		t.Fatalf("failed to migrate antiflood: %v", err)
	}

	chatID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&AntifloodSettings{}).Error
	})

	// Set limit to 5 (enable flood detection)
	SetFlood(chatID, 5)

	var settings AntifloodSettings
	if err := DB.Where("chat_id = ?", chatID).First(&settings).Error; err != nil {
		t.Fatalf("expected record after SetFlood(5), got error: %v", err)
	}
	if settings.Limit != 5 {
		t.Fatalf("expected Limit=5, got %d", settings.Limit)
	}

	// Set limit to 0 (disable) — this was the bug: zero value was silently skipped
	SetFlood(chatID, 0)

	if err := DB.Where("chat_id = ?", chatID).First(&settings).Error; err != nil {
		t.Fatalf("query error after SetFlood(0): %v", err)
	}
	if settings.Limit != 0 {
		t.Fatalf("expected Limit=0 after disabling flood, got %d", settings.Limit)
	}
}

func TestSetFloodMsgDelCreatesRecord(t *testing.T) {
	if err := DB.AutoMigrate(&AntifloodSettings{}); err != nil {
		t.Fatalf("failed to migrate antiflood: %v", err)
	}

	chatID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&AntifloodSettings{}).Error
	})

	// First-time call on a fresh chat should create a record
	SetFloodMsgDel(chatID, true)

	var settings AntifloodSettings
	if err := DB.Where("chat_id = ?", chatID).First(&settings).Error; err != nil {
		t.Fatalf("expected record to be created, got error: %v", err)
	}
	if !settings.DeleteAntifloodMessage {
		t.Fatalf("expected DeleteAntifloodMessage=true, got false")
	}
}
