package shutdown

import (
	"errors"
	"sync"
	"testing"

	log "github.com/sirupsen/logrus"
)

func init() {
	log.SetLevel(log.PanicLevel)
}

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	m.mu.RLock()
	count := len(m.handlers)
	m.mu.RUnlock()
	if count != 0 {
		t.Errorf("new manager has %d handlers, want 0", count)
	}
}

func TestRegisterHandler_Single(t *testing.T) {
	m := NewManager()
	m.RegisterHandler(func() error { return nil })
	m.mu.RLock()
	count := len(m.handlers)
	m.mu.RUnlock()
	if count != 1 {
		t.Errorf("handler count = %d, want 1", count)
	}
}

func TestRegisterHandler_Multiple(t *testing.T) {
	m := NewManager()
	for i := range 5 {
		m.RegisterHandler(func() error { _ = i; return nil })
	}
	m.mu.RLock()
	count := len(m.handlers)
	m.mu.RUnlock()
	if count != 5 {
		t.Errorf("handler count = %d, want 5", count)
	}
}

func TestRegisterHandler_ExecutionOrder(t *testing.T) {
	// Verify handlers are stored in registration order
	m := NewManager()
	order := []int{}
	for i := range 3 {
		m.RegisterHandler(func() error {
			order = append(order, i)
			return nil
		})
	}
	// Call each handler in registration order to confirm storage
	m.mu.RLock()
	for _, h := range m.handlers {
		_ = h()
	}
	m.mu.RUnlock()
	for j, v := range order {
		if v != j {
			t.Errorf("handler[%d] = %d, want %d", j, v, j)
		}
	}
}

func TestExecuteHandler_Success(t *testing.T) {
	m := NewManager()
	called := false
	err := m.executeHandler(func() error {
		called = true
		return nil
	}, 0)
	if !called {
		t.Error("handler was not called")
	}
	if err != nil {
		t.Errorf("executeHandler returned error: %v", err)
	}
}

func TestExecuteHandler_ReturnsError(t *testing.T) {
	m := NewManager()
	expected := errors.New("handler failed")
	err := m.executeHandler(func() error {
		return expected
	}, 0)
	if !errors.Is(err, expected) {
		t.Errorf("executeHandler error = %v, want %v", err, expected)
	}
}

func TestExecuteHandler_PanicRecovery(t *testing.T) {
	m := NewManager()
	// Should not propagate the panic
	err := m.executeHandler(func() error {
		panic("test panic")
	}, 1)
	// After panic recovery, err is nil (panic is caught, handler never returned normally)
	if err != nil {
		t.Errorf("after panic recovery, err = %v, want nil", err)
	}
}

func TestExecuteHandler_PanicWithError(t *testing.T) {
	m := NewManager()
	err := m.executeHandler(func() error {
		panic(errors.New("panic error"))
	}, 2)
	if err != nil {
		t.Errorf("after panic recovery, err = %v, want nil", err)
	}
}

func TestLIFOOrder(t *testing.T) {
	// Register handlers [1, 2, 3]; executing in reverse (LIFO) must yield [3, 2, 1].
	m := NewManager()

	var mu sync.Mutex
	var order []int

	for i := 1; i <= 3; i++ {
		i := i
		m.RegisterHandler(func() error {
			mu.Lock()
			order = append(order, i)
			mu.Unlock()
			return nil
		})
	}

	// Execute in LIFO order without calling shutdown() (which calls os.Exit).
	m.mu.RLock()
	handlers := make([]func() error, len(m.handlers))
	copy(handlers, m.handlers)
	m.mu.RUnlock()

	for i := len(handlers) - 1; i >= 0; i-- {
		if err := m.executeHandler(handlers[i], i); err != nil {
			t.Errorf("handler %d returned error: %v", i, err)
		}
	}

	expected := []int{3, 2, 1}
	if len(order) != len(expected) {
		t.Fatalf("LIFO: executed %d handlers, want %d", len(order), len(expected))
	}
	for i, v := range order {
		if v != expected[i] {
			t.Errorf("LIFO order[%d] = %d, want %d", i, v, expected[i])
		}
	}
}

func TestRegisterHandler_Concurrent(t *testing.T) {
	m := NewManager()
	done := make(chan struct{})
	for range 10 {
		go func() {
			m.RegisterHandler(func() error { return nil })
			done <- struct{}{}
		}()
	}
	for range 10 {
		<-done
	}
	m.mu.RLock()
	count := len(m.handlers)
	m.mu.RUnlock()
	if count != 10 {
		t.Errorf("concurrent register: count = %d, want 10", count)
	}
}
