package jobs

import (
	"context"
	"sync"
	"time"
)

// std is the process's registry: packages register their jobs on it, main
// starts and stops it.
var std = &registry{}

// Register adds the periodic job name, which runs tick through Run every
// interval once Start is called; a job registered after Start begins at once.
// Registering a name again does nothing, and registering starts nothing, so
// package init and repeated Load calls may register freely. name must come
// from a fixed, bounded set (see Run).
//
// A tick never overlaps the previous one of the same job: ticks missed while
// one runs are dropped, as with time.Ticker.
func Register(name string, interval time.Duration, tick func(context.Context) error) {
	std.register(name, interval, tick)
}

// Start begins every registered job. Calling it again does nothing.
func Start() { std.start() }

// Stop cancels the context every tick receives, waits for the ticks in flight
// and lets no tick begin afterwards. It is idempotent and safe without Start.
func Stop() { std.stop() }

type job struct {
	name     string
	interval time.Duration
	tick     func(context.Context) error
}

type registry struct {
	mu      sync.Mutex
	jobs    map[string]job
	ctx     context.Context
	cancel  context.CancelFunc
	stopped bool
	wg      sync.WaitGroup
}

func (r *registry) register(name string, interval time.Duration, tick func(context.Context) error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.jobs[name]; ok {
		return
	}
	if r.jobs == nil {
		r.jobs = map[string]job{}
	}
	j := job{name: name, interval: interval, tick: tick}
	r.jobs[name] = j
	if r.ctx != nil && !r.stopped {
		r.launch(j)
	}
}

func (r *registry) start() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.ctx != nil || r.stopped {
		return
	}
	r.ctx, r.cancel = context.WithCancel(context.Background())
	for _, j := range r.jobs {
		r.launch(j)
	}
}

func (r *registry) stop() {
	r.mu.Lock()
	r.stopped = true
	if r.cancel != nil {
		r.cancel()
	}
	r.mu.Unlock()
	r.wg.Wait()
}

// launch runs j's ticker loop; r.mu must be held.
func (r *registry) launch(j job) {
	ctx := r.ctx
	r.wg.Go(func() {
		ticker := time.NewTicker(j.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Both cases may be ready at once; never begin a tick after Stop.
				if ctx.Err() != nil {
					return
				}
				Run(ctx, j.name, j.tick)
			}
		}
	})
}
