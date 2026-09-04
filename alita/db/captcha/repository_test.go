package captcha

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	dbcache "github.com/divkix/Alita_Robot/alita/db/cache"
	dbmodels "github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

func skipIfNoDb(t *testing.T) {
	t.Helper()
	if db.DB == nil {
		t.Skip("requires database connection")
	}
}

func ensureCaptchaParents(t *testing.T, userID, chatID int64) {
	t.Helper()
	if err := db.DB.Where("user_id = ?", userID).FirstOrCreate(&dbmodels.User{UserId: userID}).Error; err != nil {
		t.Fatalf("create captcha user parent: %v", err)
	}
	if err := db.DB.Where("chat_id = ?", chatID).FirstOrCreate(&dbmodels.Chat{ChatId: chatID}).Error; err != nil {
		t.Fatalf("create captcha chat parent: %v", err)
	}
	t.Cleanup(func() {
		_ = db.DB.Where("user_id = ?", userID).Delete(&dbmodels.User{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&dbmodels.Chat{}).Error
	})
}

func TestDeleteCaptchaAttemptByIDAtomicSingleClaim(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	userID := base
	chatID := base + 1
	ensureCaptchaParents(t, userID, chatID)

	attempt, err := CreateCaptchaAttemptPreMessage(userID, chatID, "42", 2)
	if err != nil {
		t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
	}
	if attempt == nil {
		t.Fatalf("expected captcha attempt, got nil")
	}
	if err := StoreMessageForCaptcha(userID, chatID, attempt.ID, 1, "pending", "", ""); err != nil {
		t.Fatalf("StoreMessageForCaptcha() error = %v", err)
	}
	completed, err := CompleteCaptchaAttemptAtomic(
		attempt.ID,
		userID,
		chatID,
		"wrong",
		attempt.MessageID,
		attempt.RefreshCount,
	)
	if err != nil {
		t.Fatalf("CompleteCaptchaAttemptAtomic(wrong) error = %v", err)
	}
	if completed {
		t.Fatal("CompleteCaptchaAttemptAtomic(wrong) claimed the attempt")
	}

	t.Cleanup(func() {
		if err := db.DB.Where("id = ?", attempt.ID).Delete(&dbmodels.CaptchaAttempts{}).Error; err != nil {
			t.Fatalf("cleanup Delete(CaptchaAttempts) error: %v", err)
		}
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
	stored, err := CountStoredMessagesForAttempt(attempt.ID)
	if err != nil {
		t.Fatalf("CountStoredMessagesForAttempt() error = %v", err)
	}
	if stored != 0 {
		t.Fatalf("stored messages after atomic claim = %d, want 0", stored)
	}
}

func TestCaptchaSettingsCacheInvalidation(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&dbmodels.CaptchaSettings{}).Error
		if m := cache.GetMarshal(); m != nil {
			_ = m.Delete(cache.Context, dbcache.CacheKey("captcha_settings", chatID))
		}
	})

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

	if err := SetCaptchaEnabled(chatID, true); err != nil {
		t.Fatalf("SetCaptchaEnabled(true) error = %v", err)
	}

	// If cache is available, verify the old default is no longer served
	if m := cache.GetMarshal(); m != nil {
		var cached dbmodels.CaptchaSettings
		_, cacheErr := m.Get(cache.Context, dbcache.CacheKey("captcha_settings", chatID), &cached)
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

	expectedKey := fmt.Sprintf("alita:captcha_settings:%d", chatID)
	actualKey := dbcache.CacheKey("captcha_settings", chatID)
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
		if err := db.DB.Where("id = ?", attempt.ID).Delete(&dbmodels.CaptchaAttempts{}).Error; err != nil {
			t.Fatalf("cleanup Delete(CaptchaAttempts) error: %v", err)
		}
	})

	fetched, err := GetCaptchaAttemptByID(attempt.ID)
	if err != nil {
		t.Fatalf("GetCaptchaAttemptByID() error = %v", err)
	}
	if fetched == nil {
		t.Fatal("GetCaptchaAttemptByID() returned nil")
	}

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
		if err := db.DB.Where("id = ?", attempt.ID).Delete(&dbmodels.CaptchaAttempts{}).Error; err != nil {
			t.Fatalf("cleanup Delete(CaptchaAttempts) error: %v", err)
		}
	})

	if attempt.Attempts != 0 {
		t.Fatalf("initial Attempts = %d, want 0", attempt.Attempts)
	}

	updated, err := IncrementCaptchaAttempts(
		attempt.ID,
		userID,
		chatID,
		attempt.Answer,
		attempt.MessageID,
		attempt.RefreshCount,
	)
	if err != nil {
		t.Fatalf("IncrementCaptchaAttempts() first error = %v", err)
	}
	if updated.Attempts != 1 {
		t.Fatalf("after first increment Attempts = %d, want 1", updated.Attempts)
	}

	updated, err = IncrementCaptchaAttempts(
		updated.ID,
		userID,
		chatID,
		updated.Answer,
		updated.MessageID,
		updated.RefreshCount,
	)
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
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&dbmodels.CaptchaSettings{}).Error
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
		if err := DeleteStoredMessagesForAttempt(attempt.ID); err != nil {
			t.Fatalf("cleanup DeleteStoredMessagesForAttempt error: %v", err)
		}
		if err := db.DB.Where("id = ?", attempt.ID).Delete(&dbmodels.CaptchaAttempts{}).Error; err != nil {
			t.Fatalf("cleanup Delete(CaptchaAttempts) error: %v", err)
		}
	})

	for i := 0; i < 3; i++ {
		if err := StoreMessageForCaptcha(userID, chatID, attempt.ID, 1, fmt.Sprintf("msg%d", i), "", ""); err != nil {
			t.Fatalf("StoreMessageForCaptcha() msg%d error = %v", i, err)
		}
	}

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

	pastTime := time.Now().Add(-1 * time.Hour)
	if err := CreateMutedUser(userID, chatID, pastTime); err != nil {
		t.Fatalf("CreateMutedUser() error = %v", err)
	}

	expired, err := GetUsersToUnmute()
	if err != nil {
		t.Fatalf("GetUsersToUnmute() error = %v", err)
	}

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

	if _, err := DeleteMutedUsersByIDs([]uint{foundID}); err != nil {
		t.Fatalf("DeleteMutedUsersByIDs() error = %v", err)
	}

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

func TestCreateMutedUserUpdatesExistingSchedule(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	userID, chatID := base+1, base+2
	ensureCaptchaParents(t, userID, chatID)
	first := time.Now().Add(time.Hour)
	second := time.Now().Add(2 * time.Hour)
	t.Cleanup(func() {
		_ = DeleteMutedUser(userID, chatID)
	})

	if err := CreateMutedUser(userID, chatID, first); err != nil {
		t.Fatalf("CreateMutedUser(first) error = %v", err)
	}
	if err := CreateMutedUser(userID, chatID, second); err != nil {
		t.Fatalf("CreateMutedUser(second) error = %v", err)
	}

	users, err := GetMutedUsersForChat(chatID)
	if err != nil {
		t.Fatalf("GetMutedUsersForChat() error = %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("muted user rows = %d, want 1", len(users))
	}
	if users[0].UnmuteAt.Before(second.Add(-time.Second)) {
		t.Fatalf("UnmuteAt = %v, want updated schedule near %v", users[0].UnmuteAt, second)
	}
}

func TestCreateMutedUserConcurrentUpsert(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	userID, chatID := base+10, base+11
	if err := db.DB.Create(&dbmodels.User{UserId: userID}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.DB.Create(&dbmodels.Chat{ChatId: chatID}).Error; err != nil {
		t.Fatalf("create chat: %v", err)
	}
	t.Cleanup(func() {
		_ = DeleteMutedUser(userID, chatID)
		_ = db.DB.Where("user_id = ?", userID).Delete(&dbmodels.User{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&dbmodels.Chat{}).Error
	})

	const workers = 12
	errs := make(chan error, workers)
	start := make(chan struct{})
	var wait sync.WaitGroup
	wait.Add(workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wait.Done()
			<-start
			errs <- CreateMutedUser(userID, chatID, time.Now().Add(time.Duration(i)*time.Minute))
		}(i)
	}
	close(start)
	wait.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("CreateMutedUser() error = %v", err)
		}
	}

	users, err := GetMutedUsersForChat(chatID)
	if err != nil {
		t.Fatalf("GetMutedUsersForChat() error = %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("muted user rows = %d, want 1", len(users))
	}
}

func TestCreateCaptchaAttemptReplacesExistingChallenge(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	userID, chatID := base+20, base+21
	ensureCaptchaParents(t, userID, chatID)
	first, err := CreateCaptchaAttemptPreMessage(userID, chatID, "first", 2)
	if err != nil {
		t.Fatalf("CreateCaptchaAttemptPreMessage(first) error = %v", err)
	}
	if err := UpdateCaptchaAttemptMessageID(first.ID, 123); err != nil {
		t.Fatalf("UpdateCaptchaAttemptMessageID() error = %v", err)
	}
	if err := StoreMessageForCaptcha(userID, chatID, first.ID, 1, "pending", "", ""); err != nil {
		t.Fatalf("StoreMessageForCaptcha() error = %v", err)
	}
	t.Cleanup(func() {
		_ = DeleteCaptchaAttempt(userID, chatID)
	})

	second, err := CreateCaptchaAttemptPreMessage(userID, chatID, "second", 2)
	if err != nil {
		t.Fatalf("CreateCaptchaAttemptPreMessage(second) error = %v", err)
	}
	if second.ID == first.ID {
		t.Fatalf("replacement reused attempt ID %d", first.ID)
	}
	if second.PreviousMessageID != 123 {
		t.Fatalf("PreviousMessageID = %d, want 123", second.PreviousMessageID)
	}

	var count int64
	if err := db.DB.Model(&dbmodels.CaptchaAttempts{}).
		Where("user_id = ? AND chat_id = ?", userID, chatID).
		Count(&count).Error; err != nil {
		t.Fatalf("count replacement attempts: %v", err)
	}
	if count != 1 {
		t.Fatalf("replacement attempt rows = %d, want 1", count)
	}
	if stored, err := CountStoredMessagesForAttempt(first.ID); err != nil {
		t.Fatalf("CountStoredMessagesForAttempt(first) error = %v", err)
	} else if stored != 0 {
		t.Fatalf("stored messages for replaced attempt = %d, want 0", stored)
	}
}

func TestCreateCaptchaAttemptIfEnabledRejectsDisabledChat(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	userID, chatID := base+25, base+26
	ensureCaptchaParents(t, userID, chatID)
	t.Cleanup(func() {
		_ = DeleteCaptchaAttempt(userID, chatID)
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&dbmodels.CaptchaSettings{}).Error
	})

	if err := SetCaptchaEnabled(chatID, true); err != nil {
		t.Fatalf("SetCaptchaEnabled(true) error = %v", err)
	}
	if _, err := CreateCaptchaAttemptPreMessageIfEnabled(userID, chatID, "42", 2); err != nil {
		t.Fatalf("CreateCaptchaAttemptPreMessageIfEnabled(enabled) error = %v", err)
	}
	if err := SetCaptchaEnabled(chatID, false); err != nil {
		t.Fatalf("SetCaptchaEnabled(false) error = %v", err)
	}
	if _, err := CreateCaptchaAttemptPreMessageIfEnabled(userID, chatID, "42", 2); !errors.Is(err, ErrCaptchaDisabled) {
		t.Fatalf("CreateCaptchaAttemptPreMessageIfEnabled(disabled) error = %v, want ErrCaptchaDisabled", err)
	}
}

func TestCaptchaAttemptClaimsSchedulePermissionRestore(t *testing.T) {
	skipIfNoDb(t)

	tests := []struct {
		name  string
		claim func(*dbmodels.CaptchaAttempts) (bool, error)
	}{
		{
			name: "complete",
			claim: func(attempt *dbmodels.CaptchaAttempts) (bool, error) {
				return CompleteCaptchaAttemptAtomic(
					attempt.ID,
					attempt.UserID,
					attempt.ChatID,
					attempt.Answer,
					attempt.MessageID,
					attempt.RefreshCount,
				)
			},
		},
		{
			name: "release",
			claim: func(attempt *dbmodels.CaptchaAttempts) (bool, error) {
				return ReleaseCaptchaAttemptAtomic(attempt.ID, attempt.UserID, attempt.ChatID)
			},
		},
	}

	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			base := time.Now().UnixNano() + int64(index*10)
			userID, chatID := base+30, base+31
			ensureCaptchaParents(t, userID, chatID)
			attempt, err := CreateCaptchaAttemptPreMessage(userID, chatID, "42", 2)
			if err != nil {
				t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
			}
			if err := UpdateCaptchaAttemptMessageID(attempt.ID, 456); err != nil {
				t.Fatalf("UpdateCaptchaAttemptMessageID() error = %v", err)
			}
			attempt.MessageID = 456
			t.Cleanup(func() {
				_ = DeleteCaptchaAttempt(userID, chatID)
				_ = DeleteMutedUser(userID, chatID)
			})

			claimed, err := test.claim(attempt)
			if err != nil {
				t.Fatalf("claim() error = %v", err)
			}
			if !claimed {
				t.Fatal("claim() = false, want true")
			}
			if remaining, err := GetCaptchaAttemptByID(attempt.ID); err != nil {
				t.Fatalf("GetCaptchaAttemptByID() error = %v", err)
			} else if remaining != nil {
				t.Fatalf("claimed attempt still exists: %+v", remaining)
			}
			scheduled, err := GetMutedUser(userID, chatID)
			if err != nil {
				t.Fatalf("GetMutedUser() error = %v", err)
			}
			if scheduled == nil || scheduled.UnmuteAt.After(time.Now().Add(time.Second)) {
				t.Fatalf("permission restore was not scheduled immediately: %+v", scheduled)
			}
		})
	}
}

func TestDeleteMutedUserIfUnchangedPreservesNewerSchedule(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	userID, chatID := base+40, base+41
	ensureCaptchaParents(t, userID, chatID)
	first := time.Now().UTC().Add(time.Minute)
	second := first.Add(time.Hour)
	t.Cleanup(func() {
		_ = DeleteMutedUser(userID, chatID)
	})

	if err := CreateMutedUser(userID, chatID, first); err != nil {
		t.Fatalf("CreateMutedUser(first) error = %v", err)
	}
	stale, err := GetMutedUser(userID, chatID)
	if err != nil || stale == nil {
		t.Fatalf("GetMutedUser(first) = (%+v, %v)", stale, err)
	}
	if err := CreateMutedUser(userID, chatID, second); err != nil {
		t.Fatalf("CreateMutedUser(second) error = %v", err)
	}

	deleted, err := DeleteMutedUserIfUnchanged(stale.ID, stale.UnmuteAt)
	if err != nil {
		t.Fatalf("DeleteMutedUserIfUnchanged(stale) error = %v", err)
	}
	if deleted {
		t.Fatal("stale worker deleted a newer unmute schedule")
	}
	current, err := GetMutedUser(userID, chatID)
	if err != nil || current == nil {
		t.Fatalf("GetMutedUser(current) = (%+v, %v)", current, err)
	}
	if !current.UnmuteAt.After(stale.UnmuteAt) {
		t.Fatalf("current UnmuteAt = %v, want after stale %v", current.UnmuteAt, stale.UnmuteAt)
	}
}

func TestGetCaptchaAttempt_ExistingAndMissing(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	userID := base + 500
	chatID := base + 501

	attempt, err := GetCaptchaAttempt(userID, chatID)
	if err != nil {
		t.Fatalf("GetCaptchaAttempt() error = %v", err)
	}
	if attempt != nil {
		t.Fatalf("expected nil for missing attempt, got %+v", attempt)
	}

	created, err := CreateCaptchaAttemptPreMessage(userID, chatID, "88", 5)
	if err != nil {
		t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
	}

	t.Cleanup(func() {
		if err := db.DB.Where("id = ?", created.ID).Delete(&dbmodels.CaptchaAttempts{}).Error; err != nil {
			t.Fatalf("cleanup Delete(CaptchaAttempts) error: %v", err)
		}
	})

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

	created, err := CreateCaptchaAttemptPreMessage(userID, chatID, "11", 1)
	if err != nil {
		t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
	}

	t.Cleanup(func() {
		if err := db.DB.Where("id = ?", created.ID).Delete(&dbmodels.CaptchaAttempts{}).Error; err != nil {
			t.Fatalf("cleanup Delete(CaptchaAttempts) error: %v", err)
		}
	})

	// Backdate both created_at and expires_at so expires_at > created_at but both are in the past
	past := time.Now().Add(-10 * time.Minute)
	if err := db.DB.Model(&dbmodels.CaptchaAttempts{}).Where("id = ?", created.ID).Updates(map[string]any{
		"created_at": past,
		"expires_at": past.Add(1 * time.Minute),
	}).Error; err != nil {
		t.Fatalf("failed to backdate timestamps: %v", err)
	}

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

	attempt, err := GetCaptchaAttemptByID(99999999)
	if err != nil {
		t.Fatalf("GetCaptchaAttemptByID() error = %v", err)
	}
	if attempt != nil {
		t.Fatalf("expected nil for missing ID, got %+v", attempt)
	}

	created, err := CreateCaptchaAttemptPreMessage(userID, chatID, "22", 5)
	if err != nil {
		t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
	}

	t.Cleanup(func() {
		if err := db.DB.Where("id = ?", created.ID).Delete(&dbmodels.CaptchaAttempts{}).Error; err != nil {
			t.Fatalf("cleanup Delete(CaptchaAttempts) error: %v", err)
		}
	})

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

	created, err := CreateCaptchaAttemptPreMessage(userID, chatID, "33", 5)
	if err != nil {
		t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
	}

	attempt, err := GetCaptchaAttemptByID(created.ID)
	if err != nil {
		t.Fatalf("GetCaptchaAttemptByID() error = %v", err)
	}
	if attempt == nil {
		t.Fatal("expected non-nil attempt before deletion")
	}

	if err := DeleteCaptchaAttempt(userID, chatID); err != nil {
		t.Fatalf("DeleteCaptchaAttempt() error = %v", err)
	}

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

	for i := int64(0); i < 3; i++ {
		_, err := CreateCaptchaAttemptPreMessage(base+1000+i, chatID, fmt.Sprintf("ans%d", i), 5)
		if err != nil {
			t.Fatalf("CreateCaptchaAttemptPreMessage() user %d error = %v", i, err)
		}
	}

	t.Cleanup(func() {
		if err := db.DB.Where("chat_id = ?", chatID).Delete(&dbmodels.CaptchaAttempts{}).Error; err != nil {
			t.Fatalf("cleanup Delete(CaptchaAttempts) error: %v", err)
		}
	})

	var count int64
	if err := db.DB.Model(&dbmodels.CaptchaAttempts{}).Where("chat_id = ?", chatID).Count(&count).Error; err != nil {
		t.Fatalf("count before delete error = %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 attempts, got %d", count)
	}

	if err := DeleteAllCaptchaAttempts(chatID); err != nil {
		t.Fatalf("DeleteAllCaptchaAttempts() error = %v", err)
	}

	if err := db.DB.Model(&dbmodels.CaptchaAttempts{}).Where("chat_id = ?", chatID).Count(&count).Error; err != nil {
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
		if err := DeleteStoredMessagesForAttempt(attempt.ID); err != nil {
			t.Fatalf("cleanup DeleteStoredMessagesForAttempt error: %v", err)
		}
		if err := db.DB.Where("id = ?", attempt.ID).Delete(&dbmodels.CaptchaAttempts{}).Error; err != nil {
			t.Fatalf("cleanup Delete(CaptchaAttempts) error: %v", err)
		}
	})

	for i := 0; i < 3; i++ {
		if err := StoreMessageForCaptcha(userID, chatID, attempt.ID, 1, fmt.Sprintf("content%d", i), "", ""); err != nil {
			t.Fatalf("StoreMessageForCaptcha() msg %d error = %v", i, err)
		}
	}

	messages, err := GetStoredMessagesForUser(userID, chatID)
	if err != nil {
		t.Fatalf("GetStoredMessagesForUser() error = %v", err)
	}
	if len(messages) != 3 {
		t.Fatalf("GetStoredMessagesForUser() len = %d, want 3", len(messages))
	}

	// Compare deterministically as a set to avoid flaking on DB ordering.
	expectedContents := map[string]bool{"content0": true, "content1": true, "content2": true}
	seen := 0
	for _, msg := range messages {
		if !expectedContents[msg.Content] {
			t.Fatalf("unexpected message content %q", msg.Content)
		}
		if msg.UserID != userID {
			t.Fatalf("message.UserID = %d, want %d", msg.UserID, userID)
		}
		if msg.ChatID != chatID {
			t.Fatalf("message.ChatID = %d, want %d", msg.ChatID, chatID)
		}
		seen++
		delete(expectedContents, msg.Content)
	}
	if seen != 3 || len(expectedContents) != 0 {
		t.Fatalf("expected 3 unique messages, got %d seen, %d missing", seen, len(expectedContents))
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
		if err := db.DB.Where("id = ?", attempt.ID).Delete(&dbmodels.CaptchaAttempts{}).Error; err != nil {
			t.Fatalf("cleanup Delete(CaptchaAttempts) error: %v", err)
		}
	})

	for i := 0; i < 2; i++ {
		if err := StoreMessageForCaptcha(userID, chatID, attempt.ID, 1, fmt.Sprintf("msg%d", i), "", ""); err != nil {
			t.Fatalf("StoreMessageForCaptcha() msg %d error = %v", i, err)
		}
	}

	messages, err := GetStoredMessagesForAttempt(attempt.ID)
	if err != nil {
		t.Fatalf("GetStoredMessagesForAttempt() error = %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("before delete: expected 2 messages, got %d", len(messages))
	}

	if err := DeleteStoredMessagesForAttempt(attempt.ID); err != nil {
		t.Fatalf("DeleteStoredMessagesForAttempt() error = %v", err)
	}

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

	updated, err := IncrementCaptchaAttempts(999999, userID, chatID, "missing", 0, 0)
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

func TestUpdateCaptchaAttemptMessageIDRejectsMissingAttempt(t *testing.T) {
	skipIfNoDb(t)

	err := UpdateCaptchaAttemptMessageID(^uint(0), 10)
	if !errors.Is(err, ErrNoActiveCaptcha) {
		t.Fatalf("UpdateCaptchaAttemptMessageID() error = %v, want ErrNoActiveCaptcha", err)
	}
}

func TestIncrementCaptchaAttemptsRejectsRefreshedChallenge(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	userID := base + 1400
	chatID := base + 1401
	ensureCaptchaParents(t, userID, chatID)
	attempt, err := CreateCaptchaAttemptPreMessage(userID, chatID, "old-answer", 2)
	if err != nil {
		t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.DB.Where("id = ?", attempt.ID).Delete(&dbmodels.CaptchaAttempts{}).Error
	})
	if err := UpdateCaptchaAttemptMessageID(attempt.ID, 10); err != nil {
		t.Fatalf("UpdateCaptchaAttemptMessageID() error = %v", err)
	}
	attempt.MessageID = 10

	refreshed, err := UpdateCaptchaAttemptOnRefreshByID(
		attempt.ID,
		attempt.Answer,
		attempt.MessageID,
		attempt.RefreshCount,
		"new-answer",
		11,
	)
	if err != nil || refreshed == nil {
		t.Fatalf("UpdateCaptchaAttemptOnRefreshByID() = (%+v, %v)", refreshed, err)
	}

	updated, err := IncrementCaptchaAttempts(
		attempt.ID,
		userID,
		chatID,
		attempt.Answer,
		attempt.MessageID,
		attempt.RefreshCount,
	)
	if !errors.Is(err, ErrNoActiveCaptcha) || updated != nil {
		t.Fatalf("stale IncrementCaptchaAttempts() = (%+v, %v), want nil, ErrNoActiveCaptcha", updated, err)
	}

	current, err := GetCaptchaAttemptByID(attempt.ID)
	if err != nil {
		t.Fatalf("GetCaptchaAttemptByID() error = %v", err)
	}
	if current.Attempts != 0 {
		t.Fatalf("attempt count after stale callback = %d, want 0", current.Attempts)
	}
}

func TestCaptchaAttempt_MessageIDAndRefreshLifecycle(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	userID := base + 1400
	chatID := base + 1401

	attempt, err := CreateCaptchaAttemptPreMessage(userID, chatID, "old", 5)
	if err != nil {
		t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.DB.Where("id = ?", attempt.ID).Delete(&dbmodels.CaptchaAttempts{}).Error; err != nil {
			t.Fatalf("cleanup CaptchaAttempts: %v", err)
		}
	})

	if err := UpdateCaptchaAttemptMessageID(attempt.ID, 98765); err != nil {
		t.Fatalf("UpdateCaptchaAttemptMessageID() error = %v", err)
	}
	got, err := GetCaptchaAttemptByID(attempt.ID)
	if err != nil {
		t.Fatalf("GetCaptchaAttemptByID() error = %v", err)
	}
	if got.MessageID != 98765 {
		t.Fatalf("MessageID = %d, want 98765", got.MessageID)
	}

	refreshed, err := UpdateCaptchaAttemptOnRefreshByID(attempt.ID, "old", 98765, 0, "new", 12345)
	if err != nil {
		t.Fatalf("UpdateCaptchaAttemptOnRefreshByID() error = %v", err)
	}
	if refreshed == nil {
		t.Fatal("UpdateCaptchaAttemptOnRefreshByID() returned nil, want attempt")
	}
	if refreshed.Answer != "new" {
		t.Fatalf("Answer = %q, want new", refreshed.Answer)
	}
	if refreshed.MessageID != 12345 {
		t.Fatalf("MessageID = %d, want 12345", refreshed.MessageID)
	}
	if refreshed.RefreshCount != 1 {
		t.Fatalf("RefreshCount = %d, want 1", refreshed.RefreshCount)
	}

	stale, err := UpdateCaptchaAttemptOnRefreshByID(attempt.ID, "old", 98765, 0, "stale", 54321)
	if err != nil {
		t.Fatalf("UpdateCaptchaAttemptOnRefreshByID(stale) error = %v", err)
	}
	if stale != nil {
		t.Fatalf("UpdateCaptchaAttemptOnRefreshByID(stale) = %+v, want nil", stale)
	}

	missing, err := UpdateCaptchaAttemptOnRefreshByID(attempt.ID+999_999, "old", 98765, 0, "missing", 1)
	if err != nil {
		t.Fatalf("UpdateCaptchaAttemptOnRefreshByID(missing) error = %v", err)
	}
	if missing != nil {
		t.Fatalf("UpdateCaptchaAttemptOnRefreshByID(missing) = %+v, want nil", missing)
	}
}

func TestCaptchaAttempt_BulkAndExpiryQueries(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	chatID := base + 1501

	expired, err := CreateCaptchaAttemptPreMessage(base+1502, chatID, "expired", 5)
	if err != nil {
		t.Fatalf("CreateCaptchaAttemptPreMessage(expired) error = %v", err)
	}
	active, err := CreateCaptchaAttemptPreMessage(base+1503, chatID, "active", 5)
	if err != nil {
		t.Fatalf("CreateCaptchaAttemptPreMessage(active) error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.DB.Where("id IN ?", []uint{expired.ID, active.ID}).Delete(&dbmodels.CaptchaAttempts{}).Error; err != nil {
			t.Fatalf("cleanup CaptchaAttempts: %v", err)
		}
	})

	past := time.Now().Add(-10 * time.Minute)
	if err := db.DB.Model(&dbmodels.CaptchaAttempts{}).Where("id = ?", expired.ID).Updates(map[string]any{
		"created_at": past,
		"expires_at": past.Add(time.Minute),
	}).Error; err != nil {
		t.Fatalf("failed to expire attempt: %v", err)
	}

	expiredAttempts, err := GetExpiredCaptchaAttempts()
	if err != nil {
		t.Fatalf("GetExpiredCaptchaAttempts() error = %v", err)
	}
	if !containsCaptchaAttemptID(expiredAttempts, expired.ID) {
		t.Fatalf("GetExpiredCaptchaAttempts() did not include expired attempt %d", expired.ID)
	}
	if containsCaptchaAttemptID(expiredAttempts, active.ID) {
		t.Fatalf("GetExpiredCaptchaAttempts() included active attempt %d", active.ID)
	}

	allAttempts, err := GetAllPendingCaptchaAttempts()
	if err != nil {
		t.Fatalf("GetAllPendingCaptchaAttempts() error = %v", err)
	}
	if !containsCaptchaAttemptID(allAttempts, expired.ID) || !containsCaptchaAttemptID(allAttempts, active.ID) {
		t.Fatalf("GetAllPendingCaptchaAttempts() missing expected attempts")
	}

	deleted, err := DeleteCaptchaAttemptsByIDs([]uint{})
	if err != nil {
		t.Fatalf("DeleteCaptchaAttemptsByIDs(empty) error = %v", err)
	}
	if deleted != 0 {
		t.Fatalf("DeleteCaptchaAttemptsByIDs(empty) deleted = %d, want 0", deleted)
	}

	deleted, err = DeleteCaptchaAttemptsByIDs([]uint{expired.ID, active.ID})
	if err != nil {
		t.Fatalf("DeleteCaptchaAttemptsByIDs() error = %v", err)
	}
	if deleted != 2 {
		t.Fatalf("DeleteCaptchaAttemptsByIDs() deleted = %d, want 2", deleted)
	}
}

func TestStoredMessages_CountAndDeleteByUser(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	userID := base + 1600
	chatID := base + 1601

	attempt, err := CreateCaptchaAttemptPreMessage(userID, chatID, "answer", 5)
	if err != nil {
		t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
	}
	t.Cleanup(func() {
		if err := DeleteStoredMessagesForAttempt(attempt.ID); err != nil {
			t.Fatalf("cleanup stored messages: %v", err)
		}
		if err := db.DB.Where("id = ?", attempt.ID).Delete(&dbmodels.CaptchaAttempts{}).Error; err != nil {
			t.Fatalf("cleanup CaptchaAttempts: %v", err)
		}
	})

	for i := 0; i < 3; i++ {
		if err := StoreMessageForCaptcha(userID, chatID, attempt.ID, 1, fmt.Sprintf("msg-%d", i), "", ""); err != nil {
			t.Fatalf("StoreMessageForCaptcha(%d) error = %v", i, err)
		}
	}

	count, err := CountStoredMessagesForAttempt(attempt.ID)
	if err != nil {
		t.Fatalf("CountStoredMessagesForAttempt() error = %v", err)
	}
	if count != 3 {
		t.Fatalf("CountStoredMessagesForAttempt() = %d, want 3", count)
	}

	if err := DeleteStoredMessagesForUser(userID, chatID); err != nil {
		t.Fatalf("DeleteStoredMessagesForUser() error = %v", err)
	}
	count, err = CountStoredMessagesForAttempt(attempt.ID)
	if err != nil {
		t.Fatalf("CountStoredMessagesForAttempt(after delete) error = %v", err)
	}
	if count != 0 {
		t.Fatalf("CountStoredMessagesForAttempt(after delete) = %d, want 0", count)
	}
}

func containsCaptchaAttemptID(attempts []*dbmodels.CaptchaAttempts, id uint) bool {
	for _, attempt := range attempts {
		if attempt.ID == id {
			return true
		}
	}
	return false
}
