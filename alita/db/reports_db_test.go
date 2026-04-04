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
	if err := SetChatReportStatus(chatID, false); err != nil {
		t.Fatalf("SetChatReportStatus() error = %v", err)
	}
	settings := GetChatReportSettings(chatID)
	if settings.Enabled {
		t.Fatal("expected Enabled=false after SetChatReportStatus(false)")
	}

	// Re-enable reports
	if err := SetChatReportStatus(chatID, true); err != nil {
		t.Fatalf("SetChatReportStatus() error = %v", err)
	}
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
	if err := SetUserReportSettings(userID, false); err != nil {
		t.Fatalf("SetUserReportSettings() error = %v", err)
	}
	settings := GetUserReportSettings(userID)
	if settings.Enabled {
		t.Fatal("expected Enabled=false after SetUserReportSettings(false)")
	}

	// Re-enable user reports
	if err := SetUserReportSettings(userID, true); err != nil {
		t.Fatalf("SetUserReportSettings() error = %v", err)
	}
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
	if err := BlockReportUser(chatID, userID); err != nil {
		t.Fatalf("BlockReportUser() error = %v", err)
	}

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

func TestRemoveBlockedReport_UnblockOneOfTwo(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	chatID := base
	userID1 := base + 1
	userID2 := base + 2

	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&ReportChatSettings{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
	})

	if err := EnsureChatInDb(chatID, ""); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}

	// Initialize default settings
	_ = GetChatReportSettings(chatID)

	// Block two users, then unblock one — list stays non-nil so GORM persists it
	if err := BlockReportUser(chatID, userID1); err != nil {
		t.Fatalf("BlockReportUser() error = %v", err)
	}
	if err := BlockReportUser(chatID, userID2); err != nil {
		t.Fatalf("BlockReportUser() error = %v", err)
	}
	if err := UnblockReportUser(chatID, userID1); err != nil {
		t.Fatalf("UnblockReportUser() error = %v", err)
	}

	settings := GetChatReportSettings(chatID)
	foundUser1 := false
	foundUser2 := false
	for _, id := range settings.BlockedList {
		if id == userID1 {
			foundUser1 = true
		}
		if id == userID2 {
			foundUser2 = true
		}
	}
	if foundUser1 {
		t.Fatalf("expected userID1 %d to be removed from blocked list, but it's still present", userID1)
	}
	if !foundUser2 {
		t.Fatalf("expected userID2 %d to remain in blocked list, but it's missing", userID2)
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
	if err := BlockReportUser(chatID, userID); err != nil {
		t.Fatalf("BlockReportUser() error = %v", err)
	}
	// Second call should return nil error (idempotent)
	if err := BlockReportUser(chatID, userID); err != nil {
		t.Fatalf("BlockReportUser() second call error = %v", err)
	}

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
