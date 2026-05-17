package db

import (
	"testing"
	"time"
)

func TestSetFloodMsgDelZeroValueBoolean(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	t.Cleanup(func() {
		if err := DB.Where("chat_id = ?", chatID).Delete(&AntifloodSettings{}).Error; err != nil {
			t.Fatalf("cleanup failed: %v", err)
		}
	})

	// Set to true first
	if err := SetFloodMsgDel(chatID, true); err != nil {
		t.Fatalf("SetFloodMsgDel(true) failed: %v", err)
	}

	var settings AntifloodSettings
	if err := DB.Where("chat_id = ?", chatID).First(&settings).Error; err != nil {
		t.Fatalf("expected record to exist after SetFloodMsgDel(true), got error: %v", err)
	}
	if !settings.DeleteAntifloodMessage {
		t.Fatalf("expected DeleteAntifloodMessage=true, got false")
	}

	// Now set to false — this was the bug: zero value was silently skipped
	if err := SetFloodMsgDel(chatID, false); err != nil {
		t.Fatalf("SetFloodMsgDel(false) failed: %v", err)
	}

	if err := DB.Where("chat_id = ?", chatID).First(&settings).Error; err != nil {
		t.Fatalf("query error after SetFloodMsgDel(false): %v", err)
	}
	if settings.DeleteAntifloodMessage {
		t.Fatalf("expected DeleteAntifloodMessage=false after update, got true")
	}
}

func TestSetFloodZeroValueLimit(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	t.Cleanup(func() {
		if err := DB.Where("chat_id = ?", chatID).Delete(&AntifloodSettings{}).Error; err != nil {
			t.Fatalf("cleanup failed: %v", err)
		}
	})

	// Set limit to 5 (enable flood detection)
	if err := SetFlood(chatID, 5); err != nil {
		t.Fatalf("SetFlood(5) failed: %v", err)
	}

	var settings AntifloodSettings
	if err := DB.Where("chat_id = ?", chatID).First(&settings).Error; err != nil {
		t.Fatalf("expected record after SetFlood(5), got error: %v", err)
	}
	if settings.Limit != 5 {
		t.Fatalf("expected Limit=5, got %d", settings.Limit)
	}

	// Set limit to 0 (disable) — this was the bug: zero value was silently skipped
	if err := SetFlood(chatID, 0); err != nil {
		t.Fatalf("SetFlood(0) failed: %v", err)
	}

	if err := DB.Where("chat_id = ?", chatID).First(&settings).Error; err != nil {
		t.Fatalf("query error after SetFlood(0): %v", err)
	}
	if settings.Limit != 0 {
		t.Fatalf("expected Limit=0 after disabling flood, got %d", settings.Limit)
	}
}

func TestSetFloodMsgDelCreatesRecord(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	t.Cleanup(func() {
		if err := DB.Where("chat_id = ?", chatID).Delete(&AntifloodSettings{}).Error; err != nil {
			t.Fatalf("cleanup failed: %v", err)
		}
	})

	// First-time call on a fresh chat should create a record
	if err := SetFloodMsgDel(chatID, true); err != nil {
		t.Fatalf("SetFloodMsgDel(true) failed: %v", err)
	}

	var settings AntifloodSettings
	if err := DB.Where("chat_id = ?", chatID).First(&settings).Error; err != nil {
		t.Fatalf("expected record to be created, got error: %v", err)
	}
	if !settings.DeleteAntifloodMessage {
		t.Fatalf("expected DeleteAntifloodMessage=true, got false")
	}
}

func TestSetFloodMode(t *testing.T) {
	skipIfNoDb(t)

	t.Run("creates record with valid mode", func(t *testing.T) {
		chatID := time.Now().UnixNano()
		t.Cleanup(func() {
			if err := DB.Where("chat_id = ?", chatID).Delete(&AntifloodSettings{}).Error; err != nil {
				t.Fatalf("cleanup failed: %v", err)
			}
		})

		if err := SetFloodMode(chatID, "ban"); err != nil {
			t.Fatalf("SetFloodMode failed: %v", err)
		}

		var settings AntifloodSettings
		if err := DB.Where("chat_id = ?", chatID).First(&settings).Error; err != nil {
			t.Fatalf("expected record to exist, got error: %v", err)
		}
		if settings.Action != "ban" {
			t.Fatalf("expected action=ban, got %s", settings.Action)
		}
	})

	t.Run("updates existing record", func(t *testing.T) {
		chatID := time.Now().UnixNano()
		t.Cleanup(func() {
			if err := DB.Where("chat_id = ?", chatID).Delete(&AntifloodSettings{}).Error; err != nil {
				t.Fatalf("cleanup failed: %v", err)
			}
		})

		if err := SetFloodMode(chatID, "kick"); err != nil {
			t.Fatalf("initial SetFloodMode failed: %v", err)
		}
		if err := SetFloodMode(chatID, "warn"); err != nil {
			t.Fatalf("update SetFloodMode failed: %v", err)
		}

		var settings AntifloodSettings
		if err := DB.Where("chat_id = ?", chatID).First(&settings).Error; err != nil {
			t.Fatalf("query error: %v", err)
		}
		if settings.Action != "warn" {
			t.Fatalf("expected action=warn, got %s", settings.Action)
		}
	})

	t.Run("no-op when mode matches existing", func(t *testing.T) {
		chatID := time.Now().UnixNano()
		t.Cleanup(func() {
			if err := DB.Where("chat_id = ?", chatID).Delete(&AntifloodSettings{}).Error; err != nil {
				t.Fatalf("cleanup failed: %v", err)
			}
		})

		if err := SetFloodMode(chatID, "tban"); err != nil {
			t.Fatalf("initial SetFloodMode failed: %v", err)
		}
		if err := SetFloodMode(chatID, "tban"); err != nil {
			t.Fatalf("no-op SetFloodMode failed: %v", err)
		}

		var settings AntifloodSettings
		if err := DB.Where("chat_id = ?", chatID).First(&settings).Error; err != nil {
			t.Fatalf("query error: %v", err)
		}
		if settings.Action != "tban" {
			t.Fatalf("expected action=tban, got %s", settings.Action)
		}
	})

	t.Run("default mode no-op does not create record", func(t *testing.T) {
		chatID := time.Now().UnixNano()
		t.Cleanup(func() {
			if err := DB.Where("chat_id = ?", chatID).Delete(&AntifloodSettings{}).Error; err != nil {
				t.Fatalf("cleanup failed: %v", err)
			}
		})

		// Default action is "mute"; on a fresh chat this should be a no-op
		if err := SetFloodMode(chatID, "mute"); err != nil {
			t.Fatalf("SetFloodMode failed: %v", err)
		}

		var count int64
		if err := DB.Model(&AntifloodSettings{}).Where("chat_id = ?", chatID).Count(&count).Error; err != nil {
			t.Fatalf("count query failed: %v", err)
		}
		if count != 0 {
			t.Fatalf("expected no record for default mode no-op, got count=%d", count)
		}
	})

	t.Run("rejects invalid mode", func(t *testing.T) {
		chatID := time.Now().UnixNano()
		t.Cleanup(func() {
			if err := DB.Where("chat_id = ?", chatID).Delete(&AntifloodSettings{}).Error; err != nil {
				t.Fatalf("cleanup failed: %v", err)
			}
		})

		err := SetFloodMode(chatID, "invalid")
		if err == nil {
			t.Fatalf("expected error for invalid mode, got nil")
		}
	})
}

func TestLoadAntifloodStats(t *testing.T) {
	skipIfNoDb(t)

	t.Run("empty table returns zero", func(t *testing.T) {
		// Ensure table is empty for this assertion
		if err := DB.Where("1 = 1").Delete(&AntifloodSettings{}).Error; err != nil {
			t.Fatalf("cleanup failed: %v", err)
		}

		count := LoadAntifloodStats()
		if count != 0 {
			t.Fatalf("expected 0 for empty table, got %d", count)
		}
	})

	t.Run("counts only enabled chats", func(t *testing.T) {
		chat1 := time.Now().UnixNano()
		chat2 := chat1 + 1
		chat3 := chat1 + 2

		t.Cleanup(func() {
			if err := DB.Where("chat_id IN ?", []int64{chat1, chat2, chat3}).Delete(&AntifloodSettings{}).Error; err != nil {
				t.Fatalf("cleanup failed: %v", err)
			}
		})

		// chat1: enabled (limit > 0)
		if err := SetFlood(chat1, 5); err != nil {
			t.Fatalf("SetFlood failed: %v", err)
		}
		if err := SetFloodMode(chat1, "ban"); err != nil {
			t.Fatalf("SetFloodMode failed: %v", err)
		}

		// chat2: disabled (limit = 0) — must create record with non-zero limit first
		if err := SetFlood(chat2, 5); err != nil {
			t.Fatalf("SetFlood failed: %v", err)
		}
		if err := SetFlood(chat2, 0); err != nil {
			t.Fatalf("SetFlood failed: %v", err)
		}
		if err := SetFloodMode(chat2, "mute"); err != nil {
			t.Fatalf("SetFloodMode failed: %v", err)
		}

		// chat3: enabled (limit > 0)
		if err := SetFlood(chat3, 10); err != nil {
			t.Fatalf("SetFlood failed: %v", err)
		}
		if err := SetFloodMode(chat3, "kick"); err != nil {
			t.Fatalf("SetFloodMode failed: %v", err)
		}

		count := LoadAntifloodStats()
		if count != 2 {
			t.Fatalf("expected 2 enabled chats, got %d", count)
		}
	})

	t.Run("all disabled returns zero", func(t *testing.T) {
		chat1 := time.Now().UnixNano()
		chat2 := chat1 + 1

		t.Cleanup(func() {
			if err := DB.Where("chat_id IN ?", []int64{chat1, chat2}).Delete(&AntifloodSettings{}).Error; err != nil {
				t.Fatalf("cleanup failed: %v", err)
			}
		})

		// Create records with non-zero limit first, then set to 0.
		// SetFlood(chat, 0) on a fresh chat is a no-op because the default limit is 0.
		if err := SetFlood(chat1, 5); err != nil {
			t.Fatalf("SetFlood failed: %v", err)
		}
		if err := SetFlood(chat1, 0); err != nil {
			t.Fatalf("SetFlood failed: %v", err)
		}
		if err := SetFloodMode(chat1, "mute"); err != nil {
			t.Fatalf("SetFloodMode failed: %v", err)
		}

		if err := SetFlood(chat2, 5); err != nil {
			t.Fatalf("SetFlood failed: %v", err)
		}
		if err := SetFlood(chat2, 0); err != nil {
			t.Fatalf("SetFlood failed: %v", err)
		}
		if err := SetFloodMode(chat2, "ban"); err != nil {
			t.Fatalf("SetFloodMode failed: %v", err)
		}

		count := LoadAntifloodStats()
		if count != 0 {
			t.Fatalf("expected 0 for all disabled, got %d", count)
		}
	})
}
