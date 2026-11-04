//go:build testtools

package joinrequestcaptchas

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	gocache "github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/marshaler"
	gocache_store "github.com/eko/gocache/store/redis/v4"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/db"
	dbcache "github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/captcha"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

func newAttempt(t *testing.T) *models.JoinRequestCaptcha {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	previousMarshal, previousManager, previousClient := cache.GetCacheState()
	manager := gocache.New[any](gocache_store.NewRedis(client))
	cache.SetCacheState(marshaler.New(manager), manager, client)
	dbcache.ResetLocalForTest()
	t.Cleanup(func() {
		cache.SetCacheState(previousMarshal, previousManager, previousClient)
		dbcache.ResetLocalForTest()
	})
	chatID := -time.Now().UnixNano()
	userID := -chatID
	if err := chats.EnsureChatInDb(chatID, "Join request captcha"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.JoinRequestCaptcha{}).Error; err != nil {
			t.Error(err)
		}
		if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error; err != nil {
			t.Error(err)
		}
	})
	missing, err := GetJoinRequestCaptcha(chatID, userID)
	if err != nil || missing != nil {
		t.Fatalf("initial attempt = (%+v, %v), want nil, nil", missing, err)
	}
	attempt, err := CreateJoinRequestCaptcha(chatID, userID, 17, userID+100, "Applicant <b>名</b>", "42", 10)
	if err != nil {
		t.Fatal(err)
	}
	return attempt
}

func assertAttempt(t *testing.T, want *models.JoinRequestCaptcha) {
	t.Helper()
	byChat, err := GetJoinRequestCaptcha(want.ChatID, want.UserID)
	if err != nil {
		t.Fatal(err)
	}
	byID, err := GetJoinRequestCaptchaByID(want.ID)
	if err != nil {
		t.Fatal(err)
	}
	var stored models.JoinRequestCaptcha
	if err := db.DB.First(&stored, want.ID).Error; err != nil {
		t.Fatal(err)
	}
	for _, got := range []*models.JoinRequestCaptcha{byChat, byID, &stored} {
		if got == nil || got.ID != want.ID || got.ChatID != want.ChatID || got.UserID != want.UserID ||
			got.UserChatID != want.UserChatID || got.FirstName != want.FirstName || got.MessageID != want.MessageID ||
			got.Answer != want.Answer || got.Attempts != want.Attempts || got.RefreshCount != want.RefreshCount || !got.ExpiresAt.Equal(want.ExpiresAt) {
			t.Fatalf("attempt = %+v, want %+v", got, want)
		}
	}
}

func assertAbsent(t *testing.T, attempt *models.JoinRequestCaptcha) {
	t.Helper()
	got, err := GetJoinRequestCaptcha(attempt.ChatID, attempt.UserID)
	if err != nil || got != nil {
		t.Fatalf("get after terminal mutation = (%+v, %v), want nil, nil", got, err)
	}
	got, err = GetJoinRequestCaptchaByID(attempt.ID)
	if err != nil || got != nil {
		t.Fatalf("get ID after terminal mutation = (%+v, %v), want nil, nil", got, err)
	}
}

func TestJoinRequestCaptchaCachedLifecycle(t *testing.T) {
	attempt := newAttempt(t)
	assertAttempt(t, attempt)
	if attempt.ID == 0 || attempt.ExpiresAt.Sub(attempt.CreatedAt) > 5*time.Minute || attempt.UserChatID == attempt.UserID || attempt.FirstName != "Applicant <b>名</b>" {
		t.Fatalf("invalid persisted applicant challenge: %+v", attempt)
	}
	// Bypassing the repository proves both getters really use the populated cache.
	if err := db.DB.Model(&models.JoinRequestCaptcha{}).Where("id = ?", attempt.ID).Update("answer", "43").Error; err != nil {
		t.Fatal(err)
	}
	for _, getter := range []func() (*models.JoinRequestCaptcha, error){
		func() (*models.JoinRequestCaptcha, error) {
			return GetJoinRequestCaptcha(attempt.ChatID, attempt.UserID)
		},
		func() (*models.JoinRequestCaptcha, error) { return GetJoinRequestCaptchaByID(attempt.ID) },
	} {
		got, err := getter()
		if err != nil || got == nil || got.Answer != "42" {
			t.Fatalf("cached answer = (%+v, %v), want 42", got, err)
		}
	}
	count, err := IncrementJoinRequestCaptchaAttempts(attempt.ID, 0, 3)
	if err != nil || count != 1 {
		t.Fatalf("increment = (%d, %v), want 1, nil", count, err)
	}
	attempt.Answer, attempt.Attempts = "43", 1
	assertAttempt(t, attempt)
	if err := UpdateJoinRequestCaptchaMessageID(attempt.ID, 23); err != nil {
		t.Fatal(err)
	}
	attempt.MessageID = 23
	assertAttempt(t, attempt)
	for generation := range models.CaptchaMaxRefreshes {
		refreshed, err := RefreshJoinRequestCaptcha(attempt.ID, fmt.Sprint(generation), generation, int64(30+generation))
		if err != nil || refreshed == nil {
			t.Fatalf("refresh %d = (%+v, %v)", generation, refreshed, err)
		}
		attempt.Answer, attempt.RefreshCount = fmt.Sprint(generation), generation+1
		attempt.MessageID = int64(30 + generation)
		assertAttempt(t, attempt)
	}
	refreshed, err := RefreshJoinRequestCaptcha(attempt.ID, "over cap", models.CaptchaMaxRefreshes, 99)
	if err != nil || refreshed != nil {
		t.Fatalf("refresh over cap = (%+v, %v), want nil, nil", refreshed, err)
	}
	if _, err := IncrementJoinRequestCaptchaAttempts(attempt.ID, 0, 3); !errors.Is(err, ErrNoActiveCaptcha) {
		t.Fatalf("stale increment error = %v", err)
	}
	completed, err := CompleteJoinRequestCaptchaAtomic(attempt.ID, attempt.UserID, attempt.ChatID, attempt.Answer, 0, 3)
	if err != nil || completed {
		t.Fatalf("stale completion = (%v, %v), want false, nil", completed, err)
	}
	completed, err = CompleteJoinRequestCaptchaAtomic(attempt.ID, attempt.UserID, attempt.ChatID, "wrong", attempt.RefreshCount, 3)
	if err != nil || completed {
		t.Fatalf("wrong completion = (%v, %v), want false, nil", completed, err)
	}
	completed, err = CompleteJoinRequestCaptchaAtomic(attempt.ID, attempt.UserID, attempt.ChatID, attempt.Answer, attempt.RefreshCount, 3)
	if err != nil || !completed {
		t.Fatalf("completion = (%v, %v), want true, nil", completed, err)
	}
	assertAbsent(t, attempt)
}

func TestJoinRequestCaptchaReplacementRejectsOldIdentity(t *testing.T) {
	old := newAttempt(t)
	assertAttempt(t, old)
	replacement, err := CreateJoinRequestCaptcha(old.ChatID, old.UserID, 27, old.UserChatID+1, "Replacement", "new", 2)
	if err != nil {
		t.Fatal(err)
	}
	if replacement.ID == old.ID {
		t.Fatal("replacement reused the old attempt identity")
	}
	assertAttempt(t, replacement)
	got, err := GetJoinRequestCaptchaByID(old.ID)
	if err != nil || got != nil {
		t.Fatalf("old cached identity = (%+v, %v), want nil, nil", got, err)
	}
	if won, err := ReleaseJoinRequestCaptchaAtomic(old.ID, old.UserID, old.ChatID, old.Answer, old.RefreshCount); err != nil || won {
		t.Fatalf("old timeout claim = (%v, %v), want false, nil", won, err)
	}
	if err := DeleteJoinRequestCaptcha(old.ID); err != nil {
		t.Fatal(err)
	}
	assertAttempt(t, replacement)
	if err := DeleteJoinRequestCaptcha(replacement.ID); err != nil {
		t.Fatal(err)
	}
	assertAbsent(t, replacement)
}

func TestJoinRequestCaptchaAtomicAttemptsAndTerminalClaim(t *testing.T) {
	attempt := newAttempt(t)
	const workers = 12
	var wg sync.WaitGroup
	counts := make(chan int, workers)
	errs := make(chan error, workers)
	for range workers {
		wg.Go(func() {
			count, err := IncrementJoinRequestCaptchaAttempts(attempt.ID, 0, 3)
			if err != nil && !errors.Is(err, ErrNoActiveCaptcha) {
				errs <- err
			}
			if err == nil {
				counts <- count
			}
		})
	}
	wg.Wait()
	close(counts)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	seen := map[int]bool{}
	for count := range counts {
		seen[count] = true
	}
	if len(seen) != 3 || !seen[1] || !seen[2] || !seen[3] {
		t.Fatalf("atomic bounded increment counts = %v, want 1,2,3", seen)
	}
	attempt.Attempts = 3
	assertAttempt(t, attempt)
	if won, err := CompleteJoinRequestCaptchaAtomic(attempt.ID, attempt.UserID, attempt.ChatID, attempt.Answer, 0, 3); err != nil || won {
		t.Fatalf("exhausted completion = (%v, %v), want false, nil", won, err)
	}
	// A larger bound lets completion compete with failure on the same row.
	results := make(chan bool, workers)
	errs = make(chan error, workers)
	for i := range workers {
		wg.Go(func() {
			var won bool
			var err error
			if i%2 == 0 {
				won, err = CompleteJoinRequestCaptchaAtomic(attempt.ID, attempt.UserID, attempt.ChatID, attempt.Answer, 0, 4)
			} else {
				won, err = ReleaseJoinRequestCaptchaAtomic(attempt.ID, attempt.UserID, attempt.ChatID, attempt.Answer, 0)
			}
			results <- won
			if err != nil {
				errs <- err
			}
		})
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	winners := 0
	for won := range results {
		if won {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("terminal winners = %d, want 1", winners)
	}
	assertAbsent(t, attempt)
}

func TestJoinRequestCaptchaExpiryAndRecovery(t *testing.T) {
	active := newAttempt(t)
	expired := &models.JoinRequestCaptcha{
		ChatID: active.ChatID, UserID: active.UserID + 1, UserChatID: active.UserChatID + 1,
		Answer: "expired", CreatedAt: time.Now().Add(-2 * time.Hour), ExpiresAt: time.Now().Add(-time.Hour),
	}
	if err := db.DB.Create(expired).Error; err != nil {
		t.Fatal(err)
	}
	assertAttempt(t, expired)
	pending, err := GetPendingJoinRequestCaptchas()
	if err != nil {
		t.Fatal(err)
	}
	found := map[uint]bool{}
	for _, attempt := range pending {
		found[attempt.ID] = true
	}
	if !found[active.ID] || !found[expired.ID] {
		t.Fatalf("recovery did not include active and expired attempts: %v", found)
	}
	stale, err := GetExpiredJoinRequestCaptchas()
	if err != nil {
		t.Fatal(err)
	}
	found = map[uint]bool{}
	for _, attempt := range stale {
		found[attempt.ID] = true
	}
	if found[active.ID] || !found[expired.ID] {
		t.Fatalf("expired selection = %v", found)
	}
	if _, err := IncrementJoinRequestCaptchaAttempts(expired.ID, 0, 3); !errors.Is(err, ErrNoActiveCaptcha) {
		t.Fatalf("expired increment error = %v", err)
	}
	if got, err := RefreshJoinRequestCaptcha(expired.ID, "new", 0, 99); err != nil || got != nil {
		t.Fatalf("expired refresh = (%+v, %v)", got, err)
	}
	if won, err := CompleteJoinRequestCaptchaAtomic(expired.ID, expired.UserID, expired.ChatID, expired.Answer, 0, 3); err != nil || won {
		t.Fatalf("expired completion = (%v, %v)", won, err)
	}
	if won, err := ReleaseJoinRequestCaptchaAtomic(expired.ID, expired.UserID, expired.ChatID, expired.Answer, 0); err != nil || !won {
		t.Fatalf("expired release = (%v, %v)", won, err)
	}
	assertAbsent(t, expired)
	assertAttempt(t, active)
}

func TestJoinRequestCaptchaMutationErrors(t *testing.T) {
	attempt := newAttempt(t)
	failure := errors.New("storage unavailable")
	tests := []struct {
		name string
		kind string
		run  func() error
	}{
		{"create", "create", func() error {
			_, err := CreateJoinRequestCaptcha(attempt.ChatID, attempt.UserID, 1, attempt.UserChatID, "Replacement", "replacement", 2)
			return err
		}},
		{"message", "update", func() error { return UpdateJoinRequestCaptchaMessageID(attempt.ID, 99) }},
		{"increment", "update", func() error {
			_, err := IncrementJoinRequestCaptchaAttempts(attempt.ID, 0, 3)
			return err
		}},
		{"refresh", "update", func() error {
			_, err := RefreshJoinRequestCaptcha(attempt.ID, "new", 0, 99)
			return err
		}},
		{"complete", "delete", func() error {
			_, err := CompleteJoinRequestCaptchaAtomic(attempt.ID, attempt.UserID, attempt.ChatID, attempt.Answer, 0, 3)
			return err
		}},
		{"release", "delete", func() error {
			_, err := ReleaseJoinRequestCaptchaAtomic(attempt.ID, attempt.UserID, attempt.ChatID, attempt.Answer, 0)
			return err
		}},
		{"delete", "delete", func() error { return DeleteJoinRequestCaptcha(attempt.ID) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			callback := "test_join_request_storage_error"
			fail := func(tx *gorm.DB) { _ = tx.AddError(failure) }
			var registerErr error
			switch test.kind {
			case "create":
				registerErr = db.DB.Callback().Create().Before("gorm:create").Register(callback, fail)
				t.Cleanup(func() { _ = db.DB.Callback().Create().Remove(callback) })
			case "update":
				registerErr = db.DB.Callback().Update().Before("gorm:update").Register(callback, fail)
				t.Cleanup(func() { _ = db.DB.Callback().Update().Remove(callback) })
			case "delete":
				registerErr = db.DB.Callback().Delete().Before("gorm:delete").Register(callback, fail)
				t.Cleanup(func() { _ = db.DB.Callback().Delete().Remove(callback) })
			}
			if registerErr != nil {
				t.Fatal(registerErr)
			}
			if err := test.run(); !errors.Is(err, failure) {
				t.Fatalf("mutation error = %v, want %v", err, failure)
			}
			assertAttempt(t, attempt)
		})
	}
}

func TestCaptchaJoinRequestsSettingPersistsFalseAndInvalidatesCache(t *testing.T) {
	attempt := newAttempt(t)
	t.Cleanup(func() {
		if err := db.DB.Where("chat_id = ?", attempt.ChatID).Delete(&models.CaptchaSettings{}).Error; err != nil {
			t.Error(err)
		}
	})
	settings, err := captcha.GetCaptchaSettings(attempt.ChatID)
	if err != nil || settings == nil || settings.JoinRequests || settings.Enabled {
		t.Fatalf("default settings = (%+v, %v), want both captcha modes off", settings, err)
	}
	if err := captcha.SetCaptchaMode(attempt.ChatID, "Join request captcha", "text"); err != nil {
		t.Fatal(err)
	}
	if err := captcha.SetCaptchaTimeout(attempt.ChatID, "Join request captcha", 7); err != nil {
		t.Fatal(err)
	}
	for _, enabled := range []bool{true, false} {
		if err := captcha.SetCaptchaJoinRequests(attempt.ChatID, "", enabled); err != nil {
			t.Fatal(err)
		}
		settings, err = captcha.GetCaptchaSettings(attempt.ChatID)
		if err != nil || settings == nil || settings.JoinRequests != enabled || settings.Enabled ||
			settings.CaptchaMode != "text" || settings.Timeout != 7 {
			t.Fatalf("settings after toggle %v = (%+v, %v)", enabled, settings, err)
		}
		var stored models.CaptchaSettings
		if err := db.DB.Where("chat_id = ?", attempt.ChatID).First(&stored).Error; err != nil {
			t.Fatal(err)
		}
		if stored.JoinRequests != enabled {
			t.Fatalf("persisted join_requests = %v, want %v", stored.JoinRequests, enabled)
		}
	}
	var chat models.Chat
	if err := db.DB.Where("chat_id = ?", attempt.ChatID).First(&chat).Error; err != nil {
		t.Fatal(err)
	}
	if chat.ChatName != "Join request captcha" {
		t.Fatalf("setter lost existing chat title: %q", chat.ChatName)
	}
	failure := errors.New("settings unavailable")
	callback := "test_join_request_setting_error"
	if err := db.DB.Callback().Update().Before("gorm:update").Register(callback, func(tx *gorm.DB) {
		_ = tx.AddError(failure)
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.DB.Callback().Update().Remove(callback) })
	if err := captcha.SetCaptchaJoinRequests(attempt.ChatID, "", true); !errors.Is(err, failure) {
		t.Fatalf("setting write error = %v, want %v", err, failure)
	}
	settings, err = captcha.GetCaptchaSettings(attempt.ChatID)
	if err != nil || settings == nil || settings.JoinRequests {
		t.Fatalf("failed setter changed persisted value: (%+v, %v)", settings, err)
	}
}

func TestJoinRequestCaptchaCompositeUniqueness(t *testing.T) {
	attempt := newAttempt(t)
	duplicate := &models.JoinRequestCaptcha{
		ChatID: attempt.ChatID, UserID: attempt.UserID, UserChatID: attempt.UserChatID,
		Answer: "duplicate", ExpiresAt: time.Now().Add(time.Minute),
	}
	if err := db.DB.Create(duplicate).Error; err == nil {
		t.Fatal("database accepted two attempts for the same chat and applicant")
	}
	assertAttempt(t, attempt)
}
