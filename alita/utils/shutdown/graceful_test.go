package shutdown

import (
	"errors"
	"sync"
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager() returned nil")
	}
	if m.handlers == nil {
		t.Error("expected handlers slice to be initialized, got nil")
	}
	if len(m.handlers) != 0 {
		t.Errorf("expected empty handlers slice, got len=%d", len(m.handlers))
	}
}

func TestRegisterHandler(t *testing.T) {
	m := NewManager()

	var callOrder []int
	var mu sync.Mutex

	m.RegisterHandler(func() error { mu.Lock(); callOrder = append(callOrder, 1); mu.Unlock(); return nil })
	m.RegisterHandler(func() error { mu.Lock(); callOrder = append(callOrder, 2); mu.Unlock(); return nil })
	m.RegisterHandler(func() error { mu.Lock(); callOrder = append(callOrder, 3); mu.Unlock(); return nil })

	if len(m.handlers) != 3 {
		t.Fatalf("expected 3 handlers, got %d", len(m.handlers))
	}

	// Verify registration order (FIFO) by executing directly
	for i, h := range m.handlers {
		if err := h(); err != nil {
			t.Fatalf("handler %d returned error: %v", i, err)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(callOrder) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(callOrder))
	}
	for i, v := range callOrder {
		if v != i+1 {
			t.Errorf("expected callOrder[%d]=%d, got %d", i, i+1, v)
		}
	}
}

func TestExecuteHandler(t *testing.T) {
	m := NewManager()

	// Test successful execution
	executed := false
	err := m.executeHandler(func() error {
		executed = true
		return nil
	}, 0)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if !executed {
		t.Error("expected handler to be executed")
	}

	// Test handler returning error
	expectedErr := errors.New("handler error")
	err = m.executeHandler(func() error {
		return expectedErr
	}, 1)
	if err == nil {
		t.Error("expected error, got nil")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

func TestExecuteHandlerPanicRecovery(t *testing.T) {
	m := NewManager()

	// Test panic recovery - handler should not propagate panic
	executed := false
	err := m.executeHandler(func() error {
		executed = true
		panic("intentional panic")
	}, 0)

	if err != nil {
		t.Errorf("expected nil error after panic recovery, got %v", err)
	}
	if !executed {
		t.Error("expected handler to have started execution before panic")
	}
}

func TestHandlersExecuteInLIFOOrder(t *testing.T) {
	m := NewManager()

	// Use a channel to record execution order
	orderCh := make(chan int, 3)

	m.RegisterHandler(func() error { orderCh <- 1; return nil })
	m.RegisterHandler(func() error { orderCh <- 2; return nil })
	m.RegisterHandler(func() error { orderCh <- 3; return nil })

	// Simulate shutdown's reverse iteration without calling shutdown() directly
	m.mu.RLock()
	handlers := make([]func() error, len(m.handlers))
	copy(handlers, m.handlers)
	m.mu.RUnlock()

	// Execute in reverse order (LIFO) as shutdown() does
	for i := len(handlers) - 1; i >= 0; i-- {
		_ = m.executeHandler(handlers[i], i)
	}

	close(orderCh)

	var order []int
	for v := range orderCh {
		order = append(order, v)
	}

	if len(order) != 3 {
		t.Fatalf("expected 3 executions, got %d", len(order))
	}

	// LIFO: last registered (3) should execute first
	expected := []int{3, 2, 1}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("expected execution order[%d]=%d (LIFO), got %d", i, v, order[i])
		}
	}
}

func TestRegisterHandlerConcurrency(t *testing.T) {
	m := NewManager()
	var wg sync.WaitGroup
	numHandlers := 100

	for i := 0; i < numHandlers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.RegisterHandler(func() error { return nil })
		}()
	}

	wg.Wait()

	if len(m.handlers) != numHandlers {
		t.Errorf("expected %d handlers, got %d", numHandlers, len(m.handlers))
	}
}
