//go:build testtools

package monitoring

import (
	"testing"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/utils/jobs"
)

// The job registry is process-wide and has no unregister, so one test covers
// both switch states in order: off registers nothing, then on registers.
func TestAutoRemediationRegistersItsJobOnlyWithPerformanceMonitoring(t *testing.T) {
	// Do not use t.Parallel() - tests global config and job registry state.
	orig := config.AppConfig.EnablePerformanceMonitoring
	t.Cleanup(func() { config.AppConfig.EnablePerformanceMonitoring = orig })

	config.AppConfig.EnablePerformanceMonitoring = false
	NewAutoRemediationManager(NewBackgroundStatsCollector()).Register()
	if jobs.Registered("autoRemediation") {
		t.Fatal("autoRemediation registered with performance monitoring off")
	}

	config.AppConfig.EnablePerformanceMonitoring = true
	NewAutoRemediationManager(NewBackgroundStatsCollector()).Register()
	if !jobs.Registered("autoRemediation") {
		t.Fatal("autoRemediation not registered with performance monitoring on")
	}
}
