//go:build testtools

package main

import (
	"testing"

	"github.com/divkix/Alita_Robot/alita/utils/shutdown"
)

// The periodic jobs and the stats flusher write to the database, so they must
// stop and flush before the database-close handler, and the jobs stop first so
// the final flush sees their last counters. Registration is DB close, flush,
// jobs stop; shutdown runs handlers in reverse order.
func TestShutdownStopsJobsAndFlushesStatsBeforeClosingDatabase(t *testing.T) {
	manager := shutdown.NewManager()

	var order []string
	registerShutdownDrains(manager,
		func() error {
			order = append(order, "db close")
			return nil
		},
		func() error {
			order = append(order, "stats flush")
			return nil
		},
		func() error {
			order = append(order, "jobs stop")
			return nil
		},
	)

	manager.ExecuteHandlersForTest()

	if len(order) != 3 || order[0] != "jobs stop" || order[1] != "stats flush" || order[2] != "db close" {
		t.Fatalf("handler order = %v, want [jobs stop, stats flush, db close]", order)
	}
}
