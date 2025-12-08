package shutdown

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	log "github.com/sirupsen/logrus"
)

// Manager handles graceful shutdown of the application
type Manager struct {
	handlers []func() error
	mu       sync.RWMutex
	once     sync.Once
}

// NewManager creates a new shutdown manager
func NewManager() *Manager {
	return &Manager{
		handlers: make([]func() error, 0),
	}
}

// RegisterHandler registers a shutdown handler
func (m *Manager) RegisterHandler(handler func() error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers = append(m.handlers, handler)
}

// WaitForShutdown waits for shutdown signals and executes handlers
func (m *Manager) WaitForShutdown() {
	defer error_handling.RecoverFromPanic("WaitForShutdown", "shutdown")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// Wait for signal
	sig := <-sigChan
	log.Infof("[Shutdown] Received signal: %v", sig)

	m.shutdown()
}

// executeHandler safely executes a single shutdown handler with panic recovery.
// Returns the error from the handler, or nil if the handler panicked (panic is logged separately).
func (m *Manager) executeHandler(handler func() error, index int) (err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("[Shutdown] Handler %d panicked: %v", index, r)
			error_handling.RecoverFromPanic("executeHandler", "shutdown")
		}
	}()
	return handler()
}

// shutdown performs graceful shutdown
func (m *Manager) shutdown() {
	m.once.Do(func() {
		defer error_handling.RecoverFromPanic("shutdown", "shutdown")

		log.Info("[Shutdown] Starting graceful shutdown...")

		// Create context with timeout for shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Execute shutdown handlers in reverse order
		m.mu.RLock()
		handlers := make([]func() error, len(m.handlers))
		copy(handlers, m.handlers)
		m.mu.RUnlock()

		// Execute handlers in reverse order (LIFO)
		for i := len(handlers) - 1; i >= 0; i-- {
			select {
			case <-ctx.Done():
				log.Warn("[Shutdown] Timeout reached, forcing exit")
				os.Exit(1)
			default:
				if err := m.executeHandler(handlers[i], i); err != nil {
					log.Errorf("[Shutdown] Handler error: %v", err)
				}
			}
		}

		log.Info("[Shutdown] Graceful shutdown completed")
		os.Exit(0)
	})
}
