package modules

import (
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/bans"
)

// banRecordRetention is how long a ban record is kept after the ban was
// lifted or a temporary ban ended. Open bans are kept while they last.
const banRecordRetention = 90 * 24 * time.Hour

var banRecordRetentionOnce sync.Once

// sweepExpiredBanRecords deletes the records lifted, or of temporary bans
// ended, more than banRecordRetention before now. Every replica sweeps; the
// delete is idempotent.
func sweepExpiredBanRecords(now time.Time) {
	deleted, err := bans.DeleteExpiredBefore(now.Add(-banRecordRetention))
	if err != nil {
		log.Errorf("[Bans] retention sweep failed: %v", err)
		return
	}
	if deleted > 0 {
		log.Infof("[Bans] deleted %d ban records older than %v", deleted, banRecordRetention)
	}
}

// startBanRecordRetentionSweeper sweeps expired ban records hourly. LoadBans
// calls it once per process.
func startBanRecordRetentionSweeper() {
	startRetentionSweeper(&banRecordRetentionOnce, "banRecordRetention", time.Hour, sweepExpiredBanRecords)
}
