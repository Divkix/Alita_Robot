package db

import (
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
			_ = cache.Marshal.Delete(cache.Context, captchaSettingsCacheKey(chatID))
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
		_, cacheErr := cache.Marshal.Get(cache.Context, captchaSettingsCacheKey(chatID), &cached)
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
	actualKey := captchaSettingsCacheKey(chatID)
	if actualKey != expectedKey {
		t.Fatalf("cache key mismatch: expected %q, got %q", expectedKey, actualKey)
	}
}

func TestCaptchaAttempt_Lifecycle(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	if err := DeleteMutedUser(foundID); err != nil {
		t.Fatalf("DeleteMutedUser() error = %v", err)
	}

	// Verify gone
	afterExpired, err := GetUsersToUnmute()
	if err != nil {
		t.Fatalf("GetUsersToUnmute() after cleanup error = %v", err)
	}
	for _, u := range afterExpired {
		if u.UserID == userID && u.ChatID == chatID {
			t.Fatalf("muted user still present after DeleteMutedUser()")
		}
	}
}
