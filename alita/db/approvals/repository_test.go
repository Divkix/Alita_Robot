//go:build testtools

package approvals

import (
	"testing"

	"github.com/divkix/Alita_Robot/alita/db"
)

func skipIfNoDb(t *testing.T) {
	if db.DB == nil {
		t.Skip("DB not initialized")
	}
}

func TestIsUserApproved(t *testing.T) {
	skipIfNoDb(t)

	chatID := int64(-999999999999999)

	t.Cleanup(func() {
		_ = RemoveAllApprovedUsers(chatID)
	})

	// User should not be approved initially
	if IsUserApproved(chatID, 12345) {
		t.Fatalf("IsUserApproved() = true, expected false for non-existent user")
	}

	// Approve user and check
	if err := AddApprovedUser(chatID, 12345, 99999, "trusted member"); err != nil {
		t.Fatalf("AddApprovedUser() error = %v", err)
	}
	if !IsUserApproved(chatID, 12345) {
		t.Fatalf("IsUserApproved() = false, expected true after approval")
	}

	// Different user should not be approved
	if IsUserApproved(chatID, 99999) {
		t.Fatalf("IsUserApproved() = true for unapproved user")
	}

	// Different chat should not have user approved
	if IsUserApproved(-888888888888, 12345) {
		t.Fatalf("IsUserApproved() = true in wrong chat")
	}
}

func TestAddApprovedUser(t *testing.T) {
	skipIfNoDb(t)

	chatID := int64(-999999999999999)

	t.Cleanup(func() {
		_ = RemoveAllApprovedUsers(chatID)
	})

	if err := AddApprovedUser(chatID, 11111, 99999, "test reason"); err != nil {
		t.Fatalf("AddApprovedUser() error = %v", err)
	}

	users := GetApprovedUsers(chatID)
	if len(users) != 1 {
		t.Fatalf("GetApprovedUsers() returned %d users, expected 1", len(users))
	}
	if users[0].UserID != 11111 {
		t.Fatalf("expected UserID=11111, got %d", users[0].UserID)
	}
	if users[0].Reason != "test reason" {
		t.Fatalf("expected Reason='test reason', got %q", users[0].Reason)
	}
	if users[0].ApprovedBy != 99999 {
		t.Fatalf("expected ApprovedBy=99999, got %d", users[0].ApprovedBy)
	}
}

func TestRemoveApprovedUser(t *testing.T) {
	skipIfNoDb(t)

	chatID := int64(-999999999999999)

	t.Cleanup(func() {
		_ = RemoveAllApprovedUsers(chatID)
	})

	// Add two users, remove one
	if err := AddApprovedUser(chatID, 100, 1, ""); err != nil {
		t.Fatalf("AddApprovedUser() error = %v", err)
	}
	if err := AddApprovedUser(chatID, 200, 1, ""); err != nil {
		t.Fatalf("AddApprovedUser() error = %v", err)
	}

	if err := RemoveApprovedUser(chatID, 100); err != nil {
		t.Fatalf("RemoveApprovedUser() error = %v", err)
	}

	if IsUserApproved(chatID, 100) {
		t.Fatalf("IsUserApproved() = true after removal")
	}
	if !IsUserApproved(chatID, 200) {
		t.Fatalf("IsUserApproved() = false for remaining user")
	}
}

func TestGetApprovedUsers(t *testing.T) {
	skipIfNoDb(t)

	chatID := int64(-999999999999999)

	t.Cleanup(func() {
		_ = RemoveAllApprovedUsers(chatID)
	})

	// Empty chat returns empty slice, not nil
	users := GetApprovedUsers(chatID)
	if users == nil {
		t.Fatalf("GetApprovedUsers() returned nil, expected empty slice")
	}
	if len(users) != 0 {
		t.Fatalf("expected 0 approved users for new chat, got %d", len(users))
	}

	// Add users and verify
	if err := AddApprovedUser(chatID, 10, 1, "reason1"); err != nil {
		t.Fatalf("AddApprovedUser() error = %v", err)
	}
	if err := AddApprovedUser(chatID, 20, 1, "reason2"); err != nil {
		t.Fatalf("AddApprovedUser() error = %v", err)
	}

	users = GetApprovedUsers(chatID)
	if len(users) != 2 {
		t.Fatalf("expected 2 approved users, got %d", len(users))
	}
}

func TestRemoveAllApprovedUsers(t *testing.T) {
	skipIfNoDb(t)

	chatID := int64(-999999999999999)

	t.Cleanup(func() {
		_ = RemoveAllApprovedUsers(chatID)
	})

	for i := range 3 {
		if err := AddApprovedUser(chatID, int64(100+i), 1, ""); err != nil {
			t.Fatalf("AddApprovedUser() error = %v", err)
		}
	}

	if err := RemoveAllApprovedUsers(chatID); err != nil {
		t.Fatalf("RemoveAllApprovedUsers() error = %v", err)
	}

	users := GetApprovedUsers(chatID)
	if len(users) != 0 {
		t.Fatalf("expected 0 users after RemoveAllApprovedUsers, got %d", len(users))
	}
}

func TestCacheInvalidationOnWrite(t *testing.T) {
	skipIfNoDb(t)

	chatID := int64(-999999999999998)

	t.Cleanup(func() {
		_ = RemoveAllApprovedUsers(chatID)
	})

	// Add initial user
	if err := AddApprovedUser(chatID, 5555, 1, ""); err != nil {
		t.Fatalf("AddApprovedUser() error = %v", err)
	}

	// Populate cache
	users1 := GetApprovedUsers(chatID)
	if len(users1) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users1))
	}

	// Add another user and verify cache invalidated
	if err := AddApprovedUser(chatID, 6666, 1, ""); err != nil {
		t.Fatalf("AddApprovedUser() error = %v", err)
	}

	users2 := GetApprovedUsers(chatID)
	if len(users2) != 2 {
		t.Fatalf("cache not invalidated: expected 2 users after add, got %d", len(users2))
	}

	// Remove user and verify cache invalidated
	if err := RemoveApprovedUser(chatID, 5555); err != nil {
		t.Fatalf("RemoveApprovedUser() error = %v", err)
	}

	users3 := GetApprovedUsers(chatID)
	if len(users3) != 1 {
		t.Fatalf("cache not invalidated: expected 1 user after remove, got %d", len(users3))
	}

	// RemoveAll and verify cache invalidated
	if err := RemoveAllApprovedUsers(chatID); err != nil {
		t.Fatalf("RemoveAllApprovedUsers() error = %v", err)
	}

	users4 := GetApprovedUsers(chatID)
	if len(users4) != 0 {
		t.Fatalf("cache not invalidated: expected 0 users after clear, got %d", len(users4))
	}
}

func TestDuplicateApprovalIsError(t *testing.T) {
	skipIfNoDb(t)

	chatID := int64(-999999999999997)

	t.Cleanup(func() {
		_ = RemoveAllApprovedUsers(chatID)
	})

	if err := AddApprovedUser(chatID, 7777, 1, ""); err != nil {
		t.Fatalf("AddApprovedUser() error = %v", err)
	}

	// Duplicate should error (GORM duplicate check on unique index)
	err := AddApprovedUser(chatID, 7777, 1, "other reason")
	if err == nil {
		t.Fatalf("AddApprovedUser() duplicate = nil, expected error")
	}
}
