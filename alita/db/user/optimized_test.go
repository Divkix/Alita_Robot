//go:build testtools

package user

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"gorm.io/gorm"
)

func TestGetUserBasicInfo_NilDB(t *testing.T) {
	originalDB := db.DB
	db.DB = nil
	t.Cleanup(func() {
		db.DB = originalDB
	})

	user, err := GetUserBasicInfo(123)
	if err == nil {
		t.Fatal("GetUserBasicInfo() with nil db expected error, got nil")
	}
	if err.Error() != "database not initialized" {
		t.Fatalf("GetUserBasicInfo() error = %q, want %q", err.Error(), "database not initialized")
	}
	if user != nil {
		t.Fatal("GetUserBasicInfo() with nil db expected nil user, got non-nil")
	}
}

func TestGetUserBasicInfo(t *testing.T) {
	skipIfNoDb(t)

	userID := time.Now().UnixNano()
	username := fmt.Sprintf("testuser_%d", userID)
	name := "Test User"

	t.Cleanup(func() {
		_ = db.DB.Where("user_id = ?", userID).Delete(&models.User{}).Error
	})

	// Insert a user
	if err := db.EnsureUserInDb(userID, username, name); err != nil {
		t.Fatalf("EnsureUserInDb() error = %v", err)
	}

	// Get user by ID
	user, err := GetUserBasicInfo(userID)
	if err != nil {
		t.Fatalf("GetUserBasicInfo() error = %v", err)
	}
	if user.UserId != userID {
		t.Fatalf("GetUserBasicInfo() UserId = %d, want %d", user.UserId, userID)
	}

	// Non-existent user -> ErrRecordNotFound
	nonExistentID := userID + 999999
	_, err = GetUserBasicInfo(nonExistentID)
	if err == nil {
		t.Fatal("GetUserBasicInfo() nonexistent expected error, got nil")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("GetUserBasicInfo() nonexistent error = %v, want gorm.ErrRecordNotFound", err)
	}
}

func TestGetUserBasicInfoCached(t *testing.T) {
	skipIfNoDb(t)

	userID := time.Now().UnixNano()
	username := fmt.Sprintf("testuser_%d", userID)
	name := "Test User"

	t.Cleanup(func() {
		_ = db.DB.Where("user_id = ?", userID).Delete(&models.User{}).Error
	})

	// Insert a user
	if err := db.EnsureUserInDb(userID, username, name); err != nil {
		t.Fatalf("EnsureUserInDb() error = %v", err)
	}

	// First call should load from DB
	user, err := GetUserBasicInfoCached(userID)
	if err != nil {
		t.Fatalf("GetUserBasicInfoCached() error = %v", err)
	}
	if user.UserId != userID {
		t.Fatalf("GetUserBasicInfoCached() UserId = %d, want %d", user.UserId, userID)
	}

	// Second call should use cache
	user2, err := GetUserBasicInfoCached(userID)
	if err != nil {
		t.Fatalf("GetUserBasicInfoCached() cached error = %v", err)
	}
	if user2.UserId != userID {
		t.Fatalf("GetUserBasicInfoCached() cached UserId = %d, want %d", user2.UserId, userID)
	}
}

func TestGetUserBasicInfoCached_RecordNotFound(t *testing.T) {
	skipIfNoDb(t)

	userID := time.Now().UnixNano()

	// Non-existent user -> ErrRecordNotFound
	_, err := GetUserBasicInfoCached(userID)
	if err == nil {
		t.Fatal("GetUserBasicInfoCached() nonexistent expected error, got nil")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("GetUserBasicInfoCached() nonexistent error = %v, want gorm.ErrRecordNotFound", err)
	}
}
