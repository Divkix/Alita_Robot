package db

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestCheckWarnSettings_Defaults(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	if err := EnsureChatInDb(chatID, "test-warn-defaults"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&WarnSettings{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
	})

	settings := GetWarnSetting(chatID)
	if settings == nil {
		t.Fatalf("GetWarnSetting() returned nil")
	}
	if settings.WarnLimit != 3 {
		t.Fatalf("expected default WarnLimit=3, got %d", settings.WarnLimit)
	}
	if settings.WarnMode != "mute" {
		t.Fatalf("expected default WarnMode='mute', got %q", settings.WarnMode)
	}
}

func TestWarnUser(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	chatID := base
	userID := base + 1

	if err := EnsureChatInDb(chatID, "test-warn-user"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ? AND user_id = ?", chatID, userID).Delete(&Warns{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&WarnSettings{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
	})

	numWarns, reasons := WarnUser(userID, chatID, "test reason")
	if numWarns != 1 {
		t.Fatalf("expected numWarns=1, got %d", numWarns)
	}
	if len(reasons) != 1 {
		t.Fatalf("expected 1 reason, got %d", len(reasons))
	}
	if reasons[0] != "test reason" {
		t.Fatalf("expected reason='test reason', got %q", reasons[0])
	}
}

func TestWarnUserReachesLimit(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	chatID := base
	userID := base + 1
	limit := 3

	if err := EnsureChatInDb(chatID, "test-warn-limit"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ? AND user_id = ?", chatID, userID).Delete(&Warns{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&WarnSettings{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
	})

	var lastCount int
	for i := 1; i <= limit; i++ {
		reason := fmt.Sprintf("reason %d", i)
		n, _ := WarnUser(userID, chatID, reason)
		lastCount = n
	}

	if lastCount != limit {
		t.Fatalf("expected warn count=%d after %d warns, got %d", limit, limit, lastCount)
	}

	setting := GetWarnSetting(chatID)
	if lastCount >= setting.WarnLimit {
		// Limit reached — this is expected behavior.
	} else {
		t.Fatalf("expected to reach warn limit %d, got %d", setting.WarnLimit, lastCount)
	}
}

func TestRemoveWarn(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	chatID := base
	userID := base + 1

	if err := EnsureChatInDb(chatID, "test-remove-warn"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ? AND user_id = ?", chatID, userID).Delete(&Warns{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&WarnSettings{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
	})

	// Add two warns then remove one
	WarnUser(userID, chatID, "first")
	WarnUser(userID, chatID, "second")

	removed := RemoveWarn(userID, chatID)
	if !removed {
		t.Fatalf("RemoveWarn() returned false, expected true")
	}

	numWarns, reasons := GetWarns(userID, chatID)
	if numWarns != 1 {
		t.Fatalf("expected numWarns=1 after remove, got %d", numWarns)
	}
	if len(reasons) != 1 {
		t.Fatalf("expected 1 reason after remove, got %d", len(reasons))
	}
	if reasons[0] != "first" {
		t.Fatalf("expected remaining reason='first', got %q", reasons[0])
	}
}

func TestResetWarns(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	chatID := base
	userID := base + 1

	if err := EnsureChatInDb(chatID, "test-reset-warns"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ? AND user_id = ?", chatID, userID).Delete(&Warns{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&WarnSettings{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
	})

	WarnUser(userID, chatID, "a")
	WarnUser(userID, chatID, "b")
	WarnUser(userID, chatID, "c")

	numBefore, _ := GetWarns(userID, chatID)
	if numBefore != 3 {
		t.Fatalf("expected 3 warns before reset, got %d", numBefore)
	}

	ok := ResetUserWarns(userID, chatID)
	if !ok {
		t.Fatalf("ResetUserWarns() returned false")
	}

	numAfter, reasons := GetWarns(userID, chatID)
	if numAfter != 0 {
		t.Fatalf("expected 0 warns after reset, got %d", numAfter)
	}
	if len(reasons) != 0 {
		t.Fatalf("expected 0 reasons after reset, got %d", len(reasons))
	}
}

func TestSetWarnLimit(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	if err := EnsureChatInDb(chatID, "test-set-warn-limit"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&WarnSettings{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
	})

	SetWarnLimit(chatID, 5)

	settings := GetWarnSetting(chatID)
	if settings.WarnLimit != 5 {
		t.Fatalf("expected WarnLimit=5, got %d", settings.WarnLimit)
	}
}

func TestSetWarnMode(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	if err := EnsureChatInDb(chatID, "test-set-warn-mode"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&WarnSettings{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
	})

	SetWarnMode(chatID, "ban")

	settings := GetWarnSetting(chatID)
	if settings.WarnMode != "ban" {
		t.Fatalf("expected WarnMode='ban', got %q", settings.WarnMode)
	}
}

func TestWarnWithEmptyReason(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	chatID := base
	userID := base + 1

	if err := EnsureChatInDb(chatID, "test-warn-empty-reason"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ? AND user_id = ?", chatID, userID).Delete(&Warns{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&WarnSettings{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
	})

	numWarns, reasons := WarnUser(userID, chatID, "")
	if numWarns != 1 {
		t.Fatalf("expected numWarns=1, got %d", numWarns)
	}
	if len(reasons) != 1 {
		t.Fatalf("expected 1 reason even for empty reason, got %d", len(reasons))
	}
	// Empty reason should be stored as a default "No Reason" placeholder
	if reasons[0] == "" {
		t.Fatalf("expected a non-empty placeholder for empty reason, got empty string")
	}
}

func TestResetWarns_NoWarns(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	chatID := base
	userID := base + 1

	if err := EnsureChatInDb(chatID, "test-reset-no-warns"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ? AND user_id = ?", chatID, userID).Delete(&Warns{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&WarnSettings{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
	})

	// Reset when user has no warns — should succeed without error
	ok := ResetUserWarns(userID, chatID)
	if !ok {
		t.Fatalf("ResetUserWarns() returned false when no warns exist")
	}
}

func TestConcurrentWarns(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	chatID := base
	userID := base + 1

	if err := EnsureChatInDb(chatID, "test-concurrent-warns"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ? AND user_id = ?", chatID, userID).Delete(&Warns{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&WarnSettings{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
	})

	const workers = 10
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(idx int) {
			defer wg.Done()
			WarnUser(userID, chatID, fmt.Sprintf("concurrent reason %d", idx))
		}(i)
	}

	wg.Wait()

	numWarns, reasons := GetWarns(userID, chatID)
	if numWarns != workers {
		t.Fatalf("expected %d warns after concurrent inserts, got %d", workers, numWarns)
	}
	if len(reasons) != workers {
		t.Fatalf("expected %d reasons after concurrent inserts, got %d", workers, len(reasons))
	}
}

func TestGetAllChatWarns(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	chatID := base
	userID1 := base + 1
	userID2 := base + 2

	if err := EnsureChatInDb(chatID, "test-get-all-chat-warns"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&Warns{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&WarnSettings{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
	})

	WarnUser(userID1, chatID, "reason1")
	WarnUser(userID2, chatID, "reason2")

	count := GetAllChatWarns(chatID)
	if count < 2 {
		t.Fatalf("expected at least 2 warned users, got %d", count)
	}
}

func TestResetAllChatWarns(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	chatID := base
	userID1 := base + 1
	userID2 := base + 2

	if err := EnsureChatInDb(chatID, "test-reset-all-chat-warns"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&Warns{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&WarnSettings{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
	})

	WarnUser(userID1, chatID, "a")
	WarnUser(userID2, chatID, "b")

	ok := ResetAllChatWarns(chatID)
	if !ok {
		t.Fatalf("ResetAllChatWarns() returned false")
	}

	count := GetAllChatWarns(chatID)
	if count != 0 {
		t.Fatalf("expected 0 warns after ResetAllChatWarns, got %d", count)
	}
}

func TestRemoveWarn_NoWarns(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	chatID := base
	userID := base + 1

	if err := EnsureChatInDb(chatID, "test-remove-warn-none"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ? AND user_id = ?", chatID, userID).Delete(&Warns{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&WarnSettings{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
	})

	// RemoveWarn on user with no prior warns should return false gracefully
	removed := RemoveWarn(userID, chatID)
	if removed {
		t.Fatalf("expected RemoveWarn=false when user has no warns")
	}
}
