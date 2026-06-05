package warns

import (
	"context"
	"errors"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/i18n"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// checkWarnSettings retrieves or creates default warn settings for a chat.
func checkWarnSettings(chatID int64) (warnrc *models.WarnSettings) {
	defaultWarnSettings := &models.WarnSettings{ChatId: chatID, WarnLimit: 3, WarnMode: "mute"}
	warnrc = &models.WarnSettings{}
	err := db.GetRecord(warnrc, bson.M{"chat_id": chatID})
	if errors.Is(err, db.ErrRecordNotFound) {
		// Ensure chat exists before creating warn settings
		if !db.ChatExists(chatID) {
			log.Warnf("[Database][checkWarnSettings]: Chat %d doesn't exist, returning default settings", chatID)
			return defaultWarnSettings
		}

		// Create default settings only if chat exists
		warnrc = defaultWarnSettings
		warnrc.CreatedAt = time.Now()
		warnrc.UpdatedAt = time.Now()
		err := db.CreateRecord(warnrc)
		if err != nil {
			log.Errorf("[Database] checkWarnSettings: %v", err)
		}
	} else if err != nil {
		log.Errorf("[Database][checkWarnSettings]: %d - %v", chatID, err)
		warnrc = defaultWarnSettings
	}
	return
}

// checkWarns retrieves or creates default warn record for a user in a specific chat.
func checkWarns(userId, chatId int64) (warnrc *models.Warns) {
	defaultWarnSrc := &models.Warns{UserId: userId, ChatId: chatId, NumWarns: 0, Reasons: make(models.StringArray, 0)}
	warnrc = &models.Warns{}
	err := db.GetRecord(warnrc, bson.M{"user_id": userId, "chat_id": chatId})
	if errors.Is(err, db.ErrRecordNotFound) {
		// Ensure chat exists before creating warn record
		if !db.ChatExists(chatId) {
			log.Warnf("[Database][checkWarns]: Chat %d doesn't exist, returning default settings", chatId)
			return defaultWarnSrc
		}

		// Create default record only if chat exists
		warnrc = defaultWarnSrc
		warnrc.CreatedAt = time.Now()
		warnrc.UpdatedAt = time.Now()
		err := db.CreateRecord(warnrc)
		if err != nil {
			log.Errorf("[Database] checkWarns: %v", err)
		}
	} else if err != nil {
		log.Errorf("[Database][checkUserWarns]: %d - %v", userId, err)
		warnrc = defaultWarnSrc
	}
	return
}

// WarnUser adds a warning to a user in a specific chat with an optional reason.
func WarnUser(userId, chatId int64, reason string) (int, []string) {
	return WarnUserWithContext(context.Background(), userId, chatId, reason)
}

// WarnUserWithContext adds a warning to a user with context support for cancellation.
func WarnUserWithContext(ctx context.Context, userId, chatId int64, reason string) (int, []string) {
	// MongoDB doesn't use the same transaction pattern as GORM easily without replica sets.
	// For simplicity and compatibility, we'll use findOneAndUpdate or atomic updates.

	collectionSettings := db.DB.Collection("warns_settings")
	opts := options.Update().SetUpsert(true)
	_, _ = collectionSettings.UpdateOne(ctx, bson.M{"chat_id": chatId}, bson.M{"$setOnInsert": bson.M{"warn_limit": 3, "warn_mode": "mute", "created_at": time.Now()}}, opts)

	if reason == "" {
		tr := i18n.MustNewTranslator("en")
		reason, _ = tr.GetString("db_warn_no_reason")
		if reason == "" {
			reason = "No Reason"
		}
	}
	if len(reason) >= 3001 {
		reason = reason[:3000]
	}

	collectionWarns := db.DB.Collection("warns_users")
	filter := bson.M{"user_id": userId, "chat_id": chatId}
	update := bson.M{
		"$inc": bson.M{"num_warns": 1},
		"$push": bson.M{"warns": reason},
		"$set": bson.M{"updated_at": time.Now()},
		"$setOnInsert": bson.M{"created_at": time.Now()},
	}

	var warnrc models.Warns
	err := collectionWarns.FindOneAndUpdate(ctx, filter, update, options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)).Decode(&warnrc)

	if err != nil {
		log.Errorf("[Database] WarnUser: %v", err)
		return 0, []string{}
	}

	cache.DeleteCache(cache.CacheKey("warns", userId, chatId))
	cache.DeleteCache(cache.CacheKey("warn_settings", chatId))

	return warnrc.NumWarns, []string(warnrc.Reasons)
}

// RemoveWarn removes the most recent warning from a user in a specific chat.
func RemoveWarn(userId, chatId int64) bool {
	return RemoveWarnWithContext(context.Background(), userId, chatId)
}

// RemoveWarnWithContext removes the most recent warning with context support.
func RemoveWarnWithContext(ctx context.Context, userId, chatId int64) bool {
	collection := db.DB.Collection("warns_users")

	// We need to get the current state to pop the last reason correctly if we want to mimic the previous behavior exactly.
	// MongoDB's $pop only removes from ends, but we also decrement num_warns.

	filter := bson.M{"user_id": userId, "chat_id": chatId, "num_warns": bson.M{"$gt": 0}}
	update := bson.M{
		"$inc": bson.M{"num_warns": -1},
		"$pop": bson.M{"warns": 1}, // Removes the last element
		"$set": bson.M{"updated_at": time.Now()},
	}

	res, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		log.Errorf("[Database] RemoveWarn: %v", err)
		return false
	}

	if res.ModifiedCount > 0 {
		cache.DeleteCache(cache.CacheKey("warns", userId, chatId))
		cache.DeleteCache(cache.CacheKey("warn_settings", chatId))
		return true
	}

	return false
}

// ResetUserWarns removes all warnings for a specific user in a chat.
func ResetUserWarns(userId, chatId int64) (removed bool) {
	collection := db.DB.Collection("warns_users")
	_, err := collection.DeleteOne(context.Background(), bson.M{"user_id": userId, "chat_id": chatId})
	if err != nil {
		log.Errorf("[Database] ResetUserWarns: %v", err)
		return false
	}
	cache.DeleteCache(cache.CacheKey("warns", userId, chatId))
	cache.DeleteCache(cache.CacheKey("warn_settings", chatId))
	return true
}

// GetWarns retrieves the current warning count and reasons for a user in a specific chat.
func GetWarns(userId, chatId int64) (int, []string) {
	type warnCache struct {
		NumWarns int
		Reasons  []string
	}
	cached, err := cache.GetFromCacheOrLoad(
		cache.CacheKey("warns", userId, chatId),
		cache.CacheTTLLanguage,
		func() (warnCache, error) {
			w := checkWarns(userId, chatId)
			return warnCache{NumWarns: w.NumWarns, Reasons: []string(w.Reasons)}, nil
		},
	)
	if err != nil {
		w := checkWarns(userId, chatId)
		return w.NumWarns, []string(w.Reasons)
	}
	return cached.NumWarns, cached.Reasons
}

// SetWarnLimit updates the warning limit for a specific chat.
func SetWarnLimit(chatId int64, warnLimit int) error {
	collection := db.DB.Collection("warns_settings")
	_, err := collection.UpdateOne(context.Background(), bson.M{"chat_id": chatId}, bson.M{"$set": bson.M{"warn_limit": warnLimit, "updated_at": time.Now()}}, options.Update().SetUpsert(true))
	if err != nil {
		log.Errorf("[Database] SetWarnLimit: %v", err)
		return err
	}
	cache.DeleteCache(cache.CacheKey("warn_settings", chatId))
	return nil
}

// SetWarnMode updates the action to take when users reach the warning limit.
func SetWarnMode(chatId int64, warnMode string) error {
	collection := db.DB.Collection("warns_settings")
	_, err := collection.UpdateOne(context.Background(), bson.M{"chat_id": chatId}, bson.M{"$set": bson.M{"warn_mode": warnMode, "updated_at": time.Now()}}, options.Update().SetUpsert(true))
	if err != nil {
		log.Errorf("[Database] SetWarnMode: %v", err)
		return err
	}
	cache.DeleteCache(cache.CacheKey("warn_settings", chatId))
	return nil
}

// GetWarnSetting returns the warning settings for the specified chat.
func GetWarnSetting(chatId int64) *models.WarnSettings {
	cached, err := cache.GetFromCacheOrLoad(
		cache.CacheKey("warn_settings", chatId),
		cache.CacheTTLLanguage,
		func() (*models.WarnSettings, error) {
			return checkWarnSettings(chatId), nil
		},
	)
	if err != nil {
		return checkWarnSettings(chatId)
	}
	return cached
}

// GetAllChatWarns returns the total count of warned users in a specific chat.
func GetAllChatWarns(chatId int64) int {
	collection := db.DB.Collection("warns_users")
	count, err := collection.CountDocuments(context.Background(), bson.M{"chat_id": chatId})
	if err != nil {
		log.Errorf("[Database] GetAllChatWarns: %v", err)
		return 0
	}
	return int(count)
}

// ResetAllChatWarns removes all warning records for all users in a specific chat.
func ResetAllChatWarns(chatId int64) bool {
	collection := db.DB.Collection("warns_users")

	// Collect user IDs before deletion
	cursor, err := collection.Find(context.Background(), bson.M{"chat_id": chatId}, options.Find().SetProjection(bson.M{"user_id": 1}))
	if err == nil {
		var results []struct {
			UserID int64 `bson:"user_id"`
		}
		if err := cursor.All(context.Background(), &results); err == nil {
			for _, res := range results {
				cache.DeleteCache(cache.CacheKey("warns", res.UserID, chatId))
			}
		}
		cursor.Close(context.Background())
	}

	_, err = collection.DeleteMany(context.Background(), bson.M{"chat_id": chatId})
	if err != nil {
		log.Errorf("[Database] ResetAllChatWarns: %v", err)
		return false
	}
	cache.DeleteCache(cache.CacheKey("warn_settings", chatId))
	return true
}
