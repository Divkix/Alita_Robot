package db

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

func TestDeleteCaptchaAttemptByIDAtomicSingleClaim(t *testing.T) {
	if err := DB.AutoMigrate(&CaptchaAttempts{}); err != nil {
		t.Fatalf("failed to migrate captcha_attempts: %v", err)
	}

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
	if err := DB.AutoMigrate(&CaptchaSettings{}); err != nil {
		t.Fatalf("failed to migrate captcha_settings: %v", err)
	}

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
