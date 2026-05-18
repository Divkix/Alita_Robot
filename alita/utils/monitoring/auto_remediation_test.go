package monitoring

import (
	"context"
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/config"
)

// ---------------------------------------------------------------------------
// GCAction
// ---------------------------------------------------------------------------

func TestGCActionCanExecute(t *testing.T) {
	// Cannot run in parallel because it reads global config.AppConfig.

	action := &GCAction{}
	gcThreshold := float64(config.AppConfig.ResourceMaxMemoryMB) * 0.6

	t.Run("memory above 60% threshold returns true", func(t *testing.T) {
		metrics := SystemMetrics{
			MemoryAllocMB: gcThreshold + 1,
			GCPauseMs:     0,
		}
		if !action.CanExecute(metrics) {
			t.Fatalf("expected CanExecute=true when MemoryAllocMB %.1f > threshold %.1f", metrics.MemoryAllocMB, gcThreshold)
		}
	})

	t.Run("memory below threshold and GCPauseMs below 50 returns false", func(t *testing.T) {
		metrics := SystemMetrics{
			MemoryAllocMB: gcThreshold - 1,
			GCPauseMs:     10,
		}
		if action.CanExecute(metrics) {
			t.Fatalf("expected CanExecute=false when memory and GC pause are both below thresholds")
		}
	})

	t.Run("GCPauseMs above 50 even with low memory returns true", func(t *testing.T) {
		metrics := SystemMetrics{
			MemoryAllocMB: 0,
			GCPauseMs:     51,
		}
		if !action.CanExecute(metrics) {
			t.Fatalf("expected CanExecute=true when GCPauseMs=51 > 50")
		}
	})

	t.Run("zero-value SystemMetrics returns false", func(t *testing.T) {
		metrics := SystemMetrics{}
		if action.CanExecute(metrics) {
			t.Fatal("expected CanExecute=false for zero-value SystemMetrics")
		}
	})
}

// ---------------------------------------------------------------------------
// MemoryCleanupAction
// ---------------------------------------------------------------------------

func TestMemoryCleanupActionCanExecute(t *testing.T) {
	// Cannot run in parallel because it reads global config.AppConfig.

	action := &MemoryCleanupAction{}
	threshold := float64(config.AppConfig.ResourceGCThresholdMB)

	t.Run("above ResourceGCThresholdMB returns true", func(t *testing.T) {
		metrics := SystemMetrics{MemoryAllocMB: threshold + 1}
		if !action.CanExecute(metrics) {
			t.Fatalf("expected CanExecute=true when MemoryAllocMB %.1f > threshold %.1f", metrics.MemoryAllocMB, threshold)
		}
	})

	t.Run("below ResourceGCThresholdMB returns false", func(t *testing.T) {
		metrics := SystemMetrics{MemoryAllocMB: threshold - 1}
		if action.CanExecute(metrics) {
			t.Fatalf("expected CanExecute=false when MemoryAllocMB %.1f < threshold %.1f", metrics.MemoryAllocMB, threshold)
		}
	})
}

// ---------------------------------------------------------------------------
// LogWarningAction & RestartRecommendationAction (threshold-based actions)
// ---------------------------------------------------------------------------

func TestThresholdBasedActionsCanExecute(t *testing.T) {
	// Cannot run in parallel because it reads global config.AppConfig.

	cases := []struct {
		name               string
		action             RemediationAction
		goroutineThreshold int
		memoryThreshold    float64
	}{
		{
			name:               "LogWarningAction",
			action:             &LogWarningAction{},
			goroutineThreshold: int(float64(config.AppConfig.ResourceMaxGoroutines) * 0.8),
			memoryThreshold:    float64(config.AppConfig.ResourceMaxMemoryMB) * 0.5,
		},
		{
			name:               "RestartRecommendationAction",
			action:             &RestartRecommendationAction{},
			goroutineThreshold: int(float64(config.AppConfig.ResourceMaxGoroutines) * 1.5),
			memoryThreshold:    float64(config.AppConfig.ResourceMaxMemoryMB) * 1.6,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name+"/goroutines above threshold returns true", func(t *testing.T) {
			metrics := SystemMetrics{GoroutineCount: tc.goroutineThreshold + 1, MemoryAllocMB: 0}
			if !tc.action.CanExecute(metrics) {
				t.Fatalf("expected CanExecute=true when goroutines %d > threshold %d", metrics.GoroutineCount, tc.goroutineThreshold)
			}
		})

		t.Run(tc.name+"/memory above threshold returns true", func(t *testing.T) {
			metrics := SystemMetrics{GoroutineCount: 0, MemoryAllocMB: tc.memoryThreshold + 1}
			if !tc.action.CanExecute(metrics) {
				t.Fatalf("expected CanExecute=true when memory %.1f > threshold %.1f", metrics.MemoryAllocMB, tc.memoryThreshold)
			}
		})

		t.Run(tc.name+"/both below thresholds returns false", func(t *testing.T) {
			metrics := SystemMetrics{GoroutineCount: tc.goroutineThreshold - 1, MemoryAllocMB: tc.memoryThreshold - 1}
			if tc.action.CanExecute(metrics) {
				t.Fatal("expected CanExecute=false when both metrics are below thresholds")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Action Names
// ---------------------------------------------------------------------------

func TestActionNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		action   RemediationAction
		wantName string
	}{
		{name: "GCAction", action: &GCAction{}, wantName: "garbage_collection"},
		{name: "MemoryCleanupAction", action: &MemoryCleanupAction{}, wantName: "memory_cleanup"},
		{name: "LogWarningAction", action: &LogWarningAction{}, wantName: "log_warning"},
		{name: "RestartRecommendationAction", action: &RestartRecommendationAction{}, wantName: "restart_recommendation"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := tc.action.Name()
			if got != tc.wantName {
				t.Fatalf("%s.Name() = %q, want %q", tc.name, got, tc.wantName)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Action Severity Ordering
// ---------------------------------------------------------------------------

func TestActionSeverityOrdering(t *testing.T) {
	t.Parallel()

	logWarn := &LogWarningAction{}
	gc := &GCAction{}
	memClean := &MemoryCleanupAction{}
	restart := &RestartRecommendationAction{}

	t.Run("LogWarning severity is 0", func(t *testing.T) {
		t.Parallel()
		if logWarn.Severity() != 0 {
			t.Fatalf("expected LogWarning.Severity()=0, got %d", logWarn.Severity())
		}
	})

	t.Run("GC severity is 1", func(t *testing.T) {
		t.Parallel()
		if gc.Severity() != 1 {
			t.Fatalf("expected GC.Severity()=1, got %d", gc.Severity())
		}
	})

	t.Run("MemoryCleanup severity is 2", func(t *testing.T) {
		t.Parallel()
		if memClean.Severity() != 2 {
			t.Fatalf("expected MemoryCleanup.Severity()=2, got %d", memClean.Severity())
		}
	})

	t.Run("RestartRecommendation severity is 10", func(t *testing.T) {
		t.Parallel()
		if restart.Severity() != 10 {
			t.Fatalf("expected RestartRecommendation.Severity()=10, got %d", restart.Severity())
		}
	})

	t.Run("severity ordering LogWarning < GC < MemoryCleanup < RestartRecommendation", func(t *testing.T) {
		t.Parallel()

		if logWarn.Severity() >= gc.Severity() {
			t.Fatalf("expected LogWarning(%d) < GC(%d)", logWarn.Severity(), gc.Severity())
		}
		if gc.Severity() >= memClean.Severity() {
			t.Fatalf("expected GC(%d) < MemoryCleanup(%d)", gc.Severity(), memClean.Severity())
		}
		if memClean.Severity() >= restart.Severity() {
			t.Fatalf("expected MemoryCleanup(%d) < RestartRecommendation(%d)", memClean.Severity(), restart.Severity())
		}
	})
}

// ---------------------------------------------------------------------------
// Additional: AggressiveGCAction (RestartRecommendationAction at 150%+ threshold)
// ---------------------------------------------------------------------------

// TestAggressiveGCAction_CanExecute tests RestartRecommendationAction which triggers
// at 150% goroutine threshold or 160% memory threshold -- the most aggressive action.
func TestAggressiveGCAction_CanExecute(t *testing.T) {
	// Cannot run in parallel because it reads global config.AppConfig.

	action := &RestartRecommendationAction{}
	goroutineThreshold := int(float64(config.AppConfig.ResourceMaxGoroutines) * 1.5)
	memoryThreshold := float64(config.AppConfig.ResourceMaxMemoryMB) * 1.6

	t.Run("metrics below 150% thresholds returns false", func(t *testing.T) {
		metrics := SystemMetrics{
			GoroutineCount: goroutineThreshold - 1,
			MemoryAllocMB:  memoryThreshold - 1,
		}
		if action.CanExecute(metrics) {
			t.Fatalf("expected CanExecute=false when goroutines=%d < threshold=%d and memory=%.1f < threshold=%.1f",
				metrics.GoroutineCount, goroutineThreshold, metrics.MemoryAllocMB, memoryThreshold)
		}
	})

	t.Run("memory above 160% threshold returns true", func(t *testing.T) {
		metrics := SystemMetrics{
			GoroutineCount: 0,
			MemoryAllocMB:  memoryThreshold + 1,
		}
		if !action.CanExecute(metrics) {
			t.Fatalf("expected CanExecute=true when MemoryAllocMB=%.1f > threshold=%.1f",
				metrics.MemoryAllocMB, memoryThreshold)
		}
	})

	t.Run("goroutines above 150% threshold returns true", func(t *testing.T) {
		metrics := SystemMetrics{
			GoroutineCount: goroutineThreshold + 1,
			MemoryAllocMB:  0,
		}
		if !action.CanExecute(metrics) {
			t.Fatalf("expected CanExecute=true when goroutines=%d > threshold=%d",
				metrics.GoroutineCount, goroutineThreshold)
		}
	})

	t.Run("zero-value metrics returns false", func(t *testing.T) {
		metrics := SystemMetrics{}
		if action.CanExecute(metrics) {
			t.Fatal("expected CanExecute=false for zero-value SystemMetrics")
		}
	})
}

// ---------------------------------------------------------------------------
// Additional: WarningAction (LogWarningAction at 80% goroutine / 50% memory threshold)
// ---------------------------------------------------------------------------

// TestWarningAction_CanExecute tests LogWarningAction which triggers at 80% goroutine
// threshold or 50% memory threshold.
func TestWarningAction_CanExecute(t *testing.T) {
	// Cannot run in parallel because it reads global config.AppConfig.

	action := &LogWarningAction{}
	goroutineThreshold := int(float64(config.AppConfig.ResourceMaxGoroutines) * 0.8)
	memoryThreshold := float64(config.AppConfig.ResourceMaxMemoryMB) * 0.5

	t.Run("metrics below 80% thresholds returns false", func(t *testing.T) {
		metrics := SystemMetrics{
			GoroutineCount: goroutineThreshold - 1,
			MemoryAllocMB:  memoryThreshold - 1,
		}
		if action.CanExecute(metrics) {
			t.Fatalf("expected CanExecute=false when goroutines=%d < threshold=%d and memory=%.1f < threshold=%.1f",
				metrics.GoroutineCount, goroutineThreshold, metrics.MemoryAllocMB, memoryThreshold)
		}
	})

	t.Run("memory above 50% ResourceMaxMemoryMB returns true", func(t *testing.T) {
		metrics := SystemMetrics{
			GoroutineCount: 0,
			MemoryAllocMB:  memoryThreshold + 1,
		}
		if !action.CanExecute(metrics) {
			t.Fatalf("expected CanExecute=true when MemoryAllocMB=%.1f > threshold=%.1f",
				metrics.MemoryAllocMB, memoryThreshold)
		}
	})

	t.Run("goroutines above 80% ResourceMaxGoroutines returns true", func(t *testing.T) {
		metrics := SystemMetrics{
			GoroutineCount: goroutineThreshold + 1,
			MemoryAllocMB:  0,
		}
		if !action.CanExecute(metrics) {
			t.Fatalf("expected CanExecute=true when goroutines=%d > threshold=%d",
				metrics.GoroutineCount, goroutineThreshold)
		}
	})
}

// ---------------------------------------------------------------------------
// Additional: GCAction Name and Severity
// ---------------------------------------------------------------------------

// TestGCAction_NameAndSeverity tests that all built-in actions have non-empty names
// and valid severity values.
func TestGCAction_NameAndSeverity(t *testing.T) {
	t.Parallel()

	actions := []RemediationAction{
		&GCAction{},
		&MemoryCleanupAction{},
		&LogWarningAction{},
		&RestartRecommendationAction{},
	}

	for _, action := range actions {
		action := action
		t.Run(action.Name(), func(t *testing.T) {
			t.Parallel()

			name := action.Name()
			if name == "" {
				t.Fatalf("%T.Name() returned empty string", action)
			}

			severity := action.Severity()
			if severity < 0 {
				t.Fatalf("%T.Severity() = %d, must be >= 0", action, severity)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Execute methods — no panic, return nil
// ---------------------------------------------------------------------------

func TestExecuteActions_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		newAction func() RemediationAction
	}{
		{"GCAction", func() RemediationAction { return &GCAction{} }},
		{"MemoryCleanupAction", func() RemediationAction { return &MemoryCleanupAction{} }},
		{"LogWarningAction", func() RemediationAction { return &LogWarningAction{} }},
		{"RestartRecommendationAction", func() RemediationAction { return &RestartRecommendationAction{} }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.newAction().Execute(context.Background()); err != nil {
				t.Fatalf("expected Execute to return nil, got %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// NewAutoRemediationManager — disabled monitoring
// ---------------------------------------------------------------------------

func TestNewAutoRemediationManager_Disabled_StartDoesNothing(t *testing.T) {
	// Do not use t.Parallel() - tests global config state

	origEnabled := config.AppConfig.EnablePerformanceMonitoring
	config.AppConfig.EnablePerformanceMonitoring = false
	defer func() {
		config.AppConfig.EnablePerformanceMonitoring = origEnabled
	}()

	collector := NewBackgroundStatsCollector()
	manager := NewAutoRemediationManager(collector)

	if manager.enabled {
		t.Fatal("expected manager.enabled to be false when EnablePerformanceMonitoring is false")
	}

	// Start() should return immediately without spawning goroutines
	manager.Start()

	// Stop should not deadlock even though Start() did nothing
	manager.Stop()
}

// ---------------------------------------------------------------------------
// NewAutoRemediationManager — with collector and metrics
// ---------------------------------------------------------------------------

func TestNewAutoRemediationManager_WithMetrics(t *testing.T) {
	// Do not use t.Parallel() - tests global config state

	origMaxGoroutines := config.AppConfig.ResourceMaxGoroutines
	origMaxMemoryMB := config.AppConfig.ResourceMaxMemoryMB
	origGCThresholdMB := config.AppConfig.ResourceGCThresholdMB
	origEnabled := config.AppConfig.EnablePerformanceMonitoring

	config.AppConfig.ResourceMaxGoroutines = 100
	config.AppConfig.ResourceMaxMemoryMB = 500
	config.AppConfig.ResourceGCThresholdMB = 400
	config.AppConfig.EnablePerformanceMonitoring = true
	defer func() {
		config.AppConfig.ResourceMaxGoroutines = origMaxGoroutines
		config.AppConfig.ResourceMaxMemoryMB = origMaxMemoryMB
		config.AppConfig.ResourceGCThresholdMB = origGCThresholdMB
		config.AppConfig.EnablePerformanceMonitoring = origEnabled
	}()

	collector := NewBackgroundStatsCollector()
	collector.RecordMessage()
	collector.RecordError()
	collector.RecordResponseTime(50 * time.Millisecond)

	manager := NewAutoRemediationManager(collector)

	if manager.collector != collector {
		t.Fatal("expected manager.collector to be the same collector passed in")
	}
	if !manager.enabled {
		t.Fatal("expected manager.enabled to be true when EnablePerformanceMonitoring is true")
	}
	if manager.thresholds.MaxGoroutines != 100 {
		t.Errorf("expected MaxGoroutines=100, got %d", manager.thresholds.MaxGoroutines)
	}
	if manager.thresholds.MaxMemoryMB != 500 {
		t.Errorf("expected MaxMemoryMB=500, got %f", manager.thresholds.MaxMemoryMB)
	}
	if manager.thresholds.CriticalGoroutines != 200 {
		t.Errorf("expected CriticalGoroutines=200, got %d", manager.thresholds.CriticalGoroutines)
	}
	if manager.thresholds.CriticalMemoryMB != 1000 {
		t.Errorf("expected CriticalMemoryMB=1000, got %f", manager.thresholds.CriticalMemoryMB)
	}
}

// ---------------------------------------------------------------------------
// AutoRemediationManager — getApplicableActions
// ---------------------------------------------------------------------------

func TestAutoRemediationManager_GetApplicableActions(t *testing.T) {
	// Do not use t.Parallel() - tests global config state

	origMaxGoroutines := config.AppConfig.ResourceMaxGoroutines
	origMaxMemoryMB := config.AppConfig.ResourceMaxMemoryMB
	origGCThresholdMB := config.AppConfig.ResourceGCThresholdMB
	origEnabled := config.AppConfig.EnablePerformanceMonitoring

	config.AppConfig.ResourceMaxGoroutines = 100
	config.AppConfig.ResourceMaxMemoryMB = 500
	config.AppConfig.ResourceGCThresholdMB = 400
	config.AppConfig.EnablePerformanceMonitoring = true
	defer func() {
		config.AppConfig.ResourceMaxGoroutines = origMaxGoroutines
		config.AppConfig.ResourceMaxMemoryMB = origMaxMemoryMB
		config.AppConfig.ResourceGCThresholdMB = origGCThresholdMB
		config.AppConfig.EnablePerformanceMonitoring = origEnabled
	}()

	collector := NewBackgroundStatsCollector()
	manager := NewAutoRemediationManager(collector)

	// Metrics that trigger all applicable actions
	metrics := SystemMetrics{
		GoroutineCount:      200, // above all thresholds
		MemoryAllocMB:       600, // above all thresholds
		GCPauseMs:           60,
		AverageResponseTime: 10 * time.Millisecond,
	}

	applicable := manager.getApplicableActions(metrics)
	if len(applicable) == 0 {
		t.Fatal("expected some applicable actions")
	}

	// Verify sorted by severity (ascending)
	for i := 1; i < len(applicable); i++ {
		if applicable[i].Severity() < applicable[i-1].Severity() {
			t.Fatalf("actions not sorted by severity: %d at index %d < %d at index %d",
				applicable[i].Severity(), i, applicable[i-1].Severity(), i-1)
		}
	}
}

// ---------------------------------------------------------------------------
// AutoRemediationManager — shouldExecuteAction cooldown
// ---------------------------------------------------------------------------

func TestAutoRemediationManager_ShouldExecuteAction_Cooldown(t *testing.T) {
	// Do not use t.Parallel() - tests global config state

	origEnabled := config.AppConfig.EnablePerformanceMonitoring
	config.AppConfig.EnablePerformanceMonitoring = true
	defer func() {
		config.AppConfig.EnablePerformanceMonitoring = origEnabled
	}()

	collector := NewBackgroundStatsCollector()
	manager := NewAutoRemediationManager(collector)
	action := &GCAction{}

	// First execution should be allowed
	if !manager.shouldExecuteAction(action) {
		t.Fatal("expected shouldExecuteAction to be true on first call")
	}

	// Record execution
	manager.markActionExecuted(action)

	// Second execution should be blocked by cooldown
	if manager.shouldExecuteAction(action) {
		t.Fatal("expected shouldExecuteAction to be false immediately after execution")
	}
}
