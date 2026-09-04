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

func NewManager() *Manager {
	return &Manager{
		handlers: make([]func() error, 0),
	}
}

func (m *Manager) RegisterHandler(handler func() error) {
	if m.shutdownStarted.Load() {
		log.Warn("[Shutdown] RegisterHandler called after shutdown started - handler may not run")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers = append(m.handlers, handler)
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

func (m *Manager) executeHandler(handler func() error, index int) (err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("[Shutdown] Handler %d panicked: %v", index, r)
		}
	}()
	return handler()
}

func (m *Manager) shutdown() {
	m.once.Do(func() {
		defer error_handling.RecoverFromPanic("shutdown", "shutdown")

		m.shutdownStarted.Store(true)
		log.Info("[Shutdown] Starting graceful shutdown...")

		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		m.mu.Lock()
		handlers := make([]func() error, len(m.handlers))
		copy(handlers, m.handlers)
		m.mu.Unlock()

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
