package helpers

import (
	"fmt"
	"testing"
)

func TestAddCmdToDisableable(t *testing.T) {
	const testCmd = "test_disableable_cmd_42"

	cmdsMu.Lock()
	orig := make([]string, len(DisableCmds))
	copy(orig, DisableCmds)
	cmdsMu.Unlock()

	defer func() {
		cmdsMu.Lock()
		DisableCmds = orig
		cmdsMu.Unlock()
	}()

	AddCmdToDisableable(testCmd)

	cmdsMu.Lock()
	found := false
	for _, c := range DisableCmds {
		if c == testCmd {
			found = true
			break
		}
	}
	cmdsMu.Unlock()

	if !found {
		t.Fatalf("expected %q in DisableCmds", testCmd)
	}
}

func TestAddCmdToDisableableThreadSafe(t *testing.T) {
	cmdsMu.Lock()
	orig := make([]string, len(DisableCmds))
	copy(orig, DisableCmds)
	cmdsMu.Unlock()

	defer func() {
		cmdsMu.Lock()
		DisableCmds = orig
		cmdsMu.Unlock()
	}()

	const n = 100
	done := make(chan bool, n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			AddCmdToDisableable(fmt.Sprintf("concurrent_cmd_%d", idx))
			done <- true
		}(i)
	}
	for i := 0; i < n; i++ {
		<-done
	}

	cmdsMu.Lock()
	count := len(DisableCmds)
	cmdsMu.Unlock()

	if count != len(orig)+n {
		t.Fatalf("expected %d commands in DisableCmds, got %d", len(orig)+n, count)
	}
}
