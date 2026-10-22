package chatstats

import (
	"errors"
	"fmt"

	"gorm.io/gorm/clause"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

// ListChatsDueMemberCount returns up to limit chats that had any activity on
// a day from sinceDay on and have no chat_member_counts row (claimed or
// written) for day yet, lowest chat ID first.
func ListChatsDueMemberCount(sinceDay, day string, limit int) ([]int64, error) {
	if db.DB == nil {
		return nil, errors.New("database not initialized")
	}
	if limit <= 0 {
		return nil, nil
	}

	var chatIDs []int64
	err := db.DB.Model(&models.ChatDailyStat{}).
		Distinct("chat_id").
		Where("day >= ? AND messages + joins + leaves > 0", sinceDay).
		Where("NOT EXISTS (SELECT 1 FROM chat_member_counts c WHERE c.chat_id = chat_daily_stats.chat_id AND c.day = ?)", day).
		Order("chat_id ASC").
		Limit(limit).
		Pluck("chat_id", &chatIDs).Error
	if err != nil {
		return nil, fmt.Errorf("list chats due a member count: %w", err)
	}
	return chatIDs, nil
}

// ClaimMemberCount inserts the chat's row for day with no count yet. It
// returns true only when this call inserted it: of two replicas that race, one
// gets true and calls Telegram, the other gets false and does nothing.
func ClaimMemberCount(chatID int64, day string) (bool, error) {
	if db.DB == nil {
		return false, errors.New("database not initialized")
	}

	res := db.DB.Clauses(clause.OnConflict{DoNothing: true}).
		Create(&models.ChatMemberCount{ChatID: chatID, Day: day})
	if res.Error != nil {
		return false, fmt.Errorf("claim member count: %w", res.Error)
	}
	return res.RowsAffected == 1, nil
}

// SetMemberCount writes the count into a claimed row. It fails when the row is
// gone or already has a count, which only a bug or a manual edit can cause.
func SetMemberCount(chatID int64, day string, count int64) error {
	if db.DB == nil {
		return errors.New("database not initialized")
	}

	res := db.DB.Model(&models.ChatMemberCount{}).
		Where("chat_id = ? AND day = ? AND count IS NULL", chatID, day).
		Update("count", count)
	if res.Error != nil {
		return fmt.Errorf("set member count: %w", res.Error)
	}
	if res.RowsAffected != 1 {
		return fmt.Errorf("set member count: no claimed row for chat %d on %s", chatID, day)
	}
	return nil
}

// ReleaseMemberCount deletes a claim that has no count yet, so the next tick
// retries the chat. A row that already has a count is left alone.
func ReleaseMemberCount(chatID int64, day string) error {
	if db.DB == nil {
		return errors.New("database not initialized")
	}

	err := db.DB.
		Where("chat_id = ? AND day = ? AND count IS NULL", chatID, day).
		Delete(&models.ChatMemberCount{}).Error
	if err != nil {
		return fmt.Errorf("release member count claim: %w", err)
	}
	return nil
}

// GetMemberCounts returns the chat's measured counts from fromDay to toDay
// inclusive, oldest first. Claims without a count are skipped.
func GetMemberCounts(chatID int64, fromDay, toDay string) ([]models.ChatMemberCount, error) {
	if db.DB == nil {
		return nil, errors.New("database not initialized")
	}

	var rows []models.ChatMemberCount
	err := db.DB.
		Where("chat_id = ? AND day >= ? AND day <= ? AND count IS NOT NULL", chatID, fromDay, toDay).
		Order("day ASC").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("get member counts: %w", err)
	}
	return rows, nil
}

// DeleteMemberCountsBefore removes every member count for a day before day and
// returns how many it removed.
func DeleteMemberCountsBefore(day string) (int64, error) {
	return deleteBefore(&models.ChatMemberCount{}, "member count", day)
}
