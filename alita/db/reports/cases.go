package reports

import (
	"fmt"
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
	if !isDecision(status) {
		return nil, fmt.Errorf("resolve report: invalid status %q", status)
	}
	err = db.DB.Transaction(func(tx *gorm.DB) error {
		var ids []int64
		if err := tx.Model(&models.Report{}).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("chat_id = ? AND reported_message_id = ? AND status = ?", chatID, messageID, models.ReportStatusOpen).
			Order("id").
			Pluck("id", &ids).Error; err != nil {
			return err
		}
		if len(ids) == 0 {
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

// SetAdminMessage records where the report card for a message was posted.
// It locks the open rows in id order, like Resolve, so the two cannot
// deadlock on the same message.
func SetAdminMessage(chatID, messageID, adminChatID, adminMessageID int64) error {
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		var ids []int64
		if err := tx.Model(&models.Report{}).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("chat_id = ? AND reported_message_id = ? AND status = ?", chatID, messageID, models.ReportStatusOpen).
			Order("id").
			Pluck("id", &ids).Error; err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		return tx.Model(&models.Report{}).
			Where("id IN ? AND status = ?", ids, models.ReportStatusOpen).
			Updates(map[string]any{
				"admin_chat_id":    adminChatID,
				"admin_message_id": adminMessageID,
				"updated_at":       time.Now(),
			}).Error
	})
	if err != nil {
		return fmt.Errorf("set report admin message: %w", err)
	}
	return nil
}
