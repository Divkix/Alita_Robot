package disabling

import (
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

func skipIfNoDb(t *testing.T) {
	if db.DB == nil {
		t.Skip("DB not initialized")
	}
}

func TestDisableCommand(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	cmd := "start"

	t.Cleanup(func() {
		db.DB.Where("chat_id = ? AND command = ?", chatID, cmd).Delete(&models.DisableSettings{})
	})

	if err := DisableCMD(chatID, cmd); err != nil {
		t.Fatalf("DisableCMD() error = %v", err)
	}

	cmds := GetChatDisabledCMDs(chatID)
	if !slices.Contains(cmds, cmd) {
		t.Fatalf("expected command %q in disabled list, got %v", cmd, cmds)
	}
}

func TestEnableCommand(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano() + 1000
	cmd := "help"

	t.Cleanup(func() {
		db.DB.Where("chat_id = ? AND command = ?", chatID, cmd).Delete(&models.DisableSettings{})
	})

	if err := DisableCMD(chatID, cmd); err != nil {
		t.Fatalf("DisableCMD() error = %v", err)
	}

	if err := EnableCMD(chatID, cmd); err != nil {
		t.Fatalf("EnableCMD() error = %v", err)
	}

	cmds := GetChatDisabledCMDs(chatID)
	if slices.Contains(cmds, cmd) {
		t.Fatalf("command %q should have been enabled, but still found in disabled list", cmd)
	}
}

func TestIsCommandDisabled(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano() + 2000
	cmd := "ban"

	t.Cleanup(func() {
		db.DB.Where("chat_id = ? AND command = ?", chatID, cmd).Delete(&models.DisableSettings{})
	})

	if IsCommandDisabled(chatID, cmd) {
		t.Fatal("expected IsCommandDisabled=false before disabling")
	}

	if err := DisableCMD(chatID, cmd); err != nil {
		t.Fatalf("DisableCMD() error = %v", err)
	}

	if !IsCommandDisabled(chatID, cmd) {
		t.Fatal("expected IsCommandDisabled=true after disabling")
	}
}

func TestGetDisabledCommands(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano() + 3000
	cmds := []string{"cmd1", "cmd2", "cmd3"}

	t.Cleanup(func() {
		for _, c := range cmds {
			db.DB.Where("chat_id = ? AND command = ?", chatID, c).Delete(&models.DisableSettings{})
		}
	})

	for _, c := range cmds {
		if err := DisableCMD(chatID, c); err != nil {
			t.Fatalf("DisableCMD(%q) error = %v", c, err)
		}
	}

	got := GetChatDisabledCMDs(chatID)
	if len(got) != len(cmds) {
		t.Fatalf("expected %d disabled commands, got %d: %v", len(cmds), len(got), got)
	}

	gotSet := make(map[string]bool, len(got))
	for _, g := range got {
		gotSet[g] = true
	}
	for _, c := range cmds {
		if !gotSet[c] {
			t.Fatalf("command %q not found in disabled list %v", c, got)
		}
	}
}

func TestToggleDeleteEnabled_ZeroValueBoolean(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano() + 4000

	t.Cleanup(func() {
		db.DB.Where("chat_id = ?", chatID).Delete(&models.DisableChatSettings{})
	})

	// Create the record first
	if err := ToggleDel(chatID, true); err != nil {
		t.Fatalf("ToggleDel(true) error = %v", err)
	}
	if !ShouldDel(chatID) {
		t.Fatal("expected ShouldDel=true after ToggleDel(true)")
	}

	// Toggle back to false -- zero-value round-trip
	if err := ToggleDel(chatID, false); err != nil {
		t.Fatalf("ToggleDel(false) error = %v", err)
	}
	if ShouldDel(chatID) {
		t.Fatal("expected ShouldDel=false after ToggleDel(false)")
	}
}

func TestDisableNonExistentCommand(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano() + 5000
	cmd := "nonexistent_cmd_xyz"

	t.Cleanup(func() {
		db.DB.Where("chat_id = ? AND command = ?", chatID, cmd).Delete(&models.DisableSettings{})
	})

	// Disabling a command that was never enabled should still succeed
	if err := DisableCMD(chatID, cmd); err != nil {
		t.Fatalf("DisableCMD() on nonexistent command error = %v", err)
	}

	if !IsCommandDisabled(chatID, cmd) {
		t.Fatal("expected IsCommandDisabled=true after DisableCMD")
	}
}

func TestLoadDisableStats(t *testing.T) {
	skipIfNoDb(t)

	disabledCmds, disableEnabledChats := LoadDisableStats()
	if disabledCmds < 0 {
		t.Fatalf("LoadDisableStats() disabledCmds=%d, want >= 0", disabledCmds)
	}
	if disableEnabledChats < 0 {
		t.Fatalf("LoadDisableStats() disableEnabledChats=%d, want >= 0", disableEnabledChats)
	}
}

func TestLoadDisableStatsErrorBranch(t *testing.T) {
	skipIfNoDb(t)

	_ = db.DB.Migrator().DropTable(&models.DisableSettings{})
	t.Cleanup(func() {
		_ = db.DB.AutoMigrate(&models.DisableSettings{})
	})

	cmds, chats := LoadDisableStats()
	if cmds != 0 || chats != 0 {
		t.Fatalf("LoadDisableStats() = (%d, %d), want (0, 0) on error", cmds, chats)
	}
}

func TestGetDisableSettings_Defaults(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano() + 6000

	t.Cleanup(func() {
		db.DB.Where("chat_id = ?", chatID).Delete(&models.DisableChatSettings{})
	})

	// New chat should not have delete_commands enabled by default
	if ShouldDel(chatID) {
		t.Fatal("expected ShouldDel=false for new chat")
	}
}

func TestConcurrentDisableEnable(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano() + 7000
	const workers = 5

	var wg sync.WaitGroup
	wg.Add(workers)

	for i := range workers {
		chatID := base + int64(i)
		cmd := "testcmd"
		go func(cid int64) {
			defer wg.Done()
			t.Cleanup(func() {
				db.DB.Where("chat_id = ? AND command = ?", cid, cmd).Delete(&models.DisableSettings{})
			})
			if err := DisableCMD(cid, cmd); err != nil {
				t.Errorf("goroutine cid=%d: DisableCMD() error = %v", cid, err)
				return
			}
			if err := EnableCMD(cid, cmd); err != nil {
				t.Errorf("goroutine cid=%d: EnableCMD() error = %v", cid, err)
			}
		}(chatID)
	}

	wg.Wait()
}

func TestDisableSameCommandTwice(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano() + 8000
	cmd := "start"

	t.Cleanup(func() {
		db.DB.Where("chat_id = ? AND command = ?", chatID, cmd).Delete(&models.DisableSettings{})
	})

	// Disable "start" twice -- DisableCMD uses CreateRecord (no deduplication),
	// so two rows may be inserted. The important invariant is that IsCommandDisabled
	// returns true and the command appears at least once in the list.
	if err := DisableCMD(chatID, cmd); err != nil {
		t.Fatalf("DisableCMD() first call error = %v", err)
	}
	if err := DisableCMD(chatID, cmd); err != nil {
		t.Fatalf("DisableCMD() second call error = %v", err)
	}

	// Verify the command is reported as disabled
	if !IsCommandDisabled(chatID, cmd) {
		t.Fatalf("IsCommandDisabled() = false after two DisableCMD calls, want true")
	}

	// Verify it appears at least once in the list
	cmds := GetChatDisabledCMDs(chatID)
	if !slices.Contains(cmds, cmd) {
		t.Fatalf("DisableCMD() twice: command %q not found in list at all; list: %v", cmd, cmds)
	}
}
