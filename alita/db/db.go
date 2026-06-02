package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

// Re-exports for backward compatibility during migration
var (
	CacheKey    = cache.CacheKey
	DeleteCache = cache.DeleteCache
)

// getFromCacheOrLoad is a backward-compatible wrapper for the cache package.
func getFromCacheOrLoad[T any](key string, ttl time.Duration, loader func() (T, error)) (T, error) {
	return cache.GetFromCacheOrLoad(key, ttl, loader)
}

// deleteCache is a backward-compatible wrapper for the cache package.
func deleteCache(key string) {
	cache.DeleteCache(key)
}

// Backward-compatible TTL constant re-exports
const (
	CacheTTLChatSettings    = cache.CacheTTLChatSettings
	CacheTTLLanguage        = cache.CacheTTLLanguage
	CacheTTLFilterList      = cache.CacheTTLFilterList
	CacheTTLBlacklist       = cache.CacheTTLBlacklist
	CacheTTLGreetings       = cache.CacheTTLGreetings
	CacheTTLNotesList       = cache.CacheTTLNotesList
	CacheTTLNotesSettings   = cache.CacheTTLNotesSettings
	CacheTTLWarnSettings    = cache.CacheTTLWarnSettings
	CacheTTLAntiflood       = cache.CacheTTLAntiflood
	CacheTTLDisabledCmds    = cache.CacheTTLDisabledCmds
	CacheTTLCaptchaSettings = cache.CacheTTLCaptchaSettings
	CacheTTLApprovals       = cache.CacheTTLApprovals
	CacheTTLAntiRaid        = cache.CacheTTLAntiRaid
)

// Re-export model types for backward compatibility
type (
	Button            = models.Button
	ButtonArray       = models.ButtonArray
	StringArray       = models.StringArray
	Int64Array        = models.Int64Array
	User              = models.User
	Chat              = models.Chat
	ChatUser          = models.ChatUser
	WarnSettings      = models.WarnSettings
	Warns             = models.Warns
	GreetingSettings  = models.GreetingSettings
	WelcomeSettings   = models.WelcomeSettings
	GoodbyeSettings   = models.GoodbyeSettings
	ChatFilters       = models.ChatFilters
	AdminSettings     = models.AdminSettings
	BlacklistSettings = models.BlacklistSettings
	BlacklistSettingsSlice = models.BlacklistSettingsSlice
	PinSettings       = models.PinSettings
	ReportChatSettings = models.ReportChatSettings
	ReportUserSettings = models.ReportUserSettings
	DevSettings       = models.DevSettings
	ChannelSettings   = models.ChannelSettings
	AntifloodSettings = models.AntifloodSettings
	ConnectionSettings = models.ConnectionSettings
	ConnectionChatSettings = models.ConnectionChatSettings
	DisableSettings   = models.DisableSettings
	DisableChatSettings = models.DisableChatSettings
	RulesSettings     = models.RulesSettings
	LockSettings      = models.LockSettings
	NotesSettings     = models.NotesSettings
	Notes             = models.Notes
	ApprovedUsers     = models.ApprovedUsers
	CaptchaSettings   = models.CaptchaSettings
	CaptchaAttempts   = models.CaptchaAttempts
	StoredMessages    = models.StoredMessages
	CaptchaMutedUsers = models.CaptchaMutedUsers
	AntiRaidSettings  = models.AntiRaidSettings
)

// Message type constants - maintain compatibility with existing code
const (
	TEXT      int = 1
	STICKER   int = 2
	DOCUMENT  int = 3
	PHOTO     int = 4
	AUDIO     int = 5
	VOICE     int = 6
	VIDEO     int = 7
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
	return CreateRecordWithContext(context.Background(), model)
}

// CreateRecordWithContext creates a new database record with context support for trace propagation.
func CreateRecordWithContext(ctx context.Context, model any) error {
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
	return UpdateRecordWithContext(context.Background(), model, where, updates)
}

// UpdateRecordWithZeroValues updates a database record including zero values.
func UpdateRecordWithZeroValues(model any, where any, updates map[string]any) error {
	return UpdateRecordWithZeroValuesWithContext(context.Background(), model, where, updates)
}

// UpdateRecordWithContext updates a database record with context support.
func UpdateRecordWithContext(ctx context.Context, model any, where any, updates any) error {
	return updateRecordInternal(ctx, model, where, updates, "UpdateRecord")
}

// UpdateRecordWithZeroValuesWithContext updates a database record including zero values with context.
func UpdateRecordWithZeroValuesWithContext(ctx context.Context, model any, where any, updates map[string]any) error {
	return updateRecordInternal(ctx, model, where, updates, "UpdateRecordWithZeroValues")
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
	return GetRecordWithContext(context.Background(), model, where)
}

// GetRecordWithContext retrieves a single database record with context support.
func GetRecordWithContext(ctx context.Context, model any, where any) error {
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
func ChatExists(chatID int64) bool {
	chatExists := &Chat{}
	err := GetRecord(chatExists, Chat{ChatId: chatID})
	return !errors.Is(err, gorm.ErrRecordNotFound)
}

// GetRecords retrieves multiple database records matching the where clause.
func GetRecords(models any, where any) error {
	return GetRecordsWithContext(context.Background(), models, where)
}

// GetRecordsWithContext retrieves multiple database records with context support.
func GetRecordsWithContext(ctx context.Context, models any, where any) error {
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
