package antiflood

import (
	"context"
	"errors"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GetAntifloodSettings retrieves antiflood settings with minimal column selection.
// Optimized for high-frequency calls (58K+ calls) and returns default settings if none exist.
func GetAntifloodSettings(chatID int64) (*models.AntifloodSettings, error) {
	if db.DB == nil {
		return &models.AntifloodSettings{
			ChatId: chatID,
			Limit:  0,
			Action: "mute",
		}, errors.New("database not initialized")
	}

	var settings models.AntifloodSettings
	collection := db.DB.Collection("antiflood_settings")
	err := collection.FindOne(context.Background(), bson.M{"chat_id": chatID}, options.FindOne().SetProjection(bson.M{
		"chat_id":                  1,
		"flood_limit":              1,
		"action":                   1,
		"delete_antiflood_message": 1,
	})).Decode(&settings)

	if err == db.ErrRecordNotFound {
		return &models.AntifloodSettings{
			ChatId: chatID,
			Limit:  0,
			Action: "mute",
		}, nil
	}
	if err != nil {
		log.Errorf("[OptimizedAntifloodQueries] GetAntifloodSettings: %v", err)
		return nil, err
	}

	return &settings, nil
}

// GetAntifloodSettingsCached retrieves antiflood settings with caching layer for improved performance.
// Uses cache.CacheTTLAntiflood TTL and falls back to direct query if cache fails.
func GetAntifloodSettingsCached(chatID int64) (*models.AntifloodSettings, error) {
	cacheKey := cache.CacheKey("antiflood", chatID)

	cached, err := cache.GetFromCacheOrLoad(cacheKey, cache.CacheTTLAntiflood, func() (*models.AntifloodSettings, error) {
		return GetAntifloodSettings(chatID)
	})
	if err != nil {
		return GetAntifloodSettings(chatID)
	}

	return cached, nil
}
