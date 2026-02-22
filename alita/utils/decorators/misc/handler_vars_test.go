package misc

import (
	"fmt"
	"sync"
	"testing"
)

func TestAddToArray(t *testing.T) {
	t.Parallel()

	t.Run("nil slice with one value", func(t *testing.T) {
		var arr []string
		result := addToArray(arr, "cmd1")
		if len(result) != 1 {
			t.Errorf("expected length 1, got %d", len(result))
		}
		if result[0] != "cmd1" {
			t.Errorf("expected 'cmd1', got %q", result[0])
		}
	})

	t.Run("existing slice with multiple values", func(t *testing.T) {
		arr := []string{"existing"}
		result := addToArray(arr, "val1", "val2", "val3")
		if len(result) != 4 {
			t.Errorf("expected length 4, got %d", len(result))
		}
		if result[0] != "existing" {
			t.Errorf("expected first element 'existing', got %q", result[0])
		}
		if result[1] != "val1" || result[2] != "val2" || result[3] != "val3" {
			t.Errorf("unexpected elements: %v", result[1:])
		}
	})

	t.Run("empty variadic args", func(t *testing.T) {
		arr := []string{"keep"}
		result := addToArray(arr)
		if len(result) != 1 {
			t.Errorf("expected length 1, got %d", len(result))
		}
		if result[0] != "keep" {
			t.Errorf("expected 'keep', got %q", result[0])
		}
	})

	t.Run("empty string value", func(t *testing.T) {
		arr := []string{"before"}
		result := addToArray(arr, "")
		if len(result) != 2 {
			t.Errorf("expected length 2, got %d", len(result))
		}
		if result[1] != "" {
			t.Errorf("expected empty string at index 1, got %q", result[1])
		}
	})
}

func TestAddCmdToDisableable(t *testing.T) {
	t.Run("single command", func(t *testing.T) {
		t.Cleanup(func() {
			mu.Lock()
			DisableCmds = make([]string, 0)
			mu.Unlock()
		})

		AddCmdToDisableable("ban")

		mu.Lock()
		length := len(DisableCmds)
		val := ""
		if length > 0 {
			val = DisableCmds[0]
		}
		mu.Unlock()

		if length != 1 {
			t.Errorf("expected 1 command, got %d", length)
		}
		if val != "ban" {
			t.Errorf("expected 'ban', got %q", val)
		}
	})

	t.Run("duplicate command appears twice", func(t *testing.T) {
		t.Cleanup(func() {
			mu.Lock()
			DisableCmds = make([]string, 0)
			mu.Unlock()
		})

		AddCmdToDisableable("kick")
		AddCmdToDisableable("kick")

		mu.Lock()
		length := len(DisableCmds)
		mu.Unlock()

		if length != 2 {
			t.Errorf("expected 2 entries for duplicate command, got %d", length)
		}
	})

	t.Run("concurrent 50 goroutines each adding unique command", func(t *testing.T) {
		t.Cleanup(func() {
			mu.Lock()
			DisableCmds = make([]string, 0)
			mu.Unlock()
		})

		const numGoroutines = 50
		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			cmd := fmt.Sprintf("cmd_%d", i)
			go func(c string) {
				defer wg.Done()
				AddCmdToDisableable(c)
			}(cmd)
		}

		wg.Wait()

		mu.Lock()
		length := len(DisableCmds)
		mu.Unlock()

		if length != numGoroutines {
			t.Errorf("expected %d commands after concurrent adds, got %d", numGoroutines, length)
		}
	})
}
