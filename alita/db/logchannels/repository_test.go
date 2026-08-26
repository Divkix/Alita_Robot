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

func TestLogChannelCategoryHelpers(t *testing.T) {
	if !IsValidCategory("ADMIN") || IsValidCategory("nope") {
		t.Fatal("IsValidCategory mismatch")
	}
	if categoryColumn("settings") != "cat_settings" ||
		categoryColumn("admin") != "cat_admin" ||
		categoryColumn("user") != "cat_user" ||
		categoryColumn("automated") != "cat_automated" ||
		categoryColumn("reports") != "cat_reports" ||
		categoryColumn("other") != "cat_other" ||
		categoryColumn("nope") != "" {
		t.Fatal("categoryColumn mismatch")
	}

	settings := &models.LogChannel{
		CatSettings:  true,
		CatAdmin:     true,
		CatUser:      true,
		CatAutomated: true,
		CatReports:   true,
		CatOther:     true,
	}
	for _, name := range AllCategories {
		if !CategoryEnabled(settings, name) {
			t.Fatalf("%s should be enabled", name)
		}
	}
	if CategoryEnabled(nil, CategoryAdmin) || CategoryEnabled(settings, "nope") {
		t.Fatal("CategoryEnabled should reject nil settings and unknown names")
	}
	settings.CatSettings = false
	settings.CatAdmin = false
	settings.CatUser = false
	settings.CatAutomated = false
	settings.CatReports = false
	settings.CatOther = false
	for _, name := range AllCategories {
		if CategoryEnabled(settings, name) {
			t.Fatalf("%s should be disabled", name)
		}
	}
}
