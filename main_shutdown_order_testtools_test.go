//go:build testtools

package main

import (
	"testing"

	"github.com/divkix/Alita_Robot/alita/utils/shutdown"
)

// The stats flusher drains into the database, so its flush must run before the
// database-close handler. Registration is DB close first, then the flush;
// shutdown runs handlers in reverse order.
func TestShutdownFlushesStatsBeforeClosingDatabase(t *testing.T) {
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
	)

	manager.ExecuteHandlersForTest()

	if len(order) != 2 || order[0] != "stats flush" || order[1] != "db close" {
		t.Fatalf("handler order = %v, want [stats flush, db close]", order)
	}
}
