package shutdown

import (
	"cmp"
	"context"
	"os"
	"os/signal"
	"slices"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
)

// handlerEntry is one registered drain. handlers stays sorted by phase, then
// by registration sequence descending, so iterating it forward runs phases in
// order and drains registered later first inside their phase.
type handlerEntry struct {
	name   string
	phase  Phase
	seq    int
	handle func() error
}

type Manager struct {
	handlers        []handlerEntry
	mu              sync.RWMutex
	once            sync.Once
	shutdownStarted atomic.Bool
}

var (
	notifySignals = signal.Notify
	stopSignals   = signal.Stop
	// Runs logrus's exit handlers first, which flush telemetry.
	exitProcess     = log.StandardLogger().Exit
	shutdownTimeout = 60 * time.Second
)

// handlerTimeout is how long shutdown waits for one handler before moving on.
const handlerTimeout = 10 * time.Second

func NewManager() *Manager {
	return &Manager{
		handlers: make([]handlerEntry, 0),
	}
}

// Register adds a named drain to phase. Drains run in phase order — see
// Phases — and drains registered later run first inside one phase, so a drain
// that must precede another in the same phase registers after it.
func (m *Manager) Register(name string, phase Phase, handler func() error) {
	if m.shutdownStarted.Load() {
		log.Warnf("[Shutdown] Register(%s) called after shutdown started - handler may not run", name)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	entry := handlerEntry{name: name, phase: phase, seq: len(m.handlers), handle: handler}
	m.handlers = append(m.handlers, entry)
	slices.SortFunc(m.handlers, func(a, b handlerEntry) int {
		return cmp.Or(
			cmp.Compare(a.phase, b.phase),
			cmp.Compare(b.seq, a.seq),
		)
	})
}

func (m *Manager) WaitForShutdown() {
	sigChan := make(chan os.Signal, 1)
	notifySignals(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	sig := <-sigChan
	log.Infof("[Shutdown] Received signal: %v", sig)

	stopSignals(sigChan)
	close(sigChan)

	m.shutdown()
}

func (m *Manager) executeHandler(entry handlerEntry) (err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("[Shutdown] Handler %q panicked: %v", entry.name, r)
		}
	}()
	return entry.handle()
}

func (m *Manager) shutdown() {
	m.once.Do(func() {
		defer error_handling.RecoverFromPanic("shutdown", "shutdown")

		m.shutdownStarted.Store(true)
		log.Info("[Shutdown] Starting graceful shutdown...")

		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		m.mu.RLock()
		handlers := make([]handlerEntry, len(m.handlers))
		copy(handlers, m.handlers)
		m.mu.RUnlock()

		for _, entry := range handlers {
			hCtx, hCancel := context.WithTimeout(ctx, handlerTimeout)
			done := make(chan error, 1)
			go func(e handlerEntry) {
				done <- m.executeHandler(e)
			}(entry)

			select {
			case <-hCtx.Done():
				log.Warnf("[Shutdown] Handler %q timed out, skipping", entry.name)
			case err := <-done:
				if err != nil {
					log.Errorf("[Shutdown] Handler %q error: %v", entry.name, err)
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
