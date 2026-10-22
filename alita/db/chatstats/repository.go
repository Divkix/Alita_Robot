// Package chatstats stores per-chat daily activity counters.
//
// The table is written once per flush interval, never per event, and read by
// /digest now; reads deliberately bypass the read-through cache, because a
// cached day would hide the last flush and every replica adds to the same
// rows.
package chatstats

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

// dailyStatBatchSize caps the rows per INSERT. PostgreSQL rejects a statement
// with more than 65535 bind parameters, so one row per chat in a single
// statement would fail on a large install and, because the flush re-queues
// everything on error, fail again on every retry. 500 rows stay far below the
// limit on both PostgreSQL and SQLite.
const dailyStatBatchSize = 500

// AddDaily adds each row's counters to its (chat_id, day) row. The counters
// are added, not replaced, so two replicas that flush the same day produce the
// sum instead of the last write. Rows are inserted in batches so a flush with
// more chats than one statement's bind-parameter limit still succeeds; all
// batches are in one transaction, so a failure leaves no partial flush behind.
func AddDaily(rows []models.ChatDailyStat) error {
	if len(rows) == 0 {
		return nil
	}
	if db.DB == nil {
		return errors.New("database not initialized")
	}

	// Qualifying a column with the table name refers to the existing row in
	// both PostgreSQL and SQLite ON CONFLICT DO UPDATE; EXCLUDED is the row
	// proposed by this statement.
	err := db.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "chat_id"}, {Name: "day"}},
		DoUpdates: clause.Assignments(map[string]any{
			"messages": gorm.Expr("chat_daily_stats.messages + EXCLUDED.messages"),
			"joins":    gorm.Expr("chat_daily_stats.joins + EXCLUDED.joins"),
			"leaves":   gorm.Expr("chat_daily_stats.leaves + EXCLUDED.leaves"),
		}),
	}).CreateInBatches(&rows, dailyStatBatchSize).Error
	if err != nil {
		return fmt.Errorf("add daily chat stats: %w", err)
	}
	return nil
}

// GetRange returns the chat's rows from fromDay to toDay inclusive, oldest
// first. Both bounds are YYYY-MM-DD UTC keys from stats.Day.
func GetRange(chatID int64, fromDay, toDay string) ([]models.ChatDailyStat, error) {
	if db.DB == nil {
		return nil, errors.New("database not initialized")
	}

	var rows []models.ChatDailyStat
	err := db.DB.
		Where("chat_id = ? AND day >= ? AND day <= ?", chatID, fromDay, toDay).
		Order("day ASC").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("get daily chat stats: %w", err)
	}
	return rows, nil
}

// DeleteBefore removes every row for a day before day and returns how many it
// removed. The comparison is on the YYYY-MM-DD key, so it needs no timezone.
func DeleteBefore(day string) (int64, error) {
	if db.DB == nil {
		return 0, errors.New("database not initialized")
	}

	res := db.DB.Where("day < ?", day).Delete(&models.ChatDailyStat{})
	if res.Error != nil {
		return 0, fmt.Errorf("delete daily chat stats: %w", res.Error)
	}
	return res.RowsAffected, nil
}
