//go:build testtools

package blacklists

import (
	"errors"
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
	utilsCache "github.com/divkix/Alita_Robot/alita/utils/cache"
)

func TestBlacklistActionTimePersistsAndInvalidatesCache(t *testing.T) {
	skipIfNoDb(t)
	utilsCache.SetupTestMemoryMarshaler(t)
	chatID := -time.Now().UnixNano()
	t.Cleanup(func() { _ = RemoveAllBlacklist(chatID) })
	if err := SetBlacklistAction(chatID, "tmute", 600); !errors.Is(err, ErrNoBlacklistWords) {
		t.Fatalf("empty-chat setter error = %v, want ErrNoBlacklistWords", err)
	}
	if err := AddBlacklistBulk(chatID, []string{"one", "two"}); err != nil {
		t.Fatal(err)
	}
	_ = GetBlacklistSettings(chatID)
	for _, step := range []struct {
		action     string
		actionTime int
		wantTime   int
	}{
		{action: "tmute", actionTime: 600, wantTime: 600},
		{action: "tmute", actionTime: 1200, wantTime: 1200},
		{action: "tban", actionTime: 7200, wantTime: 7200},
		{action: "ban", actionTime: 7200, wantTime: 0},
		{action: "tban", actionTime: 0, wantTime: 0},
	} {
		if err := SetBlacklistAction(chatID, step.action, step.actionTime); err != nil {
			t.Fatal(err)
		}
		cached := GetBlacklistSettings(chatID)
		if len(cached) != 2 {
			t.Fatalf("cached rows = %d, want 2", len(cached))
		}
		for _, row := range cached {
			if row.Action != step.action || row.ActionTime != step.wantTime {
				t.Fatalf("cached row = %+v, want %s/%d", row, step.action, step.wantTime)
			}
		}
		var stored []models.BlacklistSettings
		if err := db.DB.Where("chat_id = ?", chatID).Find(&stored).Error; err != nil {
			t.Fatal(err)
		}
		for _, row := range stored {
			if row.Action != step.action || row.ActionTime != step.wantTime {
				t.Fatalf("stored row = %+v, want %s/%d", row, step.action, step.wantTime)
			}
		}
	}
	if err := SetBlacklistAction(chatID, "tmute", -1); err == nil {
		t.Fatal("negative duration was accepted")
	}
	var stored []models.BlacklistSettings
	if err := db.DB.Where("chat_id = ?", chatID).Find(&stored).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range stored {
		if row.Action != "tban" || row.ActionTime != 0 {
			t.Fatalf("failed write changed action or duration: %+v", row)
		}
	}
}

func TestNewBlacklistWordsInheritOldestActionAndDuration(t *testing.T) {
	skipIfNoDb(t)
	utilsCache.SetupTestMemoryMarshaler(t)
	chatID := -time.Now().UnixNano()
	t.Cleanup(func() { _ = RemoveAllBlacklist(chatID) })
	if err := AddBlacklist(chatID, "oldest"); err != nil {
		t.Fatal(err)
	}
	if err := SetBlacklistAction(chatID, "tmute", 600); err != nil {
		t.Fatal(err)
	}
	// A later imported row can disagree; new words still inherit the oldest.
	if err := db.DB.Create(&models.BlacklistSettings{
		ChatId: chatID, Word: "newer", Action: "tban", ActionTime: 7200,
	}).Error; err != nil {
		t.Fatal(err)
	}
	_ = GetBlacklistSettings(chatID)
	if err := AddBlacklist(chatID, "Single"); err != nil {
		t.Fatal(err)
	}
	if row := GetBlacklistSettings(chatID).Find("single"); row == nil || row.Action != "tmute" || row.ActionTime != 600 {
		t.Fatalf("single word did not inherit action and duration: %+v", row)
	}
	if err := AddBlacklistBulk(chatID, []string{"Bulk", "bulk", "other", "single"}); err != nil {
		t.Fatal(err)
	}
	settings := GetBlacklistSettings(chatID)
	if len(settings) != 5 {
		t.Fatalf("word count = %d, want 5", len(settings))
	}
	for _, word := range []string{"bulk", "other"} {
		row := settings.Find(word)
		if row == nil || row.Action != "tmute" || row.ActionTime != 600 || row.Reason != models.DefaultBlacklistReason {
			t.Fatalf("bulk word %q did not inherit action and duration: %+v", word, row)
		}
	}
}
