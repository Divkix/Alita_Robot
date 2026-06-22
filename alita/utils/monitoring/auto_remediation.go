package monitoring

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/divkix/Alita_Robot/alita/config"
	log "github.com/sirupsen/logrus"
)

// logResourceUsage logs current goroutine count and memory at the given level.
func logResourceUsage(level log.Level, msg string) {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	log.WithFields(log.Fields{
		"goroutines": runtime.NumGoroutine(),
		"memory_mb":  float64(ms.Alloc) / 1024 / 1024,
	}).Log(level, msg)
}

// remediationAction is a single remediation step. The manager holds these in a
// fixed slice ordered by ascending severity, so the lowest-severity applicable
// action is always chosen first without any runtime sorting.
type remediationAction struct {
	name       string
	severity   int
	canExecute func(metrics SystemMetrics) bool
	execute    func()
}

// AutoRemediationManager handles automatic remediation of performance issues
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

// NewAutoRemediationManager creates a new auto-remediation manager
func NewAutoRemediationManager(collector *BackgroundStatsCollector) *AutoRemediationManager {
	ctx, cancel := context.WithCancel(context.Background()) // #nosec G118 -- cancel stored in struct field, called in Stop()

	manager := &AutoRemediationManager{
		ctx:             ctx,
		cancel:          cancel,
		enabled:         config.AppConfig.EnablePerformanceMonitoring,
		lastActionTime:  make(map[string]time.Time),
		actionCooldown:  5 * time.Minute, // Minimum time between same actions
		monitorInterval: 1 * time.Minute,
		collector:       collector,
	}

	// Built-in remediation actions, ordered by ascending severity so the check
	// cycle picks the least severe applicable action first.
	manager.actions = []remediationAction{
		{
			name:     "log_warning",
			severity: 0,
			canExecute: func(metrics SystemMetrics) bool {
				// Log warning when resources are above 80% goroutines / 50% memory.
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
				// Trigger GC when memory is above 60% of max threshold.
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
				// Trigger cleanup when memory is above the GC threshold.
				return metrics.MemoryAllocMB > float64(config.AppConfig.ResourceGCThresholdMB)
			},
			execute: func() {
				log.Info("[AutoRemediation] Performing memory cleanup operations")

				// Trigger multiple GC cycles for thorough cleanup.
				for range 3 {
					runtime.GC()
					time.Sleep(100 * time.Millisecond)
				}

				// Force release of unused memory back to OS.
				runtime.GC()
			},
		},
		{
			name:     "restart_recommendation",
			severity: 10,
			canExecute: func(metrics SystemMetrics) bool {
				// Recommend restart when resources are above 150% goroutines / 160% memory.
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

// Start begins monitoring for issues requiring remediation
func (m *AutoRemediationManager) Start() {
	if !m.enabled {
		log.Info("[AutoRemediation] Auto-remediation is disabled")
		return
	}

	log.Info("[AutoRemediation] Starting auto-remediation monitoring")
	m.wg.Add(1)
	go m.monitorAndRemediate()
}

// monitorAndRemediate continuously monitors metrics and applies remediation
func (m *AutoRemediationManager) monitorAndRemediate() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.monitorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.checkAndRemediate()
		case <-m.ctx.Done():
			return
		}
	}
}

// checkAndRemediate checks current metrics and applies appropriate remediation.
// Actions are evaluated in ascending-severity order; the first applicable action
// that is past its cooldown runs, and only one action runs per check cycle.
func (m *AutoRemediationManager) checkAndRemediate() {
	if m.collector == nil {
		return
	}

	metrics := m.collector.GetCurrentMetrics()

	for _, action := range m.actions {
		if !action.canExecute(metrics) || !m.shouldExecuteAction(action.name) {
			continue
		}

		log.WithFields(log.Fields{
			"action":               action.name,
			"goroutines":           metrics.GoroutineCount,
			"memory_mb":            metrics.MemoryAllocMB,
			"gc_pause_ms":          metrics.GCPauseMs,
			"avg_response_time_ms": metrics.AverageResponseTime.Milliseconds(),
		}).Info("[AutoRemediation] Executing remediation action")

		action.execute()
		m.markActionExecuted(action.name)

		log.WithField("action", action.name).Info("[AutoRemediation] Successfully executed remediation action")

		// Only execute one action per check cycle.
		break
	}
}

// shouldExecuteAction determines if an action should be executed based on cooldown
func (m *AutoRemediationManager) shouldExecuteAction(name string) bool {
	m.mu.RLock()
	lastExecution, exists := m.lastActionTime[name]
	m.mu.RUnlock()

	if !exists {
		return true
	}

	return time.Since(lastExecution) >= m.actionCooldown
}

// markActionExecuted records when an action was last executed
func (m *AutoRemediationManager) markActionExecuted(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastActionTime[name] = time.Now()
}

// Stop gracefully shuts down the auto-remediation manager
func (m *AutoRemediationManager) Stop() {
	m.stopOnce.Do(func() {
		log.Info("[AutoRemediation] Stopping auto-remediation monitoring")
		m.cancel()
		m.wg.Wait()
		log.Info("[AutoRemediation] Auto-remediation monitoring stopped")
	})
}
