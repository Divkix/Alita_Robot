package blacklists

import (
	"context"
	"strings"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
)

// AddBlacklist adds a new blacklist word to a chat with default 'warn' action.
// The trigger is converted to lowercase before storage.
// Returns an error if the database operation fails.
func AddBlacklist(chatId int64, trigger string) error {
	// Create a new blacklist entry
	blacklist := &models.BlacklistSettings{
		ChatId: chatId,
		Word:   strings.ToLower(trigger),
		Action: "warn",                   // default action (intentionally 'warn' for safety)
		Reason: "Blacklisted word: '%s'", // default format string with placeholder for trigger word
	}

	err := db.CreateRecord(blacklist)
	if err != nil {
		log.Errorf("[Database] AddBlacklist: %v - %d", err, chatId)
		return err
	}

	// Invalidate cache after adding blacklist
	cache.DeleteCache(cache.CacheKey("blacklist", chatId))
	return nil
}

// RemoveBlacklist removes a specific blacklist word from a chat.
// The trigger is converted to lowercase before removal.
// Returns an error if the database operation fails.
func RemoveBlacklist(chatId int64, trigger string) error {
	collection := db.DB.Collection("blacklists")
	result, err := collection.DeleteOne(context.Background(), bson.M{"chat_id": chatId, "word": strings.ToLower(trigger)})
	if err != nil {
		log.Errorf("[Database] RemoveBlacklist: %v - %d", err, chatId)
		return err
	}

	// Invalidate cache if something was deleted
	if result.DeletedCount > 0 {
		cache.DeleteCache(cache.CacheKey("blacklist", chatId))
	}
	return nil
}

// RemoveAllBlacklist removes all blacklist entries for a specific chat.
// Returns an error if the database operation fails.
func RemoveAllBlacklist(chatId int64) error {
	collection := db.DB.Collection("blacklists")
	_, err := collection.DeleteMany(context.Background(), bson.M{"chat_id": chatId})
	if err != nil {
		log.Errorf("[Database] RemoveAllBlacklist: %v - %d", err, chatId)
		return err
	}

	// Invalidate cache after removing all blacklist entries
	cache.DeleteCache(cache.CacheKey("blacklist", chatId))
	return nil
}

// SetBlacklistAction updates the action for all blacklist entries in a chat.
// The action is converted to lowercase before storage.
func SetBlacklistAction(chatId int64, action string) error {
	collection := db.DB.Collection("blacklists")
	_, err := collection.UpdateMany(context.Background(), bson.M{"chat_id": chatId}, bson.M{"$set": bson.M{"action": strings.ToLower(action)}})
	if err != nil {
		log.Errorf("[Database] SetBlacklistAction: %v - %d", err, chatId)
		return err
	}

	// Invalidate cache after updating action
	cache.DeleteCache(cache.CacheKey("blacklist", chatId))
	return nil
}

// GetBlacklistSettings retrieves all blacklist settings for a chat with caching support.
// Returns an empty slice if no blacklists are found or on error.
func GetBlacklistSettings(chatId int64) models.BlacklistSettingsSlice {
	// Try to get from cache first
	cacheKey := cache.CacheKey("blacklist", chatId)
	result, err := cache.GetFromCacheOrLoad(cacheKey, cache.CacheTTLBlacklist, func() (models.BlacklistSettingsSlice, error) {
		var blacklists []*models.BlacklistSettings
		err := db.GetRecords(&blacklists, bson.M{"chat_id": chatId})
		if err != nil {
			log.Errorf("[Database] GetBlacklistSettings: %v - %d", err, chatId)
			return models.BlacklistSettingsSlice{}, err
		}
		return models.BlacklistSettingsSlice(blacklists), nil
	})
	if err != nil {
		return models.BlacklistSettingsSlice{}
	}
	return result
}

// LoadBlacklistsStats returns statistics about blacklist usage.
// Returns the total number of blacklist entries and distinct chats using blacklists.
func LoadBlacklistsStats() (blacklistTriggers, blacklistChats int64) {
	collection := db.DB.Collection("blacklists")

	// Count total blacklist entries
	var err error
	blacklistTriggers, err = collection.CountDocuments(context.Background(), bson.M{})
	if err != nil {
		log.Errorf("[Database] LoadBlacklistsStats (triggers): %v", err)
		return 0, 0
	}

	// Count distinct chats with blacklists
	distinctChats, err := collection.Distinct(context.Background(), "chat_id", bson.M{})
	if err != nil {
		log.Errorf("[Database] LoadBlacklistsStats (chats): %v", err)
		return blacklistTriggers, 0
	}
	blacklistChats = int64(len(distinctChats))

	return
}
