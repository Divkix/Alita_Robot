package window_test

import (
	"math"
	"testing"

	"github.com/divkix/Alita_Robot/alita/utils/punishment/window"
)

func TestTemporaryUntilDate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		now      int64
		duration int64
		want     int64
		wantOK   bool
	}{
		{name: "minimum", now: 100, duration: 30, want: 130, wantOK: true},
		{name: "below minimum", now: 100, duration: 29},
		{name: "maximum", now: 100, duration: 366 * 24 * 60 * 60, want: 100 + 366*24*60*60, wantOK: true},
		{name: "above maximum", now: 100, duration: 366*24*60*60 + 1},
		{name: "addition overflow", now: math.MaxInt64 - 29, duration: 30},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := window.UntilDate(tc.now, tc.duration)
			if got != tc.want || ok != tc.wantOK {
				t.Fatalf("TemporaryUntilDate(%d, %d) = (%d, %v), want (%d, %v)", tc.now, tc.duration, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}
