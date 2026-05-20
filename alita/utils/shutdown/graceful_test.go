package shutdown

import (
	"errors"
	"os"
	"sync"
	"testing"
	"time"
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
	tests := []struct {
		name         string
		handler      func() error
		delay        int
		wantExecuted bool
		wantErr      bool
		wantErrIs    error
	}{
		{
			name: "successful execution",
			handler: func() error {
				return nil
			},
			delay:        0,
			wantExecuted: true,
			wantErr:      false,
		},
		{
			name: "handler returns error",
			handler: func() error {
				return errors.New("handler error")
			},
			delay:        1,
			wantExecuted: true,
			wantErr:      true,
			wantErrIs:    errors.New("handler error"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := NewManager()
			executed := false
			wrapped := func() error {
				executed = true
				return tc.handler()
			}
			err := m.executeHandler(wrapped, tc.delay)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tc.wantErrIs != nil && err.Error() != tc.wantErrIs.Error() {
					t.Errorf("expected error %v, got %v", tc.wantErrIs, err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
			if executed != tc.wantExecuted {
				t.Errorf("executed: got %v, want %v", executed, tc.wantExecuted)
			}
		})
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
		if err := m.executeHandler(handlers[i], i); err != nil {
			t.Fatalf("handler %d returned error: %v", i, err)
		}
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

func TestShutdownExecutesHandlersInLIFOOrderAndExitsOnce(t *testing.T) {
	oldExit := exitProcess
	oldTimeout := shutdownTimeout
	defer func() {
		exitProcess = oldExit
		shutdownTimeout = oldTimeout
	}()

	var exitCodes []int
	exitProcess = func(code int) {
		exitCodes = append(exitCodes, code)
	}
	shutdownTimeout = time.Second

	m := NewManager()
	var order []int
	m.RegisterHandler(func() error {
		order = append(order, 1)
		return nil
	})
	m.RegisterHandler(func() error {
		order = append(order, 2)
		return errors.New("second handler failed")
	})
	m.RegisterHandler(func() error {
		order = append(order, 3)
		return nil
	})

	m.shutdown()
	m.shutdown()

	expectedOrder := []int{3, 2, 1}
	if len(order) != len(expectedOrder) {
		t.Fatalf("expected order %v, got %v", expectedOrder, order)
	}
	for i, want := range expectedOrder {
		if order[i] != want {
			t.Fatalf("expected order %v, got %v", expectedOrder, order)
		}
	}
	if len(exitCodes) != 1 || exitCodes[0] != 0 {
		t.Fatalf("expected one successful exit, got %v", exitCodes)
	}
}

func TestWaitForShutdownUsesSignalHooks(t *testing.T) {
	oldExit := exitProcess
	oldNotify := notifySignals
	oldStop := stopSignals
	oldTimeout := shutdownTimeout
	defer func() {
		exitProcess = oldExit
		notifySignals = oldNotify
		stopSignals = oldStop
		shutdownTimeout = oldTimeout
	}()

	var notifiedSignals []os.Signal
	var stopCalled bool
	var exitCodes []int
	notifySignals = func(ch chan<- os.Signal, sig ...os.Signal) {
		notifiedSignals = append(notifiedSignals, sig...)
		ch <- os.Interrupt
	}
	stopSignals = func(_ chan<- os.Signal) {
		stopCalled = true
	}
	exitProcess = func(code int) {
		exitCodes = append(exitCodes, code)
	}
	shutdownTimeout = time.Second

	m := NewManager()
	var handlerCalled bool
	m.RegisterHandler(func() error {
		handlerCalled = true
		return nil
	})

	m.WaitForShutdown()

	if len(notifiedSignals) != 3 {
		t.Fatalf("expected three registered signals, got %d", len(notifiedSignals))
	}
	if !stopCalled {
		t.Fatal("expected signal stop hook to be called")
	}
	if !handlerCalled {
		t.Fatal("expected shutdown handler to run")
	}
	if len(exitCodes) != 1 || exitCodes[0] != 0 {
		t.Fatalf("expected one successful exit, got %v", exitCodes)
	}
}
