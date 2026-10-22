// Package recurringmessages stores the messages admins set to be posted in a
// chat every interval. The sending lives in alita/recurring.
package recurringmessages

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

const (
	// MaxPerChat is how many recurring messages a chat may have, ended and
	// stopped ones included until an admin removes them.
	MaxPerChat = 5
	// MinInterval is the shortest time between two occurrences.
	MinInterval = time.Hour
)

const cacheKeyPrefix = "recurring_messages"

var (
	// ErrLimitReached is returned by Add when the chat already has
	// MaxPerChat recurring messages.
	ErrLimitReached = errors.New("recurringmessages: the chat has the most recurring messages allowed")
	// ErrIntervalTooShort is returned by Add for an interval under MinInterval.
	ErrIntervalTooShort = errors.New("recurringmessages: interval is shorter than the minimum")
)

func listKey(chatID int64) string {
	return cache.CacheKey(cacheKeyPrefix, chatID)
}

func invalidate(chatID int64) {
	cache.DeleteCache(listKey(chatID))
}

// runTime is the form next_run_at is written and compared in: UTC, whole
// seconds. Postgres keeps microseconds, so a Go time with nanoseconds would
// never compare equal to the stored value and every claim would miss.
func runTime(t time.Time) time.Time {
	return t.UTC().Truncate(time.Second)
}

// List returns the chat's recurring messages, oldest first.
func List(chatID int64) ([]models.RecurringMessage, error) {
	return ListContext(context.Background(), chatID)
}

func ListContext(ctx context.Context, chatID int64) ([]models.RecurringMessage, error) {
	return cache.GetFromCacheOrLoad(ctx, listKey(chatID), cache.CacheTTLRecurring,
		func(ctx context.Context) ([]models.RecurringMessage, error) {
			if db.DB == nil {
				return nil, errors.New("database not initialized")
			}
			var rows []models.RecurringMessage
			err := db.DB.WithContext(ctx).Where("chat_id = ?", chatID).Order("id ASC").Find(&rows).Error
			if err != nil {
				return nil, fmt.Errorf("list recurring messages: %w", err)
			}
			return rows, nil
		})
}

// Add stores msg and sets its ID. It holds the chat row's lock while it
// counts, so two admins adding at once cannot pass MaxPerChat between them.
func Add(msg *models.RecurringMessage) error {
	if db.DB == nil {
		return errors.New("database not initialized")
	}
	if msg.Interval() < MinInterval {
		return ErrIntervalTooShort
	}
	if err := chats.EnsureChatInDb(msg.ChatID, ""); err != nil {
		return err
	}

	msg.Enabled = true
	msg.NextRunAt = runTime(msg.NextRunAt)
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		var chat models.Chat
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id").
			Where("chat_id = ?", msg.ChatID).
			Take(&chat).Error; err != nil {
			return fmt.Errorf("lock chat: %w", err)
		}

		var count int64
		if err := tx.Model(&models.RecurringMessage{}).Where("chat_id = ?", msg.ChatID).Count(&count).Error; err != nil {
			return fmt.Errorf("count recurring messages: %w", err)
		}
		if count >= MaxPerChat {
			return ErrLimitReached
		}
		return tx.Create(msg).Error
	})
	if err != nil {
		if errors.Is(err, ErrLimitReached) {
			return err
		}
		return fmt.Errorf("add recurring message: %w", err)
	}

	invalidate(msg.ChatID)
	return nil
}

// Remove deletes the chat's recurring message id. It reports false when the
// chat has no such message.
func Remove(chatID, id int64) (bool, error) {
	if db.DB == nil {
		return false, errors.New("database not initialized")
	}
	res := db.DB.Where("chat_id = ? AND id = ?", chatID, id).Delete(&models.RecurringMessage{})
	if res.Error != nil {
		return false, fmt.Errorf("remove recurring message: %w", res.Error)
	}
	if res.RowsAffected > 0 {
		invalidate(chatID)
	}
	return res.RowsAffected > 0, nil
}

// ListDue returns up to limit enabled messages whose next run is at or before
// now, the most overdue first. It bypasses the cache: the scheduler needs the
// row each replica will claim against, not a copy.
func ListDue(now time.Time, limit int) ([]models.RecurringMessage, error) {
	if db.DB == nil {
		return nil, errors.New("database not initialized")
	}
	var rows []models.RecurringMessage
	err := db.DB.
		Where("enabled AND next_run_at <= ?", now.UTC()).
		Order("next_run_at ASC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list due recurring messages: %w", err)
	}
	return rows, nil
}

// Claim takes ownership of msg's due occurrence by moving next_run_at to next
// only when it still holds the value the caller read. It returns true when
// exactly one row was updated, which is the only case in which the caller may
// send: two replicas that race get true from one and false from the other.
// With last set the message also stops (enabled=false): the claimed
// occurrence is its last, or it ended without one.
func Claim(msg *models.RecurringMessage, next time.Time, last bool) (bool, error) {
	if db.DB == nil {
		return false, errors.New("database not initialized")
	}
	updates := map[string]any{
		"next_run_at": runTime(next),
		"updated_at":  time.Now().UTC(),
	}
	if last {
		updates["enabled"] = false
	}
	res := db.DB.Model(&models.RecurringMessage{}).
		Where("id = ? AND enabled = ? AND next_run_at = ?", msg.ID, true, runTime(msg.NextRunAt)).
		Updates(updates)
	if res.Error != nil {
		return false, fmt.Errorf("claim recurring message: %w", res.Error)
	}
	if res.RowsAffected == 1 {
		invalidate(msg.ChatID)
	}
	return res.RowsAffected == 1, nil
}

// Restore gives a claim back after a send that shutdown stopped, so the
// occurrence goes out on the next tick. It only touches a row still holding
// the claimed next run: a slower replica must not undo a later claim.
func Restore(msg *models.RecurringMessage, claimedNext time.Time) error {
	if db.DB == nil {
		return errors.New("database not initialized")
	}
	res := db.DB.Model(&models.RecurringMessage{}).
		Where("id = ? AND next_run_at = ?", msg.ID, runTime(claimedNext)).
		Updates(map[string]any{
			"next_run_at": runTime(msg.NextRunAt),
			"enabled":     msg.Enabled,
			"updated_at":  time.Now().UTC(),
		})
	if res.Error != nil {
		return fmt.Errorf("restore recurring message claim: %w", res.Error)
	}
	invalidate(msg.ChatID)
	return nil
}

// SetLastMessageID records the message the last occurrence posted.
func SetLastMessageID(chatID, id, messageID int64) error {
	if db.DB == nil {
		return errors.New("database not initialized")
	}
	res := db.DB.Model(&models.RecurringMessage{}).
		Where("chat_id = ? AND id = ?", chatID, id).
		Updates(map[string]any{
			"last_message_id": messageID,
			"updated_at":      time.Now().UTC(),
		})
	if res.Error != nil {
		return fmt.Errorf("save recurring message's last message: %w", res.Error)
	}
	invalidate(chatID)
	return nil
}

// Disable stops the chat's recurring message id; it stays listed until an
// admin removes it.
func Disable(chatID, id int64) error {
	if db.DB == nil {
		return errors.New("database not initialized")
	}
	res := db.DB.Model(&models.RecurringMessage{}).
		Where("chat_id = ? AND id = ?", chatID, id).
		Updates(map[string]any{
			"enabled":    false,
			"updated_at": time.Now().UTC(),
		})
	if res.Error != nil {
		return fmt.Errorf("disable recurring message: %w", res.Error)
	}
	invalidate(chatID)
	return nil
}
