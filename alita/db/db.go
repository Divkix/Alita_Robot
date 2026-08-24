package db

import (
	"context"
	"errors"
	"fmt"

	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

// Re-export model types for backward compatibility
type (
	Button                 = models.Button
	ButtonArray            = models.ButtonArray
	Int64Array             = models.Int64Array
	User                   = models.User
	Chat                   = models.Chat
	ChatFilters            = models.ChatFilters
	BlacklistSettingsSlice = models.BlacklistSettingsSlice
	DevSettings            = models.DevSettings
	AntifloodSettings      = models.AntifloodSettings
	LockSettings           = models.LockSettings
	NotesSettings          = models.NotesSettings
	Notes                  = models.Notes
	ApprovedUsers          = models.ApprovedUsers
	CaptchaSettings        = models.CaptchaSettings
	CaptchaAttempts        = models.CaptchaAttempts
)

// Message type constants - maintain compatibility with existing code
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

// Default greeting messages used when no custom greetings are configured.
const (
	DefaultWelcome = "Hey {first}, how are you?"
	DefaultGoodbye = "Sad to see you leaving {first}"
)

// getSpanAttributes returns common span attributes for database operations.
func getSpanAttributes(model any) []attribute.KeyValue {
	attrs := []attribute.KeyValue{}
	if model != nil {
		attrs = append(attrs, attribute.String("db.model", fmt.Sprintf("%T", model)))
	}
	return attrs
}

// CreateRecord creates a new database record using the provided model.
func CreateRecord(model any) error {
	ctx := context.Background()
	ctx, span := tracing.StartSpan(ctx, "db.create",
		trace.WithAttributes(append(getSpanAttributes(model), tracing.WorkingModeAttribute())...))
	defer span.End()

	result := DB.WithContext(ctx).Create(model)
	if result.Error != nil {
		log.Errorf("[Database][CreateRecord]: %v", result.Error)
		span.SetStatus(codes.Error, result.Error.Error())
		return result.Error
	}
	span.SetAttributes(attribute.Int64("db.rows_affected", result.RowsAffected))
	return nil
}

// UpdateRecord updates an existing database record with the provided updates.
func UpdateRecord(model any, where any, updates any) error {
	return updateRecordInternal(context.Background(), model, where, updates, "UpdateRecord")
}

// UpdateRecordWithZeroValues updates a database record including zero values.
func UpdateRecordWithZeroValues(model any, where any, updates map[string]any) error {
	return updateRecordInternal(context.Background(), model, where, updates, "UpdateRecordWithZeroValues")
}

// updateRecordInternal is the shared implementation for record updates.
func updateRecordInternal(ctx context.Context, model any, where any, updates any, logPrefix string) error {
	ctx, span := tracing.StartSpan(ctx, "db.update",
		trace.WithAttributes(append(getSpanAttributes(model), tracing.WorkingModeAttribute())...))
	defer span.End()

	result := DB.WithContext(ctx).Model(model).Where(where).Updates(updates)
	if result.Error != nil {
		log.Errorf("[Database][%s]: %v", logPrefix, result.Error)
		span.SetStatus(codes.Error, result.Error.Error())
		return result.Error
	}
	if result.RowsAffected == 0 {
		span.SetStatus(codes.Error, "record not found")
		return gorm.ErrRecordNotFound
	}
	span.SetAttributes(attribute.Int64("db.rows_affected", result.RowsAffected))
	return nil
}

// GetRecord retrieves a single database record matching the where clause.
func GetRecord(model any, where any) error {
	ctx := context.Background()
	ctx, span := tracing.StartSpan(ctx, "db.get",
		trace.WithAttributes(append(getSpanAttributes(model), tracing.WorkingModeAttribute())...))
	defer span.End()

	result := DB.WithContext(ctx).Where(where).First(model)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			span.SetAttributes(attribute.Bool("db.record_found", false))
			return result.Error
		}
		log.Errorf("[Database][GetRecord]: %v", result.Error)
		span.SetStatus(codes.Error, result.Error.Error())
		return result.Error
	}
	span.SetAttributes(attribute.Bool("db.record_found", true))
	return nil
}

// ChatExists checks if a chat with the given ID exists in the database.
// Any error (including connection failures) is treated as "not exists" so callers
// that ensure the chat will attempt recovery instead of skipping FK setup.
func ChatExists(chatID int64) bool {
	chatExists := &Chat{}
	err := GetRecord(chatExists, Chat{ChatId: chatID})
	if err != nil {
		return false // not found OR any other error → treat as absent
	}
	return true
}

// GetRecords retrieves multiple database records matching the where clause.
func GetRecords(models any, where any) error {
	ctx := context.Background()
	ctx, span := tracing.StartSpan(ctx, "db.find",
		trace.WithAttributes(append(getSpanAttributes(models), tracing.WorkingModeAttribute())...))
	defer span.End()

	result := DB.WithContext(ctx).Where(where).Find(models)
	if result.Error != nil {
		log.Errorf("[Database][GetRecords]: %v", result.Error)
		span.SetStatus(codes.Error, result.Error.Error())
		return result.Error
	}
	span.SetAttributes(attribute.Int64("db.rows_affected", result.RowsAffected))
	return nil
}
