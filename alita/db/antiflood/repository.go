package antiflood

import (
	"context"
	"errors"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// default mode is 'mute'
const defaultFloodsettingsMode string = "mute"

// GetFlood Get flood settings for a chat
func GetFlood(chatID int64) *models.AntifloodSettings {
	return checkFloodSetting(chatID)
}

// checkFloodSetting retrieves or returns default antiflood settings for a chat.
// Uses optimized cached queries and returns default settings if not found.
func checkFloodSetting(chatID int64) (floodSrc *models.AntifloodSettings) {
	// Use optimized cached query instead of SELECT *
	floodSrc, err := GetAntifloodSettingsCached(chatID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			// Return default settings
			return &models.AntifloodSettings{ChatId: chatID, Limit: 0, Action: defaultFloodsettingsMode}
		}
		log.Errorf("[Database][checkFloodSetting]: %v", err)
		return &models.AntifloodSettings{ChatId: chatID, Limit: 0, Action: defaultFloodsettingsMode}
	}
	return floodSrc
}

// SetFlood set Flood Setting for a Chat
func SetFlood(chatID int64, limit int) error {
	floodSrc := checkFloodSetting(chatID)

	// Check if update is actually needed
	if floodSrc.Limit == limit {
		return nil
	}

	action := floodSrc.Action
	if action == "" {
		action = defaultFloodsettingsMode
	}

	updates := bson.M{
		"flood_limit": limit,
		"action":      action,
		"updated_at":  time.Now(),
	}

	collection := db.DB.Collection("antiflood_settings")
	opts := options.Update().SetUpsert(true)
	_, err := collection.UpdateOne(context.Background(), bson.M{"chat_id": chatID}, bson.M{"$set": updates}, opts)

	if err != nil {
		log.Errorf("[Database] SetFlood: %v - %d", err, chatID)
		return err
	}
	// Invalidate cache after update
	cache.DeleteCache(cache.CacheKey("antiflood", chatID))
	return nil
}

// SetFloodMode Set flood mode for a chat
func SetFloodMode(chatID int64, mode string) error {
	floodSrc := checkFloodSetting(chatID)
	// Check if update is actually needed
	if floodSrc.Action == mode {
		return nil
	}

	updates := bson.M{
		"action":     mode,
		"updated_at": time.Now(),
	}

	collection := db.DB.Collection("antiflood_settings")
	opts := options.Update().SetUpsert(true)
	_, err := collection.UpdateOne(context.Background(), bson.M{"chat_id": chatID}, bson.M{"$set": updates}, opts)

	if err != nil {
		log.Errorf("[Database] SetFloodMode: %v - %d", err, chatID)
		return err
	}
	// Invalidate cache after update
	cache.DeleteCache(cache.CacheKey("antiflood", chatID))
	return nil
}

// SetFloodMsgDel Set flood message deletion setting for a chat
func SetFloodMsgDel(chatID int64, val bool) error {
	floodSrc := checkFloodSetting(chatID)
	// Check if update is actually needed
	if floodSrc.DeleteAntifloodMessage == val {
		return nil
	}

	updates := bson.M{
		"delete_antiflood_message": val,
		"updated_at":               time.Now(),
	}

	collection := db.DB.Collection("antiflood_settings")
	opts := options.Update().SetUpsert(true)
	_, err := collection.UpdateOne(context.Background(), bson.M{"chat_id": chatID}, bson.M{"$set": updates}, opts)

	if err != nil {
		log.Errorf("[Database] SetFloodMsgDel: %v", err)
		return err
	}
	// Invalidate cache after update
	cache.DeleteCache(cache.CacheKey("antiflood", chatID))
	return nil
}

// LoadAntifloodStats returns the count of chats with antiflood enabled (limit > 0).
func LoadAntifloodStats() (antiCount int64) {
	collection := db.DB.Collection("antiflood_settings")

	// Count settings with limit > 0
	antiCount, err := collection.CountDocuments(context.Background(), bson.M{"flood_limit": bson.M{"$gt": 0}})
	if err != nil {
		log.Errorf("[Database] LoadAntifloodStats: %v", err)
		return 0
	}

	return antiCount
}
