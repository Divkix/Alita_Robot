package monitoring

import (
	"testing"

	"github.com/divkix/Alita_Robot/alita/config"
)

// ---------------------------------------------------------------------------
// GCAction
// ---------------------------------------------------------------------------

func TestGCActionCanExecute(t *testing.T) {
	t.Parallel()

	action := &GCAction{}
	gcThreshold := float64(config.AppConfig.ResourceMaxMemoryMB) * 0.6

	t.Run("memory above 60% threshold returns true", func(t *testing.T) {
		t.Parallel()

		metrics := SystemMetrics{
			MemoryAllocMB: gcThreshold + 1,
			GCPauseMs:     0,
		}
		if !action.CanExecute(metrics) {
			t.Fatalf("expected CanExecute=true when MemoryAllocMB %.1f > threshold %.1f", metrics.MemoryAllocMB, gcThreshold)
		}
	})

	t.Run("memory below threshold and GCPauseMs below 50 returns false", func(t *testing.T) {
		t.Parallel()

		metrics := SystemMetrics{
			MemoryAllocMB: gcThreshold - 1,
			GCPauseMs:     10,
		}
		if action.CanExecute(metrics) {
			t.Fatalf("expected CanExecute=false when memory and GC pause are both below thresholds")
		}
	})

	t.Run("GCPauseMs above 50 even with low memory returns true", func(t *testing.T) {
		t.Parallel()

		metrics := SystemMetrics{
			MemoryAllocMB: 0,
			GCPauseMs:     51,
		}
		if !action.CanExecute(metrics) {
			t.Fatalf("expected CanExecute=true when GCPauseMs=51 > 50")
		}
	})

	t.Run("zero-value SystemMetrics returns false", func(t *testing.T) {
		t.Parallel()

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
	t.Parallel()

	action := &MemoryCleanupAction{}
	threshold := float64(config.AppConfig.ResourceGCThresholdMB)

	t.Run("above ResourceGCThresholdMB returns true", func(t *testing.T) {
		t.Parallel()

		metrics := SystemMetrics{MemoryAllocMB: threshold + 1}
		if !action.CanExecute(metrics) {
			t.Fatalf("expected CanExecute=true when MemoryAllocMB %.1f > threshold %.1f", metrics.MemoryAllocMB, threshold)
		}
	})

	t.Run("below ResourceGCThresholdMB returns false", func(t *testing.T) {
		t.Parallel()

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
	t.Parallel()

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
			t.Parallel()
			metrics := SystemMetrics{GoroutineCount: tc.goroutineThreshold + 1, MemoryAllocMB: 0}
			if !tc.action.CanExecute(metrics) {
				t.Fatalf("expected CanExecute=true when goroutines %d > threshold %d", metrics.GoroutineCount, tc.goroutineThreshold)
			}
		})

		t.Run(tc.name+"/memory above threshold returns true", func(t *testing.T) {
			t.Parallel()
			metrics := SystemMetrics{GoroutineCount: 0, MemoryAllocMB: tc.memoryThreshold + 1}
			if !tc.action.CanExecute(metrics) {
				t.Fatalf("expected CanExecute=true when memory %.1f > threshold %.1f", metrics.MemoryAllocMB, tc.memoryThreshold)
			}
		})

		t.Run(tc.name+"/both below thresholds returns false", func(t *testing.T) {
			t.Parallel()
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

		if !(logWarn.Severity() < gc.Severity()) {
			t.Fatalf("expected LogWarning(%d) < GC(%d)", logWarn.Severity(), gc.Severity())
		}
		if !(gc.Severity() < memClean.Severity()) {
			t.Fatalf("expected GC(%d) < MemoryCleanup(%d)", gc.Severity(), memClean.Severity())
		}
		if !(memClean.Severity() < restart.Severity()) {
			t.Fatalf("expected MemoryCleanup(%d) < RestartRecommendation(%d)", memClean.Severity(), restart.Severity())
		}
	})
}
