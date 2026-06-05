package db

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

var ErrRecordNotFound = mongo.ErrNoDocuments

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

// getCollectionName returns the MongoDB collection name for a given model or slice of models.
func getCollectionName(model any) string {
	val := reflect.ValueOf(model)
	for val.Kind() == reflect.Ptr || val.Kind() == reflect.Slice {
		if val.Kind() == reflect.Slice {
			if val.Len() > 0 {
				// Try to get TableName from an element
				if t, ok := val.Index(0).Interface().(interface{ TableName() string }); ok {
					return t.TableName()
				}
			}
			// If slice is empty or doesn't implement TableName, we need to get the element type
			elemType := val.Type().Elem()
			for elemType.Kind() == reflect.Ptr {
				elemType = elemType.Elem()
			}
			// Create a zero value of the element type to check for TableName()
			if t, ok := reflect.New(elemType).Interface().(interface{ TableName() string }); ok {
				return t.TableName()
			}
			return strings.ToLower(elemType.Name())
		}
		val = val.Elem()
	}

	if t, ok := val.Interface().(interface{ TableName() string }); ok {
		return t.TableName()
	}
	return strings.ToLower(val.Type().Name())
}

// getSpanAttributes returns common span attributes for database operations.
func getSpanAttributes(model any) []attribute.KeyValue {
	attrs := []attribute.KeyValue{}
	if model != nil {
		attrs = append(attrs, attribute.String("db.model", fmt.Sprintf("%T", model)))
		attrs = append(attrs, attribute.String("db.collection", getCollectionName(model)))
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

	collection := DB.Collection(getCollectionName(model))
	_, err := collection.InsertOne(ctx, model)
	if err != nil {
		log.Errorf("[Database][CreateRecord]: %v", err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
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

	collection := DB.Collection(getCollectionName(model))

	filter, err := toBson(where)
	if err != nil {
		return err
	}

	update, err := toBson(updates)
	if err != nil {
		return err
	}

	result, err := collection.UpdateMany(ctx, filter, bson.M{"$set": update})
	if err != nil {
		log.Errorf("[Database][%s]: %v", logPrefix, err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	if result.MatchedCount == 0 {
		span.SetStatus(codes.Error, "record not found")
		return ErrRecordNotFound
	}

	span.SetAttributes(attribute.Int64("db.rows_affected", result.ModifiedCount))
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

	collection := DB.Collection(getCollectionName(model))

	filter, err := toBson(where)
	if err != nil {
		return err
	}

	err = collection.FindOne(ctx, filter).Decode(model)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			span.SetAttributes(attribute.Bool("db.record_found", false))
			return err
		}
		log.Errorf("[Database][GetRecord]: %v", err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	span.SetAttributes(attribute.Bool("db.record_found", true))
	return nil
}

// ChatExists checks if a chat with the given ID exists in the database.
func ChatExists(chatID int64) bool {
	chatExists := &Chat{}
	err := GetRecord(chatExists, Chat{ChatId: chatID})
	return err == nil
}

// GetRecords retrieves multiple database records matching the where clause.
func GetRecords(models_slice any, where any) error {
	return GetRecordsWithContext(context.Background(), models_slice, where)
}

// GetRecordsWithContext retrieves multiple database records with context support.
func GetRecordsWithContext(ctx context.Context, models_slice any, where any) error {
	ctx, span := tracing.StartSpan(ctx, "db.find",
		trace.WithAttributes(append(getSpanAttributes(models_slice), tracing.WorkingModeAttribute())...))
	defer span.End()

	collection := DB.Collection(getCollectionName(models_slice))

	filter, err := toBson(where)
	if err != nil {
		return err
	}

	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		log.Errorf("[Database][GetRecords]: %v", err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	defer cursor.Close(ctx)

	if err := cursor.All(ctx, models_slice); err != nil {
		log.Errorf("[Database][GetRecords]: %v", err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}

// toBson converts a struct or map to bson.M for MongoDB queries,
// stripping zero values for structs to match GORM's default behavior for filters.
func toBson(v any) (bson.M, error) {
	if v == nil {
		return bson.M{}, nil
	}

	if m, ok := v.(bson.M); ok {
		return m, nil
	}

	if m, ok := v.(map[string]any); ok {
		return bson.M(m), nil
	}

	// For structs, we want to strip zero values if we're using it as a filter
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		// Fallback for non-structs
		data, err := bson.Marshal(v)
		if err != nil {
			return nil, err
		}
		var m bson.M
		err = bson.Unmarshal(data, &m)
		return m, err
	}

	m := bson.M{}
	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// Respect bson tags
		bsonTag := fieldType.Tag.Get("bson")
		if bsonTag == "-" || bsonTag == "" {
			continue
		}

		name := strings.Split(bsonTag, ",")[0]

		// Skip zero values
		if isZero(field) {
			continue
		}

		m[name] = field.Interface()
	}

	return m, nil
}

func isZero(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Func, reflect.Map, reflect.Slice:
		return v.IsNil()
	case reflect.Array:
		z := true
		for i := 0; i < v.Len(); i++ {
			z = z && isZero(v.Index(i))
		}
		return z
	case reflect.Struct:
		if t, ok := v.Interface().(time.Time); ok {
			return t.IsZero()
		}
		z := true
		for i := 0; i < v.NumField(); i++ {
			z = z && isZero(v.Field(i))
		}
		return z
	}
	// For other types, use the standard IsZero if available (Go 1.13+)
	return v.IsZero()
}
