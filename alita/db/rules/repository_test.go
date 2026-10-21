package rules

import (
	"os"
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/internal/testdb"
)

func TestMain(m *testing.M) {
	os.Exit(testdb.Run(m,
		&models.WarnSettings{},
		&models.Warns{},
		&models.WarnEntry{},
		&models.GreetingSettings{},
		&models.ChatFilters{},
		&models.AdminSettings{},
		&models.BlacklistSettings{},
		&models.PinSettings{},
		&models.ReportChatSettings{},
		&models.ReportUserSettings{},
		&models.DevSettings{},
		&models.ChannelSettings{},
		&models.AntifloodSettings{},
		&models.ConnectionSettings{},
		&models.ConnectionChatSettings{},
		&models.DisableSettings{},
		&models.DisableChatSettings{},
		&models.RulesSettings{},
		&models.LockSettings{},
		&models.NotesSettings{},
		&models.Notes{},
		&models.CaptchaSettings{},
		&models.CaptchaAttempts{},
		&models.StoredMessages{},
		&models.CaptchaMutedUsers{},
		&models.ApprovedUsers{},
		&models.AntiRaidSettings{},
	))
}

func skipIfNoDb(t *testing.T) {
	t.Helper()
	if db.DB == nil {
		t.Fatal("test database was not initialized")
	}
}

func TestRulesCRUD(t *testing.T) {
	skipIfNoDb(t)

	t.Run("defaults", func(t *testing.T) {
		chatID := time.Now().UnixNano()

		t.Cleanup(func() {
			_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.RulesSettings{}).Error
			_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
		})

		if err := chats.EnsureChatInDb(chatID, ""); err != nil {
			t.Fatalf("EnsureChatInDb() error = %v", err)
		}

		rulesrc := GetChatRulesInfo(chatID)
		if rulesrc == nil {
			t.Fatal("expected non-nil RulesSettings")
		}
		if rulesrc.Rules != "" {
			t.Fatalf("expected empty default Rules, got %q", rulesrc.Rules)
		}
		if rulesrc.Private {
			t.Fatal("expected default Private=false")
		}
	})

	t.Run("set and get", func(t *testing.T) {
		chatID := time.Now().UnixNano()
		const rulesText = "Be kind. No spam. Respect each other."

		t.Cleanup(func() {
			_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.RulesSettings{}).Error
			_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
		})

		if err := chats.EnsureChatInDb(chatID, ""); err != nil {
			t.Fatalf("EnsureChatInDb() error = %v", err)
		}

		_ = GetChatRulesInfo(chatID)

		if err := SetChatRules(chatID, rulesText); err != nil {
			t.Fatalf("SetChatRules() error = %v", err)
		}

		rulesrc := GetChatRulesInfo(chatID)
		if rulesrc.Rules != rulesText {
			t.Fatalf("expected rules %q, got %q", rulesText, rulesrc.Rules)
		}
	})

	t.Run("overwrite", func(t *testing.T) {
		chatID := time.Now().UnixNano()

		t.Cleanup(func() {
			_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.RulesSettings{}).Error
			_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
		})

		if err := chats.EnsureChatInDb(chatID, ""); err != nil {
			t.Fatalf("EnsureChatInDb() error = %v", err)
		}

		_ = GetChatRulesInfo(chatID)

		if err := SetChatRules(chatID, "original rules"); err != nil {
			t.Fatalf("SetChatRules() error = %v", err)
		}
		if err := SetChatRules(chatID, "updated rules"); err != nil {
			t.Fatalf("SetChatRules() error = %v", err)
		}

		rulesrc := GetChatRulesInfo(chatID)
		if rulesrc.Rules != "updated rules" {
			t.Fatalf("expected rules %q after overwrite, got %q", "updated rules", rulesrc.Rules)
		}
	})

	t.Run("button set and get", func(t *testing.T) {
		chatID := time.Now().UnixNano()
		const buttonText = "View Rules"

		t.Cleanup(func() {
			_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.RulesSettings{}).Error
			_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
		})

		if err := chats.EnsureChatInDb(chatID, ""); err != nil {
			t.Fatalf("EnsureChatInDb() error = %v", err)
		}

		_ = GetChatRulesInfo(chatID)

		if err := SetChatRulesButton(chatID, buttonText); err != nil {
			t.Fatalf("SetChatRulesButton() error = %v", err)
		}

		rulesrc := GetChatRulesInfo(chatID)
		if rulesrc.RulesBtn != buttonText {
			t.Fatalf("expected RulesBtn %q, got %q", buttonText, rulesrc.RulesBtn)
		}
	})

	t.Run("private toggle roundtrip", func(t *testing.T) {
		chatID := time.Now().UnixNano()

		t.Cleanup(func() {
			_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.RulesSettings{}).Error
			_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
		})

		if err := chats.EnsureChatInDb(chatID, ""); err != nil {
			t.Fatalf("EnsureChatInDb() error = %v", err)
		}

		_ = GetChatRulesInfo(chatID)

		if err := SetPrivateRules(chatID, true); err != nil {
			t.Fatalf("SetPrivateRules(true) error = %v", err)
		}
		rulesrc := GetChatRulesInfo(chatID)
		if !rulesrc.Private {
			t.Fatal("expected Private=true after SetPrivateRules(true)")
		}

		if err := SetPrivateRules(chatID, false); err != nil {
			t.Fatalf("SetPrivateRules(false) error = %v", err)
		}
		rulesrc = GetChatRulesInfo(chatID)
		if rulesrc.Private {
			t.Fatal("expected Private=false after SetPrivateRules(false)")
		}
	})

	t.Run("private set creates missing row", func(t *testing.T) {
		chatID := time.Now().UnixNano() + 500

		t.Cleanup(func() {
			if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.RulesSettings{}).Error; err != nil {
				t.Fatalf("cleanup RulesSettings failed: %v", err)
			}
			if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error; err != nil {
				t.Fatalf("cleanup Chat failed: %v", err)
			}
		})

		if err := SetPrivateRules(chatID, true); err != nil {
			t.Fatalf("SetPrivateRules() error = %v", err)
		}

		rulesrc := GetChatRulesInfo(chatID)
		if rulesrc == nil {
			t.Fatal("GetChatRulesInfo() returned nil")
		}
		if !rulesrc.Private {
			t.Fatal("Private = false, want true after setting missing rules row")
		}
	})

	t.Run("settings carry chat id", func(t *testing.T) {
		chatID := time.Now().UnixNano()

		t.Cleanup(func() {
			_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.RulesSettings{}).Error
			_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
		})

		if err := chats.EnsureChatInDb(chatID, ""); err != nil {
			t.Fatalf("EnsureChatInDb() error = %v", err)
		}

		// GetChatRulesInfo is the public wrapper for checkRulesSetting
		rulesrc := GetChatRulesInfo(chatID)
		if rulesrc == nil {
			t.Fatal("expected non-nil RulesSettings from GetChatRulesInfo")
		}
		if rulesrc.ChatId != chatID {
			t.Fatalf("expected ChatId=%d, got %d", chatID, rulesrc.ChatId)
		}
	})

	t.Run("empty string persists", func(t *testing.T) {
		chatID := time.Now().UnixNano()

		t.Cleanup(func() {
			_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.RulesSettings{}).Error
			_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
		})

		if err := chats.EnsureChatInDb(chatID, ""); err != nil {
			t.Fatalf("EnsureChatInDb() error = %v", err)
		}

		_ = GetChatRulesInfo(chatID)

		if err := SetChatRules(chatID, ""); err != nil {
			t.Fatalf("SetChatRules(empty) error = %v", err)
		}

		rulesrc := GetChatRulesInfo(chatID)
		if rulesrc == nil {
			t.Fatal("GetChatRulesInfo() returned nil")
		}
		// Default empty string persists whether or not the call is a no-op
		if rulesrc.Rules != "" {
			t.Fatalf("expected Rules='', got %q", rulesrc.Rules)
		}
	})

	t.Run("clear", func(t *testing.T) {
		chatID := time.Now().UnixNano()

		t.Cleanup(func() {
			_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.RulesSettings{}).Error
			_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
		})

		if err := chats.EnsureChatInDb(chatID, ""); err != nil {
			t.Fatalf("EnsureChatInDb() error = %v", err)
		}

		_ = GetChatRulesInfo(chatID)
		if err := SetChatRules(chatID, "Some rules text"); err != nil {
			t.Fatalf("SetChatRules() error = %v", err)
		}

		rulesrc := GetChatRulesInfo(chatID)
		if rulesrc.Rules != "Some rules text" {
			t.Fatalf("expected 'Some rules text', got %q", rulesrc.Rules)
		}

		if err := SetChatRules(chatID, ""); err != nil {
			t.Fatalf("SetChatRules(clear) error = %v", err)
		}

		rulesrc = GetChatRulesInfo(chatID)
		if rulesrc == nil {
			t.Fatal("GetChatRulesInfo() returned nil after clearing")
		}
		if rulesrc.Rules != "" {
			t.Fatalf("expected empty rules after clear, got %q", rulesrc.Rules)
		}
	})
}

func TestLoadRulesStats(t *testing.T) {
	skipIfNoDb(t)

	setRules, pvtRules := LoadRulesStats()
	if setRules < 0 {
		t.Fatalf("expected non-negative setRules, got %d", setRules)
	}
	if pvtRules < 0 {
		t.Fatalf("expected non-negative pvtRules, got %d", pvtRules)
	}
}

func TestRulesSettersErrorBranch(t *testing.T) {
	skipIfNoDb(t)

	if err := db.DB.Migrator().DropTable(&models.RulesSettings{}); err != nil {
		t.Fatalf("DropTable RulesSettings: %v", err)
	}
	t.Cleanup(func() {
		if err := db.DB.AutoMigrate(&models.RulesSettings{}); err != nil {
			t.Fatalf("restore RulesSettings: %v", err)
		}
	})

	chatID := time.Now().UnixNano()
	if err := SetChatRules(chatID, "rules"); err == nil {
		t.Fatal("SetChatRules() error = nil with missing rules table")
	}
	if err := SetChatRulesButton(chatID, "Rules"); err == nil {
		t.Fatal("SetChatRulesButton() error = nil with missing rules table")
	}
	if err := SetPrivateRules(chatID, true); err == nil {
		t.Fatal("SetPrivateRules() error = nil with missing rules table")
	}
}
