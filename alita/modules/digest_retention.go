package modules

import (
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/chatstats"
	"github.com/divkix/Alita_Robot/alita/utils/stats"
)

// digestStatsRetentionDays is how long a day's counters are kept. The digest
// only reads this week and last week; a year of history is enough for any
// later report while keeping the table bounded.
const digestStatsRetentionDays = 400

var digestStatsRetentionOnce sync.Once

// sweepExpiredDailyStats deletes counters older than the retention window.
func sweepExpiredDailyStats(now time.Time) {
	cutoff := stats.Day(now.AddDate(0, 0, -digestStatsRetentionDays))
	deleted, err := chatstats.DeleteBefore(cutoff)
	if err != nil {
		log.Errorf("[Digest] daily stats retention sweep failed: %v", err)
		return
	}
	if deleted > 0 {
		log.Infof("[Digest] deleted %d daily stats rows older than %d days", deleted, digestStatsRetentionDays)
	}
}

// startDigestStatsRetention sweeps expired rows. The window is weeks long, but
// a 7-day ticker would never fire on a bot redeployed more often than weekly,
// so it ticks daily; every replica sweeps and the delete is idempotent.
func startDigestStatsRetention() {
	startRetentionSweeper(&digestStatsRetentionOnce, "digestStatsRetention", 24*time.Hour, sweepExpiredDailyStats)
}
