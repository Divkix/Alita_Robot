package db

import (
	"sync"
	"testing"
	"time"
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
