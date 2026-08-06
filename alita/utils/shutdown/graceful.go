package shutdown

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	log "github.com/sirupsen/logrus"
)

// Manager handles graceful shutdown of the application
type Manager struct {
	handlers        []func() error
	mu              sync.RWMutex
	once            sync.Once
	shutdownStarted atomic.Bool
}

var (
	notifySignals   = signal.Notify
	stopSignals     = signal.Stop
	exitProcess     = os.Exit
	shutdownTimeout = 60 * time.Second
)

// NewManager creates a new shutdown manager
func NewManager() *Manager {
	return &Manager{
		handlers: make([]func() error, 0),
	}
}

// RegisterHandler registers a shutdown handler
func (m *Manager) RegisterHandler(handler func() error) {
	if m.shutdownStarted.Load() {
		log.Warn("[Shutdown] RegisterHandler called after shutdown started - handler may not run")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers = append(m.handlers, handler)
}

// WaitForShutdown waits for shutdown signals and executes handlers
func (m *Manager) WaitForShutdown() {
	sigChan := make(chan os.Signal, 1)
	notifySignals(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// Wait for signal
	sig := <-sigChan
	log.Infof("[Shutdown] Received signal: %v", sig)

	// Stop receiving signals - prevents duplicate shutdown calls
	stopSignals(sigChan)
	close(sigChan)

	m.shutdown()
}

// executeHandler safely executes a single shutdown handler with panic recovery.
// Returns the error from the handler, or nil if the handler panicked (panic is logged separately).
func (m *Manager) executeHandler(handler func() error, index int) (err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("[Shutdown] Handler %d panicked: %v", index, r)
		}
	}()
	return handler()
}

// shutdown performs graceful shutdown
func (m *Manager) shutdown() {
	m.once.Do(func() {
		defer error_handling.RecoverFromPanic("shutdown", "shutdown")

		m.shutdownStarted.Store(true)
		log.Info("[Shutdown] Starting graceful shutdown...")

		// Create context with timeout for shutdown
		// Use 60 seconds to allow time for database connections and large cleanup tasks
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		// Execute shutdown handlers in reverse order - need exclusive lock to
		// copy safely while preventing concurrent RegisterHandler races.
		m.mu.Lock()
		handlers := make([]func() error, len(m.handlers))
		copy(handlers, m.handlers)
		m.mu.Unlock()

		// Execute handlers in reverse order (LIFO) with per-handler timeout
		for i := len(handlers) - 1; i >= 0; i-- {
			hCtx, hCancel := context.WithTimeout(ctx, 10*time.Second)
			done := make(chan error, 1)
			go func(h func() error, idx int) {
				done <- m.executeHandler(h, idx)
			}(handlers[i], i)

			select {
			case <-hCtx.Done():
				log.Warnf("[Shutdown] Handler %d timeout, skipping", i)
			case err := <-done:
				if err != nil {
					log.Errorf("[Shutdown] Handler %d error: %v", i, err)
				}
			}
			hCancel()

			if ctx.Err() != nil {
				log.Warn("[Shutdown] Global timeout, forcing exit")
				exitProcess(1)
				return
			}
		}

		log.Info("[Shutdown] Graceful shutdown completed")
		exitProcess(0)
	})
}
