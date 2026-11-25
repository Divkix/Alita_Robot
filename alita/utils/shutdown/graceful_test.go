package shutdown

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"testing/synctest"
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

func TestRegister(t *testing.T) {
	m := NewManager()

	var callOrder []int
	var mu sync.Mutex

	m.Register("first", PhaseModuleDrains, func(context.Context) error { mu.Lock(); callOrder = append(callOrder, 1); mu.Unlock(); return nil })
	m.Register("second", PhaseModuleDrains, func(context.Context) error { mu.Lock(); callOrder = append(callOrder, 2); mu.Unlock(); return nil })
	m.Register("third", PhaseModuleDrains, func(context.Context) error { mu.Lock(); callOrder = append(callOrder, 3); mu.Unlock(); return nil })

	if len(m.handlers) != 3 {
		t.Fatalf("expected 3 handlers, got %d", len(m.handlers))
	}

	for i, entry := range m.handlers {
		if err := entry.handle(context.Background()); err != nil {
			t.Fatalf("handler %d returned error: %v", i, err)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(callOrder) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(callOrder))
	}
	// handlers is stored in execution order: within one phase the drain
	// registered last runs first.
	for i, v := range callOrder {
		if v != 3-i {
			t.Errorf("expected callOrder[%d]=%d (LIFO), got %d", i, 3-i, v)
		}
	}
}

func TestExecuteHandler(t *testing.T) {
	tests := []struct {
		name         string
		handler      func(context.Context) error
		delay        int
		wantExecuted bool
		wantErr      bool
		wantErrIs    error
	}{
		{
			name: "successful execution",
			handler: func(context.Context) error {
				return nil
			},
			delay:        0,
			wantExecuted: true,
			wantErr:      false,
		},
		{
			name: "handler returns error",
			handler: func(context.Context) error {
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
			wrapped := func(ctx context.Context) error {
				executed = true
				return tc.handler(ctx)
			}
			entry := handlerEntry{name: "test", phase: PhaseModuleDrains, handle: wrapped}
			err := m.executeHandler(context.Background(), entry)
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

	executed := false
	entry := handlerEntry{name: "panicking", phase: PhaseModuleDrains, handle: func(context.Context) error {
		executed = true
		panic("intentional panic")
	}}
	err := m.executeHandler(context.Background(), entry)

	if err != nil {
		t.Errorf("expected nil error after panic recovery, got %v", err)
	}
	if !executed {
		t.Error("expected handler to have started execution before panic")
	}
}

// Within one phase drains run LIFO: the drain registered last runs first.
func TestHandlersInOnePhaseExecuteInLIFOOrder(t *testing.T) {
	m := NewManager()

	orderCh := make(chan int, 3)

	m.Register("first", PhaseModuleDrains, func(context.Context) error { orderCh <- 1; return nil })
	m.Register("second", PhaseModuleDrains, func(context.Context) error { orderCh <- 2; return nil })
	m.Register("third", PhaseModuleDrains, func(context.Context) error { orderCh <- 3; return nil })

	m.mu.RLock()
	handlers := make([]handlerEntry, len(m.handlers))
	copy(handlers, m.handlers)
	m.mu.RUnlock()

	for _, entry := range handlers {
		if err := m.executeHandler(context.Background(), entry); err != nil {
			t.Fatalf("handler %q returned error: %v", entry.name, err)
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

	expected := []int{3, 2, 1}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("expected execution order[%d]=%d (LIFO), got %d", i, v, order[i])
		}
	}
}

// Registration order does not decide execution order: a drain registered
// first still runs in its phase, and a later phase never runs before an
// earlier one.
func TestPhasesDecideExecutionOrderNotRegistrationOrder(t *testing.T) {
	m := NewManager()

	var order []string
	var mu sync.Mutex
	record := func(name string) func(context.Context) error {
		return func(context.Context) error {
			mu.Lock()
			defer mu.Unlock()
			order = append(order, name)
			return nil
		}
	}

	// Register in reverse phase order on purpose.
	m.Register("tracer", PhaseTelemetry, record("tracer"))
	m.Register("db close", PhaseDatabase, record("db close"))
	m.Register("stats flush", PhaseFlush, record("stats flush"))
	m.Register("jobs stop", PhaseJobs, record("jobs stop"))
	m.Register("users drain", PhaseModuleDrains, record("users drain"))
	m.Register("http stop", PhaseHTTPServer, record("http stop"))
	m.Register("updater stop", PhaseUpdateSource, record("updater stop"))

	m.mu.RLock()
	handlers := make([]handlerEntry, len(m.handlers))
	copy(handlers, m.handlers)
	m.mu.RUnlock()
	for _, entry := range handlers {
		if err := m.executeHandler(context.Background(), entry); err != nil {
			t.Fatalf("handler %q returned error: %v", entry.name, err)
		}
	}

	want := []string{
		"updater stop", "http stop", "jobs stop", "users drain",
		"stats flush", "db close", "tracer",
	}
	mu.Lock()
	defer mu.Unlock()
	if len(order) != len(want) {
		t.Fatalf("executed %v, want %v", order, want)
	}
	for i, w := range want {
		if order[i] != w {
			t.Fatalf("executed %v, want %v", order, want)
		}
	}
}

// Phases is the total ordered list of phases; a phase registered outside it
// would sort unpredictably, so every phase must be one of them.
func TestPhasesListIsTotalAndContiguous(t *testing.T) {
	phases := Phases()
	if len(phases) != int(PhaseTelemetry)+1 {
		t.Fatalf("Phases() = %v, want %d contiguous phases ending at PhaseTelemetry", phases, int(PhaseTelemetry)+1)
	}
	for i, phase := range phases {
		if int(phase) != i {
			t.Fatalf("Phases()[%d] = %v, want phase %d", i, phase, i)
		}
	}
}

func TestRegisterConcurrency(t *testing.T) {
	m := NewManager()
	var wg sync.WaitGroup
	numHandlers := 100

	for range numHandlers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.Register("handler", PhaseModuleDrains, func(context.Context) error { return nil })
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
	var mu sync.Mutex
	record := func(v int) func(context.Context) error {
		return func(context.Context) error {
			mu.Lock()
			defer mu.Unlock()
			order = append(order, v)
			return nil
		}
	}
	m.Register("first", PhaseModuleDrains, record(1))
	m.Register("second", PhaseModuleDrains, func(context.Context) error {
		mu.Lock()
		defer mu.Unlock()
		order = append(order, 2)
		return errors.New("second handler failed")
	})
	m.Register("third", PhaseModuleDrains, record(3))

	m.shutdown()
	m.shutdown()

	expectedOrder := []int{3, 2, 1}
	mu.Lock()
	defer mu.Unlock()
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

func TestShutdownTimeoutInterruptsBlockedHandler(t *testing.T) {
	oldExit := exitProcess
	oldTimeout := shutdownTimeout
	defer func() {
		exitProcess = oldExit
		shutdownTimeout = oldTimeout
	}()

	block := make(chan struct{})
	t.Cleanup(func() { close(block) })
	exitCode := make(chan int, 1)
	exitProcess = func(code int) {
		exitCode <- code
	}
	shutdownTimeout = 10 * time.Millisecond

	m := NewManager()
	m.Register("blocked", PhaseModuleDrains, func(context.Context) error {
		<-block
		return nil
	})

	done := make(chan struct{})
	go func() {
		m.shutdown()
		close(done)
	}()

	select {
	case code := <-exitCode:
		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown did not enforce its timeout")
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not return after timed out exit")
	}
}

// A handler blocked on its context sees the cancellation at the handler
// limit, so the work stops instead of running under the next phase.
func TestShutdownCancelsHandlerAtTheHandlerTimeout(t *testing.T) {
	oldExit := exitProcess
	defer func() { exitProcess = oldExit }()

	synctest.Test(t, func(t *testing.T) {
		var exitCode int
		exitProcess = func(code int) { exitCode = code }

		cancelled := make(chan struct{})
		m := NewManager()
		m.Register("waits for cancel", PhaseModuleDrains, func(ctx context.Context) error {
			<-ctx.Done()
			close(cancelled)
			return ctx.Err()
		})

		start := time.Now()
		m.shutdown()

		if elapsed := time.Since(start); elapsed != handlerTimeout {
			t.Fatalf("shutdown took %v, want the %v handler limit", elapsed, handlerTimeout)
		}
		select {
		case <-cancelled:
		default:
			t.Fatal("handler did not observe its context cancellation")
		}
		if exitCode != 0 {
			t.Fatalf("exit code = %d, want 0", exitCode)
		}
	})
}

// An abandoned handler that ignores cancellation still delays the later
// phases: the database closes only once it returns, so it cannot write into
// a closed database.
func TestPhaseBarrierWaitsForAnAbandonedHandlerBeforeClosingTheDatabase(t *testing.T) {
	oldExit := exitProcess
	defer func() { exitProcess = oldExit }()

	synctest.Test(t, func(t *testing.T) {
		var exitCode int
		exitProcess = func(code int) { exitCode = code }

		release := make(chan struct{})
		var mu sync.Mutex
		var order []string
		record := func(step string) {
			mu.Lock()
			defer mu.Unlock()
			order = append(order, step)
		}

		cancelled := make(chan struct{})
		m := NewManager()
		m.Register("db close", PhaseDatabase, func(context.Context) error { record("db close"); return nil })
		m.Register("stats flush", PhaseFlush, func(context.Context) error { record("stats flush"); return nil })
		m.Register("ignores cancel", PhaseModuleDrains, func(ctx context.Context) error {
			<-ctx.Done()
			close(cancelled)
			<-release
			record("ignores cancel")
			return nil
		})

		done := make(chan struct{})
		go func() {
			m.shutdown()
			close(done)
		}()

		// At the handler limit the drain is abandoned but still writing.
		time.Sleep(handlerTimeout)
		<-cancelled

		mu.Lock()
		if len(order) != 0 {
			mu.Unlock()
			t.Fatalf("order before the abandoned handler returned = %v, want none", order)
		}
		mu.Unlock()

		close(release)
		<-done

		if exitCode != 0 {
			t.Fatalf("exit code = %d, want 0", exitCode)
		}
		mu.Lock()
		defer mu.Unlock()
		want := []string{"ignores cancel", "stats flush", "db close"}
		if len(order) != len(want) {
			t.Fatalf("order = %v, want %v", order, want)
		}
		for i, step := range want {
			if order[i] != step {
				t.Fatalf("order = %v, want %v", order, want)
			}
		}
	})
}

// The global deadline ends the process without running the database phase:
// closing the database is safe only after every earlier handler returned,
// and the deadline says one did not.
func TestGlobalTimeoutExitsWithoutRunningTheDatabasePhase(t *testing.T) {
	oldExit := exitProcess
	oldTimeout := shutdownTimeout
	defer func() {
		exitProcess = oldExit
		shutdownTimeout = oldTimeout
	}()

	synctest.Test(t, func(t *testing.T) {
		release := make(chan struct{})
		defer close(release)
		var exitCode int
		exitProcess = func(code int) { exitCode = code }
		shutdownTimeout = 5 * time.Second

		dbClosed := false
		m := NewManager()
		m.Register("db close", PhaseDatabase, func(context.Context) error { dbClosed = true; return nil })
		m.Register("tracer", PhaseTelemetry, func(context.Context) error { dbClosed = true; return nil })
		m.Register("hung drain", PhaseModuleDrains, func(context.Context) error {
			<-release
			return nil
		})

		start := time.Now()
		m.shutdown()

		if elapsed := time.Since(start); elapsed != shutdownTimeout {
			t.Fatalf("shutdown took %v, want the %v global limit", elapsed, shutdownTimeout)
		}
		if exitCode != 1 {
			t.Fatalf("exit code = %d, want 1", exitCode)
		}
		if dbClosed {
			t.Fatal("database phase or a later phase ran after the global timeout")
		}
	})
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
	handlerCalled := false
	m.Register("handler", PhaseModuleDrains, func(context.Context) error {
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
