//go:build testtools

package app

import (
	"errors"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/utils/shutdown"
)

// The shutdown contract is declared as data: every drain carries a phase, and
// the registered set covers every phase Phases() lists. A drain added without
// a phase — or in a phase outside the declared order — fails here instead of
// silently landing in the wrong position.
func TestShutdownDrainsDeclareEveryPhase(t *testing.T) {
	// ShutdownDrainSpecs declares the shared tail: database close, stats
	// flush, jobs stop and the module drains.
	specs := ShutdownDrainSpecs(
		func() error { return nil },
		func() error { return nil },
		func() error { return nil },
		[]DrainSpec{
			{Name: "users drain", Handler: func() error { return nil }},
			{Name: "captcha stop", Handler: func() error { return nil }},
			{Name: "aispam drain", Handler: func() error { return nil }},
		},
	)

	// The remaining drains are registered directly: "updater stop" in
	// runPolling (polling mode only), "http stop" and "tracer" in Run. Their
	// phases are asserted here so the whole set stays enumerable.
	direct := []struct {
		name  string
		phase shutdown.Phase
	}{
		{"updater stop", shutdown.PhaseUpdateSource},
		{"http stop", shutdown.PhaseHTTPServer},
		{"tracer", shutdown.PhaseTelemetry},
	}

	phases := shutdown.Phases()
	seen := make(map[shutdown.Phase]bool)
	for _, spec := range specs {
		if !slices.Contains(phases, spec.Phase) {
			t.Fatalf("drain %q declares phase %v, which Phases() does not list", spec.Name, spec.Phase)
		}
		seen[spec.Phase] = true
	}
	for _, d := range direct {
		if !slices.Contains(phases, d.phase) {
			t.Fatalf("drain %q declares phase %v, which Phases() does not list", d.name, d.phase)
		}
		seen[d.phase] = true
	}
	for _, phase := range phases {
		if !seen[phase] {
			t.Fatalf("no drain declares phase %v", phase)
		}
	}
}

// No drain registered after the database phase may write to the database: the
// database is already closed when they run. This registers the same nine
// drains Run registers — same names, same phases — against a real SQLite
// database. A write probe runs in front of every drain: each probe before the
// close must land, each probe after it must fail, and a drain that writes
// after the close makes its probe report an error.
func TestNoDrainAfterDatabasePhaseWritesToTheDatabase(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "shutdown.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.Exec("CREATE TABLE probe (id INTEGER)").Error; err != nil {
		t.Fatalf("create probe table: %v", err)
	}

	previousDB := db.DB
	db.DB = database
	t.Cleanup(func() { db.DB = previousDB })

	var mu sync.Mutex
	var closed bool
	var writesBeforeClose int
	var probeErrors []error
	probe := func() {
		err := database.Exec("INSERT INTO probe VALUES (1)").Error
		mu.Lock()
		defer mu.Unlock()
		switch {
		case err == nil && !closed:
			writesBeforeClose++
		case err != nil && !closed:
			probeErrors = append(probeErrors, err) // write failed while the DB should be open
		case err == nil && closed:
			probeErrors = append(probeErrors, errors.New("database write landed after the database phase"))
		}
	}

	// wrap puts the probe in front of a drain, so the probe observes the
	// database state the drain itself would see.
	wrap := func(name string, phase shutdown.Phase) DrainSpec {
		return DrainSpec{Name: name, Phase: phase, Handler: func() error {
			probe()
			return nil
		}}
	}

	manager := shutdown.NewManager()
	RegisterShutdownSteps(manager, append(ShutdownDrainSpecs(
		func() error {
			err := closeDBConnections()
			mu.Lock()
			closed = true
			mu.Unlock()
			return err
		},
		func() error { probe(); return nil }, // stats flush
		func() error { probe(); return nil }, // jobs stop
		[]DrainSpec{
			wrap("users drain", 0), // module drains get their phase from the specs helper
			wrap("captcha stop", 0),
			wrap("aispam drain", 0),
		},
	),
		wrap("updater stop", shutdown.PhaseUpdateSource),
		wrap("http stop", shutdown.PhaseHTTPServer),
		wrap("tracer", shutdown.PhaseTelemetry),
	)...)

	manager.ExecuteHandlersForTest()

	mu.Lock()
	defer mu.Unlock()
	// Seven drains run before the database phase — updater, http, the three
	// module drains, jobs and the stats flush — and each probe must have
	// landed while the database was open.
	if writesBeforeClose != 7 {
		t.Fatalf("writes before the database phase = %d, want 7", writesBeforeClose)
	}
	for _, err := range probeErrors {
		t.Errorf("probe: %v", err)
	}
}
