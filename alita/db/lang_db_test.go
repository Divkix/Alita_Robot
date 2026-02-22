package db

import (
	"testing"
	"time"
)

func TestGetGroupLanguage_DefaultsToEn(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
		deleteCache(chatLanguageCacheKey(chatID))
	})

	// No chat record → should return "en"
	lang := getGroupLanguage(chatID)
	if lang != "en" {
		t.Fatalf("expected default language 'en', got %q", lang)
	}
}

func TestGetUserLanguage_DefaultsToEn(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	userID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = DB.Where("user_id = ?", userID).Delete(&User{}).Error
		deleteCache(userLanguageCacheKey(userID))
	})

	// No user record → should return "en"
	lang := getUserLanguage(userID)
	if lang != "en" {
		t.Fatalf("expected default language 'en', got %q", lang)
	}
}

func TestChangeGroupLanguage_SetAndGet(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
		deleteCache(chatLanguageCacheKey(chatID))
		deleteCache(chatSettingsCacheKey(chatID))
		deleteCache(chatCacheKey(chatID))
	})

	// Set language to "es"
	ChangeGroupLanguage(chatID, "es")

	lang := getGroupLanguage(chatID)
	if lang != "es" {
		t.Fatalf("expected language 'es', got %q", lang)
	}
}

func TestChangeUserLanguage_SetAndGet(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	userID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = DB.Where("user_id = ?", userID).Delete(&User{}).Error
		deleteCache(userLanguageCacheKey(userID))
		deleteCache(userCacheKey(userID))
	})

	// Set language to "fr"
	ChangeUserLanguage(userID, "fr")

	lang := getUserLanguage(userID)
	if lang != "fr" {
		t.Fatalf("expected language 'fr', got %q", lang)
	}
}

func TestChangeGroupLanguage_Update(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
		deleteCache(chatLanguageCacheKey(chatID))
		deleteCache(chatSettingsCacheKey(chatID))
		deleteCache(chatCacheKey(chatID))
	})

	// Create with "en"
	ChangeGroupLanguage(chatID, "en")

	// Update to "hi"
	ChangeGroupLanguage(chatID, "hi")

	lang := getGroupLanguage(chatID)
	if lang != "hi" {
		t.Fatalf("expected language 'hi', got %q", lang)
	}
}

func TestChangeUserLanguage_Update(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	userID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = DB.Where("user_id = ?", userID).Delete(&User{}).Error
		deleteCache(userLanguageCacheKey(userID))
		deleteCache(userCacheKey(userID))
	})

	// Create with "en"
	ChangeUserLanguage(userID, "en")

	// Update to "es"
	ChangeUserLanguage(userID, "es")

	lang := getUserLanguage(userID)
	if lang != "es" {
		t.Fatalf("expected language 'es', got %q", lang)
	}
}

func TestChangeGroupLanguage_NoopWhenSame(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
		deleteCache(chatLanguageCacheKey(chatID))
		deleteCache(chatSettingsCacheKey(chatID))
		deleteCache(chatCacheKey(chatID))
	})

	ChangeGroupLanguage(chatID, "en")
	// Calling again with same value should be a no-op (no error)
	ChangeGroupLanguage(chatID, "en")

	lang := getGroupLanguage(chatID)
	if lang != "en" {
		t.Fatalf("expected language 'en', got %q", lang)
	}
}

func TestChangeUserLanguage_NoopWhenSame(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	userID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = DB.Where("user_id = ?", userID).Delete(&User{}).Error
		deleteCache(userLanguageCacheKey(userID))
		deleteCache(userCacheKey(userID))
	})

	ChangeUserLanguage(userID, "fr")
	// Calling again with same value should be a no-op
	ChangeUserLanguage(userID, "fr")

	lang := getUserLanguage(userID)
	if lang != "fr" {
		t.Fatalf("expected language 'fr', got %q", lang)
	}
}
