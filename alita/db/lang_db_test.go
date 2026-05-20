package db

import (
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func TestGetGroupLanguage_DefaultsToEn(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
		deleteCache(CacheKey("chat_lang", chatID))
	})

	// No chat record -> should return "en"
	lang := getGroupLanguage(chatID)
	if lang != "en" {
		t.Fatalf("expected default language 'en', got %q", lang)
	}
}

func TestGetUserLanguage_DefaultsToEn(t *testing.T) {
	skipIfNoDb(t)

	userID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = DB.Where("user_id = ?", userID).Delete(&User{}).Error
		deleteCache(CacheKey("user_lang", userID))
	})

	// No user record -> should return "en"
	lang := getUserLanguage(userID)
	if lang != "en" {
		t.Fatalf("expected default language 'en', got %q", lang)
	}
}

func TestChangeGroupLanguage_SetAndGet(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
		deleteCache(CacheKey("chat_lang", chatID))
		deleteCache(CacheKey("chat_settings", chatID))
		deleteCache(CacheKey("chat", chatID))
	})

	// Set language to "es"
	_ = ChangeGroupLanguage(chatID, "es")

	lang := getGroupLanguage(chatID)
	if lang != "es" {
		t.Fatalf("expected language 'es', got %q", lang)
	}
}

func TestChangeUserLanguage_SetAndGet(t *testing.T) {
	skipIfNoDb(t)

	userID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = DB.Where("user_id = ?", userID).Delete(&User{}).Error
		deleteCache(CacheKey("user_lang", userID))
		deleteCache(CacheKey("user", userID))
	})

	// Set language to "fr"
	_ = ChangeUserLanguage(userID, "fr")

	lang := getUserLanguage(userID)
	if lang != "fr" {
		t.Fatalf("expected language 'fr', got %q", lang)
	}
}

//nolint:dupl // Test functions intentionally similar for clarity
func TestChangeGroupLanguage_Update(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
		deleteCache(CacheKey("chat_lang", chatID))
		deleteCache(CacheKey("chat_settings", chatID))
		deleteCache(CacheKey("chat", chatID))
	})

	// Create with "en"
	_ = ChangeGroupLanguage(chatID, "en")

	// Update to "hi"
	_ = ChangeGroupLanguage(chatID, "hi")

	lang := getGroupLanguage(chatID)
	if lang != "hi" {
		t.Fatalf("expected language 'hi', got %q", lang)
	}
}

func TestChangeUserLanguage_Update(t *testing.T) {
	skipIfNoDb(t)

	userID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = DB.Where("user_id = ?", userID).Delete(&User{}).Error
		deleteCache(CacheKey("user_lang", userID))
		deleteCache(CacheKey("user", userID))
	})

	// Create with "en"
	_ = ChangeUserLanguage(userID, "en")

	// Update to "es"
	_ = ChangeUserLanguage(userID, "es")

	lang := getUserLanguage(userID)
	if lang != "es" {
		t.Fatalf("expected language 'es', got %q", lang)
	}
}

//nolint:dupl // Test functions intentionally similar for clarity
func TestChangeGroupLanguage_NoopWhenSame(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
		deleteCache(CacheKey("chat_lang", chatID))
		deleteCache(CacheKey("chat_settings", chatID))
		deleteCache(CacheKey("chat", chatID))
	})

	_ = ChangeGroupLanguage(chatID, "en")
	// Calling again with same value should be a no-op (no error)
	_ = ChangeGroupLanguage(chatID, "en")

	lang := getGroupLanguage(chatID)
	if lang != "en" {
		t.Fatalf("expected language 'en', got %q", lang)
	}
}

func TestChangeUserLanguage_NoopWhenSame(t *testing.T) {
	skipIfNoDb(t)

	userID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = DB.Where("user_id = ?", userID).Delete(&User{}).Error
		deleteCache(CacheKey("user_lang", userID))
		deleteCache(CacheKey("user", userID))
	})

	_ = ChangeUserLanguage(userID, "fr")
	// Calling again with same value should be a no-op
	_ = ChangeUserLanguage(userID, "fr")

	lang := getUserLanguage(userID)
	if lang != "fr" {
		t.Fatalf("expected language 'fr', got %q", lang)
	}
}

func TestGetLanguageFromPrivateAndGroupContexts(t *testing.T) {
	skipIfNoDb(t)

	userID := int64(901001)
	groupID := int64(-100901001)

	if got := GetLanguage(nil); got != "en" {
		t.Fatalf("GetLanguage(nil) = %q, want en", got)
	}
	if got := GetLanguage(&ext.Context{}); got != "en" {
		t.Fatalf("GetLanguage(empty context) = %q, want en", got)
	}

	if err := ChangeUserLanguage(userID, "es"); err != nil {
		t.Fatalf("ChangeUserLanguage() error = %v", err)
	}
	privateCtx := ext.NewContext(
		&gotgbot.Bot{User: gotgbot.User{Id: 999, IsBot: true}},
		&gotgbot.Update{
			Message: &gotgbot.Message{
				Chat: gotgbot.Chat{Id: userID, Type: "private", FirstName: "Tester"},
				From: &gotgbot.User{Id: userID, FirstName: "Tester"},
			},
		},
		nil,
	)
	if got := GetLanguage(privateCtx); got != "es" {
		t.Fatalf("GetLanguage(private) = %q, want es", got)
	}

	noSenderCtx := ext.NewContext(
		&gotgbot.Bot{User: gotgbot.User{Id: 999, IsBot: true}},
		&gotgbot.Update{Message: &gotgbot.Message{
			Chat: gotgbot.Chat{Id: userID + 1, Type: "private", FirstName: "No Sender"},
		}},
		nil,
	)
	if got := GetLanguage(noSenderCtx); got != "en" {
		t.Fatalf("GetLanguage(private without sender) = %q, want en", got)
	}

	if err := ChangeGroupLanguage(groupID, "fr"); err != nil {
		t.Fatalf("ChangeGroupLanguage() error = %v", err)
	}
	groupCtx := ext.NewContext(
		&gotgbot.Bot{User: gotgbot.User{Id: 999, IsBot: true}},
		&gotgbot.Update{Message: &gotgbot.Message{
			Chat: gotgbot.Chat{Id: groupID, Type: "supergroup", Title: "Lang Group"},
			From: &gotgbot.User{Id: userID, FirstName: "Tester"},
		}},
		nil,
	)
	if got := GetLanguage(groupCtx); got != "fr" {
		t.Fatalf("GetLanguage(group) = %q, want fr", got)
	}
}
