package modules

import (
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/bans"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

// banRecordRetention is how long a ban record is kept after the ban was
// lifted or a temporary ban ended. Open bans are kept while they last.
const banRecordRetention = 90 * 24 * time.Hour

var banRecordRetentionOnce sync.Once

// Telegram treats a ban that ends less than 30 seconds or more than 366 days
// from now as permanent.
const (
	minTemporaryBan = 30 * time.Second
	maxTemporaryBan = 366 * 24 * time.Hour
)

// banEnd is the end of a ban made at now with Telegram until_date untilUnix:
// nil for a permanent ban, as Telegram reads 0 and out-of-range dates.
func banEnd(untilUnix int64, now time.Time) *time.Time {
	if untilUnix <= 0 {
		return nil
	}
	until := time.Unix(untilUnix, 0).UTC()
	if d := until.Sub(now); d < minTemporaryBan || d > maxTemporaryBan {
		return nil
	}
	return &until
}

// banRecord is a ban the bot just issued, for recordBan.
type banRecord struct {
	chatID   int64
	userID   int64
	bannedBy int64 // 0 when the bot banned on its own
	source   string
	detail   string
	reason   string
	// untilUnix is the Telegram until_date the ban was made with, 0 for a
	// permanent ban.
	untilUnix int64
}

// recordBan stores a ban Telegram has just accepted and returns its record's
// ID, or 0 when the write failed. A failed write is logged and changes
// nothing else: the ban already happened, so it is never undone, refused or
// answered differently over its record.
func recordBan(ban banRecord) int64 {
	now := time.Now()
	id, err := bans.Record(models.BanRecord{
		ChatId:       ban.chatID,
		UserId:       ban.userID,
		BannedBy:     ban.bannedBy,
		Reason:       ban.reason,
		Source:       ban.source,
		SourceDetail: ban.detail,
		CreatedAt:    now,
		Until:        banEnd(ban.untilUnix, now),
	})
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"chat_id": ban.chatID,
			"user_id": ban.userID,
			"source":  ban.source,
		}).Error("[Bans] Failed to record the ban")
		return 0
	}
	return id
}

// liftBanRecord closes the record of a ban Telegram has just lifted, as
// liftedBy (0 for the bot). Like recordBan, a failed write is only logged.
func liftBanRecord(chatID, userID, liftedBy int64) {
	if _, err := bans.Lift(chatID, userID, liftedBy); err != nil {
		log.WithError(err).WithFields(log.Fields{
			"chat_id": chatID,
			"user_id": userID,
		}).Error("[Bans] Failed to close the ban record")
	}
}

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
