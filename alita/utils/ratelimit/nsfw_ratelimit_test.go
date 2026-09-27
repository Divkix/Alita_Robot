package ratelimit

import (
	"sync"
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/utils/state"
)

func TestNSFWRateLimiter(t *testing.T) {
	state.SimulateRestart()
	t.Cleanup(state.SimulateRestart)
	r := &NSFWRateLimiter{}
	now := time.Unix(1_700_000_000, 0)

	for i := 0; i < 10; i++ {
		if allowed, _ := r.Acquire(42, false, now); !allowed {
			t.Fatalf("member request %d was denied", i+1)
		}
	}
	if allowed, wait := r.Acquire(42, false, now); allowed || wait != 144*time.Minute {
		t.Fatalf("exhausted member bucket = (%v, %v), want (false, 144m)", allowed, wait)
	}
	if allowed, _ := r.Acquire(42, false, now.Add(144*time.Minute)); !allowed {
		t.Fatal("one member token should refill after 144 minutes")
	}

	for i := 0; i < 100; i++ {
		if allowed, _ := r.Acquire(43, true, now); !allowed {
			t.Fatalf("admin request %d was denied", i+1)
		}
	}
	if allowed, wait := r.Acquire(43, true, now); allowed || wait != 14*time.Minute+24*time.Second {
		t.Fatalf("exhausted admin bucket = (%v, %v), want (false, 14m24s)", allowed, wait)
	}
	if allowed, _ := r.Acquire(43, false, now); !allowed {
		t.Fatal("admin and member buckets should be separate")
	}
}

func TestNSFWRateLimiterConcurrent(t *testing.T) {
	state.SimulateRestart()
	t.Cleanup(state.SimulateRestart)
	r := &NSFWRateLimiter{}
	now := time.Unix(1_700_000_000, 0)
	var wg sync.WaitGroup
	var mu sync.Mutex
	allowedCount := 0
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			allowed, _ := r.Acquire(44, false, now)
			if allowed {
				mu.Lock()
				allowedCount++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if allowedCount != 10 {
		t.Fatalf("concurrent allowed requests = %d, want 10", allowedCount)
	}
}
