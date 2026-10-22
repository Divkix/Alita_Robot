package panel

import (
	"sync"
	"time"
)

// rateLimit is the per-(user, chat) budget the ticket fixes at 60 requests per
// minute. It is in-process: each replica enforces its own window, which is
// enough for a single-bot deployment.
const (
	rateLimitPerWindow = 60
	rateLimitWindow    = time.Minute
	// rateLimitSweepThreshold bounds the map's growth between lazy sweeps.
	rateLimitSweepThreshold = 4096
)

type rateLimitKey struct {
	userID int64
	chatID int64
}

type rateLimitEntry struct {
	windowStart time.Time
	count       int
}

type rateLimiter struct {
	mu      sync.Mutex
	now     func() time.Time
	entries map[rateLimitKey]rateLimitEntry
}

func newRateLimiter(now func() time.Time) *rateLimiter {
	return &rateLimiter{now: now, entries: make(map[rateLimitKey]rateLimitEntry)}
}

// allow records a request and returns how long the caller must wait when the
// window is exhausted; zero means the request may proceed.
func (l *rateLimiter) allow(userID, chatID int64) time.Duration {
	now := l.now()
	key := rateLimitKey{userID: userID, chatID: chatID}

	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.entries[key]
	if !ok || now.Sub(entry.windowStart) >= rateLimitWindow {
		entry = rateLimitEntry{windowStart: now}
	}
	if entry.count >= rateLimitPerWindow {
		retryAfter := entry.windowStart.Add(rateLimitWindow).Sub(now)
		if retryAfter < time.Second {
			retryAfter = time.Second
		}
		l.entries[key] = entry
		return retryAfter
	}
	entry.count++
	l.entries[key] = entry

	if len(l.entries) > rateLimitSweepThreshold {
		for k, e := range l.entries {
			if now.Sub(e.windowStart) >= rateLimitWindow {
				delete(l.entries, k)
			}
		}
	}
	return 0
}
