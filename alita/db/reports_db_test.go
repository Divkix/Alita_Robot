package db

import (
	"testing"
	"time"
)

func TestGetChatReportSettings_Defaults(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&ReportChatSettings{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
	})

	if err := EnsureChatInDb(chatID, ""); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}

	settings := GetChatReportSettings(chatID)
	if settings == nil {
		t.Fatal("expected non-nil ReportChatSettings")
	}
	if !settings.Enabled {
		t.Fatal("expected default Enabled=true for chat report settings")
	}
	if len(settings.BlockedList) != 0 {
		t.Fatalf("expected empty BlockedList by default, got %v", settings.BlockedList)
	}
}

func TestSetChatReportEnabled_BooleanRoundTrip(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&ReportChatSettings{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
	})

	if err := EnsureChatInDb(chatID, ""); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}

	// Initialize default settings
	_ = GetChatReportSettings(chatID)

	// Disable reports — zero value boolean must persist
	SetChatReportStatus(chatID, false)
	settings := GetChatReportSettings(chatID)
	if settings.Enabled {
		t.Fatal("expected Enabled=false after SetChatReportStatus(false)")
	}

	// Re-enable reports
	SetChatReportStatus(chatID, true)
	settings = GetChatReportSettings(chatID)
	if !settings.Enabled {
		t.Fatal("expected Enabled=true after SetChatReportStatus(true)")
	}
}

func TestGetUserReportSettings_Defaults(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	userID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = DB.Where("user_id = ?", userID).Delete(&ReportUserSettings{}).Error
	})

	settings := GetUserReportSettings(userID)
	if settings == nil {
		t.Fatal("expected non-nil ReportUserSettings")
	}
	if !settings.Enabled {
		t.Fatal("expected default Enabled=true for user report settings")
	}
}

func TestSetUserReportEnabled_BooleanRoundTrip(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	userID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = DB.Where("user_id = ?", userID).Delete(&ReportUserSettings{}).Error
	})

	// Initialize default settings
	_ = GetUserReportSettings(userID)

	// Disable user reports — zero value boolean must persist
	SetUserReportSettings(userID, false)
	settings := GetUserReportSettings(userID)
	if settings.Enabled {
		t.Fatal("expected Enabled=false after SetUserReportSettings(false)")
	}

	// Re-enable user reports
	SetUserReportSettings(userID, true)
	settings = GetUserReportSettings(userID)
	if !settings.Enabled {
		t.Fatal("expected Enabled=true after SetUserReportSettings(true)")
	}
}

func TestGetBlockedReportsList_EmptyForNewChat(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&ReportChatSettings{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
	})

	if err := EnsureChatInDb(chatID, ""); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}

	settings := GetChatReportSettings(chatID)
	if len(settings.BlockedList) != 0 {
		t.Fatalf("expected empty blocked list for new chat, got %v", settings.BlockedList)
	}
}

func TestAddBlockedReport_AddAndVerify(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	chatID := base
	userID := base + 1

	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&ReportChatSettings{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
	})

	if err := EnsureChatInDb(chatID, ""); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}

	// Initialize default settings
	_ = GetChatReportSettings(chatID)

	// Block a user
	BlockReportUser(chatID, userID)

	settings := GetChatReportSettings(chatID)
	found := false
	for _, id := range settings.BlockedList {
		if id == userID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected userID %d in blocked list, got %v", userID, settings.BlockedList)
	}
}

func TestRemoveBlockedReport_AddThenRemove(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	chatID := base
	userID := base + 1

	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&ReportChatSettings{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
	})

	if err := EnsureChatInDb(chatID, ""); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}

	// Initialize default settings
	_ = GetChatReportSettings(chatID)

	// Block then unblock
	BlockReportUser(chatID, userID)
	UnblockReportUser(chatID, userID)

	settings := GetChatReportSettings(chatID)
	for _, id := range settings.BlockedList {
		if id == userID {
			t.Fatalf("expected userID %d to be removed from blocked list, but it's still present", userID)
		}
	}
}

func TestBlockReportUser_IdempotentAdd(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	chatID := base
	userID := base + 1

	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&ReportChatSettings{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
	})

	if err := EnsureChatInDb(chatID, ""); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}

	// Initialize default settings
	_ = GetChatReportSettings(chatID)

	// Add the same user twice
	BlockReportUser(chatID, userID)
	BlockReportUser(chatID, userID)

	settings := GetChatReportSettings(chatID)
	count := 0
	for _, id := range settings.BlockedList {
		if id == userID {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected userID to appear exactly once in blocked list, got %d occurrences", count)
	}
}

func TestLoadReportStats_Returns(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	// Just verify the function executes without error and returns non-negative values
	uRCount, gRCount := LoadReportStats()
	if uRCount < 0 {
		t.Fatalf("expected non-negative uRCount, got %d", uRCount)
	}
	if gRCount < 0 {
		t.Fatalf("expected non-negative gRCount, got %d", gRCount)
	}
}
