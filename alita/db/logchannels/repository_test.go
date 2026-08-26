package logchannels

import (
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

func TestLogChannelSetCategoryAndUnset(t *testing.T) {
	if db.DB == nil {
		t.Skip("DB not initialized")
	}
	chatID := -time.Now().UnixNano()
	logID := chatID - 1
	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.LogChannel{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
	})

	if Get(chatID) != nil {
		t.Fatal("expected no log channel")
	}
	if err := Set(chatID, "logs chat", logID); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got := Get(chatID)
	if got == nil || got.LogChannelID != logID {
		t.Fatalf("Get = %+v", got)
	}
	if !CategoryEnabled(got, CategoryAdmin) {
		t.Fatal("admin category should default on")
	}
	if err := SetCategory(chatID, CategoryAdmin, false); err != nil {
		t.Fatalf("SetCategory: %v", err)
	}
	got = Get(chatID)
	if CategoryEnabled(got, CategoryAdmin) {
		t.Fatal("admin category still enabled")
	}
	if err := Unset(chatID); err != nil {
		t.Fatalf("Unset: %v", err)
	}
	if Get(chatID) != nil {
		t.Fatal("log channel remained after unset")
	}
}
