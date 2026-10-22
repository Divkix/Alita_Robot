package modules

import (
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/reports"
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
	// An auto-mute no admin decided on outlives its mute only to keep the
	// same reports from muting again; those reports are gone by now.
	if _, err := reports.DeleteAutoMutesBefore(now.Add(-reportRetention)); err != nil {
		log.Errorf("[Reports] auto-mute retention sweep failed: %v", err)
	}
}

// startReportRetentionSweeper sweeps expired reports hourly.
func startReportRetentionSweeper() {
	startRetentionSweeper(&reportRetentionOnce, "reportRetention", time.Hour, sweepExpiredReports)
}
