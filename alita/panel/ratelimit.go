package panel

import (
	"sync"
	"time"
)

// rateLimit is the per-user budget the ticket fixes at 60 requests per minute.
// The key is the user alone, never the URL chat id: switching chat ids cannot
// buy extra requests or extra Telegram calls. It is in-process: each replica
// enforces its own window, which is enough for a single-bot deployment.
const (
	rateLimitPerWindow = 60
	rateLimitWindow    = time.Minute
	// rateLimitMaxUsers bounds the map. One user is one entry whatever chats
	// they open; reaching the cap clears the window state, which at most grants
	// one extra burst and never grows the map past the cap. Clearing is O(n) but
	// happens once per rateLimitMaxUsers new users, never on every request.
	rateLimitMaxUsers = 4096
)

type rateLimitEntry struct {
	windowStart time.Time
	count       int
}

type rateLimiter struct {
	mu      sync.Mutex
	now     func() time.Time
	entries map[int64]rateLimitEntry
}

func newRateLimiter(now func() time.Time) *rateLimiter {
	return &rateLimiter{now: now, entries: make(map[int64]rateLimitEntry)}
}

// allow records a request from userID and returns how long the caller must wait
// when the window is exhausted; zero means the request may proceed.
func (l *rateLimiter) allow(userID int64) time.Duration {
	now := l.now()

	l.mu.Lock()
	defer l.mu.Unlock()

	if _, ok := l.entries[userID]; !ok && len(l.entries) >= rateLimitMaxUsers {
		clear(l.entries)
	}

	entry, ok := l.entries[userID]
	if !ok || now.Sub(entry.windowStart) >= rateLimitWindow {
		entry = rateLimitEntry{windowStart: now}
	}
	if entry.count >= rateLimitPerWindow {
		retryAfter := entry.windowStart.Add(rateLimitWindow).Sub(now)
		if retryAfter < time.Second {
			retryAfter = time.Second
		}
		l.entries[userID] = entry
		return retryAfter
	}
	entry.count++
	l.entries[userID] = entry
	return 0
}
