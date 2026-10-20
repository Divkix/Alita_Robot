package reports

import (
	"fmt"
	"slices"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

// Create stores a report. A second report of the same message by the same
// member is ignored: created is false and r.ID stays zero.
func Create(r *models.Report) (created bool, err error) {
	if err := chats.EnsureChatInDb(r.ChatId, ""); err != nil {
		return false, fmt.Errorf("create report: %w", err)
	}
	if r.Status == "" {
		r.Status = models.ReportStatusOpen
	}
	res := db.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}, {Name: "reported_message_id"}, {Name: "reporter_id"}},
		DoNothing: true,
	}).Create(r)
	if res.Error != nil {
		return false, fmt.Errorf("create report: %w", res.Error)
	}
	return res.RowsAffected > 0, nil
}

// GetByID returns gorm.ErrRecordNotFound when no report has this id.
func GetByID(id int64) (*models.Report, error) {
	var r models.Report
	if err := db.DB.Where("id = ?", id).Take(&r).Error; err != nil {
		return nil, err
	}
	return &r, nil
}

func OpenForMessage(chatID, messageID int64) ([]models.Report, error) {
	var rows []models.Report
	err := db.DB.
		Where("chat_id = ? AND reported_message_id = ? AND status = ?", chatID, messageID, models.ReportStatusOpen).
		Order("id").
		Find(&rows).Error
	return rows, err
}

// HasReports reports whether a message has any stored report, open or
// decided.
func HasReports(chatID, messageID int64) (bool, error) {
	var ids []int64
	err := db.DB.Model(&models.Report{}).
		Where("chat_id = ? AND reported_message_id = ?", chatID, messageID).
		Limit(1).
		Pluck("id", &ids).Error
	if err != nil {
		return false, fmt.Errorf("check reports of message: %w", err)
	}
	return len(ids) > 0, nil
}

// DeleteExpiredBefore removes closed reports resolved before cutoff and open
// reports made before it, and returns how many it removed.
func DeleteExpiredBefore(cutoff time.Time) (int64, error) {
	// 'open' (models.ReportStatusOpen) is a literal so a prepared generic plan can match the partial indexes.
	res := db.DB.
		Where("(status <> 'open' AND resolved_at < ?) OR (status = 'open' AND created_at < ?)", cutoff, cutoff).
		Delete(&models.Report{})
	if res.Error != nil {
		return 0, fmt.Errorf("delete expired reports: %w", res.Error)
	}
	return res.RowsAffected, nil
}

func isDecision(status string) bool {
	switch status {
	case models.ReportStatusBanned, models.ReportStatusKicked, models.ReportStatusMuted,
		models.ReportStatusDeleted, models.ReportStatusDismissed:
		return true
	}
	return false
}

// Resolve closes every open report of a message with one decision and
// returns the ids it closed. Only open rows change, so of several admins
// acting at once exactly one gets ids back; the rest get none and must not
// act. Rows are locked in id order so concurrent resolvers cannot split a
// message's reports between them.
func Resolve(chatID, messageID int64, status string, resolvedBy int64) (closed []int64, err error) {
	return resolveLocked(status, resolvedBy, 0, func(q *gorm.DB) *gorm.DB {
		return q.Where("chat_id = ? AND reported_message_id = ?", chatID, messageID)
	})
}

// ResolveCard closes the open reports of r's message on r's report card,
// like Resolve, and only while r itself is open: a card whose report was
// decided returns no ids, even if a newer report of the message is open on
// a card of its own. Reports not linked to a card yet are closed too: while
// r is open, a new report is counted onto r's card before it is linked to
// it. r must have a card.
func ResolveCard(r *models.Report, status string, resolvedBy int64) (closed []int64, err error) {
	if r.AdminChatId == nil || r.AdminMessageId == nil {
		return nil, fmt.Errorf("resolve report %d: no report card", r.ID)
	}
	return resolveLocked(status, resolvedBy, r.ID, func(q *gorm.DB) *gorm.DB {
		return q.Where("chat_id = ? AND reported_message_id = ? AND ((admin_chat_id = ? AND admin_message_id = ?) OR admin_message_id IS NULL)",
			r.ChatId, r.ReportedMessageId, *r.AdminChatId, *r.AdminMessageId)
	})
}

// resolveLocked closes the open reports scope selects, locking them in id
// order like every writer of a message's open rows. With mustInclude set it
// closes nothing unless that report is among them.
func resolveLocked(status string, resolvedBy, mustInclude int64, scope func(*gorm.DB) *gorm.DB) (closed []int64, err error) {
	if !isDecision(status) {
		return nil, fmt.Errorf("resolve report: invalid status %q", status)
	}
	err = db.DB.Transaction(func(tx *gorm.DB) error {
		var ids []int64
		if err := scope(tx.Model(&models.Report{}).
			Clauses(clause.Locking{Strength: "UPDATE"})).
			Where("status = ?", models.ReportStatusOpen).
			Order("id").
			Pluck("id", &ids).Error; err != nil {
			return err
		}
		if len(ids) == 0 || (mustInclude != 0 && !slices.Contains(ids, mustInclude)) {
			return nil
		}
		now := time.Now()
		res := tx.Model(&models.Report{}).
			Where("id IN ? AND status = ?", ids, models.ReportStatusOpen).
			Updates(map[string]any{
				"status":      status,
				"resolved_by": resolvedBy,
				"resolved_at": now,
				"updated_at":  now,
			})
		if res.Error != nil {
			return res.Error
		}
		// The rows are locked, so every one of them changed.
		if res.RowsAffected != int64(len(ids)) {
			return fmt.Errorf("closed %d of %d locked reports", res.RowsAffected, len(ids))
		}
		closed = ids
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("resolve report: %w", err)
	}
	return closed, nil
}

// Reopen undoes a Resolve whose action failed: the reports it closed with
// status become open again, so the decision can be retried.
func Reopen(ids []int64, status string) error {
	if len(ids) == 0 {
		return nil
	}
	err := db.DB.Model(&models.Report{}).
		Where("id IN ? AND status = ?", ids, status).
		Updates(map[string]any{
			"status":      models.ReportStatusOpen,
			"resolved_by": nil,
			"resolved_at": nil,
			"updated_at":  time.Now(),
		}).Error
	if err != nil {
		return fmt.Errorf("reopen reports: %w", err)
	}
	return nil
}

// SetAdminMessage links the open reports of a message to report card
// adminMessageID in adminChatID and returns the card they are linked to
// afterwards, or 0 when none is open. replaces is the card of adminChatID
// the caller found on them before posting its own, or 0. Any other card of
// adminChatID on them was posted by a concurrent report of the message and
// recorded first: it keeps the reports and the rest join it, so a message
// never has two cards that act. It locks the open rows in id order, like
// Resolve, so the two cannot deadlock on the same message.
func SetAdminMessage(chatID, messageID, adminChatID, adminMessageID, replaces int64) (recorded int64, err error) {
	err = db.DB.Transaction(func(tx *gorm.DB) error {
		var rows []models.Report
		if err := tx.Model(&models.Report{}).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id", "admin_chat_id", "admin_message_id").
			Where("chat_id = ? AND reported_message_id = ? AND status = ?", chatID, messageID, models.ReportStatusOpen).
			Order("id").
			Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		card := adminMessageID
		ids := make([]int64, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.ID)
			if card == adminMessageID && row.AdminChatId != nil && *row.AdminChatId == adminChatID &&
				row.AdminMessageId != nil && *row.AdminMessageId != replaces && *row.AdminMessageId != adminMessageID {
				card = *row.AdminMessageId
			}
		}
		if err := tx.Model(&models.Report{}).
			Where("id IN ? AND status = ?", ids, models.ReportStatusOpen).
			Updates(map[string]any{
				"admin_chat_id":    adminChatID,
				"admin_message_id": card,
				"updated_at":       time.Now(),
			}).Error; err != nil {
			return err
		}
		recorded = card
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("set report admin message: %w", err)
	}
	return recorded, nil
}
