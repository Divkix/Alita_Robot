package shutdown

import "testing"

func TestPhaseString(t *testing.T) {
	cases := []struct {
		phase Phase
		want  string
	}{
		{PhaseUpdateSource, "update-source"},
		{PhaseHTTPServer, "http-server"},
		{PhaseJobs, "jobs"},
		{PhaseModuleDrains, "module-drains"},
		{PhaseFlush, "flush"},
		{PhaseDatabase, "database"},
		{PhaseTelemetry, "telemetry"},
		{Phase(99), "phase(99)"},
	}
	for _, tc := range cases {
		if got := tc.phase.String(); got != tc.want {
			t.Errorf("Phase(%d).String() = %q, want %q", tc.phase, got, tc.want)
		}
	}
}
