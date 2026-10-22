// Package chatstats stores per-chat activity counters: per UTC day
// (chat_daily_stats), per UTC hour (chat_hourly_stats) and per member and day
// (chat_member_daily_stats), plus one Telegram member count per chat and day
// (chat_member_counts, membercounts.go).
//
// The tables are written once per flush interval, never per event, and read
// by /digest now and the panel; reads deliberately bypass the read-through
// cache, because a cached day would hide the last flush and every replica adds
// to the same rows.
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

// Batch is one flush's counters. Add writes all of it in one transaction: the
// flush puts the whole batch back when the write fails, so a partial commit
// would count the committed part twice on the retry.
type Batch struct {
	Daily   []models.ChatDailyStat
	Hourly  []models.ChatHourlyStat
	Members []models.ChatMemberDailyStat
}

func (b Batch) empty() bool {
	return len(b.Daily) == 0 && len(b.Hourly) == 0 && len(b.Members) == 0
}

// AddDaily adds each row's counters to its (chat_id, day) row. It is Add with
// daily rows only.
func AddDaily(rows []models.ChatDailyStat) error {
	return Add(Batch{Daily: rows})
}

// Add adds every row's counters to the row with the same key. The counters are
// added, not replaced, so two replicas that flush the same day or hour produce
// the sum instead of the last write. Rows are inserted in batches so a flush
// with more chats than one statement's bind-parameter limit still succeeds;
// every batch of every table is in one transaction, so a failure leaves no
// partial flush behind.
func Add(batch Batch) error {
	if batch.empty() {
		return nil
	}
	if db.DB == nil {
		return errors.New("database not initialized")
	}

	// Qualifying a column with the table name refers to the existing row in
	// both PostgreSQL and SQLite ON CONFLICT DO UPDATE; EXCLUDED is the row
	// proposed by this statement.
	return db.DB.Transaction(func(tx *gorm.DB) error {
		if len(batch.Daily) > 0 {
			err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "chat_id"}, {Name: "day"}},
				DoUpdates: clause.Assignments(map[string]any{
					"messages": gorm.Expr("chat_daily_stats.messages + EXCLUDED.messages"),
					"joins":    gorm.Expr("chat_daily_stats.joins + EXCLUDED.joins"),
					"leaves":   gorm.Expr("chat_daily_stats.leaves + EXCLUDED.leaves"),
				}),
			}).CreateInBatches(&batch.Daily, dailyStatBatchSize).Error
			if err != nil {
				return fmt.Errorf("add daily chat stats: %w", err)
			}
		}
		if len(batch.Hourly) > 0 {
			err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "chat_id"}, {Name: "day"}, {Name: "hour"}},
				DoUpdates: clause.Assignments(map[string]any{
					"messages": gorm.Expr("chat_hourly_stats.messages + EXCLUDED.messages"),
				}),
			}).CreateInBatches(&batch.Hourly, dailyStatBatchSize).Error
			if err != nil {
				return fmt.Errorf("add hourly chat stats: %w", err)
			}
		}
		if len(batch.Members) > 0 {
			err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "chat_id"}, {Name: "day"}, {Name: "user_id"}},
				DoUpdates: clause.Assignments(map[string]any{
					"messages": gorm.Expr("chat_member_daily_stats.messages + EXCLUDED.messages"),
				}),
			}).CreateInBatches(&batch.Members, dailyStatBatchSize).Error
			if err != nil {
				return fmt.Errorf("add member chat stats: %w", err)
			}
		}
		return nil
	})
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

// GetHourlyRange returns the chat's hourly rows from fromDay to toDay
// inclusive, oldest first. Both bounds are YYYY-MM-DD UTC keys from stats.Day.
func GetHourlyRange(chatID int64, fromDay, toDay string) ([]models.ChatHourlyStat, error) {
	if db.DB == nil {
		return nil, errors.New("database not initialized")
	}

	var rows []models.ChatHourlyStat
	err := db.DB.
		Where("chat_id = ? AND day >= ? AND day <= ?", chatID, fromDay, toDay).
		Order("day ASC").
		Order("hour ASC").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("get hourly chat stats: %w", err)
	}
	return rows, nil
}

// MemberTotal is one user's message count summed over a range of days.
type MemberTotal struct {
	UserID   int64 `gorm:"column:user_id"`
	Messages int64 `gorm:"column:messages"`
}

// TopMembers returns the chat's limit most active users from fromDay to toDay
// inclusive, most messages first; ties go to the lower user ID so the order is
// stable. Both bounds are YYYY-MM-DD UTC keys from stats.Day.
func TopMembers(chatID int64, fromDay, toDay string, limit int) ([]MemberTotal, error) {
	if db.DB == nil {
		return nil, errors.New("database not initialized")
	}
	if limit <= 0 {
		return nil, nil
	}

	var rows []MemberTotal
	err := db.DB.Model(&models.ChatMemberDailyStat{}).
		Select("user_id, SUM(messages) AS messages").
		Where("chat_id = ? AND day >= ? AND day <= ?", chatID, fromDay, toDay).
		Group("user_id").
		Order("SUM(messages) DESC").
		Order("user_id ASC").
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("get top chat members: %w", err)
	}
	return rows, nil
}

// DeleteBefore removes every row for a day before day and returns how many it
// removed. The comparison is on the YYYY-MM-DD key, so it needs no timezone.
func DeleteBefore(day string) (int64, error) {
	return deleteBefore(&models.ChatDailyStat{}, "daily", day)
}

// DeleteHourlyBefore removes every hourly row for a day before day and returns
// how many it removed.
func DeleteHourlyBefore(day string) (int64, error) {
	return deleteBefore(&models.ChatHourlyStat{}, "hourly", day)
}

// DeleteMemberDailyBefore removes every per-member row for a day before day
// and returns how many it removed.
func DeleteMemberDailyBefore(day string) (int64, error) {
	return deleteBefore(&models.ChatMemberDailyStat{}, "member", day)
}

func deleteBefore(model any, what, day string) (int64, error) {
	if db.DB == nil {
		return 0, errors.New("database not initialized")
	}

	res := db.DB.Where("day < ?", day).Delete(model)
	if res.Error != nil {
		return 0, fmt.Errorf("delete %s chat stats: %w", what, res.Error)
	}
	return res.RowsAffected, nil
}
