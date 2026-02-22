package db

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// EnsureUserInDb
// ---------------------------------------------------------------------------

func TestEnsureUserInDb(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	skipIfNoDb(t)

	count := LoadUsersStats()
	if count < 0 {
		t.Errorf("LoadUsersStats() = %d, want >= 0", count)
	}
}

// ---------------------------------------------------------------------------
// ConcurrentUserCreation
// ---------------------------------------------------------------------------

func TestConcurrentUserCreation(t *testing.T) {
	t.Parallel()
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
