package db

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

// ---------------------------------------------------------------------------
// EnsureBotInDb
// ---------------------------------------------------------------------------

type fakeBotClient struct {
	response json.RawMessage
	err      error
}

func (f fakeBotClient) RequestWithContext(context.Context, string, string, map[string]any, *gotgbot.RequestOpts) (json.RawMessage, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.response, nil
}

func (f fakeBotClient) GetAPIURL(*gotgbot.RequestOpts) string {
	return "https://api.telegram.org"
}

func (f fakeBotClient) FileURL(token string, tgFilePath string, _ *gotgbot.RequestOpts) string {
	return "https://api.telegram.org/file/bot" + token + "/" + tgFilePath
}

func TestEnsureBotInDbUsesGetMeResponse(t *testing.T) {
	skipIfNoDb(t)

	botID := time.Now().UnixNano()
	bot := &gotgbot.Bot{
		Token: "123:test",
		User: gotgbot.User{
			Id:        botID,
			Username:  "stale_bot",
			FirstName: "Stale",
		},
		BotClient: fakeBotClient{response: json.RawMessage(fmt.Sprintf(
			`{"id":%d,"is_bot":true,"first_name":"Fresh","username":"fresh_bot"}`,
			botID,
		))},
	}
	t.Cleanup(func() {
		if err := DB.Where("user_id = ?", botID).Delete(&User{}).Error; err != nil {
			t.Fatalf("cleanup user: %v", err)
		}
	})

	if err := EnsureBotInDb(bot); err != nil {
		t.Fatalf("EnsureBotInDb() error = %v", err)
	}

	var user User
	if err := DB.Where("user_id = ?", botID).First(&user).Error; err != nil {
		t.Fatalf("expected bot user record: %v", err)
	}
	if user.UserName != "fresh_bot" {
		t.Fatalf("UserName = %q, want fresh_bot", user.UserName)
	}
	if user.Name != "Fresh" {
		t.Fatalf("Name = %q, want Fresh", user.Name)
	}
}

func TestEnsureBotInDbFallsBackToEmbeddedBotOnGetMeError(t *testing.T) {
	skipIfNoDb(t)

	botID := time.Now().UnixNano()
	bot := &gotgbot.Bot{
		Token: "123:test",
		User: gotgbot.User{
			Id:        botID,
			Username:  "fallback_bot",
			FirstName: "Fallback",
		},
		BotClient: fakeBotClient{err: fmt.Errorf("telegram unavailable")},
	}
	t.Cleanup(func() {
		if err := DB.Where("user_id = ?", botID).Delete(&User{}).Error; err != nil {
			t.Fatalf("cleanup user: %v", err)
		}
	})

	if err := EnsureBotInDb(bot); err != nil {
		t.Fatalf("EnsureBotInDb() error = %v", err)
	}

	var user User
	if err := DB.Where("user_id = ?", botID).First(&user).Error; err != nil {
		t.Fatalf("expected fallback bot user record: %v", err)
	}
	if user.UserName != "fallback_bot" {
		t.Fatalf("UserName = %q, want fallback_bot", user.UserName)
	}
	if user.Name != "Fallback" {
		t.Fatalf("Name = %q, want Fallback", user.Name)
	}
}

// ---------------------------------------------------------------------------
// EnsureUserInDb
// ---------------------------------------------------------------------------

func TestEnsureUserInDb(t *testing.T) {
	skipIfNoDb(t)

	userID := time.Now().UnixNano()
	username := fmt.Sprintf("user_%d", userID)
	firstName := "TestFirst"

	err := EnsureUserInDb(userID, username, firstName)
	if err != nil {
		t.Fatalf("EnsureUserInDb() error = %v", err)
	}
	t.Cleanup(func() { DB.Where("user_id = ?", userID).Delete(&User{}) })

	var user User
	if err := DB.Where("user_id = ?", userID).First(&user).Error; err != nil {
		t.Fatalf("expected user %d to exist: %v", userID, err)
	}
	if user.UserName != username {
		t.Errorf("username = %q, want %q", user.UserName, username)
	}
	if user.Name != firstName {
		t.Errorf("name = %q, want %q", user.Name, firstName)
	}
}

func TestEnsureUserInDb_Idempotent(t *testing.T) {
	skipIfNoDb(t)

	userID := time.Now().UnixNano()
	t.Cleanup(func() { DB.Where("user_id = ?", userID).Delete(&User{}) })

	if err := EnsureUserInDb(userID, "user_a", "First"); err != nil {
		t.Fatalf("first EnsureUserInDb() error = %v", err)
	}
	if err := EnsureUserInDb(userID, "user_a", "First"); err != nil {
		t.Fatalf("second EnsureUserInDb() error = %v", err)
	}

	var count int64
	DB.Model(&User{}).Where("user_id = ?", userID).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 user record, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// UpdateUser
// ---------------------------------------------------------------------------

func TestUpdateUser(t *testing.T) {
	skipIfNoDb(t)

	userID := time.Now().UnixNano()
	t.Cleanup(func() { DB.Where("user_id = ?", userID).Delete(&User{}) })

	if err := UpdateUser(userID, "original_user", "OriginalName"); err != nil {
		t.Fatalf("initial UpdateUser() error = %v", err)
	}

	if err := UpdateUser(userID, "updated_user", "UpdatedName"); err != nil {
		t.Fatalf("UpdateUser() update error = %v", err)
	}

	var user User
	if err := DB.Where("user_id = ?", userID).First(&user).Error; err != nil {
		t.Fatalf("expected user to exist: %v", err)
	}
	if user.UserName != "updated_user" {
		t.Errorf("username = %q, want %q", user.UserName, "updated_user")
	}
	if user.Name != "UpdatedName" {
		t.Errorf("name = %q, want %q", user.Name, "UpdatedName")
	}
}

// ---------------------------------------------------------------------------
// GetUserIdByUserName
// ---------------------------------------------------------------------------

func TestGetUserIdByUserName(t *testing.T) {
	skipIfNoDb(t)

	userID := time.Now().UnixNano()
	username := fmt.Sprintf("lookup_user_%d", userID)

	if err := EnsureUserInDb(userID, username, "LookupFirst"); err != nil {
		t.Fatalf("EnsureUserInDb() error = %v", err)
	}
	t.Cleanup(func() { DB.Where("user_id = ?", userID).Delete(&User{}) })

	gotID := GetUserIdByUserName(username)
	if gotID != userID {
		t.Errorf("GetUserIdByUserName(%q) = %d, want %d", username, gotID, userID)
	}
}

func TestGetUserIdByUserName_NotFound(t *testing.T) {
	skipIfNoDb(t)

	gotID := GetUserIdByUserName("nonexistent_user_xyzabc123")
	if gotID != 0 {
		t.Errorf("GetUserIdByUserName() = %d for non-existent user, want 0", gotID)
	}
}

// ---------------------------------------------------------------------------
// GetUserInfoById
// ---------------------------------------------------------------------------

func TestGetUserInfoById(t *testing.T) {
	skipIfNoDb(t)

	userID := time.Now().UnixNano()
	username := fmt.Sprintf("info_user_%d", userID)
	firstName := "InfoFirst"

	if err := EnsureUserInDb(userID, username, firstName); err != nil {
		t.Fatalf("EnsureUserInDb() error = %v", err)
	}
	t.Cleanup(func() { DB.Where("user_id = ?", userID).Delete(&User{}) })

	gotUsername, gotName, found := GetUserInfoById(userID)
	if !found {
		t.Fatalf("GetUserInfoById(%d) found=false, want true", userID)
	}
	if gotUsername != username {
		t.Errorf("username = %q, want %q", gotUsername, username)
	}
	if gotName != firstName {
		t.Errorf("name = %q, want %q", gotName, firstName)
	}
}

func TestGetUserInfoById_NotFound(t *testing.T) {
	skipIfNoDb(t)

	_, _, found := GetUserInfoById(9999999999999999)
	if found {
		t.Error("GetUserInfoById() found=true for non-existent user, want false")
	}
}

// ---------------------------------------------------------------------------
// LoadUsersStats
// ---------------------------------------------------------------------------

func TestLoadUserStats(t *testing.T) {
	skipIfNoDb(t)

	baseline := LoadUsersStats()

	userID := time.Now().UnixNano()
	if err := EnsureUserInDb(userID, "stat_user", "StatFirst"); err != nil {
		t.Fatalf("EnsureUserInDb() error = %v", err)
	}
	t.Cleanup(func() {
		if err := DB.Where("user_id = ?", userID).Delete(&User{}).Error; err != nil {
			t.Fatalf("cleanup delete error = %v", err)
		}
	})

	count := LoadUsersStats()
	if count-baseline != 1 {
		t.Errorf("LoadUsersStats() delta = %d, want 1 (baseline=%d, after=%d)", count-baseline, baseline, count)
	}
}

// ---------------------------------------------------------------------------
// LoadUserActivityStats
// ---------------------------------------------------------------------------

func TestLoadUsersStatsErrorBranch(t *testing.T) {
	skipIfNoDb(t)

	DB.Migrator().DropTable(&User{})
	t.Cleanup(func() {
		DB.AutoMigrate(&User{})
	})

	count := LoadUsersStats()
	if count != 0 {
		t.Fatalf("LoadUsersStats() = %d, want 0 on error", count)
	}
}

func TestLoadUserActivityStats(t *testing.T) {
	skipIfNoDb(t)

	now := time.Now()

	// Create users with different last_activity times
	users := []User{
		{UserId: now.UnixNano(), UserName: "daily_user", Name: "Daily", LastActivity: now.Add(-2 * time.Hour)},                 // DAU, WAU, MAU
		{UserId: now.UnixNano() + 1, UserName: "weekly_user", Name: "Weekly", LastActivity: now.Add(-3 * 24 * time.Hour)},      // WAU, MAU
		{UserId: now.UnixNano() + 2, UserName: "monthly_user", Name: "Monthly", LastActivity: now.Add(-15 * 24 * time.Hour)},   // MAU
		{UserId: now.UnixNano() + 3, UserName: "inactive_user", Name: "Inactive", LastActivity: now.Add(-45 * 24 * time.Hour)}, // none
	}

	// Capture baseline before inserting test users.
	baseDau, baseWau, baseMau := LoadUserActivityStats()

	for i := range users {
		if err := DB.Create(&users[i]).Error; err != nil {
			t.Fatalf("failed to create user: %v", err)
		}
	}

	// Cleanup
	t.Cleanup(func() {
		for _, u := range users {
			res := DB.Where("user_id = ?", u.UserId).Delete(&User{})
			if res.Error != nil {
				t.Errorf("failed to delete test user %d: %v", u.UserId, res.Error)
			}
		}
	})

	dau, wau, mau := LoadUserActivityStats()

	if dau-baseDau != 1 {
		t.Errorf("DAU delta = %d, want 1", dau-baseDau)
	}
	if wau-baseWau != 2 {
		t.Errorf("WAU delta = %d, want 2", wau-baseWau)
	}
	if mau-baseMau != 3 {
		t.Errorf("MAU delta = %d, want 3", mau-baseMau)
	}
}

// ---------------------------------------------------------------------------
// ConcurrentUserCreation
// ---------------------------------------------------------------------------

func TestConcurrentUserCreation(t *testing.T) {
	skipIfNoDb(t)

	userID := time.Now().UnixNano()
	t.Cleanup(func() { DB.Where("user_id = ?", userID).Delete(&User{}) })

	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		go func(i int) {
			defer wg.Done()
			_ = EnsureUserInDb(userID, fmt.Sprintf("concurrent_%d", i), "ConcFirst")
		}(i)
	}
	wg.Wait()

	// Exactly one record should exist
	var count int64
	DB.Model(&User{}).Where("user_id = ?", userID).Count(&count)
	if count != 1 {
		t.Errorf("after concurrent creation, expected 1 user record, got %d", count)
	}
}
