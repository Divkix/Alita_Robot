package devs

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/dustin/go-humanize"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/federations"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

func skipIfNoDb(t *testing.T) {
	if db.DB == nil {
		t.Skip("DB not initialized")
	}
}

// ---------------------------------------------------------------------------
// AddDev / RemDev
// ---------------------------------------------------------------------------

func TestAddDev(t *testing.T) {
	skipIfNoDb(t)

	userID := time.Now().UnixNano()
	t.Cleanup(func() {
		if err := db.DB.Where("user_id = ?", userID).Delete(&models.DevSettings{}).Error; err != nil {
			t.Errorf("cleanup Delete(DevSettings) error: %v", err)
		}
	})

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
	t.Cleanup(func() {
		if err := db.DB.Where("user_id = ?", userID).Delete(&models.DevSettings{}).Error; err != nil {
			t.Errorf("cleanup Delete(DevSettings) error: %v", err)
		}
	})

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
	t.Cleanup(func() {
		if err := db.DB.Where("user_id = ?", userID).Delete(&models.DevSettings{}).Error; err != nil {
			t.Errorf("cleanup Delete(DevSettings) error: %v", err)
		}
	})

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
	t.Cleanup(func() {
		if err := db.DB.Where("user_id = ?", userID).Delete(&models.DevSettings{}).Error; err != nil {
			t.Errorf("cleanup Delete(DevSettings) error: %v", err)
		}
	})

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
			if err := db.DB.Where("user_id = ?", id).Delete(&models.DevSettings{}).Error; err != nil {
				t.Fatalf("cleanup Delete(DevSettings) for user %d error: %v", id, err)
			}
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
		t.Errorf("GetTeamMembers()[%d] = %q, want %q", bothDevAndSudo, got, want)
	}
}

func TestGetTeamMembersEmpty(t *testing.T) {
	skipIfNoDb(t)

	// Ensure no leftover dev/sudo rows from other tests by deleting all DevSettings
	if err := db.DB.Where("1 = 1").Delete(&models.DevSettings{}).Error; err != nil {
		t.Fatalf("failed to clean DevSettings: %v", err)
	}

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
		"Federations",
		"Total",
		"Chats",
		"Admins",
		"Bans",
		"Subscriptions",
		"Channels Stored",
	}

	for _, section := range expectedSections {
		if !strings.Contains(stats, section) {
			t.Errorf("LoadAllStats() missing expected section %q", section)
		}
	}
}

func TestLoadAllStats_IncludesFederationCounts(t *testing.T) {
	skipIfNoDb(t)

	ownerA := time.Now().UnixNano()
	ownerB := ownerA + 1
	adminID := ownerA + 2
	bannedID := ownerA + 3
	chatID := -ownerA

	fedA, err := federations.CreateFederation(ownerA, "Stats Fed A")
	if err != nil {
		t.Fatalf("CreateFederation A: %v", err)
	}
	fedB, err := federations.CreateFederation(ownerB, "Stats Fed B")
	if err != nil {
		t.Fatalf("CreateFederation B: %v", err)
	}
	t.Cleanup(func() {
		_ = federations.DeleteFederation(fedA.FedID)
		_ = federations.DeleteFederation(fedB.FedID)
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
		_ = db.DB.Where("user_id IN ?", []int64{ownerA, ownerB, adminID, bannedID}).
			Delete(&models.User{}).Error
	})

	if err := federations.JoinFed(chatID, "stats chat", fedA.FedID); err != nil {
		t.Fatalf("JoinFed: %v", err)
	}
	if err := federations.PromoteFedAdmin(fedA.FedID, adminID); err != nil {
		t.Fatalf("PromoteFedAdmin: %v", err)
	}
	if _, _, err := federations.Fban(fedA.FedID, bannedID, ownerA, "stats"); err != nil {
		t.Fatalf("Fban: %v", err)
	}
	if err := federations.SubscribeFed(fedA.FedID, fedB.FedID); err != nil {
		t.Fatalf("SubscribeFed: %v", err)
	}

	feds, fedChats, admins, bans, subs := federations.LoadFederationStats()
	if feds < 2 || fedChats < 1 || admins < 1 || bans < 1 || subs < 1 {
		t.Fatalf("LoadFederationStats() = (%d, %d, %d, %d, %d), want at least (2, 1, 1, 1, 1)",
			feds, fedChats, admins, bans, subs)
	}

	stats := LoadAllStats()
	idx := strings.Index(stats, "<b>Federations:</b>")
	if idx < 0 {
		t.Fatal("LoadAllStats() missing Federations section")
	}
	section := stats[idx:]
	if end := strings.Index(section, "<b>Channels Stored"); end >= 0 {
		section = section[:end]
	}

	want := []string{
		fmt.Sprintf("<b>Total:</b> %s", humanize.Comma(feds)),
		fmt.Sprintf("<b>Chats:</b> %s", humanize.Comma(fedChats)),
		fmt.Sprintf("<b>Admins:</b> %s", humanize.Comma(admins)),
		fmt.Sprintf("<b>Bans:</b> %s", humanize.Comma(bans)),
		fmt.Sprintf("<b>Subscriptions:</b> %s", humanize.Comma(subs)),
	}
	for _, line := range want {
		if !strings.Contains(section, line) {
			t.Errorf("Federations section missing %q\nsection=%q", line, section)
		}
	}
}
