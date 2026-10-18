//go:build testtools

package spamdbsettings

import (
	"context"
	"errors"
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
		if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.SpamDBSettings{}).Error; err != nil {
			t.Fatalf("cleanup failed: %v", err)
		}
	})
}

func mustGet(t *testing.T, chatID int64) *models.SpamDBSettings {
	t.Helper()
	settings, err := GetSpamDBSettingsContext(context.Background(), chatID)
	if err != nil {
		t.Fatalf("GetSpamDBSettingsContext(%d) error = %v", chatID, err)
	}
	return settings
}

func TestSpamDBEnabledRoundTrip(t *testing.T) {
	skipIfNoDb(t)
	cache.SetupTestMemoryMarshaler(t)

	chatID := time.Now().UnixNano()
	cleanupChat(t, chatID)

	if mustGet(t, chatID).Enabled {
		t.Fatal("missing row = enabled, want disabled")
	}

	if err := SetSpamDBEnabled(chatID, true); err != nil {
		t.Fatalf("SetSpamDBEnabled(true) error = %v", err)
	}
	if !mustGet(t, chatID).Enabled {
		t.Fatal("after enable = disabled, want enabled")
	}

	if err := SetSpamDBEnabled(chatID, false); err != nil {
		t.Fatalf("SetSpamDBEnabled(false) error = %v", err)
	}
	if mustGet(t, chatID).Enabled {
		t.Fatal("after disable = enabled, want disabled")
	}

	var count int64
	if err := db.DB.Model(&models.SpamDBSettings{}).Where("chat_id = ?", chatID).Count(&count).Error; err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("rows for chat = %d, want 1 (toggle must upsert, not insert)", count)
	}
}

func TestSpamDBEnabledWriteInvalidatesCache(t *testing.T) {
	skipIfNoDb(t)
	cache.SetupTestMemoryMarshaler(t)

	chatID := time.Now().UnixNano()
	cleanupChat(t, chatID)

	if err := SetSpamDBEnabled(chatID, false); err != nil {
		t.Fatalf("SetSpamDBEnabled(false) error = %v", err)
	}
	// Prime the cache with the disabled value.
	if mustGet(t, chatID).Enabled {
		t.Fatal("primed read = enabled, want disabled")
	}

	// The next read must see the write even though the old value is cached.
	if err := SetSpamDBEnabled(chatID, true); err != nil {
		t.Fatalf("SetSpamDBEnabled(true) error = %v", err)
	}
	if !mustGet(t, chatID).Enabled {
		t.Fatal("read after enable returned the stale cached value")
	}
}

// One chat's switch must not leak into another chat.
func TestSpamDBEnabledIsPerChat(t *testing.T) {
	skipIfNoDb(t)
	cache.SetupTestMemoryMarshaler(t)

	chatID := time.Now().UnixNano()
	otherChat := chatID + 1
	cleanupChat(t, chatID)
	cleanupChat(t, otherChat)

	if err := SetSpamDBEnabled(chatID, true); err != nil {
		t.Fatalf("SetSpamDBEnabled(true) error = %v", err)
	}
	if mustGet(t, otherChat).Enabled {
		t.Fatal("other chat = enabled, want disabled")
	}
}

func TestSpamDBActionDefaultsToBan(t *testing.T) {
	skipIfNoDb(t)
	cache.SetupTestMemoryMarshaler(t)

	chatID := time.Now().UnixNano()
	cleanupChat(t, chatID)

	if got := mustGet(t, chatID).Action; got != ActionBan {
		t.Fatalf("missing row action = %q, want %q", got, ActionBan)
	}
	if err := SetSpamDBEnabled(chatID, true); err != nil {
		t.Fatalf("SetSpamDBEnabled(true) error = %v", err)
	}
	if got := mustGet(t, chatID).Action; got != ActionBan {
		t.Fatalf("new row action = %q, want %q", got, ActionBan)
	}
}

// The action and the switch are set by separate commands; neither may reset
// the other, and each write must reach the next cached read.
func TestSpamDBActionAndSwitchAreIndependent(t *testing.T) {
	skipIfNoDb(t)
	cache.SetupTestMemoryMarshaler(t)

	chatID := time.Now().UnixNano()
	cleanupChat(t, chatID)

	if err := SetSpamDBAction(chatID, ActionMute); err != nil {
		t.Fatalf("SetSpamDBAction(mute) error = %v", err)
	}
	if got := mustGet(t, chatID); got.Action != ActionMute || got.Enabled {
		t.Fatalf("after action = %+v, want mute and still disabled", got)
	}

	if err := SetSpamDBEnabled(chatID, true); err != nil {
		t.Fatalf("SetSpamDBEnabled(true) error = %v", err)
	}
	if got := mustGet(t, chatID); got.Action != ActionMute || !got.Enabled {
		t.Fatalf("after enable = %+v, want mute and enabled", got)
	}

	if err := SetSpamDBAction(chatID, ActionLog); err != nil {
		t.Fatalf("SetSpamDBAction(log) error = %v", err)
	}
	if got := mustGet(t, chatID); got.Action != ActionLog || !got.Enabled {
		t.Fatalf("after second action = %+v, want log and enabled", got)
	}
}

func TestSetSpamDBActionRejectsUnknownAction(t *testing.T) {
	skipIfNoDb(t)
	cache.SetupTestMemoryMarshaler(t)

	chatID := time.Now().UnixNano()
	cleanupChat(t, chatID)

	if err := SetSpamDBAction(chatID, "kick"); !errors.Is(err, ErrInvalidAction) {
		t.Fatalf("SetSpamDBAction(kick) error = %v, want ErrInvalidAction", err)
	}
	var count int64
	if err := db.DB.Model(&models.SpamDBSettings{}).Where("chat_id = ?", chatID).Count(&count).Error; err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("rows for chat = %d, want 0", count)
	}
}
