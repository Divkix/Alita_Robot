//go:build testtools

package helpers

import "testing"

// SetDisableCmdsForTest replaces DisableCmds for the duration of a test.
func SetDisableCmdsForTest(t *testing.T, cmds []string) {
	t.Helper()

	cmdsMu.Lock()
	original := append([]string(nil), DisableCmds...)
	DisableCmds = append([]string(nil), cmds...)
	cmdsMu.Unlock()

	t.Cleanup(func() {
		cmdsMu.Lock()
		DisableCmds = original
		cmdsMu.Unlock()
	})
}
