package modules

import (
	"sync"
	"time"

	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
)

// startRetentionSweeper runs sweep every interval in one background goroutine,
// started at most once per once. Every replica sweeps, so sweep must be an
// idempotent delete; a dropped tick on exit is harmless.
func startRetentionSweeper(once *sync.Once, name string, interval time.Duration, sweep func(now time.Time)) {
	once.Do(func() {
		go func() {
			defer error_handling.RecoverFromPanic(name, "retention")
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for range ticker.C {
				func() {
					defer error_handling.RecoverFromPanic(name+"Tick", "retention")
					sweep(time.Now())
				}()
			}
		}()
	})
}
