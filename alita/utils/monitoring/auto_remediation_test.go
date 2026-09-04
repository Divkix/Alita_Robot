package monitoring

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/config"
)

func findAction(t *testing.T, m *AutoRemediationManager, name string) remediationAction {
	t.Helper()
	for _, a := range m.actions {
		if a.name == name {
			return a
		}
	}
	t.Fatalf("action %q not found in manager.actions", name)
	return remediationAction{}
}

func TestBuiltInActionsAreSeverityOrdered(t *testing.T) {
	// Cannot run in parallel because it reads global config.AppConfig.

	manager := NewAutoRemediationManager(NewBackgroundStatsCollector())

	wantOrder := []struct {
		name     string
		severity int
	}{
		{"log_warning", 0},
		{"garbage_collection", 1},
		{"memory_cleanup", 2},
		{"restart_recommendation", 10},
	}

	if len(manager.actions) != len(wantOrder) {
		t.Fatalf("got %d actions, want %d", len(manager.actions), len(wantOrder))
	}

	for i, want := range wantOrder {
		got := manager.actions[i]
		if got.name != want.name || got.severity != want.severity {
			t.Fatalf("action[%d] = {%q, %d}, want {%q, %d}", i, got.name, got.severity, want.name, want.severity)
		}
		if i > 0 && manager.actions[i-1].severity >= got.severity {
			t.Fatalf("actions not in ascending severity order at index %d", i)
		}
	}
}

func TestActionThresholds(t *testing.T) {
	// Cannot run in parallel because it reads global config.AppConfig.

	manager := NewAutoRemediationManager(NewBackgroundStatsCollector())

	maxGoroutines := config.AppConfig.ResourceMaxGoroutines
	maxMemoryMB := config.AppConfig.ResourceMaxMemoryMB
	gcThresholdMB := config.AppConfig.ResourceGCThresholdMB

	t.Run("log_warning", func(t *testing.T) {
		a := findAction(t, manager, "log_warning")
		goroutineThreshold := int(float64(maxGoroutines) * 0.8)
		memoryThreshold := float64(maxMemoryMB) * 0.5

		if a.canExecute(SystemMetrics{GoroutineCount: goroutineThreshold - 1, MemoryAllocMB: memoryThreshold - 1}) {
			t.Fatal("expected false when both metrics below thresholds")
		}
		if !a.canExecute(SystemMetrics{GoroutineCount: goroutineThreshold + 1}) {
			t.Fatal("expected true when goroutines above threshold")
		}
		if !a.canExecute(SystemMetrics{MemoryAllocMB: memoryThreshold + 1}) {
			t.Fatal("expected true when memory above threshold")
		}
	})

	t.Run("garbage_collection", func(t *testing.T) {
		a := findAction(t, manager, "garbage_collection")
		gcThreshold := float64(maxMemoryMB) * 0.6

		if a.canExecute(SystemMetrics{MemoryAllocMB: gcThreshold - 1, GCPauseMs: 10}) {
			t.Fatal("expected false when memory and GC pause below thresholds")
		}
		if !a.canExecute(SystemMetrics{MemoryAllocMB: gcThreshold + 1}) {
			t.Fatal("expected true when memory above 60% threshold")
		}
		if !a.canExecute(SystemMetrics{GCPauseMs: 51}) {
			t.Fatal("expected true when GCPauseMs above 50")
		}
	})

	t.Run("memory_cleanup", func(t *testing.T) {
		a := findAction(t, manager, "memory_cleanup")
		threshold := float64(gcThresholdMB)

		if a.canExecute(SystemMetrics{MemoryAllocMB: threshold - 1}) {
			t.Fatal("expected false when memory below ResourceGCThresholdMB")
		}
		if !a.canExecute(SystemMetrics{MemoryAllocMB: threshold + 1}) {
			t.Fatal("expected true when memory above ResourceGCThresholdMB")
		}
	})

	t.Run("restart_recommendation", func(t *testing.T) {
		a := findAction(t, manager, "restart_recommendation")
		goroutineThreshold := int(float64(maxGoroutines) * 1.5)
		memoryThreshold := float64(maxMemoryMB) * 1.6

		if a.canExecute(SystemMetrics{GoroutineCount: goroutineThreshold - 1, MemoryAllocMB: memoryThreshold - 1}) {
			t.Fatal("expected false when both metrics below 150%/160% thresholds")
		}
		if !a.canExecute(SystemMetrics{GoroutineCount: goroutineThreshold + 1}) {
			t.Fatal("expected true when goroutines above 150% threshold")
		}
		if !a.canExecute(SystemMetrics{MemoryAllocMB: memoryThreshold + 1}) {
			t.Fatal("expected true when memory above 160% threshold")
		}
	})
}

func TestBuiltInActionsExecuteDoNotPanic(t *testing.T) {
	// Cannot run in parallel because it reads global config.AppConfig.

	manager := NewAutoRemediationManager(NewBackgroundStatsCollector())
	for _, a := range manager.actions {
		t.Run(a.name, func(t *testing.T) {
			a.execute()
		})
	}
}

func TestNewAutoRemediationManager_Disabled_StartDoesNothing(t *testing.T) {
	// Do not use t.Parallel() - tests global config state.

	origEnabled := config.AppConfig.EnablePerformanceMonitoring
	config.AppConfig.EnablePerformanceMonitoring = false
	defer func() {
		config.AppConfig.EnablePerformanceMonitoring = origEnabled
	}()

	manager := NewAutoRemediationManager(NewBackgroundStatsCollector())

	if manager.enabled {
		t.Fatal("expected manager.enabled to be false when EnablePerformanceMonitoring is false")
	}

	manager.Start()
	manager.Stop()
}

func TestNewAutoRemediationManager_WithCollector(t *testing.T) {
	// Do not use t.Parallel() - tests global config state.

	origEnabled := config.AppConfig.EnablePerformanceMonitoring
	config.AppConfig.EnablePerformanceMonitoring = true
	defer func() {
		config.AppConfig.EnablePerformanceMonitoring = origEnabled
	}()

	collector := NewBackgroundStatsCollector()
	manager := NewAutoRemediationManager(collector)

	if manager.collector != collector {
		t.Fatal("expected manager.collector to be the collector passed in")
	}
	if !manager.enabled {
		t.Fatal("expected manager.enabled to be true when EnablePerformanceMonitoring is true")
	}
}

func TestAutoRemediationManager_Cooldown(t *testing.T) {
	// Do not use t.Parallel() - tests global config state.

	manager := NewAutoRemediationManager(NewBackgroundStatsCollector())

	if !manager.shouldExecuteAction("garbage_collection") {
		t.Fatal("expected shouldExecuteAction to be true on first call")
	}

	manager.markActionExecuted("garbage_collection")

	if manager.shouldExecuteAction("garbage_collection") {
		t.Fatal("expected shouldExecuteAction to be false immediately after execution")
	}
}

func TestCheckAndRemediateExecutesLowestSeverityAction(t *testing.T) {
	// Do not use t.Parallel() - tests global config state.

	collector := NewBackgroundStatsCollector()
	collector.updateSystemMetrics(SystemMetrics{
		GoroutineCount: 200,
		MemoryAllocMB:  600,
		GCPauseMs:      75,
	})

	manager := NewAutoRemediationManager(collector)

	var lowExec, highExec int32
	// Ordered ascending by severity, mirroring the constructor invariant.
	manager.actions = []remediationAction{
		{name: "low", severity: 1, canExecute: func(SystemMetrics) bool { return true }, execute: func() { atomic.AddInt32(&lowExec, 1) }},
		{name: "high", severity: 5, canExecute: func(SystemMetrics) bool { return true }, execute: func() { atomic.AddInt32(&highExec, 1) }},
	}

	manager.checkAndRemediate()

	if atomic.LoadInt32(&lowExec) != 1 {
		t.Fatalf("low severity action executions = %d, want 1", lowExec)
	}
	if atomic.LoadInt32(&highExec) != 0 {
		t.Fatalf("high severity action executions = %d, want 0 (one action per cycle)", highExec)
	}
	if manager.shouldExecuteAction("low") {
		t.Fatal("executed action was not put on cooldown")
	}
}

func TestCheckAndRemediateHandlesNilCollector(t *testing.T) {
	t.Parallel()

	manager := NewAutoRemediationManager(nil)
	manager.checkAndRemediate()
}

func TestAutoRemediationManagerStartRunsMonitorLoop(t *testing.T) {
	// Do not use t.Parallel() - tests global config state.

	origEnabled := config.AppConfig.EnablePerformanceMonitoring
	config.AppConfig.EnablePerformanceMonitoring = true
	defer func() {
		config.AppConfig.EnablePerformanceMonitoring = origEnabled
	}()

	collector := NewBackgroundStatsCollector()
	collector.updateSystemMetrics(SystemMetrics{
		GoroutineCount: 200,
		MemoryAllocMB:  600,
		GCPauseMs:      75,
	})

	manager := NewAutoRemediationManager(collector)

	var executed int32
	manager.actions = []remediationAction{
		{name: "fast-loop", severity: 1, canExecute: func(SystemMetrics) bool { return true }, execute: func() { atomic.AddInt32(&executed, 1) }},
	}
	manager.actionCooldown = 0
	manager.monitorInterval = time.Millisecond

	manager.Start()
	t.Cleanup(manager.Stop)

	deadline := time.After(500 * time.Millisecond)
	for {
		if atomic.LoadInt32(&executed) > 0 {
			return
		}
		select {
		case <-deadline:
			t.Fatal("expected Start monitor loop to execute applicable action")
		default:
			time.Sleep(time.Millisecond)
		}
	}
}
