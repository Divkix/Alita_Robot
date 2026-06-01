package rules

import (
	"fmt"
	"os"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

func TestMain(m *testing.M) {
	var dbFileName string
	if db.DB == nil {
		dbFile, err := os.CreateTemp("", "alita_test_*.db")
		if err != nil {
			fmt.Printf("temp file creation failed: %v\n", err)
			os.Exit(1)
		}
		dbFileName = dbFile.Name()
		if closeErr := dbFile.Close(); closeErr != nil {
			fmt.Printf("temp file close failed: %v\n", closeErr)
			os.Exit(1)
		}
		dbPath := dbFileName + "?_busy_timeout=10000&_journal_mode=WAL"
		sqliteDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err != nil {
			fmt.Printf("SQLite init failed: %v\n", err)
			os.Exit(1)
		}
		db.DB = sqliteDB
	}

	err := db.DB.AutoMigrate(
		&models.User{},
		&models.Chat{},
		&models.WarnSettings{},
		&models.Warns{},
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
	)
	if err != nil {
		fmt.Printf("AutoMigrate failed: %v\n", err)
		os.Exit(1)
	}

	exitCode := m.Run()

	// Close DB handle before removing temp file.
	if db.DB != nil {
		sqlDB, err := db.DB.DB()
		if err != nil {
			fmt.Printf("failed to get underlying DB: %v\n", err)
		} else if closeErr := sqlDB.Close(); closeErr != nil {
			fmt.Printf("DB close failed: %v\n", closeErr)
		}
	}

	// Remove temp file before exit.
	if dbFileName != "" {
		if rmErr := os.Remove(dbFileName); rmErr != nil {
			fmt.Printf("temp file remove failed: %v\n", rmErr)
		}
	}

	os.Exit(exitCode)
}

func skipIfNoDb(t *testing.T) {
	t.Helper()
	if db.DB == nil {
		t.Skip("requires database connection")
	}
}

func TestGetRules_Defaults(t *testing.T) {
	skipIfNoDb(t)

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
}

//nolint:dupl // Test functions intentionally similar for clarity
func TestSetRules_SetAndGet(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	const rulesText = "Be kind. No spam. Respect each other."

	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.RulesSettings{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
	})

	if err := chats.EnsureChatInDb(chatID, ""); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}

	// Create default settings first
	_ = GetChatRulesInfo(chatID)

	SetChatRules(chatID, rulesText)

	rulesrc := GetChatRulesInfo(chatID)
	if rulesrc.Rules != rulesText {
		t.Fatalf("expected rules %q, got %q", rulesText, rulesrc.Rules)
	}
}

func TestSetRules_OverwriteWithNewValue(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.RulesSettings{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
	})

	if err := chats.EnsureChatInDb(chatID, ""); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}

	// Create default settings first
	_ = GetChatRulesInfo(chatID)

	// Set rules then overwrite with different non-empty value
	SetChatRules(chatID, "original rules")
	SetChatRules(chatID, "updated rules")

	rulesrc := GetChatRulesInfo(chatID)
	if rulesrc.Rules != "updated rules" {
		t.Fatalf("expected rules %q after overwrite, got %q", "updated rules", rulesrc.Rules)
	}
}

//nolint:dupl // Test functions intentionally similar for clarity
func TestSetChatRulesButton_SetAndGet(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	const buttonText = "View Rules"

	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.RulesSettings{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
	})

	if err := chats.EnsureChatInDb(chatID, ""); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}

	// Create default settings first
	_ = GetChatRulesInfo(chatID)

	SetChatRulesButton(chatID, buttonText)

	rulesrc := GetChatRulesInfo(chatID)
	if rulesrc.RulesBtn != buttonText {
		t.Fatalf("expected RulesBtn %q, got %q", buttonText, rulesrc.RulesBtn)
	}
}

func TestTogglePrivateRules_ZeroValueBoolean(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.RulesSettings{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
	})

	if err := chats.EnsureChatInDb(chatID, ""); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}

	// Create default settings first
	_ = GetChatRulesInfo(chatID)

	// Enable private rules
	SetPrivateRules(chatID, true)
	rulesrc := GetChatRulesInfo(chatID)
	if !rulesrc.Private {
		t.Fatal("expected Private=true after SetPrivateRules(true)")
	}

	// Disable private rules — zero value boolean must persist
	SetPrivateRules(chatID, false)
	rulesrc = GetChatRulesInfo(chatID)
	if rulesrc.Private {
		t.Fatal("expected Private=false after SetPrivateRules(false)")
	}
}

func TestSetPrivateRulesCreatesMissingSettings(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano() + 500

	t.Cleanup(func() {
		if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.RulesSettings{}).Error; err != nil {
			t.Fatalf("cleanup RulesSettings failed: %v", err)
		}
		if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error; err != nil {
			t.Fatalf("cleanup Chat failed: %v", err)
		}
	})

	SetPrivateRules(chatID, true)

	rulesrc := GetChatRulesInfo(chatID)
	if rulesrc == nil {
		t.Fatal("GetChatRulesInfo() returned nil")
	}
	if !rulesrc.Private {
		t.Fatal("Private = false, want true after setting missing rules row")
	}
}

func TestGetRulesSettings_Defaults(t *testing.T) {
	skipIfNoDb(t)

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
}

func TestLoadRulesStats(t *testing.T) {
	skipIfNoDb(t)

	// Just verify the function executes without error and returns non-negative values
	setRules, pvtRules := LoadRulesStats()
	if setRules < 0 {
		t.Fatalf("expected non-negative setRules, got %d", setRules)
	}
	if pvtRules < 0 {
		t.Fatalf("expected non-negative pvtRules, got %d", pvtRules)
	}
}

func TestLoadRulesStats_ReflectsNewEntries(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.RulesSettings{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
	})

	if err := chats.EnsureChatInDb(chatID, ""); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}

	// Create default settings and set rules text
	_ = GetChatRulesInfo(chatID)
	SetChatRules(chatID, "test rules for stat counting")
	SetPrivateRules(chatID, true)

	setRules, pvtRules := LoadRulesStats()
	if setRules < 1 {
		t.Fatalf("expected at least 1 chat with rules set, got %d", setRules)
	}
	if pvtRules < 1 {
		t.Fatalf("expected at least 1 chat with private rules enabled, got %d", pvtRules)
	}
}

func TestSetRules_EmptyString(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.RulesSettings{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
	})

	if err := chats.EnsureChatInDb(chatID, ""); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}

	// Initialize settings (default rules = "")
	_ = GetChatRulesInfo(chatID)

	// SetChatRules with empty string is a GORM zero-value skip (no-op on a struct Update)
	// The initial default of "" should remain.
	SetChatRules(chatID, "")

	rulesrc := GetChatRulesInfo(chatID)
	if rulesrc == nil {
		t.Fatal("GetChatRulesInfo() returned nil")
	}
	// Default empty string persists whether or not the call is a no-op
	if rulesrc.Rules != "" {
		t.Fatalf("expected Rules='', got %q", rulesrc.Rules)
	}
}

func TestClearRules(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.RulesSettings{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
	})

	if err := chats.EnsureChatInDb(chatID, ""); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}

	// Initialize and set rules to something non-empty
	_ = GetChatRulesInfo(chatID)
	SetChatRules(chatID, "Some rules text")

	rulesrc := GetChatRulesInfo(chatID)
	if rulesrc.Rules != "Some rules text" {
		t.Fatalf("expected 'Some rules text', got %q", rulesrc.Rules)
	}

	// SetChatRules uses struct-based Updates which skips zero values (empty string).
	// To "clear" rules via the DB layer directly:
	if err := db.DB.Model(&models.RulesSettings{}).
		Where("chat_id = ?", chatID).
		Update("rules", "").Error; err != nil {
		t.Fatalf("DB direct update to clear rules error = %v", err)
	}

	rulesrc = GetChatRulesInfo(chatID)
	if rulesrc == nil {
		t.Fatal("GetChatRulesInfo() returned nil after clearing")
	}
	if rulesrc.Rules != "" {
		t.Fatalf("expected empty rules after clear, got %q", rulesrc.Rules)
	}
}
