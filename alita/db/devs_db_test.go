package db

import (
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// AddDev / RemDev
// ---------------------------------------------------------------------------

func TestAddDev(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	userID := time.Now().UnixNano()
	t.Cleanup(func() { DB.Where("user_id = ?", userID).Delete(&DevSettings{}) })

	if err := AddDev(userID); err != nil {
		t.Fatalf("AddDev() error = %v", err)
	}

	devrc := GetTeamMemInfo(userID)
	if !devrc.IsDev {
		t.Errorf("GetTeamMemInfo(%d).IsDev = false, want true after AddDev", userID)
	}
	if !devrc.Dev {
		t.Errorf("GetTeamMemInfo(%d).Dev = false, want true after AddDev", userID)
	}
}

func TestRemoveDev(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	userID := time.Now().UnixNano()
	t.Cleanup(func() { DB.Where("user_id = ?", userID).Delete(&DevSettings{}) })

	if err := AddDev(userID); err != nil {
		t.Fatalf("AddDev() error = %v", err)
	}

	if err := RemDev(userID); err != nil {
		t.Fatalf("RemDev() error = %v", err)
	}

	devrc := GetTeamMemInfo(userID)
	if devrc.IsDev {
		t.Errorf("GetTeamMemInfo(%d).IsDev = true, want false after RemDev", userID)
	}
	if devrc.Dev {
		t.Errorf("GetTeamMemInfo(%d).Dev = true, want false after RemDev", userID)
	}
}

// ---------------------------------------------------------------------------
// AddSudo / RemSudo
// ---------------------------------------------------------------------------

func TestAddSudo(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	userID := time.Now().UnixNano()
	t.Cleanup(func() { DB.Where("user_id = ?", userID).Delete(&DevSettings{}) })

	if err := AddSudo(userID); err != nil {
		t.Fatalf("AddSudo() error = %v", err)
	}

	devrc := GetTeamMemInfo(userID)
	if !devrc.Sudo {
		t.Errorf("GetTeamMemInfo(%d).Sudo = false, want true after AddSudo", userID)
	}
}

func TestRemoveSudo(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	userID := time.Now().UnixNano()
	t.Cleanup(func() { DB.Where("user_id = ?", userID).Delete(&DevSettings{}) })

	if err := AddSudo(userID); err != nil {
		t.Fatalf("AddSudo() error = %v", err)
	}

	if err := RemSudo(userID); err != nil {
		t.Fatalf("RemSudo() error = %v", err)
	}

	devrc := GetTeamMemInfo(userID)
	if devrc.Sudo {
		t.Errorf("GetTeamMemInfo(%d).Sudo = true, want false after RemSudo", userID)
	}
}

// ---------------------------------------------------------------------------
// GetTeamMemInfo (GetDevSettings equivalent)
// ---------------------------------------------------------------------------

func TestGetDevSettings(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	// Non-existent user should return defaults (not a dev)
	const nonExistentID = int64(9876543210987)
	devrc := GetTeamMemInfo(nonExistentID)
	if devrc == nil {
		t.Fatal("GetTeamMemInfo() returned nil for non-existent user")
	}
	if devrc.IsDev {
		t.Errorf("GetTeamMemInfo(%d).IsDev = true for non-existent user, want false", nonExistentID)
	}
	if devrc.Dev {
		t.Errorf("GetTeamMemInfo(%d).Dev = true for non-existent user, want false", nonExistentID)
	}
	if devrc.Sudo {
		t.Errorf("GetTeamMemInfo(%d).Sudo = true for non-existent user, want false", nonExistentID)
	}
}

// ---------------------------------------------------------------------------
// DevDualBooleanFields
// ---------------------------------------------------------------------------

func TestDevDualBooleanFields(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	userID := time.Now().UnixNano()
	t.Cleanup(func() { DB.Where("user_id = ?", userID).Delete(&DevSettings{}) })

	if err := AddDev(userID); err != nil {
		t.Fatalf("AddDev() error = %v", err)
	}

	devrc := GetTeamMemInfo(userID)
	// Both Dev and IsDev must be set consistently
	if devrc.IsDev != devrc.Dev {
		t.Errorf("IsDev (%v) and Dev (%v) are inconsistent after AddDev", devrc.IsDev, devrc.Dev)
	}

	if err := RemDev(userID); err != nil {
		t.Fatalf("RemDev() error = %v", err)
	}

	devrc = GetTeamMemInfo(userID)
	// Both must be false after removal
	if devrc.IsDev != devrc.Dev {
		t.Errorf("IsDev (%v) and Dev (%v) are inconsistent after RemDev", devrc.IsDev, devrc.Dev)
	}
	if devrc.IsDev {
		t.Errorf("IsDev must be false after RemDev, got true")
	}
}
