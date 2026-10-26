package ratelimit

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

// Telegram allows a bot about 30 messages a second overall
// (https://core.telegram.org/bots/faq). The background fan-outs stay under it
// with room for the replies the bot sends to commands at the same time.
const (
	// DefaultSendRate is how many background sends a second the global
	// limiter lets through.
	DefaultSendRate = 25
	// DefaultSendBurst is how many sends may go out back to back after the
	// limiter has been idle.
	DefaultSendBurst = 5
	// MaxSendRetryAfter caps how long Do waits on a flood wait: the
	// retry_after of its own 429, or a pause already running when it is
	// called. A longer flood wait still pauses the limiter, but Do returns
	// instead of holding its caller that long.
	MaxSendRetryAfter = time.Minute
)

// ErrSendPaused is what Do returns, without sending, while the limiter is
// paused for longer than MaxSendRetryAfter.
var ErrSendPaused = errors.New("ratelimit: sends are paused by a Telegram flood wait")

// SendLimiter is a token bucket for messages the bot sends on its own (the
// weekly digest, recurring messages), not for replies to updates.
//
// It is in-process: each replica has its own bucket, so N replicas sending at
// the same moment can reach N times the rate. Telegram counts per bot token,
// not per process; operators who run several replicas and large fan-outs
// should keep that in mind. A 429 pauses the whole bucket for retry_after,
// because Telegram does not say whether a flood wait is for one chat or for
// the bot. Concurrent waiters are not served in order.
type SendLimiter struct {
	mu          sync.Mutex
	rate        float64
	burst       float64
	tokens      float64
	last        time.Time
	pausedUntil time.Time

	// now and sleep are the clock; tests replace them.
	now   func() time.Time
	sleep func(context.Context, time.Duration) error
}

// NewSendLimiter returns a limiter that lets rate sends a second through after
// an initial burst. A rate or burst below 1 is raised to 1.
func NewSendLimiter(rate, burst int) *SendLimiter {
	if rate < 1 {
		rate = 1
	}
	if burst < 1 {
		burst = 1
	}
	return &SendLimiter{
		rate:   float64(rate),
		burst:  float64(burst),
		tokens: float64(burst),
		now:    time.Now,
		sleep:  sleepContext,
	}
}

var globalSendLimiter = NewSendLimiter(DefaultSendRate, DefaultSendBurst)

// GlobalSendLimiter returns the process-wide limiter every background
// fan-out shares. Route new scheduled sends through its Do method, not a
// separate limiter; the bucket is per replica and is not shared through Redis.
func GlobalSendLimiter() *SendLimiter {
	return globalSendLimiter
}

// Wait blocks until the limiter lets one send through or ctx ends, in which
// case it returns ctx's error and takes no token.
func (l *SendLimiter) Wait(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		wait := l.reserve()
		if wait <= 0 {
			return nil
		}
		if err := l.sleep(ctx, wait); err != nil {
			return err
		}
	}
}

// reserve takes a token and returns 0, or returns how long to wait before
// asking again.
func (l *SendLimiter) reserve() time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	if now.Before(l.pausedUntil) {
		return l.pausedUntil.Sub(now)
	}
	if !l.last.IsZero() {
		l.tokens += now.Sub(l.last).Seconds() * l.rate
		if l.tokens > l.burst {
			l.tokens = l.burst
		}
	}
	l.last = now
	if l.tokens >= 1 {
		l.tokens--
		return 0
	}
	wait := time.Duration((1 - l.tokens) / l.rate * float64(time.Second))
	if wait <= 0 {
		wait = time.Millisecond
	}
	return wait
}

// Pause lets nothing through for d, and the bucket starts empty when the
// pause ends, so no burst follows a flood wait. A shorter pause never shortens
// a longer one already running.
func (l *SendLimiter) Pause(d time.Duration) {
	if d <= 0 {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	until := l.now().Add(d)
	if until.After(l.pausedUntil) {
		l.pausedUntil = until
		l.tokens = 0
		l.last = until
	}
}

// PausedFor returns how long the current flood-wait pause still runs, or 0.
// A fan-out checks it before claiming work it could not send in time.
// End the tick when it exceeds MaxSendRetryAfter, before taking another claim.
func (l *SendLimiter) PausedFor() time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	if left := l.pausedUntil.Sub(l.now()); left > 0 {
		return left
	}
	return 0
}

// Do waits for a token and runs send. When Telegram answers 429 it pauses the
// limiter for the retry_after it asks for and, when that is at most
// MaxSendRetryAfter, waits and runs send once more. It returns send's last
// error, ErrSendPaused when a pause longer than MaxSendRetryAfter is already
// running, or ctx's error when ctx ends while it waits.
func (l *SendLimiter) Do(ctx context.Context, send func() error) error {
	if l.PausedFor() > MaxSendRetryAfter {
		return ErrSendPaused
	}
	if err := l.Wait(ctx); err != nil {
		return err
	}
	err := l.sendAndPause(send)
	retryAfter, flooded := RetryAfter(err)
	if !flooded || retryAfter > MaxSendRetryAfter {
		return err
	}
	if waitErr := l.Wait(ctx); waitErr != nil {
		return waitErr
	}
	return l.sendAndPause(send)
}

// sendAndPause runs send and pauses the limiter for the retry_after of a 429.
func (l *SendLimiter) sendAndPause(send func() error) error {
	err := send()
	if retryAfter, flooded := RetryAfter(err); flooded {
		l.Pause(retryAfter)
	}
	return err
}

// RetryAfter reports whether err is Telegram's 429 and the wait it asked for.
// A 429 without retry_after asks for one second.
func RetryAfter(err error) (time.Duration, bool) {
	var tgErr *gotgbot.TelegramError
	if !errors.As(err, &tgErr) || tgErr.Code != http.StatusTooManyRequests {
		return 0, false
	}
	if tgErr.ResponseParams == nil || tgErr.ResponseParams.RetryAfter <= 0 {
		return time.Second, true
	}
	return time.Duration(tgErr.ResponseParams.RetryAfter) * time.Second, true
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
