package shutdown

import "strconv"

// Phase orders shutdown drains: lower phases run first, so the process stops
// taking in work before it drains it, drains before it closes the database,
// and exports telemetry last. Phases is the total, contiguous list: a drain
// registers the phase it belongs to, and a phase appended to Phases runs last.
type Phase int

const (
	// PhaseUpdateSource stops the feed of new work: the Telegram updater or
	// webhook intake. It runs first so nothing new arrives while draining.
	PhaseUpdateSource Phase = iota
	// PhaseHTTPServer stops the HTTP server (webhook intake included).
	PhaseHTTPServer
	// PhaseModuleDrains waits for in-flight module work to finish writing.
	PhaseModuleDrains
	// PhaseJobs stops the periodic jobs registry.
	PhaseJobs
	// PhaseFlush makes one last flush of buffered counters before the close.
	PhaseFlush
	// PhaseDatabase closes the database. Nothing registered in a later phase
	// may write to it.
	PhaseDatabase
	// PhaseTelemetry shuts the tracing providers down last, after every other
	// drain's spans and logs are exported.
	PhaseTelemetry
)

// Phases lists every phase in execution order.
func Phases() []Phase {
	return []Phase{
		PhaseUpdateSource,
		PhaseHTTPServer,
		PhaseModuleDrains,
		PhaseJobs,
		PhaseFlush,
		PhaseDatabase,
		PhaseTelemetry,
	}
}

// String names the phase for logs; an unknown phase reports its number.
func (p Phase) String() string {
	switch p {
	case PhaseUpdateSource:
		return "update-source"
	case PhaseHTTPServer:
		return "http-server"
	case PhaseModuleDrains:
		return "module-drains"
	case PhaseJobs:
		return "jobs"
	case PhaseFlush:
		return "flush"
	case PhaseDatabase:
		return "database"
	case PhaseTelemetry:
		return "telemetry"
	default:
		return "phase(" + strconv.Itoa(int(p)) + ")"
	}
}
