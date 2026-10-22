// Package bans keeps the record of the bans the bot issues: one row per ban,
// at most one open per user and chat. Rows are read straight from the
// database, never cached: whether a ban is open changes with every ban and
// unban, and the readers (/baninfo, the unban paths) are rare.
package bans

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
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
	return q.Where("ban_records.lifted_at IS NULL AND (ban_records.until IS NULL OR ban_records.until > ?)", now.UTC())
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

// GetByID returns the record with id, or nil when there is none (never made,
// or removed by the retention sweep).
func GetByID(id int64) (*models.BanRecord, error) {
	var r models.BanRecord
	err := db.DB.Where("id = ?", id).Take(&r).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read ban record %d: %w", id, err)
	}
	return &r, nil
}

// LiftByID closes the record with id as liftedBy if its ban is in force, and
// reports whether it did. One statement claims the row, so of two callers
// lifting the same ban only one gets true.
func LiftByID(id, liftedBy int64) (bool, error) {
	now := time.Now().UTC()
	res := inForce(db.DB.Model(&models.BanRecord{}), now).
		Where("id = ?", id).
		Updates(map[string]any{"lifted_at": now, "lifted_by": liftedBy})
	if res.Error != nil {
		return false, fmt.Errorf("lift ban record %d: %w", id, res.Error)
	}
	return res.RowsAffected > 0, nil
}

// Reopen undoes a LiftByID by liftedBy whose unban Telegram then refused:
// the ban is still in force.
func Reopen(id, liftedBy int64) error {
	res := db.DB.Model(&models.BanRecord{}).
		Where("id = ? AND lifted_by = ? AND lifted_at IS NOT NULL", id, liftedBy).
		Updates(map[string]any{"lifted_at": nil, "lifted_by": nil})
	if res.Error != nil {
		return fmt.Errorf("reopen ban record %d: %w", id, res.Error)
	}
	return nil
}

// likeEscaper escapes the LIKE wildcards of a search with '!', the ESCAPE
// character ListOpen declares (a backslash reads differently across drivers).
var likeEscaper = strings.NewReplacer("!", "!!", "%", "!%", "_", "!_")

// NormalizeSearch is the form of a ban list search ListOpen matches: trimmed,
// without a leading @. An empty result means no search.
func NormalizeSearch(search string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(search), "@"))
}

// ListOpen returns page (from 0) of the bans in force in chatID at now,
// newest first, size to a page, with the count of every match and the page
// it returned: a page past the end returns the last one. A search
// (NormalizeSearch) matches the banned ID exactly, or part of the username or
// name users holds for it, case-insensitively.
func ListOpen(chatID int64, search string, now time.Time, page, size int) ([]models.BanRecord, int64, int, error) {
	search = NormalizeSearch(search)
	scope := func() *gorm.DB {
		q := inForce(db.DB.Model(&models.BanRecord{}), now).Where("ban_records.chat_id = ?", chatID)
		if search == "" {
			return q
		}
		// Both sides are folded in SQL, so a search folds like the names do.
		pattern := "%" + likeEscaper.Replace(search) + "%"
		match := db.DB.Where("LOWER(users.username) LIKE LOWER(?) ESCAPE '!'", pattern).
			Or("LOWER(users.name) LIKE LOWER(?) ESCAPE '!'", pattern)
		if id, err := strconv.ParseInt(search, 10, 64); err == nil {
			match = match.Or("ban_records.user_id = ?", id)
		}
		return q.Joins("LEFT JOIN users ON users.user_id = ban_records.user_id").Where(match)
	}
	var total int64
	if err := scope().Count(&total).Error; err != nil {
		return nil, 0, 0, fmt.Errorf("count open bans: %w", err)
	}
	if total == 0 {
		return nil, 0, 0, nil
	}
	page = min(max(page, 0), int((total-1)/int64(size)))
	var rows []models.BanRecord
	if err := scope().
		Select("ban_records.*").
		Order("ban_records.created_at DESC, ban_records.id DESC").
		Offset(page * size).
		Limit(size).
		Find(&rows).Error; err != nil {
		return nil, 0, 0, fmt.Errorf("list open bans: %w", err)
	}
	return rows, total, page, nil
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
