// Package window holds Telegram's temporary-punishment bounds without database
// dependencies, so stored settings can share them without an import cycle.
package window

import "math"

const (
	MinSeconds = 30
	MaxSeconds = 366 * 24 * 60 * 60
)

// UntilDate refuses durations that Telegram would interpret as permanent.
func UntilDate(now, seconds int64) (int64, bool) {
	if seconds < MinSeconds || seconds > MaxSeconds || now > math.MaxInt64-seconds {
		return 0, false
	}
	return now + seconds, true
}
