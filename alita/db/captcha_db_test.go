package db

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

func TestDeleteCaptchaAttemptByIDAtomicSingleClaim(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	userID := base
	chatID := base + 1

	attempt, err := CreateCaptchaAttemptPreMessage(userID, chatID, "42", 2)
	if err != nil {
		t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
	}
	if attempt == nil {
		t.Fatalf("expected captcha attempt, got nil")
	}

	t.Cleanup(func() {
		_ = DB.Where("id = ?", attempt.ID).Delete(&CaptchaAttempts{}).Error
	})

	const workers = 20
	var wg sync.WaitGroup
	wg.Add(workers)

	results := make(chan bool, workers)
	errs := make(chan error, workers)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			claimed, claimErr := DeleteCaptchaAttemptByIDAtomic(attempt.ID, userID, chatID)
			if claimErr != nil {
				errs <- claimErr
				return
			}
			results <- claimed
		}()
	}

	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		t.Fatalf("DeleteCaptchaAttemptByIDAtomic() returned error: %v", err)
	}

	claimedCount := 0
	for claimed := range results {
		if claimed {
			claimedCount++
		}
	}

	if claimedCount != 1 {
		t.Fatalf("expected exactly one successful claim, got %d", claimedCount)
	}

	remaining, err := GetCaptchaAttemptByID(attempt.ID)
	if err != nil {
		t.Fatalf("GetCaptchaAttemptByID() error = %v", err)
	}
	if remaining != nil {
		t.Fatalf("expected attempt to be deleted, got %+v", remaining)
	}
}

func TestCaptchaSettingsCacheInvalidation(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&CaptchaSettings{}).Error
		if cache.Marshal != nil {
			_ = cache.Marshal.Delete(cache.Context, CacheKey("captcha_settings", chatID))
		}
	})

	// Verify defaults when no record exists
	settings, err := GetCaptchaSettings(chatID)
	if err != nil {
		t.Fatalf("GetCaptchaSettings() error = %v", err)
	}
	if settings.Enabled {
		t.Fatalf("expected default Enabled=false")
	}
	if settings.CaptchaMode != "math" {
		t.Fatalf("expected default CaptchaMode='math', got %q", settings.CaptchaMode)
	}

	// SetCaptchaEnabled should create record and invalidate cache
	if err := SetCaptchaEnabled(chatID, true); err != nil {
		t.Fatalf("SetCaptchaEnabled(true) error = %v", err)
	}

	// If cache is available, verify the old default is no longer served
	if cache.Marshal != nil {
		var cached CaptchaSettings
		_, cacheErr := cache.Marshal.Get(cache.Context, CacheKey("captcha_settings", chatID), &cached)
		if cacheErr == nil && !cached.Enabled {
			t.Fatalf("cache was not invalidated after SetCaptchaEnabled(true)")
		}
	}

	settings, err = GetCaptchaSettings(chatID)
	if err != nil {
		t.Fatalf("GetCaptchaSettings() after enable error = %v", err)
	}
	if !settings.Enabled {
		t.Fatalf("expected Enabled=true after SetCaptchaEnabled(true)")
	}

	// SetCaptchaEnabled(false) — zero-value boolean round-trip
	if err := SetCaptchaEnabled(chatID, false); err != nil {
		t.Fatalf("SetCaptchaEnabled(false) error = %v", err)
	}
	settings, err = GetCaptchaSettings(chatID)
	if err != nil {
		t.Fatalf("GetCaptchaSettings() after disable error = %v", err)
	}
	if settings.Enabled {
		t.Fatalf("expected Enabled=false after SetCaptchaEnabled(false)")
	}

	// SetCaptchaMode invalidates cache
	if err := SetCaptchaMode(chatID, "text"); err != nil {
		t.Fatalf("SetCaptchaMode() error = %v", err)
	}
	settings, err = GetCaptchaSettings(chatID)
	if err != nil {
		t.Fatalf("GetCaptchaSettings() after mode change error = %v", err)
	}
	if settings.CaptchaMode != "text" {
		t.Fatalf("expected CaptchaMode='text', got %q", settings.CaptchaMode)
	}

	// SetCaptchaTimeout invalidates cache
	if err := SetCaptchaTimeout(chatID, 5); err != nil {
		t.Fatalf("SetCaptchaTimeout() error = %v", err)
	}
	settings, err = GetCaptchaSettings(chatID)
	if err != nil {
		t.Fatalf("GetCaptchaSettings() after timeout change error = %v", err)
	}
	if settings.Timeout != 5 {
		t.Fatalf("expected Timeout=5, got %d", settings.Timeout)
	}

	// SetCaptchaMaxAttempts invalidates cache
	if err := SetCaptchaMaxAttempts(chatID, 7); err != nil {
		t.Fatalf("SetCaptchaMaxAttempts() error = %v", err)
	}
	settings, err = GetCaptchaSettings(chatID)
	if err != nil {
		t.Fatalf("GetCaptchaSettings() after max_attempts change error = %v", err)
	}
	if settings.MaxAttempts != 7 {
		t.Fatalf("expected MaxAttempts=7, got %d", settings.MaxAttempts)
	}

	// SetCaptchaFailureAction invalidates cache
	if err := SetCaptchaFailureAction(chatID, "ban"); err != nil {
		t.Fatalf("SetCaptchaFailureAction() error = %v", err)
	}
	settings, err = GetCaptchaSettings(chatID)
	if err != nil {
		t.Fatalf("GetCaptchaSettings() after failure_action change error = %v", err)
	}
	if settings.FailureAction != "ban" {
		t.Fatalf("expected FailureAction='ban', got %q", settings.FailureAction)
	}

	// Verify cache key uses correct prefix
	expectedKey := fmt.Sprintf("alita:captcha_settings:%d", chatID)
	actualKey := CacheKey("captcha_settings", chatID)
	if actualKey != expectedKey {
		t.Fatalf("cache key mismatch: expected %q, got %q", expectedKey, actualKey)
	}
}

func TestCaptchaAttempt_Lifecycle(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	userID := base + 100
	chatID := base + 101

	attempt, err := CreateCaptchaAttemptPreMessage(userID, chatID, "42", 2)
	if err != nil {
		t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
	}
	if attempt == nil {
		t.Fatalf("expected non-nil attempt, got nil")
	}

	t.Cleanup(func() {
		_ = DB.Where("id = ?", attempt.ID).Delete(&CaptchaAttempts{}).Error
	})

	// Read back by ID
	fetched, err := GetCaptchaAttemptByID(attempt.ID)
	if err != nil {
		t.Fatalf("GetCaptchaAttemptByID() error = %v", err)
	}
	if fetched == nil {
		t.Fatal("GetCaptchaAttemptByID() returned nil")
	}

	// Verify answer
	if fetched.Answer != "42" {
		t.Fatalf("Answer = %q, want %q", fetched.Answer, "42")
	}
}

func TestCaptchaAttempt_IncrementAttempts(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	userID := base + 200
	chatID := base + 201

	attempt, err := CreateCaptchaAttemptPreMessage(userID, chatID, "99", 2)
	if err != nil {
		t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
	}
	if attempt == nil {
		t.Fatalf("expected non-nil attempt, got nil")
	}

	t.Cleanup(func() {
		_ = DB.Where("id = ?", attempt.ID).Delete(&CaptchaAttempts{}).Error
	})

	// Initial Attempts == 0
	if attempt.Attempts != 0 {
		t.Fatalf("initial Attempts = %d, want 0", attempt.Attempts)
	}

	// Increment to 1
	updated, err := IncrementCaptchaAttempts(userID, chatID)
	if err != nil {
		t.Fatalf("IncrementCaptchaAttempts() first error = %v", err)
	}
	if updated.Attempts != 1 {
		t.Fatalf("after first increment Attempts = %d, want 1", updated.Attempts)
	}

	// Increment to 2
	updated, err = IncrementCaptchaAttempts(userID, chatID)
	if err != nil {
		t.Fatalf("IncrementCaptchaAttempts() second error = %v", err)
	}
	if updated.Attempts != 2 {
		t.Fatalf("after second increment Attempts = %d, want 2", updated.Attempts)
	}
}

func TestGetCaptchaSettings_NonExistentChat(t *testing.T) {
	skipIfNoDb(t)

	// Very large unique ID to avoid collision with other tests
	chatID := time.Now().UnixNano() + 9_000_000_000_000

	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&CaptchaSettings{}).Error
	})

	settings, err := GetCaptchaSettings(chatID)
	if err != nil {
		t.Fatalf("GetCaptchaSettings() error = %v", err)
	}
	if settings == nil {
		t.Fatal("GetCaptchaSettings() returned nil, want non-nil defaults")
	}
	if settings.Enabled {
		t.Fatalf("expected default Enabled=false, got true")
	}
	if settings.CaptchaMode != "math" {
		t.Fatalf("expected default CaptchaMode='math', got %q", settings.CaptchaMode)
	}
	if settings.Timeout != 2 {
		t.Fatalf("expected default Timeout=2, got %d", settings.Timeout)
	}
	if settings.FailureAction != "kick" {
		t.Fatalf("expected default FailureAction='kick', got %q", settings.FailureAction)
	}
	if settings.MaxAttempts != 3 {
		t.Fatalf("expected default MaxAttempts=3, got %d", settings.MaxAttempts)
	}
}

func TestStoredMessages_CRUD(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	userID := base + 300
	chatID := base + 301

	attempt, err := CreateCaptchaAttemptPreMessage(userID, chatID, "77", 2)
	if err != nil {
		t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
	}
	if attempt == nil {
		t.Fatalf("expected non-nil attempt, got nil")
	}

	t.Cleanup(func() {
		_ = DeleteStoredMessagesForAttempt(attempt.ID)
		_ = DB.Where("id = ?", attempt.ID).Delete(&CaptchaAttempts{}).Error
	})

	// Store 3 messages
	for i := 0; i < 3; i++ {
		if err := StoreMessageForCaptcha(userID, chatID, attempt.ID, 1, fmt.Sprintf("msg%d", i), "", ""); err != nil {
			t.Fatalf("StoreMessageForCaptcha() msg%d error = %v", i, err)
		}
	}

	// Get stored messages by attemptID -> should have 3
	messages, err := GetStoredMessagesForAttempt(attempt.ID)
	if err != nil {
		t.Fatalf("GetStoredMessagesForAttempt() error = %v", err)
	}
	if len(messages) != 3 {
		t.Fatalf("GetStoredMessagesForAttempt() len = %d, want 3", len(messages))
	}
}

func TestCaptchaMutedUsers_CleanupExpired(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	userID := base + 400
	chatID := base + 401

	// Insert a muted user with a past UnmuteAt time (already expired)
	pastTime := time.Now().Add(-1 * time.Hour)
	if err := CreateMutedUser(userID, chatID, pastTime); err != nil {
		t.Fatalf("CreateMutedUser() error = %v", err)
	}

	// GetUsersToUnmute should find this user
	expired, err := GetUsersToUnmute()
	if err != nil {
		t.Fatalf("GetUsersToUnmute() error = %v", err)
	}

	// Find our muted user in the results
	var foundID uint
	for _, u := range expired {
		if u.UserID == userID && u.ChatID == chatID {
			foundID = u.ID
			break
		}
	}
	if foundID == 0 {
		t.Fatalf("muted user (userID=%d, chatID=%d) not found in GetUsersToUnmute() results", userID, chatID)
	}

	// Delete the muted user (simulates cleanup)
	if _, err := DeleteMutedUsersByIDs([]uint{foundID}); err != nil {
		t.Fatalf("DeleteMutedUsersByIDs() error = %v", err)
	}

	// Verify gone
	afterExpired, err := GetUsersToUnmute()
	if err != nil {
		t.Fatalf("GetUsersToUnmute() after cleanup error = %v", err)
	}
	for _, u := range afterExpired {
		if u.UserID == userID && u.ChatID == chatID {
			t.Fatalf("muted user still present after DeleteMutedUsersByIDs()")
		}
	}
}

func TestGetCaptchaAttempt_ExistingAndMissing(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	userID := base + 500
	chatID := base + 501

	// No attempt exists yet — should return nil
	attempt, err := GetCaptchaAttempt(userID, chatID)
	if err != nil {
		t.Fatalf("GetCaptchaAttempt() error = %v", err)
	}
	if attempt != nil {
		t.Fatalf("expected nil for missing attempt, got %+v", attempt)
	}

	// Create an attempt
	created, err := CreateCaptchaAttemptPreMessage(userID, chatID, "88", 5)
	if err != nil {
		t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
	}

	t.Cleanup(func() {
		_ = DB.Where("id = ?", created.ID).Delete(&CaptchaAttempts{}).Error
	})

	// Now it should be found
	attempt, err = GetCaptchaAttempt(userID, chatID)
	if err != nil {
		t.Fatalf("GetCaptchaAttempt() error = %v", err)
	}
	if attempt == nil {
		t.Fatal("expected non-nil attempt, got nil")
	}
	if attempt.Answer != "88" {
		t.Fatalf("Answer = %q, want %q", attempt.Answer, "88")
	}
}

func TestGetCaptchaAttempt_Expired(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	userID := base + 600
	chatID := base + 601

	// Create an attempt with a 1-minute timeout, then backdate expires_at
	created, err := CreateCaptchaAttemptPreMessage(userID, chatID, "11", 1)
	if err != nil {
		t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
	}

	t.Cleanup(func() {
		_ = DB.Where("id = ?", created.ID).Delete(&CaptchaAttempts{}).Error
	})

	// Backdate both created_at and expires_at so expires_at > created_at but both are in the past
	past := time.Now().Add(-10 * time.Minute)
	if err := DB.Model(&CaptchaAttempts{}).Where("id = ?", created.ID).Updates(map[string]any{
		"created_at": past,
		"expires_at": past.Add(1 * time.Minute),
	}).Error; err != nil {
		t.Fatalf("failed to backdate timestamps: %v", err)
	}

	// Expired attempt should not be returned
	attempt, err := GetCaptchaAttempt(userID, chatID)
	if err != nil {
		t.Fatalf("GetCaptchaAttempt() error = %v", err)
	}
	if attempt != nil {
		t.Fatalf("expected nil for expired attempt, got %+v", attempt)
	}
}

func TestGetCaptchaAttemptByID_ExistingAndMissing(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	userID := base + 700
	chatID := base + 701

	// Missing ID — should return nil
	attempt, err := GetCaptchaAttemptByID(99999999)
	if err != nil {
		t.Fatalf("GetCaptchaAttemptByID() error = %v", err)
	}
	if attempt != nil {
		t.Fatalf("expected nil for missing ID, got %+v", attempt)
	}

	// Create an attempt
	created, err := CreateCaptchaAttemptPreMessage(userID, chatID, "22", 5)
	if err != nil {
		t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
	}

	t.Cleanup(func() {
		_ = DB.Where("id = ?", created.ID).Delete(&CaptchaAttempts{}).Error
	})

	// Read back by ID
	attempt, err = GetCaptchaAttemptByID(created.ID)
	if err != nil {
		t.Fatalf("GetCaptchaAttemptByID() error = %v", err)
	}
	if attempt == nil {
		t.Fatal("expected non-nil attempt, got nil")
	}
	if attempt.ID != created.ID {
		t.Fatalf("ID = %d, want %d", attempt.ID, created.ID)
	}
}

func TestDeleteCaptchaAttempt(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	userID := base + 800
	chatID := base + 801

	// Create an attempt
	created, err := CreateCaptchaAttemptPreMessage(userID, chatID, "33", 5)
	if err != nil {
		t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
	}

	// Verify it exists
	attempt, err := GetCaptchaAttemptByID(created.ID)
	if err != nil {
		t.Fatalf("GetCaptchaAttemptByID() error = %v", err)
	}
	if attempt == nil {
		t.Fatal("expected non-nil attempt before deletion")
	}

	// Delete it
	if err := DeleteCaptchaAttempt(userID, chatID); err != nil {
		t.Fatalf("DeleteCaptchaAttempt() error = %v", err)
	}

	// Verify it's gone
	attempt, err = GetCaptchaAttemptByID(created.ID)
	if err != nil {
		t.Fatalf("GetCaptchaAttemptByID() after delete error = %v", err)
	}
	if attempt != nil {
		t.Fatalf("expected nil after deletion, got %+v", attempt)
	}
}

func TestDeleteAllCaptchaAttempts(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	chatID := base + 900

	// Create multiple attempts for the same chat with different users
	for i := int64(0); i < 3; i++ {
		_, err := CreateCaptchaAttemptPreMessage(base+1000+i, chatID, fmt.Sprintf("ans%d", i), 5)
		if err != nil {
			t.Fatalf("CreateCaptchaAttemptPreMessage() user %d error = %v", i, err)
		}
	}

	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&CaptchaAttempts{}).Error
	})

	// Verify all 3 exist
	var count int64
	if err := DB.Model(&CaptchaAttempts{}).Where("chat_id = ?", chatID).Count(&count).Error; err != nil {
		t.Fatalf("count before delete error = %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 attempts, got %d", count)
	}

	// Delete all for this chat
	if err := DeleteAllCaptchaAttempts(chatID); err != nil {
		t.Fatalf("DeleteAllCaptchaAttempts() error = %v", err)
	}

	// Verify all gone
	if err := DB.Model(&CaptchaAttempts{}).Where("chat_id = ?", chatID).Count(&count).Error; err != nil {
		t.Fatalf("count after delete error = %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 attempts after delete, got %d", count)
	}
}

func TestStoreMessageForCaptchaAndGetStoredMessagesForUser(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	userID := base + 1100
	chatID := base + 1101

	attempt, err := CreateCaptchaAttemptPreMessage(userID, chatID, "44", 5)
	if err != nil {
		t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
	}
	if attempt == nil {
		t.Fatalf("expected non-nil attempt, got nil")
	}

	t.Cleanup(func() {
		_ = DeleteStoredMessagesForAttempt(attempt.ID)
		_ = DB.Where("id = ?", attempt.ID).Delete(&CaptchaAttempts{}).Error
	})

	// Store 3 messages for this user/chat
	for i := 0; i < 3; i++ {
		if err := StoreMessageForCaptcha(userID, chatID, attempt.ID, 1, fmt.Sprintf("content%d", i), "", ""); err != nil {
			t.Fatalf("StoreMessageForCaptcha() msg %d error = %v", i, err)
		}
	}

	// Get stored messages via GetStoredMessagesForUser
	messages, err := GetStoredMessagesForUser(userID, chatID)
	if err != nil {
		t.Fatalf("GetStoredMessagesForUser() error = %v", err)
	}
	if len(messages) != 3 {
		t.Fatalf("GetStoredMessagesForUser() len = %d, want 3", len(messages))
	}
	for i, msg := range messages {
		expected := fmt.Sprintf("content%d", i)
		if msg.Content != expected {
			t.Fatalf("messages[%d].Content = %q, want %q", i, msg.Content, expected)
		}
		if msg.UserID != userID {
			t.Fatalf("messages[%d].UserID = %d, want %d", i, msg.UserID, userID)
		}
		if msg.ChatID != chatID {
			t.Fatalf("messages[%d].ChatID = %d, want %d", i, msg.ChatID, chatID)
		}
	}
}

func TestDeleteStoredMessagesForAttempt(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	userID := base + 1200
	chatID := base + 1201

	attempt, err := CreateCaptchaAttemptPreMessage(userID, chatID, "55", 5)
	if err != nil {
		t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
	}
	if attempt == nil {
		t.Fatalf("expected non-nil attempt, got nil")
	}

	t.Cleanup(func() {
		_ = DB.Where("id = ?", attempt.ID).Delete(&CaptchaAttempts{}).Error
	})

	// Store 2 messages
	for i := 0; i < 2; i++ {
		if err := StoreMessageForCaptcha(userID, chatID, attempt.ID, 1, fmt.Sprintf("msg%d", i), "", ""); err != nil {
			t.Fatalf("StoreMessageForCaptcha() msg %d error = %v", i, err)
		}
	}

	// Verify they exist
	messages, err := GetStoredMessagesForAttempt(attempt.ID)
	if err != nil {
		t.Fatalf("GetStoredMessagesForAttempt() error = %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("before delete: expected 2 messages, got %d", len(messages))
	}

	// Delete them
	if err := DeleteStoredMessagesForAttempt(attempt.ID); err != nil {
		t.Fatalf("DeleteStoredMessagesForAttempt() error = %v", err)
	}

	// Verify they're gone
	messages, err = GetStoredMessagesForAttempt(attempt.ID)
	if err != nil {
		t.Fatalf("GetStoredMessagesForAttempt() after delete error = %v", err)
	}
	if len(messages) != 0 {
		t.Fatalf("after delete: expected 0 messages, got %d", len(messages))
	}
}

func TestIncrementCaptchaAttempts_NoActiveCaptcha(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	userID := base + 1300
	chatID := base + 1301

	// No active attempt — should return ErrNoActiveCaptcha
	updated, err := IncrementCaptchaAttempts(userID, chatID)
	if err == nil {
		t.Fatal("expected error for missing attempt, got nil")
	}
	if !errors.Is(err, ErrNoActiveCaptcha) {
		t.Fatalf("expected ErrNoActiveCaptcha, got %v", err)
	}
	if updated != nil {
		t.Fatalf("expected nil result, got %+v", updated)
	}
}
