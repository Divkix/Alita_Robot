package modules

import (
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/moderation"
)

// moderationEventRetention is how long a moderation event is kept. It feeds
// the shared reputation window and the dashboard timeline, so it outlives the
// longest configured window; after it, only the retention job deletes rows.
const moderationEventRetention = 180 * 24 * time.Hour

var moderationEventRetentionOnce sync.Once

// sweepExpiredModerationEvents deletes the events created more than
// moderationEventRetention before now. Every replica sweeps; the delete is
// idempotent.
func sweepExpiredModerationEvents(now time.Time) {
	deleted, err := moderation.DeleteExpiredBefore(now.Add(-moderationEventRetention))
	if err != nil {
		log.Errorf("[Moderation] retention sweep failed: %v", err)
		return
	}
	if deleted > 0 {
		log.Infof("[Moderation] deleted %d events older than %v", deleted, moderationEventRetention)
	}
}

// startModerationEventRetentionSweeper sweeps expired moderation events
// hourly. LoadBans calls it once per process.
func startModerationEventRetentionSweeper() {
	startRetentionSweeper(&moderationEventRetentionOnce, "moderationEventRetention", time.Hour, sweepExpiredModerationEvents)
}
