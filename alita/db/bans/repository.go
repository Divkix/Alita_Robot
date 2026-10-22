// Package bans keeps the record of the bans the bot issues: one row per ban,
// at most one open per user and chat. Rows are read straight from the
// database, never cached: whether a ban is open changes with every ban and
// unban, and the readers (/baninfo, the unban paths) are rare.
package bans

import (
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

// liftChunk bounds the IDs one LiftUsers statement names.
const liftChunk = 500

// openRow selects the open row of a chat and user: the one the partial
// unique index uq_ban_records_open covers.
var openRow = clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "lifted_at IS NULL"}}}

func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) > n {
		return string([]rune(s)[:n])
	}
	return s
}

// ensureChat makes sure chatID has the chats row the foreign key needs: a
// ban can come before the users tracker creates it. The row almost always
// exists, so it is looked up first: chats.EnsureChatInDb evicts the chat's
// cache entries on every call, and a raid bans many users in a row.
func ensureChat(chatID int64) error {
	var found []int64
	if err := db.DB.Model(&models.Chat{}).Where("chat_id = ?", chatID).Limit(1).Pluck("chat_id", &found).Error; err != nil {
		return err
	}
	if len(found) > 0 {
		return nil
	}
	return chats.EnsureChatInDb(chatID, "")
}

// Record stores a ban the bot issued and returns the ID of its row. A chat
// holds one open row per user, so a repeat ban, or a ban after a temporary
// one expired, updates that row: who, why, the source, the time and the end.
// A missing CreatedAt is stamped now; times are stored in UTC. The reason is
// cut to models.MaxModerationReasonRunes and the detail to
// models.MaxBanSourceDetailRunes.
func Record(r models.BanRecord) (int64, error) {
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now()
	}
	r.CreatedAt = r.CreatedAt.UTC()
	if r.Until != nil {
		until := r.Until.UTC()
		r.Until = &until
	}
	r.ID, r.LiftedAt, r.LiftedBy = 0, nil, nil
	r.Reason = truncateRunes(r.Reason, models.MaxModerationReasonRunes)
	r.SourceDetail = truncateRunes(r.SourceDetail, models.MaxBanSourceDetailRunes)
	if err := ensureChat(r.ChatId); err != nil {
		return 0, fmt.Errorf("record ban: %w", err)
	}
	err := db.DB.Clauses(clause.OnConflict{
		Columns:     []clause.Column{{Name: "chat_id"}, {Name: "user_id"}},
		TargetWhere: openRow,
		DoUpdates: clause.AssignmentColumns([]string{
			"banned_by", "reason", "source", "source_detail", "created_at", "until",
		}),
	}).Create(&r).Error
	if err != nil {
		return 0, fmt.Errorf("record ban: %w", err)
	}
	if r.ID != 0 {
		return r.ID, nil
	}
	// Not every driver returns the ID of a row the upsert updated.
	var ids []int64
	if err := db.DB.Model(&models.BanRecord{}).
		Where("chat_id = ? AND user_id = ? AND lifted_at IS NULL", r.ChatId, r.UserId).
		Limit(1).
		Pluck("id", &ids).Error; err != nil {
		return 0, fmt.Errorf("record ban: read its id: %w", err)
	}
	if len(ids) == 0 {
		return 0, errors.New("record ban: the row is gone")
	}
	return ids[0], nil
}

// inForce narrows q to the rows of bans in force at now: open, and permanent
// or not yet past their end. Bounds are compared in UTC, the zone Record
// writes: SQLite compares the stored text.
func inForce(q *gorm.DB, now time.Time) *gorm.DB {
	return q.Where("lifted_at IS NULL AND (until IS NULL OR until > ?)", now.UTC())
}

// Lift closes the ban of userID in chatID that is in force, as liftedBy (0
// for the bot), and reports whether there was one. A temporary ban past its
// end is left as it is: Telegram lifted it, not liftedBy.
func Lift(chatID, userID, liftedBy int64) (bool, error) {
	now := time.Now().UTC()
	res := inForce(db.DB.Model(&models.BanRecord{}), now).
		Where("chat_id = ? AND user_id = ?", chatID, userID).
		Updates(map[string]any{"lifted_at": now, "lifted_by": liftedBy})
	if res.Error != nil {
		return false, fmt.Errorf("lift ban: %w", res.Error)
	}
	return res.RowsAffected > 0, nil
}

// LiftUsers closes the bans in force of userIDs in chatID, like Lift, and
// returns how many it closed.
func LiftUsers(chatID int64, userIDs []int64, liftedBy int64) (int64, error) {
	var closed int64
	now := time.Now().UTC()
	for start := 0; start < len(userIDs); start += liftChunk {
		end := min(start+liftChunk, len(userIDs))
		res := inForce(db.DB.Model(&models.BanRecord{}), now).
			Where("chat_id = ? AND user_id IN ?", chatID, userIDs[start:end]).
			Updates(map[string]any{"lifted_at": now, "lifted_by": liftedBy})
		if res.Error != nil {
			return closed, fmt.Errorf("lift bans: %w", res.Error)
		}
		closed += res.RowsAffected
	}
	return closed, nil
}

// Latest returns the newest record of userID in chatID, open, expired or
// lifted, or nil when there is none. The open row is always the newest: a
// new row is only made once the previous one was lifted.
func Latest(chatID, userID int64) (*models.BanRecord, error) {
	var r models.BanRecord
	err := db.DB.Where("chat_id = ? AND user_id = ?", chatID, userID).
		Order("id DESC").
		Take(&r).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read ban record: %w", err)
	}
	return &r, nil
}

// DeleteExpiredBefore removes the rows lifted before cutoff and the open
// temporary bans that ended before it, and returns how many it removed. Open
// permanent bans are kept.
func DeleteExpiredBefore(cutoff time.Time) (int64, error) {
	cutoff = cutoff.UTC()
	res := db.DB.
		Where("(lifted_at IS NOT NULL AND lifted_at < ?) OR (lifted_at IS NULL AND until IS NOT NULL AND until < ?)", cutoff, cutoff).
		Delete(&models.BanRecord{})
	if res.Error != nil {
		return 0, fmt.Errorf("delete expired ban records: %w", res.Error)
	}
	return res.RowsAffected, nil
}
