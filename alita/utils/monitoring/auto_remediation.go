package monitoring

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	log "github.com/sirupsen/logrus"
)

func logResourceUsage(level log.Level, msg string) {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	log.WithFields(log.Fields{
		"goroutines": runtime.NumGoroutine(),
		"memory_mb":  float64(ms.Alloc) / 1024 / 1024,
	}).Log(level, msg)
}

type remediationAction struct {
	name       string
	severity   int
	canExecute func(metrics SystemMetrics) bool
	execute    func()
}

type AutoRemediationManager struct {
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	stopOnce        sync.Once
	actions         []remediationAction
	enabled         bool
	lastActionTime  map[string]time.Time
	actionCooldown  time.Duration
	monitorInterval time.Duration
	mu              sync.RWMutex
	collector       *BackgroundStatsCollector
}

func NewAutoRemediationManager(collector *BackgroundStatsCollector) *AutoRemediationManager {
	ctx, cancel := context.WithCancel(context.Background()) // #nosec G118 -- cancel stored in struct field, called in Stop()

	manager := &AutoRemediationManager{
		ctx:             ctx,
		cancel:          cancel,
		enabled:         config.AppConfig.EnablePerformanceMonitoring,
		lastActionTime:  make(map[string]time.Time),
		actionCooldown:  5 * time.Minute,
		monitorInterval: 1 * time.Minute,
		collector:       collector,
	}

	manager.actions = []remediationAction{
		{
			name:     "log_warning",
			severity: 0,
			canExecute: func(metrics SystemMetrics) bool {
				goroutineThreshold := int(float64(config.AppConfig.ResourceMaxGoroutines) * 0.8)
				memoryThreshold := float64(config.AppConfig.ResourceMaxMemoryMB) * 0.5
				return metrics.GoroutineCount > goroutineThreshold || metrics.MemoryAllocMB > memoryThreshold
			},
			execute: func() {
				logResourceUsage(log.WarnLevel, "[AutoRemediation] High resource usage detected")
			},
		},
		{
			name:     "garbage_collection",
			severity: 1,
			canExecute: func(metrics SystemMetrics) bool {
				gcThreshold := float64(config.AppConfig.ResourceMaxMemoryMB) * 0.6
				return metrics.MemoryAllocMB > gcThreshold || metrics.GCPauseMs > 50
			},
			execute: func() {
				log.Info("[AutoRemediation] Triggering garbage collection")
				runtime.GC()
			},
		},
		{
			name:     "memory_cleanup",
			severity: 2,
			canExecute: func(metrics SystemMetrics) bool {
				return metrics.MemoryAllocMB > float64(config.AppConfig.ResourceGCThresholdMB)
			},
			execute: func() {
				log.Info("[AutoRemediation] Performing memory cleanup operations")

				for range 3 {
					runtime.GC()
					time.Sleep(100 * time.Millisecond)
				}

				runtime.GC()
			},
		},
		{
			name:     "restart_recommendation",
			severity: 10,
			canExecute: func(metrics SystemMetrics) bool {
				goroutineThreshold := int(float64(config.AppConfig.ResourceMaxGoroutines) * 1.5)
				memoryThreshold := float64(config.AppConfig.ResourceMaxMemoryMB) * 1.6
				return metrics.GoroutineCount > goroutineThreshold || metrics.MemoryAllocMB > memoryThreshold
			},
			execute: func() {
				logResourceUsage(log.ErrorLevel, "[AutoRemediation] CRITICAL: Resource usage is dangerously high. Manual restart recommended.")
			},
		},
	}

	return manager
}

func (m *AutoRemediationManager) Start() {
	if !m.enabled {
		log.Info("[AutoRemediation] Auto-remediation is disabled")
		return
	}

	log.Info("[AutoRemediation] Starting auto-remediation monitoring")
	m.wg.Add(1)
	go m.monitorAndRemediate()
}

func (m *AutoRemediationManager) monitorAndRemediate() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.monitorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			func() {
				defer error_handling.RecoverFromPanic("checkAndRemediate", "AutoRemediation")
				m.checkAndRemediate()
			}()
		case <-m.ctx.Done():
			return
		}
	}
}

func (m *AutoRemediationManager) checkAndRemediate() {
	if m.collector == nil {
		return
	}

	metrics := m.collector.GetCurrentMetrics()

	if metrics.GCPauseMs > 100 {
		log.WithField("gc_pause_ms", metrics.GCPauseMs).Warn("[AutoRemediation] Long GC pause detected")
	}

	for _, action := range m.actions {
		if !action.canExecute(metrics) || !m.shouldExecuteAction(action.name) {
			continue
		}

		log.WithFields(log.Fields{
			"action":      action.name,
			"goroutines":  metrics.GoroutineCount,
			"memory_mb":   metrics.MemoryAllocMB,
			"gc_pause_ms": metrics.GCPauseMs,
		}).Info("[AutoRemediation] Executing remediation action")

		action.execute()
		m.markActionExecuted(action.name)

		log.WithField("action", action.name).Info("[AutoRemediation] Successfully executed remediation action")

		break
	}
}

func (m *AutoRemediationManager) shouldExecuteAction(name string) bool {
	m.mu.RLock()
	lastExecution, exists := m.lastActionTime[name]
	m.mu.RUnlock()

	if !exists {
		return true
	}

	return time.Since(lastExecution) >= m.actionCooldown
}

func (m *AutoRemediationManager) markActionExecuted(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastActionTime[name] = time.Now()
}

func (m *AutoRemediationManager) Stop() {
	m.stopOnce.Do(func() {
		log.Info("[AutoRemediation] Stopping auto-remediation monitoring")
		m.cancel()
		m.wg.Wait()
		log.Info("[AutoRemediation] Auto-remediation monitoring stopped")
	})
}
