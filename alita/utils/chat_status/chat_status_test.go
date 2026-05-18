package chat_status

import (
	"testing"

	"github.com/divkix/Alita_Robot/alita/db"
)

func skipIfNoDb(t *testing.T) {
	t.Helper()
	if db.DB == nil {
		t.Skip("requires database connection")
	}
}

func TestIsApproved(t *testing.T) {
	skipIfNoDb(t)

	chatID := int64(-999999999900000)

	t.Cleanup(func() {
		_ = db.RemoveAllApprovedUsers(chatID)
	})

	// Mock bot pointer - IsApproved doesn't actually use it, just needs non-nil
	// We pass nil intentionally since IsApproved delegates to db.IsUserApproved which ignores bot
	got := IsApproved(nil, chatID, 1001)
	if got != false {
		t.Fatalf("IsApproved(nil, chat, unapproved) = %v, want false", got)
	}

	// Approve user and verify
	if err := db.AddApprovedUser(chatID, 1001, 1, "test"); err != nil {
		t.Fatalf("AddApprovedUser() error = %v", err)
	}
	got = IsApproved(nil, chatID, 1001)
	if got != true {
		t.Fatalf("IsApproved(nil, chat, approved) = %v, want true", got)
	}
}
