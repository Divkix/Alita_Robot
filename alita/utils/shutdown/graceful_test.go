package shutdown

import (
	"errors"
	"sync"
	"testing"
)

func TestNewManager(t *testing.T) {
	t.Parallel()

	m := NewManager()

	if m == nil {
		t.Fatal("NewManager() returned nil")
	}

	m.mu.RLock()
	handlers := m.handlers
	m.mu.RUnlock()

	if handlers == nil {
		t.Fatal("handlers slice should be non-nil after NewManager()")
	}

	if len(handlers) != 0 {
		t.Fatalf("handlers slice should be empty after NewManager(), got length %d", len(handlers))
	}
}

func TestRegisterHandler(t *testing.T) {
	t.Parallel()

	t.Run("single handler", func(t *testing.T) {
		t.Parallel()

		m := NewManager()
		m.RegisterHandler(func() error { return nil })

		m.mu.RLock()
		length := len(m.handlers)
		m.mu.RUnlock()

		if length != 1 {
			t.Fatalf("expected 1 handler, got %d", length)
		}
	})

	t.Run("multiple sequential handlers", func(t *testing.T) {
		t.Parallel()

		const n = 5
		m := NewManager()

		for i := 0; i < n; i++ {
			m.RegisterHandler(func() error { return nil })
		}

		m.mu.RLock()
		length := len(m.handlers)
		m.mu.RUnlock()

		if length != n {
			t.Fatalf("expected %d handlers, got %d", n, length)
		}
	})

	t.Run("concurrent 50 goroutines", func(t *testing.T) {
		t.Parallel()

		const count = 50
		m := NewManager()

		var wg sync.WaitGroup
		wg.Add(count)

		for i := 0; i < count; i++ {
			go func() {
				defer wg.Done()
				m.RegisterHandler(func() error { return nil })
			}()
		}

		wg.Wait()

		m.mu.RLock()
		length := len(m.handlers)
		m.mu.RUnlock()

		if length != count {
			t.Fatalf("expected %d handlers after concurrent registration, got %d", count, length)
		}
	})
}

func TestExecuteHandler(t *testing.T) {
	t.Parallel()

	t.Run("handler returns nil", func(t *testing.T) {
		t.Parallel()

		m := NewManager()
		err := m.executeHandler(func() error { return nil }, 0)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})

	t.Run("handler returns error", func(t *testing.T) {
		t.Parallel()

		m := NewManager()
		sentinel := errors.New("shutdown error")
		err := m.executeHandler(func() error { return sentinel }, 1)
		if !errors.Is(err, sentinel) {
			t.Fatalf("expected sentinel error, got %v", err)
		}
	})

	t.Run("handler panics — recovered, no propagation", func(t *testing.T) {
		t.Parallel()

		m := NewManager()

		// executeHandler must recover from the panic internally; if it does not,
		// the deferred recover below would catch it and we would explicitly fail.
		var panicCaught bool
		func() {
			defer func() {
				if r := recover(); r != nil {
					panicCaught = true
				}
			}()
			_ = m.executeHandler(func() error {
				panic("test panic")
			}, 2)
		}()

		if panicCaught {
			t.Fatal("panic escaped executeHandler — recovery is not working")
		}
	})
}
