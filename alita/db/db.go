package db

import (
	"context"
	"errors"
	"fmt"

	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

type (
	Button            = models.Button
	User              = models.User
	Chat              = models.Chat
	ChatFilters       = models.ChatFilters
	DevSettings       = models.DevSettings
	AntifloodSettings = models.AntifloodSettings
	LockSettings      = models.LockSettings
	NotesSettings     = models.NotesSettings
	Notes             = models.Notes
	ApprovedUsers     = models.ApprovedUsers
	CaptchaSettings   = models.CaptchaSettings
	CaptchaAttempts   = models.CaptchaAttempts
)

const (
	TEXT       int = 1
	STICKER    int = 2
	DOCUMENT   int = 3
	PHOTO      int = 4
	AUDIO      int = 5
	VOICE      int = 6
	VIDEO      int = 7
	VIDEO_NOTE int = 8
)

const (
	DefaultWelcome = "Hey {first}, how are you?"
	DefaultGoodbye = "Sad to see you leaving {first}"
)

func getSpanAttributes(model any) []attribute.KeyValue {
	attrs := []attribute.KeyValue{}
	if model != nil {
		attrs = append(attrs, attribute.String("db.model", fmt.Sprintf("%T", model)))
	}
	return attrs
}

func withSpan(ctx context.Context, op string, model any, fn func(ctx context.Context, span trace.Span) error) error {
	ctx, span := tracing.StartDBSpan(ctx, op, model)
	defer span.End()
	return fn(ctx, span)
}

func CreateRecord(model any) error {
	return CreateRecordContext(context.Background(), model)
}

func CreateRecordContext(ctx context.Context, model any) error {
	return withSpan(ctx, "db.create", model, func(ctx context.Context, span trace.Span) error {
		result := DB.WithContext(ctx).Create(model)
		if result.Error != nil {
			log.WithContext(ctx).Errorf("[Database][CreateRecord]: %v", result.Error)
			recordQueryError(span, result.Error)
			return result.Error
		}
		span.SetAttributes(attribute.Int64("db.rows_affected", result.RowsAffected))
		return nil
	})
}

func UpdateRecord(model any, where any, updates any) error {
	return updateRecordInternal(context.Background(), model, where, updates, "UpdateRecord")
}

func UpdateRecordWithZeroValues(model any, where any, updates map[string]any) error {
	return updateRecordInternal(context.Background(), model, where, updates, "UpdateRecordWithZeroValues")
}

func updateRecordInternal(ctx context.Context, model any, where any, updates any, logPrefix string) error {
	ctx, span := tracing.StartDBSpan(ctx, "db.update", model)
	defer span.End()

	result := DB.WithContext(ctx).Model(model).Where(where).Updates(updates)
	if result.Error != nil {
		log.WithContext(ctx).Errorf("[Database][%s]: %v", logPrefix, result.Error)
		recordQueryError(span, result.Error)
		return result.Error
	}
	if result.RowsAffected == 0 {
		// Callers expect a miss; it is not a failed query.
		span.SetAttributes(attribute.Bool("db.record_found", false))
		return gorm.ErrRecordNotFound
	}
	span.SetAttributes(attribute.Int64("db.rows_affected", result.RowsAffected))
	return nil
}

func GetRecord(model any, where any) error {
	return GetRecordContext(context.Background(), model, where)
}

func GetRecordContext(ctx context.Context, model any, where any) error {
	return withSpan(ctx, "db.get", model, func(ctx context.Context, span trace.Span) error {
		result := DB.WithContext(ctx).Where(where).First(model)
		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				span.SetAttributes(attribute.Bool("db.record_found", false))
				return result.Error
			}
			log.WithContext(ctx).Errorf("[Database][GetRecord]: %v", result.Error)
			recordQueryError(span, result.Error)
			return result.Error
		}
		span.SetAttributes(attribute.Bool("db.record_found", true))
		return nil
	})
}

func ChatExists(chatID int64) bool {
	return ChatExistsContext(context.Background(), chatID)
}

func ChatExistsContext(ctx context.Context, chatID int64) bool {
	chatExists := &Chat{}
	return GetRecordContext(ctx, chatExists, Chat{ChatId: chatID}) == nil
}

// TableRowCount returns an estimated row count for the given table.
// On PostgreSQL it reads pg_stat_user_tables.n_live_tup (O(1), updated by the
// stats collector on every insert and delete), avoiding the full-table-scan
// that COUNT(*) requires under MVCC. n_live_tup covers the whole table only
// once a VACUUM or ANALYZE has run since the stats were last reset (a crash or
// failover resets them); until then it counts just the rows inserted since,
// so the estimate is pg_class.reltuples (as of the last VACUUM or ANALYZE)
// plus the net inserts since the reset. A table never vacuumed or analyzed
// (reltuples -1) has no estimate: COUNT(*) gives it, as it does on SQLite in
// tests.
func TableRowCount(tableName string) int64 {
	if DB == nil {
		return 0
	}
	var count int64
	estimate := DB.Raw(`SELECT CASE
		WHEN coalesce(s.last_vacuum, s.last_autovacuum, s.last_analyze, s.last_autoanalyze) IS NOT NULL THEN s.n_live_tup
		ELSE greatest(c.reltuples::bigint + coalesce(s.n_tup_ins - s.n_tup_del, 0), 0)
	END
	FROM pg_class c LEFT JOIN pg_stat_user_tables s ON s.relid = c.oid
	WHERE c.oid = to_regclass(?) AND c.reltuples >= 0`, tableName).Scan(&count)
	if estimate.Error == nil && estimate.RowsAffected > 0 {
		return count
	}
	DB.Table(tableName).Count(&count)
	return count
}

// CountRows returns how many rows of model match the where clause (empty where counts all rows).
func CountRows(model any, where string, args ...any) int64 {
	if DB == nil {
		return 0
	}
	var count int64
	if err := countQuery(model, where, args...).Count(&count).Error; err != nil {
		log.Errorf("[Database][CountRows]: %v", err)
	}
	return count
}

// CountDistinct returns how many distinct values of column the matching rows of model span.
func CountDistinct(model any, column, where string, args ...any) int64 {
	if DB == nil {
		return 0
	}
	var count int64
	if err := countQuery(model, where, args...).Distinct(column).Count(&count).Error; err != nil {
		log.Errorf("[Database][CountDistinct]: %v", err)
	}
	return count
}

func countQuery(model any, where string, args ...any) *gorm.DB {
	query := DB.Model(model)
	if where != "" {
		query = query.Where(where, args...)
	}
	return query
}

func GetRecords(models any, where any) error {
	return GetRecordsContext(context.Background(), models, where)
}

func GetRecordsContext(ctx context.Context, models any, where any) error {
	return withSpan(ctx, "db.find", models, func(ctx context.Context, span trace.Span) error {
		result := DB.WithContext(ctx).Where(where).Find(models)
		if result.Error != nil {
			log.WithContext(ctx).Errorf("[Database][GetRecords]: %v", result.Error)
			recordQueryError(span, result.Error)
			return result.Error
		}
		span.SetAttributes(attribute.Int64("db.rows_affected", result.RowsAffected))
		return nil
	})
}
