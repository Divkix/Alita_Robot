package models

// Bounds for timed moderation actions, in seconds. Telegram reads an until_date
// less than 30 s or more than 366 days away as forever; the 60 s floor absorbs
// second rounding and request latency.
const (
	MinActionTime = 60
	MaxActionTime = 366 * 24 * 60 * 60
)

// ValidActionTime reports whether seconds is a usable tban/tmute duration.
func ValidActionTime(seconds int) bool {
	return seconds >= MinActionTime && seconds <= MaxActionTime
}
