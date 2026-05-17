package db

import (
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// AddDev / RemDev
// ---------------------------------------------------------------------------

func TestAddDev(t *testing.T) {
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
}

func TestRemoveDev(t *testing.T) {
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
}

// ---------------------------------------------------------------------------
// AddSudo / RemSudo
// ---------------------------------------------------------------------------

func TestAddSudo(t *testing.T) {
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
	if devrc.Sudo {
		t.Errorf("GetTeamMemInfo(%d).Sudo = true for non-existent user, want false", nonExistentID)
	}
}

// ---------------------------------------------------------------------------
// GetTeamMembers
// ---------------------------------------------------------------------------

func TestGetTeamMembers(t *testing.T) {
	skipIfNoDb(t)

	devOnly := time.Now().UnixNano()
	sudoOnly := devOnly + 1
	bothDevAndSudo := devOnly + 2

	t.Cleanup(func() {
		for _, id := range []int64{devOnly, sudoOnly, bothDevAndSudo} {
			DB.Where("user_id = ?", id).Delete(&DevSettings{})
		}
	})

	if err := AddDev(devOnly); err != nil {
		t.Fatalf("AddDev(%d) error = %v", devOnly, err)
	}
	if err := AddSudo(sudoOnly); err != nil {
		t.Fatalf("AddSudo(%d) error = %v", sudoOnly, err)
	}
	if err := AddSudo(bothDevAndSudo); err != nil {
		t.Fatalf("AddSudo(%d) error = %v", bothDevAndSudo, err)
	}
	if err := AddDev(bothDevAndSudo); err != nil {
		t.Fatalf("AddDev(%d) error = %v", bothDevAndSudo, err)
	}

	members := GetTeamMembers()
	if members == nil {
		t.Fatal("GetTeamMembers() returned nil, want non-nil map")
	}

	if got, want := members[devOnly], "dev"; got != want {
		t.Errorf("GetTeamMembers()[%d] = %q, want %q", devOnly, got, want)
	}
	if got, want := members[sudoOnly], "sudo"; got != want {
		t.Errorf("GetTeamMembers()[%d] = %q, want %q", sudoOnly, got, want)
	}
	if got, want := members[bothDevAndSudo], "dev"; got != want {
		t.Errorf("GetTeamMembers()[%d] = %q, want %q (dev takes precedence)", bothDevAndSudo, got, want)
	}
}

func TestGetTeamMembersEmpty(t *testing.T) {
	skipIfNoDb(t)

	// Ensure no leftover dev/sudo rows from other tests by deleting all DevSettings
	DB.Where("1 = 1").Delete(&DevSettings{})

	members := GetTeamMembers()
	if members == nil {
		t.Fatal("GetTeamMembers() returned nil, want empty map")
	}
	if len(members) != 0 {
		t.Errorf("len(GetTeamMembers()) = %d, want 0", len(members))
	}
}

// ---------------------------------------------------------------------------
// LoadAllStats
// ---------------------------------------------------------------------------

func TestLoadAllStats(t *testing.T) {
	skipIfNoDb(t)

	stats := LoadAllStats()
	if stats == "" {
		t.Fatal("LoadAllStats() returned empty string, want non-empty HTML stats")
	}

	// Verify expected sections are present
	expectedSections := []string{
		"Alita's Stats",
		"Deployment Mode",
		"Go Version",
		"Goroutines",
		"Antiflood",
		"Users",
		"Group Activity Metrics",
		"Daily Active Groups",
		"Weekly Active Groups",
		"Monthly Active Groups",
		"User Activity Metrics",
		"Daily Active Users",
		"Weekly Active Users",
		"Monthly Active Users",
		"Pins",
		"CleanLinked Enabled",
		"AntiChannelPin Enabled",
		"Reports",
		"Rules",
		"Set",
		"Private",
		"Blacklists",
		"Connections",
		"Disabling",
		"Filters",
		"Greetings",
		"Welcome Enabled",
		"Goodbye Enabled",
		"CleanService",
		"CleanWelcome",
		"CleanGoodbye",
		"Notes",
		"Channels Stored",
	}

	for _, section := range expectedSections {
		if !strings.Contains(stats, section) {
			t.Errorf("LoadAllStats() missing expected section %q", section)
		}
	}
}


