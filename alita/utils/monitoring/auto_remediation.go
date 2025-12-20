package monitoring

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/divkix/Alita_Robot/alita/config"
	log "github.com/sirupsen/logrus"
)

// RemediationAction represents an action that can be taken to remediate issues
type RemediationAction interface {
	Name() string
	Execute(ctx context.Context) error
	CanExecute(metrics SystemMetrics) bool
	Severity() int // Higher number = more severe action
}

// AutoRemediationManager handles automatic remediation of performance issues
type AutoRemediationManager struct {
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	actions        []RemediationAction
	enabled        bool
	lastActionTime map[string]time.Time
	actionCooldown time.Duration
	mu             sync.RWMutex
	collector      *BackgroundStatsCollector
	thresholds     RemediationThresholds
}

// RemediationThresholds defines when remediation actions should be triggered
type RemediationThresholds struct {
	MaxGoroutines      int
	MaxMemoryMB        float64
	MaxGCPauseMs       float64
	MaxResponseTimeMs  int64
	MaxErrorRate       float64
	CriticalMemoryMB   float64
	CriticalGoroutines int
}

// NewAutoRemediationManager creates a new auto-remediation manager
func NewAutoRemediationManager(collector *BackgroundStatsCollector) *AutoRemediationManager {
	ctx, cancel := context.WithCancel(context.Background())

	manager := &AutoRemediationManager{
		ctx:            ctx,
		cancel:         cancel,
		enabled:        config.AppConfig.EnablePerformanceMonitoring,
		lastActionTime: make(map[string]time.Time),
		actionCooldown: 5 * time.Minute, // Minimum time between same actions
		collector:      collector,
		thresholds: RemediationThresholds{
			MaxGoroutines:      config.AppConfig.ResourceMaxGoroutines,
			MaxMemoryMB:        float64(config.AppConfig.ResourceMaxMemoryMB),
			MaxGCPauseMs:       100,
			MaxResponseTimeMs:  5000,
			MaxErrorRate:       0.1,
			CriticalMemoryMB:   float64(config.AppConfig.ResourceMaxMemoryMB * 2),
			CriticalGoroutines: config.AppConfig.ResourceMaxGoroutines * 2,
		},
	}

	// Register built-in remediation actions
	manager.registerBuiltInActions()

	return manager
}

// registerBuiltInActions registers the built-in remediation actions
func (m *AutoRemediationManager) registerBuiltInActions() {
	m.RegisterAction(&GCAction{})
	m.RegisterAction(&MemoryCleanupAction{})
	m.RegisterAction(&LogWarningAction{})
	m.RegisterAction(&RestartRecommendationAction{})
}

// RegisterAction registers a new remediation action
func (m *AutoRemediationManager) RegisterAction(action RemediationAction) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.actions = append(m.actions, action)
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

	ticker := time.NewTicker(1 * time.Minute)
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

// checkAndRemediate checks current metrics and applies appropriate remediation
func (m *AutoRemediationManager) checkAndRemediate() {
	metrics := m.collector.GetCurrentMetrics()

	// Get applicable actions sorted by severity (least severe first)
	applicableActions := m.getApplicableActions(metrics)

	// Execute actions if needed
	for _, action := range applicableActions {
		if m.shouldExecuteAction(action) {
			if err := m.executeAction(action, metrics); err != nil {
				log.WithFields(log.Fields{
					"action": action.Name(),
					"error":  err,
				}).Error("[AutoRemediation] Failed to execute remediation action")
			} else {
				m.markActionExecuted(action)
				log.WithField("action", action.Name()).Info("[AutoRemediation] Successfully executed remediation action")
				// Only execute one action per check cycle
				break
			}
		}
	}
}

// getApplicableActions returns actions that can be executed for current metrics
func (m *AutoRemediationManager) getApplicableActions(metrics SystemMetrics) []RemediationAction {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var applicable []RemediationAction
	for _, action := range m.actions {
		if action.CanExecute(metrics) {
			applicable = append(applicable, action)
		}
	}

	// Sort by severity (ascending - least severe first)
	for i := range len(applicable) - 1 {
		for j := i + 1; j < len(applicable); j++ {
			if applicable[i].Severity() > applicable[j].Severity() {
				applicable[i], applicable[j] = applicable[j], applicable[i]
			}
		}
	}

	return applicable
}

// shouldExecuteAction determines if an action should be executed based on cooldown
func (m *AutoRemediationManager) shouldExecuteAction(action RemediationAction) bool {
	m.mu.RLock()
	lastExecution, exists := m.lastActionTime[action.Name()]
	m.mu.RUnlock()

	if !exists {
		return true
	}

	return time.Since(lastExecution) >= m.actionCooldown
}

// executeAction executes a remediation action with proper context
func (m *AutoRemediationManager) executeAction(action RemediationAction, metrics SystemMetrics) error {
	ctx, cancel := context.WithTimeout(m.ctx, 30*time.Second)
	defer cancel()

	log.WithFields(log.Fields{
		"action":               action.Name(),
		"goroutines":           metrics.GoroutineCount,
		"memory_mb":            metrics.MemoryAllocMB,
		"gc_pause_ms":          metrics.GCPauseMs,
		"avg_response_time_ms": metrics.AverageResponseTime.Milliseconds(),
	}).Info("[AutoRemediation] Executing remediation action")

	return action.Execute(ctx)
}

// markActionExecuted records when an action was last executed
func (m *AutoRemediationManager) markActionExecuted(action RemediationAction) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastActionTime[action.Name()] = time.Now()
}

// Stop gracefully shuts down the auto-remediation manager
func (m *AutoRemediationManager) Stop() {
	log.Info("[AutoRemediation] Stopping auto-remediation monitoring")
	m.cancel()
	m.wg.Wait()
	log.Info("[AutoRemediation] Auto-remediation monitoring stopped")
}

// Built-in remediation actions

// GCAction triggers garbage collection
type GCAction struct{}

// Name returns the name of the GC action
func (a *GCAction) Name() string { return "garbage_collection" }

// Severity returns the severity level of the GC action
func (a *GCAction) Severity() int { return 1 }

// CanExecute determines if the GC action should be executed based on current metrics
func (a *GCAction) CanExecute(metrics SystemMetrics) bool {
	// Trigger GC when memory is above 60% of max threshold
	gcThreshold := float64(config.AppConfig.ResourceMaxMemoryMB) * 0.6
	return metrics.MemoryAllocMB > gcThreshold || metrics.GCPauseMs > 50
}

// Execute performs the garbage collection action
func (a *GCAction) Execute(ctx context.Context) error {
	log.Info("[AutoRemediation] Triggering garbage collection")
	runtime.GC()
	return nil
}

// MemoryCleanupAction triggers memory cleanup operations
type MemoryCleanupAction struct{}

// Name returns the name of the memory cleanup action
func (a *MemoryCleanupAction) Name() string { return "memory_cleanup" }

// Severity returns the severity level of the memory cleanup action
func (a *MemoryCleanupAction) Severity() int { return 2 }

// CanExecute determines if the memory cleanup action should be executed based on current metrics
func (a *MemoryCleanupAction) CanExecute(metrics SystemMetrics) bool {
	// Trigger cleanup when memory is above GC threshold (80% of max)
	return metrics.MemoryAllocMB > float64(config.AppConfig.ResourceGCThresholdMB)
}

// Execute performs the memory cleanup action
func (a *MemoryCleanupAction) Execute(ctx context.Context) error {
	log.Info("[AutoRemediation] Performing memory cleanup operations")

	// Trigger multiple GC cycles for thorough cleanup
	for range 3 {
		runtime.GC()
		time.Sleep(100 * time.Millisecond)
	}

	// Force release of unused memory back to OS
	runtime.GC()
	return nil
}

// LogWarningAction logs warnings for high resource usage
type LogWarningAction struct{}

// Name returns the name of the log warning action
func (a *LogWarningAction) Name() string { return "log_warning" }

// Severity returns the severity level of the log warning action
func (a *LogWarningAction) Severity() int { return 0 }

// CanExecute determines if the log warning action should be executed based on current metrics
func (a *LogWarningAction) CanExecute(metrics SystemMetrics) bool {
	// Log warning when resources are above 80% of max thresholds
	goroutineThreshold := int(float64(config.AppConfig.ResourceMaxGoroutines) * 0.8)
	memoryThreshold := float64(config.AppConfig.ResourceMaxMemoryMB) * 0.5
	return metrics.GoroutineCount > goroutineThreshold || metrics.MemoryAllocMB > memoryThreshold
}

// Execute logs warning messages about high resource usage
func (a *LogWarningAction) Execute(ctx context.Context) error {
	log.WithFields(log.Fields{
		"goroutines": runtime.NumGoroutine(),
		"memory_mb": func() float64 {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			return float64(m.Alloc) / 1024 / 1024
		}(),
	}).Warn("[AutoRemediation] High resource usage detected")
	return nil
}

// RestartRecommendationAction logs recommendations for restart when critical thresholds are reached
type RestartRecommendationAction struct{}

// Name returns the name of the restart recommendation action
func (a *RestartRecommendationAction) Name() string { return "restart_recommendation" }

// Severity returns the severity level of the restart recommendation action
func (a *RestartRecommendationAction) Severity() int { return 10 }

// CanExecute determines if the restart recommendation action should be executed based on current metrics
func (a *RestartRecommendationAction) CanExecute(metrics SystemMetrics) bool {
	// Recommend restart when resources are above 150% of max thresholds
	goroutineThreshold := int(float64(config.AppConfig.ResourceMaxGoroutines) * 1.5)
	memoryThreshold := float64(config.AppConfig.ResourceMaxMemoryMB) * 1.6
	return metrics.GoroutineCount > goroutineThreshold || metrics.MemoryAllocMB > memoryThreshold
}

// Execute logs critical warnings recommending a manual restart
func (a *RestartRecommendationAction) Execute(ctx context.Context) error {
	log.WithFields(log.Fields{
		"goroutines": runtime.NumGoroutine(),
		"memory_mb": func() float64 {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			return float64(m.Alloc) / 1024 / 1024
		}(),
	}).Error("[AutoRemediation] CRITICAL: Resource usage is dangerously high. Manual restart recommended.")
	return nil
}
