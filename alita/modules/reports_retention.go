package modules

import (
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/reports"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
)

// reportRetention is how long a report is kept after it was closed, or after
// it was made while it stays open. Its card, with the copy of the message,
// stays in the log channel.
const reportRetention = 90 * 24 * time.Hour

var reportRetentionOnce sync.Once

// sweepExpiredReports deletes the reports closed, or made and still open,
// more than reportRetention before now. Every replica sweeps; the delete is
// idempotent.
func sweepExpiredReports(now time.Time) {
	deleted, err := reports.DeleteExpiredBefore(now.Add(-reportRetention))
	if err != nil {
		log.Errorf("[Reports] retention sweep failed: %v", err)
		return
	}
	if deleted > 0 {
		log.Infof("[Reports] deleted %d reports older than %v", deleted, reportRetention)
	}
}

// startReportRetentionSweeper sweeps expired reports hourly.
func startReportRetentionSweeper() {
	reportRetentionOnce.Do(func() {
		go func() {
			defer error_handling.RecoverFromPanic("reportRetention", "reports")
			ticker := time.NewTicker(time.Hour)
			defer ticker.Stop()
			for range ticker.C {
				func() {
					defer error_handling.RecoverFromPanic("reportRetentionTick", "reports")
					sweepExpiredReports(time.Now())
				}()
			}
		}()
	})
}
