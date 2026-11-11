//go:build testtools

package jobs

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

// counter returns a tick that counts its calls.
func counter(n *atomic.Int32) func(context.Context) error {
	return func(context.Context) error {
		n.Add(1)
		return nil
	}
}

// after advances the bubble's fake clock by d and lets every job settle.
func after(d time.Duration) {
	time.Sleep(d)
	synctest.Wait()
}

func TestJobTicksOncePerIntervalAndNotBeforeTheFirst(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := &registry{}
		defer r.stop()
		var ticks atomic.Int32
		r.register("probe", time.Minute, counter(&ticks))
		r.start()

		after(time.Minute - time.Second)
		if got := ticks.Load(); got != 0 {
			t.Fatalf("ticks before the first interval = %d, want 0", got)
		}
		after(time.Second)
		if got := ticks.Load(); got != 1 {
			t.Fatalf("ticks after one interval = %d, want 1", got)
		}
		after(2 * time.Minute)
		if got := ticks.Load(); got != 3 {
			t.Fatalf("ticks after three intervals = %d, want 3", got)
		}
	})
}

func TestTickAfterAnErrorAndAPanic(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := &registry{}
		defer r.stop()
		var ticks atomic.Int32
		r.register("flaky", time.Minute, func(context.Context) error {
			switch ticks.Add(1) {
			case 1:
				return errors.New("boom")
			case 2:
				panic("bad tick")
			}
			return nil
		})
		r.start()

		after(3 * time.Minute)
		if got := ticks.Load(); got != 3 {
			t.Fatalf("ticks = %d, want 3 (error, panic, then a normal tick)", got)
		}
	})
}

func TestLongTickDoesNotOverlapItself(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := &registry{}
		defer r.stop()
		var running, maxRunning, ticks atomic.Int32
		r.register("slow", time.Minute, func(context.Context) error {
			ticks.Add(1)
			if n := running.Add(1); n > maxRunning.Load() {
				maxRunning.Store(n)
			}
			time.Sleep(150 * time.Second)
			running.Add(-1)
			return nil
		})
		r.start()

		after(10*time.Minute - time.Second)
		if got := maxRunning.Load(); got != 1 {
			t.Fatalf("ticks running at once = %d, want 1", got)
		}
		// Ten intervals passed, but a 150 s tick fits four times at most: the
		// ticks missed while one ran were dropped, not queued.
		if got := ticks.Load(); got > 4 {
			t.Fatalf("ticks = %d, want at most 4", got)
		}
	})
}

func TestStopWaitsForTheTickInFlightAndCancelsIt(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := &registry{}
		var ticks atomic.Int32
		var finished, sawCancel atomic.Bool
		r.register("inflight", time.Minute, func(ctx context.Context) error {
			ticks.Add(1)
			<-ctx.Done()
			sawCancel.Store(errors.Is(ctx.Err(), context.Canceled))
			time.Sleep(10 * time.Second) // wind-down stop must wait out
			finished.Store(true)
			return nil
		})
		r.start()
		after(time.Minute)
		if ticks.Load() != 1 {
			t.Fatal("tick not in flight")
		}

		r.stop()
		if !finished.Load() {
			t.Fatal("stop returned before the tick in flight finished")
		}
		if !sawCancel.Load() {
			t.Fatal("the tick in flight did not see its context cancelled")
		}
		after(5 * time.Minute)
		if got := ticks.Load(); got != 1 {
			t.Fatalf("ticks after stop = %d, want 1", got)
		}
	})
}

func TestStopTwiceAndWithoutStart(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := &registry{}
		r.stop()
		r.stop()

		var ticks atomic.Int32
		r.register("late", time.Minute, counter(&ticks))
		r.start()
		after(2 * time.Minute)
		if got := ticks.Load(); got != 0 {
			t.Fatalf("ticks after a stop = %d, want 0", got)
		}
	})
}

func TestStartTwiceRunsEachJobOnce(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := &registry{}
		defer r.stop()
		var ticks atomic.Int32
		r.register("probe", time.Minute, counter(&ticks))
		r.start()
		r.start()

		after(time.Minute)
		if got := ticks.Load(); got != 1 {
			t.Fatalf("ticks = %d, want 1", got)
		}
	})
}

func TestJobRegisteredAfterStartTicks(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := &registry{}
		defer r.stop()
		r.start()
		var ticks atomic.Int32
		r.register("late", time.Minute, counter(&ticks))

		after(time.Minute)
		if got := ticks.Load(); got != 1 {
			t.Fatalf("ticks = %d, want 1", got)
		}
	})
}

func TestNameRegisteredTwiceRunsOnce(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := &registry{}
		defer r.stop()
		var first, second atomic.Int32
		r.register("dup", time.Minute, counter(&first))
		r.register("dup", time.Minute, counter(&second))
		r.start()

		after(time.Minute)
		if first.Load() != 1 || second.Load() != 0 {
			t.Fatalf("ticks = %d and %d, want 1 and 0", first.Load(), second.Load())
		}
	})
}

func TestJobsWithDifferentIntervalsDoNotDelayEachOther(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := &registry{}
		defer r.stop()
		var fast, slow atomic.Int32
		r.register("slow", time.Minute, func(ctx context.Context) error {
			slow.Add(1)
			select {
			case <-ctx.Done():
			case <-time.After(10 * time.Minute):
			}
			return nil
		})
		r.register("fast", 10*time.Second, counter(&fast))
		r.start()

		after(2 * time.Minute)
		if got := fast.Load(); got != 12 {
			t.Fatalf("fast ticks = %d, want 12 while the slow job ran", got)
		}
		if got := slow.Load(); got != 1 {
			t.Fatalf("slow ticks = %d, want 1", got)
		}
	})
}
