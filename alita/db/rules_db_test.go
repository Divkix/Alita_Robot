package db

import (
	"testing"
	"time"
)

func TestGetRules_Defaults(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&RulesSettings{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
	})

	if err := EnsureChatInDb(chatID, ""); err != nil {
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

func TestSetRules_SetAndGet(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	const rulesText = "Be kind. No spam. Respect each other."

	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&RulesSettings{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
	})

	if err := EnsureChatInDb(chatID, ""); err != nil {
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

func TestClearRules_SetThenClear(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&RulesSettings{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
	})

	if err := EnsureChatInDb(chatID, ""); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}

	// Create default settings first
	_ = GetChatRulesInfo(chatID)

	// Set then clear
	SetChatRules(chatID, "some rules")
	SetChatRules(chatID, "")

	rulesrc := GetChatRulesInfo(chatID)
	if rulesrc.Rules != "" {
		t.Fatalf("expected empty rules after clearing, got %q", rulesrc.Rules)
	}
}

func TestSetChatRulesButton_SetAndGet(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	const buttonText = "View Rules"

	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&RulesSettings{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
	})

	if err := EnsureChatInDb(chatID, ""); err != nil {
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
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&RulesSettings{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
	})

	if err := EnsureChatInDb(chatID, ""); err != nil {
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

func TestGetRulesSettings_Defaults(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&RulesSettings{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
	})

	if err := EnsureChatInDb(chatID, ""); err != nil {
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
	t.Parallel()
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
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&RulesSettings{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
	})

	if err := EnsureChatInDb(chatID, ""); err != nil {
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
