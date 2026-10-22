package modules

import (
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/chatstats"
	"github.com/divkix/Alita_Robot/alita/utils/stats"
)

// activityStatsRetentionDays is how long the panel's finer-grained counters
// are kept: the panel's longest window is 90 days, and these tables hold up to
// 24 rows (hourly) or one row per active member (per member) per chat and day,
// so they are kept far shorter than the 400 days of chat_daily_stats.
const activityStatsRetentionDays = 90

// activityStatsTable is one table the activity retention sweep trims.
type activityStatsTable struct {
	name          string
	retentionDays int
	deleteBefore  func(day string) (int64, error)
}

var (
	activityStatsTables = []activityStatsTable{
		{name: "chat_hourly_stats", retentionDays: activityStatsRetentionDays, deleteBefore: chatstats.DeleteHourlyBefore},
		{name: "chat_member_daily_stats", retentionDays: activityStatsRetentionDays, deleteBefore: chatstats.DeleteMemberDailyBefore},
	}
	activityStatsRetentionOnce sync.Once
)

// sweepExpiredActivityStats deletes the rows of each table older than its
// window. One table failing does not stop the others.
func sweepExpiredActivityStats(now time.Time) {
	for _, table := range activityStatsTables {
		cutoff := stats.Day(now.AddDate(0, 0, -table.retentionDays))
		deleted, err := table.deleteBefore(cutoff)
		if err != nil {
			log.Errorf("[Stats] %s retention sweep failed: %v", table.name, err)
			continue
		}
		if deleted > 0 {
			log.Infof("[Stats] deleted %d %s rows older than %d days", deleted, table.name, table.retentionDays)
		}
	}
}

// startActivityStatsRetention sweeps expired rows daily; every replica sweeps
// and the delete is idempotent.
func startActivityStatsRetention() {
	startRetentionSweeper(&activityStatsRetentionOnce, "activityStatsRetention", 24*time.Hour, sweepExpiredActivityStats)
}
