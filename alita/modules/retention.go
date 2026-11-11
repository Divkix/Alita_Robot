package modules

import (
	"context"
	"time"

	"github.com/divkix/Alita_Robot/alita/utils/jobs"
)

// registerRetentionSweep registers sweep as the job name, run every interval
// once jobs start; registering again does nothing. Every replica sweeps, so
// sweep must be an idempotent delete; a dropped tick on exit is harmless.
func registerRetentionSweep(name string, interval time.Duration, sweep func(now time.Time)) {
	jobs.Register(name, interval, func(context.Context) error {
		sweep(time.Now())
		return nil
	})
}
