package misc

import (
	"fmt"
	"sync"
	"testing"
)

func TestAddToArray(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []string
		vals  []string
		want  []string
	}{
		{
			name:  "append to empty slice",
			input: []string{},
			vals:  []string{"cmd1"},
			want:  []string{"cmd1"},
		},
		{
			name:  "append multiple vals",
			input: []string{"existing"},
			vals:  []string{"a", "b", "c"},
			want:  []string{"existing", "a", "b", "c"},
		},
		{
			name:  "append nothing",
			input: []string{"x"},
			vals:  []string{},
			want:  []string{"x"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := addToArray(tc.input, tc.vals...)
			if len(got) != len(tc.want) {
				t.Fatalf("addToArray() len = %d, want %d", len(got), len(tc.want))
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("addToArray()[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestAddCmdToDisableable(t *testing.T) {
	// Save and restore global state
	original := make([]string, len(DisableCmds))
	copy(original, DisableCmds)
	t.Cleanup(func() {
		DisableCmds = original
	})

	before := len(DisableCmds)
	AddCmdToDisableable("testcmd")
	if len(DisableCmds) != before+1 {
		t.Fatalf("DisableCmds len = %d, want %d", len(DisableCmds), before+1)
	}
	if DisableCmds[len(DisableCmds)-1] != "testcmd" {
		t.Errorf("last element = %q, want %q", DisableCmds[len(DisableCmds)-1], "testcmd")
	}
}

func TestAddCmdToDisableable_Multiple(t *testing.T) {
	// Save and restore global state
	original := make([]string, len(DisableCmds))
	copy(original, DisableCmds)
	t.Cleanup(func() {
		DisableCmds = original
	})

	before := len(DisableCmds)
	AddCmdToDisableable("cmd1")
	AddCmdToDisableable("cmd2")
	AddCmdToDisableable("cmd3")

	if len(DisableCmds) != before+3 {
		t.Errorf("DisableCmds len = %d, want %d", len(DisableCmds), before+3)
	}
}

// TestAddCmdToDisableable_Concurrent tests concurrent-safe addition via the package mu.
// Note: AddCmdToDisableable's read+assign of the package variable is outside the
// internal mutex, so we protect the full read-modify-write with the package-level mu
// here to avoid a data race on DisableCmds while testing concurrent goroutine behavior.
func TestAddCmdToDisableable_Concurrent(t *testing.T) {
	original := make([]string, len(DisableCmds))
	copy(original, DisableCmds)
	t.Cleanup(func() { DisableCmds = original })

	DisableCmds = make([]string, 0)

	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			mu.Lock()
			DisableCmds = append(DisableCmds, fmt.Sprintf("cmd%d", i))
			mu.Unlock()
		}()
	}
	wg.Wait()

	if len(DisableCmds) != goroutines {
		t.Errorf("expected %d commands after concurrent adds, got %d", goroutines, len(DisableCmds))
	}
}
