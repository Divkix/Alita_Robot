//go:build testtools

package ratelimit

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

// fakeClock is a clock whose sleeps advance it instantly.
type fakeClock struct {
	mu    sync.Mutex
	now   time.Time
	slept time.Duration
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Sleep(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
	c.slept += d
	return nil
}

func (c *fakeClock) Elapsed() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.slept
}

func newTestSendLimiter(rate, burst int) (*SendLimiter, *fakeClock) {
	clock := &fakeClock{now: time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)}
	l := NewSendLimiter(rate, burst)
	l.now = clock.Now
	l.sleep = clock.Sleep
	return l, clock
}

func floodError(retryAfter int64) error {
	return &gotgbot.TelegramError{
		Method:         "sendMessage",
		Code:           http.StatusTooManyRequests,
		Description:    "Too Many Requests: retry after",
		ResponseParams: &gotgbot.ResponseParameters{RetryAfter: retryAfter},
	}
}

// After the burst, sends go out no faster than the rate: 25 sends past a
// burst of 5 at 25/s take 20 sends' worth of spacing, 0.8s.
func TestSendLimiterPacesSendsAtTheRate(t *testing.T) {
	l, clock := newTestSendLimiter(25, 5)
	for i := 0; i < 25; i++ {
		if err := l.Wait(context.Background()); err != nil {
			t.Fatalf("Wait() #%d error = %v", i, err)
		}
	}
	if got, want := clock.Elapsed(), 800*time.Millisecond; got < want-time.Millisecond || got > want+10*time.Millisecond {
		t.Fatalf("25 sends took %v, want about %v", got, want)
	}
}

// The burst goes out at once after the limiter has been idle.
func TestSendLimiterLetsTheBurstThroughWithoutWaiting(t *testing.T) {
	l, clock := newTestSendLimiter(25, 5)
	for i := 0; i < 5; i++ {
		if err := l.Wait(context.Background()); err != nil {
			t.Fatalf("Wait() error = %v", err)
		}
	}
	if got := clock.Elapsed(); got != 0 {
		t.Fatalf("burst waited %v, want no wait", got)
	}
}

// A 429 pauses the limiter for retry_after and the send runs once more.
func TestSendLimiterDoRetriesAfterRetryAfter(t *testing.T) {
	l, clock := newTestSendLimiter(25, 5)
	calls := 0
	err := l.Do(context.Background(), func() error {
		calls++
		if calls == 1 {
			return floodError(3)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Do() error = %v, want the retry to succeed", err)
	}
	if calls != 2 {
		t.Fatalf("send ran %d times, want 2", calls)
	}
	if got := clock.Elapsed(); got < 3*time.Second {
		t.Fatalf("retry came after %v, want at least retry_after 3s", got)
	}

	// A pause holds every sender, not only the one that got the 429.
	l.Pause(2 * time.Second)
	before := clock.Elapsed()
	if err := l.Wait(context.Background()); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
	if got := clock.Elapsed() - before; got < 2*time.Second {
		t.Fatalf("send during a pause waited %v, want at least 2s", got)
	}
}

// A flood wait longer than MaxSendRetryAfter pauses the limiter but returns
// Telegram's error at once instead of retrying.
func TestSendLimiterDoGivesUpOnALongFloodWait(t *testing.T) {
	l, clock := newTestSendLimiter(25, 5)
	calls := 0
	want := floodError(int64((MaxSendRetryAfter + time.Second) / time.Second))
	err := l.Do(context.Background(), func() error {
		calls++
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("Do() error = %v, want the 429", err)
	}
	if calls != 1 {
		t.Fatalf("send ran %d times, want no retry", calls)
	}
	if got := clock.Elapsed(); got != 0 {
		t.Fatalf("Do() waited %v before giving up, want none", got)
	}
	if err := l.Wait(context.Background()); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
	if got := clock.Elapsed(); got < MaxSendRetryAfter+time.Second {
		t.Fatalf("next send waited %v, want the pause of %v honoured", got, MaxSendRetryAfter+time.Second)
	}
}

// While a pause longer than MaxSendRetryAfter runs, Do does not wait it out
// and does not send.
func TestSendLimiterDoRefusesDuringALongPause(t *testing.T) {
	l, clock := newTestSendLimiter(25, 5)
	l.Pause(MaxSendRetryAfter + time.Second)
	ran := false
	if err := l.Do(context.Background(), func() error { ran = true; return nil }); !errors.Is(err, ErrSendPaused) {
		t.Fatalf("Do() error = %v, want ErrSendPaused", err)
	}
	if ran {
		t.Fatal("send ran during a long pause")
	}
	if got := clock.Elapsed(); got != 0 {
		t.Fatalf("Do() waited %v, want none", got)
	}
	if got := l.PausedFor(); got != MaxSendRetryAfter+time.Second {
		t.Fatalf("PausedFor() = %v, want %v", got, MaxSendRetryAfter+time.Second)
	}
}

// A 429 on the retry pauses the limiter too, so the next sender does not run
// straight into the same flood wait.
func TestSendLimiterDoPausesOnASecond429(t *testing.T) {
	l, clock := newTestSendLimiter(25, 5)
	calls := 0
	err := l.Do(context.Background(), func() error {
		calls++
		return floodError(5)
	})
	if _, flooded := RetryAfter(err); !flooded {
		t.Fatalf("Do() error = %v, want the second 429", err)
	}
	if calls != 2 {
		t.Fatalf("send ran %d times, want 2", calls)
	}
	before := clock.Elapsed()
	if err := l.Wait(context.Background()); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
	if got := clock.Elapsed() - before; got < 5*time.Second {
		t.Fatalf("next send waited %v, want the second retry_after of 5s", got)
	}
}

// The bucket starts empty after a pause: sends resume at the rate, not with a
// burst right after Telegram asked the bot to slow down.
func TestSendLimiterSendsNoBurstAfterAPause(t *testing.T) {
	l, clock := newTestSendLimiter(25, 5)
	l.Pause(10 * time.Second)
	for i := 0; i < 5; i++ {
		if err := l.Wait(context.Background()); err != nil {
			t.Fatalf("Wait() error = %v", err)
		}
	}
	// 10s of pause, then five sends at 25/s: 40ms each.
	if got, want := clock.Elapsed(), 10*time.Second+200*time.Millisecond; got < want-time.Millisecond {
		t.Fatalf("five sends after the pause took %v, want at least %v", got, want)
	}
}

// Concurrent senders share the bucket: none gets a token the rate does not
// allow.
func TestSendLimiterIsSafeForConcurrentSenders(t *testing.T) {
	l, clock := newTestSendLimiter(25, 5)
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := l.Wait(context.Background()); err != nil {
				t.Errorf("Wait() error = %v", err)
			}
		}()
	}
	wg.Wait()
	// The fake clock adds every sleep, so concurrent sleeps sum: the bound
	// that matters is the lower one, 25 sends past the burst at 25/s.
	if got := clock.Elapsed(); got < time.Second-time.Millisecond {
		t.Fatalf("30 concurrent sends took %v, want at least 1s", got)
	}
}

// Errors other than 429 are returned as they are, without a retry.
func TestSendLimiterDoReturnsOtherErrors(t *testing.T) {
	l, _ := newTestSendLimiter(25, 5)
	calls := 0
	want := errors.New("Forbidden: bot was kicked from the group chat")
	if err := l.Do(context.Background(), func() error { calls++; return want }); !errors.Is(err, want) {
		t.Fatalf("Do() error = %v, want %v", err, want)
	}
	if calls != 1 {
		t.Fatalf("send ran %d times, want 1", calls)
	}
}

// A cancelled context stops a waiting send without running it.
func TestSendLimiterStopsWaitingWhenTheContextEnds(t *testing.T) {
	l := NewSendLimiter(1, 1)
	l.Pause(30 * time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ran := false
	if err := l.Do(ctx, func() error { ran = true; return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("Do() error = %v, want context.Canceled", err)
	}
	if ran {
		t.Fatal("send ran after the context ended")
	}

	ctx, cancel = context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err := l.Wait(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Wait() error = %v, want context.DeadlineExceeded", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("Wait() returned after %v, want soon after the deadline", elapsed)
	}
}

func TestRetryAfter(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		want    time.Duration
		flooded bool
	}{
		{"nil", nil, 0, false},
		{"other error", errors.New("boom"), 0, false},
		{"400", &gotgbot.TelegramError{Code: http.StatusBadRequest}, 0, false},
		{"429 with retry_after", floodError(7), 7 * time.Second, true},
		{"429 without parameters", &gotgbot.TelegramError{Code: http.StatusTooManyRequests}, time.Second, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, flooded := RetryAfter(tc.err)
			if got != tc.want || flooded != tc.flooded {
				t.Fatalf("RetryAfter() = %v, %v; want %v, %v", got, flooded, tc.want, tc.flooded)
			}
		})
	}
}
