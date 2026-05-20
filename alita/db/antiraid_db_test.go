package db

import (
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

func TestSetRaidTime(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	t.Cleanup(func() {
		if err := DB.Where("chat_id = ?", chatID).Delete(&AntiRaidSettings{}).Error; err != nil {
			t.Fatalf("cleanup failed: %v", err)
		}
	})

	if err := SetRaidTime(chatID, 10800); err != nil {
		t.Fatalf("SetRaidTime failed: %v", err)
	}

	var settings AntiRaidSettings
	if err := DB.Where("chat_id = ?", chatID).First(&settings).Error; err != nil {
		t.Fatalf("expected record, got error: %v", err)
	}
	if settings.RaidTime != 10800 {
		t.Fatalf("expected RaidTime=10800, got %d", settings.RaidTime)
	}
}

func TestSetRaidTimeZeroValue(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	t.Cleanup(func() {
		if err := DB.Where("chat_id = ?", chatID).Delete(&AntiRaidSettings{}).Error; err != nil {
			t.Fatalf("cleanup failed: %v", err)
		}
	})

	// Set to non-zero first, then set to 0 (zero value must persist)
	if err := SetRaidTime(chatID, 10800); err != nil {
		t.Fatalf("SetRaidTime(10800) failed: %v", err)
	}
	if err := SetRaidTime(chatID, 0); err != nil {
		t.Fatalf("SetRaidTime(0) failed: %v", err)
	}

	var settings AntiRaidSettings
	if err := DB.Where("chat_id = ?", chatID).First(&settings).Error; err != nil {
		t.Fatalf("query error: %v", err)
	}
	if settings.RaidTime != 0 {
		t.Fatalf("expected RaidTime=0 after update, got %d", settings.RaidTime)
	}
}

func TestSetRaidTimeNoOp(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	t.Cleanup(func() {
		if err := DB.Where("chat_id = ?", chatID).Delete(&AntiRaidSettings{}).Error; err != nil {
			t.Fatalf("cleanup failed: %v", err)
		}
	})

	// First call creates record
	if err := SetRaidTime(chatID, 7200); err != nil {
		t.Fatalf("SetRaidTime(7200) failed: %v", err)
	}
	// Second call with same value should be no-op but not error
	if err := SetRaidTime(chatID, 7200); err != nil {
		t.Fatalf("no-op SetRaidTime(7200) failed: %v", err)
	}

	var settings AntiRaidSettings
	if err := DB.Where("chat_id = ?", chatID).First(&settings).Error; err != nil {
		t.Fatalf("query error: %v", err)
	}
	if settings.RaidTime != 7200 {
		t.Fatalf("expected RaidTime=7200, got %d", settings.RaidTime)
	}
}

func TestSetRaidActionTime(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	t.Cleanup(func() {
		if err := DB.Where("chat_id = ?", chatID).Delete(&AntiRaidSettings{}).Error; err != nil {
			t.Fatalf("cleanup failed: %v", err)
		}
	})

	// Use non-default value (default is 3600) to trigger actual DB write
	if err := SetRaidActionTime(chatID, 1800); err != nil {
		t.Fatalf("SetRaidActionTime failed: %v", err)
	}

	var settings AntiRaidSettings
	if err := DB.Where("chat_id = ?", chatID).First(&settings).Error; err != nil {
		t.Fatalf("expected record, got error: %v", err)
	}
	if settings.RaidActionTime != 1800 {
		t.Fatalf("expected RaidActionTime=1800, got %d", settings.RaidActionTime)
	}
}

func TestSetAutoAntiRaidThreshold(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	t.Cleanup(func() {
		if err := DB.Where("chat_id = ?", chatID).Delete(&AntiRaidSettings{}).Error; err != nil {
			t.Fatalf("cleanup failed: %v", err)
		}
	})

	if err := SetAutoAntiRaidThreshold(chatID, 5); err != nil {
		t.Fatalf("SetAutoAntiRaidThreshold failed: %v", err)
	}

	var settings AntiRaidSettings
	if err := DB.Where("chat_id = ?", chatID).First(&settings).Error; err != nil {
		t.Fatalf("expected record, got error: %v", err)
	}
	if settings.AutoAntiRaidThreshold != 5 {
		t.Fatalf("expected AutoAntiRaidThreshold=5, got %d", settings.AutoAntiRaidThreshold)
	}
}

func TestDefaultAntiRaidSettings(t *testing.T) {
	t.Parallel()

	settings := defaultAntiRaidSettings(-100123)
	if settings.ChatID != -100123 {
		t.Fatalf("ChatID = %d, want -100123", settings.ChatID)
	}
	if settings.RaidTime != 21600 {
		t.Fatalf("RaidTime = %d, want 21600", settings.RaidTime)
	}
	if settings.RaidActionTime != 3600 {
		t.Fatalf("RaidActionTime = %d, want 3600", settings.RaidActionTime)
	}
	if settings.AutoAntiRaidThreshold != 0 {
		t.Fatalf("AutoAntiRaidThreshold = %d, want 0", settings.AutoAntiRaidThreshold)
	}
}

func TestAntiRaidSettersRejectNegativeValues(t *testing.T) {
	t.Parallel()

	chatID := time.Now().UnixNano()
	tests := []struct {
		name string
		call func() error
	}{
		{name: "raid time", call: func() error { return SetRaidTime(chatID, -1) }},
		{name: "raid action time", call: func() error { return SetRaidActionTime(chatID, -1) }},
		{name: "auto threshold", call: func() error { return SetAutoAntiRaidThreshold(chatID, -1) }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); err == nil {
				t.Fatal("expected negative value error, got nil")
			}
		})
	}
}

func TestSetAutoAntiRaidThresholdZeroValue(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	t.Cleanup(func() {
		if err := DB.Where("chat_id = ?", chatID).Delete(&AntiRaidSettings{}).Error; err != nil {
			t.Fatalf("cleanup failed: %v", err)
		}
	})

	if err := SetAutoAntiRaidThreshold(chatID, 10); err != nil {
		t.Fatalf("SetAutoAntiRaidThreshold(10) failed: %v", err)
	}
	if err := SetAutoAntiRaidThreshold(chatID, 0); err != nil {
		t.Fatalf("SetAutoAntiRaidThreshold(0) failed: %v", err)
	}

	var settings AntiRaidSettings
	if err := DB.Where("chat_id = ?", chatID).First(&settings).Error; err != nil {
		t.Fatalf("query error: %v", err)
	}
	if settings.AutoAntiRaidThreshold != 0 {
		t.Fatalf("expected AutoAntiRaidThreshold=0 after update, got %d", settings.AutoAntiRaidThreshold)
	}
}

func TestGetAntiRaidSettingsDefault(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	// No record created, should return defaults

	settings := GetAntiRaidSettings(chatID)
	if settings == nil {
		t.Fatal("expected default settings, got nil")
	}
	if settings.ChatID != chatID {
		t.Fatalf("expected ChatID=%d, got %d", chatID, settings.ChatID)
	}
	if settings.RaidTime != 21600 {
		t.Fatalf("expected default RaidTime=21600, got %d", settings.RaidTime)
	}
	if settings.RaidActionTime != 3600 {
		t.Fatalf("expected default RaidActionTime=3600, got %d", settings.RaidActionTime)
	}
	if settings.AutoAntiRaidThreshold != 0 {
		t.Fatalf("expected default AutoAntiRaidThreshold=0, got %d", settings.AutoAntiRaidThreshold)
	}
}

func TestGetAntiRaidSettingsWithRecord(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	t.Cleanup(func() {
		if err := DB.Where("chat_id = ?", chatID).Delete(&AntiRaidSettings{}).Error; err != nil {
			t.Fatalf("cleanup failed: %v", err)
		}
	})

	// Use FirstOrCreate to set custom values
	updates := map[string]any{
		"chat_id":                 chatID,
		"raid_time":               7200,
		"raid_action_time":        1800,
		"auto_antiraid_threshold": 3,
	}
	if err := DB.Where("chat_id = ?", chatID).Assign(updates).FirstOrCreate(&AntiRaidSettings{}).Error; err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	settings := GetAntiRaidSettings(chatID)
	if settings == nil {
		t.Fatal("expected settings, got nil")
	}
	if settings.RaidTime != 7200 {
		t.Fatalf("expected RaidTime=7200, got %d", settings.RaidTime)
	}
	if settings.RaidActionTime != 1800 {
		t.Fatalf("expected RaidActionTime=1800, got %d", settings.RaidActionTime)
	}
	if settings.AutoAntiRaidThreshold != 3 {
		t.Fatalf("expected AutoAntiRaidThreshold=3, got %d", settings.AutoAntiRaidThreshold)
	}
}

func TestSetAntiRaidThresholdNegativeRejection(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	t.Cleanup(func() {
		if err := DB.Where("chat_id = ?", chatID).Delete(&AntiRaidSettings{}).Error; err != nil {
			t.Fatalf("cleanup failed: %v", err)
		}
	})

	err := SetAutoAntiRaidThreshold(chatID, -1)
	if err == nil {
		t.Fatal("expected error for negative threshold, got nil")
	}
}

func TestAntiRaidSettingsCacheInvalidation(t *testing.T) {
	skipIfNoDb(t)
	if !cache.IsRedisAvailable() {
		t.Skip("requires Redis cache")
	}

	chatID := time.Now().UnixNano()
	t.Cleanup(func() {
		if err := DB.Where("chat_id = ?", chatID).Delete(&AntiRaidSettings{}).Error; err != nil {
			t.Fatalf("cleanup failed: %v", err)
		}
	})

	// Create initial record via setter (populates cache)
	if err := SetRaidTime(chatID, 3600); err != nil {
		t.Fatalf("SetRaidTime failed: %v", err)
	}

	// Populate cache with the initial value
	first := GetAntiRaidSettings(chatID)
	if first.RaidTime != 3600 {
		t.Fatalf("expected cached RaidTime=3600, got %d", first.RaidTime)
	}

	// Direct DB update to simulate external change; the cache is now stale
	if err := DB.Model(&AntiRaidSettings{}).Where("chat_id = ?", chatID).Update("raid_time", 10800).Error; err != nil {
		t.Fatalf("direct DB update failed: %v", err)
	}

	// Stale cached value should still reflect 3600
	stale := GetAntiRaidSettings(chatID)
	if stale.RaidTime != 3600 {
		t.Fatalf("expected stale cached RaidTime=3600, got %d", stale.RaidTime)
	}

	// Setter should invalidate cache and persist the new value
	if err := SetRaidTime(chatID, 10800); err != nil {
		t.Fatalf("SetRaidTime(10800) failed: %v", err)
	}

	// After cache invalidation, read should reflect the DB update (10800)
	fresh := GetAntiRaidSettings(chatID)
	if fresh.RaidTime != 10800 {
		t.Fatalf("expected RaidTime=10800 after cache invalidation, got %d", fresh.RaidTime)
	}
}
