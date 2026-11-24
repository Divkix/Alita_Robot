//go:build testtools

package app

import (
	"errors"
	"net/http"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/utils/httpserver"
	"github.com/divkix/Alita_Robot/alita/utils/shutdown"
)

type healthCheckTransport func(*http.Request) (*http.Response, error)

func (transport healthCheckTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return transport(req)
}

func TestHealthCheckCLIPort(t *testing.T) {
	previousConfig := config.AppConfig
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = previousConfig })

	tests := []struct {
		name     string
		httpPort string
		port     string
		wantPort string
	}{
		{name: "HTTP_PORT takes precedence", httpPort: "8081", port: "9091", wantPort: "8081"},
		{name: "PORT fallback", port: "9091", wantPort: "9091"},
		{name: "default port", wantPort: "8080"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("HTTP_PORT", test.httpPort)
			t.Setenv("PORT", test.port)

			var requestedURL string
			previousClient := http.DefaultClient
			http.DefaultClient = &http.Client{Transport: healthCheckTransport(func(req *http.Request) (*http.Response, error) {
				requestedURL = req.URL.String()
				return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
			})}
			t.Cleanup(func() { http.DefaultClient = previousClient })

			if err := HealthCheck(); err != nil {
				t.Fatalf("HealthCheck() failed: %v", err)
			}
			wantURL := "http://localhost:" + test.wantPort + "/health"
			if requestedURL != wantURL {
				t.Errorf("HealthCheck() requested %q, want %q", requestedURL, wantURL)
			}
		})
	}
}

// The shutdown contract is declared as data: every drain carries a phase, and
// the registered set covers every phase Phases() lists. A drain added without
// a phase — or in a phase outside the declared order — fails here instead of
// silently landing in the wrong position.
func TestShutdownDrainsDeclareEveryPhase(t *testing.T) {
	// RunDrainSpecs declares every drain Run registers except the polling
	// updater stop; updaterStopSpec declares that one. A new drain added to
	// either list is covered here automatically.
	specs := append(RunDrainSpecs(
		func() error { return nil },
		func() error { return nil },
		func() error { return nil },
		[]DrainSpec{{Name: "users drain", Handler: func() error { return nil }}},
		nil,
		nil,
	), updaterStopSpec(nil))

	phases := shutdown.Phases()
	seen := make(map[shutdown.Phase]bool)
	for _, spec := range specs {
		if !slices.Contains(phases, spec.Phase) {
			t.Fatalf("drain %q declares phase %v, which Phases() does not list", spec.Name, spec.Phase)
		}
		seen[spec.Phase] = true
	}
	for _, phase := range phases {
		if !seen[phase] {
			t.Fatalf("no drain declares phase %v", phase)
		}
	}
}

// No drain registered after the database phase may write to the database: the
// database is already closed when they run. This registers the same drains
// Run registers — enumerated from RunDrainSpecs and updaterStopSpec, so a new
// drain is covered automatically — against a real SQLite database. A write
// probe runs in front of and behind every drain: each probe before the close
// must land, each probe after it must fail, and a drain that writes after the
// close makes its trailing probe report an error.
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
		case !closed:
			if err != nil {
				probeErrors = append(probeErrors, err) // write failed while the DB should be open
			} else {
				writesBeforeClose++
			}
		case err == nil:
			probeErrors = append(probeErrors, errors.New("database write landed after the database phase"))
		}
	}

	// wrap puts the probe in front of a drain, so the probe observes the
	// database state the drain itself would see; a drain that writes after
	// the close is caught by the next drain's probe.
	wrap := func(spec DrainSpec) DrainSpec {
		handler := spec.Handler
		spec.Handler = func() error {
			probe()
			return handler()
		}
		return spec
	}

	moduleDrains := []DrainSpec{
		{Name: "users drain", Handler: func() error { return nil }},
		{Name: "captcha stop", Handler: func() error { return nil }},
		{Name: "aispam drain", Handler: func() error { return nil }},
	}
	specs := RunDrainSpecs(
		func() error {
			err := closeDBConnections()
			mu.Lock()
			closed = true
			mu.Unlock()
			return err
		},
		func() error { return nil }, // stats flush
		func() error { return nil }, // jobs stop
		moduleDrains,
		nil,
		func(*httpserver.Server) error { return nil },
	)
	// Polling mode adds the updater stop in the update-source phase; including
	// it keeps the full drain set under the probes. The tracer drain is
	// replaced by a no-op: tracing.Shutdown is not what this test measures.
	updater := ext.NewUpdater(ext.NewDispatcher(nil), nil)
	specs = append(specs[:len(specs)-1], updaterStopSpec(updater), DrainSpec{
		Name: "tracer", Phase: shutdown.PhaseTelemetry, Handler: func() error { return nil },
	})
	for i := range specs {
		specs[i] = wrap(specs[i])
	}

	manager := shutdown.NewManager()
	RegisterShutdownSteps(manager, specs...)

	manager.ExecuteHandlersForTest()

	mu.Lock()
	defer mu.Unlock()
	// Every drain but the telemetry-phase tracer runs while the database is
	// open — module drains included — so each of their leading probes must
	// have landed. The count derives from the spec list, not a literal.
	if writesBeforeClose != len(specs)-1 {
		t.Fatalf("writes before the database phase = %d, want %d", writesBeforeClose, len(specs)-1)
	}
	for _, err := range probeErrors {
		t.Errorf("probe: %v", err)
	}
}
