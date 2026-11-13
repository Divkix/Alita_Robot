//go:build testtools

package shutdown

import (
	"os"
	"syscall"
	"time"
)

// HandlerTimeoutForTest returns how long shutdown waits for one handler, so a
// test outside this package can keep a step such as the polling stop below it.
func HandlerTimeoutForTest() time.Duration {
	return handlerTimeout
}

// ExecuteHandlersForTest runs the registered handlers in the same LIFO order
// and with the same panic recovery as shutdown, without exiting the process.
// It exists so tests outside this package can assert the relative order of
// the drains main.go registers.
func (m *Manager) ExecuteHandlersForTest() {
	m.mu.RLock()
	handlers := make([]func() error, len(m.handlers))
	copy(handlers, m.handlers)
	m.mu.RUnlock()

	for i := len(handlers) - 1; i >= 0; i-- {
		_ = m.executeHandler(handlers[i], i)
	}
}

// SetExitForTest replaces the process exit that ends a shutdown with exit and
// returns a function restoring the real one. A test outside this package uses
// it to watch a full shutdown, including what WaitForShutdown's caller does
// after the exit, without ending the test binary.
func SetExitForTest(exit func(code int)) (restore func()) {
	previous := exitProcess
	exitProcess = exit
	return func() { exitProcess = previous }
}

// StubSignalsForTest keeps WaitForShutdown off the OS signals, which a
// testing/synctest bubble cannot share, and returns start, which delivers
// SIGTERM to it as the OS would once it is waiting, and restore.
func StubSignalsForTest() (start func(), restore func()) {
	previousNotify, previousStop := notifySignals, stopSignals
	subscribed := make(chan chan<- os.Signal, 1)
	notifySignals = func(c chan<- os.Signal, _ ...os.Signal) { subscribed <- c }
	stopSignals = func(chan<- os.Signal) {}
	return func() { (<-subscribed) <- syscall.SIGTERM },
		func() { notifySignals, stopSignals = previousNotify, previousStop }
}
